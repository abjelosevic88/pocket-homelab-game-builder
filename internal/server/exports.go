package server

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/gamedoc"
	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/store"
)

// The export is everything a person made: games (with their documents),
// the library (pictures and sounds inside, as data), saved copies, to-dos
// and plays. Importing twice changes nothing: games are matched by title and
// when they were made, pictures and sounds by name and size.

type exportGame struct {
	store.Game
	DocRaw json.RawMessage `json:"document"`
}

type exportAsset struct {
	store.Asset
	Data string `json:"data"`
}

type exportDoc struct {
	App        string          `json:"app"`
	Version    string          `json:"version"`
	ExportedAt string          `json:"exported_at"`
	Games      []exportGame    `json:"games"`
	Assets     []exportAsset   `json:"assets"`
	Versions   []store.Version `json:"versions"`
	Todos      []store.Todo    `json:"todos"`
	Plays      []store.Play    `json:"plays"`
}

func (s *Server) export(ctx context.Context, uid int64) (*exportDoc, error) {
	doc := &exportDoc{App: "pocket-gamebuilder", Version: Version, ExportedAt: store.Now(), Games: []exportGame{}, Assets: []exportAsset{}, Versions: []store.Version{}}
	for _, arch := range []bool{false, true} {
		list, err := s.store.ListGames(ctx, uid, store.GameFilter{Archived: arch})
		if err != nil {
			return nil, err
		}
		for _, g := range list {
			full, err := s.store.GetGame(ctx, uid, g.ID)
			if err != nil {
				return nil, err
			}
			eg := exportGame{Game: *full, DocRaw: full.Doc}
			eg.Doc = nil
			doc.Games = append(doc.Games, eg)
			vs, _ := s.store.ListVersions(ctx, uid, g.ID)
			for _, v := range vs {
				if fv, err := s.store.GetVersion(ctx, uid, v.ID); err == nil {
					doc.Versions = append(doc.Versions, *fv)
				}
			}
		}
	}
	assets, err := s.store.ListAssets(ctx, uid, "")
	if err != nil {
		return nil, err
	}
	for _, a := range assets {
		full, err := s.store.GetAsset(ctx, uid, a.ID)
		if err != nil {
			continue
		}
		ea := exportAsset{Asset: *full}
		if b, err := os.ReadFile(s.mediaPath(a.Path)); err == nil {
			ea.Data = "data:" + a.Mime + ";base64," + base64.StdEncoding.EncodeToString(b)
		}
		doc.Assets = append(doc.Assets, ea)
	}
	if doc.Todos, err = s.store.ListTodos(ctx, uid, 0); err != nil {
		return nil, err
	}
	if doc.Plays, err = s.store.ListPlays(ctx, uid, store.PlayFilter{Limit: 1000000}); err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	doc, err := s.export(r.Context(), actorID(r.Context()))
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pocket-gamebuilder-%s.json"`, time.Now().Format("2006-01-02")))
	_ = json.NewEncoder(w).Encode(doc)
}

// handleExportCSV is one row per play.
func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	plays, err := s.store.ListPlays(ctx, actorID(ctx), store.PlayFilter{Limit: 1000000})
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pocket-gamebuilder-plays-%s.csv"`, time.Now().Format("2006-01-02")))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"when", "game", "who", "score", "seconds", "result", "level"})
	for _, p := range plays {
		who := p.Who
		if p.Mine {
			who = "(you)"
		}
		_ = cw.Write([]string{p.CreatedAt, p.Game, who, strconv.FormatFloat(p.Score, 'f', -1, 64), strconv.FormatFloat(p.Seconds, 'f', 1, 64), p.Result, p.Level})
	}
	cw.Flush()
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var doc exportDoc
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<20)).Decode(&doc); err != nil || doc.App != "pocket-gamebuilder" {
		writeErr(w, http.StatusBadRequest, "that is not a Pocket Game Builder export")
		return
	}
	res, err := s.importDoc(r, actorID(ctx), &doc)
	if err != nil {
		s.userFail(w, err, "import")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) importDoc(r *http.Request, uid int64, doc *exportDoc) (map[string]any, error) {
	ctx := r.Context()
	added := map[string]int{"games": 0, "assets": 0, "versions": 0, "todos": 0, "plays": 0}
	// the library
	have, _ := s.store.ListAssets(ctx, uid, "")
	byKey := map[string]int64{}
	for _, a := range have {
		byKey[fmt.Sprintf("%s|%s|%d", a.Kind, a.Name, a.Size)] = a.ID
	}
	remap := map[int64]int64{}
	for _, ea := range doc.Assets {
		key := fmt.Sprintf("%s|%s|%d", ea.Kind, ea.Name, ea.Size)
		if id, ok := byKey[key]; ok {
			remap[ea.ID] = id
			continue
		}
		b, err := dataURL(ea.Data)
		if err != nil {
			continue
		}
		a, err := s.addAsset(r, uid, ea.Kind, ea.Name, b, ea.Origin, ea.OriginRef, 0)
		if err != nil {
			continue
		}
		a.Name, a.Frames, a.Pixels = ea.Name, ea.Frames, ea.Pixels
		_, _ = s.store.UpdateAsset(ctx, a)
		remap[ea.ID] = a.ID
		byKey[key] = a.ID
		added["assets"]++
	}
	// games
	existing := map[string]int64{}
	for _, arch := range []bool{false, true} {
		list, _ := s.store.ListGames(ctx, uid, store.GameFilter{Archived: arch})
		for _, g := range list {
			existing[g.Title+"|"+g.CreatedAt] = g.ID
		}
	}
	gameMap := map[int64]int64{}
	for _, eg := range doc.Games {
		if id, ok := existing[eg.Title+"|"+eg.CreatedAt]; ok {
			gameMap[eg.ID] = id
			continue
		}
		raw := remapAssets(eg.DocRaw, remap)
		if _, err := gamedoc.Check(raw); err != nil {
			continue
		}
		g := &store.Game{UserID: uid, Title: eg.Title, Description: eg.Description, Template: eg.Template, Doc: raw, ScoresOn: eg.ScoresOn, RemixOn: eg.RemixOn, Favorite: eg.Favorite}
		if err := s.store.CreateGame(ctx, g); err != nil {
			return nil, err
		}
		_, _ = s.store.DB.ExecContext(ctx, `UPDATE games SET created_at = ?, archived = ? WHERE id = ? AND user_id = ?`, eg.CreatedAt, boolInt(eg.Archived), g.ID, uid)
		gameMap[eg.ID] = g.ID
		added["games"]++
	}
	for _, v := range doc.Versions {
		gid, ok := gameMap[v.GameID]
		if !ok {
			continue
		}
		vs, _ := s.store.ListVersions(ctx, uid, gid)
		dup := false
		for _, x := range vs {
			if x.CreatedAt == v.CreatedAt && x.Note == v.Note {
				dup = true
			}
		}
		if dup {
			continue
		}
		if nv, err := s.store.AddVersion(ctx, uid, gid, v.Note, remapAssets(v.Doc, remap), 0); err == nil {
			_, _ = s.store.DB.ExecContext(ctx, `UPDATE versions SET created_at = ? WHERE id = ? AND user_id = ?`, v.CreatedAt, nv.ID, uid)
			added["versions"]++
		}
	}
	for _, t := range doc.Todos {
		gid, ok := gameMap[t.GameID]
		if !ok {
			continue
		}
		list, _ := s.store.ListTodos(ctx, uid, gid)
		dup := false
		for _, x := range list {
			if x.Text == t.Text {
				dup = true
			}
		}
		if dup {
			continue
		}
		nt := &store.Todo{UserID: uid, GameID: gid, Text: t.Text, Done: t.Done, TaskURL: t.TaskURL}
		if s.store.CreateTodo(ctx, nt) == nil {
			added["todos"]++
		}
	}
	seen := map[string]bool{}
	for gid := range gameMap {
		_ = gid
	}
	for _, ng := range gameMap {
		ps, _ := s.store.ListPlays(ctx, uid, store.PlayFilter{GameID: ng, Limit: 1000000})
		for _, p := range ps {
			seen[fmt.Sprintf("%d|%s|%s", ng, p.CreatedAt, p.Who)] = true
		}
	}
	for _, p := range doc.Plays {
		gid, ok := gameMap[p.GameID]
		if !ok || seen[fmt.Sprintf("%d|%s|%s", gid, p.CreatedAt, p.Who)] {
			continue
		}
		np := p
		np.ID, np.UserID, np.GameID = 0, uid, gid
		if s.store.AddPlay(ctx, &np) == nil {
			_, _ = s.store.DB.ExecContext(ctx, `UPDATE plays SET created_at = ? WHERE id = ?`, p.CreatedAt, np.ID)
			added["plays"]++
		}
	}
	return map[string]any{"added": added}, nil
}

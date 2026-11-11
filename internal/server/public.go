package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/gamedoc"
	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/store"
)

// Shared games: /p/<key> plays a game for anyone with the link (no
// account; a name for the best-scores table), /e/<key> is the same without
// anything around it, for putting in a page. Only what was shared is
// sent: the game as it was when shared, and the pictures and sounds it uses.

func dataURL(v string) ([]byte, error) {
	i := strings.Index(v, ";base64,")
	if !strings.HasPrefix(v, "data:") || i < 0 {
		return nil, errors.New("not a data URL")
	}
	return base64.StdEncoding.DecodeString(v[i+8:])
}

func (s *Server) sharedGame(r *http.Request) (*store.Game, []byte, error) {
	g, err := s.store.GameByShareKey(r.Context(), chiParam(r, "key"))
	if err != nil {
		return nil, nil, err
	}
	doc := []byte(g.Published)
	if len(doc) == 0 {
		doc = g.Doc
	}
	return g, doc, nil
}

func (s *Server) makerName(ctx context.Context, uid int64) string {
	st := s.settingsOf(ctx, uid)
	if st.MakerName != "" {
		return st.MakerName
	}
	if u, err := s.store.GetUser(ctx, uid); err == nil {
		return strings.Fields(firstNonEmpty(u.Name, "someone"))[0]
	}
	return ""
}

// handlePlayerPage is the shared player page (the player bundle).
func (s *Server) handlePlayerPage(w http.ResponseWriter, r *http.Request) {
	if s.webFS == nil {
		http.Error(w, "web UI not built", http.StatusNotFound)
		return
	}
	b, err := fs.ReadFile(s.webFS, "play.html")
	if err != nil {
		http.Error(w, "the player is missing (run make web)", http.StatusNotFound)
		return
	}
	title := "Pocket Game Builder"
	desc := "A game made with Pocket Game Builder."
	if g, _, err := s.sharedGame(r); err == nil {
		title = g.Title
		if g.Description != "" {
			desc = g.Description
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
	page := strings.Replace(string(b), "<title>Pocket Game Builder</title>",
		"<title>"+html.EscapeString(title)+"</title><meta property=\"og:title\" content=\""+html.EscapeString(title)+"\"><meta name=\"description\" content=\""+html.EscapeString(desc)+"\"><meta property=\"og:image\" content=\""+html.EscapeString(s.link("/api/v1/public/"+chiParam(r, "key")+"/cover.png"))+"\">", 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if strings.HasPrefix(r.URL.Path, "/e/") {
		w.Header().Set("Content-Security-Policy", "frame-ancestors *")
	} else {
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	}
	_, _ = w.Write([]byte(page))
}

func (s *Server) handlePublicGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	g, doc, err := s.sharedGame(r)
	if err != nil {
		writeErr(w, http.StatusNotFound, "this game is not shared (any more)")
		return
	}
	out := map[string]any{"title": g.Title, "description": g.Description, "doc": json.RawMessage(doc), "by": s.makerName(ctx, g.UserID),
		"scores_on": g.ScoresOn, "remix_on": g.RemixOn, "plays": g.Plays, "key": g.ShareKey, "low_is_best": lowIsBest(doc), "frames": s.framesOf(ctx, g.UserID, doc)}
	if g.ScoresOn {
		best, _ := s.store.BestPlays(ctx, g.UserID, g.ID, lowIsBest(doc), 10)
		for i := range best {
			best[i].CreatedAt = best[i].CreatedAt[:min(10, len(best[i].CreatedAt))]
		}
		out["best"] = best
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePublicAsset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	g, doc, err := s.sharedGame(r)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	id := idParam(r, "id")
	used := false
	for _, x := range gamedoc.AssetIDs(doc) {
		if x == id {
			used = true
		}
	}
	if !used {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	a, err := s.store.GetAsset(ctx, g.UserID, id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	s.serveMediaFile(w, r, a.Path, a.Mime, "")
}

func (s *Server) handlePublicCover(w http.ResponseWriter, r *http.Request) {
	g, _, err := s.sharedGame(r)
	if err != nil || g.Cover == "" {
		http.Redirect(w, r, "/icon-512.png", http.StatusFound)
		return
	}
	s.serveMediaFile(w, r, g.Cover, "", "")
}

// a small limit on plays sent from one address to one game
type limiter struct {
	mu   sync.Mutex
	seen map[string][]time.Time
}

var playLimit = &limiter{seen: map[string][]time.Time{}}

func (l *limiter) allow(key string, n int, per time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	keep := l.seen[key][:0]
	for _, t := range l.seen[key] {
		if now.Sub(t) < per {
			keep = append(keep, t)
		}
	}
	if len(keep) >= n {
		l.seen[key] = keep
		return false
	}
	l.seen[key] = append(keep, now)
	if len(l.seen) > 5000 {
		l.seen = map[string][]time.Time{}
	}
	return true
}

func (s *Server) handlePublicPlay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	g, doc, err := s.sharedGame(r)
	if err != nil {
		writeErr(w, http.StatusNotFound, "this game is not shared (any more)")
		return
	}
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i > 0 {
		ip = ip[:i]
	}
	if !playLimit.allow(ip+" "+g.ShareKey, 20, time.Minute) {
		writeErr(w, http.StatusTooManyRequests, "slow down a little")
		return
	}
	p, err := readPlay(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !g.ScoresOn {
		p.Who = ""
	}
	low := lowIsBest(doc)
	before, _ := s.store.BestPlays(ctx, g.UserID, g.ID, low, 1)
	p.UserID, p.GameID = g.UserID, g.ID
	if err := s.store.AddPlay(ctx, p); err != nil {
		s.fail(w, err, "play")
		return
	}
	rank := 0
	beat := false
	if g.ScoresOn && p.Who != "" && p.Result != "quit" {
		best, _ := s.store.BestPlays(ctx, g.UserID, g.ID, low, 10)
		for i, b := range best {
			if strings.EqualFold(b.Who, p.Who) && b.Score == p.Score {
				rank = i + 1
				break
			}
		}
		if rank == 1 && len(before) > 0 && !strings.EqualFold(before[0].Who, p.Who) && (low && p.Score < before[0].Score || !low && p.Score > before[0].Score) {
			beat = true
		}
	}
	s.publish(ctx, g.UserID, "gamebuilder.played", g.Title, map[string]any{"game_id": g.ID, "title": g.Title, "who": p.Who, "score": p.Score, "result": p.Result})
	if beat {
		s.publish(ctx, g.UserID, "gamebuilder.best_score", g.Title, map[string]any{"game_id": g.ID, "title": g.Title, "who": p.Who, "score": p.Score})
		st := s.settingsOf(ctx, g.UserID)
		if st.NotifyBest {
			go s.tell(context.WithoutCancel(ctx), g.UserID, fmt.Sprintf("New best score in %s", g.Title), fmt.Sprintf("%s scored %s, beating %s.", p.Who, fmtScore(p.Score), before[0].Who), fmt.Sprintf("/games/%d", g.ID))
		}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"rank": rank, "best": beat})
}

func fmtScore(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprint(int64(v))
	}
	return fmt.Sprintf("%.1f", v)
}

// handleRemix takes a copy of a shared game someone allowed copies of.
func (s *Server) handleRemix(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, doc, err := s.sharedGame(r)
	if err != nil || (!g.RemixOn && g.UserID != uid) {
		writeErr(w, http.StatusNotFound, "this game cannot be copied")
		return
	}
	// pictures and sounds come along as the person's own
	ids := gamedoc.AssetIDs(doc)
	remap := map[int64]int64{}
	if g.UserID != uid {
		for _, id := range ids {
			a, err := s.store.GetAsset(ctx, g.UserID, id)
			if err != nil {
				continue
			}
			b, err := os.ReadFile(s.mediaPath(a.Path))
			if err != nil {
				continue
			}
			rel, err := s.saveBytes(uid, extOf(a.Path), b)
			if err != nil {
				continue
			}
			na := *a
			na.ID, na.UserID, na.Path, na.GameID, na.Origin, na.OriginRef = 0, uid, rel, nil, "link", "remix:"+g.ShareKey
			if s.store.CreateAsset(ctx, &na) == nil {
				remap[id] = na.ID
			}
		}
		doc = remapAssets(doc, remap)
	}
	title := clip(g.Title+" (my copy)", maxName)
	ng := &store.Game{UserID: uid, Title: title, Description: g.Description, Template: g.Template, Doc: gamedoc.SetTitle(doc, title), ScoresOn: true}
	if err := s.store.CreateGame(ctx, ng); err != nil {
		s.fail(w, err, "copy")
		return
	}
	ng, _ = s.store.GetGame(ctx, uid, ng.ID)
	writeJSON(w, http.StatusCreated, s.gameOut(ng))
}

func extOf(p string) string {
	if i := strings.LastIndex(p, "."); i >= 0 {
		return p[i:]
	}
	return ".bin"
}

// remapAssets swaps asset ids inside a document.
func remapAssets(doc []byte, m map[int64]int64) []byte {
	if len(m) == 0 {
		return doc
	}
	var v any
	if json.Unmarshal(doc, &v) != nil {
		return doc
	}
	var walk func(x any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for k, val := range t {
				if k == "image" || k == "bgImage" || k == "asset" {
					if f, ok := val.(float64); ok {
						if n, ok := m[int64(f)]; ok {
							t[k] = float64(n)
						}
					}
					continue
				}
				walk(val)
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	walk(v)
	b, err := json.Marshal(v)
	if err != nil {
		return doc
	}
	return b
}

// ---- one file that plays anywhere ----

// standalone is a game as one HTML file: the player, the game and its
// pictures and sounds inside it. It opens from a disk, a USB stick or any
// web space, with no server.
func (s *Server) standalone(ctx context.Context, uid int64, g *store.Game) ([]byte, error) {
	if s.webFS == nil {
		return nil, &userErr{"the web part is not built on this server"}
	}
	js, err := fs.ReadFile(s.webFS, "play/player.js")
	if err != nil {
		return nil, &userErr{"the player is missing on this server"}
	}
	css, _ := fs.ReadFile(s.webFS, "play/player.css")
	assets := map[string]string{}
	for _, id := range gamedoc.AssetIDs(g.Doc) {
		a, err := s.store.GetAsset(ctx, uid, id)
		if err != nil {
			continue
		}
		b, err := os.ReadFile(s.mediaPath(a.Path))
		if err != nil {
			continue
		}
		assets[fmt.Sprint(id)] = "data:" + firstNonEmpty(a.Mime, "application/octet-stream") + ";base64," + base64.StdEncoding.EncodeToString(b)
	}
	payload, _ := json.Marshal(map[string]any{"doc": json.RawMessage(g.Doc), "assets": assets, "frames": s.framesOf(ctx, uid, g.Doc), "title": g.Title, "by": s.makerName(ctx, uid)})
	var b bytes.Buffer
	b.WriteString("<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1, viewport-fit=cover\">")
	b.WriteString("<title>" + html.EscapeString(g.Title) + "</title><meta name=\"generator\" content=\"Pocket Game Builder " + html.EscapeString(Version) + "\">")
	b.WriteString("<style>")
	b.Write(css)
	b.WriteString("</style></head><body><div id=\"player\"></div>\n<script>window.__GAME__=")
	b.Write(bytes.ReplaceAll(payload, []byte("</"), []byte("<\\/")))
	b.WriteString(";</script>\n<script>")
	b.Write(bytes.ReplaceAll(js, []byte("</script"), []byte("<\\/script")))
	b.WriteString("</script></body></html>\n")
	return b.Bytes(), nil
}

func (s *Server) handleExportHTML(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, err := s.store.GetGame(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	b, err := s.standalone(ctx, uid, g)
	if err != nil {
		s.userFail(w, err, "export")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+urlName(g.Title)+".html")
	_, _ = w.Write(b)
}

// gameFile is a game with its pictures and sounds, to bring to another box.
type gameFile struct {
	App    string            `json:"app"`
	Kind   string            `json:"kind"`
	Title  string            `json:"title"`
	About  string            `json:"description"`
	Doc    json.RawMessage   `json:"doc"`
	Assets []gameFileAsset   `json:"assets"`
	Extra  map[string]string `json:"extra,omitempty"`
}

type gameFileAsset struct {
	ID     int64  `json:"id"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Frames int    `json:"frames"`
	Pixels string `json:"pixels,omitempty"`
	Data   string `json:"data"`
}

func (s *Server) handleExportGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, err := s.store.GetGame(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	f := gameFile{App: "pocket-gamebuilder", Kind: "game", Title: g.Title, About: g.Description, Doc: g.Doc, Assets: []gameFileAsset{}}
	for _, id := range gamedoc.AssetIDs(g.Doc) {
		a, err := s.store.GetAsset(ctx, uid, id)
		if err != nil {
			continue
		}
		b, err := os.ReadFile(s.mediaPath(a.Path))
		if err != nil {
			continue
		}
		f.Assets = append(f.Assets, gameFileAsset{ID: a.ID, Kind: a.Kind, Name: a.Name, Frames: a.Frames, Pixels: a.Pixels,
			Data: "data:" + a.Mime + ";base64," + base64.StdEncoding.EncodeToString(b)})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+urlName(g.Title)+".game.json")
	_ = json.NewEncoder(w).Encode(f)
}

// handleImportGame takes a .game.json (from another box, or this one).
func (s *Server) handleImportGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var f gameFile
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<20)).Decode(&f); err != nil || f.App != "pocket-gamebuilder" || f.Kind != "game" {
		writeErr(w, http.StatusBadRequest, "that is not a game file from Pocket Game Builder")
		return
	}
	if _, err := gamedoc.Check(f.Doc); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	remap := map[int64]int64{}
	for _, fa := range f.Assets {
		b, err := dataURL(fa.Data)
		if err != nil {
			continue
		}
		a, err := s.addAsset(r, uid, fa.Kind, fa.Name, b, "upload", "import", 0)
		if err != nil {
			continue
		}
		if fa.Pixels != "" || fa.Frames > 1 {
			a.Pixels, a.Frames = fa.Pixels, fa.Frames
			_, _ = s.store.UpdateAsset(ctx, a)
		}
		remap[fa.ID] = a.ID
	}
	g, err := s.makeGame(r, uid, newGame{Title: f.Title, Description: f.About, Doc: remapAssets(f.Doc, remap)})
	if err != nil {
		s.userFail(w, err, "import")
		return
	}
	writeJSON(w, http.StatusCreated, s.gameOut(g))
}

func urlName(title string) string {
	t := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(title))
	if t == "" {
		t = "game"
	}
	return strings.ReplaceAll(strings.ReplaceAll(t, " ", "%20"), "'", "%27")
}

// framesOf is how many animation frames each picture a game uses has.
func (s *Server) framesOf(ctx context.Context, uid int64, doc []byte) map[string]int {
	out := map[string]int{}
	for _, id := range gamedoc.AssetIDs(doc) {
		if a, err := s.store.GetAsset(ctx, uid, id); err == nil && a.Frames > 1 {
			out[fmt.Sprint(id)] = a.Frames
		}
	}
	return out
}

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/gamedoc"
	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/store"
)

// Games: list, make (from a template, an idea, a copy or a file), save,
// share, saved copies, plays, best scores, to-dos.

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"templates": gamedoc.Templates, "legends": gamedoc.Legends, "sizes": gamedoc.Sizes})
}

// gameOut adds what the page needs to a game: its addresses.
func (s *Server) gameOut(g *store.Game) map[string]any {
	b, _ := json.Marshal(g)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	m["url"] = s.link(fmt.Sprintf("/games/%d", g.ID))
	m["cover_url"] = ""
	if g.HasCover {
		m["cover_url"] = fmt.Sprintf("/api/v1/games/%d/cover?v=%d", g.ID, g.Saves)
	}
	m["share_url"] = ""
	if g.ShareOn && g.ShareKey != "" {
		m["share_url"] = s.link("/p/" + g.ShareKey)
		m["embed_url"] = s.link("/e/" + g.ShareKey)
	}
	m["changed_since_share"] = g.ShareOn && len(g.Published) > 0 && !bytes.Equal(compactJSON(g.Published), compactJSON(g.Doc))
	return m
}

func compactJSON(b []byte) []byte {
	var buf bytes.Buffer
	if json.Compact(&buf, b) != nil {
		return b
	}
	return buf.Bytes()
}

func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f := store.GameFilter{Query: r.URL.Query().Get("q"), Limit: queryInt(r, "limit")}
	switch r.URL.Query().Get("filter") {
	case "fav":
		f.Favorite = true
	case "shared":
		f.Shared = true
	case "archived":
		f.Archived = true
	}
	list, err := s.store.ListGames(ctx, actorID(ctx), f)
	if err != nil {
		s.fail(w, err, "games")
		return
	}
	out := make([]map[string]any, 0, len(list))
	for i := range list {
		out = append(out, s.gameOut(&list[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"games": out})
}

func (s *Server) handleGetGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	g, err := s.store.GetGame(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	writeJSON(w, http.StatusOK, s.gameOut(g))
}

type newGame struct {
	Template    string             `json:"template"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Size        string             `json:"size"`
	Options     gamedoc.Options    `json:"options"`
	Doc         json.RawMessage    `json:"doc"`
	Questions   []gamedoc.Question `json:"questions"`
}

// makeGame builds and stores a new game from a template (or a whole
// document). It is what the page, Pocket AI and other apps use.
func (s *Server) makeGame(r *http.Request, uid int64, in newGame) (*store.Game, error) {
	ctx := r.Context()
	st := s.settingsOf(ctx, uid)
	title := cleanText(in.Title, maxName)
	var doc []byte
	tpl := in.Template
	if len(in.Doc) > 0 && string(in.Doc) != "null" {
		t, err := gamedoc.Check(in.Doc)
		if err != nil {
			return nil, &userErr{err.Error()}
		}
		title = firstNonEmpty(title, t, "My game")
		doc = gamedoc.SetTitle(in.Doc, title)
		if tpl == "" {
			tpl = "import"
		}
	} else {
		if tpl == "" {
			tpl = "blank"
		}
		if _, ok := gamedoc.TemplateByID(tpl); !ok {
			return nil, &userErr{"there is no template called " + tpl}
		}
		o := in.Options
		o.Title = firstNonEmpty(title, o.Title)
		o.Size = firstNonEmpty(in.Size, o.Size)
		if o.Size == "" && tpl == "blank" {
			o.Size = st.DefaultSize
		}
		if len(in.Questions) > 0 {
			o.Questions = in.Questions
		}
		d, err := gamedoc.Build(tpl, o)
		if err != nil {
			return nil, &userErr{err.Error()}
		}
		title = d.Title
		doc = d.Marshal()
	}
	g := &store.Game{UserID: uid, Title: clip(title, maxName), Description: cleanText(in.Description, 2000), Template: tpl, Doc: doc,
		ScoresOn: st.ScoresOn, Source: sourceOf(ctx)}
	if err := s.store.CreateGame(ctx, g); err != nil {
		return nil, err
	}
	s.publish(ctx, uid, "gamebuilder.game_made", g.Title, map[string]any{"game_id": g.ID, "title": g.Title, "template": tpl, "url": s.link(fmt.Sprintf("/games/%d", g.ID))})
	return s.store.GetGame(ctx, uid, g.ID)
}

func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	var in newGame
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "send the game as JSON")
		return
	}
	g, err := s.makeGame(r, actorID(r.Context()), in)
	if err != nil {
		s.userFail(w, err, "new game")
		return
	}
	writeJSON(w, http.StatusCreated, s.gameOut(g))
}

func (s *Server) handleUpdateGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, err := s.store.GetGame(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	if !mayChange(ctx, g.Source) {
		writeErr(w, http.StatusForbidden, "this app may only change games it made")
		return
	}
	var in struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		ScoresOn    bool   `json:"scores_on"`
		RemixOn     bool   `json:"remix_on"`
		Favorite    bool   `json:"favorite"`
		Archived    bool   `json:"archived"`
	}
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "send JSON")
		return
	}
	retitle := false
	if keys["title"] {
		if t := cleanText(in.Title, maxName); t != "" && t != g.Title {
			g.Title, retitle = t, true
		}
	}
	if keys["description"] {
		g.Description = cleanText(in.Description, 2000)
	}
	if keys["scores_on"] {
		g.ScoresOn = in.ScoresOn
	}
	if keys["remix_on"] {
		g.RemixOn = in.RemixOn
	}
	if keys["favorite"] {
		g.Favorite = in.Favorite
	}
	if keys["archived"] {
		g.Archived = in.Archived
	}
	if err := s.store.UpdateGame(ctx, g); err != nil {
		s.fail(w, err, "game")
		return
	}
	if retitle {
		_, _ = s.store.SaveGameDoc(ctx, uid, g.ID, gamedoc.SetTitle(g.Doc, g.Title), "", -1)
	}
	g, _ = s.store.GetGame(ctx, uid, g.ID)
	writeJSON(w, http.StatusOK, s.gameOut(g))
}

// handleSaveDoc stores what the editor made. "saves" is the save count the
// editor started from: when the game was saved elsewhere since, 409.
func (s *Server) handleSaveDoc(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Doc   json.RawMessage `json:"doc"`
		Saves *int            `json:"saves"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, gamedoc.MaxBytes+4096)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "the game is too big or not JSON")
		return
	}
	title, err := gamedoc.Check(in.Doc)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	saves := -1
	if in.Saves != nil {
		saves = *in.Saves
	}
	n, err := s.store.SaveGameDoc(ctx, uid, idParam(r, "id"), compactJSON(in.Doc), clip(title, maxName), saves)
	if err == store.ErrConflict {
		writeErr(w, http.StatusConflict, "this game was saved somewhere else (another tab?) since you opened it")
		return
	}
	if err != nil {
		s.fail(w, err, "save")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saves": n, "saved_at": store.Now()})
}

func (s *Server) handleDeleteGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	id := idParam(r, "id")
	if app := linkApp(ctx); app != "" {
		g, err := s.store.GetGame(ctx, uid, id)
		if err != nil || g.Source != app {
			writeErr(w, http.StatusForbidden, "this app may only remove games it made")
			return
		}
	}
	cover, err := s.store.DeleteGame(ctx, uid, id)
	if err != nil {
		s.fail(w, err, "delete")
		return
	}
	s.dropMedia(cover)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCopyGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, err := s.store.GetGame(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	title := clip(g.Title+" (copy)", maxName)
	ng := &store.Game{UserID: uid, Title: title, Description: g.Description, Template: g.Template, Doc: gamedoc.SetTitle(g.Doc, title), ScoresOn: g.ScoresOn}
	if err := s.store.CreateGame(ctx, ng); err != nil {
		s.fail(w, err, "copy")
		return
	}
	s.copyCover(ctx, uid, g, ng.ID)
	ng, _ = s.store.GetGame(ctx, uid, ng.ID)
	writeJSON(w, http.StatusCreated, s.gameOut(ng))
}

func (s *Server) copyCover(ctx context.Context, uid int64, from *store.Game, to int64) {
	if from.Cover == "" {
		return
	}
	p := s.mediaPath(from.Cover)
	if p == "" {
		return
	}
	if b, err := os.ReadFile(p); err == nil {
		if rel, err := s.saveBytes(uid, ".png", b); err == nil {
			_, _ = s.store.SetCover(ctx, uid, to, rel)
		}
	}
}

// handleCover is a game's picture: GET it, POST a new one (PNG from the
// editor's first scene), DELETE it.
func (s *Server) handleCover(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	id := idParam(r, "id")
	g, err := s.store.GetGame(ctx, uid, id)
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if g.Cover == "" {
			writeErr(w, http.StatusNotFound, "no picture yet")
			return
		}
		s.serveMediaFile(w, r, g.Cover, "", "")
	case http.MethodPost:
		b, err := s.readPicture(r)
		if err != nil {
			s.userFail(w, err, "picture")
			return
		}
		rel, err := s.saveBytes(uid, imageExt(b), b)
		if err != nil {
			s.fail(w, err, "picture")
			return
		}
		old, err := s.store.SetCover(ctx, uid, id, rel)
		if err != nil {
			s.dropMedia(rel)
			s.fail(w, err, "picture")
			return
		}
		s.dropMedia(old)
		writeJSON(w, http.StatusOK, map[string]any{"cover_url": fmt.Sprintf("/api/v1/games/%d/cover?t=%d", id, time.Now().Unix())})
	case http.MethodDelete:
		old, _ := s.store.SetCover(ctx, uid, id, "")
		s.dropMedia(old)
		w.WriteHeader(http.StatusNoContent)
	}
}

// ---- sharing ----

func (s *Server) handleShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	id := idParam(r, "id")
	var in struct {
		On bool `json:"on"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "send {on: true|false}")
		return
	}
	g, err := s.store.GetGame(ctx, uid, id)
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	if err := s.store.SetShare(ctx, uid, id, in.On, newToken()[:12], g.Doc); err != nil {
		s.fail(w, err, "share")
		return
	}
	if in.On {
		_, _ = s.store.AddVersion(ctx, uid, id, "Shared", g.Doc, 30)
	}
	g, _ = s.store.GetGame(ctx, uid, id)
	if in.On {
		s.publish(ctx, uid, "gamebuilder.game_shared", g.Title, map[string]any{"game_id": g.ID, "title": g.Title, "play_url": s.link("/p/" + g.ShareKey)})
	}
	writeJSON(w, http.StatusOK, s.gameOut(g))
}

func (s *Server) handleNewShareKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	id := idParam(r, "id")
	if err := s.store.NewShareKey(ctx, uid, id, newToken()[:12]); err != nil {
		s.fail(w, err, "share")
		return
	}
	g, _ := s.store.GetGame(ctx, uid, id)
	writeJSON(w, http.StatusOK, s.gameOut(g))
}

// ---- saved copies ----

func (s *Server) handleVersions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	id := idParam(r, "id")
	if r.Method == http.MethodPost {
		var in struct {
			Note string `json:"note"`
		}
		_ = decode(r, &in)
		g, err := s.store.GetGame(ctx, uid, id)
		if err != nil {
			s.fail(w, err, "game")
			return
		}
		v, err := s.store.AddVersion(ctx, uid, id, firstNonEmpty(cleanText(in.Note, maxLine), "Saved by hand"), g.Doc, 30)
		if err != nil {
			s.fail(w, err, "save a copy")
			return
		}
		writeJSON(w, http.StatusCreated, v)
		return
	}
	if _, err := s.store.GetGame(ctx, uid, id); err != nil {
		s.fail(w, err, "game")
		return
	}
	list, err := s.store.ListVersions(ctx, uid, id)
	if err != nil {
		s.fail(w, err, "versions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": list})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	vid := idParam(r, "id")
	switch r.Method {
	case http.MethodDelete:
		if err := s.store.DeleteVersion(ctx, uid, vid); err != nil {
			s.fail(w, err, "version")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	v, err := s.store.GetVersion(ctx, uid, vid)
	if err != nil {
		s.fail(w, err, "version")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// handleRestoreVersion puts a saved copy back (keeping what was there as
// another copy first).
func (s *Server) handleRestoreVersion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	v, err := s.store.GetVersion(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "version")
		return
	}
	g, err := s.store.GetGame(ctx, uid, v.GameID)
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	_, _ = s.store.AddVersion(ctx, uid, g.ID, "Before going back", g.Doc, 30)
	title, _ := gamedoc.Check(v.Doc)
	n, err := s.store.SaveGameDoc(ctx, uid, g.ID, v.Doc, clip(title, maxName), -1)
	if err != nil {
		s.fail(w, err, "restore")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saves": n})
}

// ---- plays and best scores ----

func lowIsBest(doc []byte) bool {
	var d struct {
		Score gamedoc.Score `json:"score"`
	}
	_ = json.Unmarshal(doc, &d)
	return d.Score.Kind == "low" || d.Score.Kind == "time"
}

func (s *Server) handlePlays(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	id := idParam(r, "id")
	g, err := s.store.GetGame(ctx, uid, id)
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	if r.Method == http.MethodPost {
		p, err := readPlay(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		p.UserID, p.GameID, p.Mine = uid, id, true
		if err := s.store.AddPlay(ctx, p); err != nil {
			s.fail(w, err, "play")
			return
		}
		writeJSON(w, http.StatusCreated, p)
		return
	}
	plays, err := s.store.ListPlays(ctx, uid, store.PlayFilter{GameID: id, Limit: firstPos(queryInt(r, "limit"), 100)})
	if err != nil {
		s.fail(w, err, "plays")
		return
	}
	best, _ := s.store.BestPlays(ctx, uid, id, lowIsBest(g.Doc), 10)
	since := time.Now().AddDate(0, 0, -13).Format("2006-01-02")
	days, _ := s.store.GamePlaysPerDay(ctx, uid, id, since)
	perDay := []map[string]any{}
	for i := 13; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		perDay = append(perDay, map[string]any{"day": d, "plays": days[d]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"plays": plays, "best": best, "low_is_best": lowIsBest(g.Doc), "days": perDay})
}

type playIn struct {
	Who     string  `json:"who"`
	Score   float64 `json:"score"`
	Seconds float64 `json:"seconds"`
	Result  string  `json:"result"`
	Level   string  `json:"level"`
}

func readPlay(r *http.Request) (*store.Play, error) {
	var in playIn
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in); err != nil {
		return nil, fmt.Errorf("send the play as JSON")
	}
	if !oneOf(in.Result, []string{"won", "lost", "quit"}) {
		in.Result = "quit"
	}
	if in.Score != in.Score || in.Score > 1e9 || in.Score < -1e9 || in.Seconds < 0 || in.Seconds > 86400 {
		return nil, fmt.Errorf("that score does not look right")
	}
	return &store.Play{Who: cleanText(in.Who, 24), Score: in.Score, Seconds: in.Seconds, Result: in.Result, Level: cleanText(in.Level, 60)}, nil
}

func (s *Server) handleDeletePlay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeletePlay(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "play")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleClearScores(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	id := idParam(r, "id")
	if _, err := s.store.GetGame(ctx, uid, id); err != nil {
		s.fail(w, err, "game")
		return
	}
	if err := s.store.ClearScores(ctx, uid, id); err != nil {
		s.fail(w, err, "scores")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- to-dos ----

func (s *Server) handleTodos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	id := idParam(r, "id")
	if _, err := s.store.GetGame(ctx, uid, id); err != nil {
		s.fail(w, err, "game")
		return
	}
	if r.Method == http.MethodPost {
		var in struct {
			Text string `json:"text"`
		}
		if err := decode(r, &in); err != nil || strings.TrimSpace(in.Text) == "" {
			writeErr(w, http.StatusBadRequest, "write what is to do")
			return
		}
		t := &store.Todo{UserID: uid, GameID: id, Text: cleanText(in.Text, maxLine), Source: sourceOf(ctx)}
		if err := s.store.CreateTodo(ctx, t); err != nil {
			s.fail(w, err, "to-do")
			return
		}
		writeJSON(w, http.StatusCreated, t)
		return
	}
	list, err := s.store.ListTodos(ctx, uid, id)
	if err != nil {
		s.fail(w, err, "to-dos")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"todos": list})
}

func (s *Server) handleTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	t, err := s.store.GetTodo(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "to-do")
		return
	}
	if !mayChange(ctx, t.Source) {
		writeErr(w, http.StatusForbidden, "this app may only change to-dos it added")
		return
	}
	if r.Method == http.MethodDelete {
		if err := s.store.DeleteTodo(ctx, uid, t.ID); err != nil {
			s.fail(w, err, "to-do")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var in struct {
		Text string `json:"text"`
		Done bool   `json:"done"`
	}
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "send JSON")
		return
	}
	if keys["text"] && strings.TrimSpace(in.Text) != "" {
		t.Text = cleanText(in.Text, maxLine)
	}
	if keys["done"] {
		t.Done = in.Done
	}
	if err := s.store.UpdateTodo(ctx, t); err != nil {
		s.fail(w, err, "to-do")
		return
	}
	writeJSON(w, http.StatusOK, t)
}

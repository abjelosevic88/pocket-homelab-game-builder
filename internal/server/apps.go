package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/gamedoc"
	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/store"
)

// The other Pocket apps. None of it is needed: when an app is not there,
// the page greys the button and says what to do.

func linkTo(sib *siblingConn, p string) string {
	if sib == nil || sib.public == "" {
		return ""
	}
	return strings.TrimRight(sib.public, "/") + p
}

func plainErr(a pocketApp, err error) string {
	if isNotConnected(err) {
		return err.Error()
	}
	if ue, ok := err.(*userErr); ok {
		return ue.msg
	}
	return a.name + ": " + err.Error()
}

func (s *Server) appFail(w http.ResponseWriter, a pocketApp, err error) {
	if isNotConnected(err) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if ue, ok := err.(*userErr); ok {
		writeErr(w, http.StatusBadRequest, ue.msg)
		return
	}
	writeErr(w, http.StatusBadGateway, plainErr(a, err))
}

// handleApps lists how the person reaches each app, in words they can act on.
func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c := s.clients(ctx, actorID(ctx))
	out := []appState{}
	for _, a := range pocketApps {
		out = append(out, c.status(ctx, a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": out, "core": s.core.Configured()})
}

func (s *Server) sib(w http.ResponseWriter, r *http.Request, a pocketApp, scopes ...string) (*siblingConn, bool) {
	ctx := r.Context()
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, a, scopes...)
	if err != nil {
		s.appFail(w, a, err)
		return nil, false
	}
	return sib, true
}

func (s *Server) sibFor(ctx context.Context, uid int64, a pocketApp, scopes ...string) (*siblingConn, error) {
	return s.clients(ctx, uid).conn(ctx, a, scopes...)
}

func ifs(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}

// proxyBytes passes a picture from another app through (thumbnails in pickers).
func (s *Server) proxyBytes(w http.ResponseWriter, r *http.Request, a pocketApp, scope, p string, q url.Values) {
	sib, ok := s.sib(w, r, a, scope)
	if !ok {
		return
	}
	b, ct, err := sib.client.Bytes(r.Context(), p, q, 16<<20)
	if err != nil {
		s.appFail(w, a, err)
		return
	}
	if ct == "" || strings.HasPrefix(ct, "text/plain") {
		ct = http.DetectContentType(b)
		if looksSVG(b) {
			ct = "image/svg+xml"
		}
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(b)
}

type importIn struct {
	IDs    []string `json:"ids"`
	Space  string   `json:"space"`
	Paths  []string `json:"paths"`
	GameID int64    `json:"game_id"`
	Kind   string   `json:"kind"`
}

func readImport(r *http.Request) (importIn, bool) {
	var in importIn
	if err := decode(r, &in); err != nil || len(in.IDs)+len(in.Paths) == 0 || len(in.IDs)+len(in.Paths) > 40 {
		return in, false
	}
	return in, true
}

func (s *Server) importBytes(w http.ResponseWriter, r *http.Request, a pocketApp, scope string, in importIn, fetch func(sib *siblingConn, id string) ([]byte, string, string, error), kind, origin string) {
	sib, ok := s.sib(w, r, a, scope)
	if !ok {
		return
	}
	uid := actorID(r.Context())
	out := []map[string]any{}
	var errs []string
	ids := append(append([]string{}, in.IDs...), in.Paths...)
	for _, id := range ids {
		b, name, k, err := fetch(sib, id)
		if err != nil {
			errs = append(errs, plainErr(a, err))
			continue
		}
		if k == "" {
			k = kind
		}
		as, err := s.addAsset(r, uid, k, name, b, origin, id, in.GameID)
		if err != nil {
			errs = append(errs, name+": "+plainMsg(err))
			continue
		}
		out = append(out, assetOut(*as))
	}
	if len(out) == 0 && len(errs) > 0 {
		writeErr(w, http.StatusBadRequest, strings.Join(errs, "; "))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"assets": out, "errors": errs})
}

// ---- Pocket Photos ----

func (s *Server) handlePhotosList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, ok := s.sib(w, r, appPhotos, scopePhotosRead)
	if !ok {
		return
	}
	body := map[string]any{"type": "IMAGE", "size": 60, "order": "desc"}
	p := "/api/v1/immich/search/metadata"
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		body = map[string]any{"type": "IMAGE", "size": 60, "query": q}
		p = "/api/v1/immich/search/smart"
	}
	if al := r.URL.Query().Get("album"); al != "" && reUUID.MatchString(al) {
		body["albumIds"] = []string{al}
	}
	var res map[string]any
	if err := sib.client.Do(ctx, http.MethodPost, p, nil, body, &res); err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	out := []map[string]any{}
	for _, it := range rows(obj(res, "assets"), "items") {
		id := str(it, "id")
		if !reUUID.MatchString(id) {
			continue
		}
		out = append(out, map[string]any{"id": id, "name": str(it, "originalFileName"), "date": day10(it["fileCreatedAt"]), "thumb": "/api/v1/apps/photos/" + id + "/thumb"})
	}
	albums := []map[string]any{}
	if v, err := getJSONFrom(ctx, sib, "/api/v1/immich/albums", nil); err == nil {
		for _, al := range rows(v) {
			albums = append(albums, map[string]any{"id": str(al, "id"), "name": str(al, "albumName"), "count": num(al, "assetCount")})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"photos": out, "albums": albums, "open_url": linkTo(sib, "/")})
}

func (s *Server) handlePhotoThumb(w http.ResponseWriter, r *http.Request) {
	id := chiParam(r, "id")
	if !reUUID.MatchString(id) {
		writeErr(w, http.StatusBadRequest, "bad id")
		return
	}
	s.proxyBytes(w, r, appPhotos, scopePhotosRead, "/api/v1/immich/assets/"+id+"/thumbnail", url.Values{"size": {"thumbnail"}})
}

func (s *Server) handlePhotosImport(w http.ResponseWriter, r *http.Request) {
	in, ok := readImport(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "pick one or more photos")
		return
	}
	s.importBytes(w, r, appPhotos, scopePhotosRead, in, func(sib *siblingConn, id string) ([]byte, string, string, error) {
		if !reUUID.MatchString(id) {
			return nil, "", "", &userErr{"bad photo id"}
		}
		b, _, err := sib.client.Bytes(r.Context(), "/api/v1/immich/assets/"+id+"/thumbnail", url.Values{"size": {"preview"}}, 16<<20)
		return b, "Photo", "image", err
	}, "image", "photos")
}

// ---- Pocket Files ----

func fileKind(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg":
		return "image"
	case ".mp3", ".wav", ".ogg", ".oga", ".opus", ".m4a", ".aac", ".flac", ".webm":
		return "sound"
	case ".json":
		if strings.HasSuffix(strings.ToLower(name), ".game.json") {
			return "game"
		}
	}
	return ""
}

func (s *Server) handleFilesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, ok := s.sib(w, r, appFiles, scopeFilesRead)
	if !ok {
		return
	}
	space := firstNonEmpty(r.URL.Query().Get("space"), "home")
	p := firstNonEmpty(r.URL.Query().Get("path"), "/")
	v, err := getJSONFrom(ctx, sib, "/api/v1/fs/list", url.Values{"space": {space}, "path": {p}})
	if err != nil {
		s.appFail(w, appFiles, err)
		return
	}
	out := []map[string]any{}
	for _, e := range rows(v, "entries") {
		name := str(e, "name")
		if strings.HasPrefix(name, ".") {
			continue
		}
		kind := fileKind(name)
		if e["is_dir"] == true {
			kind = "folder"
		}
		if kind == "" {
			continue
		}
		out = append(out, map[string]any{"name": name, "path": str(e, "path"), "kind": kind, "size": num(e, "size"), "modified": str(e, "modified")})
	}
	spaces := []map[string]any{}
	if sv, err := getJSONFrom(ctx, sib, "/api/v1/spaces", nil); err == nil {
		for _, sp := range rows(sv, "spaces") {
			spaces = append(spaces, map[string]any{"id": str(sp, "id"), "name": str(sp, "name")})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out, "space": space, "path": p, "spaces": spaces})
}

func (s *Server) handleFilesImport(w http.ResponseWriter, r *http.Request) {
	in, ok := readImport(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "pick one or more files")
		return
	}
	space := firstNonEmpty(in.Space, "home")
	s.importBytes(w, r, appFiles, scopeFilesRead, in, func(sib *siblingConn, p string) ([]byte, string, string, error) {
		k := fileKind(p)
		if k == "" || k == "game" {
			return nil, path.Base(p), "", &userErr{path.Base(p) + " is not a picture or a sound"}
		}
		if k == "sound" && in.Kind == "music" {
			k = "music"
		}
		b, _, err := sib.client.Bytes(r.Context(), "/api/v1/fs/raw", url.Values{"space": {space}, "path": {p}}, s.cfg.MaxUploadBytes)
		return b, path.Base(p), k, err
	}, "image", "files")
}

// handleGameToFiles saves a game in Pocket Files as one HTML file.
func (s *Server) handleGameToFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, err := s.store.GetGame(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	sib, ok := s.sib(w, r, appFiles, scopeFilesWrite)
	if !ok {
		return
	}
	b, err := s.standalone(ctx, uid, g)
	if err != nil {
		s.userFail(w, err, "export")
		return
	}
	st := s.settingsOf(ctx, uid)
	dir := "/" + strings.Trim(firstNonEmpty(st.FilesFolder, "Games"), "/")
	name := strings.NewReplacer("/", "-", "\\", "-").Replace(g.Title) + ".html"
	var res map[string]any
	q := url.Values{"space": {"home"}, "path": {dir}, "name": {name}, "conflict": {"overwrite"}}
	if err := sib.client.PutRaw(ctx, "/api/v1/fs/upload", q, b, "text/html", &res); err != nil {
		s.appFail(w, appFiles, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": dir + "/" + name, "open_url": linkTo(sib, "/")})
}

// ---- Pocket Draw ----

func (s *Server) handleDrawList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, ok := s.sib(w, r, appDraw, scopeDrawRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/drawings", url.Values{"q": {r.URL.Query().Get("q")}, "limit": {"60"}})
	if err != nil {
		s.appFail(w, appDraw, err)
		return
	}
	out := []map[string]any{}
	for _, d := range rows(v, "drawings") {
		id := idOf(d)
		out = append(out, map[string]any{"id": id, "name": firstNonEmpty(str(d, "title"), "Drawing"), "date": day10(d["edited_at"]), "thumb": "/api/v1/apps/draw/" + id + "/preview"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"drawings": out, "open_url": linkTo(sib, "/")})
}

func (s *Server) handleDrawPreview(w http.ResponseWriter, r *http.Request) {
	s.proxyBytes(w, r, appDraw, scopeDrawRead, fmt.Sprintf("/api/v1/drawings/%d/preview.svg", idParam(r, "id")), nil)
}

func (s *Server) handleDrawImport(w http.ResponseWriter, r *http.Request) {
	in, ok := readImport(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "pick one or more drawings")
		return
	}
	s.importBytes(w, r, appDraw, scopeDrawRead, in, func(sib *siblingConn, id string) ([]byte, string, string, error) {
		name := "Drawing"
		if v, err := getJSONFrom(r.Context(), sib, "/api/v1/drawings/"+url.PathEscape(id), nil); err == nil {
			if m, ok := v.(map[string]any); ok {
				name = firstNonEmpty(str(m, "title"), str(obj(m, "drawing"), "title"), name)
			}
		}
		b, _, err := sib.client.Bytes(r.Context(), "/api/v1/drawings/"+url.PathEscape(id)+"/preview.svg", nil, 16<<20)
		return b, name, "image", err
	}, "image", "draw")
}

// ---- Pocket Photo Lab ----

func (s *Server) handlePhotoLabList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, ok := s.sib(w, r, appPhotoLab, scopePhotoLabRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/images", url.Values{"q": {r.URL.Query().Get("q")}, "limit": {"60"}})
	if err != nil {
		s.appFail(w, appPhotoLab, err)
		return
	}
	out := []map[string]any{}
	for _, im := range rows(v, "items") {
		id := idOf(im)
		out = append(out, map[string]any{"id": id, "name": firstNonEmpty(str(im, "title"), "Picture"), "edited": im["edited"] == true, "thumb": "/api/v1/apps/photolab/" + id + "/thumb"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"images": out, "open_url": linkTo(sib, "/")})
}

func (s *Server) handlePhotoLabThumb(w http.ResponseWriter, r *http.Request) {
	s.proxyBytes(w, r, appPhotoLab, scopePhotoLabRead, fmt.Sprintf("/api/v1/images/%d/thumb", idParam(r, "id")), nil)
}

func (s *Server) handlePhotoLabImport(w http.ResponseWriter, r *http.Request) {
	in, ok := readImport(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "pick one or more pictures")
		return
	}
	s.importBytes(w, r, appPhotoLab, scopePhotoLabRead, in, func(sib *siblingConn, id string) ([]byte, string, string, error) {
		name := "Picture"
		if v, err := getJSONFrom(r.Context(), sib, "/api/v1/images/"+url.PathEscape(id), nil); err == nil {
			if m, ok := v.(map[string]any); ok {
				name = firstNonEmpty(str(m, "title"), name)
			}
		}
		b, _, err := sib.client.Bytes(r.Context(), "/api/v1/images/"+url.PathEscape(id)+"/preview", nil, 16<<20)
		return b, name, "image", err
	}, "image", "photolab")
}

// ---- Pocket Music ----

func (s *Server) handleMusicSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, ok := s.sib(w, r, appMusic, scopeMusicPlay)
	if !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"songs": []any{}})
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/search", url.Values{"q": {q}, "limit": {"25"}})
	if err != nil {
		s.appFail(w, appMusic, err)
		return
	}
	out := []map[string]any{}
	if m, ok := v.(map[string]any); ok {
		for _, so := range rows(m["songs"]) {
			out = append(out, map[string]any{"id": idOf(so), "title": str(so, "title"), "artist": str(so, "artist"), "album": str(so, "album"), "seconds": num(so, "duration")})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"songs": out})
}

func (s *Server) handleMusicImport(w http.ResponseWriter, r *http.Request) {
	in, ok := readImport(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "pick a song")
		return
	}
	s.importBytes(w, r, appMusic, scopeMusicPlay, in, func(sib *siblingConn, id string) ([]byte, string, string, error) {
		name := "Song"
		if v, err := getJSONFrom(r.Context(), sib, "/api/v1/songs/"+url.PathEscape(id), nil); err == nil {
			if m, ok := v.(map[string]any); ok {
				name = joinNonEmpty(" — ", firstNonEmpty(str(m, "title"), str(obj(m, "song"), "title")), str(m, "artist"))
			}
		}
		b, _, err := sib.client.Bytes(r.Context(), "/api/v1/stream/"+url.PathEscape(id), url.Values{"format": {"mp3"}, "q": {"128"}}, s.cfg.MaxUploadBytes)
		return b, firstNonEmpty(name, "Song") + ".mp3", "music", err
	}, "music", "music")
}

// ---- Pocket Voice ----

func (s *Server) handleVoiceList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, ok := s.sib(w, r, appVoice, scopeVoiceRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/memos", url.Values{"q": {r.URL.Query().Get("q")}, "limit": {"60"}})
	if err != nil {
		s.appFail(w, appVoice, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "items") {
		if m["has_audio"] != true {
			continue
		}
		out = append(out, map[string]any{"id": idOf(m), "name": firstNonEmpty(str(m, "title"), "Recording"), "date": day10(m["recorded_at"]), "seconds": num(m, "duration_ms") / 1000})
	}
	writeJSON(w, http.StatusOK, map[string]any{"memos": out, "open_url": linkTo(sib, "/")})
}

func (s *Server) handleVoiceImport(w http.ResponseWriter, r *http.Request) {
	in, ok := readImport(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "pick a recording")
		return
	}
	s.importBytes(w, r, appVoice, scopeVoiceRead, in, func(sib *siblingConn, id string) ([]byte, string, string, error) {
		name := "Recording"
		ext := ".m4a"
		if v, err := getJSONFrom(r.Context(), sib, "/api/v1/memos/"+url.PathEscape(id), nil); err == nil {
			if m, ok := v.(map[string]any); ok {
				name = firstNonEmpty(str(m, "title"), name)
				if t := str(m, "audio_type"); t != "" {
					ext = "." + strings.TrimPrefix(strings.ToLower(t), ".")
				}
			}
		}
		b, _, err := sib.client.Bytes(r.Context(), "/api/v1/memos/"+url.PathEscape(id)+"/audio", nil, s.cfg.MaxUploadBytes)
		return b, name + ext, "sound", err
	}, "sound", "voice")
}

// ---- Pocket Puzzlecraft: a maze becomes a maze game ----

func (s *Server) handleMazeList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, ok := s.sib(w, r, appPuzzles, scopePuzzlesRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/puzzles", url.Values{"kind": {"maze"}, "limit": {"60"}})
	if err != nil {
		s.appFail(w, appPuzzles, err)
		return
	}
	out := []map[string]any{}
	for _, p := range rows(v, "puzzles") {
		out = append(out, map[string]any{"id": idOf(p), "name": firstNonEmpty(str(p, "title"), "Maze"), "size": str(p, "size", "difficulty")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"mazes": out, "open_url": linkTo(sib, "/")})
}

func (s *Server) handleMazeGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		ID string `json:"id"`
	}
	if err := decode(r, &in); err != nil || in.ID == "" {
		writeErr(w, http.StatusBadRequest, "pick a maze")
		return
	}
	sib, ok := s.sib(w, r, appPuzzles, scopePuzzlesRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/puzzles/"+url.PathEscape(in.ID), nil)
	if err != nil {
		s.appFail(w, appPuzzles, err)
		return
	}
	m, _ := v.(map[string]any)
	if pz := obj(m, "puzzle"); len(pz) > 0 {
		m = pz
	}
	data := obj(m, "data")
	var mz struct {
		W     int    `json:"w"`
		H     int    `json:"h"`
		Walls []int  `json:"walls"`
		Start [2]int `json:"start"`
		End   [2]int `json:"end"`
	}
	if err := remarshal(data, &mz); err != nil || mz.W == 0 || len(mz.Walls) < mz.W*mz.H || mz.W > 40 || mz.H > 40 {
		writeErr(w, http.StatusBadRequest, "that maze has no grid this app can read")
		return
	}
	if mz.End == [2]int{0, 0} {
		mz.End = [2]int{mz.W - 1, mz.H - 1}
	}
	level := gamedoc.MazeLevel(mz.W, mz.H, mz.Walls, mz.Start, mz.End, rand.New(rand.NewSource(time.Now().UnixNano())))
	title := firstNonEmpty(str(m, "title"), "Maze")
	g, err := s.makeGame(r, actorID(ctx), newGame{Template: "maze", Title: title, Options: gamedoc.Options{Levels: [][]string{level}}})
	if err != nil {
		s.userFail(w, err, "maze game")
		return
	}
	writeJSON(w, http.StatusCreated, s.gameOut(g))
}

// ---- Pocket Translate: a copy in another language ----

func (s *Server) handleTranslateGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		To   string `json:"to"`
		Name string `json:"name"` // the language's name, for the title
	}
	if err := decode(r, &in); err != nil || in.To == "" || len(in.To) > 12 {
		writeErr(w, http.StatusBadRequest, "choose a language")
		return
	}
	g, err := s.store.GetGame(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	sib, ok := s.sib(w, r, appTranslate, scopeTranslate)
	if !ok {
		return
	}
	texts := gamedoc.Texts(g.Doc)
	if len(texts) == 0 {
		writeErr(w, http.StatusBadRequest, "this game has no words to translate")
		return
	}
	if len(texts) > 200 {
		texts = texts[:200]
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/translate", map[string]any{"texts": texts, "to": in.To, "from": "auto", "about": "Words on the screen of a small video game: short, friendly."})
	if err != nil {
		s.appFail(w, appTranslate, err)
		return
	}
	var out []string
	_ = remarshal(res["texts"], &out)
	if len(out) == 0 {
		writeErr(w, http.StatusBadGateway, "Pocket Translate gave nothing back")
		return
	}
	doc, err := gamedoc.Replace(g.Doc, out)
	if err != nil {
		s.fail(w, err, "translate")
		return
	}
	title, _ := gamedoc.Check(doc)
	title = clip(firstNonEmpty(title, g.Title)+" ("+firstNonEmpty(in.Name, in.To)+")", maxName)
	ng := &store.Game{UserID: uid, Title: title, Description: g.Description, Template: g.Template, Doc: gamedoc.SetTitle(doc, title), ScoresOn: g.ScoresOn}
	if err := s.store.CreateGame(ctx, ng); err != nil {
		s.fail(w, err, "translate")
		return
	}
	s.copyCover(ctx, uid, g, ng.ID)
	ng, _ = s.store.GetGame(ctx, uid, ng.ID)
	writeJSON(w, http.StatusCreated, s.gameOut(ng))
}

// ---- telling people about a game ----

// shared makes sure a game has its play link on and returns it.
func (s *Server) shared(ctx context.Context, uid int64, g *store.Game) (string, error) {
	if !g.ShareOn || g.ShareKey == "" {
		if err := s.store.SetShare(ctx, uid, g.ID, true, newToken()[:12], g.Doc); err != nil {
			return "", err
		}
		ng, err := s.store.GetGame(ctx, uid, g.ID)
		if err != nil {
			return "", err
		}
		*g = *ng
	}
	return s.link("/p/" + g.ShareKey), nil
}

func (s *Server) gameFor(w http.ResponseWriter, r *http.Request) (*store.Game, bool) {
	ctx := r.Context()
	g, err := s.store.GetGame(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "game")
		return nil, false
	}
	return g, true
}

func gameLine(g *store.Game) string {
	return firstNonEmpty(g.Description, fmt.Sprintf("A %s game with %d %s.", strings.ToLower(templateName(g.Template)), g.Scenes, plural(g.Scenes, "level", "levels")))
}

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appChat, scopeChatRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/conversations", nil)
	if err != nil {
		s.appFail(w, appChat, err)
		return
	}
	out := []map[string]any{}
	for _, c := range rows(v, "conversations") {
		out = append(out, map[string]any{"id": idOf(c), "name": firstNonEmpty(str(c, "title", "name"), ifs(str(c, "kind") == "self", "Notes to self", "Chat")), "kind": str(c, "kind")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"chats": out})
}

func (s *Server) handleGameChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, ok := s.gameFor(w, r)
	if !ok {
		return
	}
	var in struct {
		To   string `json:"to"`
		Text string `json:"text"`
	}
	_ = decode(r, &in)
	to := firstNonEmpty(in.To, s.settingsOf(ctx, uid).ChatTo, "me")
	sib, ok := s.sib(w, r, appChat, scopeChatWrite)
	if !ok {
		return
	}
	play, err := s.shared(ctx, uid, g)
	if err != nil {
		s.fail(w, err, "share")
		return
	}
	card := map[string]any{"title": g.Title, "subtitle": "A game to play", "text": clip(gameLine(g), 300), "url": play, "app": "gamebuilder", "kind": "game", "ref": fmt.Sprint(g.ID), "icon": "🎮"}
	if g.HasCover {
		card["image"] = s.link("/api/v1/public/" + g.ShareKey + "/cover.png")
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/send", map[string]any{"to": to, "text": firstNonEmpty(cleanText(in.Text, 2000), "Play my game: "+play), "card": card})
	if err != nil {
		s.appFail(w, appChat, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "play_url": play, "open_url": linkTo(sib, "/c/"+str(res, "conversation_id"))})
}

func (s *Server) handleGameBlog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, ok := s.gameFor(w, r)
	if !ok {
		return
	}
	var in struct {
		Text string `json:"text"`
	}
	_ = decode(r, &in)
	sib, ok := s.sib(w, r, appBlog, scopeBlogWrite)
	if !ok {
		return
	}
	play, err := s.shared(ctx, uid, g)
	if err != nil {
		s.fail(w, err, "share")
		return
	}
	var b strings.Builder
	if t := strings.TrimSpace(in.Text); t != "" {
		b.WriteString(t + "\n\n")
	} else {
		b.WriteString(gameLine(g) + "\n\n")
	}
	if intro := docIntro(g.Doc); intro != "" {
		b.WriteString("**How to play:** " + intro + "\n\n")
	}
	b.WriteString("[▶ Play " + g.Title + "](" + play + ")\n\n")
	b.WriteString("_Made with Pocket Game Builder._\n")
	body := map[string]any{"title": g.Title, "body": b.String(), "excerpt": clip(gameLine(g), 200), "tags": []string{"games"}, "source_ref": fmt.Sprintf("gamebuilder:%d", g.ID)}
	if g.HasCover {
		body["cover"] = s.link("/api/v1/public/" + g.ShareKey + "/cover.png")
		body["cover_alt"] = g.Title
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/posts", body)
	if err != nil {
		s.appFail(w, appBlog, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "play_url": play, "open_url": firstNonEmpty(str(res, "edit_url"), linkTo(sib, "/posts/"+idOf(res)))})
}

func docIntro(doc []byte) string {
	var d struct {
		Intro gamedoc.Intro `json:"intro"`
	}
	_ = json.Unmarshal(doc, &d)
	return strings.TrimSpace(d.Intro.Text)
}

func (s *Server) handleForums(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, ok := s.sib(w, r, appForum, scopeForumRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/forums", nil)
	if err != nil {
		s.appFail(w, appForum, err)
		return
	}
	out := []map[string]any{}
	for _, f := range rows(v, "forums") {
		slug := str(f, "slug")
		cats := []map[string]any{}
		if cv, err := getJSONFrom(ctx, sib, "/api/v1/forums/"+url.PathEscape(slug)+"/categories", nil); err == nil {
			for _, c := range rows(cv, "categories") {
				cats = append(cats, map[string]any{"id": idOf(c), "name": str(c, "name")})
			}
		}
		out = append(out, map[string]any{"slug": slug, "name": firstNonEmpty(str(f, "name"), slug), "emoji": str(f, "emoji"), "categories": cats})
	}
	writeJSON(w, http.StatusOK, map[string]any{"forums": out})
}

func (s *Server) handleGameForum(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, ok := s.gameFor(w, r)
	if !ok {
		return
	}
	var in struct {
		Forum    string `json:"forum"`
		Category any    `json:"category_id"`
		Title    string `json:"title"`
		Text     string `json:"text"`
	}
	if err := decode(r, &in); err != nil || in.Forum == "" {
		writeErr(w, http.StatusBadRequest, "choose a forum")
		return
	}
	sib, ok := s.sib(w, r, appForum, scopeForumWrite)
	if !ok {
		return
	}
	play, err := s.shared(ctx, uid, g)
	if err != nil {
		s.fail(w, err, "share")
		return
	}
	body := firstNonEmpty(cleanText(in.Text, 8000), "I made a game: "+gameLine(g)+" What do you think? Too easy, too hard?") + "\n\n[▶ Play " + g.Title + "](" + play + ")"
	req := map[string]any{"forum": in.Forum, "title": firstNonEmpty(cleanText(in.Title, 200), "My game: "+g.Title), "body": body, "tags": []string{"games"}, "source_ref": fmt.Sprintf("gamebuilder:%d", g.ID)}
	if in.Category != nil && fmt.Sprint(in.Category) != "" && fmt.Sprint(in.Category) != "0" {
		req["category_id"] = in.Category
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/topics", req)
	if err != nil {
		s.appFail(w, appForum, err)
		return
	}
	open := str(res, "url")
	if open != "" && strings.HasPrefix(open, "/") {
		open = linkTo(sib, open)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "play_url": play, "open_url": open})
}

func (s *Server) handleContacts(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appContacts, scopeContactsRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/suggest", url.Values{"q": {r.URL.Query().Get("q")}, "limit": {"12"}})
	if err != nil {
		s.appFail(w, appContacts, err)
		return
	}
	out := []map[string]any{}
	for _, c := range rows(v, "contacts") {
		if e := str(c, "email"); e != "" {
			out = append(out, map[string]any{"name": str(c, "name"), "email": e})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"contacts": out})
}

func (s *Server) handleGameEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, ok := s.gameFor(w, r)
	if !ok {
		return
	}
	var in struct {
		To []struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"to"`
		Text string `json:"text"`
	}
	if err := decode(r, &in); err != nil || len(in.To) == 0 || len(in.To) > 20 {
		writeErr(w, http.StatusBadRequest, "who should get it?")
		return
	}
	to := []map[string]string{}
	for _, t := range in.To {
		if e := strings.TrimSpace(t.Email); strings.Contains(e, "@") {
			to = append(to, map[string]string{"name": cleanText(t.Name, maxName), "addr": e})
		}
	}
	if len(to) == 0 {
		writeErr(w, http.StatusBadRequest, "add an email address")
		return
	}
	sib, ok := s.sib(w, r, appEmail, scopeMailSend)
	if !ok {
		return
	}
	play, err := s.shared(ctx, uid, g)
	if err != nil {
		s.fail(w, err, "share")
		return
	}
	who := s.makerName(ctx, uid)
	msg := firstNonEmpty(cleanText(in.Text, 4000), "I made a game and I'd like you to play it.")
	html := "<p>" + htmlEsc(msg) + "</p><p><b>" + htmlEsc(g.Title) + "</b><br>" + htmlEsc(gameLine(g)) + "</p><p><a href=\"" + htmlEsc(play) + "\">▶ Play it</a> — in the browser, on a phone or a computer, no sign-up.</p><p>— " + htmlEsc(who) + "</p>"
	if _, err := postJSONTo(ctx, sib, "/api/v1/send", map[string]any{"to": to, "subject": "Play my game: " + g.Title, "body_html": html, "body_text": msg + "\n\n" + g.Title + "\n" + play + "\n\n— " + who}); err != nil {
		s.appFail(w, appEmail, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "play_url": play})
}

func (s *Server) handleGameFamily(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, ok := s.gameFor(w, r)
	if !ok {
		return
	}
	var in struct {
		Text string `json:"text"`
		Chat bool   `json:"chat"`
	}
	_ = decode(r, &in)
	sib, ok := s.sib(w, r, appFamily, scopeFamilyWrite)
	if !ok {
		return
	}
	play, err := s.shared(ctx, uid, g)
	if err != nil {
		s.fail(w, err, "share")
		return
	}
	body := clip(firstNonEmpty(cleanText(in.Text, 1500), "🎮 New game to play: "+g.Title+"!")+"\n"+play, 2000)
	if _, err := postJSONTo(ctx, sib, "/api/v1/posts", map[string]any{"body": body, "chat": in.Chat, "until": time.Now().AddDate(0, 0, 14).Format("2006-01-02")}); err != nil {
		s.appFail(w, appFamily, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "play_url": play, "open_url": linkTo(sib, "/")})
}

func (s *Server) handleCalendars(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appCalendar, scopeCalendarRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/calendars", nil)
	if err != nil {
		s.appFail(w, appCalendar, err)
		return
	}
	out := []map[string]any{}
	for _, c := range rows(v, "calendars") {
		out = append(out, map[string]any{"id": idOf(c), "name": str(c, "name"), "color": str(c, "color")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"calendars": out})
}

// handleGamePlaytest puts a game night (or a play-test) in Pocket Calendar.
func (s *Server) handleGamePlaytest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, ok := s.gameFor(w, r)
	if !ok {
		return
	}
	var in struct {
		Start      string `json:"start"` // local "2006-01-02T15:04"
		Minutes    int    `json:"minutes"`
		CalendarID any    `json:"calendar_id"`
		Title      string `json:"title"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "send JSON")
		return
	}
	start, err := time.ParseInLocation("2006-01-02T15:04", in.Start, time.Local)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "choose a day and a time")
		return
	}
	mins := clampInt(firstPos(in.Minutes, 60), 10, 600)
	sib, ok := s.sib(w, r, appCalendar, scopeCalendarWrite)
	if !ok {
		return
	}
	play, err := s.shared(ctx, uid, g)
	if err != nil {
		s.fail(w, err, "share")
		return
	}
	body := map[string]any{"title": firstNonEmpty(cleanText(in.Title, 200), "Play-test: "+g.Title), "description": gameLine(g) + "\n\nPlay: " + play, "url": play,
		"start": start.Format(time.RFC3339), "end": start.Add(time.Duration(mins) * time.Minute).Format(time.RFC3339), "categories": []string{"Games"}}
	if in.CalendarID != nil && fmt.Sprint(in.CalendarID) != "" {
		body["calendar_id"] = in.CalendarID
	}
	if _, err := postJSONTo(ctx, sib, "/api/v1/events", body); err != nil {
		s.appFail(w, appCalendar, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "play_url": play, "open_url": linkTo(sib, "/")})
}

func (s *Server) handleTaskLists(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appTasks, scopeTasksRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/lists", nil)
	if err != nil {
		s.appFail(w, appTasks, err)
		return
	}
	out := []map[string]any{}
	for _, l := range rows(v, "lists") {
		out = append(out, map[string]any{"id": idOf(l), "name": str(l, "name", "title")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"lists": out})
}

// handleTodoTask sends a game's to-do to Pocket Tasks.
func (s *Server) handleTodoTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	t, err := s.store.GetTodo(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "to-do")
		return
	}
	g, err := s.store.GetGame(ctx, uid, t.GameID)
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	sib, ok := s.sib(w, r, appTasks, scopeTasksWrite)
	if !ok {
		return
	}
	body := map[string]any{"title": t.Text, "notes": "For the game " + g.Title, "url": s.link(fmt.Sprintf("/games/%d/edit", g.ID)), "tags": []string{"games"}}
	if l := s.settingsOf(ctx, uid).TasksList; l != "" {
		body["list_id"] = l
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/tasks", body)
	if err != nil {
		s.appFail(w, appTasks, err)
		return
	}
	t.TaskURL = firstNonEmpty(str(res, "url"), linkTo(sib, "/"))
	if strings.HasPrefix(t.TaskURL, "/") {
		t.TaskURL = linkTo(sib, t.TaskURL)
	}
	_ = s.store.UpdateTodo(ctx, t)
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleTeams(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appProjects, scopeProjectsRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/teams", nil)
	if err != nil {
		s.appFail(w, appProjects, err)
		return
	}
	out := []map[string]any{}
	for _, t := range rows(v, "teams") {
		out = append(out, map[string]any{"id": idOf(t), "name": str(t, "name"), "key": str(t, "key")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"teams": out})
}

func (s *Server) handleGameIssue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	g, ok := s.gameFor(w, r)
	if !ok {
		return
	}
	var in struct {
		Title  string `json:"title"`
		Text   string `json:"text"`
		TeamID any    `json:"team_id"`
		Bug    bool   `json:"bug"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Title) == "" {
		writeErr(w, http.StatusBadRequest, "give the issue a title")
		return
	}
	sib, ok := s.sib(w, r, appProjects, scopeProjectsWrite)
	if !ok {
		return
	}
	edit := s.link(fmt.Sprintf("/games/%d/edit", g.ID))
	body := map[string]any{"title": cleanText(in.Title, 200), "description": strings.TrimSpace(in.Text + "\n\nGame: [" + g.Title + "](" + edit + ")"), "labels": []string{ifs(in.Bug, "bug", "game")},
		"source_ref": fmt.Sprintf("gamebuilder:%d", g.ID), "source_url": edit}
	if in.TeamID != nil && fmt.Sprint(in.TeamID) != "" {
		body["team_id"] = in.TeamID
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/issues", body)
	if err != nil {
		s.appFail(w, appProjects, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "identifier": str(res, "identifier"), "open_url": firstNonEmpty(str(res, "url"), linkTo(sib, "/"))})
}

// handleGameNote writes a game down in Pocket Notes: what it is, how to
// play, its levels, things and every rule in words, and the to-dos.
func (s *Server) handleGameNote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	g, ok := s.gameFor(w, r)
	if !ok {
		return
	}
	sib, ok := s.sib(w, r, appNotes, scopeNotesWrite)
	if !ok {
		return
	}
	todos, _ := s.store.ListTodos(ctx, uid, g.ID)
	md := gamedoc.Describe(g.Doc)
	if len(todos) > 0 {
		md += "\n## To do\n\n"
		for _, t := range todos {
			md += "- [" + ifs(t.Done, "x", " ") + "] " + t.Text + "\n"
		}
	}
	md += "\n[Open in Game Builder](" + s.link(fmt.Sprintf("/games/%d", g.ID)) + ")"
	if g.ShareOn {
		md += " · [Play](" + s.link("/p/"+g.ShareKey) + ")"
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/notes", map[string]any{"title": g.Title, "content": md, "category": firstNonEmpty(s.settingsOf(ctx, uid).FilesFolder, "Games")})
	if err != nil {
		s.appFail(w, appNotes, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "open_url": linkTo(sib, "/notes/"+idOf(res))})
}

// ---- Pocket Alerts (and the person's own push) ----

// tell sends a message to the person's phone (their notification URL) and
// to Pocket Alerts when it is connected.
func (s *Server) tell(ctx context.Context, uid int64, title, body, p string) {
	st := s.settingsOf(ctx, uid)
	s.push(ctx, uid, st, title, body, p)
	sib, err := optional(s.sibFor(ctx, uid, appAlerts, scopeAlertsWrite))
	if err != nil || sib == nil {
		return
	}
	if _, err := postJSONTo(ctx, sib, "/api/v1/alerts", map[string]any{"title": clip(title, 200), "body": clip(body, 2000), "priority": "default", "tags": []string{"video_game"}, "click": s.link(p)}); err != nil {
		s.log.Debug("alerts failed", "err", err)
	}
}

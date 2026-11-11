package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif" // sizes of GIF pictures
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/store"
)

// The library: pictures (sprites, backgrounds), sounds and music. Uploaded
// here, drawn in the pixel editor, or brought from another Pocket app
// (apps.go). Every game of the person's may use any of them.

func assetOut(a store.Asset) map[string]any {
	b, _ := json.Marshal(a)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	m["url"] = fmt.Sprintf("/api/v1/assets/%d/file", a.ID)
	return m
}

func (s *Server) handleAssets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	kind := r.URL.Query().Get("kind")
	list, err := s.store.ListAssets(ctx, actorID(ctx), kind)
	if err != nil {
		s.fail(w, err, "library")
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, a := range list {
		out = append(out, assetOut(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": out})
}

func (s *Server) handleGetAsset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, err := s.store.GetAsset(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "asset")
		return
	}
	writeJSON(w, http.StatusOK, assetOut(*a))
}

func (s *Server) handleAssetFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, err := s.store.GetAsset(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "asset")
		return
	}
	dl := ""
	if queryBool(r, "download") {
		dl = a.Name + filepath.Ext(a.Path)
	}
	s.serveMediaFile(w, r, a.Path, a.Mime, dl)
}

// picSize reads a picture's width and height (SVG: from its attributes).
func picSize(b []byte) (int, int) {
	if looksSVG(b) {
		return svgSize(b)
	}
	if c, _, err := image.DecodeConfig(bytes.NewReader(b)); err == nil {
		return c.Width, c.Height
	}
	return 0, 0
}

var (
	reSVGW  = regexp.MustCompile(`<svg[^>]*?\swidth="([\d.]+)`)
	reSVGH  = regexp.MustCompile(`<svg[^>]*?\sheight="([\d.]+)`)
	reSVGVB = regexp.MustCompile(`viewBox="[\d.\-]+[ ,]+[\d.\-]+[ ,]+([\d.]+)[ ,]+([\d.]+)"`)
)

func svgSize(b []byte) (int, int) {
	head := string(b[:min(len(b), 4096)])
	if m := reSVGVB.FindStringSubmatch(head); m != nil {
		w, _ := strconv.ParseFloat(m[1], 64)
		h, _ := strconv.ParseFloat(m[2], 64)
		return int(w), int(h)
	}
	mw, mh := reSVGW.FindStringSubmatch(head), reSVGH.FindStringSubmatch(head)
	if mw != nil && mh != nil {
		w, _ := strconv.ParseFloat(mw[1], 64)
		h, _ := strconv.ParseFloat(mh[1], 64)
		return int(w), int(h)
	}
	return 256, 256
}

// addAsset keeps bytes as a new asset of the person's.
func (s *Server) addAsset(r *http.Request, uid int64, kind, name string, b []byte, origin, ref string, gameID int64) (*store.Asset, error) {
	ctx := r.Context()
	a := &store.Asset{UserID: uid, Kind: kind, Name: clip(strings.TrimSpace(strings.TrimSuffix(name, filepath.Ext(name))), maxName), Origin: origin, OriginRef: clip(ref, 500), Source: sourceOf(ctx), Size: int64(len(b))}
	if a.Name == "" {
		a.Name = map[string]string{"image": "Picture", "sound": "Sound", "music": "Music"}[kind]
	}
	if gameID > 0 {
		if _, err := s.store.GetGame(ctx, uid, gameID); err == nil {
			a.GameID = &gameID
		}
	}
	var ext string
	switch kind {
	case "image":
		ext = imageExt(b)
		if ext == "" {
			return nil, &userErr{"that is not a picture (PNG, JPEG, WebP, GIF or SVG)"}
		}
		a.Width, a.Height = picSize(b)
		if ext == ".svg" {
			a.Mime = "image/svg+xml"
		} else {
			a.Mime = http.DetectContentType(b)
		}
	case "sound", "music":
		ext = soundExt(b, name)
		if ext == "" {
			return nil, &userErr{"that is not a sound file (MP3, WAV, OGG, M4A, WebM or FLAC)"}
		}
		a.Mime = mimeByExt("x" + ext)
	default:
		return nil, &userErr{"kind must be image, sound or music"}
	}
	rel, err := s.saveBytes(uid, ext, b)
	if err != nil {
		return nil, err
	}
	a.Path = rel
	if err := s.store.CreateAsset(ctx, a); err != nil {
		s.dropMedia(rel)
		return nil, err
	}
	return a, nil
}

// handleUploadAsset takes one or more files (multipart "file"), with
// kind=image|sound|music (guessed when missing) and game_id.
func (s *Server) handleUploadAsset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "send the files as a form (file)")
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck
	kind := r.FormValue("kind")
	gameID, _ := strconv.ParseInt(r.FormValue("game_id"), 10, 64)
	out := []map[string]any{}
	var errs []string
	for _, fh := range r.MultipartForm.File["file"] {
		f, err := fh.Open()
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(f, s.cfg.MaxUploadBytes+1))
		f.Close()
		if err != nil {
			continue
		}
		if int64(len(b)) > s.cfg.MaxUploadBytes {
			errs = append(errs, fh.Filename+": too big")
			continue
		}
		k := kind
		if k == "" {
			if imageExt(b) != "" {
				k = "image"
			} else if len(b) > 2<<20 {
				k = "music"
			} else {
				k = "sound"
			}
		}
		a, err := s.addAsset(r, uid, k, fh.Filename, b, "upload", "", gameID)
		if err != nil {
			errs = append(errs, fh.Filename+": "+plainMsg(err))
			continue
		}
		out = append(out, assetOut(*a))
	}
	if len(out) == 0 && len(errs) > 0 {
		writeErr(w, http.StatusBadRequest, strings.Join(errs, "; "))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"assets": out, "errors": errs})
}

func plainMsg(err error) string {
	if ue, ok := err.(*userErr); ok {
		return ue.msg
	}
	return err.Error()
}

// handleDrawnAsset keeps a sprite drawn in the pixel editor: the PNG it
// makes (all frames side by side) and its pixels to draw on later. POST
// makes one, PUT /assets/{id}/drawn changes one.
func (s *Server) handleDrawnAsset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Name   string          `json:"name"`
		PNG    string          `json:"png"` // data:image/png;base64,…
		Pixels json.RawMessage `json:"pixels"`
		Frames int             `json:"frames"`
		GameID int64           `json:"game_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "send the drawing as JSON")
		return
	}
	b, err := dataURL(in.PNG)
	if err != nil || imageExt(b) != ".png" {
		writeErr(w, http.StatusBadRequest, "the drawing must come as a PNG")
		return
	}
	if len(in.Pixels) > 2<<20 {
		writeErr(w, http.StatusBadRequest, "that drawing is too big")
		return
	}
	frames := clampInt(in.Frames, 1, 64)
	width, height := picSize(b)
	if r.Method == http.MethodPut {
		a, err := s.store.GetAsset(ctx, uid, idParam(r, "id"))
		if err != nil {
			s.fail(w, err, "drawing")
			return
		}
		rel, err := s.saveBytes(uid, ".png", b)
		if err != nil {
			s.fail(w, err, "drawing")
			return
		}
		a.Path, a.Size, a.Width, a.Height, a.Frames, a.Pixels, a.Mime = rel, int64(len(b)), width, height, frames, string(in.Pixels), "image/png"
		if n := cleanText(in.Name, maxName); n != "" {
			a.Name = n
		}
		old, err := s.store.UpdateAsset(ctx, a)
		if err != nil {
			s.dropMedia(rel)
			s.fail(w, err, "drawing")
			return
		}
		s.dropMedia(old)
		writeJSON(w, http.StatusOK, assetOut(*a))
		return
	}
	a, err := s.addAsset(r, uid, "image", firstNonEmpty(cleanText(in.Name, maxName), "Drawing"), b, "drawn", "", in.GameID)
	if err != nil {
		s.userFail(w, err, "drawing")
		return
	}
	a.Frames, a.Pixels = frames, string(in.Pixels)
	_, _ = s.store.UpdateAsset(ctx, a)
	writeJSON(w, http.StatusCreated, assetOut(*a))
}

func (s *Server) handleUpdateAsset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	a, err := s.store.GetAsset(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "asset")
		return
	}
	var in struct {
		Name   string `json:"name"`
		Frames int    `json:"frames"`
	}
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "send JSON")
		return
	}
	if keys["name"] && strings.TrimSpace(in.Name) != "" {
		a.Name = cleanText(in.Name, maxName)
	}
	if keys["frames"] {
		a.Frames = clampInt(in.Frames, 1, 64)
	}
	if _, err := s.store.UpdateAsset(ctx, a); err != nil {
		s.fail(w, err, "asset")
		return
	}
	writeJSON(w, http.StatusOK, assetOut(*a))
}

func (s *Server) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := s.store.DeleteAsset(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "asset")
		return
	}
	s.dropMedia(p)
	w.WriteHeader(http.StatusNoContent)
}

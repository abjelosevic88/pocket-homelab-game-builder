package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/store"
)

// Router builds the chi router.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	if s.cfg.TrustProxy {
		r.Use(middleware.RealIP)
	}
	r.Use(middleware.RequestID)
	r.Use(s.logging)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	r.Get("/pocket-ask.js", s.core.AskWidget("gamebuilder"))
	r.Get("/healthz", s.handleHealth)
	r.Get("/readyz", s.handleReady)
	r.Get("/api/health", s.handleHealth)
	if s.cfg.MetricsEnabled {
		r.Get("/metrics", s.handleMetrics)
	}

	// shared games: anyone with the link
	r.Get("/p/{key}", s.handlePlayerPage)
	r.Get("/e/{key}", s.handlePlayerPage)

	r.Route("/api/v1", func(r chi.Router) {
		// uploads and file downloads: no time limit
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Post("/assets", s.handleUploadAsset)
			r.Post("/apps/files/import", s.handleFilesImport)
			r.Post("/apps/music/import", s.handleMusicImport)
			r.Post("/apps/voice/import", s.handleVoiceImport)
			r.Get("/assets/{id}/file", s.handleAssetFile)
			r.Post("/import", s.handleImport)
			r.Post("/games/import", s.handleImportGame)
			r.Get("/export.json", s.handleExportJSON)
			r.Get("/games/{id}/export.html", s.handleExportHTML)
			r.Get("/games/{id}/export.json", s.handleExportGame)
			r.Post("/games/{id}/files", s.handleGameToFiles)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(120 * time.Second))
			r.Post("/auth/login", s.handleLogin)
			r.Post("/auth/setup", s.handleSetup)
			r.Get("/auth/status", s.handleAuthStatus)
			r.Get("/auth/oidc/start", s.handleOIDCStart)
			r.Get("/auth/oidc/callback", s.handleOIDCCallback)

			// shared games (no account)
			r.Get("/public/{key}", s.handlePublicGame)
			r.Get("/public/{key}/cover.png", s.handlePublicCover)
			r.Get("/public/{key}/assets/{id}", s.handlePublicAsset)
			r.Post("/public/{key}/plays", s.handlePublicPlay)

			r.Group(func(r chi.Router) {
				r.Use(s.requireAuth)
				r.Post("/auth/logout", s.handleLogout)
				r.Get("/auth/me", s.handleMe)
				r.Put("/auth/me", s.handleUpdateMe)

				// for the person, and for the rest of the box: Pocket Day, Search, AI
				r.Get("/home", s.handleHome)
				r.Get("/summary", s.handleSummary)
				r.Get("/day/{date}", s.handleDay)
				r.Get("/search", s.handleSearch)
				r.Get("/stats", s.handleStats)
				r.Get("/templates", s.handleTemplates)

				// games
				r.Get("/games", s.handleGames)
				r.Post("/games", s.handleCreateGame)
				r.Get("/games/{id}", s.handleGetGame)
				r.Put("/games/{id}", s.handleUpdateGame)
				r.Delete("/games/{id}", s.handleDeleteGame)
				r.Put("/games/{id}/doc", s.handleSaveDoc)
				r.Post("/games/{id}/copy", s.handleCopyGame)
				r.Get("/games/{id}/cover", s.handleCover)
				r.Post("/games/{id}/cover", s.handleCover)
				r.Delete("/games/{id}/cover", s.handleCover)
				r.Post("/games/{id}/share", s.handleShare)
				r.Post("/games/{id}/share/new-key", s.handleNewShareKey)
				r.Get("/games/{id}/versions", s.handleVersions)
				r.Post("/games/{id}/versions", s.handleVersions)
				r.Get("/versions/{id}", s.handleVersion)
				r.Delete("/versions/{id}", s.handleVersion)
				r.Post("/versions/{id}/restore", s.handleRestoreVersion)
				r.Get("/games/{id}/plays", s.handlePlays)
				r.Post("/games/{id}/plays", s.handlePlays)
				r.Delete("/games/{id}/scores", s.handleClearScores)
				r.Delete("/plays/{id}", s.handleDeletePlay)
				r.Get("/games/{id}/todos", s.handleTodos)
				r.Post("/games/{id}/todos", s.handleTodos)
				r.Put("/todos/{id}", s.handleTodo)
				r.Delete("/todos/{id}", s.handleTodo)
				r.Post("/remix/{key}", s.handleRemix)

				// the library
				r.Get("/assets", s.handleAssets)
				r.Post("/assets/drawn", s.handleDrawnAsset)
				r.Get("/assets/{id}", s.handleGetAsset)
				r.Put("/assets/{id}", s.handleUpdateAsset)
				r.Put("/assets/{id}/drawn", s.handleDrawnAsset)
				r.Delete("/assets/{id}", s.handleDeleteAsset)

				// the other Pocket apps
				r.Get("/apps", s.handleApps)
				r.Get("/apps/photos", s.handlePhotosList)
				r.Get("/apps/photos/{id}/thumb", s.handlePhotoThumb)
				r.Post("/apps/photos/import", s.handlePhotosImport)
				r.Get("/apps/files", s.handleFilesList)
				r.Get("/apps/draw", s.handleDrawList)
				r.Get("/apps/draw/{id}/preview", s.handleDrawPreview)
				r.Post("/apps/draw/import", s.handleDrawImport)
				r.Get("/apps/photolab", s.handlePhotoLabList)
				r.Get("/apps/photolab/{id}/thumb", s.handlePhotoLabThumb)
				r.Post("/apps/photolab/import", s.handlePhotoLabImport)
				r.Get("/apps/music", s.handleMusicSearch)
				r.Get("/apps/voice", s.handleVoiceList)
				r.Get("/apps/mazes", s.handleMazeList)
				r.Post("/apps/mazes/game", s.handleMazeGame)
				r.Get("/apps/chats", s.handleChats)
				r.Get("/apps/forums", s.handleForums)
				r.Get("/apps/contacts", s.handleContacts)
				r.Get("/apps/calendars", s.handleCalendars)
				r.Get("/apps/tasks/lists", s.handleTaskLists)
				r.Get("/apps/projects/teams", s.handleTeams)
				r.Post("/games/{id}/translate", s.handleTranslateGame)
				r.Post("/games/{id}/chat", s.handleGameChat)
				r.Post("/games/{id}/blog", s.handleGameBlog)
				r.Post("/games/{id}/forum", s.handleGameForum)
				r.Post("/games/{id}/email", s.handleGameEmail)
				r.Post("/games/{id}/family", s.handleGameFamily)
				r.Post("/games/{id}/playtest", s.handleGamePlaytest)
				r.Post("/games/{id}/issue", s.handleGameIssue)
				r.Post("/games/{id}/note", s.handleGameNote)
				r.Post("/todos/{id}/task", s.handleTodoTask)

				// AI
				r.Get("/ai/status", s.handleAIStatus)
				r.Get("/ai/models", s.handleAIModels)
				r.Post("/ai/game", s.handleAIGame)
				r.Post("/ai/rule", s.handleAIRule)
				r.Post("/ai/questions", s.handleAIQuestions)
				r.Post("/games/{id}/ideas", s.handleAIIdeas)

				r.Post("/integrations/test", s.handleIntegrationTest)
				r.Post("/notify/test", s.handleNotifyTest)

				r.Get("/users", s.handleListUsers)
				r.Post("/users", s.handleCreateUser)
				r.Put("/users/{id}", s.handleUpdateUser)
				r.Delete("/users/{id}", s.handleDeleteUser)

				r.Get("/settings", s.handleGetSettings)
				r.Put("/settings", s.handlePutSettings)
				r.Get("/settings/system", s.handleSystemInfo)

				r.Get("/tokens", s.handleListTokens)
				r.Post("/tokens", s.handleCreateToken)
				r.Delete("/tokens/{id}", s.handleDeleteToken)

				r.Get("/backup", s.handleBackup)
				r.Get("/export.csv", s.handleExportCSV)
			})
		})
	})

	r.NotFound(s.handleSPA)
	return r
}

// ---------- helpers ----------

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.requests.Add(1)
		if ww.Status() >= 500 {
			s.errors.Add(1)
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			return
		}
		s.log.Debug("http", "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "dur", time.Since(start).String(), "ip", r.RemoteAddr)
	})
}

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func (s *Server) fail(w http.ResponseWriter, err error, context string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not found")
		return
	case errors.Is(err, store.ErrConflict), errors.Is(err, core.ErrOperationConflict):
		writeErr(w, http.StatusConflict, "already exists")
		return
	}
	s.log.Error(context, "err", err)
	writeErr(w, http.StatusInternalServerError, context+": "+err.Error())
}

func (s *Server) readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(http.MaxBytesReader(nil, r.Body, s.cfg.MaxBodyBytes))
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 32<<20))
	return dec.Decode(v)
}

// decodeKeys decodes JSON into v and also reports which top-level keys were
// present, so PUT handlers can tell "not sent" from "sent empty".
func decodeKeys(r *http.Request, v any) (map[string]bool, error) {
	b, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	keys := make(map[string]bool, len(raw))
	for k := range raw {
		keys[k] = true
	}
	return keys, json.Unmarshal(b, v)
}

func idParam(r *http.Request, name string) int64 {
	n, _ := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return n
}

func queryInt(r *http.Request, name string) int {
	n, _ := strconv.Atoi(r.URL.Query().Get(name))
	return n
}

func queryBool(r *http.Request, name string) bool {
	v := strings.ToLower(r.URL.Query().Get(name))
	return v == "1" || v == "true" || v == "yes"
}

func validDate(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// ---------- health / metrics ----------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"core_outbox": s.coreOutboxHealth(r.Context()), "status": "ok", "version": Version, "uptime": time.Since(s.started).Round(time.Second).String()})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DB.PingContext(r.Context()); err != nil {
		writeErr(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	counts := s.store.Counts(ctx)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP pocket_gamebuilder_info Build info.\n# TYPE pocket_gamebuilder_info gauge\npocket_gamebuilder_info{version=%q} 1\n", Version)
	fmt.Fprintf(w, "# HELP pocket_gamebuilder_uptime_seconds Process uptime.\n# TYPE pocket_gamebuilder_uptime_seconds gauge\npocket_gamebuilder_uptime_seconds %d\n", int(time.Since(s.started).Seconds()))
	fmt.Fprintf(w, "# HELP pocket_gamebuilder_http_requests_total HTTP requests served.\n# TYPE pocket_gamebuilder_http_requests_total counter\npocket_gamebuilder_http_requests_total %d\n", s.requests.Load())
	fmt.Fprintf(w, "# HELP pocket_gamebuilder_http_errors_total HTTP 5xx responses.\n# TYPE pocket_gamebuilder_http_errors_total counter\npocket_gamebuilder_http_errors_total %d\n", s.errors.Load())
	fmt.Fprintf(w, "# HELP pocket_gamebuilder_rows Rows per table.\n# TYPE pocket_gamebuilder_rows gauge\n")
	for k, v := range counts {
		fmt.Fprintf(w, "pocket_gamebuilder_rows{table=%q} %d\n", k, v)
	}
}

// ---------- static / SPA ----------

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if s.webFS == nil {
		http.Error(w, "web UI not built (run `make web`)", http.StatusNotFound)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if f, err := s.webFS.Open(path); err == nil {
		f.Close()
		if strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.FileServerFS(s.webFS).ServeHTTP(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	r.URL.Path = "/"
	http.FileServerFS(s.webFS).ServeHTTP(w, r)
}

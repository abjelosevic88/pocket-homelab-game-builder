package server

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/store"
)

// handleHome is what the sidebar needs: counts and the latest games.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	games, _ := s.store.ListGames(ctx, uid, store.GameFilter{})
	shared, fav, todo := 0, 0, 0
	recent := []map[string]any{}
	for i, g := range games {
		if g.ShareOn {
			shared++
		}
		if g.Favorite {
			fav++
		}
		todo += g.Todo
		if i < 6 {
			recent = append(recent, map[string]any{"id": g.ID, "title": g.Title, "shared": g.ShareOn, "template": g.Template})
		}
	}
	week := time.Now().AddDate(0, 0, -7).UTC().Format(time.RFC3339Nano)
	plays, _ := s.store.ListPlays(ctx, uid, store.PlayFilter{Others: true, From: week, Limit: 10000})
	assets, _ := s.store.ListAssets(ctx, uid, "")
	writeJSON(w, http.StatusOK, map[string]any{"games": len(games), "shared": shared, "favorites": fav, "todo": todo, "assets": len(assets),
		"plays_week": len(plays), "recent": recent})
}

// handleSummary is a short overview for other apps (Pocket Hub, AI, Day).
func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	games, _ := s.store.ListGames(ctx, uid, store.GameFilter{})
	week := time.Now().AddDate(0, 0, -7).UTC().Format(time.RFC3339Nano)
	plays, _ := s.store.ListPlays(ctx, uid, store.PlayFilter{Others: true, From: week, Limit: 10000})
	perGame := map[int64]int{}
	for _, p := range plays {
		perGame[p.GameID]++
	}
	list := []map[string]any{}
	for i, g := range games {
		if i >= 8 {
			break
		}
		item := map[string]any{"id": g.ID, "title": g.Title, "kind": templateName(g.Template), "scenes": g.Scenes, "rules": g.Rules, "shared": g.ShareOn,
			"plays_by_others": g.Plays, "plays_this_week": perGame[g.ID], "open_todos": g.Todo, "changed": g.UpdatedAt, "url": s.link(fmt.Sprintf("/games/%d", g.ID))}
		if g.ShareOn {
			item["play_url"] = s.link("/p/" + g.ShareKey)
		}
		list = append(list, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"games": len(games), "latest": list, "plays_this_week": len(plays), "url": s.link("/")})
}

// handleDay is what happened on one day, for Pocket Day's journal: games
// made or worked on, and plays.
func (s *Server) handleDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	day := chiParam(r, "date")
	if _, err := time.Parse("2006-01-02", day); err != nil {
		writeErr(w, http.StatusBadRequest, "date must be YYYY-MM-DD")
		return
	}
	games, _ := s.store.ListGames(ctx, uid, store.GameFilter{})
	made, changed := []map[string]any{}, []map[string]any{}
	for _, g := range games {
		item := map[string]any{"id": g.ID, "title": g.Title, "url": s.link(fmt.Sprintf("/games/%d", g.ID))}
		if localDay(g.CreatedAt) == day {
			made = append(made, item)
		} else if localDay(g.UpdatedAt) == day {
			changed = append(changed, item)
		}
	}
	plays, _ := s.store.ListPlays(ctx, uid, store.PlayFilter{Day: day, Limit: 1000})
	mine, others := 0, 0
	byGame := map[string]int{}
	for _, p := range plays {
		if p.Mine {
			mine++
		} else {
			others++
			byGame[p.Game]++
		}
	}
	played := []map[string]any{}
	for t, n := range byGame {
		played = append(played, map[string]any{"title": t, "plays": n})
	}
	sort.Slice(played, func(i, j int) bool { return played[i]["plays"].(int) > played[j]["plays"].(int) })
	lines := []string{}
	for _, g := range made {
		lines = append(lines, "Started a game: "+g["title"].(string))
	}
	for _, g := range changed {
		lines = append(lines, "Worked on "+g["title"].(string))
	}
	if others > 0 {
		lines = append(lines, fmt.Sprintf("Your games were played %d %s by others", others, plural(others, "time", "times")))
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": day, "made": made, "changed": changed, "test_plays": mine, "plays_by_others": others, "played": played, "lines": lines})
}

// handleSearch finds games by title, description and the words in them,
// and pictures and sounds by name.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	limit := firstPos(queryInt(r, "limit"), 20)
	out := []map[string]any{}
	if q != "" {
		games, _ := s.store.ListGames(ctx, uid, store.GameFilter{})
		for _, g := range games {
			hit := strings.Contains(strings.ToLower(g.Title), q) || strings.Contains(strings.ToLower(g.Description), q)
			if !hit {
				if full, err := s.store.GetGame(ctx, uid, g.ID); err == nil {
					hit = strings.Contains(strings.ToLower(string(full.Doc)), q)
				}
			}
			if hit {
				out = append(out, map[string]any{"kind": "game", "id": g.ID, "title": g.Title,
					"subtitle": joinNonEmpty(" · ", templateName(g.Template), fmt.Sprintf("%d %s", g.Scenes, plural(g.Scenes, "scene", "scenes")), ifs(g.ShareOn, "shared", "")),
					"text":     g.Description, "url": s.link(fmt.Sprintf("/games/%d", g.ID)), "date": g.UpdatedAt})
			}
			if len(out) >= limit {
				break
			}
		}
		if len(out) < limit {
			assets, _ := s.store.ListAssets(ctx, uid, "")
			for _, a := range assets {
				if strings.Contains(strings.ToLower(a.Name), q) {
					out = append(out, map[string]any{"kind": a.Kind, "id": a.ID, "title": a.Name, "subtitle": kindWord(a.Kind), "url": s.link("/library"), "path": "/library", "date": a.CreatedAt})
				}
				if len(out) >= limit {
					break
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out, "items": out, "hits": out})
}

func kindWord(k string) string {
	return map[string]string{"image": "Picture", "sound": "Sound", "music": "Music"}[k]
}

func templateName(id string) string {
	switch id {
	case "platformer":
		return "Jump and run"
	case "maze":
		return "Maze"
	case "shooter":
		return "Shooter"
	case "bricks":
		return "Brick breaker"
	case "catch":
		return "Catch"
	case "quiz":
		return "Quiz"
	case "import":
		return "Brought in"
	}
	return "Game"
}

// handleStats: plays per day, the most played games, the library.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	days := clampInt(firstPos(queryInt(r, "days"), 30), 7, 365)
	from := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	per, _ := s.store.PlaysPerDay(ctx, uid, from)
	series := []map[string]any{}
	total := 0
	for i := days - 1; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		series = append(series, map[string]any{"day": d, "plays": per[d]})
		total += per[d]
	}
	games, _ := s.store.ListGames(ctx, uid, store.GameFilter{})
	sort.SliceStable(games, func(i, j int) bool { return games[i].Plays > games[j].Plays })
	top := []map[string]any{}
	for i, g := range games {
		if i >= 8 || g.Plays == 0 {
			break
		}
		top = append(top, map[string]any{"id": g.ID, "title": g.Title, "plays": g.Plays, "best": g.Best})
	}
	assets, _ := s.store.ListAssets(ctx, uid, "")
	kinds := map[string]int{"image": 0, "sound": 0, "music": 0}
	for _, a := range assets {
		kinds[a.Kind]++
	}
	writeJSON(w, http.StatusOK, map[string]any{"days": series, "plays": total, "top": top, "games": len(games), "assets": kinds})
}

func localDay(iso string) string {
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		return ""
	}
	return t.In(time.Local).Format("2006-01-02")
}

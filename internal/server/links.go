package server

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/store"
)

// Pocket Core app links (docs/links.md in pocket-homelab-core), Game Builder as
// the target: another Pocket app the person allowed in Core may act here for
// them with a pcx_ token, only within these scopes. Every route not listed
// in linkRoutes stays closed to it (403).

var linkOffers = []core.Offer{
	{Scope: "gamebuilder:read", Name: "See the games you made", Description: "Read your games (what they are, their levels, things and rules), how often they were played, the best scores and what is still to do: for your journal, search and Pocket AI."},
	{Scope: "gamebuilder:write", Name: "Make games for you", Description: "Make a new game from a template or an idea (\"a maze with a dragon\"), add a to-do to a game, and change or remove only the games and to-dos that app added. Never shares a game or changes one you made."},
}

// linkRoute is one request a Pocket Core link token may make.
type linkRoute struct {
	methods string
	path    *regexp.Regexp
	scope   string // "" = any linked app
}

var linkRoutes = []linkRoute{
	{"GET", regexp.MustCompile(`^/api/v1/auth/me$`), ""},
	{"GET", regexp.MustCompile(`^/api/v1/(home|summary|search|stats|templates|games|assets|day/\d{4}-\d{2}-\d{2})$`), "gamebuilder:read"},
	{"GET", regexp.MustCompile(`^/api/v1/games/\d+(/plays|/todos|/versions|/cover)?$`), "gamebuilder:read"},
	{"POST", regexp.MustCompile(`^/api/v1/(games|ai/game)$`), "gamebuilder:write"},
	{"POST", regexp.MustCompile(`^/api/v1/games/\d+/todos$`), "gamebuilder:write"},
	{"PUT DELETE", regexp.MustCompile(`^/api/v1/(games|todos)/\d+$`), "gamebuilder:write"},
}

var errLinkScope = errors.New("outside the link's scope")

// linkAllows reports whether a link token with these scopes may make this request.
func linkAllows(scopes []string, r *http.Request) bool {
	for _, lr := range linkRoutes {
		if !strings.Contains(" "+lr.methods+" ", " "+r.Method+" ") || !lr.path.MatchString(r.URL.Path) {
			continue
		}
		if lr.scope == "" {
			return true
		}
		for _, sc := range scopes {
			if sc == lr.scope {
				return true
			}
		}
	}
	return false
}

// linkUser validates a pcx_ token, checks the route against its scopes and
// maps it to the person it acts for. The second value is the acting app's slug.
func (s *Server) linkUser(ctx context.Context, tok string, r *http.Request) (*store.User, string, error) {
	in, err := s.core.Introspect(ctx, tok)
	if err != nil {
		return nil, "", err
	}
	if !in.Active || !in.Delegated() || in.Audience != "gamebuilder" || in.User == nil || in.User.Email == "" || in.App == nil || in.App.Slug == "" {
		return nil, "", http.ErrNoCookie
	}
	if !linkAllows(strings.Fields(in.Scope), r) {
		return nil, "", errLinkScope
	}
	u, err := s.coreIdentityUser(ctx, in)
	return u, in.App.Slug, err
}

const linkAppKey ctxKey = 2

// linkApp is the acting app's slug when the request came in on a link token.
func linkApp(ctx context.Context) string {
	a, _ := ctx.Value(linkAppKey).(string)
	return a
}

// withLinkApp marks a context as acting for another app (tests, and the auth middleware).
func withLinkApp(ctx context.Context, slug string) context.Context {
	return context.WithValue(ctx, linkAppKey, slug)
}

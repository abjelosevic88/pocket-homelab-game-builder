package server

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/pocket"
	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/store"
)

// ---- the other Pocket apps ----
//
// Each person chooses how Game Builder reaches each app: through a Pocket Core app
// link (Game Builder asks Core for a five-minute token acting as that person,
// nothing to paste), directly with their own URL + API token, or not at
// all. Core tokens are asked for per request with only the scopes the work
// needs and never kept, so a link switched off in Core applies at once.

const (
	scopePhotosRead    = "photos:read"
	scopeFilesRead     = "files:read"
	scopeFilesWrite    = "files:write"
	scopeDrawRead      = "drawings:read"
	scopePhotoLabRead  = "photolab:read"
	scopeMusicPlay     = "music:play"
	scopeVoiceRead     = "voice:read"
	scopeTranslate     = "translate:run"
	scopePuzzlesRead   = "puzzlecraft:read"
	scopeBlogWrite     = "blog:write"
	scopeChatRead      = "chat:read"
	scopeChatWrite     = "chat:write"
	scopeForumRead     = "forum:read"
	scopeForumWrite    = "forum:write"
	scopeMailSend      = "mail:send"
	scopeContactsRead  = "contacts:read"
	scopeTasksRead     = "tasks:read"
	scopeTasksWrite    = "tasks:write"
	scopeProjectsRead  = "projects:read"
	scopeProjectsWrite = "projects:write"
	scopeNotesWrite    = "notes:write"
	scopeCalendarRead  = "calendar:read"
	scopeCalendarWrite = "calendar:write"
	scopeFamilyWrite   = "family:write"
	scopeAlertsWrite   = "alerts:write"
)

// pocketApp is one sibling Pocket app Game Builder can reach.
type pocketApp struct {
	slug, name string
	scopes     []string
	reason     string
}

var (
	appPhotos    = pocketApp{"photos", "Pocket Photos", []string{scopePhotosRead}, "Use one of your photos as a picture in a game: a character, a thing or a background"}
	appFiles     = pocketApp{"files", "Pocket Files", []string{scopeFilesRead, scopeFilesWrite}, "Bring pictures, sounds and music in from your files, and save a finished game there as one file that plays anywhere"}
	appDraw      = pocketApp{"draw", "Pocket Draw", []string{scopeDrawRead}, "Use a drawing you made in Pocket Draw as a picture in a game"}
	appPhotoLab  = pocketApp{"photolab", "Pocket Photo Lab", []string{scopePhotoLabRead}, "Use a picture you edited in Photo Lab as a background or a character"}
	appMusic     = pocketApp{"music", "Pocket Music", []string{scopeMusicPlay}, "Pick a song from your music library to play while a level runs"}
	appVoice     = pocketApp{"voice", "Pocket Voice", []string{scopeVoiceRead}, "Use a recording from Pocket Voice as a sound in a game (a voice line, a cheer)"}
	appTranslate = pocketApp{"translate", "Pocket Translate", []string{scopeTranslate}, "Make a copy of a game in another language: every word on the screen translated"}
	appPuzzles   = pocketApp{"puzzlecraft", "Pocket Puzzlecraft", []string{scopePuzzlesRead}, "Turn one of your Puzzlecraft mazes into a maze game to walk through"}
	appBlog      = pocketApp{"blog", "Pocket Blog", []string{scopeBlogWrite}, "Write a blog post about a game, with its picture and the link to play it"}
	appChat      = pocketApp{"chat", "Pocket Chat", []string{scopeChatRead, scopeChatWrite}, "Send the link to play a game to a chat"}
	appForum     = pocketApp{"forum", "Pocket Forum", []string{scopeForumRead, scopeForumWrite}, "Start a topic about a game in a forum, to ask what people think"}
	appEmail     = pocketApp{"mail", "Pocket Mail", []string{scopeMailSend}, "Send someone the link to play a game by email"}
	appContacts  = pocketApp{"contacts", "Pocket Contacts", []string{scopeContactsRead}, "Choose who to send a game to from your address book"}
	appTasks     = pocketApp{"tasks", "Pocket Tasks", []string{scopeTasksRead, scopeTasksWrite}, "Put what is still to do on a game in your to-do list"}
	appProjects  = pocketApp{"projects", "Pocket Projects", []string{scopeProjectsRead, scopeProjectsWrite}, "Make an issue from a game: a bug someone found, or the next thing to build"}
	appNotes     = pocketApp{"notes", "Pocket Notes", []string{scopeNotesWrite}, "Write a game down as a note: its story, its levels and every rule in words"}
	appCalendar  = pocketApp{"calendar", "Pocket Calendar", []string{scopeCalendarRead, scopeCalendarWrite}, "Plan a game night or a play-test in your calendar, with the link to play"}
	appFamily    = pocketApp{"family", "Pocket Family", []string{scopeFamilyWrite}, "Put a game on the family board for everyone at home to play"}
	appAlerts    = pocketApp{"alerts", "Pocket Alerts", []string{scopeAlertsWrite}, "Tell you when someone beats the best score in one of your shared games"}
	pocketApps   = []pocketApp{appPhotos, appFiles, appDraw, appPhotoLab, appMusic, appVoice, appTranslate, appPuzzles, appBlog, appChat, appForum, appEmail, appContacts, appTasks, appProjects, appNotes, appCalendar, appFamily, appAlerts}
)

func appBySlug(slug string) (pocketApp, bool) {
	for _, a := range pocketApps {
		if a.slug == slug {
			return a, true
		}
	}
	return pocketApp{}, false
}

// linkUses is the manifest's Uses.
func linkUses() []core.Use {
	out := make([]core.Use, 0, len(pocketApps))
	for _, a := range pocketApps {
		out = append(out, core.Use{App: a.slug, Scopes: a.scopes, Reason: a.reason})
	}
	return out
}

// Connection modes, chosen per person and per app in Settings.
const (
	modeCore   = "core"
	modeDirect = "direct"
	modeOff    = "off"
)

// connMode is the mode in effect: the chosen one, else direct when a token
// was entered, else Pocket Core when Game Builder is connected to one, else direct.
func (s *Server) connMode(mode, token string) string {
	switch mode {
	case modeCore, modeDirect, modeOff:
		return mode
	}
	if token != "" || !s.core.Configured() {
		return modeDirect
	}
	return modeCore
}

// notConnected means the person has no working connection to the app.
type notConnected struct{ msg string }

func (e *notConnected) Error() string { return e.msg }

func isNotConnected(err error) bool {
	var nc *notConnected
	return errors.As(err, &nc)
}

// siblingConn is how Game Builder reaches one person's copy of a sibling app.
type siblingConn struct {
	client *pocket.Client
	via    string
	public string
}

func linkReason(app, reason string) string {
	switch reason {
	case "off_person":
		return "you switched the Game Builder → " + app + " link off in Pocket Core (Settings → Apps acting for me)"
	case "off_admin":
		return "the Game Builder → " + app + " link is switched off in Pocket Core (Links)"
	case "target_missing":
		return app + " is not registered with Pocket Core"
	case "not_offered":
		return "this " + app + " version does not accept app links yet"
	case "not_declared":
		return "Pocket Core has not seen this Game Builder version yet; it re-registers within a minute of starting"
	case "not_in_audience":
		return "you do not have access to " + app + " in Pocket Core"
	case "unknown_user":
		return "your email is not a person in Pocket Core"
	}
	return ""
}

// connSettings returns a person's saved mode, URL, token and public URL for an app.
func connSettings(st store.Settings, a pocketApp) (mode, url, token, public string) {
	c := st.Conns[a.slug]
	return c.Mode, c.URL, c.Token, c.PublicURL
}

// siblingFor connects to app a for person u with exactly these scopes.
func (s *Server) siblingFor(ctx context.Context, u *store.User, st store.Settings, a pocketApp, scopes ...string) (*siblingConn, error) {
	if u == nil {
		return nil, &notConnected{"nobody to act for"}
	}
	mode, url, token, public := connSettings(st, a)
	switch s.connMode(mode, token) {
	case modeOff:
		return nil, &notConnected{a.name + " is switched off in Settings → Connected apps"}
	case modeDirect:
		if token == "" {
			return nil, &notConnected{a.name + " is not set up (Settings → Connected apps): add a URL and token, or switch to Pocket Core"}
		}
		base := url
		if base == "" {
			base = s.resolveSibling(ctx, a.slug)
		}
		if base == "" {
			return nil, &notConnected{"set the " + a.name + " URL in Settings → Connected apps"}
		}
		return &siblingConn{client: pocket.New(base, token, 15*time.Second), via: modeDirect, public: firstNonEmpty(public, base)}, nil
	}
	if !s.core.Configured() {
		return nil, &notConnected{a.name + " through Pocket Core: there is no Pocket Core on this box; use a direct connection"}
	}
	tok, err := s.core.ActAs(ctx, u.Email, a.slug, scopes...)
	if err != nil {
		var le *core.LinkError
		if errors.As(err, &le) {
			if t := linkReason(a.name, le.Reason); t != "" {
				return nil, &notConnected{a.name + " is not set up: " + t}
			}
			return nil, &notConnected{a.name + " is not set up: " + le.Error()}
		}
		return nil, fmt.Errorf("%s through Pocket Core: %w", a.name, err)
	}
	if tok.URL == "" || tok.AccessToken == "" {
		return nil, fmt.Errorf("%s through Pocket Core: Core gave no address or token", a.name)
	}
	return &siblingConn{client: pocket.New(tok.URL, tok.AccessToken, 15*time.Second), via: modeCore, public: firstNonEmpty(public, firstNonEmpty(tok.PublicURL, tok.URL))}, nil
}

// connStatus says, without issuing anything, how a person reaches an app.
func (s *Server) connStatus(ctx context.Context, u *store.User, st store.Settings, a pocketApp) (eff, via, detail string) {
	as := s.appStatus(ctx, u, st, a)
	return as.Mode, as.Via, as.Detail
}

// appState is how one person reaches one app, in words a person can act on.
type appState struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
	Mode   string `json:"mode"` // core | direct | off
	Via    string `json:"via"`  // core | direct | "" (does not work)
	// State: on | partly | off (switched off here) | link_off (switched off
	// in Pocket Core) | missing (the app is not on this box) | old (the app
	// is too old for this) | no_core | not_set_up | error
	State  string   `json:"state"`
	Detail string   `json:"detail"`
	Fix    string   `json:"fix"`
	On     []string `json:"on"` // the scopes that work
	Open   string   `json:"open_url"`
}

// Can reports whether a scope works.
func (a appState) Can(scope string) bool {
	for _, s := range a.On {
		if s == scope {
			return true
		}
	}
	return false
}

func stateOf(reason string) (state, fix string) {
	switch reason {
	case "off_person":
		return "link_off", "Switch it on in Pocket Core → Settings → Apps acting for me"
	case "off_admin":
		return "link_off", "Switch the link on in Pocket Core → Links"
	case "target_missing":
		return "missing", "Install it and connect it to Pocket Core; until then Game Builder keeps working without it"
	case "not_offered":
		return "old", "Update it to the newest version"
	case "not_declared":
		return "error", "Wait a minute: Game Builder re-registers with Pocket Core shortly after it starts"
	case "not_in_audience", "unknown_user":
		return "error", "Ask whoever runs Pocket Core to give you access"
	}
	return "error", ""
}

// appStatus works out an appState (cached for half a minute per person).
func (s *Server) appStatus(ctx context.Context, u *store.User, st store.Settings, a pocketApp) appState {
	mode, _, token, public := connSettings(st, a)
	out := appState{Slug: a.slug, Name: a.name, Reason: a.reason, Mode: s.connMode(mode, token), On: []string{}}
	switch out.Mode {
	case modeOff:
		out.State, out.Detail, out.Fix = "off", "switched off", "Switch it on in Settings → Connected apps"
		return out
	case modeDirect:
		if token == "" {
			out.State, out.Detail = "not_set_up", "no token yet"
			out.Fix = "Add its address and an API token in Settings → Connected apps"
			if !s.core.Configured() {
				out.Fix += " (or connect Game Builder to Pocket Core: then there is nothing to paste)"
			}
			return out
		}
		out.Via, out.State, out.On, out.Open = modeDirect, "on", append([]string(nil), a.scopes...), public
		return out
	}
	if u == nil || !s.core.Configured() {
		out.State, out.Detail, out.Fix = "no_core", "there is no Pocket Core on this box", "Use a direct connection (Settings → Connected apps → Advanced)"
		return out
	}
	key := fmt.Sprintf("%d %s", u.ID, a.slug)
	s.statusMu.Lock()
	if c, ok := s.statusCache[key]; ok && time.Since(c.at) < 30*time.Second {
		s.statusMu.Unlock()
		return c.st
	}
	s.statusMu.Unlock()
	defer func() {
		if out.State != "error" || out.Detail == "" || !strings.HasPrefix(out.Detail, "Pocket Core did not answer") {
			s.statusMu.Lock()
			s.statusCache[key] = cachedState{st: out, at: time.Now()}
			s.statusMu.Unlock()
		}
	}()
	list, err := s.core.LinkStatus(ctx, u.Email, a.slug)
	if err != nil {
		out.State, out.Detail, out.Fix = "error", "Pocket Core did not answer: "+err.Error(), "Try again in a moment"
		return out
	}
	state := map[string]core.LinkState{}
	for _, l := range list {
		state[l.Scope] = l
	}
	var on, off []string
	reason := ""
	for _, sc := range a.scopes {
		l, ok := state[sc]
		if ok && l.On {
			on = append(on, sc)
			continue
		}
		off = append(off, sc)
		if reason == "" {
			reason = "not_declared"
			if ok {
				reason = l.State
			}
		}
	}
	text := linkReason(a.name, reason)
	if text == "" {
		text = reason
	}
	out.On = on
	if out.On == nil {
		out.On = []string{}
	}
	out.Open = firstNonEmpty(public, s.resolvePublic(ctx, a.slug))
	switch {
	case len(off) == 0:
		out.Via, out.State = modeCore, "on"
		return out
	case len(on) == 0:
		out.State, out.Fix = stateOf(reason)
		out.Detail = text
		return out
	}
	sort.Strings(off)
	out.Via, out.State = modeCore, "partly"
	_, out.Fix = stateOf(reason)
	out.Detail = "partly: " + strings.Join(off, ", ") + " off (" + text + ")"
	return out
}

type cachedState struct {
	st appState
	at time.Time
}

// forgetStatus drops a person's cached app states (their settings changed).
func (s *Server) forgetStatus(uid int64) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	for k := range s.statusCache {
		if strings.HasPrefix(k, fmt.Sprintf("%d ", uid)) {
			delete(s.statusCache, k)
		}
	}
}

// clients carries one person's settings and their connections for one request.
type clients struct {
	s      *Server
	u      *store.User
	st     store.Settings
	notify *notify.Client
	conns  map[string]connResult
	states map[string]appState
}

type connResult struct {
	c   *siblingConn
	err error
}

// clients builds the connections from ONE user's own settings.
func (s *Server) clients(ctx context.Context, uid int64) *clients {
	st, _ := s.store.GetSettings(ctx, uid)
	u := userFrom(ctx)
	if u == nil || u.ID != uid {
		u, _ = s.store.GetUser(ctx, uid)
	}
	return &clients{s: s, u: u, st: st, notify: notify.New(st.NotifyURL, st.NotifyFormat), conns: map[string]connResult{}}
}

func (c *clients) conn(ctx context.Context, a pocketApp, scopes ...string) (*siblingConn, error) {
	sc := append([]string(nil), scopes...)
	sort.Strings(sc)
	key := a.slug + " " + strings.Join(sc, " ")
	if r, ok := c.conns[key]; ok {
		return r.c, r.err
	}
	sib, err := c.s.siblingFor(ctx, c.u, c.st, a, scopes...)
	c.conns[key] = connResult{sib, err}
	return sib, err
}

// status is a person's appState for one app, once per request.
func (c *clients) status(ctx context.Context, a pocketApp) appState {
	if c.states == nil {
		c.states = map[string]appState{}
	}
	if st, ok := c.states[a.slug]; ok {
		return st
	}
	st := c.s.appStatus(ctx, c.u, c.st, a)
	c.states[a.slug] = st
	return st
}

// optional turns "not connected" into (nil, nil).
func optional(sib *siblingConn, err error) (*siblingConn, error) {
	if isNotConnected(err) {
		return nil, nil
	}
	return sib, err
}

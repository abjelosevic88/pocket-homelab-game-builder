package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/gamedoc"
)

// Pocket AI helps: a game from an idea, a rule from a sentence, quiz
// questions on a topic, and ideas for what to add next. What it answers is
// checked here before it reaches a game.

func (s *Server) aiOrFail(w http.ResponseWriter, r *http.Request) (func(system, user string, v any) error, bool) {
	ctx := r.Context()
	uid := actorID(ctx)
	st := s.settingsOf(ctx, uid)
	if !st.AIHelp {
		writeErr(w, http.StatusBadRequest, "AI help is switched off in Settings → AI")
		return nil, false
	}
	c, _, why := s.aiFor(ctx, uid, st)
	if c == nil {
		writeErr(w, http.StatusBadRequest, firstNonEmpty(why, "set up Pocket AI or an OpenAI key in Settings → AI"))
		return nil, false
	}
	select {
	case s.aiSem <- struct{}{}:
	default:
		writeErr(w, http.StatusTooManyRequests, "AI is busy with other requests; try again in a moment")
		return nil, false
	}
	return func(system, user string, v any) error {
		defer func() { <-s.aiSem }()
		return c.JSON(ctx, system, user, v)
	}, true
}

const ideaSystem = `You design small video games for a beginner's game maker. Pick the template that fits the idea best and fill it in. Answer JSON only:
{"template":"platformer|maze|shooter|bricks|catch|quiz","title":"...","intro":"how to play, one or two short sentences","won":"...","lost":"...",
 "emoji":{"<object id>":"one emoji"},"names":{"<object id>":"name"},"bg":"#rrggbb","levels":[["row","row",...]],"questions":[{"q":"...","answers":["right","wrong","wrong"]}]}
Object ids per template — platformer: player, ground, ledge, coin, spikes, snail, flag. maze: player, wall, gem, key, door, ghost, exit. shooter: ship, laser, rock, boom, star. bricks: paddle, ball, brick. catch: basket, apple, bomb, grass. quiz: none (give questions).
Levels only for platformer and maze, as text maps (rows of equal length, 11 rows, 20-48 columns), using only these characters:
platformer: '#' ground, '=' ledge, 'P' player (exactly one), 'c' coin, '^' spikes, 'e' snail, 'F' flag, ' ' empty. The bottom row is all '#'; the player stands on ground; the flag is reachable by jumps of at most 3 cells up and 4 across.
maze: '#' wall, 'P' player (one), '*' gem, 'k' key, 'D' door, 'g' ghost, 'E' exit, ' ' path. Walls all round the edge; every gem, key and the exit reachable; a key before each door.
Give 1-3 levels (or none to keep the template's). Keep words short, friendly, for all ages. Leave out what you don't want to change.`

func (s *Server) handleAIGame(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Idea string `json:"idea"`
	}
	if err := decode(r, &in); err != nil || len(strings.TrimSpace(in.Idea)) < 3 {
		writeErr(w, http.StatusBadRequest, "describe the game you want")
		return
	}
	ask, ok := s.aiOrFail(w, r)
	if !ok {
		return
	}
	var out struct {
		Template string `json:"template"`
		gamedoc.Options
	}
	if err := ask(ideaSystem, "The idea: "+clip(in.Idea, 1500), &out); err != nil {
		writeErr(w, http.StatusBadGateway, "AI: "+err.Error())
		return
	}
	if _, ok := gamedoc.TemplateByID(out.Template); !ok || out.Template == "blank" {
		out.Template = "platformer"
	}
	g, err := s.makeGame(r, actorID(r.Context()), newGame{Template: out.Template, Title: out.Title, Description: clip(strings.TrimSpace(in.Idea), 500), Options: out.Options, Questions: out.Questions})
	if err != nil {
		s.userFail(w, err, "new game")
		return
	}
	writeJSON(w, http.StatusCreated, s.gameOut(g))
}

const ruleSystem = `You turn a sentence into rules for a beginner's game maker. Answer JSON only: {"rules":[{"when":[...],"do":[...],"note":"the sentence"}],"explain":"one short sentence"}.
Conditions ("when", all must hold):
{"t":"start"} {"t":"every","s":2} {"t":"after","s":10} {"t":"key","key":"left|right|up|down|jump|action","how":"press|hold"}
{"t":"touch","a":OBJ,"b":OBJ} {"t":"stomp","a":OBJ,"b":OBJ} (a lands on top of b) {"t":"tap","a":OBJ} {"t":"leaves","a":OBJ,"side":"any|bottom|top|left|right"}
{"t":"var","name":VAR,"op":"=|!=|<|<=|>|>=","value":N} {"t":"count","o":OBJ,"op":...,"value":N} {"t":"chance","p":10} (percent, each second)
Actions ("do", in order):
{"t":"add","name":VAR,"value":N} {"t":"set","name":VAR,"value":N} {"t":"destroy","who":"a|b|OBJ"} (a/b = the things in the touch; OBJ = all of them)
{"t":"create","o":OBJ,"at":"a|b|top|random|start"} {"t":"scene","scene":"next|restart|SCENE"} {"t":"win","text":"..."} {"t":"lose","text":"..."}
{"t":"sound","s":SOUND} {"t":"say","text":"...","s":1.5} {"t":"moveto","who":"a|b","at":"start"} {"t":"jump","who":"a","power":300} {"t":"push","who":"a|b","vx":0,"vy":-200}
{"t":"shake","s":0.3} {"t":"hide","who":"a|b|OBJ"} {"t":"show","who":OBJ}
Use only the object ids, variable names, sound ids and scene ids given. A new variable name is fine for add/set.`

func (s *Server) handleAIRule(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text    string `json:"text"`
		Objects []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"objects"`
		Vars   []string `json:"vars"`
		Sounds []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"sounds"`
		Scenes []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"scenes"`
	}
	if err := decode(r, &in); err != nil || len(strings.TrimSpace(in.Text)) < 3 {
		writeErr(w, http.StatusBadRequest, "say what should happen")
		return
	}
	ask, ok := s.aiOrFail(w, r)
	if !ok {
		return
	}
	var b strings.Builder
	b.WriteString("Objects (id = name): ")
	known := map[string]bool{"a": true, "b": true}
	for _, o := range in.Objects {
		fmt.Fprintf(&b, "%s = %s; ", o.ID, o.Name)
		known[o.ID] = true
	}
	b.WriteString("\nVariables: " + strings.Join(in.Vars, ", "))
	b.WriteString("\nSounds (id = name): ")
	for _, so := range in.Sounds {
		fmt.Fprintf(&b, "%s = %s; ", so.ID, so.Name)
	}
	b.WriteString("\nScenes (id = name): ")
	for _, sc := range in.Scenes {
		fmt.Fprintf(&b, "%s = %s; ", sc.ID, sc.Name)
	}
	b.WriteString("\n\nWhat should happen: " + clip(in.Text, 800))
	var out struct {
		Rules   []gamedoc.Rule `json:"rules"`
		Explain string         `json:"explain"`
	}
	if err := ask(ruleSystem, b.String(), &out); err != nil {
		writeErr(w, http.StatusBadGateway, "AI: "+err.Error())
		return
	}
	rules := []gamedoc.Rule{}
	for _, ru := range out.Rules {
		if len(ru.Do) == 0 || len(ru.Do) > 12 || len(ru.When) > 6 {
			continue
		}
		ok := true
		for _, st := range append(append([]gamedoc.Step{}, ru.When...), ru.Do...) {
			for _, k := range []string{"a", "b", "o"} {
				if v, has := st[k].(string); has && !known[v] {
					ok = false
				}
			}
			if v, has := st["who"].(string); has && !known[v] {
				ok = false
			}
		}
		if !ok {
			continue
		}
		ru.ID, ru.On = gamedoc.NewID(nil), true
		rules = append(rules, ru)
		if len(rules) == 4 {
			break
		}
	}
	if len(rules) == 0 {
		writeErr(w, http.StatusBadGateway, "AI could not make a rule from that with the things in this game; try saying it another way")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules, "explain": out.Explain})
}

func (s *Server) handleAIQuestions(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Topic string `json:"topic"`
		Count int    `json:"count"`
		Lang  string `json:"language"`
	}
	if err := decode(r, &in); err != nil || len(strings.TrimSpace(in.Topic)) < 2 {
		writeErr(w, http.StatusBadRequest, "what should the questions be about?")
		return
	}
	n := clampInt(firstPos(in.Count, 8), 3, 25)
	ask, ok := s.aiOrFail(w, r)
	if !ok {
		return
	}
	var out struct {
		Questions []gamedoc.Question `json:"questions"`
	}
	sys := `You write quiz questions. Answer JSON only: {"questions":[{"q":"question","answers":["the right answer","wrong","wrong"]}]}. The first answer is always the right one; 3 answers each; short; facts only, nothing you are unsure of.`
	user := fmt.Sprintf("%d questions about: %s", n, clip(in.Topic, 400))
	if in.Lang != "" {
		user += "\nWrite them in " + clip(in.Lang, 40) + "."
	}
	if err := ask(sys, user, &out); err != nil {
		writeErr(w, http.StatusBadGateway, "AI: "+err.Error())
		return
	}
	qs := []gamedoc.Question{}
	for _, q := range out.Questions {
		if strings.TrimSpace(q.Q) != "" && len(q.Answers) >= 2 {
			qs = append(qs, q)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"questions": qs})
}

func (s *Server) handleAIIdeas(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	g, err := s.store.GetGame(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "game")
		return
	}
	ask, ok := s.aiOrFail(w, r)
	if !ok {
		return
	}
	var out struct {
		Ideas []struct {
			Title string `json:"title"`
			How   string `json:"how"`
		} `json:"ideas"`
	}
	sys := `You help a beginner make their small game more fun. Answer JSON only: {"ideas":[{"title":"short idea","how":"how to do it in this game maker, in one or two plain sentences: which thing, which behaviour, which rule"}]}. Give 5 ideas that fit this game; no code.`
	if err := ask(sys, clip(gamedoc.Describe(g.Doc), 6000), &out); err != nil {
		writeErr(w, http.StatusBadGateway, "AI: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

var _ = json.Marshal

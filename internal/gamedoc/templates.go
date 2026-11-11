package gamedoc

import (
	"fmt"
	"math/rand"
	"strings"
)

// Template is a ready game to start from.
type Template struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	About string `json:"about"`
	Emoji string `json:"emoji"`
	Size  string `json:"size"`
}

// Templates are what "New game" offers.
var Templates = []Template{
	{"platformer", "Jump and run", "Run, jump on platforms, pick up coins, jump on snails and reach the flag. Two levels.", "🐱", "wide"},
	{"maze", "Maze adventure", "Walk through a maze, find the gems and keys, open doors, keep away from the ghosts.", "💎", "wide"},
	{"shooter", "Space shooter", "Fly a ship, shoot the rocks falling from the sky before they hit you.", "🚀", "tall"},
	{"bricks", "Brick breaker", "Bounce the ball with the paddle and break every brick.", "🧱", "tall"},
	{"catch", "Catch the fruit", "Move the basket, catch the apples, miss the bombs.", "🍎", "tall"},
	{"quiz", "Quiz", "Questions with answers to tap. Write your own questions.", "❓", "wide"},
	{"blank", "Empty game", "A player to move and nothing else: make it your own.", "✨", "wide"},
}

// TemplateByID finds a template.
func TemplateByID(id string) (Template, bool) {
	for _, t := range Templates {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

// Question is one quiz question: the first answer is the right one.
type Question struct {
	Q       string   `json:"q"`
	Answers []string `json:"answers"`
}

// Options change a template: a title, other words, other level maps,
// other looks (emoji per role) — what Pocket AI and other apps fill in.
type Options struct {
	Title     string            `json:"title"`
	Size      string            `json:"size"`
	Intro     string            `json:"intro"`
	Won       string            `json:"won"`
	Lost      string            `json:"lost"`
	Levels    [][]string        `json:"levels"`    // text maps, see each template's legend
	Emoji     map[string]string `json:"emoji"`     // object id → emoji
	Names     map[string]string `json:"names"`     // object id → name
	Colors    map[string]string `json:"colors"`    // object id → colour
	Questions []Question        `json:"questions"` // quiz
	Bg        string            `json:"bg"`
	Seed      int64             `json:"-"`
}

// Legends say which character is which thing in each template's maps.
var Legends = map[string]map[string]string{
	"platformer": {"#": "ground", "=": "ledge", "P": "player", "c": "coin", "^": "spikes", "e": "snail", "F": "flag"},
	"maze":       {"#": "wall", "P": "player", "*": "gem", "k": "key", "D": "door", "g": "ghost", "E": "exit"},
}

func sfx(id, name, preset string) Sound {
	return Sound{ID: id, Name: name, Sfx: map[string]any{"preset": preset}}
}

func st(kv ...any) Step {
	s := Step{}
	for i := 0; i+1 < len(kv); i += 2 {
		s[kv[i].(string)] = kv[i+1]
	}
	return s
}

func beh(t string, kv ...any) Beh {
	b := Beh{T: t}
	if len(kv) > 0 {
		b.P = map[string]any{}
		for i := 0; i+1 < len(kv); i += 2 {
			b.P[kv[i].(string)] = kv[i+1]
		}
	}
	return b
}

type ruleMaker struct{ r *rand.Rand }

func (m ruleMaker) rule(when []Step, do ...Step) Rule {
	return Rule{ID: NewID(m.r), On: true, When: when, Do: do}
}

func w(s ...Step) []Step { return s }

// Build makes a new game from a template.
func Build(id string, o Options) (*Doc, error) {
	t, ok := TemplateByID(id)
	if !ok {
		return nil, fmt.Errorf("there is no template %q", id)
	}
	seed := o.Seed
	if seed == 0 {
		seed = rand.Int63()
	}
	r := rand.New(rand.NewSource(seed))
	size := o.Size
	if _, ok := Sizes[size]; !ok {
		size = t.Size
	}
	d := &Doc{V: 1, Title: firstNon(o.Title, t.Name), Width: Sizes[size][0], Height: Sizes[size][1], Font: "round", Controls: "auto",
		Score: Score{Var: "score", Kind: "high", Label: "Score"}, Vars: []Var{}, Hud: []Hud{}, Rules: []Rule{}, Sounds: []Sound{},
		Ends: Ends{Won: "You did it!", Lost: "Game over"}}
	m := ruleMaker{r}
	switch id {
	case "platformer":
		platformer(d, m, o)
	case "maze":
		maze(d, m, o)
	case "shooter":
		shooter(d, m)
	case "bricks":
		bricks(d, m)
	case "catch":
		catch(d, m)
	case "quiz":
		quiz(d, m, o)
	default:
		blank(d, m)
	}
	if o.Intro != "" {
		d.Intro.Text = o.Intro
	}
	d.Intro.On, d.Intro.Title = true, d.Title
	if d.Intro.Button == "" {
		d.Intro.Button = "Play"
	}
	if o.Won != "" {
		d.Ends.Won = o.Won
	}
	if o.Lost != "" {
		d.Ends.Lost = o.Lost
	}
	for i := range d.Objects {
		ob := &d.Objects[i]
		if e := strings.TrimSpace(o.Emoji[ob.ID]); e != "" && len([]rune(e)) <= 4 {
			ob.Look.Type, ob.Look.Emoji = "emoji", e
		}
		if n := strings.TrimSpace(o.Names[ob.ID]); n != "" {
			ob.Name = clipName(n)
		}
		if c := strings.TrimSpace(o.Colors[ob.ID]); len(c) == 7 && c[0] == '#' {
			ob.Look.Color = c
		}
	}
	if o.Bg != "" && len(o.Bg) == 7 && o.Bg[0] == '#' {
		for i := range d.Scenes {
			d.Scenes[i].Bg = o.Bg
		}
	}
	if len(d.Scenes) > 0 && d.Start == "" {
		d.Start = d.Scenes[0].ID
	}
	return d, nil
}

func clipName(s string) string {
	if r := []rune(s); len(r) > 40 {
		return string(r[:40])
	}
	return s
}

func firstNon(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// levels keeps the given maps that only use the legend and have a player.
func levels(given [][]string, legend map[string]string, def [][]string) [][]string {
	var out [][]string
	for _, lv := range given {
		if len(lv) < 3 || len(lv) > 40 {
			continue
		}
		ok, player := true, false
		for _, row := range lv {
			if len([]rune(row)) > 120 {
				ok = false
			}
			for _, ch := range row {
				if ch == ' ' || ch == '.' {
					continue
				}
				if _, known := legend[string(ch)]; !known {
					ok = false
				}
				if ch == 'P' {
					player = true
				}
			}
		}
		if ok && player {
			out = append(out, lv)
		}
		if len(out) == 5 {
			break
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func legendOf(id string) Legend {
	l := Legend{}
	for k, v := range Legends[id] {
		l[[]rune(k)[0]] = v
	}
	return l
}

func addScenes(d *Doc, maps [][]string, grid int, legend Legend, bg string, r *rand.Rand, music string) {
	for i, rows := range maps {
		sw, sh := MapSize(rows, grid)
		if sw < float64(d.Width) {
			sw = float64(d.Width)
		}
		if sh < float64(d.Height) {
			sh = float64(d.Height)
		}
		d.Scenes = append(d.Scenes, Scene{ID: NewID(r), Name: fmt.Sprintf("Level %d", i+1), W: sw, H: sh, Bg: bg, Grid: grid,
			Items: FromMap(d, rows, grid, legend, r), Rules: []Rule{}, Music: music})
	}
}

func platformer(d *Doc, m ruleMaker, o Options) {
	d.Objects = []Obj{
		{ID: "player", Name: "Cat", Role: "player", W: 22, H: 22, Z: 5, Look: Look{Type: "emoji", Emoji: "🐱"},
			Behaviors: []Beh{beh("platformer", "speed", 150.0, "jump", 380.0, "gravity", 1000.0), beh("camera"), beh("stayin")}},
		{ID: "ground", Name: "Ground", Role: "wall", W: 24, H: 24, Look: Look{Type: "box", Color: "#7a5a3a", Line: "#5fa84a"}, Behaviors: []Beh{beh("solid")}},
		{ID: "ledge", Name: "Ledge", Role: "wall", W: 24, H: 8, Look: Look{Type: "box", Color: "#a77b4f", Round: 3}, Behaviors: []Beh{beh("oneway")}},
		{ID: "coin", Name: "Coin", Role: "coin", W: 14, H: 14, Look: Look{Type: "circle", Color: "#f5c518", Line: "#c99a06"}, Behaviors: []Beh{beh("bob", "amount", 3.0, "speed", 3.0)}},
		{ID: "spikes", Name: "Spikes", Role: "hazard", W: 24, H: 12, Look: Look{Type: "triangle", Color: "#d9480f"}},
		{ID: "snail", Name: "Snail", Role: "enemy", W: 22, H: 18, Z: 4, Look: Look{Type: "emoji", Emoji: "🐌"},
			Behaviors: []Beh{beh("patrol", "speed", 40.0, "axis", "x", "edges", true), beh("gravity", "g", 1000.0)}},
		{ID: "flag", Name: "Flag", Role: "goal", W: 24, H: 24, Look: Look{Type: "emoji", Emoji: "🚩"}},
	}
	d.Vars = []Var{{"score", 0}, {"lives", 3}}
	d.Hud = []Hud{{ID: "h1", Text: "🪙 {score}", X: 10, Y: 10, Size: 16, Color: "#ffffff", Align: "left"}, {ID: "h2", Text: "❤️ {lives}", X: float64(d.Width - 10), Y: 10, Size: 16, Color: "#ffffff", Align: "right"}}
	d.Sounds = []Sound{sfx("coin", "Coin", "coin"), sfx("jump", "Jump", "jump"), sfx("hit", "Ouch", "hit"), sfx("win", "Hooray", "powerup"), sfx("stomp", "Stomp", "blip")}
	d.Intro.Text = "Arrows or A and D to run, Space or Up to jump. Pick up the coins, jump on the snails and reach the flag."
	def := [][]string{{
		"                                        ",
		"                                        ",
		"                          c c c         ",
		"                c         =====         ",
		"           c   ===                   c  ",
		"     c    ===          c          ====  ",
		"    ===           e   ===   c         F ",
		" P        c  c  #####      ===    c  ###",
		"#######  ######     ##      e   ########",
		"#######^^######     ###^^#####  ########",
		"########################################",
	}, {
		"                                               ",
		"                                     c c       ",
		"                   c c c            =====      ",
		"          c       =======       c              ",
		"   c     ===                   ===        c   F",
		"  ===          e          c          e  ===  ##",
		"         c   ######      ===     ######        ",
		" P      ===                 c                  ",
		"####          c       e    ===        c   #####",
		"####^^^^^^^^#####^^^#####^^^^^^^^^#######^#####",
		"###############################################",
	}}
	maps := levels(o.Levels, Legends["platformer"], def)
	addScenes(d, maps, 24, legendOf("platformer"), "#7ec8f0", m.r, "")
	d.Rules = []Rule{
		m.rule(w(st("t", "touch", "a", "player", "b", "coin")), st("t", "add", "name", "score", "value", 1.0), st("t", "destroy", "who", "b"), st("t", "sound", "s", "coin")),
		m.rule(w(st("t", "stomp", "a", "player", "b", "snail")), st("t", "destroy", "who", "b"), st("t", "jump", "who", "a", "power", 300.0), st("t", "add", "name", "score", "value", 5.0), st("t", "sound", "s", "stomp")),
		m.rule(w(st("t", "touch", "a", "player", "b", "snail")), st("t", "add", "name", "lives", "value", -1.0), st("t", "moveto", "who", "a", "at", "start"), st("t", "shake", "s", 0.3), st("t", "sound", "s", "hit")),
		m.rule(w(st("t", "touch", "a", "player", "b", "spikes")), st("t", "add", "name", "lives", "value", -1.0), st("t", "moveto", "who", "a", "at", "start"), st("t", "shake", "s", 0.3), st("t", "sound", "s", "hit")),
		m.rule(w(st("t", "leaves", "a", "player", "side", "bottom")), st("t", "add", "name", "lives", "value", -1.0), st("t", "moveto", "who", "a", "at", "start"), st("t", "sound", "s", "hit")),
		m.rule(w(st("t", "key", "key", "jump", "how", "press")), st("t", "sound", "s", "jump")),
		m.rule(w(st("t", "touch", "a", "player", "b", "flag")), st("t", "sound", "s", "win"), st("t", "scene", "scene", "next")),
		m.rule(w(st("t", "var", "name", "lives", "op", "<=", "value", 0.0)), st("t", "lose", "text", "Out of lives!")),
	}
	d.Rules[5].Note = "Plays the jump sound (the cat jumps by itself: the Platformer behaviour)."
	d.Ends.Won = "You reached the last flag!"
}

func maze(d *Doc, m ruleMaker, o Options) {
	d.Objects = []Obj{
		{ID: "player", Name: "Explorer", Role: "player", W: 18, H: 18, Z: 5, Look: Look{Type: "emoji", Emoji: "🧒"}, Behaviors: []Beh{beh("topdown", "speed", 110.0), beh("camera")}},
		{ID: "wall", Name: "Wall", Role: "wall", W: 24, H: 24, Look: Look{Type: "box", Color: "#3b4a6b", Round: 2}, Behaviors: []Beh{beh("solid")}},
		{ID: "gem", Name: "Gem", Role: "coin", W: 16, H: 16, Look: Look{Type: "emoji", Emoji: "💎"}, Behaviors: []Beh{beh("bob", "amount", 2.0, "speed", 2.5)}},
		{ID: "key", Name: "Key", Role: "coin", W: 16, H: 16, Look: Look{Type: "emoji", Emoji: "🔑"}},
		{ID: "door", Name: "Door", Role: "wall", W: 24, H: 24, Look: Look{Type: "emoji", Emoji: "🚪"}, Behaviors: []Beh{beh("solid")}},
		{ID: "ghost", Name: "Ghost", Role: "enemy", W: 18, H: 18, Z: 4, Look: Look{Type: "emoji", Emoji: "👻"}, Behaviors: []Beh{beh("follow", "target", "player", "speed", 35.0, "range", 140.0)}},
		{ID: "exit", Name: "Exit", Role: "goal", W: 22, H: 22, Look: Look{Type: "emoji", Emoji: "🏁"}},
	}
	d.Vars = []Var{{"score", 0}, {"lives", 3}, {"keys", 0}}
	d.Hud = []Hud{{ID: "h1", Text: "💎 {score}   🔑 {keys}", X: 10, Y: 10, Size: 15, Color: "#ffffff", Align: "left"}, {ID: "h2", Text: "❤️ {lives}", X: float64(d.Width - 10), Y: 10, Size: 15, Color: "#ffffff", Align: "right"}}
	d.Sounds = []Sound{sfx("gem", "Gem", "coin"), sfx("key", "Key", "blip"), sfx("door", "Door", "powerup"), sfx("hit", "Boo", "hit"), sfx("win", "Way out", "powerup")}
	d.Intro.Text = "Arrows or WASD to walk. Pick up every gem, find keys to open doors, keep away from the ghosts and get to the flag."
	def := [][]string{{
		"####################",
		"#P  #     *   #   *#",
		"# # # ### ### # ## #",
		"# #   #*#   #   #  #",
		"# ### # ### ### # ##",
		"#   #   k     # #  #",
		"### ####### # # ## #",
		"#*  #  g  # #    # #",
		"# ### ### # #### # #",
		"#     #*      g  D E",
		"####################",
	}, {
		"########################",
		"#P   #       g       #*#",
		"#### # ##### ####### # #",
		"#  # #   # #  *    # # #",
		"# k# ### # ####### # # #",
		"#  #   #      g    #   #",
		"## ### ########## ## ###",
		"#*   #    *     #  D   #",
		"# ## #### ####### ######",
		"#  #   k   #  g   D   E#",
		"########################",
	}}
	maps := levels(o.Levels, Legends["maze"], def)
	addScenes(d, maps, 24, legendOf("maze"), "#151a2e", m.r, "")
	d.Rules = []Rule{
		m.rule(w(st("t", "touch", "a", "player", "b", "gem")), st("t", "add", "name", "score", "value", 1.0), st("t", "destroy", "who", "b"), st("t", "sound", "s", "gem")),
		m.rule(w(st("t", "touch", "a", "player", "b", "key")), st("t", "add", "name", "keys", "value", 1.0), st("t", "destroy", "who", "b"), st("t", "sound", "s", "key")),
		m.rule(w(st("t", "touch", "a", "player", "b", "door"), st("t", "var", "name", "keys", "op", ">=", "value", 1.0)), st("t", "add", "name", "keys", "value", -1.0), st("t", "destroy", "who", "b"), st("t", "sound", "s", "door")),
		m.rule(w(st("t", "touch", "a", "player", "b", "door"), st("t", "var", "name", "keys", "op", "<", "value", 1.0)), st("t", "say", "text", "This door needs a key.", "s", 1.5)),
		m.rule(w(st("t", "touch", "a", "player", "b", "ghost")), st("t", "add", "name", "lives", "value", -1.0), st("t", "moveto", "who", "a", "at", "start"), st("t", "shake", "s", 0.3), st("t", "sound", "s", "hit")),
		m.rule(w(st("t", "touch", "a", "player", "b", "exit"), st("t", "count", "o", "gem", "op", "=", "value", 0.0)), st("t", "sound", "s", "win"), st("t", "scene", "scene", "next")),
		m.rule(w(st("t", "touch", "a", "player", "b", "exit"), st("t", "count", "o", "gem", "op", ">", "value", 0.0)), st("t", "say", "text", "Find all the gems first!", "s", 1.5)),
		m.rule(w(st("t", "var", "name", "lives", "op", "<=", "value", 0.0)), st("t", "lose", "text", "The ghosts got you.")),
	}
	d.Ends.Won = "You found your way out!"
}

func shooter(d *Doc, m ruleMaker) {
	W, H := float64(d.Width), float64(d.Height)
	d.Objects = []Obj{
		{ID: "ship", Name: "Ship", Role: "player", W: 26, H: 26, Z: 5, Look: Look{Type: "triangle", Color: "#4dd0e1", Line: "#ffffff"},
			Behaviors: []Beh{beh("paddle", "axis", "x", "speed", 220.0, "pointer", true), beh("shooter", "bullet", "laser", "rate", 0.25, "speed", 380.0, "dir", "up", "auto", false), beh("stayin")}},
		{ID: "laser", Name: "Laser", Role: "bullet", W: 4, H: 12, Look: Look{Type: "box", Color: "#b2ff59", Round: 2}, Behaviors: []Beh{beh("move", "vx", 0.0, "vy", -380.0, "gone", true)}},
		{ID: "rock", Name: "Rock", Role: "enemy", W: 26, H: 26, Look: Look{Type: "emoji", Emoji: "🪨"}, Behaviors: []Beh{beh("move", "vx", 0.0, "vy", 90.0, "spread", 50.0, "gone", true), beh("spin", "speed", 60.0)}},
		{ID: "boom", Name: "Boom", Role: "deco", W: 28, H: 28, Z: 6, Look: Look{Type: "emoji", Emoji: "💥"}, Behaviors: []Beh{beh("fade", "after", 0.35)}},
		{ID: "star", Name: "Star", Role: "deco", W: 3, H: 3, Z: -1, Look: Look{Type: "circle", Color: "#ffffff"}, Behaviors: []Beh{beh("move", "vx", 0.0, "vy", 30.0, "wrap", true)}},
	}
	d.Vars = []Var{{"score", 0}, {"lives", 3}}
	d.Hud = []Hud{{ID: "h1", Text: "{score}", X: 10, Y: 10, Size: 18, Color: "#ffffff", Align: "left"}, {ID: "h2", Text: "❤️ {lives}", X: W - 10, Y: 10, Size: 15, Color: "#ffffff", Align: "right"}}
	d.Sounds = []Sound{sfx("laser", "Laser", "laser"), sfx("boom", "Boom", "explode"), sfx("hit", "Hit", "hit")}
	d.Controls = "sides"
	d.Intro.Text = "Move with the arrows (or drag), shoot with Space (or the fire button). Reach 300 points."
	sc := Scene{ID: NewID(m.r), Name: "Space", W: W, H: H, Bg: "#0b1026", Grid: 16, Rules: []Rule{}}
	sc.Items = append(sc.Items, Inst{ID: NewID(m.r), O: "ship", X: W/2 - 13, Y: H - 60})
	for i := 0; i < 40; i++ {
		sc.Items = append(sc.Items, Inst{ID: NewID(m.r), O: "star", X: float64(m.r.Intn(int(W))), Y: float64(m.r.Intn(int(H)))})
	}
	d.Scenes = []Scene{sc}
	d.Rules = []Rule{
		m.rule(w(st("t", "every", "s", 0.8)), st("t", "create", "o", "rock", "at", "top")),
		m.rule(w(st("t", "key", "key", "action", "how", "press")), st("t", "sound", "s", "laser")),
		m.rule(w(st("t", "touch", "a", "laser", "b", "rock")), st("t", "create", "o", "boom", "at", "b"), st("t", "destroy", "who", "a"), st("t", "destroy", "who", "b"), st("t", "add", "name", "score", "value", 10.0), st("t", "sound", "s", "boom")),
		m.rule(w(st("t", "touch", "a", "rock", "b", "ship")), st("t", "create", "o", "boom", "at", "a"), st("t", "destroy", "who", "a"), st("t", "add", "name", "lives", "value", -1.0), st("t", "shake", "s", 0.4), st("t", "sound", "s", "hit")),
		m.rule(w(st("t", "var", "name", "lives", "op", "<=", "value", 0.0)), st("t", "lose", "text", "Your ship is gone.")),
		m.rule(w(st("t", "var", "name", "score", "op", ">=", "value", 300.0)), st("t", "win", "text", "The sky is safe!")),
	}
}

func bricks(d *Doc, m ruleMaker) {
	W, H := float64(d.Width), float64(d.Height)
	d.Objects = []Obj{
		{ID: "paddle", Name: "Paddle", Role: "paddle", W: 56, H: 10, Z: 5, Look: Look{Type: "box", Color: "#e9ecef", Round: 5}, Behaviors: []Beh{beh("paddle", "axis", "x", "speed", 300.0, "pointer", true), beh("stayin"), beh("solid")}},
		{ID: "ball", Name: "Ball", Role: "ball", W: 10, H: 10, Z: 6, Look: Look{Type: "circle", Color: "#ffd43b"}, Behaviors: []Beh{beh("bounce", "speed", 230.0, "angle", -60.0, "floor", false)}},
		{ID: "brick", Name: "Brick", Role: "wall", W: 28, H: 12, Look: Look{Type: "box", Color: "#ff6b6b", Round: 2}, Behaviors: []Beh{beh("solid")}},
	}
	d.Vars = []Var{{"score", 0}, {"lives", 3}}
	d.Hud = []Hud{{ID: "h1", Text: "{score}", X: 10, Y: 8, Size: 15, Color: "#ffffff", Align: "left"}, {ID: "h2", Text: "❤️ {lives}", X: W - 10, Y: 8, Size: 15, Color: "#ffffff", Align: "right"}}
	d.Sounds = []Sound{sfx("blip", "Blip", "blip"), sfx("lost", "Lost", "hit")}
	d.Controls = "sides"
	d.Intro.Text = "Move the paddle with the arrows, or drag. Break every brick and don't let the ball fall."
	sc := Scene{ID: NewID(m.r), Name: "Wall", W: W, H: H, Bg: "#1b1f3a", Grid: 10, Rules: []Rule{}}
	colors := []string{"#ff6b6b", "#ffa94d", "#ffd43b", "#69db7c", "#4dabf7", "#b197fc"}
	cols := int((W - 20) / 31)
	left := (W - float64(cols)*31 + 3) / 2
	for row := 0; row < 6; row++ {
		for c := 0; c < cols; c++ {
			sc.Items = append(sc.Items, Inst{ID: NewID(m.r), O: "brick", X: left + float64(c)*31, Y: 40 + float64(row)*16, Color: colors[row]})
		}
	}
	sc.Items = append(sc.Items, Inst{ID: NewID(m.r), O: "paddle", X: W/2 - 28, Y: H - 40}, Inst{ID: NewID(m.r), O: "ball", X: W/2 - 5, Y: H - 70})
	d.Scenes = []Scene{sc}
	d.Rules = []Rule{
		m.rule(w(st("t", "touch", "a", "ball", "b", "brick")), st("t", "destroy", "who", "b"), st("t", "add", "name", "score", "value", 1.0), st("t", "sound", "s", "blip")),
		m.rule(w(st("t", "touch", "a", "ball", "b", "paddle")), st("t", "sound", "s", "blip")),
		m.rule(w(st("t", "leaves", "a", "ball", "side", "bottom")), st("t", "add", "name", "lives", "value", -1.0), st("t", "moveto", "who", "a", "at", "start"), st("t", "sound", "s", "lost"), st("t", "say", "text", "Watch the ball!", "s", 1.0)),
		m.rule(w(st("t", "count", "o", "brick", "op", "=", "value", 0.0)), st("t", "win", "text", "Every brick is broken!")),
		m.rule(w(st("t", "var", "name", "lives", "op", "<=", "value", 0.0)), st("t", "lose", "text", "No balls left.")),
	}
}

func catch(d *Doc, m ruleMaker) {
	W, H := float64(d.Width), float64(d.Height)
	d.Objects = []Obj{
		{ID: "basket", Name: "Basket", Role: "player", W: 40, H: 30, Z: 5, Look: Look{Type: "emoji", Emoji: "🧺"}, Behaviors: []Beh{beh("paddle", "axis", "x", "speed", 260.0, "pointer", true), beh("stayin")}},
		{ID: "apple", Name: "Apple", Role: "coin", W: 24, H: 24, Look: Look{Type: "emoji", Emoji: "🍎"}, Behaviors: []Beh{beh("move", "vx", 0.0, "vy", 120.0, "spread", 40.0, "gone", false)}},
		{ID: "bomb", Name: "Bomb", Role: "hazard", W: 24, H: 24, Look: Look{Type: "emoji", Emoji: "💣"}, Behaviors: []Beh{beh("move", "vx", 0.0, "vy", 150.0, "gone", true)}},
		{ID: "grass", Name: "Grass", Role: "deco", W: W, H: 20, Z: -1, Look: Look{Type: "box", Color: "#5fa84a"}},
	}
	d.Vars = []Var{{"score", 0}, {"lives", 3}}
	d.Hud = []Hud{{ID: "h1", Text: "🍎 {score}", X: 10, Y: 10, Size: 16, Color: "#1b1f3a", Align: "left"}, {ID: "h2", Text: "❤️ {lives}", X: W - 10, Y: 10, Size: 16, Color: "#1b1f3a", Align: "right"}}
	d.Sounds = []Sound{sfx("catch", "Catch", "coin"), sfx("boom", "Boom", "explode"), sfx("miss", "Miss", "hit")}
	d.Controls = "sides"
	d.Intro.Text = "Move the basket with the arrows or drag it. Catch 30 apples. Don't catch the bombs, don't drop the apples."
	sc := Scene{ID: NewID(m.r), Name: "Orchard", W: W, H: H, Bg: "#bfe6ff", Grid: 10, Rules: []Rule{}}
	sc.Items = []Inst{{ID: NewID(m.r), O: "grass", X: 0, Y: H - 20}, {ID: NewID(m.r), O: "basket", X: W/2 - 20, Y: H - 52}}
	d.Scenes = []Scene{sc}
	d.Rules = []Rule{
		m.rule(w(st("t", "every", "s", 0.9)), st("t", "create", "o", "apple", "at", "top")),
		m.rule(w(st("t", "every", "s", 2.6)), st("t", "create", "o", "bomb", "at", "top")),
		m.rule(w(st("t", "touch", "a", "basket", "b", "apple")), st("t", "destroy", "who", "b"), st("t", "add", "name", "score", "value", 1.0), st("t", "sound", "s", "catch")),
		m.rule(w(st("t", "touch", "a", "basket", "b", "bomb")), st("t", "destroy", "who", "b"), st("t", "add", "name", "lives", "value", -1.0), st("t", "shake", "s", 0.4), st("t", "sound", "s", "boom")),
		m.rule(w(st("t", "leaves", "a", "apple", "side", "bottom")), st("t", "destroy", "who", "a"), st("t", "add", "name", "lives", "value", -1.0), st("t", "sound", "s", "miss")),
		m.rule(w(st("t", "var", "name", "score", "op", ">=", "value", 30.0)), st("t", "win", "text", "A full basket!")),
		m.rule(w(st("t", "var", "name", "lives", "op", "<=", "value", 0.0)), st("t", "lose", "text", "Too many apples lost.")),
	}
}

// DefaultQuestions are the quiz template's questions.
var DefaultQuestions = []Question{
	{Q: "How many legs does a spider have?", Answers: []string{"Eight", "Six", "Ten"}},
	{Q: "Which planet is the largest?", Answers: []string{"Jupiter", "Mars", "Venus"}},
	{Q: "What do bees make?", Answers: []string{"Honey", "Milk", "Silk"}},
	{Q: "How many days are in a leap year?", Answers: []string{"366", "365", "364"}},
	{Q: "Which is the longest river in Africa?", Answers: []string{"The Nile", "The Congo", "The Niger"}},
}

func quiz(d *Doc, m ruleMaker, o Options) {
	W, H := float64(d.Width), float64(d.Height)
	d.Objects = []Obj{
		{ID: "question", Name: "Question", Role: "text", W: W - 40, H: 60, Look: Look{Type: "text", Color: "#ffffff", Size: 20, Bold: true}, Text: "Question"},
		{ID: "right", Name: "Right answer", Role: "button", W: W - 120, H: 34, Look: Look{Type: "box", Color: "#4c6ef5", Round: 10, Size: 16, TextOn: "#ffffff"}, Text: "Answer"},
		{ID: "wrong", Name: "Wrong answer", Role: "button", W: W - 120, H: 34, Look: Look{Type: "box", Color: "#4c6ef5", Round: 10, Size: 16, TextOn: "#ffffff"}, Text: "Answer"},
	}
	d.Vars = []Var{{"score", 0}}
	d.Hud = []Hud{{ID: "h1", Text: "Score {score}", X: W - 10, Y: 10, Size: 14, Color: "#ffffff", Align: "right"}}
	d.Sounds = []Sound{sfx("right", "Right", "coin"), sfx("wrong", "Wrong", "hit")}
	d.Controls = "none"
	d.Intro.Text = "Tap the right answer."
	qs := []Question{}
	for _, q := range o.Questions {
		if strings.TrimSpace(q.Q) == "" || len(q.Answers) < 2 {
			continue
		}
		qs = append(qs, q)
		if len(qs) == 30 {
			break
		}
	}
	if len(qs) == 0 {
		qs = DefaultQuestions
	}
	for i, q := range qs {
		sc := Scene{ID: NewID(m.r), Name: fmt.Sprintf("Question %d", i+1), W: W, H: H, Bg: "#364fc7", Grid: 10, Rules: []Rule{}}
		sc.Items = append(sc.Items, Inst{ID: NewID(m.r), O: "question", X: 20, Y: 30, Text: strings.TrimSpace(q.Q)})
		answers := q.Answers
		if len(answers) > 4 {
			answers = answers[:4]
		}
		order := m.r.Perm(len(answers))
		top := 30 + 60 + 20.0
		gap := (H - top - 20) / float64(len(answers))
		if gap > 46 {
			gap = 46
		}
		for k, idx := range order {
			obj := "wrong"
			if idx == 0 {
				obj = "right"
			}
			sc.Items = append(sc.Items, Inst{ID: NewID(m.r), O: obj, X: 60, Y: top + float64(k)*gap, Text: strings.TrimSpace(answers[idx])})
		}
		d.Scenes = append(d.Scenes, sc)
	}
	d.Rules = []Rule{
		m.rule(w(st("t", "tap", "a", "right")), st("t", "add", "name", "score", "value", 1.0), st("t", "sound", "s", "right"), st("t", "say", "text", "Right!", "s", 0.8), st("t", "scene", "scene", "next", "wait", 0.8)),
		m.rule(w(st("t", "tap", "a", "wrong")), st("t", "sound", "s", "wrong"), st("t", "say", "text", "Not that one.", "s", 0.8), st("t", "scene", "scene", "next", "wait", 0.8)),
	}
	d.Ends.Won = "That was the last question."
	d.Score.Label = "Right answers"
}

func blank(d *Doc, m ruleMaker) {
	W, H := float64(d.Width), float64(d.Height)
	d.Objects = []Obj{{ID: "player", Name: "Player", Role: "player", W: 24, H: 24, Z: 5, Look: Look{Type: "emoji", Emoji: "🙂"}, Behaviors: []Beh{beh("topdown", "speed", 120.0), beh("stayin")}}}
	d.Vars = []Var{{"score", 0}}
	d.Intro.Text = "Arrows or WASD to move."
	d.Scenes = []Scene{{ID: NewID(m.r), Name: "Scene 1", W: W, H: H, Bg: "#2b3a55", Grid: 24, Rules: []Rule{},
		Items: []Inst{{ID: NewID(m.r), O: "player", X: W/2 - 12, Y: H/2 - 12}}}}
}

// MazeLevel turns a Pocket Puzzlecraft maze (cells with wall bits N=1 E=2
// S=4 W=8) into a maze map: each cell becomes a 2×2 block of wall and path.
func MazeLevel(w, h int, walls []int, start, end [2]int, r *rand.Rand) []string {
	if w <= 0 || h <= 0 || len(walls) < w*h {
		return nil
	}
	gw, gh := 2*w+1, 2*h+1
	grid := make([][]rune, gh)
	for y := range grid {
		grid[y] = []rune(strings.Repeat("#", gw))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := walls[y*w+x]
			grid[2*y+1][2*x+1] = ' '
			if c&2 == 0 && x+1 < w {
				grid[2*y+1][2*x+2] = ' '
			}
			if c&4 == 0 && y+1 < h {
				grid[2*y+2][2*x+1] = ' '
			}
		}
	}
	grid[2*start[1]+1][2*start[0]+1] = 'P'
	grid[2*end[1]+1][2*end[0]+1] = 'E'
	// a few gems in dead ends
	gems := 0
	for y := 0; y < h && gems < 6; y++ {
		for x := 0; x < w && gems < 6; x++ {
			c := walls[y*w+x]
			open := 0
			for _, b := range []int{1, 2, 4, 8} {
				if c&b == 0 {
					open++
				}
			}
			if open == 1 && grid[2*y+1][2*x+1] == ' ' && r.Intn(2) == 0 {
				grid[2*y+1][2*x+1] = '*'
				gems++
			}
		}
	}
	out := make([]string, gh)
	for i, row := range grid {
		out[i] = string(row)
	}
	return out
}

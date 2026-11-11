// Package gamedoc is the shape of a game: the document the editor saves,
// the player runs and the templates, Pocket AI and other apps make. The web
// side (web/src/engine/types.ts) has the same shape; the player is lenient
// about anything missing, so the server only checks what would break it.
package gamedoc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
)

// Doc is a whole game.
type Doc struct {
	V        int      `json:"v"`
	Title    string   `json:"title"`
	Width    int      `json:"width"`  // the screen, in game pixels
	Height   int      `json:"height"` //
	Pixel    bool     `json:"pixel"`  // crisp, blocky pixels
	Font     string   `json:"font"`   // round | pixel | plain
	Start    string   `json:"start"`  // the first scene's id
	Controls string   `json:"controls"`
	Score    Score    `json:"score"`
	Intro    Intro    `json:"intro"`
	Vars     []Var    `json:"vars"`
	Hud      []Hud    `json:"hud"`
	Objects  []Obj    `json:"objects"`
	Scenes   []Scene  `json:"scenes"`
	Rules    []Rule   `json:"rules"` // in every scene
	Sounds   []Sound  `json:"sounds"`
	Ends     Ends     `json:"ends"`
	Words    []string `json:"-"`
}

// Score says which number is the score and whether more is better.
type Score struct {
	Var   string `json:"var"`   // a variable's name; '' = time
	Kind  string `json:"kind"`  // high | low | time
	Label string `json:"label"` // "Score", "Coins"
}

// Intro is the screen before the game starts.
type Intro struct {
	On     bool   `json:"on"`
	Title  string `json:"title"`
	Text   string `json:"text"`
	Button string `json:"button"`
}

// Ends are the words on the won and lost screens.
type Ends struct {
	Won  string `json:"won"`
	Lost string `json:"lost"`
}

// Var is a number the rules count with (score, lives, keys).
type Var struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// Hud is a line of text on the screen ("Score {score}").
type Hud struct {
	ID    string  `json:"id"`
	Text  string  `json:"text"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Size  float64 `json:"size"`
	Color string  `json:"color"`
	Align string  `json:"align"` // left | center | right
}

// Obj is a kind of thing in the game (the player, a coin, a wall).
type Obj struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Role      string  `json:"role"` // a hint for the editor: player | wall | coin | enemy | goal | hazard | text | deco | bullet | ball | paddle | button
	W         float64 `json:"w"`
	H         float64 `json:"h"`
	Look      Look    `json:"look"`
	Behaviors []Beh   `json:"behaviors"`
	Z         int     `json:"z"`
	Text      string  `json:"text,omitempty"`
}

// Look is how a thing is drawn.
type Look struct {
	Type   string  `json:"type"` // box | circle | triangle | emoji | image | text | none
	Color  string  `json:"color,omitempty"`
	Line   string  `json:"line,omitempty"`
	Emoji  string  `json:"emoji,omitempty"`
	Image  int64   `json:"image,omitempty"` // an asset id
	FPS    float64 `json:"fps,omitempty"`
	Round  float64 `json:"round,omitempty"`
	Size   float64 `json:"size,omitempty"` // text size
	Bold   bool    `json:"bold,omitempty"`
	TextOn string  `json:"textColor,omitempty"`
}

// Beh is one behaviour with its settings ({t:"platformer", p:{speed:160}}).
type Beh struct {
	T string         `json:"t"`
	P map[string]any `json:"p,omitempty"`
}

// Scene is a level (or a title screen, or a question).
type Scene struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	Bg      string  `json:"bg"`
	BgImage int64   `json:"bgImage,omitempty"`
	Music   string  `json:"music,omitempty"` // a sound's id
	Grid    int     `json:"grid"`
	Items   []Inst  `json:"items"`
	Rules   []Rule  `json:"rules"`
	Camera  string  `json:"camera,omitempty"` // an object's id to follow
}

// Inst is one thing placed in a scene.
type Inst struct {
	ID    string  `json:"id"`
	O     string  `json:"o"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w,omitempty"`
	H     float64 `json:"h,omitempty"`
	Flip  bool    `json:"flip,omitempty"`
	Text  string  `json:"text,omitempty"`
	Color string  `json:"color,omitempty"`
}

// Rule is "when … do …".
type Rule struct {
	ID   string `json:"id"`
	On   bool   `json:"on"`
	Once bool   `json:"once,omitempty"`
	Note string `json:"note,omitempty"`
	When []Step `json:"when"`
	Do   []Step `json:"do"`
}

// Step is one condition or action ({t:"touch", a:"player", b:"coin"}).
type Step map[string]any

// Sound is a sound effect: made here (sfx, from a preset) or a file (asset).
type Sound struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Sfx   map[string]any `json:"sfx,omitempty"`
	Asset int64          `json:"asset,omitempty"`
	Loop  bool           `json:"loop,omitempty"`
}

// Sizes are the screen shapes people choose from.
var Sizes = map[string][2]int{"wide": {480, 270}, "tall": {270, 480}, "square": {360, 360}, "big": {640, 360}}

// MaxBytes is the most a game document may weigh (pictures and sounds are
// separate files).
const MaxBytes = 4 << 20

var reID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)

// Check says whether a document can be played: a JSON object with at least
// one scene, a sensible screen, ids that are ids. It returns the title.
func Check(raw []byte) (string, error) {
	if len(raw) > MaxBytes {
		return "", fmt.Errorf("the game is too big (%d KB; at most %d KB without pictures and sounds)", len(raw)/1024, MaxBytes/1024)
	}
	var d struct {
		V      int    `json:"v"`
		Title  string `json:"title"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
		Scenes []struct {
			ID string `json:"id"`
		} `json:"scenes"`
		Objects []struct {
			ID string `json:"id"`
		} `json:"objects"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return "", errors.New("this is not a game (it is not JSON)")
	}
	if d.V != 1 {
		return "", errors.New("this is not a Pocket Game Builder game (v must be 1)")
	}
	if len(d.Scenes) == 0 {
		return "", errors.New("a game needs at least one scene")
	}
	if d.Width < 80 || d.Height < 80 || d.Width > 1920 || d.Height > 1920 {
		return "", errors.New("the screen must be between 80 and 1920 pixels each way")
	}
	seen := map[string]bool{}
	for _, s := range d.Scenes {
		if !reID.MatchString(s.ID) || seen["s"+s.ID] {
			return "", fmt.Errorf("scene id %q is missing or used twice", s.ID)
		}
		seen["s"+s.ID] = true
	}
	for _, o := range d.Objects {
		if !reID.MatchString(o.ID) || seen["o"+o.ID] {
			return "", fmt.Errorf("object id %q is missing or used twice", o.ID)
		}
		seen["o"+o.ID] = true
	}
	return strings.TrimSpace(d.Title), nil
}

// AssetIDs lists the pictures and sounds a document uses (look.image,
// scene.bgImage, sound.asset), so a shared game can load only those.
func AssetIDs(raw []byte) []int64 {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	seen := map[int64]bool{}
	var out []int64
	var walk func(x any)
	walk = func(x any) {
		switch t := x.(type) {
		case map[string]any:
			for k, val := range t {
				if k == "image" || k == "bgImage" || k == "asset" {
					if f, ok := val.(float64); ok && f > 0 && !seen[int64(f)] {
						seen[int64(f)] = true
						out = append(out, int64(f))
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
	return out
}

// SetTitle changes the title inside a document.
func SetTitle(raw []byte, title string) []byte {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return raw
	}
	m["title"] = title
	b, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return b
}

// Texts are the words a player reads in a game, for translating: the title,
// intro, end screens, HUD lines, text objects, placed texts and "say" lines.
// Replace puts translated words back in the same order.
func Texts(raw []byte) []string {
	out := []string{}
	_, _ = walkTexts(raw, func(s string) string { out = append(out, s); return s })
	return out
}

// Replace swaps every text Texts found, in order, for the matching one in tr.
func Replace(raw []byte, tr []string) ([]byte, error) {
	i := 0
	return walkTexts(raw, func(s string) string {
		if i < len(tr) && strings.TrimSpace(tr[i]) != "" {
			s = tr[i]
		}
		i++
		return s
	})
}

func walkTexts(raw []byte, fn func(string) string) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw, err
	}
	do := func(obj map[string]any, key string) {
		if s, ok := obj[key].(string); ok && strings.TrimSpace(s) != "" {
			obj[key] = fn(s)
		}
	}
	list := func(v any) []map[string]any {
		var out []map[string]any
		if a, ok := v.([]any); ok {
			for _, x := range a {
				if o, ok := x.(map[string]any); ok {
					out = append(out, o)
				}
			}
		}
		return out
	}
	do(m, "title")
	if in, ok := m["intro"].(map[string]any); ok {
		do(in, "title")
		do(in, "text")
		do(in, "button")
	}
	if e, ok := m["ends"].(map[string]any); ok {
		do(e, "won")
		do(e, "lost")
	}
	if sc, ok := m["score"].(map[string]any); ok {
		do(sc, "label")
	}
	for _, h := range list(m["hud"]) {
		do(h, "text")
	}
	for _, o := range list(m["objects"]) {
		do(o, "text")
	}
	steps := func(rules any) {
		for _, r := range list(rules) {
			for _, st := range list(r["do"]) {
				if t, _ := st["t"].(string); t == "say" || t == "win" || t == "lose" {
					do(st, "text")
				}
			}
		}
	}
	steps(m["rules"])
	for _, s := range list(m["scenes"]) {
		for _, it := range list(s["items"]) {
			do(it, "text")
		}
		steps(s["rules"])
	}
	return json.Marshal(m)
}

// ---- building documents (templates, Pocket AI, other apps) ----

// NewID is a short id for scenes, objects, rules and placed things.
func NewID(r *rand.Rand) string {
	const abc = "abcdefghijkmnpqrstuvwxyz23456789"
	b := make([]byte, 8)
	for i := range b {
		if r != nil {
			b[i] = abc[r.Intn(len(abc))]
		} else {
			b[i] = abc[rand.Intn(len(abc))]
		}
	}
	return string(b)
}

// Legend maps a character of a level map to an object id.
type Legend map[rune]string

// FromMap places things from a text map: one character per grid cell, " "
// and "." are empty. Things smaller than a cell sit at the bottom middle
// of it (so coins float and players stand on the ground).
func FromMap(d *Doc, rows []string, grid int, legend Legend, r *rand.Rand) []Inst {
	sizes := map[string][2]float64{}
	for _, o := range d.Objects {
		sizes[o.ID] = [2]float64{o.W, o.H}
	}
	var out []Inst
	for y, row := range rows {
		for x, ch := range []rune(row) {
			id, ok := legend[ch]
			if !ok {
				continue
			}
			sz := sizes[id]
			g := float64(grid)
			px := float64(x)*g + (g-sz[0])/2
			py := float64(y)*g + (g - sz[1])
			if sz[1] > g {
				py = float64(y) * g
			}
			out = append(out, Inst{ID: NewID(r), O: id, X: px, Y: py})
		}
	}
	return out
}

// MapSize is the world size of a text map.
func MapSize(rows []string, grid int) (float64, float64) {
	w := 0
	for _, r := range rows {
		if n := len([]rune(r)); n > w {
			w = n
		}
	}
	return float64(w * grid), float64(len(rows) * grid)
}

// Marshal turns a document into JSON.
func (d *Doc) Marshal() []byte {
	b, _ := json.Marshal(d)
	return b
}

// Obj finds an object by id.
func (d *Doc) Obj(id string) *Obj {
	for i := range d.Objects {
		if d.Objects[i].ID == id {
			return &d.Objects[i]
		}
	}
	return nil
}

// ---- a game in words (Pocket Notes, Pocket AI) ----

// Describe writes a game down in Markdown: what to do, its levels, its
// things and their behaviours, and every rule as a sentence.
func Describe(raw []byte) string {
	var d Doc
	if json.Unmarshal(raw, &d) != nil {
		return ""
	}
	names := map[string]string{}
	for _, o := range d.Objects {
		names[o.ID] = o.Name
	}
	for _, s := range d.Sounds {
		names["sound:"+s.ID] = s.Name
	}
	scenes := map[string]string{}
	for _, s := range d.Scenes {
		scenes[s.ID] = s.Name
	}
	var b strings.Builder
	if d.Intro.Text != "" {
		b.WriteString("**How to play:** " + d.Intro.Text + "\n\n")
	}
	fmt.Fprintf(&b, "Screen %d × %d.", d.Width, d.Height)
	if len(d.Vars) > 0 {
		var vs []string
		for _, v := range d.Vars {
			vs = append(vs, fmt.Sprintf("%s (starts at %g)", v.Name, v.Value))
		}
		b.WriteString(" Numbers: " + strings.Join(vs, ", ") + ".")
	}
	b.WriteString("\n\n## Levels\n\n")
	for i, s := range d.Scenes {
		count := map[string]int{}
		for _, it := range s.Items {
			count[it.O]++
		}
		var parts []string
		for _, o := range d.Objects {
			if n := count[o.ID]; n > 0 {
				parts = append(parts, fmt.Sprintf("%d × %s", n, o.Name))
			}
		}
		fmt.Fprintf(&b, "%d. **%s** — %s\n", i+1, s.Name, strings.Join(parts, ", "))
		for _, r := range s.Rules {
			b.WriteString("   - " + RuleText(r, names, scenes) + "\n")
		}
	}
	b.WriteString("\n## Things\n\n")
	for _, o := range d.Objects {
		var bs []string
		for _, be := range o.Behaviors {
			bs = append(bs, behWords[be.T])
		}
		look := o.Look.Emoji
		if look == "" {
			look = o.Look.Type
		}
		fmt.Fprintf(&b, "- **%s** (%s)", o.Name, look)
		if len(bs) > 0 {
			b.WriteString(": " + strings.Join(bs, ", "))
		}
		b.WriteString("\n")
	}
	if len(d.Rules) > 0 {
		b.WriteString("\n## Rules\n\n")
		for _, r := range d.Rules {
			b.WriteString("- " + RuleText(r, names, scenes) + "\n")
		}
	}
	return b.String()
}

var behWords = map[string]string{"platformer": "runs and jumps", "topdown": "walks in four directions", "solid": "nothing goes through it", "oneway": "can be jumped through from below",
	"gravity": "falls", "patrol": "walks back and forth", "follow": "follows", "move": "moves by itself", "bounce": "bounces", "paddle": "moves side to side",
	"shooter": "shoots", "wrap": "comes back on the other side", "stayin": "stays on the screen", "drag": "can be dragged", "spin": "spins", "bob": "floats up and down",
	"camera": "the camera follows it", "fade": "disappears after a moment"}

// RuleText is a rule as a sentence: "When Cat touches Coin: add 1 to score, remove the Coin."
func RuleText(r Rule, names, scenes map[string]string) string {
	n := func(v any) string {
		s, _ := v.(string)
		if x, ok := names[s]; ok {
			return x
		}
		return s
	}
	var when, do []string
	for _, c := range r.When {
		switch c["t"] {
		case "start":
			when = append(when, "the level starts")
		case "every":
			when = append(when, fmt.Sprintf("every %v seconds", c["s"]))
		case "after":
			when = append(when, fmt.Sprintf("after %v seconds", c["s"]))
		case "key":
			when = append(when, fmt.Sprintf("%v is pressed", c["key"]))
		case "touch":
			when = append(when, n(c["a"])+" touches "+n(c["b"]))
		case "stomp":
			when = append(when, n(c["a"])+" jumps on "+n(c["b"]))
		case "tap":
			when = append(when, n(c["a"])+" is tapped")
		case "var":
			when = append(when, fmt.Sprintf("%v %v %v", c["name"], c["op"], c["value"]))
		case "count":
			when = append(when, fmt.Sprintf("the number of %s %v %v", n(c["o"]), c["op"], c["value"]))
		case "leaves":
			when = append(when, n(c["a"])+" leaves the level")
		case "chance":
			when = append(when, fmt.Sprintf("by chance (%v%%)", c["p"]))
		default:
			when = append(when, fmt.Sprint(c["t"]))
		}
	}
	who := func(v any, b string) string {
		switch v {
		case "a":
			return "it"
		case "b":
			return "the " + b
		}
		return "every " + n(v)
	}
	b := ""
	for _, c := range r.When {
		if c["t"] == "touch" || c["t"] == "stomp" {
			b = n(c["b"])
			break
		}
	}
	for _, a := range r.Do {
		switch a["t"] {
		case "add":
			do = append(do, fmt.Sprintf("add %v to %v", a["value"], a["name"]))
		case "set":
			do = append(do, fmt.Sprintf("set %v to %v", a["name"], a["value"]))
		case "destroy":
			do = append(do, "remove "+who(a["who"], b))
		case "create":
			do = append(do, "make a "+n(a["o"]))
		case "scene":
			if a["scene"] == "next" {
				do = append(do, "go to the next level")
			} else if a["scene"] == "restart" {
				do = append(do, "start the level again")
			} else {
				do = append(do, "go to "+scenes[fmt.Sprint(a["scene"])])
			}
		case "win":
			do = append(do, "win")
		case "lose":
			do = append(do, "lose")
		case "sound":
			do = append(do, "play "+n("sound:"+fmt.Sprint(a["s"])))
		case "say":
			do = append(do, fmt.Sprintf("say “%v”", a["text"]))
		case "moveto":
			do = append(do, "put "+who(a["who"], b)+" back at the start")
		case "jump":
			do = append(do, "make "+who(a["who"], b)+" jump")
		case "shake":
			do = append(do, "shake the screen")
		default:
			do = append(do, fmt.Sprint(a["t"]))
		}
	}
	if len(when) == 0 {
		when = []string{"always"}
	}
	out := "When " + strings.Join(when, " and ") + ": " + strings.Join(do, ", ") + "."
	if !r.On {
		out += " (off)"
	}
	return out
}

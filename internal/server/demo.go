package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"strconv"
	"time"

	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/gamedoc"
	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/store"
)

// seedDemo fills the owner's account with games, a few drawn sprites,
// plays by friends over two months, best scores and to-dos (DEMO_DATA=true;
// only into an empty account).
func (s *Server) seedDemo(ctx context.Context, uid int64) error {
	ctx = context.WithoutCancel(ctx)
	if list, _ := s.store.ListGames(ctx, uid, store.GameFilter{}); len(list) > 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(42))
	now := time.Now()

	// drawn sprites
	robot, err := s.demoSprite(ctx, uid, "Robot", robotPixels())
	if err != nil {
		return err
	}
	heart, _ := s.demoSprite(ctx, uid, "Heart", heartPixels())
	_, _ = s.demoSprite(ctx, uid, "Mushroom", mushroomPixels())

	type demo struct {
		tpl, title, about string
		days              int // made this many days ago
		shared, fav       bool
		plays             int
		opts              gamedoc.Options
		todos             []string
	}
	demos := []demo{
		{"platformer", "Kitty's Big Jump", "A cat runs and jumps across two levels of ledges, coins and sleepy snails.", 58, true, true, 140, gamedoc.Options{}, []string{"Add a third level with moving platforms", "The snail in level 2 is too close to the start"}},
		{"maze", "Ghost Castle", "Find every gem in the castle, open the doors with keys and keep away from the ghosts.", 41, true, false, 85, gamedoc.Options{Emoji: map[string]string{"player": "🧙", "gem": "🔮"}, Names: map[string]string{"player": "Wizard", "gem": "Orb"}}, []string{"Ghosts should be faster in level 2"}},
		{"shooter", "Rock Storm", "Keep your ship alive in a storm of falling rocks.", 30, true, false, 120, gamedoc.Options{}, nil},
		{"catch", "Apple Orchard", "Catch the apples falling from the trees — not the bombs!", 19, true, true, 64, gamedoc.Options{}, []string{"A golden apple worth 5", "Music for the orchard"}},
		{"quiz", "Space Quiz", "Ten questions about the planets and the stars.", 12, true, false, 37, gamedoc.Options{Questions: []gamedoc.Question{
			{Q: "Which planet is closest to the Sun?", Answers: []string{"Mercury", "Venus", "Mars"}},
			{Q: "What is the Sun?", Answers: []string{"A star", "A planet", "A moon"}},
			{Q: "Which planet has the biggest rings?", Answers: []string{"Saturn", "Neptune", "Earth"}},
			{Q: "How many planets go around the Sun?", Answers: []string{"Eight", "Nine", "Seven"}},
			{Q: "Which planet is called the red planet?", Answers: []string{"Mars", "Jupiter", "Uranus"}},
			{Q: "What do we call a rock that burns up in the sky?", Answers: []string{"A meteor", "A comet", "A satellite"}},
			{Q: "Who was the first person on the Moon?", Answers: []string{"Neil Armstrong", "Yuri Gagarin", "Buzz Lightyear"}},
			{Q: "Which is the hottest planet?", Answers: []string{"Venus", "Mercury", "Mars"}},
		}}, nil},
		{"bricks", "Rainbow Bricks", "Bounce the ball and break the rainbow.", 6, false, false, 0, gamedoc.Options{}, []string{"Bricks that need two hits"}},
		{"blank", "Robot Walk", "My own robot, drawn in the pixel editor. Nothing to do yet!", 2, false, false, 0, gamedoc.Options{Title: "Robot Walk"}, []string{"Give the robot something to collect", "Draw a background"}},
	}
	names := []string{"Mia", "Luka", "Ana", "Noah", "Sara", "Ivan", "Ema", "Jakov", "Lena", "Petar", "Zoe", "Marko"}
	for i, d := range demos {
		d.opts.Seed = int64(100 + i)
		if d.opts.Title == "" {
			d.opts.Title = d.title
		}
		doc, err := gamedoc.Build(d.tpl, d.opts)
		if err != nil {
			return err
		}
		if d.tpl == "blank" && robot != nil {
			if o := doc.Obj("player"); o != nil {
				o.Name, o.Look = "Robot", gamedoc.Look{Type: "image", Image: robot.ID, FPS: 4}
				o.W, o.H = 32, 32
			}
			if heart != nil {
				doc.Objects = append(doc.Objects, gamedoc.Obj{ID: "heart", Name: "Heart", Role: "coin", W: 16, H: 16, Look: gamedoc.Look{Type: "image", Image: heart.ID}, Behaviors: []gamedoc.Beh{{T: "bob"}}})
				sc := &doc.Scenes[0]
				for k := 0; k < 5; k++ {
					sc.Items = append(sc.Items, gamedoc.Inst{ID: gamedoc.NewID(rng), O: "heart", X: float64(40 + rng.Intn(400)), Y: float64(30 + rng.Intn(200))})
				}
			}
		}
		created := now.AddDate(0, 0, -d.days).Add(-time.Duration(rng.Intn(600)) * time.Minute)
		g := &store.Game{UserID: uid, Title: d.title, Description: d.about, Template: d.tpl, Doc: doc.Marshal(), ScoresOn: true, Favorite: d.fav, RemixOn: i%2 == 0}
		if err := s.store.CreateGame(ctx, g); err != nil {
			return err
		}
		updated := created.Add(time.Duration(rng.Intn(d.days*20+30)) * time.Hour)
		if updated.After(now) {
			updated = now.Add(-time.Duration(rng.Intn(120)) * time.Minute)
		}
		_, _ = s.store.DB.ExecContext(ctx, `UPDATE games SET created_at = ?, updated_at = ?, saves = ? WHERE id = ?`,
			created.UTC().Format(time.RFC3339Nano), updated.UTC().Format(time.RFC3339Nano), 5+rng.Intn(40), g.ID)
		_, _ = s.store.AddVersion(ctx, uid, g.ID, "First try", g.Doc, 30)
		if d.shared {
			key := fmt.Sprintf("demo%s%02d", strconv.Itoa(4242+i), i)
			_ = s.store.SetShare(ctx, uid, g.ID, true, key, g.Doc)
		}
		for _, t := range d.todos {
			_ = s.store.CreateTodo(ctx, &store.Todo{UserID: uid, GameID: g.ID, Text: t})
		}
		low := d.tpl == "quiz"
		_ = low
		for k := 0; k < d.plays; k++ {
			at := created.Add(time.Duration(rng.Int63n(int64(now.Sub(created)))))
			who := names[rng.Intn(len(names))]
			score, result := demoScore(d.tpl, rng)
			p := &store.Play{UserID: uid, GameID: g.ID, Who: who, Score: score, Seconds: float64(30 + rng.Intn(300)), Result: result, Day: at.Format("2006-01-02")}
			if err := s.store.AddPlay(ctx, p); err != nil {
				return err
			}
			_, _ = s.store.DB.ExecContext(ctx, `UPDATE plays SET created_at = ? WHERE id = ?`, at.UTC().Format(time.RFC3339Nano), p.ID)
		}
		for k := 0; k < 3+rng.Intn(6); k++ {
			at := created.Add(time.Duration(rng.Int63n(int64(now.Sub(created)) + 1)))
			score, result := demoScore(d.tpl, rng)
			p := &store.Play{UserID: uid, GameID: g.ID, Mine: true, Score: score, Seconds: float64(20 + rng.Intn(200)), Result: result, Day: at.Format("2006-01-02")}
			_ = s.store.AddPlay(ctx, p)
		}
	}
	s.log.Info("demo data added", "games", len(demos))
	return nil
}

func demoScore(tpl string, rng *rand.Rand) (float64, string) {
	res := []string{"won", "lost", "lost", "quit"}[rng.Intn(4)]
	switch tpl {
	case "platformer":
		return float64(rng.Intn(60)), res
	case "maze":
		return float64(rng.Intn(14)), res
	case "shooter":
		return float64(rng.Intn(32) * 10), res
	case "catch":
		return float64(rng.Intn(31)), res
	case "quiz":
		return float64(rng.Intn(9)), "won"
	}
	return float64(rng.Intn(50)), res
}

// pixel sprites: a palette and rows of palette letters per frame
type pixelArt struct {
	W       int      `json:"w"`
	H       int      `json:"h"`
	Palette []string `json:"palette"`
	Frames  [][]int  `json:"frames"`
}

func artFrom(palette map[rune]string, frames ...[]string) pixelArt {
	keys := []rune{'.'}
	pal := []string{""}
	idx := map[rune]int{'.': 0}
	for _, f := range frames {
		for _, row := range f {
			for _, ch := range row {
				if _, ok := idx[ch]; !ok {
					idx[ch] = len(pal)
					pal = append(pal, palette[ch])
					keys = append(keys, ch)
				}
			}
		}
	}
	a := pixelArt{W: len(frames[0][0]), H: len(frames[0]), Palette: pal}
	for _, f := range frames {
		px := make([]int, 0, a.W*a.H)
		for _, row := range f {
			for _, ch := range row {
				px = append(px, idx[ch])
			}
		}
		a.Frames = append(a.Frames, px)
	}
	_ = keys
	return a
}

func (a pixelArt) png() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, a.W*len(a.Frames), a.H))
	for f, px := range a.Frames {
		for i, c := range px {
			if c <= 0 || c >= len(a.Palette) || a.Palette[c] == "" {
				continue
			}
			var r, g, b uint8
			fmt.Sscanf(a.Palette[c], "#%02x%02x%02x", &r, &g, &b)
			img.Set(f*a.W+i%a.W, i/a.W, color.NRGBA{r, g, b, 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func (s *Server) demoSprite(ctx context.Context, uid int64, name string, a pixelArt) (*store.Asset, error) {
	b := a.png()
	rel, err := s.saveBytes(uid, ".png", b)
	if err != nil {
		return nil, err
	}
	px, _ := json.Marshal(a)
	as := &store.Asset{UserID: uid, Kind: "image", Name: name, Mime: "image/png", Path: rel, Size: int64(len(b)), Width: a.W * len(a.Frames), Height: a.H, Frames: len(a.Frames), Pixels: string(px), Origin: "drawn"}
	if err := s.store.CreateAsset(ctx, as); err != nil {
		return nil, err
	}
	return as, nil
}

func robotPixels() pixelArt {
	pal := map[rune]string{'g': "#8d99ae", 'd': "#2b2d42", 'c': "#4dd0e1", 'r': "#ef233c", 'w': "#edf2f4"}
	f1 := []string{
		".......rr.......", ".......dd.......", "...gggggggggg...", "...gwwwwwwwwg...",
		"...gwccwwccwg...", "...gwccwwccwg...", "...gwwwwwwwwg...", "...gwwddddwwg...",
		"...gggggggggg...", ".....dggggd.....", "..ggggggggggggg.", "..g.gccccccg.g..",
		"..g.gggggggg.g..", "....gg....gg....", "....gg....gg....", "...ddd....ddd...",
	}
	f2 := append([]string{}, f1...)
	f2[4], f2[5] = "...gwwwwwwwwg...", "...gwddwwddwg..."
	f2[13], f2[14], f2[15] = ".....gg..gg.....", ".....gg..gg.....", "....ddd..ddd...."
	return artFrom(pal, f1, f2)
}

func heartPixels() pixelArt {
	pal := map[rune]string{'r': "#e03131", 'p': "#ff8787", 'd': "#a51111"}
	return artFrom(pal, []string{
		"................", "..rrr......rrr..", ".rpprr....rrrrr.", "rpprrrr..rrrrrrr",
		"rprrrrrrrrrrrrrr", "rrrrrrrrrrrrrrrr", "rrrrrrrrrrrrrrrr", ".rrrrrrrrrrrrrr.",
		"..rrrrrrrrrrrrd.", "...rrrrrrrrrrd..", "....rrrrrrrrd...", ".....rrrrrrd....",
		"......rrrrd.....", ".......rrd......", "........d.......", "................",
	})
}

func mushroomPixels() pixelArt {
	pal := map[rune]string{'r': "#d6336c", 'w': "#ffffff", 'k': "#fff3bf", 'd': "#5c3d2e"}
	return artFrom(pal, []string{
		"................", ".....rrrrrr.....", "...rrwwrrrrrr...", "..rrwwwwrrwwrr..",
		".rrrwwrrrrwwwrr.", ".rrrrrrrrrrwwrr.", "rrwwrrrrrrrrrrrr", "rwwwwrrrwwrrrrrr",
		"rrrrrrrrrrrrrrrr", "....kkkkkkkk....", "....kdkkkkdk....", "....kkkkkkkk....",
		"....kkkkkkkk....", ".....kkkkkk.....", "................", "................",
	})
}

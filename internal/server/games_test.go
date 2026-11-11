package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/abjelosevic88/pocket-homelab-game-builder/internal/gamedoc"
)

func TestTemplatesBuildPlayableGames(t *testing.T) {
	for _, tp := range gamedoc.Templates {
		d, err := gamedoc.Build(tp.ID, gamedoc.Options{Seed: 7})
		if err != nil {
			t.Fatalf("%s: %v", tp.ID, err)
		}
		raw := d.Marshal()
		if _, err := gamedoc.Check(raw); err != nil {
			t.Fatalf("%s does not check: %v", tp.ID, err)
		}
		objs := map[string]bool{}
		for _, o := range d.Objects {
			objs[o.ID] = true
		}
		for _, s := range d.Scenes {
			if len(s.Items) == 0 {
				t.Fatalf("%s: scene %s is empty", tp.ID, s.Name)
			}
			for _, it := range s.Items {
				if !objs[it.O] {
					t.Fatalf("%s: item of unknown object %q", tp.ID, it.O)
				}
			}
		}
		for _, r := range d.Rules {
			for _, st := range append(append([]gamedoc.Step{}, r.When...), r.Do...) {
				for _, k := range []string{"a", "b", "o"} {
					if v, ok := st[k].(string); ok && !objs[v] {
						t.Fatalf("%s: rule uses unknown object %q", tp.ID, v)
					}
				}
			}
		}
		if gamedoc.Describe(raw) == "" {
			t.Fatalf("%s: no description", tp.ID)
		}
	}
	// a level from someone else (Pocket AI): kept when it fits the legend, else the template's own
	d, _ := gamedoc.Build("maze", gamedoc.Options{Levels: [][]string{{"#####", "#P *#", "#  E#", "#####"}, {"bad", "zzz", "P"}}})
	if len(d.Scenes) != 1 {
		t.Fatalf("want only the good level, got %d", len(d.Scenes))
	}
	if _, err := gamedoc.Build("nope", gamedoc.Options{}); err == nil {
		t.Fatal("an unknown template must fail")
	}
}

func TestTextsTranslateRoundTrip(t *testing.T) {
	d, _ := gamedoc.Build("quiz", gamedoc.Options{Title: "Quiz", Questions: []gamedoc.Question{{Q: "Two plus two?", Answers: []string{"Four", "Five"}}}, Seed: 3})
	raw := d.Marshal()
	texts := gamedoc.Texts(raw)
	if len(texts) < 5 {
		t.Fatalf("too few texts: %v", texts)
	}
	up := make([]string, len(texts))
	for i, s := range texts {
		up[i] = strings.ToUpper(s)
	}
	out, err := gamedoc.Replace(raw, up)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "TWO PLUS TWO?") || !strings.Contains(string(out), `"title":"QUIZ"`) {
		t.Fatalf("not replaced: %s", out)
	}
	if strings.Contains(string(out), `"t":"SAY"`) {
		t.Fatal("rule kinds must not be translated")
	}
}

func TestMazeLevelFromPuzzlecraft(t *testing.T) {
	// a 2×2 maze: start top-left, end bottom-right, an open path
	walls := []int{1 | 8, 1 | 2, 8 | 4, 2 | 4}
	rows := gamedoc.MazeLevel(2, 2, walls, [2]int{0, 0}, [2]int{1, 1}, nil)
	if len(rows) != 5 || !strings.Contains(strings.Join(rows, ""), "P") || !strings.Contains(strings.Join(rows, ""), "E") {
		t.Fatalf("bad maze: %q", rows)
	}
}

func TestGamesArePerPerson(t *testing.T) {
	a := newApp(t, nil)
	g := a.ok(a.alice, "POST", "/api/v1/games", map[string]any{"template": "platformer", "title": "Alice's jumps"})
	gid := id(g)
	if g["title"] != "Alice's jumps" || num(g, "scenes") != 2 {
		t.Fatalf("bad game: %v", g)
	}
	// Bob sees nothing of Alice's, and gets 404 on her ids
	if l := a.ok(a.bob, "GET", "/api/v1/games", nil)["games"].([]any); len(l) != 0 {
		t.Fatalf("bob sees %d games", len(l))
	}
	for _, p := range []string{"/api/v1/games/" + gid, "/api/v1/games/" + gid + "/plays", "/api/v1/games/" + gid + "/todos", "/api/v1/games/" + gid + "/versions", "/api/v1/games/" + gid + "/export.html"} {
		a.expect(404, a.bob, "GET", p, nil)
	}
	a.expect(404, a.bob, "PUT", "/api/v1/games/"+gid, map[string]any{"title": "mine now"})
	a.expect(404, a.bob, "PUT", "/api/v1/games/"+gid+"/doc", map[string]any{"doc": json.RawMessage(gamedocJSON(t))})
	a.expect(404, a.bob, "DELETE", "/api/v1/games/"+gid, nil)
	a.expect(404, a.bob, "POST", "/api/v1/games/"+gid+"/share", map[string]any{"on": true})
	td := a.ok(a.alice, "POST", "/api/v1/games/"+gid+"/todos", map[string]any{"text": "a third level"})
	a.expect(404, a.bob, "PUT", "/api/v1/todos/"+id(td), map[string]any{"done": true})
	// admins do not see other people's games either
	b := a.ok(a.bob, "POST", "/api/v1/games", map[string]any{"template": "blank"})
	a.expect(404, a.alice, "GET", "/api/v1/games/"+id(b), nil)
}

func gamedocJSON(t *testing.T) []byte {
	d, err := gamedoc.Build("blank", gamedoc.Options{Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	return d.Marshal()
}

func TestSaveDocVersionCheck(t *testing.T) {
	a := newApp(t, nil)
	g := a.ok(a.alice, "POST", "/api/v1/games", map[string]any{"template": "blank", "title": "Saves"})
	gid := id(g)
	full := a.ok(a.alice, "GET", "/api/v1/games/"+gid, nil)
	saves := int(num(full, "saves"))
	doc := full["doc"]
	r := a.ok(a.alice, "PUT", "/api/v1/games/"+gid+"/doc", map[string]any{"doc": doc, "saves": saves})
	if int(num(r, "saves")) != saves+1 {
		t.Fatalf("saves did not go up: %v", r)
	}
	// the same starting point again: somebody saved in between
	a.expect(409, a.alice, "PUT", "/api/v1/games/"+gid+"/doc", map[string]any{"doc": doc, "saves": saves})
	// a broken document is refused
	a.expect(400, a.alice, "PUT", "/api/v1/games/"+gid+"/doc", map[string]any{"doc": map[string]any{"v": 1, "scenes": []any{}}})
}

func TestSharingAndPublicPlays(t *testing.T) {
	a := newApp(t, nil)
	g := a.ok(a.alice, "POST", "/api/v1/games", map[string]any{"template": "catch", "title": "Apples"})
	gid := id(g)
	// a picture Alice uploads, used by the game, and one that is not
	used := uploadPNG(t, a, a.alice, "used.png")
	unused := uploadPNG(t, a, a.alice, "secret.png")
	full := a.ok(a.alice, "GET", "/api/v1/games/"+gid, nil)
	doc := full["doc"].(map[string]any)
	objs := doc["objects"].([]any)
	objs[0].(map[string]any)["look"] = map[string]any{"type": "image", "image": num(used, "id")}
	a.ok(a.alice, "PUT", "/api/v1/games/"+gid+"/doc", map[string]any{"doc": doc})

	sh := a.ok(a.alice, "POST", "/api/v1/games/"+gid+"/share", map[string]any{"on": true})
	key := str(sh, "share_key")
	if key == "" || !strings.HasPrefix(str(sh, "share_url"), "http://gamebuilder.test/p/") {
		t.Fatalf("no share link: %v", sh)
	}
	// no account needed to play
	pub := a.ok("", "GET", "/api/v1/public/"+key, nil)
	if pub["title"] != "Apples" || pub["by"] != "Alice" {
		t.Fatalf("public: %v", pub)
	}
	a.expect(200, "", "GET", "/api/v1/public/"+key+"/assets/"+id(used), nil)
	a.expect(404, "", "GET", "/api/v1/public/"+key+"/assets/"+id(unused), nil)
	// plays and the best-scores table
	a.ok("", "POST", "/api/v1/public/"+key+"/plays", map[string]any{"who": "Mia", "score": 12, "seconds": 40, "result": "lost"})
	r := a.ok("", "POST", "/api/v1/public/"+key+"/plays", map[string]any{"who": "Noah", "score": 30, "seconds": 80, "result": "won"})
	if num(r, "rank") != 1 || r["best"] != true {
		t.Fatalf("noah should top the table: %v", r)
	}
	pub = a.ok("", "GET", "/api/v1/public/"+key, nil)
	best := pub["best"].([]any)
	if len(best) != 2 || best[0].(map[string]any)["who"] != "Noah" {
		t.Fatalf("best: %v", best)
	}
	if got := a.ok(a.alice, "GET", "/api/v1/games/"+gid, nil); num(got, "plays") != 2 {
		t.Fatalf("plays not counted: %v", got["plays"])
	}
	// the shared version stays until shared again
	doc["title"] = "Apples 2"
	a.ok(a.alice, "PUT", "/api/v1/games/"+gid+"/doc", map[string]any{"doc": doc})
	if g := a.ok(a.alice, "GET", "/api/v1/games/"+gid, nil); g["changed_since_share"] != true {
		t.Fatal("should say it changed since it was shared")
	}
	// the page itself
	code, _, _ := a.call("", "GET", "/p/"+key, nil)
	if code != 404 { // no web build in tests: the handler answers, the player is missing
		t.Logf("player page: %d", code)
	}
	// switched off: gone for everyone
	a.ok(a.alice, "POST", "/api/v1/games/"+gid+"/share", map[string]any{"on": false})
	a.expect(404, "", "GET", "/api/v1/public/"+key, nil)
	a.expect(404, "", "POST", "/api/v1/public/"+key+"/plays", map[string]any{"who": "x", "score": 1, "result": "won"})
	// copies only when allowed
	a.ok(a.alice, "POST", "/api/v1/games/"+gid+"/share", map[string]any{"on": true})
	a.expect(404, a.bob, "POST", "/api/v1/remix/"+key, nil)
	a.ok(a.alice, "PUT", "/api/v1/games/"+gid, map[string]any{"remix_on": true})
	cp := a.ok(a.bob, "POST", "/api/v1/remix/"+key, nil)
	bobDoc := a.ok(a.bob, "GET", "/api/v1/games/"+id(cp), nil)["doc"].(map[string]any)
	look := bobDoc["objects"].([]any)[0].(map[string]any)["look"].(map[string]any)
	if int64(num(look, "image")) == int64(num(used, "id")) {
		t.Fatal("the copy must use Bob's own copy of the picture")
	}
	a.expect(200, a.bob, "GET", "/api/v1/assets/"+itoa(int64(num(look, "image")))+"/file", nil)
}

func uploadPNG(t *testing.T, a *app, tok, name string) map[string]any {
	t.Helper()
	png := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82")
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, _ := w.CreateFormFile("file", name)
	_, _ = fw.Write(png)
	_ = w.WriteField("kind", "image")
	_ = w.Close()
	req, _ := http.NewRequest("POST", a.url+"/api/v1/assets", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	if res.StatusCode != 201 {
		t.Fatalf("upload: %d %s", res.StatusCode, raw)
	}
	return m["assets"].([]any)[0].(map[string]any)
}

func TestLinkScopes(t *testing.T) {
	req := func(m, p string) *http.Request { r, _ := http.NewRequest(m, p, nil); return r }
	read := []string{"gamebuilder:read"}
	write := []string{"gamebuilder:write"}
	cases := []struct {
		scopes []string
		m, p   string
		ok     bool
	}{
		{read, "GET", "/api/v1/auth/me", true},
		{read, "GET", "/api/v1/summary", true},
		{read, "GET", "/api/v1/games", true},
		{read, "GET", "/api/v1/games/4", true},
		{read, "GET", "/api/v1/games/4/plays", true},
		{read, "GET", "/api/v1/day/2026-11-11", true},
		{read, "POST", "/api/v1/games", false},
		{write, "POST", "/api/v1/games", true},
		{write, "POST", "/api/v1/ai/game", true},
		{write, "POST", "/api/v1/games/4/todos", true},
		{write, "PUT", "/api/v1/games/4", true},
		{write, "PUT", "/api/v1/games/4/doc", false},
		{write, "POST", "/api/v1/games/4/share", false},
		{write, "DELETE", "/api/v1/assets/3", false},
		{read, "GET", "/api/v1/settings", false},
		{read, "GET", "/api/v1/export.json", false},
		{read, "GET", "/api/v1/games/4/export.html", false},
	}
	for _, c := range cases {
		if got := linkAllows(c.scopes, req(c.m, c.p)); got != c.ok {
			t.Errorf("%v %s %s: got %v want %v", c.scopes, c.m, c.p, got, c.ok)
		}
	}
}

func TestLinkMayChangeOnlyItsOwn(t *testing.T) {
	a := newApp(t, nil)
	mine := a.ok(a.alice, "POST", "/api/v1/games", map[string]any{"template": "blank", "title": "Mine"})
	ctx := withLinkApp(context.Background(), "ai")
	if mayChange(ctx, "") {
		t.Fatal("a link may not change a game made here")
	}
	if !mayChange(ctx, "ai") || !mayChange(context.Background(), "ai") {
		t.Fatal("own rows / no link: allowed")
	}
	_ = mine
}

func TestExportImportIsIdempotent(t *testing.T) {
	a := newApp(t, nil)
	g := a.ok(a.alice, "POST", "/api/v1/games", map[string]any{"template": "maze", "title": "Castle"})
	uploadPNG(t, a, a.alice, "hero.png")
	a.ok(a.alice, "POST", "/api/v1/games/"+id(g)+"/todos", map[string]any{"text": "more ghosts"})
	a.ok(a.alice, "POST", "/api/v1/games/"+id(g)+"/plays", map[string]any{"score": 5, "seconds": 20, "result": "won"})
	exp := a.ok(a.alice, "GET", "/api/v1/export.json", nil)
	// into Bob's account: everything arrives once
	r1 := a.ok(a.bob, "POST", "/api/v1/import", exp)
	added := r1["added"].(map[string]any)
	if num(added, "games") != 1 || num(added, "assets") != 1 || num(added, "todos") != 1 || num(added, "plays") != 1 {
		t.Fatalf("first import: %v", added)
	}
	r2 := a.ok(a.bob, "POST", "/api/v1/import", exp)
	for k, v := range r2["added"].(map[string]any) {
		if v.(float64) != 0 {
			t.Fatalf("second import added %s: %v", k, v)
		}
	}
	// Alice's own export back into Alice: nothing new
	r3 := a.ok(a.alice, "POST", "/api/v1/import", exp)
	if num(r3["added"].(map[string]any), "games") != 0 {
		t.Fatalf("self import: %v", r3)
	}
}

func TestGameFileRoundTrip(t *testing.T) {
	a := newApp(t, nil)
	pic := uploadPNG(t, a, a.alice, "coin.png")
	g := a.ok(a.alice, "POST", "/api/v1/games", map[string]any{"template": "platformer", "title": "Coins"})
	full := a.ok(a.alice, "GET", "/api/v1/games/"+id(g), nil)
	doc := full["doc"].(map[string]any)
	for _, o := range doc["objects"].([]any) {
		if o.(map[string]any)["id"] == "coin" {
			o.(map[string]any)["look"] = map[string]any{"type": "image", "image": num(pic, "id")}
		}
	}
	a.ok(a.alice, "PUT", "/api/v1/games/"+id(g)+"/doc", map[string]any{"doc": doc})
	file := a.ok(a.alice, "GET", "/api/v1/games/"+id(g)+"/export.json", nil)
	if len(file["assets"].([]any)) != 1 {
		t.Fatalf("the picture should travel with the game: %v", file["assets"])
	}
	ng := a.ok(a.bob, "POST", "/api/v1/games/import", file)
	if ng["title"] != "Coins" {
		t.Fatalf("imported: %v", ng)
	}
	assets := a.ok(a.bob, "GET", "/api/v1/assets", nil)["assets"].([]any)
	if len(assets) != 1 {
		t.Fatalf("bob should now have the picture: %v", assets)
	}
}

func TestDaySearchSummary(t *testing.T) {
	a := newApp(t, nil)
	a.ok(a.alice, "POST", "/api/v1/games", map[string]any{"template": "quiz", "title": "Planets", "questions": []map[string]any{{"q": "Which planet has rings?", "answers": []string{"Saturn", "Mars", "Venus"}}}})
	res := a.ok(a.alice, "GET", "/api/v1/search?q=saturn", nil)["results"].([]any)
	if len(res) != 1 {
		t.Fatalf("search inside the game: %v", res)
	}
	if res := a.ok(a.bob, "GET", "/api/v1/search?q=saturn", nil)["results"].([]any); len(res) != 0 {
		t.Fatal("bob finds alice's game")
	}
	sum := a.ok(a.alice, "GET", "/api/v1/summary", nil)
	if num(sum, "games") != 1 {
		t.Fatalf("summary: %v", sum)
	}
	day := a.ok(a.alice, "GET", "/api/v1/day/"+today(), nil)
	if len(day["made"].([]any)) != 1 {
		t.Fatalf("day: %v", day)
	}
	a.expect(400, a.alice, "GET", "/api/v1/day/yesterday", nil)
}

func TestSettingsArePersonal(t *testing.T) {
	a := newApp(t, nil)
	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"maker_name": "Ali", "default_size": "tall", "notify_daily": true})
	b := a.ok(a.bob, "GET", "/api/v1/settings", nil)
	if b["maker_name"] != "" || b["default_size"] != "wide" || b["notify_daily"] != false {
		t.Fatalf("bob got alice's settings: %v", b)
	}
	al := a.ok(a.alice, "GET", "/api/v1/settings", nil)
	if al["maker_name"] != "Ali" || al["default_size"] != "tall" {
		t.Fatalf("alice: %v", al)
	}
}

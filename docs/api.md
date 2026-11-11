# API

Everything the app does goes through `/api/v1` with JSON. Sign in with the session cookie, or send a personal token (Settings → API & shortcuts) as `Authorization: Bearer pgb_…`.

## Games

| | |
|---|---|
| `GET /games?filter=shared\|fav\|archived&q=` | Your games, newest change first (without the game itself) |
| `POST /games` | `{"template": "platformer", "title": "…", "size": "wide"}` — templates from `GET /templates`: `platformer`, `maze`, `shooter`, `bricks`, `catch`, `quiz` (with `"questions": [{"q": "…", "answers": ["right", "wrong", "wrong"]}]`), `blank`; or `{"doc": {…}}` for a whole game |
| `GET /games/ID` | One game with `doc`: the game itself (below) |
| `PUT /games/ID` | `title`, `description`, `favorite`, `archived`, `scores_on`, `remix_on` |
| `PUT /games/ID/doc` | `{"doc": {…}, "saves": 12}` — `saves` is the count you started from; 409 when it was saved elsewhere since |
| `POST /games/ID/copy` · `DELETE /games/ID` | |
| `POST /games/ID/share` | `{"on": true}` shares the game as it is now (`share_url`, `embed_url`); `POST /games/ID/share/new-key` makes a new link |
| `GET /games/ID/versions` · `POST /games/ID/versions` · `POST /versions/ID/restore` | Saved copies |
| `GET /games/ID/plays` · `POST /games/ID/plays` | Plays, the best-scores table, plays per day; write down your own play `{"score": 12, "seconds": 40, "result": "won"}` |
| `GET /games/ID/todos` · `POST /games/ID/todos` · `PUT/DELETE /todos/ID` | What is still to do |
| `GET /games/ID/export.html` | The game as one HTML file that plays anywhere |
| `GET /games/ID/export.json` · `POST /games/import` | A game with its pictures and sounds, to bring to another box |
| `POST /ai/game` | `{"idea": "a frog crossing a river"}` — Pocket AI makes a game |
| `POST /ai/rule` · `POST /ai/questions` · `POST /games/ID/ideas` | A rule from a sentence, quiz questions, ideas |

## The library

`GET /assets?kind=image|sound|music`, `POST /assets` (form: `file`, `kind`, `game_id`), `POST /assets/drawn` (`{"name", "png": "data:…", "pixels": {…}, "frames": 2}`), `GET /assets/ID/file`, `PUT/DELETE /assets/ID`.

## Shared games (no account)

`GET /public/KEY` (the game as shared, who made it, the best scores), `GET /public/KEY/assets/ID`, `GET /public/KEY/cover.png`, `POST /public/KEY/plays` (`{"who": "Mia", "score": 30, "seconds": 80, "result": "won"}`). A signed-in person takes a copy (when allowed) with `POST /remix/KEY`.

## For the rest of the box

`GET /summary` (latest games, plays this week), `GET /search?q=` (games — also by the words in them — pictures and sounds), `GET /day/YYYY-MM-DD` (games made or worked on and plays), `GET /stats?days=30`.

Through Pocket Core, other apps act for a person with the link scopes `gamebuilder:read` (everything above that reads) and `gamebuilder:write` (`POST /games`, `POST /ai/game`, `POST /games/ID/todos`, and changing or removing only what that app made). Events: `gamebuilder.game_made`, `gamebuilder.game_shared`, `gamebuilder.played`, `gamebuilder.best_score`.

## The game itself (`doc`)

```json
{
  "v": 1, "title": "Kitty's Big Jump", "width": 480, "height": 270, "pixel": false, "font": "round",
  "start": "<scene id>", "controls": "auto",
  "score": {"var": "score", "kind": "high", "label": "Score"},
  "intro": {"on": true, "title": "…", "text": "how to play", "button": "Play"},
  "ends": {"won": "You did it!", "lost": "Game over"},
  "vars": [{"name": "score", "value": 0}, {"name": "lives", "value": 3}],
  "hud": [{"id": "h1", "text": "🪙 {score}", "x": 10, "y": 10, "size": 16, "color": "#fff", "align": "left"}],
  "objects": [{"id": "player", "name": "Cat", "role": "player", "w": 22, "h": 22, "z": 5,
               "look": {"type": "emoji", "emoji": "🐱"},
               "behaviors": [{"t": "platformer", "p": {"speed": 150, "jump": 380}}, {"t": "camera"}]}],
  "scenes": [{"id": "…", "name": "Level 1", "w": 960, "h": 264, "bg": "#7ec8f0", "grid": 24,
              "items": [{"id": "…", "o": "player", "x": 30, "y": 170}], "rules": []}],
  "rules": [{"id": "…", "on": true,
             "when": [{"t": "touch", "a": "player", "b": "coin"}],
             "do": [{"t": "add", "name": "score", "value": 1}, {"t": "destroy", "who": "b"}, {"t": "sound", "s": "coin"}]}],
  "sounds": [{"id": "coin", "name": "Coin", "sfx": {"preset": "coin"}}, {"id": "song", "name": "Song", "asset": 12, "loop": true}]
}
```

Looks: `box`, `circle`, `triangle`, `emoji`, `image` (`"image": <asset id>`, frames side by side animate), `text`, `none`. Behaviours: `platformer`, `topdown`, `solid`, `oneway`, `gravity`, `patrol`, `follow`, `move`, `bounce`, `paddle`, `shooter`, `stayin`, `wrap`, `drag`, `camera`, `bob`, `spin`, `fade`. Conditions: `start`, `every`, `after`, `key`, `touch`, `stomp`, `tap`, `leaves`, `var`, `count`, `chance`. Actions: `add`, `set`, `destroy`, `create`, `moveto`, `jump`, `push`, `hide`, `show`, `look`, `sound`, `music`, `say`, `shake`, `scene`, `win`, `lose`.

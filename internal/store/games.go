package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

// ---- games ----

const gameCols = `id, user_id, title, description, template, saves, cover, COALESCE(share_key,''), share_on, scores_on, remix_on,
	published_at, plays, favorite, archived, source, played_at, created_at, updated_at`

type scanner interface{ Scan(dest ...any) error }

// GameFilter narrows a list of games.
type GameFilter struct {
	Archived bool   // the archived ones only
	Favorite bool   // favourites only
	Shared   bool   // shared ones only
	Query    string // title or description
	Limit    int
}

// ListGames lists one person's games, newest change first, with numbers
// worked out from each game's document (scenes, objects, rules).
func (s *Store) ListGames(ctx context.Context, uid int64, f GameFilter) ([]Game, error) {
	q := `SELECT ` + gameCols + `, doc FROM games WHERE user_id = ? AND archived = ?`
	args := []any{uid, boolInt(f.Archived)}
	if f.Favorite {
		q += ` AND favorite = 1`
	}
	if f.Shared {
		q += ` AND share_on = 1`
	}
	if t := strings.TrimSpace(f.Query); t != "" {
		q += ` AND (title LIKE ? OR description LIKE ?)`
		args = append(args, "%"+t+"%", "%"+t+"%")
	}
	q += ` ORDER BY updated_at DESC`
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	out := []Game{}
	for rows.Next() {
		var g Game
		var doc string
		var share, scores, remix, fav, arch int
		if err := rows.Scan(&g.ID, &g.UserID, &g.Title, &g.Description, &g.Template, &g.Saves, &g.Cover, &g.ShareKey, &share, &scores, &remix,
			&g.PublishedAt, &g.Plays, &fav, &arch, &g.Source, &g.PlayedAt, &g.CreatedAt, &g.UpdatedAt, &doc); err != nil {
			rows.Close()
			return nil, err
		}
		g.ShareOn, g.ScoresOn, g.RemixOn, g.Favorite, g.Archived = share == 1, scores == 1, remix == 1, fav == 1, arch == 1
		g.HasCover = g.Cover != ""
		g.Scenes, g.Objects, g.Rules = DocCounts([]byte(doc))
		out = append(out, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// best scores, plays while testing and open to-dos, in three queries
	idx := map[int64]int{}
	for i, g := range out {
		idx[g.ID] = i
	}
	if len(out) > 0 {
		s.eachRow(ctx, `SELECT game_id, MAX(score), SUM(mine) FROM plays WHERE user_id = ? GROUP BY game_id`, []any{uid}, func(r *sql.Rows) {
			var id int64
			var best float64
			var mine int
			if r.Scan(&id, &best, &mine) == nil {
				if i, ok := idx[id]; ok {
					out[i].Best, out[i].MyPlays = best, mine
				}
			}
		})
		s.eachRow(ctx, `SELECT game_id, COUNT(1) FROM todos WHERE user_id = ? AND done = 0 GROUP BY game_id`, []any{uid}, func(r *sql.Rows) {
			var id int64
			var n int
			if r.Scan(&id, &n) == nil {
				if i, ok := idx[id]; ok {
					out[i].Todo = n
				}
			}
		})
	}
	return out, nil
}

func (s *Store) eachRow(ctx context.Context, q string, args []any, fn func(*sql.Rows)) {
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		fn(rows)
	}
}

// DocCounts reads how many scenes, objects and rules a game document has.
func DocCounts(doc []byte) (scenes, objects, rules int) {
	var d struct {
		Objects []json.RawMessage `json:"objects"`
		Rules   []json.RawMessage `json:"rules"`
		Scenes  []struct {
			Rules []json.RawMessage `json:"rules"`
		} `json:"scenes"`
	}
	if json.Unmarshal(doc, &d) != nil {
		return 0, 0, 0
	}
	rules = len(d.Rules)
	for _, sc := range d.Scenes {
		rules += len(sc.Rules)
	}
	return len(d.Scenes), len(d.Objects), rules
}

// GetGame reads one of the person's games, with its document.
func (s *Store) GetGame(ctx context.Context, uid, id int64) (*Game, error) {
	var g Game
	var doc, pub string
	row := s.DB.QueryRowContext(ctx, `SELECT `+gameCols+`, doc, published FROM games WHERE user_id = ? AND id = ?`, uid, id)
	var share, scores, remix, fav, arch int
	if err := row.Scan(&g.ID, &g.UserID, &g.Title, &g.Description, &g.Template, &g.Saves, &g.Cover, &g.ShareKey, &share, &scores, &remix,
		&g.PublishedAt, &g.Plays, &fav, &arch, &g.Source, &g.PlayedAt, &g.CreatedAt, &g.UpdatedAt, &doc, &pub); err != nil {
		return nil, notFound(err)
	}
	g.ShareOn, g.ScoresOn, g.RemixOn, g.Favorite, g.Archived = share == 1, scores == 1, remix == 1, fav == 1, arch == 1
	g.HasCover = g.Cover != ""
	g.Doc = json.RawMessage(doc)
	if pub != "" {
		g.Published = json.RawMessage(pub)
	}
	g.Scenes, g.Objects, g.Rules = DocCounts(g.Doc)
	return &g, nil
}

// GameByShareKey finds a shared game by its key (anyone with the link).
func (s *Store) GameByShareKey(ctx context.Context, key string) (*Game, error) {
	var uid, id int64
	if err := s.DB.QueryRowContext(ctx, `SELECT user_id, id FROM games WHERE share_key = ? AND share_on = 1 AND archived = 0`, key).Scan(&uid, &id); err != nil {
		return nil, notFound(err)
	}
	return s.GetGame(ctx, uid, id)
}

// CreateGame adds a game.
func (s *Store) CreateGame(ctx context.Context, g *Game) error {
	if len(g.Doc) == 0 {
		g.Doc = json.RawMessage(`{}`)
	}
	now := Now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO games (user_id, title, description, template, doc, scores_on, remix_on, favorite, source, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, g.UserID, g.Title, g.Description, g.Template, string(g.Doc), boolInt(g.ScoresOn), boolInt(g.RemixOn), boolInt(g.Favorite), g.Source, now, now)
	if err != nil {
		return err
	}
	g.ID, _ = res.LastInsertId()
	g.CreatedAt, g.UpdatedAt = now, now
	return nil
}

// SaveGameDoc stores a new document. When saves is given (>= 0) and the
// game was saved since, nothing changes and ErrConflict comes back (the
// editor in another tab saved first).
func (s *Store) SaveGameDoc(ctx context.Context, uid, id int64, doc json.RawMessage, title string, saves int) (int, error) {
	q := `UPDATE games SET doc = ?, saves = saves + 1, updated_at = ?`
	args := []any{string(doc), Now()}
	if title != "" {
		q += `, title = ?`
		args = append(args, title)
	}
	q += ` WHERE user_id = ? AND id = ?`
	args = append(args, uid, id)
	if saves >= 0 {
		q += ` AND saves = ?`
		args = append(args, saves)
	}
	res, err := s.DB.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.GetGame(ctx, uid, id); err != nil {
			return 0, err
		}
		return 0, ErrConflict
	}
	var n int
	err = s.DB.QueryRowContext(ctx, `SELECT saves FROM games WHERE user_id = ? AND id = ?`, uid, id).Scan(&n)
	return n, err
}

// UpdateGame stores the game's details (not its document).
func (s *Store) UpdateGame(ctx context.Context, g *Game) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE games SET title = ?, description = ?, scores_on = ?, remix_on = ?, favorite = ?, archived = ?, updated_at = ?
		WHERE user_id = ? AND id = ?`, g.Title, g.Description, boolInt(g.ScoresOn), boolInt(g.RemixOn), boolInt(g.Favorite), boolInt(g.Archived), Now(), g.UserID, g.ID)
	return affected(res, err)
}

// SetShare switches the share link on or off; on the first share a key is made.
func (s *Store) SetShare(ctx context.Context, uid, id int64, on bool, key string, published json.RawMessage) error {
	var res sql.Result
	var err error
	if on {
		res, err = s.DB.ExecContext(ctx, `UPDATE games SET share_on = 1, share_key = COALESCE(share_key, ?), published = ?, published_at = ? WHERE user_id = ? AND id = ?`,
			key, string(published), Now(), uid, id)
	} else {
		res, err = s.DB.ExecContext(ctx, `UPDATE games SET share_on = 0 WHERE user_id = ? AND id = ?`, uid, id)
	}
	return affected(res, conflict(err))
}

// NewShareKey gives the game a new key (the old link stops working).
func (s *Store) NewShareKey(ctx context.Context, uid, id int64, key string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE games SET share_key = ? WHERE user_id = ? AND id = ?`, key, uid, id)
	return affected(res, conflict(err))
}

// SetCover stores the media path of the game's picture.
func (s *Store) SetCover(ctx context.Context, uid, id int64, path string) (string, error) {
	var old string
	if err := s.DB.QueryRowContext(ctx, `SELECT cover FROM games WHERE user_id = ? AND id = ?`, uid, id).Scan(&old); err != nil {
		return "", notFound(err)
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE games SET cover = ? WHERE user_id = ? AND id = ?`, path, uid, id)
	return old, err
}

// DeleteGame removes a game with its versions, plays and to-dos; it returns
// the cover's media path so the file can go too. Pictures and sounds stay
// in the library.
func (s *Store) DeleteGame(ctx context.Context, uid, id int64) (string, error) {
	var cover string
	if err := s.DB.QueryRowContext(ctx, `SELECT cover FROM games WHERE user_id = ? AND id = ?`, uid, id).Scan(&cover); err != nil {
		return "", notFound(err)
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM games WHERE user_id = ? AND id = ?`, uid, id)
	return cover, err
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- assets ----

const assetCols = `id, user_id, game_id, kind, name, mime, path, size, width, height, frames, seconds, origin, origin_ref, source, created_at`

func scanAsset(sc scanner, a *Asset) error {
	return sc.Scan(&a.ID, &a.UserID, &a.GameID, &a.Kind, &a.Name, &a.Mime, &a.Path, &a.Size, &a.Width, &a.Height, &a.Frames, &a.Seconds, &a.Origin, &a.OriginRef, &a.Source, &a.CreatedAt)
}

// ListAssets lists a person's pictures, sounds and music ("" = all kinds).
func (s *Store) ListAssets(ctx context.Context, uid int64, kind string) ([]Asset, error) {
	q := `SELECT ` + assetCols + ` FROM assets WHERE user_id = ?`
	args := []any{uid}
	if kind != "" {
		q += ` AND kind = ?`
		args = append(args, kind)
	}
	rows, err := s.DB.QueryContext(ctx, q+` ORDER BY created_at DESC, id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Asset{}
	for rows.Next() {
		var a Asset
		if err := scanAsset(rows, &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetAsset reads one asset (with its pixels when it was drawn here).
func (s *Store) GetAsset(ctx context.Context, uid, id int64) (*Asset, error) {
	var a Asset
	row := s.DB.QueryRowContext(ctx, `SELECT `+assetCols+`, pixels FROM assets WHERE user_id = ? AND id = ?`, uid, id)
	if err := row.Scan(&a.ID, &a.UserID, &a.GameID, &a.Kind, &a.Name, &a.Mime, &a.Path, &a.Size, &a.Width, &a.Height, &a.Frames, &a.Seconds, &a.Origin, &a.OriginRef, &a.Source, &a.CreatedAt, &a.Pixels); err != nil {
		return nil, notFound(err)
	}
	return &a, nil
}

// CreateAsset adds an asset.
func (s *Store) CreateAsset(ctx context.Context, a *Asset) error {
	if a.Frames < 1 {
		a.Frames = 1
	}
	a.CreatedAt = Now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO assets (user_id, game_id, kind, name, mime, path, size, width, height, frames, seconds, pixels, origin, origin_ref, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, a.UserID, nullI(a.GameID), a.Kind, a.Name, a.Mime, a.Path, a.Size, a.Width, a.Height, a.Frames, a.Seconds, a.Pixels, a.Origin, a.OriginRef, a.Source, a.CreatedAt)
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

// UpdateAsset stores a new name, frame count, file and pixels; it returns
// the old media path when the file changed.
func (s *Store) UpdateAsset(ctx context.Context, a *Asset) (string, error) {
	var old string
	if err := s.DB.QueryRowContext(ctx, `SELECT path FROM assets WHERE user_id = ? AND id = ?`, a.UserID, a.ID).Scan(&old); err != nil {
		return "", notFound(err)
	}
	if a.Frames < 1 {
		a.Frames = 1
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE assets SET name = ?, mime = ?, path = ?, size = ?, width = ?, height = ?, frames = ?, seconds = ?, pixels = ? WHERE user_id = ? AND id = ?`,
		a.Name, a.Mime, a.Path, a.Size, a.Width, a.Height, a.Frames, a.Seconds, a.Pixels, a.UserID, a.ID)
	if old == a.Path {
		old = ""
	}
	return old, err
}

// DeleteAsset removes an asset and returns its media path.
func (s *Store) DeleteAsset(ctx context.Context, uid, id int64) (string, error) {
	var p string
	if err := s.DB.QueryRowContext(ctx, `SELECT path FROM assets WHERE user_id = ? AND id = ?`, uid, id).Scan(&p); err != nil {
		return "", notFound(err)
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM assets WHERE user_id = ? AND id = ?`, uid, id)
	return p, err
}

// AssetsByIDs reads several of one person's assets.
func (s *Store) AssetsByIDs(ctx context.Context, uid int64, ids []int64) ([]Asset, error) {
	out := []Asset{}
	for _, id := range ids {
		if a, err := s.GetAsset(ctx, uid, id); err == nil {
			a.Pixels = ""
			out = append(out, *a)
		}
	}
	return out, nil
}

// ---- versions ----

// ListVersions lists a game's saved copies, newest first (without documents).
func (s *Store) ListVersions(ctx context.Context, uid, gameID int64) ([]Version, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, user_id, game_id, note, created_at FROM versions WHERE user_id = ? AND game_id = ? ORDER BY created_at DESC, id DESC`, uid, gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Version{}
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.UserID, &v.GameID, &v.Note, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetVersion reads one saved copy with its document.
func (s *Store) GetVersion(ctx context.Context, uid, id int64) (*Version, error) {
	var v Version
	var doc string
	if err := s.DB.QueryRowContext(ctx, `SELECT id, user_id, game_id, note, doc, created_at FROM versions WHERE user_id = ? AND id = ?`, uid, id).
		Scan(&v.ID, &v.UserID, &v.GameID, &v.Note, &doc, &v.CreatedAt); err != nil {
		return nil, notFound(err)
	}
	v.Doc = json.RawMessage(doc)
	return &v, nil
}

// AddVersion keeps a copy of a game's document; only the newest keep stay.
func (s *Store) AddVersion(ctx context.Context, uid, gameID int64, note string, doc json.RawMessage, keep int) (*Version, error) {
	v := &Version{UserID: uid, GameID: gameID, Note: note, CreatedAt: Now()}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO versions (user_id, game_id, note, doc, created_at) VALUES (?, ?, ?, ?, ?)`, uid, gameID, note, string(doc), v.CreatedAt)
	if err != nil {
		return nil, err
	}
	v.ID, _ = res.LastInsertId()
	if keep > 0 {
		_, _ = s.DB.ExecContext(ctx, `DELETE FROM versions WHERE user_id = ? AND game_id = ? AND id NOT IN (SELECT id FROM versions WHERE user_id = ? AND game_id = ? ORDER BY created_at DESC, id DESC LIMIT ?)`,
			uid, gameID, uid, gameID, keep)
	}
	return v, nil
}

// DeleteVersion removes a saved copy.
func (s *Store) DeleteVersion(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM versions WHERE user_id = ? AND id = ?`, uid, id)
	return affected(res, err)
}

// ---- plays ----

// AddPlay writes down a finished play; a play by somebody else also counts
// on the game.
func (s *Store) AddPlay(ctx context.Context, p *Play) error {
	p.CreatedAt = Now()
	if p.Day == "" {
		p.Day = Today()
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	res, err := tx.ExecContext(ctx, `INSERT INTO plays (user_id, game_id, who, mine, score, seconds, result, level, day, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.UserID, p.GameID, p.Who, boolInt(p.Mine), p.Score, p.Seconds, p.Result, p.Level, p.Day, p.CreatedAt)
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	q := `UPDATE games SET played_at = ? WHERE user_id = ? AND id = ?`
	if !p.Mine {
		q = `UPDATE games SET played_at = ?, plays = plays + 1 WHERE user_id = ? AND id = ?`
	}
	if _, err := tx.ExecContext(ctx, q, p.CreatedAt, p.UserID, p.GameID); err != nil {
		return err
	}
	return tx.Commit()
}

// PlayFilter narrows plays.
type PlayFilter struct {
	GameID int64
	Others bool // only other people's
	Day    string
	From   string // created_at >= (UTC ISO)
	Limit  int
}

// ListPlays lists plays, newest first.
func (s *Store) ListPlays(ctx context.Context, uid int64, f PlayFilter) ([]Play, error) {
	q := `SELECT p.id, p.user_id, p.game_id, p.who, p.mine, p.score, p.seconds, p.result, p.level, p.day, p.created_at, g.title
		FROM plays p JOIN games g ON g.id = p.game_id WHERE p.user_id = ?`
	args := []any{uid}
	if f.GameID > 0 {
		q += ` AND p.game_id = ?`
		args = append(args, f.GameID)
	}
	if f.Others {
		q += ` AND p.mine = 0`
	}
	if f.Day != "" {
		q += ` AND p.day = ?`
		args = append(args, f.Day)
	}
	if f.From != "" {
		q += ` AND p.created_at >= ?`
		args = append(args, f.From)
	}
	q += ` ORDER BY p.created_at DESC, p.id DESC`
	if f.Limit <= 0 {
		f.Limit = 200
	}
	q += ` LIMIT ?`
	args = append(args, f.Limit)
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Play{}
	for rows.Next() {
		var p Play
		var mine int
		if err := rows.Scan(&p.ID, &p.UserID, &p.GameID, &p.Who, &mine, &p.Score, &p.Seconds, &p.Result, &p.Level, &p.Day, &p.CreatedAt, &p.Game); err != nil {
			return nil, err
		}
		p.Mine = mine == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// BestPlays is the best-scores table of a game: each name once with its
// best. low = smaller is better (fastest time).
func (s *Store) BestPlays(ctx context.Context, uid, gameID int64, low bool, limit int) ([]Play, error) {
	agg, ord := "MAX", "DESC"
	if low {
		agg, ord = "MIN", "ASC"
	}
	q := `SELECT who, ` + agg + `(score) AS best, MIN(created_at) FROM plays WHERE user_id = ? AND game_id = ? AND who <> '' AND result <> 'quit'`
	if low {
		q += ` AND result = 'won'`
	}
	q += ` GROUP BY lower(who) ORDER BY best ` + ord + `, MIN(created_at) ASC LIMIT ?`
	rows, err := s.DB.QueryContext(ctx, q, uid, gameID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Play{}
	for rows.Next() {
		var p Play
		if err := rows.Scan(&p.Who, &p.Score, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.GameID = gameID
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeletePlay removes a play (a silly name on the table).
func (s *Store) DeletePlay(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM plays WHERE user_id = ? AND id = ?`, uid, id)
	return affected(res, err)
}

// ClearScores removes the other people's plays of a game.
func (s *Store) ClearScores(ctx context.Context, uid, gameID int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM plays WHERE user_id = ? AND game_id = ? AND mine = 0`, uid, gameID)
	return err
}

// PlaysPerDay counts plays by others per day since from (YYYY-MM-DD).
func (s *Store) PlaysPerDay(ctx context.Context, uid int64, from string) (map[string]int, error) {
	return s.GamePlaysPerDay(ctx, uid, 0, from)
}

// GamePlaysPerDay is PlaysPerDay for one game (0 = all of them).
func (s *Store) GamePlaysPerDay(ctx context.Context, uid, gameID int64, from string) (map[string]int, error) {
	out := map[string]int{}
	q := `SELECT day, COUNT(1) FROM plays WHERE user_id = ? AND mine = 0 AND day >= ?`
	args := []any{uid, from}
	if gameID > 0 {
		q += ` AND game_id = ?`
		args = append(args, gameID)
	}
	rows, err := s.DB.QueryContext(ctx, q+` GROUP BY day`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		var n int
		if rows.Scan(&d, &n) == nil {
			out[d] = n
		}
	}
	return out, rows.Err()
}

// ---- to-dos ----

// ListTodos lists a game's to-dos (gameID 0 = every game's), open first.
func (s *Store) ListTodos(ctx context.Context, uid, gameID int64) ([]Todo, error) {
	q := `SELECT id, user_id, game_id, text, done, task_url, source, created_at FROM todos WHERE user_id = ?`
	args := []any{uid}
	if gameID > 0 {
		q += ` AND game_id = ?`
		args = append(args, gameID)
	}
	rows, err := s.DB.QueryContext(ctx, q+` ORDER BY done, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Todo{}
	for rows.Next() {
		var t Todo
		var done int
		if err := rows.Scan(&t.ID, &t.UserID, &t.GameID, &t.Text, &done, &t.TaskURL, &t.Source, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Done = done == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTodo reads one to-do.
func (s *Store) GetTodo(ctx context.Context, uid, id int64) (*Todo, error) {
	var t Todo
	var done int
	if err := s.DB.QueryRowContext(ctx, `SELECT id, user_id, game_id, text, done, task_url, source, created_at FROM todos WHERE user_id = ? AND id = ?`, uid, id).
		Scan(&t.ID, &t.UserID, &t.GameID, &t.Text, &done, &t.TaskURL, &t.Source, &t.CreatedAt); err != nil {
		return nil, notFound(err)
	}
	t.Done = done == 1
	return &t, nil
}

// CreateTodo adds a to-do.
func (s *Store) CreateTodo(ctx context.Context, t *Todo) error {
	t.CreatedAt = Now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO todos (user_id, game_id, text, done, task_url, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.UserID, t.GameID, t.Text, boolInt(t.Done), t.TaskURL, t.Source, t.CreatedAt)
	if err != nil {
		return err
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

// UpdateTodo stores a to-do's text, whether it is done and its task.
func (s *Store) UpdateTodo(ctx context.Context, t *Todo) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE todos SET text = ?, done = ?, task_url = ? WHERE user_id = ? AND id = ?`, t.Text, boolInt(t.Done), t.TaskURL, t.UserID, t.ID)
	return affected(res, err)
}

// DeleteTodo removes a to-do.
func (s *Store) DeleteTodo(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM todos WHERE user_id = ? AND id = ?`, uid, id)
	return affected(res, err)
}

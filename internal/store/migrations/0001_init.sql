CREATE TABLE users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  email         TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL DEFAULT '',
  role          TEXT NOT NULL DEFAULT 'member',
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT
);

CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE api_tokens (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL DEFAULT '',
  token_hash   TEXT NOT NULL UNIQUE,
  prefix       TEXT NOT NULL DEFAULT '',
  last_used_at TEXT,
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Box-wide settings (title, setup flag); everything else is personal.
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE user_settings (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key     TEXT NOT NULL,
  value   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, key)
);

CREATE TABLE cache (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  fetched_at TEXT NOT NULL
);


-- ---------- Pocket Game Builder ----------
-- Everything belongs to the person who made it (user_id); nobody else on the
-- box sees it, box admins included. Times are UTC ISO strings; days are
-- local YYYY-MM-DD. Pictures, sounds and music live in
-- DATA_DIR/media/<user id>/; the game itself (scenes, objects, rules) is one
-- JSON document in games.doc, the same shape the editor and the player use.

CREATE TABLE games (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title             TEXT NOT NULL,
  description       TEXT NOT NULL DEFAULT '',
  template          TEXT NOT NULL DEFAULT '',          -- platformer | maze | shooter | bricks | catch | quiz | blank
  doc               TEXT NOT NULL DEFAULT '{}',        -- the game (JSON)
  saves             INTEGER NOT NULL DEFAULT 0,        -- how many times it was saved (the editor's version check)
  cover             TEXT NOT NULL DEFAULT '',          -- media path of a picture of the first scene (PNG)
  share_key         TEXT UNIQUE,                       -- /p/<key>, no account needed
  share_on          INTEGER NOT NULL DEFAULT 0,
  scores_on         INTEGER NOT NULL DEFAULT 1,        -- a best-scores table on the shared page
  remix_on          INTEGER NOT NULL DEFAULT 0,        -- people may take a copy of it
  published         TEXT NOT NULL DEFAULT '',          -- the game as shared (JSON); '' = the latest save
  published_at      TEXT,
  plays             INTEGER NOT NULL DEFAULT 0,        -- plays by others on the shared page
  favorite          INTEGER NOT NULL DEFAULT 0,
  archived          INTEGER NOT NULL DEFAULT 0,
  source            TEXT NOT NULL DEFAULT '',          -- '' here, else the Pocket app that made it
  played_at         TEXT,
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX games_user ON games(user_id, archived, updated_at);

-- Pictures (sprites, backgrounds), sounds and music. A drawn sprite keeps
-- its pixels too so it can be drawn on again.
CREATE TABLE assets (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  game_id     INTEGER REFERENCES games(id) ON DELETE SET NULL,  -- the game it was added for (all games may use it)
  kind        TEXT NOT NULL,                     -- image | sound | music
  name        TEXT NOT NULL DEFAULT '',
  mime        TEXT NOT NULL DEFAULT '',
  path        TEXT NOT NULL DEFAULT '',          -- media path
  size        INTEGER NOT NULL DEFAULT 0,
  width       INTEGER NOT NULL DEFAULT 0,
  height      INTEGER NOT NULL DEFAULT 0,
  frames      INTEGER NOT NULL DEFAULT 1,        -- a strip of animation frames side by side
  seconds     REAL NOT NULL DEFAULT 0,
  pixels      TEXT NOT NULL DEFAULT '',          -- drawn here: {w,h,palette,frames:[...]}
  origin      TEXT NOT NULL DEFAULT 'upload',    -- upload | drawn | photos | files | draw | photolab | music | voice | link
  origin_ref  TEXT NOT NULL DEFAULT '',
  source      TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX assets_user ON assets(user_id, kind, created_at);

-- Saved copies of a game to go back to (made on share, and when asked).
CREATE TABLE versions (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  game_id     INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  note        TEXT NOT NULL DEFAULT '',
  doc         TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX versions_game ON versions(user_id, game_id, created_at);

-- Every finished play: yours while testing, and other people's on the
-- shared page (kept with the game's maker; a player gives only a name).
CREATE TABLE plays (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  game_id     INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  who         TEXT NOT NULL DEFAULT '',          -- the name a player gave; '' = you
  mine        INTEGER NOT NULL DEFAULT 0,        -- played by the maker
  score       REAL NOT NULL DEFAULT 0,
  seconds     REAL NOT NULL DEFAULT 0,
  result      TEXT NOT NULL DEFAULT 'quit',      -- won | lost | quit
  level       TEXT NOT NULL DEFAULT '',          -- the scene it ended in
  day         TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX plays_game ON plays(user_id, game_id, created_at);
CREATE INDEX plays_day ON plays(user_id, day);

-- What is still to do on a game ("add a boss", "level 3 is too hard").
CREATE TABLE todos (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  game_id     INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
  text        TEXT NOT NULL,
  done        INTEGER NOT NULL DEFAULT 0,
  task_url    TEXT NOT NULL DEFAULT '',          -- the task in Pocket Tasks, when sent there
  source      TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX todos_game ON todos(user_id, game_id);

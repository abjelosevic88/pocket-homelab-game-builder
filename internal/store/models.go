package store

import (
	"encoding/json"
)

// User is an account on this box.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // admin | member
	CreatedAt    string `json:"created_at"`
}

// APIToken is a personal access token (hash stored, plaintext shown once).
type APIToken struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

// Conn is one person's connection to a sibling Pocket app.
type Conn struct {
	Mode      string `json:"mode,omitempty"`
	URL       string `json:"url,omitempty"`
	Token     string `json:"token,omitempty"`
	PublicURL string `json:"public_url,omitempty"`
}

// Settings are the box-wide values plus one person's preferences and their
// own connections.
type Settings struct {
	Title         string `json:"title"`
	SetupComplete bool   `json:"setup_complete"`

	DateFormat string `json:"date_format"`
	TimeFormat string `json:"time_format"`
	WeekStart  int    `json:"week_start"` // 1 = Monday, 7 = Sunday

	// Making games.
	MakerName   string `json:"maker_name"`   // "made by …" on shared games ('' = your first name)
	ScoresOn    bool   `json:"scores_on"`    // new shared games get a best-scores table
	AutoSave    bool   `json:"auto_save"`    // the editor saves as you go
	ShowGrid    bool   `json:"show_grid"`    // the level grid
	SoundOn     bool   `json:"sound_on"`     // sounds while testing
	DefaultSize string `json:"default_size"` // wide | tall | square | big

	// The other apps.
	FilesFolder string `json:"files_folder"` // where games are saved in Pocket Files
	TasksList   string `json:"tasks_list"`   // which list a game's to-dos go to ('' = the inbox)
	ChatTo      string `json:"chat_to"`      // the Pocket Chat conversation games are shared to
	AIHelp      bool   `json:"ai_help"`

	// Notifications.
	NotifyBest   bool   `json:"notify_best"`  // someone beat the best score on a shared game
	NotifyDaily  bool   `json:"notify_daily"` // once a day: how often your shared games were played
	NotifyHour   int    `json:"notify_hour"`
	NotifyURL    string `json:"notify_url"`
	NotifyFormat string `json:"notify_format"` // apprise | ntfy | json

	// Each sibling Pocket app, by slug.
	Conns map[string]Conn `json:"conns"`

	// AI: Pocket AI through Core, or an OpenAI-compatible API with the
	// person's own key.
	AIProvider    string `json:"ai_provider"` // auto | pocket | openai
	AIPocketModel string `json:"ai_pocket_model"`
	OpenAIKey     string `json:"openai_api_key"`
	OpenAIModel   string `json:"openai_model"`
	OpenAIBaseURL string `json:"openai_base_url"`
}

// ---- games ----

// Game is one game a person made. Doc is the game itself (JSON) and is only
// filled when one game is read.
type Game struct {
	ID          int64           `json:"id"`
	UserID      int64           `json:"-"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Template    string          `json:"template"`
	Doc         json.RawMessage `json:"doc,omitempty"`
	Saves       int             `json:"saves"`
	Cover       string          `json:"-"`
	HasCover    bool            `json:"has_cover"`
	ShareKey    string          `json:"share_key"`
	ShareOn     bool            `json:"share_on"`
	ScoresOn    bool            `json:"scores_on"`
	RemixOn     bool            `json:"remix_on"`
	Published   json.RawMessage `json:"-"`
	PublishedAt *string         `json:"published_at"`
	Plays       int             `json:"plays"`
	Favorite    bool            `json:"favorite"`
	Archived    bool            `json:"archived"`
	Source      string          `json:"source"`
	PlayedAt    *string         `json:"played_at"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	// Worked out when listing.
	Scenes  int     `json:"scenes"`
	Objects int     `json:"objects"`
	Rules   int     `json:"rules"`
	Best    float64 `json:"best"`
	MyPlays int     `json:"my_plays"`
	Todo    int     `json:"todo"`
}

// Asset is a picture, a sound or a piece of music.
type Asset struct {
	ID        int64   `json:"id"`
	UserID    int64   `json:"-"`
	GameID    *int64  `json:"game_id"`
	Kind      string  `json:"kind"`
	Name      string  `json:"name"`
	Mime      string  `json:"mime"`
	Path      string  `json:"-"`
	Size      int64   `json:"size"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Frames    int     `json:"frames"`
	Seconds   float64 `json:"seconds"`
	Pixels    string  `json:"pixels,omitempty"`
	Origin    string  `json:"origin"`
	OriginRef string  `json:"origin_ref"`
	Source    string  `json:"source"`
	CreatedAt string  `json:"created_at"`
}

// Version is a saved copy of a game.
type Version struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"-"`
	GameID    int64           `json:"game_id"`
	Note      string          `json:"note"`
	Doc       json.RawMessage `json:"doc,omitempty"`
	CreatedAt string          `json:"created_at"`
}

// Play is one finished play.
type Play struct {
	ID        int64   `json:"id"`
	UserID    int64   `json:"-"`
	GameID    int64   `json:"game_id"`
	Who       string  `json:"who"`
	Mine      bool    `json:"mine"`
	Score     float64 `json:"score"`
	Seconds   float64 `json:"seconds"`
	Result    string  `json:"result"`
	Level     string  `json:"level"`
	Day       string  `json:"day"`
	CreatedAt string  `json:"created_at"`
	Game      string  `json:"game,omitempty"`
}

// Todo is one thing still to do on a game.
type Todo struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"-"`
	GameID    int64  `json:"game_id"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
	TaskURL   string `json:"task_url"`
	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

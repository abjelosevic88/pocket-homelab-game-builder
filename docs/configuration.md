# Configuration

Environment variables; every one has a default. Personal settings (how the editor works, your name on shared games, notifications, connections, AI) live in the UI per person — the `OPENAI_*`/`NOTIFY_*` variables only seed the first admin's settings once.

| Variable | Default | What it does |
|---|---|---|
| `BASE_URL` | – | Where the app is (share links, pushes, events, other apps, the OIDC callback) |
| `HOST` / `PORT` | `0.0.0.0` / `8080` | Where it listens |
| `DATA_DIR` | `./data` | The database, pictures, sounds and music (`media/<user>/`) and the Pocket Core token |
| `DB_PATH` | `$DATA_DIR/pocket-gamebuilder.db` | The SQLite file |
| `AUTH_MODE` | `local` | `local` (passwords), `proxy` (a header from your reverse proxy), `none` (one user, for trying it out) |
| `SECURE_COOKIES` | `false` | Set `true` behind HTTPS |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` / `ADMIN_NAME` | – | The first account, made on an empty database |
| `OIDC_ISSUER` / `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` | – | Sign in with an OpenID Connect provider (not needed with Pocket Core) |
| `NOTIFY_URL` / `NOTIFY_FORMAT` | – / `apprise` | Pushes: an Apprise URL, an ntfy topic or a JSON webhook |
| `NOTIFY_HOUR` | `9` | Default hour the daily "your games were played" message goes at |
| `OPENAI_API_KEY` / `OPENAI_MODEL` / `OPENAI_BASE_URL` | – | AI without Pocket Core |
| `CORE_URL` / `CORE_REGISTRATION_KEY` / `CORE_APP_URL` | – | Pocket Core |
| `MAX_UPLOAD_BYTES` | `67108864` | The biggest picture, sound or music file (64 MB) |
| `TRUST_PROXY` | `true` | Take the client address from `X-Forwarded-For` |
| `METRICS_ENABLED` | `true` | Prometheus `/metrics` |
| `MAX_BODY_BYTES` | `67108864` | Largest JSON request (an export being brought back) |
| `DEMO_DATA` | `false` | Seed seven games (jump and run, maze, shooter, catch, quiz, brick breaker and one with drawn sprites), five of them shared, two months of plays by friends with best scores, and to-dos |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `text` | |

Connections to Pocket Photos, Files, Draw, Photo Lab, Music, Voice, Translate, Puzzlecraft, Blog, Chat, Forum, Email, Contacts, Tasks, Projects, Notes, Calendar, Family and Alerts are made per person under Settings → Connected apps (through Pocket Core with nothing to paste, or directly with a URL and an API token).

## Shared games

A shared game is at `BASE_URL/p/<key>` (with the page around it) and `BASE_URL/e/<key>` (just the game, for a frame on another site). They need no account and show only the version that was shared and the pictures and sounds it uses. Plays sent from one address to one game are limited to 20 a minute. If you want people outside your network to play, only these paths (and `/api/v1/public/…`, `/play/…`, `/favicon.svg`) need to be reachable.

# Backup

Everything lives in `DATA_DIR`: the SQLite file `pocket-gamebuilder.db` (WAL mode) and `media/<user id>/` — pictures, sounds, music and the games' cover pictures. Take a consistent copy of the database with `pocket-gamebuilder backup` (writes `DATA_DIR/backups/pocket-gamebuilder-<time>.db` with `VACUUM INTO`) or Settings → Backup & data → Download database (admin), and copy `media/` as it is. Back up that copy, not the live file. `DATA_DIR/core.token*` is the Pocket Core registration.

Restore: stop the app, replace `pocket-gamebuilder.db` (remove the `-wal`/`-shm` files) and `media/`, start.

Every person can also export everything they made — games, pictures and sounds (inside the file), saved copies, to-dos and plays — from Settings → Backup & data, and bring it back on any box. Importing the same file twice adds nothing twice. One game can also travel on its own as a `.game.json` (with its pictures and sounds) or as one HTML file that plays anywhere.

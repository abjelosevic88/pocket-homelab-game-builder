# Reverse proxy

Set `BASE_URL` to the public address and `SECURE_COOKIES=true` behind HTTPS. See [deploy/reverse-proxy/Caddyfile](../deploy/reverse-proxy/Caddyfile).

- **Uploads**: a picture, sound or piece of music comes in one request (up to `MAX_UPLOAD_BYTES`, 64 MB by default). If your proxy limits request bodies, allow at least that.
- **Shared games**: `/p/<key>`, `/e/<key>` and `/api/v1/public/…` work without signing in. They show only what was shared. Keep them reachable if you want people outside to play; everything else can stay private.
- **Embedding**: `/e/<key>` may be shown in a frame on any site (`frame-ancestors *`); the rest of the app may not.

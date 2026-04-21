# OpenCloud Music

> [!WARNING]
> ## 🚧 This does NOT work with any released OpenCloud 🚧
>
> The music service depends on OpenCloud changes that are **not yet in any release** and, in several cases, **not yet upstreamed**. A single released version that works end-to-end does not exist yet. Until the required pieces land, the only way to run this is against OpenCloud and `libre-graph-api-go` built from matching feature branches — expect broken listings, wrong counts, or empty responses against a stock build.

OpenSubsonic-compatible music server for [OpenCloud](https://opencloud.eu). It translates the [Subsonic API](https://opensubsonic.netlify.app/) that native music clients speak (Symfonium, play:Sub, Feishin, Substreamer, …) into OpenCloud Graph search + WebDAV calls, so any audio file in your OpenCloud spaces becomes streamable on any Subsonic-capable client.

No separate database, no duplicate index — the music service is a stateless translator between Subsonic and OpenCloud.

## Feature Status

**Working:**
- ID3-based browsing: `getMusicFolders`, `getArtists`, `getArtist`, `getAlbum`, `getSong`, `getGenres`
- Album lists: `getAlbumList2` (`newest`, `recent`, `random`, `alphabeticalByName`, `alphabeticalByArtist`, `byGenre`, `byYear`), `getRandomSongs`, `getSongsByGenre`
- Search: `search3` — artists, albums, songs in one call
- Streaming: `stream` and `download`, with HTTP `Range` so clients can seek
- Cover art: `getCoverArt` — proxies OpenCloud's preview service for embedded JPEG covers
- Auth / system: `ping`, `tokenInfo`, `getUser`, `getOpenSubsonicExtensions`, `getLicense`

**Stubbed (return empty envelopes so client logs stay clean; state is not persisted):**
- Annotations: `star` / `unstar` / `setRating` / `scrobble`
- Now playing — always empty
- Playlists — `getPlaylists` returns empty; create/update/delete are not implemented
- Podcasts, internet-radio stations, jukebox, bookmarks, chat messages — always empty

**Not implemented:**
- Transcoding — `stream` and `download` serve the original bytes
- Shares, user management, password change
- Legacy Subsonic token+salt HMAC auth — rejected with error 42 (use `u`+`p` or HTTP Basic)

## Quick Start

You need an existing OpenCloud instance reachable from the music service. The defaults assume it's on the Docker host at `https://localhost:9200` with a self-signed certificate.

### 1. Generate an app token

In OpenCloud, open the user settings → **App Tokens** → generate a new one. Copy the token value — it's shown only once.

### 2. Run the service

**Locally, without Docker:**

```bash
make generate                       # fetch OpenSubsonic spec and run oapi-codegen
MUSIC_HTTP_ADDR=:9111 \
OC_URL=https://localhost:9200 \
OC_INSECURE=true \
  go run ./cmd/music server
```

**With Docker Compose:**

```bash
make generate                       # produce internal/subsonic/generated.go
make frontend-install
make frontend-build                 # produce frontend/dist/
docker compose up --build
```

### 3. Configure your Subsonic client

| Field | Value |
|---|---|
| Server URL | `http://<host>:9111` |
| Type | **OpenSubsonic** |
| Username | your OpenCloud username |
| Password | the app token you generated in step 1 |

### 4. Smoke test

```bash
curl 'http://localhost:9111/rest/ping?f=json'
curl -u '<user>:<app-token>' 'http://localhost:9111/rest/tokenInfo?f=json'
curl -u '<user>:<app-token>' 'http://localhost:9111/rest/getArtists?f=json'
curl -u '<user>:<app-token>' 'http://localhost:9111/rest/search3?f=json&query=beatles'

# Prove that Range is honoured (seeking works in clients):
curl -u '<user>:<app-token>' \
     -H 'Range: bytes=0-1023' -o /tmp/song.bin \
     'http://localhost:9111/rest/stream?id=<resourceId>'
file /tmp/song.bin   # expect "Audio file ..."
```

## Configuration

| Env var | Purpose | Default |
|---|---|---|
| `MUSIC_HTTP_ADDR` | Address the service listens on. | `0.0.0.0:9111` |
| `OC_URL` | Base URL of the OpenCloud instance to proxy. | `https://host.docker.internal:9200` |
| `OC_INTERNAL_URL` | Optional internal URL for service-to-service traffic. Falls back to `OC_URL` when empty. | unset |
| `OC_INSECURE` | Skip TLS verification on calls to OpenCloud (self-signed dev certs). | `false` |
| `MUSIC_LOG_LEVEL` | `panic` / `fatal` / `error` / `warn` / `info` / `debug` / `trace`. `OC_LOG_LEVEL` takes precedence when set. | `info` |
| `MUSIC_DEBUG_ADDR` | Bind address of the debug / metrics server. | `127.0.0.1:9268` |

Authentication on the Subsonic endpoint is **HTTP Basic** with the OpenCloud app token — or the classic Subsonic `?u=<user>&p=<token>` / `?u=<user>&p=enc:<hex>` query variants. The legacy Subsonic `t`+`s` HMAC scheme is rejected with error 42 (OpenCloud app tokens never leave OpenCloud in plaintext, so there's nothing to hash against).

## Architecture

The service is a stateless translator: every Subsonic call is fanned out to one or two OpenCloud endpoints using the user's app token as HTTP Basic Auth, and the response is shaped back into a Subsonic envelope.

| Subsonic endpoint(s) | OpenCloud endpoint | What happens |
|---|---|---|
| `getArtists` / `getArtist` / `getAlbum` / `getGenres` / `getAlbumList2` / `search3` / `getSong` / `getRandomSongs` / `getSongsByGenre` | `POST /graph/v1beta1/search/query` | KQL query with nested aggregations (`audio.artist` → `audio.album` → `sum(audio.duration)`) so album counts and runtimes arrive in a single round-trip instead of N+1. |
| `tokenInfo` / `getUser` | `GET /graph/v1.0/me` | Used to confirm the app token and surface the caller's username. |
| `stream` / `download` | `GET /dav/spaces/<driveId>/<path>/<file>` | Reverse-proxied from the driveItem's `webDavUrl` (falling back to a URL synthesised from `parentReference` + `name` when the search hit doesn't carry one). Forwards `Range`, `If-Range`, `If-Modified-Since`, and `If-None-Match` so clients can seek and use conditional GETs. |
| `getCoverArt` | OpenCloud preview URL of a representative track | Resolves Subsonic artist (`ar-…`) / album (`al-…`) IDs to a driveItem, then proxies the preview service's JPEG. Song IDs use the driveItem directly. |
| `ping` / `getLicense` / `getOpenSubsonicExtensions` | — | Answered locally; no OpenCloud call. |

Code layout:

- `internal/subsonic/` — handlers that implement a `ServerInterface` generated from the upstream OpenSubsonic OpenAPI spec. The `generated.go` file is **not committed** — run `make generate` after checkout.
- `internal/subsonic/proto/` — response-envelope writer and protocol error codes. Kept separate so the auth middleware can emit Subsonic-formatted errors without depending on the generated types.
- `internal/auth/` — extracts the Subsonic credentials (HTTP Basic, `?u`+`?p`, or POST form) onto the request context; rejects legacy HMAC auth.
- `internal/graph/` — thin HTTP client for OpenCloud Graph.
- `internal/stream/` — the `Range`-aware reverse proxy used by `stream`, `download`, and `getCoverArt`.

## Development

```bash
# Backend
make generate                       # regenerate Subsonic server stubs
make build                          # build ./bin/opencloud-music
make run                            # build + run against https://localhost:9200
make test                           # go test ./...
make lint                           # golangci-lint

# Frontend (placeholder /music extension page)
make frontend-install
make frontend-serve                 # vite dev server, auto-registers in OpenCloud
make frontend-build                 # production build into frontend/dist/
make frontend-lint
make frontend-format-check
make frontend-typecheck
make frontend-test-unit

# Docker
make docker-up                      # docker compose up -d --build
make docker-down
```

## License

Apache-2.0

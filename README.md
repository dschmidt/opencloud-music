# OpenCloud Music

> [!WARNING]
> ## 🚧 This does NOT work with any released OpenCloud 🚧
>
> The music service depends on OpenCloud changes that are **not yet in any release** and, in several cases, **not yet upstreamed**. A single released version that works end-to-end does not exist yet. Until the required pieces land, the only way to run this is against OpenCloud and `libre-graph-api-go` built from matching feature branches — expect broken listings, wrong counts, or empty responses against a stock build.

OpenSubsonic-compatible music server for [OpenCloud](https://opencloud.eu). It translates the [Subsonic API](https://opensubsonic.netlify.app/) that native music clients speak (Symfonium, play:Sub, Feishin, Substreamer, …) into OpenCloud Graph search + WebDAV calls, so any audio file in your OpenCloud spaces becomes streamable on any Subsonic-capable client.

No separate database, no duplicate index — the music service is a stateless translator between Subsonic and OpenCloud.

## Feature Status

**Working:**
- ID3-based browsing (`getArtists`, `getArtist`, `getAlbum`, `getSong`, `getGenres`, `getMusicFolders`)
- Album lists (`getAlbumList2`: newest, recent, random, alphabetical, byGenre, byYear)
- Search (`search3` — artists, albums, songs in one call)
- Streaming (`stream`, with HTTP `Range` so clients can seek)
- App-token auth (`ping`, `tokenInfo`, `getUser`, `getOpenSubsonicExtensions`, `getLicense`)

**Stubbed (return `ok` but do nothing):**
- Star / unstar / set rating / scrobble / get now playing

**Not implemented (yet):**
- Cover art — always returns 404
- Playlists
- Transcoding
- Podcasts, jukebox, shares, bookmarks

## Quick Start

You need an existing OpenCloud instance reachable from the music service. The defaults assume it's on the Docker host at `https://localhost:9200` with a self-signed certificate.

### 1. Generate an app token

In OpenCloud, open the user settings → **App Tokens** → generate a new one. Copy the token value — it's shown only once.

### 2. Run the service

**Locally, without Docker:**

```bash
make generate                       # fetch OpenSubsonic spec and run oapi-codegen
MUSIC_HTTP_ADDR=:9110 \
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
| Server URL | `http://<host>:9110` |
| Type | **OpenSubsonic** |
| Username | your OpenCloud username |
| Password | the app token you generated in step 1 |

### 4. Smoke test

```bash
curl 'http://localhost:9110/rest/ping?f=json'
curl -u '<user>:<app-token>' 'http://localhost:9110/rest/tokenInfo?f=json'
curl -u '<user>:<app-token>' 'http://localhost:9110/rest/getArtists?f=json'
curl -u '<user>:<app-token>' 'http://localhost:9110/rest/search3?f=json&query=beatles'

# Prove that Range is honoured (seeking works in clients):
curl -u '<user>:<app-token>' \
     -H 'Range: bytes=0-1023' -o /tmp/song.bin \
     'http://localhost:9110/rest/stream?id=<resourceId>'
file /tmp/song.bin   # expect "Audio file ..."
```

## Configuration

| Env var | Purpose | Default |
|---|---|---|
| `MUSIC_HTTP_ADDR` | Address the service listens on. | `:9110` |
| `OC_URL` | Base URL of the OpenCloud instance to proxy. | *(required)* |
| `OC_INTERNAL_URL` | Optional internal URL (e.g. behind a service mesh). Falls back to `OC_URL`. | unset |
| `OC_INSECURE` | Skip TLS verification on calls to OpenCloud. Use for self-signed dev certs. | `false` |
| `MUSIC_LOG_LEVEL` | `debug` / `info` / `warn` / `error`. | `info` |

Authentication on the Subsonic endpoint is **HTTP Basic** with the OpenCloud app token; the legacy Subsonic `t`+`s` token scheme is rejected with error 40.

## Architecture

```
  Subsonic client
        │
        ▼
  opencloud-music                         OpenCloud
  ┌──────────────┐  Basic auth     ┌──────────────────┐
  │ /rest/*      │ ───────────────▶│ /graph/search    │
  │ (OpenSubsonic│                 │ /graph/me        │
  │  handlers)   │                 │ /remote.php/dav… │
  └──────┬───────┘                 └────────┬─────────┘
         │                                  │
         │ HTTP proxy with Range passthrough│
         └──────────────────────────────────┘
```

- Handlers in `internal/subsonic/` implement a `ServerInterface` generated from the upstream OpenSubsonic OpenAPI spec (the `generated.go` file is **not committed** — run `make generate` after checkout).
- The Graph client in `internal/graph/` issues KQL searches against `/graph/v1beta1/search/query`, using nested aggregations to collapse artists→albums→`sum(duration)` into a single round-trip.
- Streaming in `internal/stream/` is a zero-copy reverse proxy from `webDavUrl`, forwarding `Range`, `If-Range`, and `If-None-Match` so clients can seek freely.

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

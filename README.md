# OpenCloud Music

> [!WARNING]
> ## 🚧 This does NOT work with any released OpenCloud 🚧
>
> The music service depends on OpenCloud changes that are **not yet in any release** and, in several cases, **not yet upstreamed**. A single released version that works end-to-end does not exist yet. Until the required pieces land, the only way to run this is against:
>
> - `github.com/dschmidt/opencloud` @ `feat/graph-search-full`
> - `github.com/dschmidt/libre-graph-api` @ `feat/graph-search-full` (drives the regenerated `libre-graph-api-go` SDK)
>
> Expect broken listings, wrong counts, or empty responses against any stock build. See [Local dev stack (`bootstrap.sh`)](#local-dev-stack-bootstrapsh) for a one-command setup that pulls everything together.
>
> While the dust is still settling, **this repo may see force-pushes and history rewrites** on any branch, including `main`. Rebase your local clone rather than expecting fast-forward merges.

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
make backend-generate               # fetch OpenSubsonic spec and run oapi-codegen
cd backend
MUSIC_HTTP_ADDR=:9111 \
OC_URL=https://localhost:9200 \
OC_INSECURE=true \
  go run ./cmd/music server
```

**With Docker Compose:**

```bash
make backend-generate               # produce backend/internal/subsonic/generated.go
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

## Local dev stack (`bootstrap.sh`)

For contributors who don't already have a matching OpenCloud on hand, `./bootstrap.sh` pulls together every moving part — cloning the `feat/graph-search-full` branches of `opencloud` and `libre-graph-api`, regenerating `libre-graph-api-go` from the spec via a pinned `openapi-generator-cli` image, building both binaries with the canonical Makefile recipes, minting a self-signed TLS cert with the right SANs for your LAN, and starting OpenCloud + the music service in the background.

> [!NOTE]
> This is a **development** helper. It runs everything unsupervised from a scratch dir, hard-codes the dev LDAP / JWT / admin secrets, and trusts self-signed certs. **Do not use the output for anything but local testing.**

```bash
./bootstrap.sh              # first run: clones, generates, builds, starts OC + music
./bootstrap.sh              # reruns: kills prior processes via pid files, rebuilds, restarts
./bootstrap.sh --fresh      # wipe ./bootstrap/ first
./bootstrap.sh --prepare    # codegen only — used by `make docs`, no services started
./bootstrap.sh --stop       # kill running OC + music via pid files and exit
```

Everything dev-generated lives under `./bootstrap/` and can be removed with `rm -rf bootstrap/`. Defaults:

| Port | Service | Override |
|---|---|---|
| `9400` | OpenCloud proxy (HTTPS) | `OC_PORT=…` |
| `9411` | music backend (HTTP) | `MUSIC_PORT=…` |
| `19998` | Tika (audio metadata extraction) | `TIKA_PORT=…` |

Admin credentials are `admin` / `admin`. Logs land in `bootstrap/logs/{opencloud,music}.log`.

**Smoke test:** open `https://<your-lan-ip>:9400/` in the browser, accept the self-signed cert, log in, and upload a handful of audio files. Give Tika a few seconds to extract ID3 tags, then:

```bash
http --verify no -a admin:admin GET https://<your-lan-ip>:9400/rest/getArtists
```

should return a populated `<subsonic-response>` with your artists. Any Subsonic-compatible client (Symfonium, play:Sub, Feishin, …) pointed at the same URL with `admin` / `admin` will sync the library and play tracks.

> [!TIP]
> Some clients — Substreamer in particular — trip over self-signed TLS and refuse to connect even after you accept the cert in the browser. Point them at the music backend directly via plain HTTP on `http://<your-lan-ip>:9411` instead; that bypasses the OpenCloud proxy and the certificate altogether.

## Configuration

| Env var | Purpose | Default |
|---|---|---|
| `MUSIC_HTTP_ADDR` | Address the service listens on. | `0.0.0.0:9111` |
| `OC_URL` | Base URL of the OpenCloud instance to proxy. | `https://host.docker.internal:9200` |
| `OC_INTERNAL_URL` | Optional internal URL for service-to-service traffic. Falls back to `OC_URL` when empty. | unset |
| `OC_INSECURE` | Skip TLS verification on calls to OpenCloud (self-signed dev certs). | `false` |
| `MUSIC_LOG_LEVEL` | `panic` / `fatal` / `error` / `warn` / `info` / `debug` / `trace`. `OC_LOG_LEVEL` takes precedence when set. | `info` |
| `MUSIC_DEBUG_ADDR` | Bind address of the debug / metrics server. | `127.0.0.1:9268` |

Authentication on the Subsonic endpoint accepts three credential shapes:

1. `Authorization: Bearer <oidc-access-token>` — used by the bundled
   web UI extension, which already holds a token scoped to OpenCloud
   and just pipes it through. No companion username required; the
   token is forwarded verbatim to Graph + WebDAV.
2. `Authorization: Basic <base64(user:app-token)>`, or
   `?u=<user>&p=<app-token>` / `?u=<user>&p=enc:<hex>` — the native
   Subsonic credential shape; `p` carries an OpenCloud app token.
3. Same pair in a POST form body.

The legacy Subsonic `t`+`s` HMAC scheme is rejected with error 42
(OpenCloud app tokens never leave OpenCloud in plaintext, so there's
nothing to hash against).

### Wiring the web UI

The frontend extension at `/music` (in the OpenCloud web UI) calls the
backend at `/api/music/*`. For that to resolve, OpenCloud's proxy
needs an `additional_policies` entry pointing at the music service —
`dev/docker/opencloud.proxy.config.yaml` ships one suitable for local
dev.

OpenCloud reads its proxy policies from `$OC_CONFIG_DIR/proxy.yaml`
(default: `~/.opencloud/config/proxy.yaml`). Symlink our file in:

```bash
mkdir -p ~/.opencloud/config
ln -sf $PWD/dev/docker/opencloud.proxy.config.yaml \
       ~/.opencloud/config/proxy.yaml
```

…then restart OpenCloud. For the Docker flow, mount
`dev/docker/opencloud.proxy.config.yaml` at `/etc/opencloud/proxy.yaml`
on the OpenCloud container.

The route is declared `unprotected: true` so OpenCloud's proxy does
not revalidate or strip the Authorization header before forwarding —
the music backend does all the auth itself and would lose visibility
of the user's Bearer token otherwise.

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

- `backend/` — the Go service. Self-contained `go.mod`; build / test / lint via `make -C backend …` or the `backend-*` targets on the root Makefile.
- `backend/internal/subsonic/` — handlers that implement a `ServerInterface` generated from the upstream OpenSubsonic OpenAPI spec. The `generated.go` file is **not committed** — run `make backend-generate` after checkout.
- `backend/internal/subsonic/proto/` — response-envelope writer and protocol error codes. Kept separate so the auth middleware can emit Subsonic-formatted errors without depending on the generated types.
- `backend/internal/auth/` — extracts the Subsonic credentials (HTTP Basic, `?u`+`?p`, or POST form) onto the request context; rejects legacy HMAC auth.
- `backend/internal/graph/` — thin HTTP client for OpenCloud Graph.
- `backend/internal/stream/` — the `Range`-aware reverse proxy used by `stream`, `download`, and `getCoverArt`.
- `backend/internal/tools/bundle-openapi/` — pre-codegen spec bundler that inlines external `$ref`s so oapi-codegen emits named types (see commit `2076cd8` for why).
- `frontend/` — Vue 3 extension registered at `/music` inside OpenCloud's web UI. Scaffold only for now.

## Development

Every task has both a root-level `make <target>` and a backend-local
`make -C backend <target>` form; use whichever fits your workflow.

```bash
# Backend
make backend-generate               # regenerate Subsonic server stubs
make backend-build                  # build backend/bin/opencloud-music
make backend-run                    # build + run against https://localhost:9200
make backend-test                   # go test ./...
make backend-lint                   # golangci-lint
make backend-format                 # gofmt

# Frontend (placeholder /music extension page)
make frontend-install
make frontend-serve                 # vite dev server, auto-registers in OpenCloud
make frontend-build                 # production build into frontend/dist/
make frontend-lint
make frontend-format-check
make frontend-typecheck
make frontend-test-unit

# Everything
make format                         # frontend (prettier) + backend (gofmt)

# Docker
make docker-up                      # docker compose up -d --build
make docker-down

# Docs (service reference site — env vars, example config, deprecations)
make docs                           # build Docusaurus site into docs/generated/ (gitignored)
make docs-serve-prod                # serve the built site at http://localhost:3000
make docs-clean                     # wipe .cache/ + docs/generated/
```

## License

Apache-2.0

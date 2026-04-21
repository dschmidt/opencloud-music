# AGENTS.md

Instructions for agents working on this repo.

## Architecture in one paragraph

opencloud-music is an OpenSubsonic-compatible façade over OpenCloud's Graph search + WebDAV. It has no database of its own, no OIDC token exchange, no CS3 gateway — every request is translated on the fly into Graph API calls (KQL + aggregations) and WebDAV range-GETs. Auth on the Subsonic side is HTTP Basic with an OpenCloud app token, forwarded verbatim to OpenCloud.

## Backend dev workflow

- The Go service lives under `backend/` (self-contained `go.mod`), the Vue 3 extension under `frontend/`. The root `Makefile` delegates to `backend/Makefile` via `backend-*` targets; you can also run `make -C backend …` directly.
- Subsonic server stubs (`backend/internal/subsonic/generated.go`) are generated from a pinned OpenSubsonic OpenAPI commit and **not committed**. Run `make backend-generate` after checkout or whenever the pin in `backend/internal/subsonic/gen.go` is bumped.
- `backend/Makefile` targets (`build`, `test`, `lint`) all depend on `generate`, so you normally don't invoke `generate` directly.
- Handlers implement the generated `ServerInterface`. Route wiring lives in `backend/internal/subsonic/router.go`.
- The Graph client lives in `internal/graph/`. It uses nested aggregations (artist → album → `sum(duration)`) to avoid N+1 round trips against OpenCloud. If those return empty where they shouldn't, check whether the OpenCloud instance actually has the upstream sub-aggregation support built in.

## Frontend dev workflow

**Do not run `pnpm build` (or `make frontend-build`) during development.** In dev, the OpenCloud web UI is served by the [opencloud-eu/web](https://github.com/opencloud-eu/web) vite dev server running on port **9201**, which connects to your OpenCloud instance on 9200. The extension is loaded via the extension-sdk's auto-registration from `pnpm serve` in this repo — assume the user keeps both running.

- Production build (`pnpm build`) is only for CI and the Docker image.
- The bundled frontend dist mounted into OpenCloud is only for production/CI.
- The frontend is a placeholder — one `/music` view showing connection hints. End users configure Subsonic clients directly; the web UI isn't the product surface here.

## Tool usage

- Use `pnpm` (not `npx`, not `npm`) to invoke frontend tools.
- Prefer the Makefile targets over raw `pnpm` / `go` commands when possible.
- Formatting: `make format` runs gofmt; `make frontend-format` runs prettier.

## Env var conventions

- Service-specific vars use the bare service name uppercase: `MUSIC_HTTP_ADDR`, `MUSIC_LOG_LEVEL`. This matches every other OpenCloud service (`STORAGE_USERS_*`, `ACTIVITYLOG_*`, `SSE_*`, …).
- Cross-service shared vars use `OC_`: `OC_URL`, `OC_INSECURE`, `OC_LOG_LEVEL`.
- Never introduce an `OC_MUSIC_*` prefix — no other OpenCloud service does that.

## Commits

- Conventional commits (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`).
- No co-author attribution.
- Semantic branch names (`feat/…`, `fix/…`, `docs/…`, `chore/…`).
- Main branch is protected — all changes go through PRs. The `All Checks Passed` gate must be green before merge.

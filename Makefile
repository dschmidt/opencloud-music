.PHONY: frontend-install frontend-serve frontend-build frontend-lint frontend-format frontend-format-check frontend-typecheck frontend-test-unit backend-generate backend-build backend-run backend-test backend-lint backend-format backend-tidy format docker-up docker-down docs docs-serve-prod docs-clean

# --- Frontend ---

frontend-install:
	cd frontend && pnpm install

frontend-serve:
	cd frontend && pnpm serve

frontend-build:
	cd frontend && pnpm build

frontend-lint:
	cd frontend && pnpm lint

frontend-format:
	cd frontend && pnpm format:write

frontend-format-check:
	cd frontend && pnpm format:check

frontend-typecheck:
	cd frontend && pnpm check:types

frontend-test-unit:
	cd frontend && pnpm test:unit --watch=false

# --- Backend ---
# Delegates to backend/Makefile — keeps the Go module self-contained
# under backend/ and matches the layout of other OpenCloud extension
# repos (synaplan-opencloud etc.).

backend-generate:
	$(MAKE) -C backend generate

backend-build:
	$(MAKE) -C backend build

backend-run:
	$(MAKE) -C backend run

backend-test:
	$(MAKE) -C backend test

backend-lint:
	$(MAKE) -C backend lint

backend-format:
	$(MAKE) -C backend format

backend-tidy:
	$(MAKE) -C backend tidy

# --- All ---

format:
	$(MAKE) frontend-format
	$(MAKE) backend-format

# --- Docker ---

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

# --- Docs ---
# Service reference site (env vars, example config, deprecations).
# Runs dschmidt/opencloud-service-docs-action at the SHA pinned in
# .github/workflows/docs.yml — identical code path to CI.

docs:
	DOCS_OUTPUT="$(CURDIR)/docs/generated" bash .github/docs/run.sh

docs-serve-prod:
	cd .github/docs/.cache/site && pnpm run serve

docs-clean:
	rm -rf .github/docs/.cache docs/generated

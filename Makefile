.PHONY: help build run test lint format generate tidy frontend-install frontend-serve frontend-build frontend-lint frontend-format frontend-format-check frontend-typecheck frontend-test-unit docker-up docker-down

help:
	@echo "Backend:"
	@echo "  generate         regenerate Subsonic server stubs from the pinned OpenSubsonic spec"
	@echo "  build            build ./bin/opencloud-music (runs generate first)"
	@echo "  run              build + run the server with OC_URL=https://localhost:9200 insecure"
	@echo "  test             go test ./..."
	@echo "  lint             golangci-lint run"
	@echo "  format           gofmt -w ."
	@echo "  tidy             go mod tidy"
	@echo "Frontend:"
	@echo "  frontend-install     pnpm install in frontend/"
	@echo "  frontend-serve       pnpm serve  (vite dev, auto-registers in OpenCloud)"
	@echo "  frontend-build       pnpm build  (produces frontend/dist/ — CI / docker only)"
	@echo "  frontend-lint        pnpm lint"
	@echo "  frontend-format      pnpm format:write"
	@echo "  frontend-format-check pnpm format:check"
	@echo "  frontend-typecheck   pnpm check:types"
	@echo "  frontend-test-unit   pnpm test:unit --watch=false"
	@echo "Docker:"
	@echo "  docker-up        docker compose up -d --build"
	@echo "  docker-down      docker compose down"

# --- Backend ---

generate:
	go generate ./...

build: generate
	CGO_ENABLED=0 go build -o bin/opencloud-music ./cmd/music

run: build
	MUSIC_HTTP_ADDR=:9110 OC_URL=https://localhost:9200 OC_INSECURE=true ./bin/opencloud-music server

test: generate
	go test ./...

lint: generate
	golangci-lint run

format:
	gofmt -w .

tidy:
	go mod tidy

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

# --- Docker ---

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

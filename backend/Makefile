.PHONY: help build run test lint format generate tidy

help:
	@echo "  generate         regenerate Subsonic server stubs from the pinned OpenSubsonic spec"
	@echo "  build            build ./bin/opencloud-music (runs generate first)"
	@echo "  run              build + run the server with OC_URL=https://localhost:9200 insecure"
	@echo "  test             go test ./..."
	@echo "  lint             golangci-lint run"
	@echo "  format           gofmt -w ."
	@echo "  tidy             go mod tidy"

generate:
	go generate ./...

build: generate
	CGO_ENABLED=0 go build -o bin/opencloud-music ./cmd/music

run: build
	MUSIC_HTTP_ADDR=:9111 OC_URL=https://localhost:9200 OC_INSECURE=true ./bin/opencloud-music server

test: generate
	go test ./...

lint: generate
	golangci-lint run

format:
	gofmt -w .

tidy:
	go mod tidy

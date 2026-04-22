FROM golang:1.25-alpine@sha256:8e02eb337d9e0ea459e041f1ee5eece41cbb61f1d83e7d883a3e2fb4862063fa AS builder

# backend/go.mod's replace for libre-graph-api-go resolves to
# ../bootstrap/src/libre-graph-api-go-local — materialized by
# `./bootstrap.sh --prepare` on the host (or the CI job that
# downloaded the libregraph-go-local artifact before build).
WORKDIR /src
COPY bootstrap/src/libre-graph-api-go-local ./bootstrap/src/libre-graph-api-go-local
COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend
RUN go mod download
COPY backend/ .

# The Subsonic server stubs are generated from the pinned OpenSubsonic OpenAPI
# spec and are NOT committed. They must be generated on the host (via `make
# generate`) or downloaded as a CI artifact before running docker build.
RUN test -f internal/subsonic/model/generated.go || { \
  echo "ERROR: internal/subsonic/model/generated.go is missing."; \
  echo "Run 'make generate' on the host before docker build."; \
  exit 1; }

RUN CGO_ENABLED=0 go build -o /opencloud-music ./cmd/music

FROM alpine:3.20@sha256:a4f4213abb84c497377b8544c81b3564f313746700372ec4fe84653e4fb03805
COPY --from=builder /opencloud-music /usr/local/bin/opencloud-music
COPY frontend/dist/ /web/apps/music/
EXPOSE 9111
ENTRYPOINT ["opencloud-music", "server"]

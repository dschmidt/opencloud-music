FROM golang:1.25-alpine@sha256:8e02eb337d9e0ea459e041f1ee5eece41cbb61f1d83e7d883a3e2fb4862063fa AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# The Subsonic server stubs are generated from the pinned OpenSubsonic OpenAPI
# spec and are NOT committed. They must be generated on the host (via `make
# generate`) or downloaded as a CI artifact before running docker build.
RUN test -f internal/subsonic/generated.go || { \
  echo "ERROR: internal/subsonic/generated.go is missing."; \
  echo "Run 'make generate' on the host before docker build."; \
  exit 1; }

RUN CGO_ENABLED=0 go build -o /opencloud-music ./cmd/music

FROM alpine:3.20@sha256:a4f4213abb84c497377b8544c81b3564f313746700372ec4fe84653e4fb03805
COPY --from=builder /opencloud-music /usr/local/bin/opencloud-music
COPY frontend/dist/ /web/apps/music/
EXPOSE 9111
ENTRYPOINT ["opencloud-music", "server"]

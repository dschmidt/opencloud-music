#!/usr/bin/env bash
# bootstrap.sh — stand up a self-contained OpenCloud + music dev stack.
#
# Everything this script generates lives under ./bootstrap/ and can be
# thrown away with `rm -rf bootstrap/` (or `./bootstrap.sh --fresh`).
#
# Upstream state pulled in:
#   - github.com/dschmidt/opencloud      @ feat/graph-search-full
#   - github.com/dschmidt/libre-graph-api @ feat/graph-search-full
#   - libre-graph-api-go is NOT pushed; the script regenerates it from
#     the spec via docker openapi-generator-cli on every run.
#
# Flags:
#   --fresh     rm -rf bootstrap/ before anything
#   --prepare   spec clone + Go SDK generation + make backend-generate
#               only — what the docs action needs to compile backend/.
#               No OC build, no tika, no services started.
set -euo pipefail

# ── flags ──────────────────────────────────────────────────────────────
FRESH=0
PREPARE=0
for arg in "$@"; do
  case $arg in
    --fresh) FRESH=1 ;;
    --prepare) PREPARE=1 ;;
    -h|--help)
      sed -n '2,23p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) echo "unknown flag: $arg" >&2; exit 2 ;;
  esac
done

# ── local environment ──────────────────────────────────────────────────
ROOT="$(cd "$(dirname "$0")" && pwd)"
BS="$ROOT/bootstrap"
OC_PORT="${OC_PORT:-9400}"
MUSIC_PORT="${MUSIC_PORT:-9411}"
TIKA_PORT="${TIKA_PORT:-19998}"
# LAN IP used in OC_URL so phones on the network reach OIDC + music.
LAN_IP="${BOOTSTRAP_LAN_IP:-$(hostname -I 2>/dev/null | awk '{print $1}')}"
[[ -z "$LAN_IP" ]] && LAN_IP="127.0.0.1"

# ── upstream sources ───────────────────────────────────────────────────
# Forks at feat/graph-search-full carry the search-aggregation work
# not yet upstream. libregraph-go is regenerated from the spec locally.
OC_REPO="${OC_REPO:-https://github.com/dschmidt/opencloud.git}"
OC_BRANCH="${OC_BRANCH:-feat/graph-search-full}"
SPEC_REPO="${SPEC_REPO:-https://github.com/dschmidt/libre-graph-api.git}"
SPEC_BRANCH="${SPEC_BRANCH:-feat/graph-search-full}"
OPENAPI_GENERATOR_IMAGE="${OPENAPI_GENERATOR_IMAGE:-openapitools/openapi-generator-cli:v7.16.0}"

# ── preflight ──────────────────────────────────────────────────────────
require() { command -v "$1" >/dev/null 2>&1 || { echo "missing required tool: $1" >&2; exit 1; }; }
require git
require go
require docker
require make
require curl
if [[ $PREPARE -eq 0 ]]; then
  require openssl
  require pnpm
  require lsof
fi

# ── stop previous bootstrap-managed processes ─────────────────────────
# $BS/run/*.pid are written by each spawned wrapper before exec; on
# rerun we SIGTERM the prior process group (setsid gives each one its
# own). Runs BEFORE --fresh so we don't rm the pidfiles out from under
# ourselves. Stale pidfiles (pid gone, or no longer our binary) are
# silently dropped.
if [[ $PREPARE -eq 0 ]]; then
  stop_prev() {
    local name=$1
    local expect=$2
    local pidfile=$BS/run/$name.pid
    [[ -f $pidfile ]] || return 0
    local pid
    pid=$(<"$pidfile")
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null \
        && tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null | grep -q "$expect"; then
      local pgid; pgid=$(ps -o pgid= "$pid" | tr -d ' ')
      echo "[stop] killing previous $name (pid $pid, pgid $pgid)"
      kill -TERM "-$pgid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null
      for _ in $(seq 1 10); do kill -0 "$pid" 2>/dev/null || break; sleep 0.5; done
      kill -KILL "-$pgid" 2>/dev/null || true
    fi
    rm -f "$pidfile"
  }
  stop_prev opencloud "$BS/bin/opencloud"
  stop_prev music     "$ROOT/backend/bin/opencloud-music"
fi

# ── optional fresh reset ───────────────────────────────────────────────
if [[ $FRESH -eq 1 ]]; then
  echo "[fresh] removing $BS"
  rm -rf "$BS"
fi
mkdir -p "$BS"/{src,bin,config,data,certs,logs,run}

# ── refuse to start if OC_PORT / MUSIC_PORT is held by something else ─
if [[ $PREPARE -eq 0 ]]; then
  port_busy() {
    local port=$1
    if lsof -iTCP:"$port" -sTCP:LISTEN -t >/dev/null 2>&1; then
      local owner
      owner=$(lsof -iTCP:"$port" -sTCP:LISTEN -nP 2>/dev/null | awk 'NR==2 {print $1 " (pid " $2 ")"}')
      echo "port :$port is already bound by $owner — stop it or override via the corresponding env var" >&2
      return 1
    fi
    return 0
  }
  port_busy "$OC_PORT" || exit 1
  port_busy "$MUSIC_PORT" || exit 1
fi

# ── clone / fast-forward ───────────────────────────────────────────────
clone_or_update() {
  local repo=$1 dest=$2 branch=$3
  if [[ -d "$dest/.git" ]]; then
    echo "[clone] $(basename "$dest"): fetching $branch"
    git -C "$dest" fetch --depth=100 origin "$branch" >/dev/null 2>&1
    git -C "$dest" checkout -q "$branch" 2>/dev/null || git -C "$dest" checkout -q -b "$branch" "origin/$branch"
    git -C "$dest" reset --hard "origin/$branch" >/dev/null
  else
    echo "[clone] $repo @ $branch"
    git clone --quiet --branch "$branch" --depth=100 "$repo" "$dest"
  fi
}

clone_or_update "$SPEC_REPO" "$BS/src/libre-graph-api" "$SPEC_BRANCH"

# ── libregraph-go SDK generation ──────────────────────────────────────
echo "[sdk] generating libre-graph-api-go from spec via $OPENAPI_GENERATOR_IMAGE"
SDK_OUT="$BS/src/libre-graph-api-go-local"
rm -rf "$SDK_OUT"
mkdir -p "$SDK_OUT"
docker run --rm \
  -u "$(id -u):$(id -g)" \
  -v "$BS/src/libre-graph-api:/spec:ro" \
  -v "$SDK_OUT:/out" \
  "$OPENAPI_GENERATOR_IMAGE" \
  generate \
    -i /spec/api/openapi-spec/v1.0.yaml \
    -g go \
    -o /out \
    --git-user-id=opencloud-eu \
    --git-repo-id=libre-graph-api-go \
    --additional-properties=packageName=libregraph,withGoMod=true,isGoSubmodule=false \
    >/dev/null
# openapi-generator drops the package name into a subdir we don't want —
# lift the *.go + go.mod into the output root if present.
if [[ -d "$SDK_OUT/go" && ! -f "$SDK_OUT/go.mod" ]]; then
  mv "$SDK_OUT/go"/* "$SDK_OUT/"
  rmdir "$SDK_OUT/go"
fi

# ── prepare docs generator for `make docs` ───────────────────────────
#    Clone markdown-docs-generator (same repo + pinned ref the docs
#    action would fetch anyway) and inject a replace for
#    libre-graph-api-go into its go.mod. The docs action's own build.sh
#    only injects a replace for the *service* module, so its subsequent
#    `go mod tidy` would otherwise fail fetching the unreleased
#    pseudo-version backend/go.mod requires.
DOCS_CFG="$ROOT/.github/docs/opencloud-service.yml"
DOCS_GEN_DIR="$ROOT/.github/docs/.cache/generator"
if [[ -f "$DOCS_CFG" ]]; then
  GEN_REF="$(awk '/^[[:space:]]*generator_ref:/{print $2; exit}' "$DOCS_CFG" | tr -d '"')"
  if [[ -n "$GEN_REF" ]]; then
    if [[ ! -d "$DOCS_GEN_DIR/.git" ]] || [[ "$(git -C "$DOCS_GEN_DIR" rev-parse HEAD 2>/dev/null)" != "$GEN_REF" ]]; then
      echo "[docs] cloning markdown-docs-generator @ $GEN_REF"
      rm -rf "$DOCS_GEN_DIR"
      mkdir -p "$(dirname "$DOCS_GEN_DIR")"
      git clone --quiet https://github.com/opencloud-eu/markdown-docs-generator.git "$DOCS_GEN_DIR"
      git -C "$DOCS_GEN_DIR" checkout --quiet "$GEN_REF"
    fi
    echo "[docs] injecting libre-graph-api-go replace into generator go.mod"
    (
      cd "$DOCS_GEN_DIR"
      go mod edit -dropreplace=github.com/opencloud-eu/libre-graph-api-go 2>/dev/null || true
      go mod edit -replace="github.com/opencloud-eu/libre-graph-api-go=$SDK_OUT"
    )
  fi
fi

# ── music backend codegen ─────────────────────────────────────────────
echo "[backend] make generate"
make -s -C "$ROOT/backend" generate >/dev/null

if [[ $PREPARE -eq 1 ]]; then
  echo
  echo "[prepare] done. backend/ is ready for 'go build' and the docs action."
  exit 0
fi

# ── full flow ─────────────────────────────────────────────────────────
clone_or_update "$OC_REPO" "$BS/src/opencloud" "$OC_BRANCH"

echo "[build] opencloud (make generate + make build)"
# The IDP service embeds HTML templates (assets/identifier/index.html)
# produced by `make generate` — a pnpm build of each service's UI
# bundle. A bare `go build` skips this and the resulting binary
# fatals on first request. Run the canonical build recipe instead:
#   repo-root/make generate   -> pnpm builds per-service assets
#   opencloud/make build      -> go build with the right tags/ldflags
(
  cd "$BS/src/opencloud"
  make -s generate
  make -s -C opencloud build
)
cp "$BS/src/opencloud/opencloud/bin/opencloud" "$BS/bin/opencloud"

echo "[build] music backend"
make -s -C "$ROOT/backend" build >/dev/null

echo "[build] music frontend"
make -s -C "$ROOT" frontend-install >/dev/null 2>&1 || (cd "$ROOT/frontend" && pnpm install >/dev/null)
make -s -C "$ROOT" frontend-build >/dev/null

# Wire the built frontend into OC's apps dir so the /music extension
# loads inside the web UI.
mkdir -p "$BS/data/web/assets/apps"
ln -sfn "$ROOT/frontend/dist" "$BS/data/web/assets/apps/music"

# ── TLS cert with SAN:localhost + 127.0.0.1 + LAN IP + serverAuth EKU ─
if [[ ! -f "$BS/certs/server.crt" ]]; then
  echo "[tls] generating cert covering localhost / 127.0.0.1 / $LAN_IP"
  cat > "$BS/certs/openssl.cnf" <<EOF
[req]
distinguished_name = req_dn
x509_extensions = v3_server
prompt = no
[req_dn]
O = OpenCloud Music Dev
CN = OpenCloud
[v3_server]
subjectAltName = @alt
basicConstraints = critical, CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
[alt]
DNS.1 = localhost
IP.1  = 127.0.0.1
IP.2  = $LAN_IP
EOF
  openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes \
    -keyout "$BS/certs/server.key" \
    -out    "$BS/certs/server.crt" \
    -config "$BS/certs/openssl.cnf" \
    -extensions v3_server 2>/dev/null
  chmod 600 "$BS/certs/server.key"
fi

# ── OC config files ───────────────────────────────────────────────────
cat > "$BS/config/proxy.yaml" <<EOF
# Proxy /api/music and /rest to the music backend, unprotected so the
# caller's Authorization header (Bearer or Basic) reaches music
# verbatim. Music handles the forwarding to Graph itself.
additional_policies:
  - name: default
    routes:
      - endpoint: /api/music
        backend: http://127.0.0.1:$MUSIC_PORT
        unprotected: true
      - endpoint: /rest
        backend: http://127.0.0.1:$MUSIC_PORT
        unprotected: true
EOF

cat > "$BS/config/apps.yaml" <<EOF
music:
  config: {}
EOF

# ── Tika (bootstrap dev port, won't clash with other tika instances) ──
echo "[tika] docker compose up -d tika (127.0.0.1:$TIKA_PORT)"
(cd "$ROOT" && TIKA_PORT="$TIKA_PORT" docker compose up -d tika >/dev/null 2>&1)

# ── opencloud runner ──────────────────────────────────────────────────
cat > "$BS/run-opencloud.sh" <<EOF
#!/usr/bin/env bash
set -euo pipefail
# Logs + secrets
export OC_LOG_LEVEL=debug
export OC_LOG_PRETTY=true
export OC_LOG_COLOR=true
export OC_INSECURE=true
export PROXY_ENABLE_BASIC_AUTH=true
export IDM_CREATE_DEMO_USERS=true
export OC_ADMIN_USER_ID="some-admin-user-id-0000-000000000000"
export IDM_ADMIN_PASSWORD=admin
export OC_SYSTEM_USER_ID="some-system-user-id-000-000000000000"
export OC_SYSTEM_USER_API_KEY="some-system-user-machine-auth-api-key"
export OC_JWT_SECRET="some-opencloud-jwt-secret"
export OC_MACHINE_AUTH_API_KEY="some-opencloud-machine-auth-api-key"
export OC_TRANSFER_SECRET="some-opencloud-transfer-secret"
export COLLABORATION_WOPI_SECRET="some-wopi-secret"
export IDM_SVC_PASSWORD="some-ldap-idm-password"
export GRAPH_LDAP_BIND_PASSWORD="some-ldap-idm-password"
export IDM_REVASVC_PASSWORD="some-ldap-reva-password"
export GROUPS_LDAP_BIND_PASSWORD="some-ldap-reva-password"
export USERS_LDAP_BIND_PASSWORD="some-ldap-reva-password"
export AUTH_BASIC_LDAP_BIND_PASSWORD="some-ldap-reva-password"
export IDM_IDPSVC_PASSWORD="some-ldap-idp-password"
export IDP_LDAP_BIND_PASSWORD="some-ldap-idp-password"
export GATEWAY_STORAGE_USERS_MOUNT_ID="storage-users-1"
export STORAGE_USERS_MOUNT_ID="storage-users-1"
export GRAPH_APPLICATION_ID="application-1"
export OC_SERVICE_ACCOUNT_ID="service-account-id"
export OC_SERVICE_ACCOUNT_SECRET="service-account-secret"
# Tika — the bootstrap-managed instance on a non-default port
export SEARCH_EXTRACTOR_TYPE=tika
export SEARCH_EXTRACTOR_TIKA_TIKA_URL="http://127.0.0.1:$TIKA_PORT"
export SEARCH_EXTRACTOR_CS3SOURCE_INSECURE=true
# Self-contained data + config under bootstrap/
export OC_BASE_DATA_PATH="$BS/data"
export OC_CONFIG_DIR="$BS/config"
export WEB_ASSET_APPS_PATH="$BS/data/web/assets/apps"
# Advertise the LAN URL so phones on the network can resolve OIDC + music
export OC_URL="https://${LAN_IP}:$OC_PORT"
# Proxy is the only externally-visible port; everything else is internal.
export PROXY_HTTP_ADDR="0.0.0.0:$OC_PORT"
export PROXY_TRANSPORT_TLS_CERT="$BS/certs/server.crt"
export PROXY_TRANSPORT_TLS_KEY="$BS/certs/server.key"
echo \$\$ > "$BS/run/opencloud.pid"
exec "$BS/bin/opencloud" server
EOF
chmod +x "$BS/run-opencloud.sh"

echo "[run] starting opencloud on :$OC_PORT — tail $BS/logs/opencloud.log"
setsid nohup "$BS/run-opencloud.sh" >"$BS/logs/opencloud.log" 2>&1 </dev/null &
disown
for i in $(seq 1 60); do
  if curl -sk "https://127.0.0.1:$OC_PORT/status.php" 2>/dev/null | grep -q installed; then
    echo "[run] opencloud ready after ${i}s"
    break
  fi
  sleep 1
done

# ── music runner ──────────────────────────────────────────────────────
cat > "$BS/run-music.sh" <<EOF
#!/usr/bin/env bash
set -euo pipefail
export MUSIC_HTTP_ADDR=":$MUSIC_PORT"
export OC_URL="https://${LAN_IP}:$OC_PORT"
export OC_INTERNAL_URL="https://127.0.0.1:$OC_PORT"
export OC_INSECURE=true
export MUSIC_LOG_LEVEL=debug
export OC_LOG_PRETTY=true
export OC_LOG_COLOR=true
echo \$\$ > "$BS/run/music.pid"
exec "$ROOT/backend/bin/opencloud-music" server
EOF
chmod +x "$BS/run-music.sh"

echo "[run] starting music on :$MUSIC_PORT — tail $BS/logs/music.log"
setsid nohup "$BS/run-music.sh" >"$BS/logs/music.log" 2>&1 </dev/null &
disown
sleep 2

# ── summary ───────────────────────────────────────────────────────────
cat <<EOF

Bootstrap complete.

  Web UI     : https://${LAN_IP}:$OC_PORT/          (admin / admin)
  Music tab  : https://${LAN_IP}:$OC_PORT/music
  Subsonic   : https://${LAN_IP}:$OC_PORT/          Type: OpenSubsonic
               (or http://${LAN_IP}:$MUSIC_PORT/ direct — same backend)

  Logs       : $BS/logs/{opencloud,music}.log
  State dir  : $BS/   (rm -rf to wipe everything)
  Reset      : ./bootstrap.sh --fresh
  Docs prep  : ./bootstrap.sh --prepare  (no services started)
EOF

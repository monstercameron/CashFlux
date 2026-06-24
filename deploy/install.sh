#!/usr/bin/env sh
# CashFlux self-host installer — one command, any fresh Debian/Ubuntu VPS.
#
#   curl -fsSL https://raw.githubusercontent.com/monstercameron/CashFlux/main/deploy/install.sh | sh
#
# Installs Docker (if missing), fetches the repo, generates server token material +
# a master key, writes deploy/cashflux-server.env, and brings up the self-host
# compose stack (cashflux-server + Caddy TLS). Host-agnostic: no DigitalOcean or
# any vendor dependency — this is the unconditional, referral-free deploy path.
set -eu

REPO_URL="${CASHFLUX_REPO_URL:-https://github.com/monstercameron/CashFlux.git}"
INSTALL_DIR="${CASHFLUX_INSTALL_DIR:-/opt/cashflux}"
COMPOSE_FILE="docker-compose.selfhost.yml"

log() { printf '\033[1;32m[cashflux]\033[0m %s\n' "$1"; }

need_root() {
  if [ "$(id -u)" -ne 0 ]; then
    echo "Please run as root (or via sudo): the installer manages Docker + /opt." >&2
    exit 1
  fi
}

install_docker() {
  if command -v docker >/dev/null 2>&1; then
    log "Docker already present."
    return
  fi
  log "Installing Docker via get.docker.com ..."
  curl -fsSL https://get.docker.com | sh
}

fetch_repo() {
  if [ -d "$INSTALL_DIR/.git" ]; then
    log "Updating existing checkout in $INSTALL_DIR ..."
    git -C "$INSTALL_DIR" pull --ff-only
  else
    log "Cloning CashFlux into $INSTALL_DIR ..."
    git clone --depth 1 "$REPO_URL" "$INSTALL_DIR"
  fi
}

gen_secret() { head -c 32 /dev/urandom | base64 | tr -d '\n'; }

write_env() {
  ENV_FILE="$INSTALL_DIR/deploy/cashflux-server.env"
  if [ -f "$ENV_FILE" ]; then
    log "Env file already exists — leaving it untouched."
    return
  fi
  log "Generating server token + master key ..."
  cp "$INSTALL_DIR/deploy/cashflux-server.env.example" "$ENV_FILE"
  MASTER_KEY="$(gen_secret)"
  # rotate-token derives + prints the token and its sha256; capture the sha for the env.
  TOKEN_OUT="$(cd "$INSTALL_DIR" && docker compose -f "$COMPOSE_FILE" run --rm \
    -e CASHFLUX_SERVER_MASTER_KEY="$MASTER_KEY" cashflux-server rotate-token 2>/dev/null || true)"
  SHA="$(printf '%s' "$TOKEN_OUT" | grep -iEo '[a-f0-9]{64}' | head -n1 || true)"
  sed -i "s|^CASHFLUX_SERVER_MASTER_KEY=.*|CASHFLUX_SERVER_MASTER_KEY=$MASTER_KEY|" "$ENV_FILE"
  [ -n "$SHA" ] && sed -i "s|^CASHFLUX_SERVER_TOKEN_SHA256=.*|CASHFLUX_SERVER_TOKEN_SHA256=$SHA|" "$ENV_FILE"
  log "Wrote $ENV_FILE — edit CASHFLUX_DOMAIN / CASHFLUX_TLS_EMAIL before going live."
  printf '\n%s\n' "$TOKEN_OUT"
  echo ">>> Save the access token above; it is shown only once."
}

up() {
  log "Starting the self-host stack ..."
  (cd "$INSTALL_DIR" && docker compose -f "$COMPOSE_FILE" up -d)
  log "Done. Set your DNS A record to this host, then browse to https://<CASHFLUX_DOMAIN>."
}

need_root
install_docker
fetch_repo
write_env
up

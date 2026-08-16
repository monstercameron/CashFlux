#!/usr/bin/env bash
# Move CashFlux to a published image tag, health-gated.
#
#   deploy-release.sh <tag> [app-dir]
#
# Runs ON THE DROPLET. Pins the tag into compose.yaml, pulls, restarts, and then
# WAITS for the container to report healthy — failing loudly if it never does.
# That wait is the whole point: without it a deploy reports success while the
# container crash-loops, which is how a service silently stops updating while
# every push still looks like it shipped.
#
# Rollback is the same command with the previous tag, which this script records
# before it changes anything:
#
#   deploy-release.sh "$(cat /opt/CashFlux/deploy/production/.previous-tag)"
set -euo pipefail

TAG="${1:-}"
APP_DIR="${2:-/opt/CashFlux/deploy/production}"
COMPOSE_FILE="${APP_DIR}/compose.yaml"
CONTAINER="cashflux"
IMAGE_REPO="ghcr.io/monstercameron/cashflux"
HEALTH_RETRIES="${HEALTH_RETRIES:-30}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-10}"

die() { echo "FAILED: $*" >&2; exit 1; }
log() { echo "[$(date -u +%H:%M:%S)] $*"; }

[ -n "${TAG}" ] || die "usage: deploy-release.sh <tag> [app-dir]"
[ -f "${COMPOSE_FILE}" ] || die "no compose file at ${COMPOSE_FILE}"

# Refuse a tag that is not actually published. Without this the pull fails
# halfway through a restart and the service is left down rather than simply not
# upgraded.
log "verifying ${IMAGE_REPO}:${TAG} exists in the registry"
docker manifest inspect "${IMAGE_REPO}:${TAG}" >/dev/null 2>&1 \
  || die "image ${IMAGE_REPO}:${TAG} is not published"

# Record what is running now, BEFORE touching the compose file, so a rollback
# target survives even if everything after this point fails.
CURRENT_TAG="$(sed -n "s|^[[:space:]]*image:[[:space:]]*${IMAGE_REPO}:\(.*\)$|\1|p" "${COMPOSE_FILE}" | head -1)"
if [ -n "${CURRENT_TAG}" ] && [ "${CURRENT_TAG}" != "${TAG}" ]; then
  echo "${CURRENT_TAG}" > "${APP_DIR}/.previous-tag"
  log "previous tag ${CURRENT_TAG} recorded for rollback"
fi
if [ "${CURRENT_TAG}" = "${TAG}" ]; then
  log "already pinned to ${TAG}; re-pulling and restarting anyway"
fi

# Anchored on the image NAME so this can never rewrite some other image line.
sed -i "s|\(image:[[:space:]]*${IMAGE_REPO}\):.*|\1:${TAG}|" "${COMPOSE_FILE}"
log "compose pinned to ${TAG}"

log "pulling"
docker compose -f "${COMPOSE_FILE}" pull

log "starting"
docker compose -f "${COMPOSE_FILE}" up -d --remove-orphans

# Wait for healthy. A container that is "running" has proved nothing: the
# process started. Healthy means it answered its own version endpoint.
log "waiting for ${CONTAINER} to report healthy (up to $((HEALTH_RETRIES * HEALTH_INTERVAL))s)"
for i in $(seq 1 "${HEALTH_RETRIES}"); do
  status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${CONTAINER}" 2>/dev/null || echo missing)"
  case "${status}" in
    healthy)
      log "healthy after $((i * HEALTH_INTERVAL))s"
      log "now running: $(docker inspect -f '{{.Config.Image}}' "${CONTAINER}")"
      exit 0
      ;;
    unhealthy)
      echo "--- last 50 log lines ---" >&2
      docker logs --tail 50 "${CONTAINER}" >&2 2>&1 || true
      die "container went unhealthy; roll back with: $0 \$(cat ${APP_DIR}/.previous-tag)"
      ;;
    none)
      die "container has no healthcheck defined — refusing to call this a successful deploy"
      ;;
  esac
  sleep "${HEALTH_INTERVAL}"
done

echo "--- last 50 log lines ---" >&2
docker logs --tail 50 "${CONTAINER}" >&2 2>&1 || true
die "never reported healthy; roll back with: $0 \$(cat ${APP_DIR}/.previous-tag)"

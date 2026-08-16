#!/usr/bin/env bash
# Decide whether CashFlux should move to a newer release, and if so, do it.
#
# Runs ON THE DROPLET, triggered two ways:
#   - the webhook receiver, when a release workflow POSTs /internal/deploy-hook
#   - a daily systemd timer, as the fallback for a webhook delivery that is lost
#
# The webhook carries NO tag and NO command — only a shared secret proving the
# request came from the repo's Actions. Everything about WHICH version to run is
# decided here, on the box, from git tags and the registry. That is deliberate:
# it means a compromised Actions run can ask for a deploy but cannot choose what
# gets deployed.
set -euo pipefail

REPO_DIR="${CASHFLUX_REPO_DIR:-/opt/CashFlux}"
APP_DIR="${CASHFLUX_APP_DIR:-/opt/CashFlux/deploy/production}"
COMPOSE_FILE="${APP_DIR}/compose.yaml"
IMAGE_REPO="ghcr.io/monstercameron/cashflux"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"; }
die() { echo "FAILED: $*" >&2; exit 1; }

cd "${REPO_DIR}" || die "no repo at ${REPO_DIR}"

log "fetching tags"
# --force and --prune-tags, because a tag that MOVED will otherwise wedge this
# permanently: plain `git fetch --tags` refuses with "would clobber existing
# tag" and exits non-zero, so every subsequent deploy dies at this line while
# the webhook still answers 200. That is the same class of silent failure this
# whole pull-deploy exists to avoid, and it is not hypothetical — re-pointing
# v1.8.0 at a buildable commit did exactly that to this box.
#
# Forcing is safe here: this checkout is a read-only mirror used to decide which
# version to run. It never authors tags, so origin is always the truth.
git fetch --tags --force --prune-tags --quiet origin 2>&1 || die "git fetch failed"

# Newest semver tag, by version order rather than commit date — a hotfix tagged
# today on an older branch must not be treated as "newest".
LATEST="$(git tag -l 'v*' --sort=-v:refname | head -1)"
[ -n "${LATEST}" ] || die "no v* tags found"

CURRENT="$(sed -n "s|^[[:space:]]*image:[[:space:]]*${IMAGE_REPO}:\(.*\)$|\1|p" "${COMPOSE_FILE}" | head -1)"
log "pinned=${CURRENT:-none} latest=${LATEST}"

if [ "${CURRENT}" = "${LATEST}" ]; then
  log "already on ${LATEST}; nothing to do"
  exit 0
fi

# Confirm the image is actually published before touching anything. The tag can
# exist in git minutes before the image finishes building and pushing, and the
# webhook fires on the tag push — so this is the common case, not an edge one.
if ! docker manifest inspect "${IMAGE_REPO}:${LATEST}" >/dev/null 2>&1; then
  log "image ${IMAGE_REPO}:${LATEST} not published yet; leaving ${CURRENT} running"
  log "the daily timer will retry, or re-POST the hook once the build finishes"
  exit 0
fi

log "deploying ${LATEST}"
exec "${APP_DIR}/deploy-release.sh" "${LATEST}" "${APP_DIR}"

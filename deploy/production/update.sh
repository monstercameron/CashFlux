#!/usr/bin/env bash
# Build, back up, deploy, and verify the native CashFlux service.
set -euo pipefail

if [ "${CASHFLUX_DEPLOY_REEXEC:-}" != "1" ]; then
	stable_script="$(mktemp /tmp/cashflux-deploy.XXXXXX)"
	cp "$0" "$stable_script"
	chmod 0700 "$stable_script"
	CASHFLUX_DEPLOY_REEXEC=1 exec "$stable_script" "$@"
fi
trap 'rm -f "$0"' EXIT

repo="${CASHFLUX_REPO:-/opt/CashFlux}"
service="${CASHFLUX_SERVICE:-cashflux}"
owner="${CASHFLUX_USER:-cashflux}"
env_file="${CASHFLUX_ENV_FILE:-/etc/cashflux/cashflux.env}"
health_url="${CASHFLUX_HEALTH_URL:-http://127.0.0.1:8105/readyz}"
public_origin="${CASHFLUX_PUBLIC_ORIGIN:-https://budget.earlcameron.com}"
backup_root="${CASHFLUX_RELEASE_BACKUPS:-/var/backups/cashflux/releases}"
go_bin="${GO:-/usr/local/go/bin/go}"
ref="main"
force=""

while [ "$#" -gt 0 ]; do
	case "$1" in
		--ref) ref="${2:?--ref needs a branch, tag, or commit}"; shift 2 ;;
		--force) force=1; shift ;;
		-h|--help)
			sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'
			exit 0
			;;
		*) echo "unknown option: $1" >&2; exit 2 ;;
	esac
done

[ "$(id -u)" -eq 0 ] || { echo "run this deployment as root" >&2; exit 1; }
[ -d "$repo/.git" ] || { echo "missing CashFlux checkout at $repo" >&2; exit 1; }
[ -x "$go_bin" ] || { echo "missing Go toolchain at $go_bin" >&2; exit 1; }
[ -r "$env_file" ] || { echo "missing production environment at $env_file" >&2; exit 1; }
systemctl cat "$service.service" >/dev/null

say() { printf '==> %s\n' "$*"; }
run_as_cashflux() {
	runuser -u "$owner" -- /bin/bash -c \
		'set -a; source "$1"; shift; exec "$@"' bash "$env_file" "$@"
}

old_sha="$(git -C "$repo" rev-parse HEAD)"
say "fetching $ref"
git -C "$repo" fetch --quiet origin --prune --tags
target_ref="$ref"
if git -C "$repo" rev-parse --verify --quiet "refs/remotes/origin/$ref" >/dev/null; then
	target_ref="refs/remotes/origin/$ref"
fi
target="$(git -C "$repo" rev-parse --verify "${target_ref}^{commit}")"
if [ "$old_sha" = "$target" ] && [ -z "$force" ]; then
	say "already at ${target:0:12}; use --force to rebuild"
	exit 0
fi
git -C "$repo" checkout --quiet --detach "$target"
new_sha="$(git -C "$repo" rev-parse HEAD)"

avail_mb="$(df -Pm "$repo" | awk 'NR == 2 {print $4}')"
[ "$avail_mb" -gt 1500 ] || { echo "a release build needs at least 1.5 GB free" >&2; exit 1; }

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$backup_root"
if [ -x "$repo/bin/cashflux-server" ] && [ -f /var/lib/cashflux/cashflux-server.db ]; then
	say "taking a verified data backup"
	run_as_cashflux "$repo/bin/cashflux-server" backup
fi

rollback_dir="$backup_root/$stamp"
if [ -d "$repo/bin" ]; then
	mkdir -p "$rollback_dir"
	cp -a "$repo/bin/." "$rollback_dir/"
fi

build_dir="$repo/bin.new"
rm -rf "$build_dir"
mkdir -p "$build_dir/web/bin" "$build_dir/web/admin" "$build_dir/web/portal"
cp -a "$repo/web/." "$build_dir/web/"
rm -f \
	"$build_dir/web/bin/main.wasm" \
	"$build_dir/web/bin/services.wasm" \
	"$build_dir/web/admin/admin.wasm" \
	"$build_dir/web/portal/portal.wasm"

export GOCACHE="${GOCACHE:-/var/cache/cashflux-go}"
mkdir -p "$GOCACHE"
say "building browser applications"
(
	cd "$repo"
	GOOS=js GOARCH=wasm CGO_ENABLED=0 "$go_bin" build -trimpath -ldflags="-s -w" \
		-o "$build_dir/web/bin/main.wasm" .
	GOOS=js GOARCH=wasm CGO_ENABLED=0 "$go_bin" build -trimpath -ldflags="-s -w" \
		-o "$build_dir/web/bin/services.wasm" ./cmd/cashflux-services
	GOOS=js GOARCH=wasm CGO_ENABLED=0 "$go_bin" build -trimpath -ldflags="-s -w" \
		-o "$build_dir/web/admin/admin.wasm" ./cmd/cashflux-admin
	GOOS=js GOARCH=wasm CGO_ENABLED=0 "$go_bin" build -trimpath -ldflags="-s -w" \
		-o "$build_dir/web/portal/portal.wasm" ./cmd/cashflux-portal
)

goroot="$("$go_bin" env GOROOT)"
wasm_exec="$goroot/lib/wasm/wasm_exec.js"
[ -f "$wasm_exec" ] || wasm_exec="$goroot/misc/wasm/wasm_exec.js"
[ -f "$wasm_exec" ] || { echo "wasm_exec.js is missing under $goroot" >&2; exit 1; }
cp "$wasm_exec" "$build_dir/web/wasm_exec.js"
cp "$wasm_exec" "$build_dir/web/admin/wasm_exec.js"
cp "$wasm_exec" "$build_dir/web/portal/wasm_exec.js"

say "building server"
(
	cd "$repo"
	CGO_ENABLED=0 "$go_bin" build -trimpath -buildvcs=true -ldflags="-s -w" \
		-o "$build_dir/cashflux-server" ./cmd/cashflux-server
)

for asset in \
	cashflux-server \
	web/index.html \
	web/wasm_exec.js \
	web/bin/main.wasm \
	web/bin/services.wasm \
	web/admin/index.html \
	web/admin/admin.wasm \
	web/portal/index.html \
	web/portal/portal.wasm
do
	[ -s "$build_dir/$asset" ] || { echo "release is missing $asset" >&2; exit 1; }
done
chmod 0755 "$build_dir/cashflux-server"
chown -R "$owner:$owner" "$build_dir"

say "checking migrations with the incoming binary"
run_as_cashflux "$build_dir/cashflux-server" migrate-check

rollback() {
	say "deployment failed; restoring the previous release"
	if [ -d "$rollback_dir" ]; then
		rm -rf "$repo/bin"
		cp -a "$rollback_dir" "$repo/bin"
		chown -R "$owner:$owner" "$repo/bin"
	fi
	systemctl reset-failed "$service.service" || true
	systemctl restart "$service.service" || true
}
trap rollback ERR

say "swapping release and restarting"
rm -rf "$repo/bin.old"
[ -d "$repo/bin" ] && mv "$repo/bin" "$repo/bin.old"
mv "$build_dir" "$repo/bin"
systemctl reset-failed "$service.service" || true
systemctl restart "$service.service"

deadline="$(( $(date +%s) + 120 ))"
until curl --fail --silent --show-error --max-time 5 --output /dev/null "$health_url"; do
	[ "$(date +%s)" -lt "$deadline" ] || { echo "CashFlux did not become ready" >&2; false; }
	sleep 2
done

say "verifying same-origin release assets"
curl --fail --silent --show-error --max-time 30 \
	-H 'Accept: text/html' http://127.0.0.1:8105/ |
	grep -q 'cashflux-hosted-app'
curl --fail --silent --show-error --max-time 30 \
	http://127.0.0.1:8105/v1/version |
	grep -q '"hostedApp":true'
for path in /bin/main.wasm /bin/services.wasm /console/ /portal/; do
	code="$(curl --silent --output /dev/null --write-out '%{http_code}' --max-time 30 \
		-H 'Accept: text/html' "http://127.0.0.1:8105$path")"
	[ "$code" = 200 ] || { echo "$path returned HTTP $code" >&2; false; }
done

trap - ERR
rm -rf "$repo/bin.old"
find "$backup_root" -mindepth 1 -maxdepth 1 -type d -printf '%T@ %p\n' |
	sort -nr |
	tail -n +11 |
	cut -d' ' -f2- |
	xargs -r rm -rf
say "deployed ${new_sha:0:12} to $public_origin"

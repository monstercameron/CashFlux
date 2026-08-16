# `budget.earlcameron.com` production deployment

This directory is the native systemd/Nginx deployment for the shared
`Earl-Cameron-dot-com` droplet. It intentionally does not replace the generic
Docker/Caddy self-host path.

## Layout

- Source checkout and built release: `/opt/CashFlux`
- Runtime state: `/var/lib/cashflux`
- Verified backup mirror: `/var/backups/cashflux`
- Root-owned, service-readable secrets: `/etc/cashflux/cashflux.env`
- Loopback listener: `127.0.0.1:8105`
- Public origin: `https://budget.earlcameron.com`

The service runs as the unprivileged `cashflux` user. Nginx is the only public
listener and must preserve the `/grpc` WebSocket upgrade.

## First install

Create the user and private directories:

```sh
sudo useradd --system --home /var/lib/cashflux --shell /usr/sbin/nologin cashflux
sudo install -d -o root -g root -m 0755 /opt/CashFlux
sudo install -d -o cashflux -g cashflux -m 0700 /var/lib/cashflux
sudo install -d -o cashflux -g cashflux -m 0700 /var/backups/cashflux
sudo install -d -o root -g cashflux -m 0750 /etc/cashflux
sudo git clone https://github.com/monstercameron/CashFlux.git /opt/CashFlux
```

Copy `cashflux.env.example` to `/etc/cashflux/cashflux.env`, replace every
secret placeholder, then keep it `root:cashflux` mode `0640`. Generate the
break-glass token with `cashflux-server rotate-token`, store only its SHA-256
digest in the environment, and put the plaintext token in a password manager.
Registration must stay closed:

```env
CASHFLUX_SERVER_REGISTRATION_OPEN=false
```

Install the service and maintenance units:

```sh
sudo install -m 0644 deploy/production/cashflux*.service /etc/systemd/system/
sudo install -m 0644 deploy/production/cashflux*.timer /etc/systemd/system/
sudo install -m 0755 deploy/production/cashflux-health.sh /usr/local/bin/cashflux-health
sudo systemctl daemon-reload
sudo systemctl enable cashflux.service cashflux-health.timer \
  cashflux-backup.timer cashflux-retention.timer cashflux-blob-gc.timer
sudo deploy/production/update.sh --force
sudo systemctl start cashflux-health.timer cashflux-backup.timer \
  cashflux-retention.timer cashflux-blob-gc.timer
```

Create the DNS A record before requesting TLS. Bootstrap Nginx with an HTTP-only
vhost, run `certbot --nginx -d budget.earlcameron.com`, then install
`nginx-budget.earlcameron.com.conf` and run `nginx -t` before reloading.

## Deploy and rollback

`update.sh` fetches the requested ref, takes a consistent application backup,
builds all four browser WASMs plus the server, checks migrations, atomically
swaps the release, and verifies readiness and the same-origin assets. A failed
restart or verification restores the preceding `bin` directory.

```sh
sudo /opt/CashFlux/deploy/production/update.sh
sudo /opt/CashFlux/deploy/production/update.sh --ref <tag-or-commit>
```

The ten newest previous releases remain under
`/var/backups/cashflux-releases`. Data backups remain under
`/var/lib/cashflux/backups` and are mirrored to `/var/backups/cashflux`.

## Deploy hook

After the `CI` workflow succeeds on `main`, the existing deploy-hook service can
invoke the updater in a transient root service. Add a third target to
`/etc/deployhook/config.json` with the repository's webhook secret:

```json
{
  "repo": "monstercameron/CashFlux",
  "workflow": "CI",
  "branch": "main",
  "secret": "<github-webhook-secret>",
  "command": "/usr/bin/systemd-run",
  "args": [
    "--pipe",
    "--wait",
    "--collect",
    "--unit=deploy-cashflux",
    "--property=TimeoutStartSec=1800",
    "/opt/CashFlux/deploy/production/update.sh"
  ]
}
```

Reloading `deployhook.service` does not affect ArticleFlux or the portfolio
targets already in that configuration.

## Verification

```sh
systemctl --no-pager --full status cashflux.service
curl -fsS -o /dev/null https://budget.earlcameron.com/readyz
curl -fsS https://budget.earlcameron.com/v1/version
systemctl start cashflux-backup.service
journalctl -u cashflux-backup.service -n 30 --no-pager
```

The production browser pass must also prove client login, console owner login,
account CRUD, access request/approval, password recovery, first sync, and
logout refresh-family revocation.

`e2e/regression/production-auth.spec.mjs` is the opt-in destructive smoke that
performs that pass with disposable accounts and removes them in `finally`.
Provide the public origin, break-glass token, and owner credentials through
`CASHFLUX_PRODUCTION_URL`, `CASHFLUX_PRODUCTION_TOKEN`,
`CASHFLUX_PRODUCTION_OWNER_USERNAME`, and
`CASHFLUX_PRODUCTION_OWNER_PASSWORD`, set `E2E_BASE_URL` to the same origin,
then run that one spec with `e2e/playwright.config.mjs`. Never put the values in
the repository or command history.

## Docker migration (in progress)

CashFlux is moving from `cashflux.service` (systemd + a binary at
`/opt/CashFlux/bin`) to a container pulled from `ghcr.io`, matching how
AnimeFeedFlux and the portfolio already run on this droplet.

The cutover is deliberately conservative. The container listens on **the same
port** nginx already proxies (8105), reads **the same env file** the systemd unit
read (`/etc/cashflux/cashflux.env`), and mounts **the same data directory**
(`/var/lib/cashflux`, a bind — not a named volume — so the live database and its
33 backup generations carry over 1:1). Nothing outside `compose.yaml` changes,
and rollback is `docker compose down && systemctl start cashflux`.

### How a release reaches the box

1. Tag `vX.Y.Z` and push it. `.github/workflows/release.yml` builds
   `Dockerfile.server` for **linux/amd64** (the maintainer's machine is arm64 —
   an image built there runs nowhere here), pushes `ghcr.io/monstercameron/cashflux:vX.Y.Z`
   plus a `sha-` tag, and refuses to publish if the tag disagrees with
   `internal/version/version.go`.
2. It then POSTs `https://budget.earlcameron.com/internal/deploy-hook` with a
   shared-secret header. **No tag and no command travel with that request** — it
   only says "something new may exist".
3. nginx forwards it to the `webhook` daemon on 127.0.0.1:9309, which runs
   `autoupdate/cashflux-autoupdate.sh`. That script decides for itself: newest
   `v*` git tag, confirmed published via `docker manifest inspect`, else it
   leaves the running version alone. A compromised Actions run can therefore ask
   for a deploy but cannot choose what gets deployed.
4. `deploy-release.sh` records the outgoing tag to `.previous-tag`, pins the new
   one, pulls, restarts, and then **waits for the container to report healthy** —
   failing loudly with the last 50 log lines if it does not. That gate is the
   point: without it a deploy reports success while the container crash-loops.
5. `cashflux-autoupdate.timer` runs the same check daily, so a lost webhook
   delivery costs a day rather than going unnoticed.

### First-time setup on the droplet

```sh
# 1. secret, shared with the repo's CASHFLUX_DEPLOY_HOOK_SECRET Actions secret
openssl rand -hex 32

# 2. append the cashflux-deploy object from autoupdate/webhook.conf.example
#    to the existing /etc/webhook.conf array, then:
systemctl reload webhook            # does not disturb the other targets

# 3. nginx: the /internal/deploy-hook location is in
#    nginx-budget.earlcameron.com.conf
nginx -t && systemctl reload nginx

# 4. autoupdate units
install -m 0755 autoupdate/cashflux-autoupdate.sh /opt/CashFlux/deploy/production/autoupdate/
install -m 0644 autoupdate/cashflux-autoupdate.{service,timer} /etc/systemd/system/
systemctl daemon-reload && systemctl enable --now cashflux-autoupdate.timer
```

### The cutover itself

```sh
id cashflux                          # confirm 993:984 matches compose.yaml `user:`
systemctl stop cashflux              # release the port and the SQLite WAL
./deploy-release.sh vX.Y.Z          # pull, start, health-gate
curl -fsS https://budget.earlcameron.com/v1/version
systemctl disable cashflux           # only once the container has proved itself
```

Stopping the systemd unit first is not optional: two processes must never hold
the same SQLite database, and the port would collide anyway.

### Rollback

```sh
./deploy-release.sh "$(cat .previous-tag)"   # previous image
# or, all the way back:
docker compose -f compose.yaml down && systemctl start cashflux
```

### What is running right now?

```sh
docker inspect -f '{{.Config.Image}}' cashflux
docker exec cashflux /usr/local/bin/cashflux-server version
```

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
`/var/backups/cashflux/releases`. Data backups remain under
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

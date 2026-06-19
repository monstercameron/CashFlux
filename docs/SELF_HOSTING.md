# CashFlux Self-Hosting

CashFlux can run with an optional backend for sync and AI proxying. The web app stays local-first; the server stores sync snapshots, blobs, encrypted BYO OpenAI keys, and auth state.

## Quickstart

1. Copy the environment template:

   ```sh
   cp deploy/cashflux-server.env.example deploy/cashflux-server.env
   ```

2. Edit `deploy/cashflux-server.env`:
   - `CASHFLUX_DOMAIN`: the hostname pointed at this server.
   - `CASHFLUX_TLS_EMAIL`: email for Caddy ACME notices.
   - `CASHFLUX_SERVER_APP_ORIGIN`: the browser app origin, usually `https://<domain>`.
   - `CASHFLUX_SERVER_MASTER_KEY`: 16, 24, or 32 bytes. Keep it secret and stable.

3. Generate a self-host access token:

   ```sh
   docker compose -f docker-compose.selfhost.yml run --rm cashflux-server rotate-token
   ```

   Put the printed `CASHFLUX_SERVER_TOKEN_SHA256` value in `deploy/cashflux-server.env`. Save the printed `CASHFLUX_SERVER_TOKEN` somewhere private; users paste that token into CashFlux Settings for this server.

4. Start the stack:

   ```sh
   docker compose -f docker-compose.selfhost.yml up -d --build
   ```

5. Check the server:

   ```sh
   curl -i https://<domain>/healthz
   curl -i https://<domain>/readyz
   ```

The gRPC tunnel is exposed at `wss://<domain>/grpc` through Caddy. The client derives that tunnel URL from the HTTPS server URL.

## Local HTTP Development

For local development without TLS, run the server directly:

```sh
CASHFLUX_SERVER_ADDR=127.0.0.1:8081 \
CASHFLUX_SERVER_DATA_DIR=.tmp/server-dev \
CASHFLUX_SERVER_MASTER_KEY=0123456789abcdef0123456789abcdef \
CASHFLUX_SERVER_TOKEN=dev-token \
CASHFLUX_SERVER_APP_ORIGIN=http://127.0.0.1:8080 \
go run ./cmd/cashflux-server
```

Use `http://127.0.0.1:8081` and `dev-token` in Settings. The browser still uses the GoGRPCBridge tunnel; `http://` is converted to `ws://.../grpc` by the client.

## OAuth Mode

Token mode is the default single-user self-host path. OAuth is optional and currently requires provider configuration:

```env
CASHFLUX_SERVER_AUTH_MODE=oauth
CASHFLUX_SERVER_OAUTH_GOOGLE_CLIENT_ID=...
CASHFLUX_SERVER_OAUTH_GOOGLE_CLIENT_SECRET=...
CASHFLUX_SERVER_OAUTH_GOOGLE_REDIRECT_URL=https://<domain>/v1/auth/google/callback
```

Google and GitHub provider variables are supported. OAuth callback/session completion is still tracked in `TODOS.md`; token mode is the working self-host path today.

## Backups

The server data volume contains:

- `cashflux-server.db`
- SQLite WAL/SHM files when active.
- Blob files under the configured data directory.

For a consistent manual backup:

1. Run the built-in backup command:

   ```sh
   docker compose -f docker-compose.selfhost.yml run --rm cashflux-server backup /data/backups
   ```

   The command opens the store, checkpoints SQLite WAL, copies `cashflux-server.db` and `blobs/` into a timestamped
   `cashflux-backup-YYYYMMDDTHHMMSSZ` directory, and writes `manifest.json` with file SHA-256 digests plus RPO/RTO notes.

2. Copy the generated backup directory off-box and encrypt it at rest. The repository includes
   `deploy/cashflux-backup.example.service` and `deploy/cashflux-backup.example.timer` as a nightly systemd example:

   ```sh
   sudo cp deploy/cashflux-backup.example.service /etc/systemd/system/cashflux-backup.service
   sudo cp deploy/cashflux-backup.example.timer /etc/systemd/system/cashflux-backup.timer
   sudo systemctl edit cashflux-backup.service
   sudo systemctl enable --now cashflux-backup.timer
   ```

   Set `WorkingDirectory`, `CASHFLUX_BACKUP_DIR`, and `CASHFLUX_OFFBOX_TARGET` for your host. The example uses `rclone`
   for off-box sync; leave `CASHFLUX_OFFBOX_TARGET` blank until the remote is tested.

Restore rehearsal should happen at least quarterly:

1. Stop the stack.
2. Copy `cashflux-server.db` and `blobs/` from a backup directory into an empty `cashflux-data` volume.
3. Start the stack.
4. Verify `https://<domain>/readyz` returns 204 and spot-check sync from a non-primary device.

Operational objective: RPO is the last successful scheduled backup; RTO is the time to restore the backup directory, start the server, and verify `/readyz`.

## Upgrades

```sh
git pull
docker compose -f docker-compose.selfhost.yml up -d --build
```

Migrations run on server startup. Back up the data volume before upgrading. The server rejects a database schema newer than it supports.

## TLS And Proxy Notes

Caddy terminates TLS and proxies websocket upgrades to the server. Keep ports `80` and `443` reachable for automatic certificates. If you use another reverse proxy, preserve websocket upgrades and forward `Host`, `X-Forwarded-Host`, and `X-Forwarded-Proto`.

Do not expose a default token or example master key in production. Generate real token material with `rotate-token`, set the SHA-256 digest in the env file, and store the plaintext token in a password manager.

## Container Runtime Hardening

The Compose stack runs the CashFlux server as the non-root `cashflux` user with a read-only root filesystem, a writable `/data` volume, a small hardened `/tmp` tmpfs, all Linux capabilities dropped, and `no-new-privileges` enabled. Caddy also runs with a read-only root filesystem and drops all capabilities except `NET_BIND_SERVICE` so it can bind ports 80/443.

The self-host Compose file also sets explicit runtime ceilings. The server is capped at 1 CPU, 512 MB memory, 256 PIDs, and 4096 open files; Caddy is capped at 0.5 CPU, 256 MB memory, 128 PIDs, and 2048 open files. Server-side backpressure is controlled by the env template's HTTP max-in-flight/rate-limit knobs and gRPC connection/stream caps, so excess work is rejected instead of growing unbounded queues.

If you add a volume or sidecar, keep writable paths explicit and prefer read-only mounts. Do not add broad capabilities or privileged mode for normal operation.

## Reliability Knobs

AI proxy calls use `CASHFLUX_SERVER_AI_UPSTREAM_TIMEOUT` and `CASHFLUX_SERVER_AI_UPSTREAM_RETRIES` to keep upstream OpenAI failures bounded. The defaults are a 45 second deadline and two retries for transient `429`/`5xx` responses or transport errors.

Blob disk reads and writes use `CASHFLUX_SERVER_BLOB_IO_TIMEOUT` so request cancellation and slow storage do not hang blob endpoints indefinitely. The default is 10 seconds.

For status-page and incident handling, expose `/status` through the same TLS host and use `docs/INCIDENT_RESPONSE.md` for severity levels, update cadence, recovery, and postmortems.

## Logging

Set `CASHFLUX_SERVER_LOG_FORMAT=json` for structured production logs, or `text` for local development. `CASHFLUX_SERVER_LOG_LEVEL` accepts `debug`, `info`, `warn`, and `error`. Sensitive attributes such as tokens, keys, secrets, cookies, passwords, and authorization values are redacted before logs are written.

Successful hot-path probes (`/livez`, `/healthz`, `/readyz`, and `/metrics`) are sampled with `CASHFLUX_SERVER_LOG_HOT_PATH_SAMPLE_RATE` to keep production logs useful under frequent health checks. Set it to `1` to log every probe.

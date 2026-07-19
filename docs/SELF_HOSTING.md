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

## Connect CashFlux Settings

After the stack is healthy, open CashFlux Settings and choose the self-hosted backend option.

1. Set the server URL to `https://<domain>`; do not include `/grpc` or `/v1`.
2. Paste the printed `CASHFLUX_SERVER_TOKEN` into the access token field.
3. Use Test connection before saving. The app calls `/v1/version` for compatibility and derives
   `wss://<domain>/grpc` for sync and AI proxy RPCs.

Keep the printed token private. The server stores only `CASHFLUX_SERVER_TOKEN_SHA256` in production config; the
plaintext token belongs in the user's password manager and CashFlux Settings only.

## Deploy Link Disclosure

CashFlux may offer a DigitalOcean referral deploy link from the app or install docs. If you use that link,
DigitalOcean may grant account credit that offsets CashFlux hosting costs. The plain self-host path in this
document always works without a referral link, and self-hosting remains free to run on any host.

## Deployment Surface

The backend ships as one `cashflux-server` binary with a configured data directory. The container image builds
that binary from `cmd/cashflux-server`, stores SQLite/blobs/backups under `CASHFLUX_SERVER_DATA_DIR`, and reads
runtime configuration from environment variables. The self-host Compose stack pairs the server with Caddy for
automatic TLS and mounts a single persistent `cashflux-data` volume for the database, blob store, and generated
backup directories.

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
Before rebuilding, run `docker compose -f docker-compose.selfhost.yml run --rm cashflux-server migrate-check`
to apply migrations to a temporary database copy and confirm the target binary can migrate safely.

For recurring operations, use `docs/OPERATIONS_RUNBOOK.md` for deploy, rollback, restore, key rotation, session revocation, and past-due handling. Use `docs/OBSERVABILITY.md` for metrics/logs and `docs/INCIDENT_RESPONSE.md` during incidents.

For capacity planning, use `docs/SCALE_LIMITS.md`. The self-host backend intentionally starts with SQLite; do not expand hosted multi-tenant usage without measuring the single-writer ceiling and migration triggers.

## TLS And Proxy Notes

Caddy terminates TLS, redirects HTTP to HTTPS automatically, and proxies websocket upgrades to the server. Keep
ports `80` and `443` reachable for automatic certificates. The bundled `deploy/Caddyfile.selfhost` pins modern
TLS 1.2/1.3 protocols and AEAD cipher suites, keeps upstream HTTP connections alive, allows
long-lived `/grpc` websocket streams, and delays stream close during reloads so browser sync/watch streams can
survive normal proxy restarts. If you use another reverse proxy, preserve websocket upgrades, avoid short idle
timeouts on `/grpc`, and forward `Host`, `X-Forwarded-Host`, and `X-Forwarded-Proto`.

When running behind a reverse proxy, set `CASHFLUX_SERVER_TRUSTED_PROXIES` to the proxy's address(es)
(comma-separated CIDRs or bare IPs). IP-based rate limiting trusts `X-Forwarded-For` / `X-Real-IP` **only**
when the direct connection comes from a trusted proxy; otherwise those headers are attacker-controlled and are
ignored in favor of the socket peer address. Without this, a proxied deployment would rate-limit every client
under the proxy's single IP; a directly internet-facing deployment must leave it empty so clients cannot spoof
`X-Forwarded-For` to bypass the limiter. The server speaks plain HTTP and expects the proxy to terminate TLS —
never expose it directly without one.

Do not expose a default token or example master key in production. Generate real token material with `rotate-token`, set the SHA-256 digest in the env file, and store the plaintext token in a password manager.

Set `CASHFLUX_SERVER_MASTER_KEY` from a secret manager, KMS-backed secret, or password manager entry that is
available only to the deploy process. It must be exactly 16, 24, or 32 bytes; prefer 32 bytes. This key
encrypts stored BYO AI keys with AES-GCM and must not be committed to the repository, copied into tickets, or
stored beside backups.

Master-key rotation requires a maintenance window: back up the server, pause AI key writes, set the new
`CASHFLUX_SERVER_MASTER_KEY`, set `CASHFLUX_SERVER_OLD_MASTER_KEY` for one `cashflux-server rotate-ai-master-key`
run so stored BYO AI keys are re-encrypted under the new key, then remove the old-key environment variable
before restart. Use
`docs/OPERATIONS_RUNBOOK.md#rotate-master-key` for the step-by-step procedure.

## Container Runtime Hardening

The Compose stack runs the CashFlux server as the non-root `cashflux` user with a read-only root filesystem, a writable `/data` volume, a small hardened `/tmp` tmpfs, all Linux capabilities dropped, and `no-new-privileges` enabled. Caddy also runs with a read-only root filesystem and drops all capabilities except `NET_BIND_SERVICE` so it can bind ports 80/443.

The self-host Compose file also sets explicit runtime ceilings. The server is capped at 1 CPU, 512 MB memory, 256 PIDs, and 4096 open files; Caddy is capped at 0.5 CPU, 256 MB memory, 128 PIDs, and 2048 open files. Server-side backpressure is controlled by the env template's HTTP max-in-flight/rate-limit knobs and gRPC connection/stream caps, so excess work is rejected instead of growing unbounded queues.

If you add a volume or sidecar, keep writable paths explicit and prefer read-only mounts. Do not add broad capabilities or privileged mode for normal operation.

## Release Artifacts

Use `deploy/release-server.example.sh` as the starting point for backend releases. It builds the server with
deterministic Go flags (`CGO_ENABLED=0`, `-trimpath`, VCS stamping, and an empty Go build id), writes
`SHA256SUMS`, generates a CycloneDX JSON SBOM for `cmd/cashflux-server`, and signs both the binary and SBOM
with `cosign sign-blob`.

Keep the Go toolchain version, `GOOS`, `GOARCH`, source checkout, and dependency cache fixed when comparing
reproducible builds. Publish the checksum, `.cdx.json` SBOM, and `.sig` files beside the binary or container
image digest.

## Reliability Knobs

AI proxy calls use `CASHFLUX_SERVER_AI_UPSTREAM_TIMEOUT` and `CASHFLUX_SERVER_AI_UPSTREAM_RETRIES` to keep upstream OpenAI failures bounded. The defaults are a 45 second deadline and two retries for transient `429`/`5xx` responses or transport errors.

Blob disk reads and writes use `CASHFLUX_SERVER_BLOB_IO_TIMEOUT` so request cancellation and slow storage do not hang blob endpoints indefinitely. The default is 10 seconds.

Set `CASHFLUX_SERVER_OTLP_ENDPOINT` or standard `OTEL_EXPORTER_OTLP_ENDPOINT` to an OTLP/HTTP collector URL to
export OpenTelemetry traces from the server process. Leave it unset for the default no-op local/self-host setup.

Set `CASHFLUX_SERVER_STORAGE_WARN_BYTES` to emit `X-CashFlux-Storage-Warning` when a new distinct blob crosses
the per-user warning line. Set `CASHFLUX_SERVER_STORAGE_MAX_BYTES` to enforce a per-user fair-use cap for blob
storage. The default `0` is unlimited for self-hosting; when set, blob uploads that would exceed the user's
distinct linked blob bytes return HTTP 507 with `storage quota exceeded`.

Hosted billing deployments set `CASHFLUX_SERVER_BILLING=true`, `CASHFLUX_SERVER_STRIPE_SECRET_KEY`, the
annual/monthly Stripe price ids, Checkout return URLs, portal return URL, and
`CASHFLUX_SERVER_STRIPE_WEBHOOK_SECRET`. The server creates Checkout/customer-portal sessions and verifies
`/v1/billing/stripe/webhook` signatures before updating subscription state. Leave billing disabled for ordinary
self-hosting.

For status-page and incident handling, expose `/status` through the same TLS host and use `docs/INCIDENT_RESPONSE.md` for severity levels, update cadence, recovery, and postmortems.

## Data Retention

CashFlux backend data is stored in the configured data directory on the host that runs the server. No server data leaves that host unless you configure the off-box backup sync described above.

Retention defaults:

- Audit events: 365 days (`CASHFLUX_SERVER_AUDIT_RETENTION_DAYS`).
- Snapshot history: 180 days (`CASHFLUX_SERVER_SNAPSHOT_HISTORY_RETENTION_DAYS`).
- Local backup directories: 30 days (`CASHFLUX_SERVER_BACKUP_RETENTION_DAYS`).

Run retention manually:

```sh
docker compose -f docker-compose.selfhost.yml run --rm cashflux-server retention
```

The repository includes `deploy/cashflux-retention.example.service` and `deploy/cashflux-retention.example.timer` for weekly pruning. Keep off-box backup retention aligned with your legal/privacy requirements and test restore before deleting older backups.

Blob garbage collection removes unreferenced content-addressed blob files and metadata:

```sh
docker compose -f docker-compose.selfhost.yml run --rm cashflux-server gc-blobs
```

The repository includes `deploy/cashflux-blob-gc.example.service` and `deploy/cashflux-blob-gc.example.timer` for weekly blob GC. Monitor `cashflux_blob_gc_sweeps_total` and `cashflux_blob_gc_deleted_total`. Audit listing is capped at 500 rows per request and snapshot history is capped per write, so list/history growth stays bounded.

## Logging

Set `CASHFLUX_SERVER_LOG_FORMAT=json` for structured production logs, or `text` for local development. `CASHFLUX_SERVER_LOG_LEVEL` accepts `debug`, `info`, `warn`, and `error`. Sensitive attributes such as tokens, keys, secrets, cookies, passwords, and authorization values are redacted before logs are written.

Successful hot-path probes (`/livez`, `/healthz`, `/readyz`, and `/metrics`) are sampled with `CASHFLUX_SERVER_LOG_HOT_PATH_SAMPLE_RATE` to keep production logs useful under frequent health checks. Set it to `1` to log every probe.

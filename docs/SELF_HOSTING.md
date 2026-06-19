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

For a consistent backup:

1. Stop writes or put the service briefly in maintenance.
2. Run a WAL checkpoint:

   ```sh
   docker compose -f docker-compose.selfhost.yml exec cashflux-server sh -c 'sqlite3 /data/cashflux-server.db "PRAGMA wal_checkpoint(TRUNCATE);"'
   ```

3. Copy the full `cashflux-data` volume, including the SQLite database and blob directory.
4. Store the backup off-box and encrypted.

Restore by stopping the stack, replacing the `cashflux-data` volume contents, then starting the stack again. Verify `/readyz` before reconnecting clients.

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

If you add a volume or sidecar, keep writable paths explicit and prefer read-only mounts. Do not add broad capabilities or privileged mode for normal operation.

## Reliability Knobs

AI proxy calls use `CASHFLUX_SERVER_AI_UPSTREAM_TIMEOUT` and `CASHFLUX_SERVER_AI_UPSTREAM_RETRIES` to keep upstream OpenAI failures bounded. The defaults are a 45 second deadline and two retries for transient `429`/`5xx` responses or transport errors.

## Logging

Set `CASHFLUX_SERVER_LOG_FORMAT=json` for structured production logs, or `text` for local development. `CASHFLUX_SERVER_LOG_LEVEL` accepts `debug`, `info`, `warn`, and `error`. Sensitive attributes such as tokens, keys, secrets, cookies, passwords, and authorization values are redacted before logs are written.

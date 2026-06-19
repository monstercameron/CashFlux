# CashFlux Operations Runbook

This runbook covers recurring operator tasks for the optional CashFlux backend. Use it with `docs/SELF_HOSTING.md`, `docs/OBSERVABILITY.md`, and `docs/INCIDENT_RESPONSE.md`.

## Deploy

1. Confirm the target commit passed `go test ./...`, wasm build, server build, and `gwc verify`.
2. Run a fresh backup:

   ```sh
   docker compose -f docker-compose.selfhost.yml run --rm cashflux-server backup /data/backups
   ```

3. Pull and rebuild:

   ```sh
   git pull --ff-only
   docker compose -f docker-compose.selfhost.yml up -d --build
   ```

4. Verify:

   ```sh
   curl -fsS https://<domain>/status
   curl -fsS https://<domain>/readyz
   curl -fsS -H "Authorization: Bearer <token>" https://<domain>/metrics >/dev/null
   ```

5. Watch logs and metrics for at least 15 minutes: HTTP/gRPC errors, p99 latency, queue depth, database errors, and websocket upgrade failures.

## Rollback

Use rollback for stateless/code regressions. Do not downgrade a database after a forward migration unless a tested restore plan is approved.

1. Identify the last known good commit.
2. Stop new risky work if data correctness is uncertain.
3. Check out the prior commit and rebuild:

   ```sh
   git checkout <last-good-commit>
   docker compose -f docker-compose.selfhost.yml up -d --build
   ```

4. Verify `/status`, `/readyz`, sync, blob download, and AI proxy if enabled.
5. Open an incident issue with the bad commit, rollback commit, timeline, and follow-up fix.

## Restore

Use restore only when the live data directory is corrupt, accidentally deleted, or a migration/data operation must be undone.

1. Stop the stack.
2. Select the newest backup whose `manifest.json` is present and whose restore point matches the desired RPO.
3. Move the current data directory aside instead of deleting it.
4. Copy `cashflux-server.db` and `blobs/` from the backup into the empty `cashflux-data` volume.
5. Start the stack and verify `/readyz`.
6. Spot-check one synced workspace and one blob-backed artifact.
7. Record the restored backup path, manifest digest results, start/end time, and user impact.

## Rotate Access Token

Token mode uses a bearer token for self-host access.

1. Generate a new token:

   ```sh
   docker compose -f docker-compose.selfhost.yml run --rm cashflux-server rotate-token
   ```

2. Store the plaintext token in a password manager.
3. Put `CASHFLUX_SERVER_TOKEN_SHA256` into `deploy/cashflux-server.env`.
4. Restart the stack.
5. Update clients with the new token and remove the old token from any shared notes.

## Rotate Master Key

The master key encrypts stored AI keys. A full re-encryption command is not available yet, so rotation requires a maintenance window.

1. Ask users to re-enter BYO AI keys after the rotation.
2. Back up the server.
3. Stop the stack.
4. Update `CASHFLUX_SERVER_MASTER_KEY`.
5. Delete old `ai_keys` rows only after confirming users can re-save keys.
6. Start the stack and verify AI key save plus AI proxy chat through the gRPC tunnel.

## Revoke Sessions

- Token mode: rotate the access token.
- OAuth mode single user/session: use `/v1/auth/logout` from the affected browser when available.
- OAuth mode broad incident: rotate OAuth client secrets, restart the backend, and clear refresh-token cookies by forcing users through login again.

Record actor, reason, time, and affected user/session ids in the incident or support issue. Check `/v1/audit` for related login, refresh, logout, and key-write events.

## Past-Due Billing

Billing is disabled for self-host by default. For hosted deployments, use the billing provider as the source of truth.

1. Confirm account status in Stripe or the configured billing provider.
2. Do not delete user data for normal delinquency.
3. Disable paid-only server features by entitlement if required: sync writes, AI proxy, or large blob uploads.
4. Keep export, login, and account deletion available.
5. Communicate the billing state, grace period, and recovery path.

## Routine Checks

- Daily: `/status`, alert queue, failed backup timer, error-rate alerts.
- Weekly: retention timer success, backup restore sample, dependency/security alerts.
- Monthly: incident runbook review, token inventory, OAuth app secret age, status page subscriptions.

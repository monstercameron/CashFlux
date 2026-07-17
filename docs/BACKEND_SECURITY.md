# CashFlux Backend Security Notes

The backend follows a deny-by-default rule for data paths. Health, readiness, version discovery, root service discovery, OAuth redirects, and `/status` are public because they do not expose user financial data or secrets. Every route that returns or mutates backend data requires authentication.

Protected HTTP data routes:

- `/metrics`
- `/v1/audit`
- `/v1/admin/usage`
- `/v1/account/export`
- `/v1/account` with `DELETE`
- `/v1/auth/sessions`
- `/v1/auth/sessions/{family}` with `DELETE`
- `/v1/blobs/{hash}` with `GET`, `HEAD`, and `PUT`

Protected gRPC services:

- `cashflux.v1.SyncService`
- `cashflux.v1.AIService`

The gRPC bridge applies auth interceptors to unary and streaming calls. Token mode validates the configured bearer token or SHA-256 digest; OAuth mode validates signed short-lived access tokens. HTTP blob/admin handlers reject missing or invalid bearer tokens before reading or returning data. The cross-tenant operator surfaces — `/metrics` and the global `/v1/audit` log — additionally require *operator* authority (the static server token, whose holder is the operator in self-host token mode, or an `CASHFLUX_SERVER_ADMIN_USER_IDS` admin); a regular authenticated Cloud user is denied metrics and gets only their own actor-scoped audit events.

## Authentication & Token Model (backend ↔ frontend)

Two auth modes share one binary:

- **Token mode (`AuthMode=token`, self-host).** A single static bearer (`CASHFLUX_SERVER_TOKEN`, or its `CASHFLUX_SERVER_TOKEN_SHA256` digest) authenticates all requests as one synthetic principal `token:<sha256[:24]>`. Possessing this token is operator authority. Compared constant-time (`crypto/subtle`).
- **OAuth mode (`AuthMode=oauth`, multi-tenant Cloud).** Per-user sessions via Google/GitHub OAuth with PKCE + state/nonce. The browser receives:
  - a **short-lived access token** (HS256 JWT, 15 min) sent as `Authorization: Bearer` on API/gRPC calls;
  - a **rotating refresh token** in an `HttpOnly` + `Secure` + `SameSite` cookie (30 days), single-use — each refresh issues a new access+refresh pair and invalidates the old refresh (`refresh_tokens` table);
  - a **CSRF token** (double-submit cookie + `X-CashFlux-CSRF` header) required on every mutating auth route (refresh, logout, session revoke).

**Refresh-token reuse detection.** A replayed (already-consumed) refresh token revokes the entire session *family* and appends an `auth.token.reuse` audit event — a stolen refresh cookie can be used at most once before the whole family is killed.

**Key separation.** Session JWTs are signed with a dedicated `CASHFLUX_SERVER_SESSION_KEY` (HMAC-SHA256), distinct from the `CASHFLUX_SERVER_MASTER_KEY` used for AES-GCM at-rest encryption of AI keys — so an encryption-key rotation does not invalidate sessions, and a leak of one secret does not compromise the other. `CASHFLUX_SERVER_SESSION_KEY_PREVIOUS` is accepted on verify only, for zero-downtime key rotation. When no dedicated key is set the signer falls back to the master key/token (non-breaking) and the server logs a warning in oauth mode.

**Frontend surfaces.** The desktop/PWA app uses the bearer directly (self-host token, or a Cloud access token). The operator console (`/console/`) and the customer portal (`/portal/`) run in the browser; the console is token-gated for operators, the portal uses the OAuth session flow above. Cookies are `Secure` except on http loopback for local development (`requestIsSecure`).

**Transport.** CORS is deny-by-default: only the configured `AppOrigin` (an https origin, or an http loopback for dev) is allowed; credentials are permitted only for that origin. Security headers (HSTS, `nosniff`, COOP/COEP, `frame-ancestors 'none'`) are set on every response. The gRPC websocket bridge enforces the same origin check.

## Production Data Access Logging Policy

Production operators must not read customer sync snapshots, blob contents, or decrypted AI-key material during
routine support. Use scoped metadata first: `/status`, `/readyz`, `/metrics`, `/v1/admin/usage`, and `/v1/audit`.
When production data access is unavoidable, record the actor, reason, ticket or incident id, user/workspace target,
request id or trace id, start/end time, and fields accessed in the support or incident record before closing the
work item.

All privileged support actions should have a corresponding audit event or structured log entry with `request_id`,
`trace_id`, actor/user scope, route or RPC, status, and cause. Retain production access records for the same window
as operational audit logs, restrict them to operators with a support need, and review access monthly alongside the
SOC 2 readiness checklist.

## Security Coverage Map

The top-level backend security checklist is reconciled against the detailed hardening stories in `TODOS.md`
section 7.14:

- AI keys are encrypted at rest with AES-GCM, using a configured master key that must be 16, 24, or 32 bytes.
  Self-host docs require secret-manager or password-manager sourcing and document the current maintenance-window
  rotation path.
- Strict tenant isolation is enforced at the repository and service layers: workspace, blob, AI-key, usage, and
  audit queries are scoped by authenticated user id, with cross-user tests covering workspace and blob access.
  The `/v1/audit` stream serves the *global* log only to operators (static server token or an admin); regular
  Cloud users get an actor-scoped read (`ListAuditEventsForActor`). `/metrics` requires operator authority. Both
  are covered by isolation tests (global-vs-actor-scoped audit, tenant-403/admin-200 metrics).
- Session tokens are signed with a dedicated `CASHFLUX_SERVER_SESSION_KEY` separate from the AES master key, so
  encryption-key rotation never invalidates sessions and neither secret's leak compromises the other;
  `CASHFLUX_SERVER_SESSION_KEY_PREVIOUS` supports zero-downtime rotation. Tests cover key isolation, the rotation
  window, and the master-key fallback.
- The read-only `/v1/admin/usage` support view ignores caller-supplied user ids and returns only the
  authenticated user's daily request/token counters.
- Self-serve account export and delete-account routes are authenticated and scoped to the caller. Export omits
  decrypted AI secrets and blob bytes; deletion explicitly unlinks subscriptions, cascades relational rows, and
  sweeps unreferenced blobs.
- Session management routes are authenticated and scoped to the caller. Session-family revoke requires CSRF,
  hides other users' families as not found, and appends an audit event.
- OAuth profile handling rejects explicit unverified-email claims from supported providers before account upsert
  and session issuance.
- Repository SQL injection coverage includes a source guard that rejects dynamic SQL formatting/builders and pins
  parameterized user/workspace predicates.
- Request-size and abuse controls are enabled across the backend: dataset caps, blob size/storage caps, AI request
  caps, OAuth/session auth-route rate limits, HTTP in-flight/rate limits, per-user rate limits, and gRPC bridge
  connection/stream/upgrade caps.
- Blob fair-use controls include a soft per-user warning threshold and a hard per-user linked-byte cap for
  distinct uploaded blobs.
- AI proxy abuse controls include per-user daily request/token caps and an operator kill switch via
  `CASHFLUX_SERVER_AI_BLOCKED_USER_IDS`, which denies selected users before key load or upstream calls.
- Optional AI usage alert thresholds (`CASHFLUX_SERVER_AI_ALERT_REQUESTS_PER_DAY` and
  `CASHFLUX_SERVER_AI_ALERT_TOKENS_PER_DAY`) append audit events when a user crosses daily request/token
  warning lines, giving operators anomaly and cost-control signals before hard caps trip.
- Load/abuse tests cover oversized sync snapshots, oversized blobs, storage quota exhaustion, AI request-size
  rejection, per-user workspace stream caps, HTTP rate-limit configuration, and gRPC bridge connection limit
  configuration.
- Unit tests cover server storage, LWW sync decisions, AES-GCM AI-key encrypt/decrypt/rotation, usage
  counters/rate limits, content-addressed blob hashing, workspace blob links, and blob GC.
- Transport and process controls are covered by TLS/self-host Caddy docs, gRPC websocket origin checks,
  secret redaction in logs, Gitleaks, `govulncheck`, and high-severity `gosec` in CI.

When adding a new backend route, decide whether it is discovery/health or data-bearing. Data-bearing routes must reject unauthenticated requests in tests before implementation is considered complete.

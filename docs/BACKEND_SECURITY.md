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

The gRPC bridge applies auth interceptors to unary and streaming calls. Token mode validates the configured bearer token or SHA-256 digest; OAuth mode validates signed short-lived access tokens. HTTP blob/audit/admin/metrics handlers reject missing or invalid bearer tokens before reading or returning data.

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

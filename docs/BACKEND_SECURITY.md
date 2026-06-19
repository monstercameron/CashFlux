# CashFlux Backend Security Notes

The backend follows a deny-by-default rule for data paths. Health, readiness, version discovery, root service discovery, OAuth redirects, and `/status` are public because they do not expose user financial data or secrets. Every route that returns or mutates backend data requires authentication.

Protected HTTP data routes:

- `/metrics`
- `/v1/audit`
- `/v1/blobs/{hash}` with `GET`, `HEAD`, and `PUT`

Protected gRPC services:

- `cashflux.v1.SyncService`
- `cashflux.v1.AIService`

The gRPC bridge applies auth interceptors to unary and streaming calls. Token mode validates the configured bearer token or SHA-256 digest; OAuth mode validates signed short-lived access tokens. HTTP blob/audit/metrics handlers reject missing or invalid bearer tokens before reading or returning data.

## Security Coverage Map

The top-level backend security checklist is reconciled against the detailed hardening stories in `TODOS.md`
section 7.14:

- AI keys are encrypted at rest with AES-GCM, using a configured master key that must be 16, 24, or 32 bytes.
  Self-host docs require secret-manager or password-manager sourcing and document the current maintenance-window
  rotation path.
- Strict tenant isolation is enforced at the repository and service layers: workspace, blob, AI-key, usage, and
  audit queries are scoped by authenticated user id, with cross-user tests covering workspace and blob access.
- Request-size and abuse controls are enabled across the backend: dataset caps, blob size/storage caps, AI request
  caps, HTTP in-flight/rate limits, per-user rate limits, and gRPC bridge connection/stream/upgrade caps.
- Load/abuse tests cover oversized sync snapshots, oversized blobs, storage quota exhaustion, AI request-size
  rejection, per-user workspace stream caps, HTTP rate-limit configuration, and gRPC bridge connection limit
  configuration.
- Transport and process controls are covered by TLS/self-host Caddy docs, gRPC websocket origin checks,
  secret redaction in logs, Gitleaks, `govulncheck`, and high-severity `gosec` in CI.

When adding a new backend route, decide whether it is discovery/health or data-bearing. Data-bearing routes must reject unauthenticated requests in tests before implementation is considered complete.

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

When adding a new backend route, decide whether it is discovery/health or data-bearing. Data-bearing routes must reject unauthenticated requests in tests before implementation is considered complete.

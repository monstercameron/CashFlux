# Custom Sync transport boundary

TODOS.md C442. One page, kept short.

## The rule

The main app's backend client is **gRPC-only**, except for the browser-navigation
redirects it fundamentally cannot avoid:

- **OAuth login** (`/v1/auth/{provider}` popup + `postMessage` handoff) — kept
  per product decision; a gRPC call can't drive a third-party OAuth consent
  redirect.
- **Stripe/PayPal checkout & portal** — `BillingService.CreateCheckoutSession`
  (gRPC) *starts* the session, but the response is a URL the browser must
  navigate to (`window.location.assign`), and the provider redirects back via
  plain HTTP. See `CreateCheckoutSessionResponse.CheckoutURL`.

Everything else the app does against its own backend — sync, AI, auth
(register/login/refresh/logout/devices), account, billing session creation —
goes over the gRPC-over-WebSocket bridge (`internal/syncbridge`), not `fetch`.

## Known gaps as of this pass (TODOS.md C440)

Two REST call sites in `internal/app/backend.go` were evaluated for gRPC
conversion and deliberately left alone rather than force a broken swap:

- `testBackendConnection` (`GET /v1/version`) is the **pre-login** discovery
  probe — it has no token yet and exists to learn the auth mode/providers
  before the user can authenticate. `AccountService.GetEntitlement` requires
  an authenticated caller, so it cannot replace this call without breaking
  the "add a backend, then sign in" flow. A dedicated unauthenticated gRPC
  health/version method would be needed first.
- `signOutBackendOAuth` (`POST /v1/auth/logout`) relies on an **httpOnly**
  refresh-token cookie the browser attaches automatically; JS never sees that
  token. `AuthService.Logout` (gRPC) requires the refresh token in the request
  body, which the client cannot supply without breaking the httpOnly design.
  Converting this needs either a cookie-aware gRPC transport or a
  non-httpOnly logout credential — a bigger change than this pass.

`createBillingSession`'s REST call is also still in place; `BillingService.
CreateCheckoutSession` now exists server-side (see `internal/server/billingservice.go`)
but wiring the client call is left to whichever lane owns the billing UI, to
avoid two lanes editing the same lines of `backend.go` in the same pass.

## Out of scope: cashflux-portal

`cmd/cashflux-portal` is a deliberate, separate REST surface — an
account-management website (billing, session, export/delete), not the
sync-critical path the main app depends on every load. It is not part of
this gRPC migration.

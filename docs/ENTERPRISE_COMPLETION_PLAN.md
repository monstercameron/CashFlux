# Enterprise Completion Plan — "Complete the Picture"

> Goal (2026-06-24): bring CashFlux to enterprise-complete on the commercial axis —
> user **and** admin interfaces, login/auth, the missing commercial features, a strong
> homescreen, and end-to-end encrypted artifact + client-dataset storage. Built bottom-up
> per `CLAUDE.md` (data model → tested logic → persistence → state → UI), one feature per
> commit, CHANGELOG + DEVLOG every commit, push after each.

## Gap analysis (what already exists — do NOT rebuild)

- **Client dataset encryption at rest** — `internal/cryptobox` (envelope: PBKDF2-SHA-256
  600k → AES-GCM-256, key never stored) + `internal/app/datasetcrypto.go` (crypto.subtle
  wiring) + `datasetcryptowire.go` + `internal/app/applock*` (passcode gate). The exported
  SQLite snapshot persisted to local storage is already encrypted when a passcode is set.
  **Status: essentially complete.** Remaining: make the encryption state visible/auditable
  and confirm artifacts are covered.
- **Login/auth (client)** — `internal/app/backend.go`: OAuth popup login (`startOAuthLogin`),
  token-mode auth, sign-out (`signOutBackendOAuth`), CSRF header plumbing, key upload.
  Server: full OAuth (PKCE/state/nonce) + token mode + refresh-token rotation + session
  revocation. **Status: functionally complete.** Remaining: client-side silent refresh.
- **User interface (Cloud)** — `internal/app/settings.go`, `settings_section.go`,
  `syncchip.go`, `subscriptionbanner.go`, `upgradesheet.go`, `deviceslist.go`,
  `cloudmention.go`. **Status: complete.**
- **Billing** — Stripe Checkout + portal + webhooks + `subscriptions` table + entitlement
  gate. **Status: complete** (open: monetization launch wiring + analytics surface).
- **Artifact blobs (sync)** — `internal/app/backend.go` uploads/downloads content-addressed
  blobs, extracts `Artifact.Bytes` → `BlobRef`. **Status: works, but plaintext on server.**

## What's missing (this plan)

### F1 — Server: admin role + tenant-safe admin API  *(native Go, table-tested)*
- `Config.AdminUserIDs` (env `CASHFLUX_SERVER_ADMIN_USER_IDS`, comma-separated); `IsAdmin(userID)`.
- Admin-gated endpoints (deny-by-default, audited): 
  - `GET /v1/admin/overview` — aggregate ops metrics: total users, active/trialing/past-due
    subscription counts, estimated MRR cents, total blob bytes, daily request/token totals.
  - `GET /v1/admin/users` — paginated user list with subscription status (no secrets, no
    decrypted keys, no blob bytes).
- Strict authz: non-admin bearer → `PermissionDenied`; every query stays parameterized.
- Tests: admin vs non-admin access, aggregate correctness, isolation (no cross-tenant leak
  of secrets/bytes).

### F2 — Client: zero-knowledge encrypted artifact blobs  *(crypto reuse)*
- Before `uploadBackendArtifactBlob`, envelope-encrypt `Artifact.Bytes` with the active
  passcode-derived key (reusing `cryptobox`/`datasetcrypto`); the blob stored server-side is
  ciphertext; content address = sha256(ciphertext) so per-user dedup still holds.
- `downloadBackendArtifactBlob` decrypts after fetch; legacy plaintext blobs still readable
  (envelope sniff via `cryptobox.IsEnvelope`).
- No-passcode ⟹ current plaintext behavior (unchanged), so the feature is additive.
- Tests: pure round-trip helper (encrypt→hash→decrypt) where it can run without JS.

### F3 — Client: Admin console screen (GWC)  *(UI last)*
- New `/admin` screen (`internal/screens/admin.go`): operator overview cards (users, MRR,
  subscriptions, storage, AI usage) + a users table, fed by F1 endpoints.
- Registry-driven route; visible only when the signed-in user is an admin (gated in the shell
  / via a backend probe). a11y + i18n (`uistate.T`) + keyboard from day one.

### F4 — Strong homescreen  *(UI)*
- A real **Home** surface: signed-out → product value + sign-in/get-started CTA; signed-in →
  a glanceable "good morning" summary (net worth, this-month cash flow, top nudges, quick
  actions) above the bento. Calm, typographic, enterprise-grade.

### F5 — Verification + docs
- End-to-end: encrypted dataset + encrypted artifacts confirmed; admin gating confirmed.
- Update README "What it does" + this plan's checkboxes; SPEC note.

## i18n strategy (avoid the in-flight `en.go` WIP)
New keys are registered from a dedicated file `internal/i18n/en_enterprise.go` whose `init()`
merges into the `english` catalog — so we never edit the concurrently-modified `en.go`.

## Execution rules
- Each feature = one Sonnet subagent, sequential (never parallel), `model=sonnet`.
- Subagents commit **only their own files** by explicit path; never `git add -A`; verify with
  `git status` that the user's WIP (`accounts.go`, `transactions.go`, `quickadd.go`,
  `flippanel.go`, `en.go`, `auditview*`, `TODOS.md`, `e2e/_*`) is never staged.
- `gofmt` + `GOOS=js GOARCH=wasm go build` (UI) and `go test ./...` (native) must pass.
- CHANGELOG (Unreleased) + DEVLOG dated entry per feature; push in background after commit.

## Status
- [ ] F1 server admin role + API
- [ ] F2 encrypted artifact blobs
- [ ] F3 admin console screen
- [ ] F4 strong homescreen
- [ ] F5 verification + docs

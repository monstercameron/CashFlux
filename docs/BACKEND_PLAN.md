# CashFlux Backend — Design Plan

> Status: backend foundation in progress. A thin server for two jobs: **dataset sync** and an
> **AI API proxy**. All business logic stays in the wasm client; the app remains
> local-first and fully usable offline — the backend is an optional sync/proxy tier.

## Decisions (locked)
- **Sync conflicts:** last-write-wins (newest snapshot wins).
- **AI key:** per-user, bring-your-own, stored **encrypted at rest** server-side.
- **Auth:** OAuth (Google / GitHub).
- **Artifacts:** separate **content-addressed blob store**; the synced dataset holds
  only references (hash + metadata), never image/dataset bytes.

## Scope
**In:** account auth, per-user/per-workspace dataset sync, blob upload/download, AI
proxy with per-user keys + metering.
**Out (for now):** real-time/collaborative editing, multi-account sharing of one
workspace, server-side business logic, server-side reporting. The server never
interprets the dataset — it stores and forwards.

## Architecture
- **Single Go binary** (matches the stack), `net/http` + a light router (chi), behind
  TLS (Caddy / managed platform). Stateless app tier; state lives in the DB + blob dir.
- **Server store:** SQLite via **`ncruces/go-sqlite3`** (same pure-Go driver the client
  uses — consistent, no cgo), WAL mode. One DB file.
- **Blob store:** content-addressed files on disk (`blobs/<sha256>`), pluggable to an
  S3-compatible bucket later. Dedup + integrity for free (hash = name).
- **Streaming:** gRPC server streams over the GoGRPCBridge tunnel for AI chunk responses.

## Data model (server SQLite)
- `users(id, provider, subject, email, created_at)` — one row per OAuth identity.
- `workspaces(id, user_id, name, color, sort, deleted, version, updated_at, device_id)`
  — mirrors the client's workspace registry; `deleted` is a tombstone so other devices
  learn of removals; `version` is a monotonic counter; `updated_at` drives LWW.
- `snapshots(workspace_id, dataset_json, version, updated_at)` — the current dataset
  blob per workspace (gzipped). Keep the **last N** prior snapshots per workspace for
  recovery (cheap insurance against LWW clobber).
- `blobs(hash, size, mime, created_at)` + `workspace_blobs(workspace_id, hash)` for
  refcount/GC. Bytes live on disk by hash.
- `ai_keys(user_id, provider, ciphertext, nonce)` — encrypted BYO key (AES-GCM).
- `usage(user_id, day, requests, tokens)` — per-user metering for rate limits.
- `subscriptions(user_id, stripe_customer, stripe_sub, status, plan, current_period_end, trial_end)` — current
  Stripe-backed Cloud entitlement state; webhooks update it and entitlement checks read it.

## Sync (last-write-wins, snapshot-based)
The dataset is already a single JSON snapshot (`store.ExportJSON`), so sync ships that
blob — no per-entity protocol needed. All endpoints are authenticated and scoped to the
caller's `user_id`.

- `GET  /v1/workspaces` → list `{id, name, color, sort, version, updatedAt, deleted}`.
- `GET  /v1/workspaces/:id` → `{version, updatedAt, dataset}` (gzipped).
- `PUT  /v1/workspaces/:id` `{dataset, clientUpdatedAt, deviceId}` → **LWW**: accept and
  bump `version` when `clientUpdatedAt >= stored.updatedAt` (newest-wins, so a stale
  device can't clobber newer data); otherwise return the current `{version, updatedAt}`
  and let the client re-pull. An explicit `?force=1` overrides (manual "use mine").
- `DELETE /v1/workspaces/:id` → soft-delete (tombstone), so other devices remove it.

**Client behavior (offline-first):** push the active workspace's dataset on the existing
debounced autosave and on reconnect; pull on load and on tab focus; apply the newest by
`updatedAt`. Works fully offline; syncs opportunistically when signed in.

> LWW caveat (accepted): concurrent edits on two devices lose one side. Mitigations:
> newest-wins-by-timestamp (not raw last-to-arrive) + keep last-N server snapshots for
> recovery. A later, cheap upgrade is optimistic concurrency (client sends its base
> `version`; server 409s on mismatch) without changing the wire format.

## Transport and auth handshake
CashFlux uses one user-facing backend base URL in Settings. The client derives two transports from it:

- **HTTP JSON/blob routes:** use the configured base URL directly, e.g. `http://127.0.0.1:8081/v1/version`
- **Billing routes:** authenticated `POST /v1/billing/checkout` and `POST /v1/billing/portal` create Stripe
  Checkout and customer-portal sessions using configured price ids and return URLs.
  and `/v1/blobs/{hash}`. OAuth and blob bytes stay on HTTP; AI and sync do not.
- **Stripe webhooks:** `POST /v1/billing/stripe/webhook` verifies Stripe signatures with
  `CASHFLUX_SERVER_STRIPE_WEBHOOK_SECRET` before updating subscription state.
- **gRPC bridge routes:** convert the same base URL to a websocket `/grpc` target (`http` → `ws`, `https` →
  `wss`) and dial it with GoGRPCBridge `BuildTunnelConn`.

The same access token is carried on both surfaces:

- HTTP routes send `Authorization: Bearer <token>`.
- gRPC calls send outgoing metadata `authorization: Bearer <token>` on every unary and streaming RPC.
- The server hashes the bearer token into a stable token-mode user id, validates it in constant time against
  `CASHFLUX_SERVER_TOKEN`, and attaches the authenticated user to the request/RPC context.

Self-hosted token mode uses this flow directly. OAuth mode will issue short-lived access tokens through the
HTTP login/refresh endpoints, then the client will carry those tokens through the same HTTP header and gRPC
metadata slots.

## Artifacts (content-addressed blobs)
Keeps the synced snapshot small even with images/datasets.
- Client computes `sha256` of artifact bytes; the dataset stores
  `{id, hash, mime, size, name}` only (no `Bytes`).
- `PUT  /v1/blobs/:hash?workspaceId=:id` (idempotent; skips if hash exists) → verifies the bytes hash
  to `:hash`, enforces `CASHFLUX_SERVER_BLOB_MAX_BYTES`, stores by hash, and links it to an owned workspace.
- `GET  /v1/blobs/:hash?workspaceId=:id` → bytes (cacheable, immutable) only when that workspace links the blob.
- `HEAD /v1/blobs/:hash?workspaceId=:id` → existence and metadata only when that workspace links the blob.
- Sync transfers only the small dataset; blobs upload on save and download on demand,
  deduped by hash across workspaces. Refcount via `workspace_blobs`; GC unreferenced.

## AI proxy (per-user encrypted BYO key)
- `AIService.SetKey {provider, key}` over `/grpc` encrypts the BYO key with AES-GCM (master key from env/secret
  manager) and stores only ciphertext. Entered once over TLS; never returned to the client.
- `AIService.Chat`, `AIService.Vision`, and `AIService.ListModels` run over the GoGRPCBridge `/grpc` tunnel.
  The server loads + decrypts the user's key, calls OpenAI, enforces the model allow-list, applies per-user
  rate limits/usage metering/request-size caps, and maps upstream failures to gRPC status codes.
- The legacy HTTP AI routes are retired: `/v1/ai/key`, `/v1/ai/chat`, and `/v1/ai/vision` are not mounted.
  The client keeps direct OpenAI as the local-only fallback, but backend proxy traffic uses authenticated gRPC.

## Auth (OAuth)
- `GET /v1/auth/:provider` (google|github) → redirect with PKCE + `state`.
- `GET /v1/auth/:provider/callback` → upsert `users` row → issue a session (short-lived
  JWT access token + httpOnly refresh cookie).
- Auth middleware validates the token and scopes every query by `user_id`.
- SPA flow: redirect-based login; CORS locked to the app origin. (The `<base href>` fix
  already in place keeps deep-link routes working through the redirect round-trip.)

## Security
TLS only; OAuth PKCE + `state`; AI keys encrypted at rest; strict per-user data isolation
in every query; request-size limits (dataset + blob caps); per-user rate limiting; CORS
restricted to the app origin; no secrets in logs; usage/audit logging.

## Tech stack
Go · `net/http` + chi · GoGRPCBridge · `google.golang.org/grpc` · `ncruces/go-sqlite3` (WAL) ·
AES-GCM · blobs on disk (S3 adapter later). Ships as one binary + a data dir.

Dependency note: OAuth uses explicit standard-library HTTP handlers for auth-code exchange, PKCE/state,
userinfo, and ID-token claim validation without carrying an unused `golang.org/x/oauth2` dependency. Keep the
dependency set small unless a provider flow needs the library.

## Deployment & ops
- Single binary + SQLite file + `blobs/` dir behind TLS (Caddy) or a managed host
  (fly.io). Backups = WAL-checkpoint the SQLite file + copy `blobs/`.
- Schema migrations versioned like the client's `store.SchemaVersion` (stepwise, reject
  newer-than-supported).
- Health/readiness endpoints; structured logs; basic per-user usage metrics.

## Required client adaptations (so the plan is honest about cost)
1. **Sync client** layered over the existing autosave: push/pull, debounce, offline queue,
   newest-wins apply. Maps the existing `internal/app/workspace.go` registry to server
   workspace ids.
2. **Artifact extraction:** move `domain.Artifact.Bytes` out of the synced snapshot —
   dataset carries the hash/metadata; bytes go to the blob endpoints (kept locally as a
   cache). This is the one schema-shaped change to the client.
3. **AI calls via proxy:** prefer `AIService` over the `/grpc` GoGRPCBridge tunnel when backend URL/token prefs
   are configured; direct OpenAI remains a local-only fallback.
4. **OAuth login + token handling**, while preserving offline-first (no login required to
   use the app locally; sync/AI activate when signed in).

## Phasing (each independently shippable)
1. **Auth + snapshot sync (LWW)** — datasets sync; artifacts still inline. Smallest useful.
2. **Blob store + client artifact extraction** — keeps sync small as artifacts grow.
3. **AI proxy + encrypted keys + metering** — removes the key from the browser.

Rollout rule: each phase must be independently shippable and reversible. The local-first app keeps working at
every phase; the backend only adds sync/proxy. If a phase has to be disabled, clients keep local data and fall
back to the prior phase instead of blocking local budgeting.

## Risks / open items
- **LWW data loss** across simultaneous-device edits (accepted) — mitigated by
  newest-wins + last-N server snapshots; optimistic-version check is an easy later upgrade.
- **Clock skew** affects timestamp LWW — consider server-stamped `updated_at` on accept
  and treating client time as advisory.
- **Blob GC / refcounting** correctness as workspaces are deleted/duplicated.
- **OAuth provider config** (client IDs/secrets, redirect URIs per environment).
- **Multi-device AI metering** fairness and abuse limits on the proxy.

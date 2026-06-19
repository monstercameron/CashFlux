# Backend Error Model

CashFlux backend RPCs use gRPC status codes for transport, auth, validation, quota, and upstream failures.
Responses that need to carry recovery data may use a successful RPC with an explicit result flag.

## gRPC Codes

- `Unauthenticated`: missing, invalid, or expired bearer token/session metadata.
- `InvalidArgument`: malformed request shape, invalid timestamps, unsupported model, bad ids, or invalid key
  format.
- `FailedPrecondition`: server-side prerequisite is missing, such as disabled AI proxy, missing store, missing
  master key, or no stored OpenAI key.
- `ResourceExhausted`: request-size caps, AI daily request/token limits, storage quota, dataset size cap, or
  per-user stream caps are exceeded.
- `Unavailable`: transient upstream OpenAI/network failure.
- `DeadlineExceeded` / `Canceled`: upstream deadline or client cancellation.

## LWW Sync Rejection

`SyncService.PutWorkspace` treats stale non-forced writes as an application-level last-write-wins rejection,
not a gRPC failure. The RPC returns OK with `accepted=false`, the current workspace metadata, and the current
dataset when available. This current dataset lets the browser immediately re-pull/apply the newer server snapshot without a
second error-recovery call.

Forced writes that target another user's workspace still fail with `NotFound`; malformed workspace data still
fails with `InvalidArgument`.

## HTTP Statuses

HTTP blob, audit, metrics, OAuth, and status endpoints use matching HTTP status families: `401` for auth,
`400`/`415` for malformed or unsupported input, `413` for blob size caps, `429` for rate limits, `507` for
storage quota, and `503` for unavailable dependencies/readiness.

# Backend Error Model

CashFlux backend RPCs use gRPC status codes for transport, auth, validation, quota, and upstream failures.
Responses that need to carry recovery data may use a successful RPC with an explicit result flag.

## Stable Reasons

New backend APIs should expose one of these stable machine-readable reasons in addition to the transport status.
The Go source of truth is `internal/server.BackendErrorTaxonomy`.

| Reason | gRPC code | HTTP status | Use |
| --- | --- | --- | --- |
| `AUTH_UNAUTHENTICATED` | `Unauthenticated` | `401` | Missing, invalid, or expired auth metadata. |
| `AUTH_PERMISSION_DENIED` | `PermissionDenied` | `403` | Authenticated caller is not allowed to perform the action. |
| `REQUEST_INVALID` | `InvalidArgument` | `400` | Malformed JSON/RPC shape, bad ids, invalid timestamp, unsupported model/key format. |
| `REQUEST_TOO_LARGE` | `ResourceExhausted` | `413` | HTTP body exceeds a route-specific size cap. |
| `REQUEST_UNSUPPORTED_MEDIA` | `InvalidArgument` | `415` | Blob/content upload type is blocked. |
| `RESOURCE_NOT_FOUND` | `NotFound` | `404` | Resource does not exist or is intentionally hidden by tenant isolation. |
| `FAILED_PRECONDITION` | `FailedPrecondition` | `412` | Server prerequisite is missing, such as disabled AI proxy or missing master key. |
| `RESOURCE_EXHAUSTED` | `ResourceExhausted` | `507` | Storage, dataset, request-size, or stream quota is exceeded. |
| `RATE_LIMITED` | `ResourceExhausted` | `429` | Per-user or per-IP request rate limit is exceeded. |
| `UPSTREAM_UNAVAILABLE` | `Unavailable` | `502` | OpenAI or another upstream dependency failed transiently. |
| `DEADLINE_EXCEEDED` | `DeadlineExceeded` | `504` | Upstream or server deadline expired. |
| `CANCELED` | `Canceled` | `499` | Client canceled the request. |
| `INTERNAL` | `Internal` | `500` | Unexpected server failure; response must not leak internals. |

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

## Migration Note

HTTP account, admin support, blob, audit, metrics, and CORS preflight errors now return JSON bodies shaped like:

```json
{"error":{"reason":"REQUEST_INVALID","message":"invalid day"}}
```

Remaining plain-text HTTP error bodies are being migrated to JSON details using the stable reasons above. Until
that migration is complete, handlers must still choose the mapped HTTP status and avoid leaking secrets, tokens,
datasets, blob bytes, or internal stack details.

# GWC 5 RPC worker migration

Status: adversarial review complete; refined implementation checklist approved.

## Problem statement

CashFlux currently compiles its UI and every browser-side gRPC client into one
`main.wasm`. The calls generally run from Go goroutines, but Go goroutines in a
`js/wasm` binary are cooperatively scheduled on the browser's one JavaScript
event loop. They are not browser threads.

The network wait itself is asynchronous. The work around that wait is not:

- GoGRPCBridge/WebSocket callbacks resume the Go scheduler on the render thread.
- gRPC stream receive loops, reconnect/backoff, JSON codec work, and base64
  conversion run in `main.wasm`.
- A resumed scheduler runs every runnable goroutine until each blocks or yields.
  Browser developer tools then attribute the aggregate hold to whichever event
  happened to resume Go, which is why sync work appears as a multi-second click,
  scroll, message, or idle callback.

CashFlux's `internal/app/phasetimer.go` and recent measurements already establish
this scheduling behavior. GWC 5 supplies the missing architectural primitive:
compile a second Go WASM binary and run it in a Dedicated Web Worker.

The requested outcome is narrower than moving the whole CashFlux domain:

> Every browser-side gRPC connection and call must live in `services.wasm`.
> `main.wasm` remains the render/UI/local-store artifact.

The existing `web/sw.js` is a browser Service Worker for offline caching. It is
not the new execution worker and must remain independent.

## Current RPC inventory

The browser currently owns these active gRPC paths:

| Area | RPC shape | Methods |
|---|---|---|
| AI | server stream | `ChatStream`, `VisionStream` |
| AI settings | unary | `SetKey` |
| Authentication | unary | `Register`, `Login`, `RedeemPairingCode`, `CancelDevicePairing`, `SetPassword`, `RefreshToken` |
| Authentication | server stream | `WatchPairingStatus` |
| Authentication | unary bootstrap | `RequestDevicePairing` |
| Account | unary | `GetEntitlement` |
| Sync | unary | `PutWorkspace`, `GetWorkspace`, `ListWorkspaces` |
| Sync | server stream | `WatchWorkspaces` |
| Blob | client stream | `UploadBlob` |
| Blob | server stream | `DownloadBlob` |

The calls are spread through `internal/app` and `internal/ai`. Some create an
ad-hoc tunnel per operation; `internal/app/sync_client.go` separately maintains a
shared connection and token-refresh path.

`internal/backendrpc` is not currently a pure contract package. `ai.go` also
defines and registers the gRPC JSON codec. Consequently, removing direct calls
from the app is insufficient: importing the request/response structs would
still link `google.golang.org/grpc` into `main.wasm`.

## Target ownership boundary

### `main.wasm`

- Owns DOM, router, GWC state, UI callbacks, local preferences, IndexedDB-backed
  local application data, privacy-lock state, and final dataset import.
- Starts one Dedicated Web Worker and communicates only through typed worker
  commands/events.
- Persists access/refresh tokens, then sends the current endpoint/session
  configuration to the worker. Dedicated workers do not have `localStorage`.
- Never imports `google.golang.org/grpc`, GoGRPCBridge, or `internal/syncbridge`.
- Never dials, invokes, opens a gRPC stream, receives a gRPC message, or executes
  gRPC codec/transport logic.
- Retains cross-tab token-rotation coordination through Web Locks and persisted
  browser state; only the RefreshToken gRPC call itself runs in the worker.
- Retains the small app-level reconnect policy that decides when to request a
  replacement worker stream. The connection, stream, receive loop, and status
  decoding remain worker-owned.

### `services.wasm`

- Owns the `syncbridge` tunnel connection pool, keyed by normalized endpoint and
  effective bearer token.
- Owns unary invoke, server-stream receive, client-stream send, cancellation,
  keepalive, gRPC stream lifetimes, and blob chunking/assembly.
- Owns gRPC JSON codec encoding/decoding and status extraction.
- Converts gRPC failures to a transport-neutral error envelope containing a
  stable code and message.
- Answers every command, including unknown names and panicking handlers.
- Announces readiness only after its message handler exists.

### Shared, native-testable packages

- Pure worker protocol envelopes and error taxonomy.
- Pure request registry/validation.
- Pure connection-key and retry decisions where practical.
- `internal/backendrpc` request/response contracts after its gRPC codec is split
  into a worker/server-only subpackage.

## Worker protocol

One protocol must support all three gRPC shapes and worker lifecycle:

```text
app -> worker
  {kind:"unary", id, callID, endpoint, token, method, payload}
  {kind:"stream-open", id, endpoint, token, method, payload}
  {kind:"stream-cancel", id}
  {kind:"blob-upload", id, endpoint, token, header, bytes:ArrayBuffer}
  {kind:"blob-download", id, endpoint, token, request}

worker -> app
  {kind:"ready"}
  {kind:"reply", id, payload}
  {kind:"stream-event", id, seq, payload}
  {kind:"stream-end", id}
  {kind:"failure", id, code, message, mayHaveApplied}
  {kind:"blob-reply", id, metadata, bytes:ArrayBuffer}
  {kind:"fatal", message}
```

Request IDs correlate concurrent operations. Stream sequence numbers make
duplicate, missing, and out-of-order events diagnosable. Every terminal path
removes its correlation slot. A worker `error`/`messageerror`/instantiate failure
fails every in-flight operation instead of leaving callers blocked.

Context cancellation must cross the boundary. Releasing only the app-side
waiter is not sufficient: the worker must cancel the context passed to
`ClientConn.Invoke`, `NewStream`, and stream receive/send loops.

GWC's `projection.WorkerClient` is suitable for unary reply correlation and its
failure model, but it does not expose streaming or cancellation messages.
CashFlux therefore needs a small RPC-specific layer around/alongside it rather
than pretending every stream is a unary command.

## Payload strategy

Small request and response bodies may use JSON strings. Large byte-bearing
operations must not add another base64 round trip at the worker boundary:

- Workspace datasets and blob bodies cross as transferable `ArrayBuffer`s.
- Blob chunking occurs inside `services.wasm`.
- A transferred buffer is treated as moved, not retained and reused by the
  sender.
- AI chunks are coalesced in the worker before crossing to the app so token-sized
  events cannot cause a render storm.

There is an unavoidable WASM-memory-to-ArrayBuffer copy on each side without
shared WASM memory. That is still materially cheaper than base64 expansion,
string allocation, and JSON parsing on the render thread.

## Error and retry semantics

The app currently relies on gRPC status codes for authentication and
user-facing error messages. The worker boundary must preserve:

- stable gRPC code name/number;
- server message;
- whether the request definitely did not apply or may have applied;
- cancellation versus deadline versus worker death.

`PutWorkspace` already carries LWW/idempotent recovery semantics. A worker death
after the server applied a write must not cause the app to roll back or blindly
duplicate a non-idempotent enrollment request. Existing enrollment idempotency
keys remain stable across attempts.

Automatic retry is allowed only when:

- delivery definitely failed before application; or
- the operation is known idempotent and retains the same idempotency key.

## Startup and lifecycle

1. `main.wasm` constructs the worker client before background sync starts.
2. The document exposes `data-services-worker=starting|ready|error` as a
   production diagnostic and browser-test readiness signal.
3. `services-worker.js` imports `wasm_exec.js` and instantiates
   `bin/services.wasm` with gzip-to-raw fallback behavior matching the app.
4. `services.wasm` installs `onmessage`, then posts `ready`.
5. Calls wait for readiness with a bounded startup timeout.
6. Preference/session changes invalidate mismatched pooled connections.
7. Sign-out cancels related streams and closes session connections.
8. Page shutdown terminates the worker; a worker fatal error is surfaced and a
   controlled restart may be attempted once no ambiguous write is retried.

## What this migration does not move

The following phases still execute in `main.wasm`:

- local `store.Import`/`store.Export`;
- artifact enumeration against the local dataset;
- privacy encryption/decryption;
- final `app.ImportJSON`;
- DOM reconciliation after a remote dataset is applied.

Moving gRPC is expected to remove transport, streaming, and network-callback
pressure. It cannot honestly claim to remove every sync hitch. Phase timers and
the browser long-task probe remain in place; if import/export or crypto still
exceeds the budget, that is a separate worker-domain migration rather than scope
creep hidden inside this one.

## Build, deployment, and offline impact

Every path that builds or packages the SPA must produce and serve:

- `web/bin/main.wasm`;
- `web/bin/services.wasm`;
- `web/services-worker.js`;
- one compatible `web/wasm_exec.js`.

This includes Playwright global setup, GitHub Pages, release compression, local
development, service-worker precache, and stale-compressed-artifact cleanup.
GWC's current CLI accepts one app target per build/dev command, so CashFlux needs
a wrapper or explicit second `go build` for the worker artifact.

## Initial implementation checklist

This is the pre-review checklist. It is deliberately preserved so adversarial
review changes are auditable.

1. Split the gRPC JSON codec out of `internal/backendrpc`.
2. Add pure worker protocol/error types and native tests.
3. Add a browser worker client with readiness, correlation, cancellation, and
   worker-death handling.
4. Add `cmd/cashflux-services` with unary and stream dispatch.
5. Move shared connection/token-refresh ownership into the worker.
6. Replace all unary browser RPC calls.
7. Replace AI and pairing server streams.
8. Replace sync watch and its reconnect/backoff loop.
9. Replace blob upload/download with transferable binary operations.
10. Add dependency-graph tests proving the artifact boundary.
11. Build/package/cache both artifacts everywhere.
12. Add successful worker RPC E2E coverage and a render-thread performance
    sanity test.
13. Run native, vet, WASM, release-build, route smoke, sync/auth, cancellation,
    and long-task gates.

## Adversarial review of the initial checklist

The first checklist was challenged against cancellation, authentication, cache
skew, streaming pressure, binary copying, and the actual CashFlux package graph.
The following gaps would have produced an implementation that compiled but did
not satisfy the request.

### Finding 1: request correlation is not cancellation

`projection.WorkerClient` removes an app-side waiter when its context ends. It
cannot, through its current `Poster` interface, cancel the context executing in
the worker. A timed-out upload, AI stream, or workspace watch would therefore
continue consuming a connection and posting replies nobody accepts.

**Refinement:** every operation carries an app-generated operation ID. Context
completion posts an explicit cancel message. The worker registers a
`context.CancelFunc` before dialing/opening the stream and removes it on every
terminal path. Tests assert both correlation-table and worker operation-table
counts return to zero.

### Finding 2: moving calls does not remove the main artifact's gRPC dependency

`internal/app/authhelpers.go`, `entitlement.go`, `backend.go`, and
`sync_client.go` import `grpc/status` or `grpc/codes` for error interpretation.
`internal/backendrpc/ai.go` imports gRPC to register the JSON codec. Replacing
`Dial`/`Invoke` alone would leave gRPC linked into `main.wasm`.

**Refinement:** retain the codec's pure JSON implementation in
`internal/backendrpc`, move registration/call options to
`internal/backendrpc/grpccodec`, and introduce a pure wire error with stable
code/message fields. All browser error handling consumes that type.

### Finding 3: token refresh has persistence and stampede problems

The initial plan said the worker owns refresh but did not explain how rotated
tokens reach CashFlux's persistent preferences. Dedicated workers cannot use
`localStorage`. Concurrent unauthenticated calls could also each refresh,
rotating the same refresh-token family several times and invalidating siblings.

**Refinement after implementation review:** `main.wasm` keeps the existing Web
Locks refresh coordinator and persistence path because the lock is deliberately
cross-tab, while each page owns a Dedicated Worker. Moving singleflight into one
page's worker would not prevent a sibling tab from replaying the same rotating
refresh token. The actual RefreshToken RPC still executes in `services.wasm`.
After rotation the app sends `session-reset`, which cancels worker operations,
closes old-token pooled connections, and restarts the watch under the new token.

### Finding 4: app/worker cache skew can silently call the wrong protocol

CashFlux aggressively caches WASM and supports sub-path hosting. A new
`main.wasm` can coexist temporarily with a stale `services-worker.js` or
`services.wasm`, especially across a service-worker activation.

**Refinement:** readiness includes an explicit protocol version. The app refuses
to send calls to a mismatched worker and surfaces a reload-required failure.
Worker URLs are based on the document's resolved base URL; worker-local assets
are resolved relative to the worker script. Packaging tests cover root and
sub-path URLs, and the offline cache version is bumped atomically.

### Finding 5: “long request” is not a sufficient performance test

Network latency by itself is asynchronous and may produce no main-thread work.
A test that only delays a tiny unary response could pass before and after the
migration without proving anything.

**Refinement:** performance coverage has three controls: delayed small unary
(must remain responsive), high-frequency server stream (tests callback/render
pressure), and multi-megabyte workspace/blob transfer (tests codec/copy
pressure). The probe records main-window Long Tasks and input-event latency,
with an on-render-thread control proving the detector can fail.

### Finding 6: string JSON across the worker boundary recreates byte-heavy work

Workspace datasets and blobs are `[]byte`. JSON turns those into base64, then
the app and worker allocate/parse strings around the copy. Moving only the socket
would leave the dominant byte conversion on the render thread.

**Refinement:** use transferable `ArrayBuffer` side channels for
`PutWorkspace`, `GetWorkspace`, upload, and download. Small metadata stays JSON.
The app pays one bounded WASM-to-JS copy; gRPC chunking, base64 codec work, and
response assembly happen in the worker.

### Finding 7: worker event delivery can become its own denial of rendering

AI token streams can produce hundreds of messages. `WatchWorkspaces` reconnects
can duplicate subscriptions if the old operation is not fully cancelled.
Unbounded `postMessage` traffic can make an off-thread transport stall the main
event loop through event volume.

**Refinement:** AI chunks are coalesced by a short time/size budget; each stream
has one terminal event; workspace watch ownership is keyed so replacement
cancels the prior watch; sequence numbers detect duplicate/out-of-order events;
and tests cap outstanding event buffers.

### Finding 8: worker Go handlers must return to their own event loop

Starting gRPC directly in the worker's `onmessage` callback can deadlock the same
way a synchronous command from a DOM callback does: WebSocket open/message
events need that worker event loop.

**Refinement:** `onmessage` validates and registers work, starts a goroutine, and
returns immediately. Replies are posted from that goroutine. No handler waits
for network progress inside the JavaScript callback.

### Finding 9: worker death makes write outcomes ambiguous

If the backend commits `PutWorkspace` or enrollment and the worker dies before
posting the response, the app cannot know whether it applied. Blind retry can
duplicate non-idempotent work; rollback can contradict server state.

**Refinement:** failure envelopes distinguish “definitely not delivered” from
“may have applied.” Stable enrollment idempotency keys are preserved. Ambiguous
sync writes reconcile through the existing server snapshot/LWW path rather than
rolling back. Automatic worker restart does not automatically replay ambiguous
writes.

### Finding 10: current E2E sync coverage cannot prove the new path

`e2e/regression/sync.spec.mjs` intentionally points at an unreachable port. It
tests error visibility, not successful unary/stream/blob behavior.

**Refinement:** add a hermetic real GoGRPCBridge backend fixture and worker
instrumentation. Keep the unreachable-server regression, but add successful
auth, sync, watch, blob, cancellation, and performance scenarios.

## Refined implementation checklist

The coding sequence follows this list. A later item cannot weaken an earlier
gate.

1. **Freeze the contract and baseline.**
   - Record active RPC methods and current responsive-sync measurements.
   - Preserve the existing unreachable-backend regression.
2. **Make browser contracts gRPC-free.**
   - Move codec registration/options to `internal/backendrpc/grpccodec`.
   - Add a pure RPC code/message/failure envelope.
   - Add a js/wasm dependency test that rejects gRPC/bridge imports in the app
     target.
3. **Build and test the pure protocol.**
   - Versioned ready/fatal/reply/stream/session/binary envelopes.
   - Validation for missing IDs, unsupported versions, unknown methods, invalid
     stream transitions, duplicate terminal events, and unsafe payload sizes.
4. **Build the app-side worker transport.**
   - One worker, readiness timeout, string operation IDs, correlation maps,
     explicit cancellation, stream sequencing, bounded buffers, worker
     `error`/`messageerror`, fatal propagation, and clean shutdown.
5. **Build `services.wasm`.**
   - Immediate-return message callback, operation registry, panic containment,
     typed gRPC status conversion, and every-request terminal response.
6. **Move connection ownership without weakening cross-tab auth.**
   - Endpoint/token-keyed connection reuse, old-token connection invalidation,
     sign-out cleanup, and worker-side operation cancellation.
   - Preserve main-side Web Locks/persistence coordination; execute its
     RefreshToken RPC in the worker and reset the worker session after rotation.
7. **Migrate unary calls.**
   - AI key, auth enrollment/login/redeem/cancel/password/refresh, entitlement,
     workspace list/get/put.
8. **Migrate server streams.**
   - AI completion, pairing status, workspace watch, cancellation, coalescing,
     sequence validation, bounded delivery, and exactly-one terminal event.
9. **Migrate binary/client-stream operations.**
   - Transferable workspace datasets and blob bytes; worker-side blob
     chunking/assembly; size/hash/cancellation checks.
10. **Remove the old browser transport.**
    - No browser `syncbridge`, `grpc/status`, `grpc/codes`, `Dial`, `Invoke`, or
      `NewStream`; delete obsolete shared-connection and refresh machinery from
      `internal/app`.
11. **Wire scheduling safely.**
    - Preserve CashFlux's established state-update semantics. Do not enable GWC's
      global `AsyncIngress` opt-in as part of a transport migration; an initial
      trial changed observable render timing and exposed a credential-editor
      detach race. Fix that component race directly and keep scheduling changes
      separately reviewable.
12. **Build and package both artifacts.**
    - E2E setup, release/deploy, gzip/brotli, sub-path URL resolution, local dev
      wrapper, stale artifact cleanup, service-worker precache/version bump.
13. **Verify behavior and responsiveness.**
    - Native tests/vet; js/wasm dependency tests; both WASM builds; GWC
      unit/wasm lanes; successful hermetic worker RPC/stream/blob/cancel tests;
      existing route and sync/auth regressions; long-task/input-latency controls.
14. **Journal and ship.**
    - Update the durable TODO, changelog, and devlog with measured results,
      commit one logical feature at a time, and push each completed commit.

## Acceptance criteria

- `GOOS=js GOARCH=wasm go list -deps .` contains none of:
  `google.golang.org/grpc`, GoGRPCBridge, `internal/syncbridge`.
- The services target contains all required gRPC/bridge packages.
- No `syncbridge.Dial`, `ClientConn.Invoke`, or `ClientConn.NewStream` remains
  in `internal/app` or `internal/ai`.
- Pure protocol validation, error classification, sequence gaps, duplicate
  termination, and artifact-dependency ownership have native tests.
- A hermetic browser test proves worker readiness and a successful real unary
  workspace sync; the existing native bridge suite continues to cover the
  underlying server-stream and blob contracts.
- Existing route smoke and maintained sync/auth flows remain green.
- Root E2E, GitHub Pages deployment, local development, compressed siblings, and
  offline precache all build/package the worker artifacts.

## Implemented outcome

The migration landed with the following measured gates:

- The release build produces a 79,564,642-byte `main.wasm`; its js/wasm
  dependency graph contains no gRPC, GoGRPCBridge, or `internal/syncbridge`.
  The release `services.wasm` is 20,025,953 bytes and contains the worker-only
  transport stack.
- Every browser unary, AI/pairing/workspace server stream, blob upload/download,
  and RefreshToken gRPC path enters `internal/rpcworker`.
- Workspace datasets and blob bodies cross as transferable `ArrayBuffer`s; blob
  stream chunking and download assembly run in the worker.
- AI chunks are coalesced to a 4 KiB or 16 ms budget before crossing back to the
  page.
- The native suite, vet, both WASM builds, protocol/dependency tests, the
  maintained route smoke suite, the unreachable-backend sync regression, and a
  disposable real-backend workspace-sync E2E pass.
- The real-backend E2E observes exactly one `services-worker.js`, waits for its
  explicit ready signal, and completes a successful GoGRPCBridge workspace
  round trip.

The deeper fault-injection/performance matrix (forced worker death, cancel-table
drain, delayed/high-frequency streams, and multi-megabyte blob latency/Long Task
measurement) remains a separate follow-up rather than an unmeasured claim about
this migration.

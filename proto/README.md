# Backend Proto Contract

CashFlux backend RPCs are defined in `proto/cashflux/v1/cashflux.proto`.

The browser and server currently use the hand-written JSON codec in `internal/backendrpc` over the
GoGRPCBridge tunnel. This proto is the canonical wire-contract target for generated clients and servers once
the repo has `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc` pinned.

## Versioning Policy

- Do not renumber existing fields.
- Do not reuse removed field numbers or names; reserve them in the message before deletion.
- Add fields as optional-compatible proto3 fields with new numbers.
- Keep the HTTP compatibility surface versioned under `/v1`; a future `/v2` must not replace `/v1` until the
  published deprecation window has elapsed.
- Keep synced workspace data opaque: `DatasetEnvelope.gzipped_json` or service-level dataset bytes carry the
  exported CashFlux dataset rather than re-modeling account, transaction, budget, or document entities.
- Keep service method names aligned with `internal/backendrpc` constants until generated code replaces the
  hand-written descriptors.

## Deprecation Windows

- Keep an API version available for at least two minor app releases after the replacement version ships.
- Warn clients through `/v1/version` compatibility fields before removing an endpoint, field, or behavior.
- Breaking changes require a new proto package (`cashflux.v2`) and a matching HTTP prefix (`/v2`).

## Future Codegen

When the toolchain is available, generate Go into `internal/backendrpc/pb` and fail CI if generated files drift
from `proto/cashflux/v1/cashflux.proto`.

# Backend Proto Contract

CashFlux backend RPCs are defined in `proto/cashflux/v1/cashflux.proto`.

The browser and server currently use the hand-written JSON codec in `internal/backendrpc` over the
GoGRPCBridge tunnel. The generated Go package in `internal/backendrpc/pb` is produced from this proto with
Buf so the canonical contract and generated descriptors stay pinned while the transport migration remains
incremental.

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

## Codegen

Run:

```powershell
go run github.com/bufbuild/buf/cmd/buf@v1.57.2 generate
```

`buf.yaml` and `buf.gen.yaml` pin the module layout and remote Go/gRPC plugins. CI runs the same command and
fails if `internal/backendrpc/pb` drifts from `proto/cashflux/v1/cashflux.proto`.

## Future Codegen

The generated `internal/backendrpc/pb` is committed and must stay reproducible from the pinned plugins — never
hand-edit it (that is what the drift check guards). As the contract evolves:

- Bump the plugin versions in `buf.gen.yaml` deliberately, regenerate, and commit the result in the same change,
  so `buf generate` is always a no-op against `HEAD`.
- Keep proto changes additive: add new fields/messages/RPCs rather than renumbering or removing existing ones
  (see the field-numbering rules above). Reserve retired field numbers before deletion.
- New plugins or output targets (e.g. a TypeScript client) are added here as additional `plugins:` entries;
  they must not change the existing Go/gRPC output.

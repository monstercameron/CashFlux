# Assistant coverage and validation

Validated on 2026-10-21 on `refactor/primary-page-components`. This is a development
checkpoint, not a release or a production deployment.

## Access and calculation paths

| Capability | Current path and evidence |
| --- | --- |
| Retained financial data | 77 typed collections; coverage includes all 40 typed dataset slices, archived records, history, financial configuration and financial-plan answers. `data_manifest`, `query_data` and `read_record` discover and recover exact records. |
| Enabled tools | 59 tools in the full-detail catalog. The actual enabled registry test checks unique names, valid object schemas, executable handlers, registered mutation permissions and previews, privacy gates, and relay limits. |
| Formulas | Complete paginated built-in, entity and custom-molecule catalog. Evaluation shares a fixed observation, household scope, base currency, reference time and financial-input fingerprint. Missing or nonfinite inputs remain unavailable. |
| Financial calculations | Checked native services for cash flow, category spending, affordability, balances, projections, goals and reservations, recurring bills, debt, account groups, pools, events, plans, reports, net worth, credit health and financial-plan assessment. Independent valid facts survive unrelated unavailable factors. |
| Reports and tax records | Explicit windows and scope, checked currency conversion, canonical refund/split treatment, exact-decimal CSV, and bounded evidence responses with recovery instructions. Large tax CSV can be reconstructed losslessly through snapshot-bound chunks. |
| Writes and Undo | Preview-bound actions, expected-record checks, native atomic operations where supported, exact change receipts, no-op detection and ordered Undo. Uncertain captures are not reported as verified actions. |
| Conversation lifecycle | Bounded protocol history, session cancellation when the bound conversation or dataset changes, fresh handlers after writes, and late reported usage credited once to the original still-open store. Incomplete provider function calls never dispatch. Automatic title requests also cancel, honor limits and preserve manual names; guarded chat transitions preserve saved messages. |
| Sandboxed changes | What-if calculations use a disposable dataset copy with explicit assumptions; source data is not changed. |
| On-device quick answers | Checked store reads and arithmetic distinguish unavailable values from real zero. Category spending uses the canonical refund-aware model; supported local category questions use the current calendar month and household scope. Other date ranges fall through to provider tools. |

See [Assistant data access](ASSISTANT_DATA_ACCESS.md) for the record protocol,
excluded fields, timestamp semantics and continuation behavior.

## Final test runs

| Gate | Result |
| --- | --- |
| Full native suite, Go 1.26.9, `go test -work ./...` | Passed; 353 package suites reported `ok`. |
| Full `internal/screens` WASM suite, one process | Passed; 1,015 tests, 141.2 seconds, before the final component/local-answer/title additions. |
| Expanded full screen run after those additions | All cases except the new New-chat preservation regression passed. That case exposed intermediate empty-thread persistence; the transition was fixed. |
| Final assistant/component regression gate after the transition fix | Passed; 23 tests, 19.4 seconds, including all Insights lifecycle cases, five actual-store local-answer cases, three component cases and the actual enabled tool catalog. |
| Focused `internal/app` WASM receipt and Undo tests | Passed; 2 tests, 5.4 seconds. |
| Diff whitespace checks | Passed before each feature commit. |

The screen runs include actual App/store fixtures, the enabled catalog,
77-collection coverage, provider-round usage writes, stale-data rejection,
large Unicode records, multi-page formula discovery, a 900-category CSV export,
financial calculation failures, approvals, cancellation and exact Undo behavior.
The previously failing notification availability test passed in the clean full
run; its production assertions were retained. The final transition fix was
verified with the focused gate above; the full screen suite was not repeated
again after that fix.

Test fixtures use disposable stores and mocked providers. No paid model request,
real household mutation, production deployment, tag or release was part of this
validation. These results establish local regression coverage; they do not prove
that every future model answer will select the right tool or interpret it correctly.

## Boundaries and remaining work

- Aggregate-only privacy mode intentionally withholds record-detail capabilities.
  Credentials, vault/auth state, arbitrary settings KV and binary blobs are not
  financial context and are excluded from record tools.
- The legacy Credit page still uses its earlier aggregate adapter. The assistant's
  checked credit calculations preserve currency and independent failure states;
  moving that page onto the same adapter remains a separate refactor.
- Browser fetch-backed web tools discard canceled-session effects, but their
  underlying network fetch is not yet aborted by the session stop signal.
- Temporary view controls, reconciliation drafts, import profiles and internal
  retry bookkeeping are not additional typed financial catalog collections.
- The privacy tier filters injected financial context and available tools.
  Switching an existing conversation to aggregate-only does not retroactively
  remove details already present in its saved conversation text.
- No before/after browser latency pair was captured during this pass. Performance
  improvement is not claimed; large-output and history limits are correctness
  and resource bounds, not a measured responsiveness result.

## Reproduction

Use the pinned Go 1.26.9 toolchain and run native tests with retained work output
on Windows (`-work` avoids the observed temporary executable cleanup failure).
Serialize WASM runners. The full screen suite runs as one process: a large list
of test names in a sharded invocation can exceed the Go/WASM argument limit.

```powershell
go test -work ./...
powershell -NoProfile -ExecutionPolicy Bypass -File tools/wasm-test.ps1 -Packages ./internal/screens/ -count=1
powershell -NoProfile -ExecutionPolicy Bypass -File tools/wasm-test.ps1 -Packages ./internal/app/ -Run '^(TestAssistantCaptureSeparatesManualBaselineAndSkipsNoop|TestUndoExpectedRequiresPublishedTopID)$' -count=1
```

The local live development server remains on port 8080. Validation does not
require starting a second server or replacing the browser's current data.

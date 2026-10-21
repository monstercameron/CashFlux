# v1.19.10 stability checkpoint — October 21, 2026

## Candidate

- GoWebComponents `/v7 v7.0.1`, normal released module, no replacement.
- Go 1.27.3; SchemaFlux latest checked tag is v2.0.1, but CashFlux has no SchemaFlux dependency.
- Current main through `82b3ae13` is integrated, retaining v1.19.9 encrypted sync, local attachment migration, complete backups and credential privacy.
- Admin transport validation runs in the shared native client before cache access or requests.
- Deferred Settings default-tab navigation rechecks the current path, preventing an old effect from replacing a newer page or tab.

## Gates

| Gate | Result |
| --- | --- |
| Native packages with vet | 370 passed after the final navigation fix |
| App WASM | 263 passed in four shards after main integration |
| Settings redirect regression | 1 additional WASM test passed after the final fix; eight ordering/control cases |
| Screens WASM | 1,101 passed after main integration |
| Shared UI WASM | 90 passed |
| Browser-store WASM | 61 passed |
| Production artifact route/theme smoke | 2 passed, 46 route anchors plus light-theme samples, no retries |
| Native View Transitions | Settings → Help → Subscriptions passed with reduced motion and normal motion |
| Encrypted file and later-edit sync | Existing three-browser journey passed, including exact downloaded bytes and encrypted server envelopes |
| Production builds | Main, services, admin, portal WASM and Linux/amd64 server passed |
| Modules | Tidy and verify passed |

The final navigation artifact SHA-256 is:

```text
f3fe8c4c0c2c5cc5b965e2c2849ef5aa54af9d55c7c06782ff2ced742cb23934
```

It was built from the final application sources with the v1.19.10 version change.
The release image additionally receives its final commit/build identity from the existing workflow.

## Evidence and limits

The first production smoke failed because a deferred Settings redirect replaced later navigation.
That failed run is retained separately; it is not counted as a pass. The corrected artifact passed
both smoke tests and focused native-transition checks without retries. Browser contexts were
disposable and external traffic was blocked; no paid AI calls or user household writes were made.

The encrypted sync test closed WebSockets through its isolation wrapper, so it establishes the
tested snapshot/file path and its fallback behavior, not a live WebSocket transport result.
Optional local backend failures and forced raw-WASM 404s in artifact injection were recorded
separately from app errors. The successful browser runs had no page errors.

Local logs use the `stability-v11910-` prefix, including `final-native-all`,
`merged-wasm-all`, `merged-views-wasm`, `settings-redirect-wasm`,
`nav-fixed-browser-smoke`, `sv0-browser` and `nav-fixed-native-transition-diagnostic`.
The app-shard command passed before its wrapper attempted a second runner with a stripped PATH;
the remaining three packages were then run successfully in a fresh PowerShell process.

This checkpoint does not declare the entire componentization/performance objective complete.
The source inventory retains remaining boundaries, the Reports timing regression, and unproven
interaction/cold-start improvements. The user requested this stability release and the end of
the session instead of continuing the eight-hour refactor run.

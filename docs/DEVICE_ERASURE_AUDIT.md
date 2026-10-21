# CashFlux device erasure audit (2026-10-21)

## Confirmed behavior

Settings → Data asks “Erase all CashFlux data on this device?” and labels its button “Erase everything.” The current `wipeAfterSession` signs out, wipes the active SQLite store, and commits an empty active dataset through `wipeFinancialLocalState`. That function deliberately preserves `cashflux:workspaces`, `cashflux:sync-queue`, inactive `cashflux:ws-data:*` bundles, and `cashflux:sync-meta:*`. It does not touch the `cashflux` IndexedDB artifact store. Consequently the UI cannot currently claim that all local financial data or files were erased. The server copy is a separate action and should stay separate.

CashFlux data can live in these browser databases on the same origin:

| Name | Contents | Source |
| --- | --- | --- |
| `cashflux` | Artifact MIME and bytes, including legacy orphan records | `internal/artifactstore/idb.go` |
| `cashflux-kv` and `cashflux-kv-v2:*` | Legacy and scoped protected records, workspace bundles, queue, recovery copies, settings | `internal/browserstore/browserstore.go`, `secure.go` |
| `cashflux-atrest-keys-v1:*` | Scoped key metadata and non-extractable device keys | `internal/atrest/keys_wasm.go` |
| `cashflux-refresh-v1:*` | Scoped refresh credentials | `internal/app/sessioncredentialstore.go` |
| `cfv2:*` | Local database scopes, including encrypted logs and snapshots | `internal/localdb/store.go` |
| `cashflux-fav1-*` and older `cashflux-*` CacheStorage entries | App shell and runtime-cached same-origin GET responses | `web/sw.js` |

Legacy `cashflux:*` localStorage entries also require removal. A fresh boot may retain a nonpersonal “sample already shown” marker to open on a blank budget rather than silently recreate sample transactions. Older service workers cached successful same-origin non-core GET responses, including possible authenticated API responses. The revised worker restricts future cache writes to known static assets and scrubs other entries on activation. Device erasure must still remove its CacheStorage entries; an old worker remains in control until its update activates.

## Required erase behavior

1. Stop this page's autosave and sync, revoke its server session on a best-effort basis, and prevent other open CashFlux tabs from writing old data back. If another tab prevents a complete erase, the UI must report that erasure is unfinished rather than claim success.
2. Remove the active and inactive workspace datasets, pending and conflict snapshots, artifact records, credentials, key stores, local database scopes, CacheStorage entries, and legacy localStorage values. Clear known CashFlux database families for every account and workspace on this origin, not only the current scope.
3. Acknowledge success only after durable browser operations finish and a post-erase inventory confirms no user records remain. A partial failure must be visible and safely retryable. Do not delete the server copy unless the user separately requests that action.
4. Test with two local workspaces, files in each, a queued sync snapshot, and a second browser tab. After erasure and reload, scan raw IndexedDB and localStorage, check the UI has no old household data, and verify the server copy still follows its separate deletion control.

This is an open privacy gap, not a completed erasure implementation.

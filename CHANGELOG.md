# Changelog

All notable changes to CashFlux are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/). Policy: **one feature per commit**,
and every commit updates this file under `Unreleased`.

## [Unreleased]

### Added
- Initialize Go module and `.gitignore`.
- Product specification (`SPEC.md`).
- Project rules and code-quality standards (`CLAUDE.md`), including version-control and journaling
  policy (one feature per commit, changelog + devlog).
- Developer journal (`DEVLOG.md`) and this changelog.
- Consume GoWebComponents as a versioned Go module via `go get` (no local `replace`); Phase 0
  WebAssembly entrypoint (`main.go`) that builds and renders.
- Host page (`index.html`) with wasm boot glue, served via the `gwc dev` live-reload server.
- Wire the `gwc` runner (`.tools/gwc.exe`) and its **MCP server** (`.mcp.json`) for development.
- Framework notes (`docs/GOWEBCOMPONENTS.md`) and a quick-reference section in `CLAUDE.md` for
  new/other sessions.
- Routed app shell (`internal/app`) with top navigation and stub screens for every feature
  (`internal/screens`).
- Master feature backlog (`TODOS.md`), ordered by implementation priority.

- `internal/money`: precise integer-minor-unit `Money` value type with safe, currency-checked
  arithmetic (`Add`/`Sub`/`Neg`/`Abs`/`Cmp`/`Sum`) and table-driven tests (backlog §1.1).
- `internal/money`: `FormatMinor`/`Money.Format` (plain decimal rendering) and `ParseMinor` (strict
  decimal → minor units with validation), round-trip tested — the basis for clean CSV and inputs.
- `internal/currency`: currency registry (code/symbol/decimals/name) + manual `Rates` table with
  base-currency `Convert`/`ToBase` (cross-currency, mixed decimals, nearest-minor rounding) + tests.
- `internal/id`: collision-resistant 128-bit hex ID generator (optional prefix; seedable source for
  deterministic tests) + tests.
- `internal/dateutil`: canonical date parsing/formatting plus month, week, and fiscal-month range
  helpers, `InRange`, and DST-safe `DaysBetween` + tests.
- `internal/domain`: core entity types (`Member`, `Account` incl. liability + allocation fields,
  `Category`, `Transaction`, `Budget`, `Goal`, `Task`) with custom-field maps, plus validated
  enumerations (`AccountClass`/`AccountType`/`CategoryKind`/`Scope`/`Period`/`TaskStatus`/
  `TaskPriority`/`RelatedType`/`TaskSource`), `AccountType.Class()`, transaction classification, and tests.
- `internal/ledger`: account `Balance`/`RunningBalances`, period income/expense totals (transfers
  excluded), `NetWorth` (assets − liabilities), and per-owner net rollups — all multi-currency via
  base conversion + tests.
- `internal/budgeting`: scope-aware `Spent` (individual vs group), `Evaluate`/`EvaluateAll` with
  remaining, percent, and ok/near/over `State` thresholds — multi-currency + tests.
- `internal/goals`: goal `Remaining`, `Percent` (clamped), `IsComplete`, and `Project` (read-only
  completion estimate from an assumed monthly contribution) via `Evaluate` + tests.
- `internal/freshness`: per-type staleness `Windows` (defaults + `Merge` overrides), `IsStale`,
  `DaysSinceUpdate`, and `StaleAccounts`; archived/exempt/untracked accounts never go stale + tests.
- `internal/validate`: per-entity validators returning all `Issues` at once (required fields, valid
  enums, positive amounts, currency consistency, class/type match, score/day ranges, related refs) + tests.
  Completes the Phase 1 pure-logic services layer.
- `internal/store`: pure `Dataset` aggregate + `Settings`, with schema-versioned JSON `Export`/
  `Import` (migration; rejects newer schema) and a lossless round-trip test. Storage-backend-agnostic
  — also the sync/transfer payload.
- `internal/store`: in-memory **SQLite** store backed by the pure-Go (no-cgo) `ncruces/go-sqlite3`
  driver, with `Load`/`Snapshot` clean dataset ingress/egress + round-trip tests. Verified to build
  for `js/wasm` (browser) and run natively.
- `internal/store`: per-entity CRUD (Put/Get/Delete/List for members, accounts, categories,
  transactions, budgets, goals, tasks) and query helpers (transactions by account/category/member/
  date-range via SQLite `json_extract`; tasks by status) + tests.
- `internal/store`: `TransactionsToCSV`/`TransactionsFromCSV` — human-readable CSV with decimal
  amounts, header-name column matching (order/extra-column tolerant), generated ids for id-less rows,
  and per-line error reporting; lossless round-trip tested.

### Changed
- Persistence switched from IndexedDB to pure-Go in-memory SQLite (`ncruces/go-sqlite3`, no cgo, no
  dependency on browser web storage); the JSON `Dataset` remains the portable import/export and sync
  payload. (Confirmed pure-Go SQLite compiles for `js/wasm` and runs in the browser.)
- Expanded `TODOS.md` into a granular, per-entity/service/screen backlog covering the full spec.
- Serve web assets from `web/` (clean project root); restyled host page with a dark theme.
- Require bottom-up SDLC build order in `CLAUDE.md` (data model → services/logic with tests →
  persistence → state → UI last).

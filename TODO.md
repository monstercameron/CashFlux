# CashFlux — Master Feature Backlog

The single source of truth for what to build, **ordered top-to-bottom by implementation
priority**. Work the list in order; within a section, earlier items unblock later ones.
See [`SPEC.md`](./SPEC.md) for product detail and [`CLAUDE.md`](./CLAUDE.md) for the rules.

**Legend:** `[ ]` todo · `[x]` done · `[~]` in progress · `(P#)` target phase ·
`★` critical-path. **Discipline:** one feature per commit; update `CHANGELOG.md` + `DEVLOG.md`
with every commit; logic packages are pure Go (no `syscall/js`) and ship with table-driven tests.

---

## 0. Foundation & tooling  (Phase 0)

- [x] Install toolchain (Go 1.26.4, Git, GitHub CLI) on PATH
- [x] Create repo, choose name, init git on `main`
- [x] Consume GoWebComponents as a versioned Go module (no local replace)
- [x] WASM entrypoint builds and serves
- [x] `gwc` runner + MCP server wired (`.tools/gwc.exe`, `.mcp.json`)
- [x] Init framework `GoGRPCBridge` submodule (unblocks `gwc dev`)
- [x] Product spec, CLAUDE rules, CHANGELOG, DEVLOG, framework notes
- [x] App skeleton: router + shell + nav + stub screens, served on live view
- [x] Clean, standard layout (`main.go`, `internal/`, `web/`, `docs/`)
- [ ] ★ Add `.gitattributes` (normalize LF, mark `*.wasm` binary)
- [ ] Create GitHub repo `monstercameron/CashFlux` and push
- [ ] Decide CI: GitHub Actions running `go test ./...` + wasm build
- [ ] Fix framework `gwc dev -html` to resolve relative to `-root` (commit in GoWebComponents, rebuild + recopy `gwc`)
- [ ] Optional: build a `playwrightgo`-tagged `gwc` + Chromium for automated DOM checks
- [ ] Install Claude Code design skills (`frontend-design`, `playground`) — user action

---

## 1. Phase 1 — Local household core

### 1.1 Domain model & pure logic  ★ (no `syscall/js`; fully unit-tested)

- [ ] ★ `internal/domain`: core types — `Member`, `Account`, `Category`, `Transaction`, `Budget`, `Goal`, `Task`
- [ ] ★ Enums/consts: account `class` (asset|liability), account `type` (checking/debit/savings/cash/credit_card/line_of_credit/loan/personal_loan/mortgage/investment/other), category `kind`
- [ ] ★ Stable ID generation (`internal/id`) — deterministic, testable, collision-safe
- [ ] ★ `internal/money`: integer minor-units `Money{Amount int64, Currency string}`; add/sub/neg/compare; never float
- [ ] Money formatting/parsing per currency (symbol, decimals, grouping); locale-aware later
- [ ] ★ `internal/currency`: currency registry (code, symbol, decimals) + FX rate table type
- [ ] ★ FX conversion to base currency; missing-rate handling; tests for rounding
- [ ] Account balance computation from opening balance + transactions
- [ ] Net worth (assets − liabilities), per-member and group rollups
- [ ] Income/expense totals for a period; transfer exclusion from income/expense
- [ ] Budget spent/remaining computation (individual + group scope)
- [ ] Goal progress computation; projected completion (read-only estimate)
- [ ] Period helpers (month boundaries, fiscal-month start) in `internal/dateutil`
- [ ] ★ Validation rules per entity (required fields, positive amounts, valid refs)
- [ ] Unit tests: money, currency/FX, balances, budgets, goals, validation (table-driven)

### 1.2 Persistence — IndexedDB store  ★

- [ ] ★ `internal/store`: interface over the framework `interop` storage (object stores per entity)
- [ ] DB open/upgrade with schema version; migration scaffold
- [ ] CRUD for each entity (members, accounts, categories, transactions, budgets, goals, tasks)
- [ ] Query helpers (by account, by member, by date range, by category)
- [ ] Settings store (base currency, FX rates, freshness overrides, OpenAI key placeholder)
- [ ] ★ Full export to JSON (schema-versioned, includes settings + custom fields)
- [ ] ★ Import from JSON (validate, merge/replace, version-migrate); **lossless round-trip test**
- [ ] CSV export for transactions
- [ ] CSV import for transactions (column mapping)
- [ ] Seed/sample dataset for first run + a "load sample data" action
- [ ] Tests: store CRUD (native-testable layer separated from JS bindings), import/export round-trip

### 1.3 App state, logging, design system

- [ ] ★ `internal/logging`: `log/slog` handler → browser console + in-app ring buffer
- [ ] Log levels, context fields; a debug log viewer panel
- [ ] ★ State wiring: atoms for members/accounts/categories/transactions/budgets/goals/tasks/settings
- [ ] Store hydration on boot → atoms; persist on mutation (single save path)
- [ ] ★ UI primitives: Button, Input, Select, Field, Money input, Modal, Toast, Badge, ProgressBar, EmptyState, ConfirmDialog
- [ ] Form helper pattern (validation + error display) consistent across screens
- [ ] App-wide currency/number/date formatting helpers bound to settings
- [ ] Responsive layout pass (mobile nav, content widths)
- [ ] Toast/notification system for save/error feedback

### 1.4 Members / Household  ★

- [ ] ★ List members; add/edit/delete; default member; color picker
- [ ] Assign ownership scope to accounts/budgets/goals (individual vs group)
- [ ] Member switcher / filter affecting relevant views
- [ ] Tests: member logic, ownership assignment

### 1.5 Accounts (assets + liabilities)  ★

- [ ] ★ Accounts list grouped by class (assets / liabilities) with balances
- [ ] ★ Add/edit/archive account; choose owner (member/group), type, currency, opening balance
- [ ] Liability fields: credit limit, APR, min payment, due day, lender
- [ ] Allocation attributes capture: expected return APR, liquidity score, stability score, lock-until
- [ ] Per-account ledger view (filtered transactions + running balance)
- [ ] Update-balance action (writes adjustment txn, sets `balanceAsOf`)
- [ ] Account detail cards (utilization for credit, due-date reminders)
- [ ] Tests: balance, utilization, liability fields

### 1.6 Categories

- [ ] Category list; add/edit/delete; income vs expense; color
- [ ] Optional sub-categories (parentId)
- [ ] Default category scheme + ability to reset/customize
- [ ] Reassign transactions when a category is deleted
- [ ] Tests: category tree, reassignment

### 1.7 Transactions (+ transfers, filters)  ★

- [ ] ★ Global ledger list (newest first, paginated/virtualized for large sets)
- [ ] ★ Add transaction (description, amount, type income/expense, category, account, date, member)
- [ ] ★ Edit + delete transaction
- [ ] ★ Transfers between accounts (paired, excluded from income/expense)
- [ ] Tags on transactions
- [ ] Filters: member, account, category, date range, text search; combine + clear
- [ ] Per-row component pattern for actions (respect On*-hooks-in-loops rule)
- [ ] Bulk actions (delete, recategorize) — later in phase
- [ ] Tests: signed amounts, transfer pairing, filter logic

### 1.8 Budgets (individual + group)

- [ ] Budget list (individual + group) with spent vs limit + progress bar
- [ ] Add/edit/delete budget; scope (member/group); category; period (monthly); limit
- [ ] Over/near-limit indicators (gentle, not naggy)
- [ ] Roll-up of group budgets across members
- [ ] Tests: spent/remaining, scope aggregation, alert thresholds

### 1.9 Goals

- [ ] Goal list with progress and projected completion
- [ ] Add/edit/delete goal; scope; target amount; target date; linked account
- [ ] Contribute-to-goal action; progress updates
- [ ] Tests: progress + projection math

### 1.10 To-do (budgeting tasks)

- [ ] Task list (open/done) with due date + priority
- [ ] Add/edit/delete/complete task; link to account/budget/goal/transaction
- [ ] Sort/filter (due, priority, status, linked entity)
- [ ] Surface overdue tasks on dashboard
- [ ] Tests: task ordering, status transitions

### 1.11 Freshness & friendly nudges

- [ ] ★ Freshness model: per-type windows; `balanceAsOf` staleness computation (pure, tested)
- [ ] Configurable windows + overrides in settings
- [ ] Dashboard nudge card ("N balances could use a refresh"), dismissible, one-tap update
- [ ] Per-account staleness indicator
- [ ] Tests: staleness windows, dismissal, recurring-bill exemption

### 1.12 Custom fields (extensibility)

- [ ] `CustomFieldDef` storage + CRUD per entity type
- [ ] Validated `custom{key->value}` map on entities; forms render core + custom fields
- [ ] Types: text/number/date/bool/select/money; required + default
- [ ] Export/import round-trips custom fields
- [ ] Tests: validation, round-trip

### 1.13 Dashboard (depends on most of the above)

- [ ] ★ Net worth + per-member/group rollups
- [ ] This-month income/expense; balance trend snapshot
- [ ] Budget health summary; next goal; overdue tasks
- [ ] Freshness nudges block
- [ ] Recent activity list
- [ ] Custom formula results placeholder (wired in P2)

### 1.14 Settings

- [ ] Members management entry point
- [ ] Base currency selector + editable FX rate table
- [ ] Category management
- [ ] Freshness window overrides
- [ ] OpenAI key + model fields (stored locally; used in P2) with caveat copy
- [ ] Data: export (JSON/CSV), import (JSON/CSV), load sample data, wipe data (confirm)
- [ ] Configurable preferences: theme/density, week-start, fiscal-month start, number/date formats, budgeting methodology
- [ ] Module visibility toggles (show/hide screens)

### 1.15 Phase 1 hardening

- [ ] Accessibility pass (labels, focus, keyboard nav) via framework a11y primitives
- [ ] Empty/error/loading states for every screen
- [ ] Plain-English copy review across all screens, nudges, errors
- [ ] Performance check (large transaction sets, virtualization)
- [ ] Phase 1 README/usage docs + screenshots
- [ ] Tag a Phase 1 release; verify `gwc release` compressed build

---

## 2. Phase 2 — Intelligence & power tools  (OpenAI, client-side)

### 2.1 OpenAI client (bring-your-own-key)

- [ ] `internal/ai`: client over `fetch` to OpenAI with user key from settings
- [ ] Structured outputs (JSON schema) mapped to Go structs
- [ ] Model selection; token/cost surfacing; graceful "AI off until key set" state
- [ ] Error handling (auth, rate limit, CORS) with plain-English messages
- [ ] Tests for request/response mapping (mock transport; keep core pure)

### 2.2 Documents — AI import

- [ ] Upload UI (PDF/CSV/image); local CSV parse in Go
- [ ] Send PDF/image to a vision-capable model → structured transactions
- [ ] Review screen: edit/accept extracted rows → import to ledger
- [ ] `Document` entity lifecycle (parsing/review/imported/discarded); link imported txns
- [ ] Monthly-spend extraction summary

### 2.3 Insights & NL query

- [ ] "Explain my month" generated analysis
- [ ] Natural-language query over data (household-aware)
- [ ] Trend/anomaly highlights; advice cards on dashboard
- [ ] Save/pin insights

### 2.4 Auto-categorization & Rules

- [ ] AI category suggestions on entry + for imported rows
- [ ] `Rule` entity (match payee/desc → category/tags) + management UI
- [ ] AI-proposed rules from history
- [ ] Apply rules on import/entry; tests for rule matching

### 2.5 Formula builder + sandboxed engine

- [ ] ★ `internal/formula`: tokenizer/parser/evaluator — allow-list ops + funcs (sum/avg/min/max/count/if), no arbitrary code
- [ ] Variable resolution: core fields, custom fields, filtered aggregates
- [ ] Typed results (number/money/percent/bool/text) + formatting
- [ ] Builder UI with live preview + validation
- [ ] Surface formula results on dashboard/entities
- [ ] Extensive tests (parsing, evaluation, errors, edge cases)

### 2.6 Planning + Forecast

- [ ] `Recurring` + `Plan`/`PlanItem` entities + management
- [ ] Forecast engine: projected balances/net worth N months out (pure, tested)
- [ ] Debt payoff math (APR, min/extra payments, months-to-zero)
- [ ] What-if scenarios (add recurring, change spend, extra debt payment)
- [ ] Planning screen: build scenario, compare vs actuals, push to forecast
- [ ] Forecast charts/visuals

### 2.7 Capital-allocation engine

- [ ] ★ `internal/allocate`: scoring per criterion (returns, stability, liquidity, debt reduction, goal progress)
- [ ] `AllocationProfile` (weights + constraints + custom/formula criteria) CRUD
- [ ] Constraints: emergency buffer, max-per-destination, exclusions
- [ ] Ranked suggestions with visible per-criterion breakdown
- [ ] Optional AI narrative ("why this split")
- [ ] Extensive tests (scoring, weighting, constraints, determinism)

---

## 3. Phase 3 — Sync & PWA

- [ ] Go sync server (HTTP) sharing client Go structs
- [ ] Auth/account model for the household dataset
- [ ] Pull/push deltas; conflict resolution strategy
- [ ] Client sync integration + status UI; offline queue
- [ ] PWA: manifest, service worker, installability, offline read
- [ ] End-to-end sync tests

---

## 4. Cross-cutting (continuous)

- [ ] Keep logic packages pure + table-driven tested as features land
- [ ] Maintain CHANGELOG + DEVLOG per commit (one feature per commit)
- [ ] Grow the design system rather than one-off styles
- [ ] Accessibility + plain-English copy on every new screen
- [ ] Keep `docs/GOWEBCOMPONENTS.md` and `CLAUDE.md` current
- [ ] CI green (tests + wasm build) before merge
- [ ] Periodic bundle-size check (`gwc wasm measure`)

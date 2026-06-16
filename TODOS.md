# CashFlux — Master Feature Backlog

Single source of truth, **ordered top-to-bottom by implementation priority**. Work in order;
within a section earlier items unblock later ones. Build **bottom-up** per the SDLC rule
(data model → services/logic with tests → persistence → state → UI last). See [`SPEC.md`](./SPEC.md)
for product detail and [`CLAUDE.md`](./CLAUDE.md) for the rules.

**Legend:** `[ ]` todo · `[x]` done · `[~]` in progress · `(P#)` phase · `★` critical path.
**Discipline:** one feature per commit; update `CHANGELOG.md` + `DEVLOG.md` each commit; pure logic
packages have no `syscall/js` and ship with table-driven tests.

---

## 0. Foundation & tooling (Phase 0)

- [x] Install toolchain (Go 1.26.4, Git, GitHub CLI) on PATH
- [x] Init repo, name project, git on `main`
- [x] Consume GoWebComponents as a versioned Go module (no local replace)
- [x] WASM entrypoint builds + serves
- [x] `gwc` runner + MCP server wired (`.tools/gwc.exe`, `.mcp.json`)
- [x] Init framework `GoGRPCBridge` submodule
- [x] Spec, CLAUDE rules, CHANGELOG, DEVLOG, framework notes, this backlog
- [x] Routed app shell + nav + stub screens, served on live view
- [x] Clean standard layout (`main.go`, `internal/`, `web/`, `docs/`)
- [x] ★ `.gitattributes` (normalize LF; mark `*.wasm` binary) — fixes CRLF warnings
- [ ] Create GitHub repo `monstercameron/CashFlux` + push
- [ ] CI: GitHub Actions — `go test` (logic pkgs) + wasm build on push/PR
- [ ] Fix framework `gwc dev -html` resolution (commit in GoWebComponents, rebuild + recopy `gwc`)
- [ ] `playwrightgo`-tagged `gwc` + Chromium for automated DOM verification (optional)
- [ ] Install Claude Code design skills (`frontend-design`, `playground`) — user action
- [ ] Decide native test command (logic pkgs only; js/wasm pkgs excluded) + document it

---

## 1. Phase 1 — Local household core

### 1.1 Domain types — `internal/domain` ★ (pure, no build tags)

- [x] ★ `Member{ID, Name, Color, IsDefault}`
- [x] ★ `Account` core fields: `ID, Name, OwnerID, Scope(individual|shared), Class(asset|liability), Type, Currency, OpeningBalance, BalanceAsOf, Archived`
- [x] ★ Account liability fields: `CreditLimit, InterestRateAPR, MinPayment, DueDayOfMonth, Lender`
- [x] ★ Account allocation fields: `ExpectedReturnAPR, LiquidityScore, StabilityScore, LockUntil`
- [x] ★ `Category{ID, Name, Kind(income|expense), Color, ParentID}`
- [x] ★ `Transaction{ID, AccountID, Date, Payee, Desc, CategoryID, Amount(Money), TransferAccountID, Cleared, Tags, MemberID, SourceDocID}`
- [x] ★ `Budget{ID, Name, Scope(individual|group), OwnerID, CategoryID, Period(monthly), Limit(Money)}`
- [x] ★ `Goal{ID, Name, Scope, OwnerID, TargetAmount, CurrentAmount, TargetDate, AccountID}`
- [x] ★ `Task{ID, Title, Notes, Due, Status(open|done), Priority(low|med|high), RelatedType, RelatedID, MemberID, Source(manual|ai|nudge)}`
- [x] Enums + `Valid()`/`String()` for `AccountClass`, `AccountType`, `CategoryKind`, `Scope`, `TaskStatus`, `TaskPriority`, `RelatedType`
- [x] `custom map[string]any` field on every entity (for custom fields)
- [x] Doc comments on every exported type/field; package doc
- [x] Unit tests: enum `Valid()`/`String()`, zero-value sanity

### 1.2 Money & currency — ★

- [x] ★ `internal/money`: `Money{Amount int64, Currency}`; `Add/Sub/Neg/Abs/Cmp/Equal/Sum`; tests
- [~] Money formatting per currency: `FormatMinor` (plain decimal) done; symbol/grouping/locale = UI layer
- [x] Money parsing: `ParseMinor` (strict decimal → minor units, validation, round-trip) + tests; grouping input later
- [x] ★ `internal/currency`: registry (code, symbol, decimals, name) + `Rates` table type
- [x] ★ `Rates.Convert` / `ToBase` rounding to target minor units (nearest; float-rate caveat noted)
- [x] Missing-rate + non-positive-rate error handling; tests for cross-currency + rounding
- [ ] Helper: format a `Money` in a target/base currency for display

### 1.3 Pure logic services — ★ (each in its own `internal/*` pkg, table-driven tests)

- [x] ★ `internal/id`: stable, collision-safe ID generation (seedable for tests)
- [x] `internal/dateutil`: month boundaries, fiscal-month start, week-start, period ranges
- [x] ★ `internal/ledger`: account balance from opening balance + transactions
- [x] `internal/ledger`: running balance series for an account
- [x] `internal/ledger`: income/expense totals for a period (exclude transfers)
- [x] `internal/ledger`: net worth (assets − liabilities) with multi-currency → base
- [x] `internal/ledger`: per-member and group rollups
- [x] `internal/budgeting`: spent vs limit per budget (individual + group scope)
- [x] `internal/budgeting`: near/over-limit threshold evaluation
- [x] `internal/goals`: progress %, remaining, projected completion (read-only estimate)
- [x] ★ `internal/freshness`: per-type staleness windows + `IsStale(balanceAsOf, type, now)`; recurring-bill exemption
- [x] ★ `internal/validate`: per-entity validation (required, positive amounts, valid refs, currency match)
- [x] Tests for every service above (edge cases, multi-currency, rounding, boundaries)

### 1.4 Persistence — `internal/store` (pure-Go in-memory SQLite via `ncruces/go-sqlite3`) ★

- [x] ★ In-memory SQLite store (`NewMemory`) with clean `Load`/`Snapshot` dataset ingress/egress (builds for js/wasm + native)
- [x] Schema + schema-version constant; migration scaffold (in `Import`) + version bump test
- [x] Object store per entity (members, accounts, categories, transactions, budgets, goals, tasks)
- [x] CRUD per entity (create/get/list/update/delete)
- [x] Query helpers: by account, by member, by date range, by category, by status
- [x] Settings store (base currency, FX rates, freshness overrides, prefs, OpenAI key) — `Get/PutSettings`
- [x] ★ Export entire dataset → versioned JSON (entities + settings + custom fields)
- [x] ★ Import dataset from JSON (version-migrate; rejects newer schema)
- [x] ★ Lossless export→import round-trip test
- [x] CSV export for transactions (stable columns)
- [x] CSV import for transactions (header-name column mapping, error rows; UI preview later)
- [x] Sample dataset (`SampleDataset`) + `Wipe` (data layer; UI "load sample"/"wipe" actions later)
- [x] Tests: pure store logic, query helpers, import/export round-trip, migration

### 1.5 Logging — `internal/logging`

- [x] `log/slog` custom `slog.Handler` → `io.Writer` (browser console writer wired in the app)
- [x] In-app ring buffer sink (bounded) for a debug log viewer
- [x] Level config + contextual fields (`slog.With`/`WithGroup`)
- [ ] Debug log viewer panel (toggleable) — UI
- [x] Tests for the handler/ring buffer (pure parts)

### 1.6 State wiring — `internal/appstate`

- [x] `internal/appstate` seam: in-memory store + slog logger, typed read accessors, validated
      write-through (`Put*`/`Delete*`), JSON export/import; `Init`/`Default` for screens
- [x] Boot hydration: `appstate.Init` loads sample data on boot (wired into `app.Run`)
- [x] Single persist path: every write goes through validated `appstate.Put*` → store (+ slog)
- [x] Reactive refresh per screen (`state.UseAtom` revision bumped after `appstate.Put*`) — Accounts add form
- [ ] Derived/computed selectors (net worth, totals, budget health) via `state.UseComputed` — with screens
- [ ] Error/toast surface for failed persistence — with UI primitives

### 1.7 Design system / UI primitives — `internal/ui`

- [ ] Tokens: colors, spacing, typography scale (extend `web/index.html` styles or a CSS file)
- [ ] Button (variants: primary/secondary/ghost/danger; sizes)
- [ ] Input, NumberInput, MoneyInput (currency-aware), TextArea
- [ ] Select / Dropdown, Combobox
- [ ] Field wrapper (label, hint, error) + form validation pattern
- [ ] Modal / Dialog, ConfirmDialog
- [ ] Toast / notification system
- [ ] Badge, Tag/Chip, ProgressBar, Meter
- [ ] Card, Section, StatCard
- [ ] EmptyState, Skeleton/Loading, ErrorState
- [ ] Table/List with row-component pattern (respect On*-hooks-in-loops)
- [ ] Color picker (members/categories), DatePicker, Icon set
- [ ] Responsive: mobile nav (drawer/hamburger), content widths

### 1.7c Dashboard UI & design system — selected design: `design/candidate-c.html` ★

The chosen visual direction is **candidate C** (flat neutral-dark · Fraunces serif headings + accounting
figures · bento grid · per-widget grip/title/gear · drag-reorder + resize · gear→flip settings ·
collapsible icon sidebar · global-settings flip). The static reference mockup is
[`design/candidate-c.html`](./design/candidate-c.html) (open via the dev server at
`/design/candidate-c.html`). Every item below is a Go/`html/shorthand` component to port from it.
Drag/resize/flip need pointer/drag events via `syscall/js`/`interop`; keep computation in the tested
logic packages, persist layout/settings to the store `Settings`.

**Reusability (required):** build these as generic, props-driven components shared across the whole
app — not per-widget bespoke markup. In particular: one `Widget` shell (grip/title/gear header slots
+ body slot), one `FlipPanel` primitive reused by **both** per-widget and global settings, one
settings-form renderer driven by a field schema, and shared primitives (`Toggle`, `Segmented`,
`StepperPill`, `Swatch`, `Chip`, `ProgressBar`, `Icon` set, and SVG `Chart` helpers). Every widget is
`Widget`-shell + content; every screen composes these. Mark each item below `(reuse)` where a single
component should serve many call sites.

Design tokens & foundation:
- [ ] `internal/ui` tokens (mirror mockup `<style>`): palette (base `#0e0e0f`, tile `#121214`, cell border `#34343a`, divider `#2a2a2c`, fg/dim/faint, up/down/warn), radii (cards squared, controls 4px, bars 2px)
- [ ] Fonts: Fraunces (display headings + figures) + Inter (UI); `.fig` tabular lining figures helper
- [ ] Accounting money display in UI (`$` + thousands + 2dp, **negatives in parentheses**, red/green) — wire `money.FormatMinor` + `currency`
- [ ] Dark modern scrollbar styling for the scroll pane

App shell & navigation:
- [ ] App shell: fixed left rail + independently scrolling `main`; sticky top bar (`h-screen`/overflow)
- [ ] Sidebar rail: brand header; nav items each with an SVG icon (Dashboard/Accounts/Transactions/Budgets/Goals/To-do/Settings)
- [ ] "My pages" section: list of user **custom pages** (+ colored page icons) and a "New page" action
- [ ] Collapsible rail: toggle → 58px icon-only mode (hide labels/captions/brand/household text); persist collapsed state
- [ ] Household card (rail bottom) → opens global settings (see below)
- [ ] Top bar: menu toggle, page title, time-resolution control, `+ Add`

Time-resolution control (top bar):
- [ ] Segmented **Week / Month / Quarter** toggle
- [ ] **From / To** stepper pills that relabel per resolution (week ranges / months / quarters); clamp From ≤ To
- [ ] Drive dashboard period from this control (feeds `ledger`/`budgeting` period range)

Bento grid system:
- [ ] Grid engine: base cell unit `--cell` (152px), N equal columns, uniform gap = outer margin, integer cell spans
- [ ] Visible squared cell borders; full-width header cell (1×N)
- [ ] Widget shell: unified header — **grip (left) · title (center) · gear (right)** + body
- [ ] Drag-to-reorder / swap widgets (pointer/HTML5 DnD via interop), keyed by widget id
- [ ] Resize: scale-wide (right edge) + scale-tall (bottom edge) handles → change col/row span, snap to cells
- [ ] Persist per-user layout (order, spans, hidden, page) to the store

Per-widget settings (gear → flip):
- [ ] Flip primitive: card lifts to center, dim/blur backdrop, 3D `rotateY` to settings back face (reuse for global)
- [ ] Settings back: centered title + right ✕ close; scrollable body; dark Save/Cancel footer
- [ ] Settings fields: editable Title; toggles (allow moving, allow resizing, show header, show on dashboard, compact, show last-updated); accent swatches; default size; refresh; Remove

Widget catalog (each backed by tested logic; see mockup):
- [ ] KPI tile — Net worth / Income / Spending / Liabilities (figure + delta/subline)
- [ ] Recent transactions (table, accounting amounts)
- [ ] Budgets (progress bars, ok/near/over) — `internal/budgeting`
- [ ] Net worth trend (SVG area chart) — `internal/ledger` series
- [ ] Goals (progress) — `internal/goals`
- [ ] To-do (task list)
- [ ] Accounts (mini balances)
- [ ] Cash flow (in/out bar chart per period) — `ledger.PeriodTotals`
- [ ] Upcoming bills (from recurring/liabilities)
- [ ] Savings rate (figure + bar)
- [ ] Spending breakdown (segmented bar + legend by category)
- [ ] Reusable SVG chart helpers (area/sparkline, bars, segmented bar, donut)

Global settings (household card → large flip panel):
- [ ] Large centered flip panel (2-column scrollable body), dark Save/Cancel
- [ ] Household members (chips + add); Base currency; editable FX rate rows
- [ ] AI (OpenAI BYO key toggle + key + model); Appearance (theme seg + accent + density)
- [ ] Data: export JSON/CSV, import, load sample, wipe (confirm)

Shared control components (from mockup):
- [ ] Switch/toggle, swatch picker, segmented control, stepper pill, member chip, ghost/data buttons, dashed "add" button

### 1.8 Members / Household

- [ ] List members; add/edit/delete; set default; color
- [ ] Ownership assignment UI (individual vs group) for accounts/budgets/goals
- [ ] Member switcher / filter affecting relevant views
- [ ] Guard: prevent deleting a member with owned entities (reassign flow)
- [ ] Tests: member logic, ownership rules

### 1.9 Accounts (assets + liabilities) ★

- [x] ★ Accounts list grouped by class (assets / liabilities) with per-account balance
- [~] ★ Add + delete account done (per-row component); edit/archive later
- [ ] Liability sub-form (credit limit, APR, min payment, due day, lender)
- [ ] Allocation attributes sub-form (expected return, liquidity, stability, lock-until)
- [ ] Per-account ledger view (filtered txns + running balance)
- [ ] "Update balance" action → adjustment txn + set `BalanceAsOf`
- [ ] Credit utilization indicator; due-date reminder surfacing
- [x] Net-worth summary header (assets, liabilities, net) in base currency
- [ ] Per-account staleness indicator (from freshness service)
- [ ] Tests already in services; add UI-state tests where logic leaks

### 1.10 Categories

- [ ] List; add/edit/delete; income vs expense; color
- [ ] Sub-categories (parentId) with tree display
- [ ] Default scheme + reset; methodology-aware presets (envelope/zero-based)
- [ ] Reassign transactions on category delete (pick replacement)
- [ ] Tests: tree building, reassignment

### 1.11 Transactions (+ transfers, filters) ★

- [x] ★ Ledger list (newest first); virtualization for large sets later
- [x] ★ Add transaction (desc, amount, income/expense, category, account, date, member)
- [~] ★ Delete transaction done (per-row component); edit + confirm later
- [ ] ★ Transfers between accounts (paired entries; excluded from income/expense)
- [ ] Tags input + tag display
- [ ] Filters: member, account, category, date range, text; combine + clear; persist last filter
- [ ] Sort options (date, amount, payee)
- [ ] Row component for actions; inline category quick-edit
- [ ] Bulk select + bulk delete/recategorize
- [ ] Duplicate / repeat-last helpers
- [ ] Tests: signed amounts, transfer pairing, filter + sort logic

### 1.12 Budgets (individual + group)

- [x] List budgets with spent vs limit + progress bar (current month)
- [~] Add + delete budget (scope, category, limit) done; edit + period selector later
- [x] Near/over-limit indicators (gentle, colored bar)
- [ ] Period selector (this month / specific month)
- [x] Tests: spent/remaining, scope aggregation, thresholds (in `internal/budgeting`)

### 1.13 Goals

- [x] List with progress bar (% + remaining); projected completion later (needs contribution input)
- [~] Add + delete goal (scope, target, current, target date) done; edit + linked account later
- [ ] Contribute-to-goal action; auto-progress from linked account option
- [x] Tests: progress + projection (in `internal/goals`)

### 1.14 To-do (budgeting tasks)

- [x] List (open/done) with due + priority
- [~] Add + complete-toggle + delete done; edit + linking later
- [~] Sort (open first, then due, then title) done; filters later
- [ ] Create-from-nudge and create-from-insight hooks (P2 wires AI source)
- [ ] Tests: ordering, status transitions

### 1.15 Freshness & friendly nudges

- [ ] Dashboard nudge card ("N balances could use a refresh"), dismissible
- [ ] One-tap "update balance" from nudge
- [ ] Per-account staleness badges
- [ ] Configurable windows in settings; recurring-bill exemption respected
- [ ] Tests already in `internal/freshness`; add dismissal-state tests

### 1.16 Custom fields (extensibility)

- [ ] `CustomFieldDef{ID, EntityType, Key, Label, Type, Options, Required, DefaultValue}` + store CRUD
- [ ] Validate `custom{}` map against defs for the entity type
- [ ] Forms render core + custom fields by type (text/number/date/bool/select/money)
- [ ] Custom field management UI (per entity type)
- [ ] Export/import round-trips custom fields + defs
- [ ] Tests: validation, defaulting, round-trip

### 1.17 Dashboard

- [~] Net worth (done) + per-member/group rollups (later)
- [~] This-month income/expense (done); balance trend snapshot (later)
- [ ] Budget health summary; next goal; overdue tasks
- [ ] Freshness nudges block
- [x] Recent activity list
- [ ] Placeholder slots for AI insight + formula results (wired P2)

### 1.18 Settings

- [ ] Members management entry
- [ ] Base currency selector + editable FX rate table (add/edit/remove rate)
- [ ] Category management entry
- [ ] Freshness window overrides editor
- [ ] OpenAI key + model fields (stored locally) with caveat copy (used P2)
- [ ] Data: export JSON, export CSV, import JSON, import CSV, load sample, wipe (confirm)
- [ ] Preferences: theme/density, week-start, fiscal-month start, number/date formats
- [ ] Budgeting methodology selector (envelope / zero-based / simple tracking)
- [ ] Module visibility toggles (show/hide screens)
- [ ] Debug: open log viewer

### 1.19 Configuration & modalities

- [ ] Layered config resolution: defaults → household → member → screen
- [ ] Config persisted + included in export/import
- [ ] Methodology changes adjust UI affordances (e.g. envelope view)
- [ ] Per-member preferences (formatting, default account/member)
- [ ] Tests: config layering/resolution

### 1.20 Phase 1 hardening

- [ ] Accessibility pass (labels, focus order, keyboard nav, ARIA) via framework a11y
- [ ] Empty/error/loading states on every screen
- [ ] Plain-English copy review (labels, nudges, errors, confirmations)
- [ ] Performance: large dataset (10k+ txns) virtualization + memoization
- [ ] Usage docs + screenshots; update framework notes if APIs learned
- [ ] Phase 1 release via `gwc release`; verify compressed sizes (`gwc wasm measure`)

---

## 2. Phase 2 — Intelligence & power tools (OpenAI, client-side)

### 2.1 OpenAI client — `internal/ai`

- [ ] Client over `fetch` with user key from settings; base URL configurable
- [ ] Chat/Responses call with JSON-schema **structured outputs** → Go structs
- [ ] Vision input support (images/PDF pages) for document parsing
- [ ] Model selection; token + cost surfacing; "AI off until key set" state
- [ ] Error handling: auth, rate limit, network, CORS — plain-English messages
- [ ] Retry/backoff; request cancellation
- [ ] Tests: request build + response decode via mock transport (keep core pure)

### 2.2 Documents — AI import

- [ ] Upload UI (PDF / CSV / image); drag-drop
- [ ] Local CSV parse → candidate transactions (no AI needed)
- [ ] Send PDF/image to vision model → structured transactions
- [ ] `Document{ID, Filename, Kind, UploadedAt, AccountID, MemberID, Status, Extracted[]}` lifecycle
- [ ] Review screen: edit/accept/reject extracted rows → import to ledger; dedupe vs existing
- [ ] Monthly-spend extraction summary view
- [ ] Tests: CSV parsing, extraction-to-transaction mapping, dedupe

### 2.3 Insights & NL query

- [ ] "Explain my month" generated narrative
- [ ] Natural-language query over data (household-aware) → answer + supporting figures
- [ ] Trend/anomaly highlights; advice cards
- [ ] Pin/save insights; show top insight on dashboard
- [ ] Guardrails: scope data sent, redact where possible
- [ ] Tests: prompt assembly, data-context selection (pure parts)

### 2.4 Auto-categorization & Rules

- [ ] `Rule{ID, Match, SetCategoryID, SetTags}` store + management UI
- [ ] Rule matching engine (pure) + tests
- [ ] AI category suggestion on entry + for imported rows
- [ ] AI-proposed rules from history (review + accept)
- [ ] Apply rules on import/entry; conflict handling

### 2.5 Formula builder + sandboxed engine — `internal/formula`

- [ ] ★ Tokenizer (numbers, strings, idents, operators, parens, commas)
- [ ] ★ Parser → AST (precedence, unary, function calls)
- [ ] ★ Evaluator with allow-list functions (`sum/avg/min/max/count/if/round/abs`) + arithmetic/compare
- [ ] Variable resolution: core fields, custom fields, filtered aggregates over transactions
- [ ] Typed results (number/money/percent/bool/text) + formatting
- [ ] `Formula{ID, Name, Target, Expr, ResultType, Format, Enabled}` store + CRUD
- [ ] Builder UI: guided insert, live preview, validation + error messages
- [ ] Surface results on dashboard / relevant entities
- [ ] ★ Extensive tests: tokenizer, parser, evaluator, errors, security (no escape), edge cases

### 2.6 Planning + Forecast

- [ ] `Recurring{ID, Kind, Label, Amount, Currency, Cadence, NextDate, AccountID, CategoryID, Autopost}` + CRUD
- [ ] `Plan{ID, Name, HorizonMonths, BaseScenario, Assumptions[]}` + `PlanItem{...}` + CRUD
- [ ] ★ Forecast engine (pure): projected balances/net worth over horizon from actuals + recurring + plan items
- [ ] Debt payoff math (APR accrual, min/extra payments, months-to-zero, interest paid) + tests
- [ ] What-if scenarios (add recurring, change spend, extra debt payment, rate change)
- [ ] Planning screen: build scenario, compare vs actuals, push to forecast
- [ ] Forecast visualization (balance/net-worth curve)
- [ ] ★ Tests: forecast projection, payoff math, scenario application

### 2.7 Capital-allocation engine — `internal/allocate`

- [ ] ★ Criterion scorers: returns, stability, liquidity/ease-of-withdrawal, debt reduction, goal progress
- [ ] ★ Weighted combination by profile; normalization; deterministic
- [ ] `AllocationProfile{ID, Name, Weights, Constraints, CustomCriteria[formulaID]}` + CRUD
- [ ] Constraints: emergency buffer, max-per-destination, exclusions — applied/clamped
- [ ] Candidate set assembly (asset accounts, goals, high-interest liabilities as guaranteed return)
- [ ] Ranked output with per-criterion breakdown (no black box)
- [ ] Allocate screen: amount input + profile select → ranked suggestions
- [ ] Optional AI narrative ("why this split")
- [ ] ★ Extensive tests: scoring, weighting, constraints, determinism, custom criteria

---

## 3. Phase 3 — Sync & PWA

### 3.1 Sync server (Go)

- [ ] HTTP service sharing client domain structs
- [ ] Household account/auth model
- [ ] Endpoints: pull deltas, push deltas, full snapshot, health
- [ ] Conflict resolution strategy (last-write-wins + vector/seq) + tests
- [ ] Storage backend (sqlite/file) for the household dataset

### 3.2 Client sync

- [ ] Sync client in wasm app; background sync + status UI
- [ ] Offline mutation queue + replay
- [ ] Settings toggle + endpoint/credentials
- [ ] End-to-end sync tests

### 3.3 PWA / offline

- [ ] Web manifest + icons
- [ ] Service worker (cache shell + assets)
- [ ] Installability prompt; offline read; update flow
- [ ] Verify via framework `pwa` package

---

## 4. Cross-cutting (continuous)

- [ ] Keep logic packages pure + table-driven tested as features land
- [ ] One feature per commit; CHANGELOG + DEVLOG updated every commit
- [ ] Grow the design system rather than one-off styles
- [ ] Accessibility + plain-English copy on every new screen
- [ ] Keep `docs/GOWEBCOMPONENTS.md`, `CLAUDE.md`, `SPEC.md`, `TODOS.md` current
- [ ] CI green (tests + wasm build) before merge
- [ ] Periodic bundle-size check (`gwc wasm measure`)
- [ ] Security review before any data leaves the device (AI calls): scope + redaction

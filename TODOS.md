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
- [x] CI: GitHub Actions — `go vet` + `go test` (logic pkgs) + wasm build on push/PR (`.github/workflows/ci.yml`)
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
      (LockUntil set on add + inline edit; excludes locked accounts from allocation)
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
- [x] `internal/ledger`: cleared balance (opening + cleared txns) for reconciliation
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
- [x] Debug log viewer panel (in the Settings screen, newest-first + Refresh)
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
- [x] `internal/ui` tokens (mirror mockup `<style>`): palette + radii — Tailwind config + design-system CSS in host page; legacy screens retargeted to match
- [x] Fonts: Fraunces (display headings + figures) + Inter (UI); `.fig` tabular lining figures helper
- [x] Accounting money display in UI (`$` + thousands + 2dp, **negatives in parentheses**, red/green) — `money.FormatAccounting` + `fmtAccounting`/`figTone`
- [x] Dark modern scrollbar styling for the scroll pane (`main.cf-scroll`)

App shell & navigation:
- [x] App shell: fixed left rail + independently scrolling `main`; sticky top bar
- [x] Sidebar rail: brand header; nav items each with an SVG icon — `internal/ui.Icon` + `navItem`
- [x] "My pages" section: example custom pages (+ colored page icons) and a "New page" action
- [x] Collapsible rail: toggle → 58px icon-only mode (shared `rail:collapsed` atom); reload-persist later
- [x] Household card (rail bottom) → opens global settings
- [x] Top bar: menu toggle, page title, time-resolution control, `+ Add`

Time-resolution control (top bar):
- [x] Segmented **Week / Month / Quarter** toggle (`ui.Segmented`)
- [x] **From / To** stepper pills that relabel per resolution; clamp From ≤ To (`period.Window`)
- [x] Drive dashboard period from this control (`uistate` window → `ledger.PeriodTotals`)

Bento grid system:
- [x] Grid engine: base cell unit `--cell` (152px), equal columns, uniform gap, integer cell spans
- [x] Visible squared cell borders; full-width header cell (1×N)
- [x] Widget shell: unified header — **grip · title · gear** + body (`ui.Widget`)
- [x] Drag-to-reorder / swap widgets (HTML5 DnD), keyed by widget id (`dashlayout.Swap`)
- [x] Resize: right/bottom handles → change col/row span (`dashlayout.Resize`; click-cycle for now, pointer-drag later)
- [~] Persist per-user layout — order + spans saved to `localStorage`; hidden/per-page + store persistence later

Per-widget settings (gear → flip):
- [x] Flip primitive: card lifts to center, dim/blur backdrop, 3D `rotateY` (`ui.FlipPanel`, reused for global)
- [x] Settings back: centered title + right ✕ close; scrollable body; dark Save/Cancel footer
- [~] Settings fields: editable Title + behavior toggles done; accent swatches/default size/refresh/Remove + persistence later

Widget catalog (each backed by tested logic; see mockup):
- [x] KPI tile — Net worth / Income / Spending / Liabilities (figure + subline)
- [x] Recent transactions (table, accounting amounts)
- [x] Budgets (progress bars, ok/near/over) — `internal/budgeting`
- [x] Net worth trend (SVG area chart) — `ledger.NetWorthSeries` + `chart`/`ui.AreaChart`
- [x] Goals (progress) — `internal/goals`
- [x] To-do (task list)
- [x] Accounts (mini balances)
- [x] Cash flow (in/out bar chart per period) — `ledger.PeriodTotals`
- [x] Upcoming bills (from liabilities' due day + min payment)
- [x] Savings rate (figure + bar)
- [x] Spending breakdown (segmented bar + legend by category)
- [~] Reusable SVG chart helpers — area/sparkline (`chart` + `ui.AreaChart`) done; bars are div-based; donut later

Global settings (household card → large flip panel):
- [x] Large centered flip panel (2-column scrollable body), dark Save/Cancel
- [x] Household members (chips + add); Base currency; editable FX rate rows (live reads)
- [x] AI (OpenAI BYO key toggle + key + model); Appearance (theme seg + accent + density) — UI (local state)
- [x] Data: export JSON/CSV, import, load sample, wipe (confirm) — wired via `appstate`

Shared control components (from mockup):
- [x] Switch/toggle, swatch picker, segmented control, stepper pill, member chip, data buttons, dashed "add" button (`internal/ui` + settings)

### 1.8 Members / Household

- [x] List members; add/delete; set default; color; inline edit (name + color)
- [x] Ownership assignment UI (individual vs group) — set at creation everywhere and editable inline
      on accounts, budgets, and goals (shared `ownerSelectOptions` owner picker)
- [~] Member switcher / filter — per-member "Transactions" drill-down filters the ledger by member;
      global cross-screen member scope deferred (ambiguous semantics)
- [x] Member delete: reassign owned accounts/budgets/goals (+ transactions) to another owner via
      `appstate.ReassignOwner` + Members reassign panel, then delete
- [ ] Tests: member logic, ownership rules

### 1.9 Accounts (assets + liabilities) ★

- [x] ★ Accounts list grouped by class (assets / liabilities) with per-account balance
- [x] ★ Add + delete + archive/restore + inline edit account (name, opening balance, type attributes)
- [x] Liability sub-form (credit limit, APR, min payment, due day, lender) — shown for liability types
- [x] Allocation attributes sub-form (expected return, liquidity, stability, lock-until) on add + edit
- [~] Per-account ledger view — account row "Transactions" button filters the ledger to that account
      and navigates; dedicated running-balance view optional later
- [x] "Update balance" action → cleared adjustment txn for the difference + set `BalanceAsOf`
- [~] Credit utilization indicator done (on liability rows); due-date reminder via Upcoming bills widget
- [x] Net-worth summary header (assets, liabilities, net) in base currency
- [x] Per-account staleness indicator (Stale badge) + per-row "Mark updated" + bulk "Mark all updated"
- [ ] Tests already in services; add UI-state tests where logic leaks

### 1.10 Categories

- [x] List + add + delete + inline edit (name + kind); income vs expense
- [x] Sub-categories (parentId): engine + parent picker (add & inline edit) + indented lists +
      dashboard breakdown rolls sub-category spend up to the top-level parent
- [ ] Default scheme + reset; methodology-aware presets (envelope/zero-based)
- [x] Reassign transactions on category delete (pick replacement) — `appstate.ReassignCategory` +
      Categories reassign panel (moves transactions and budgets, then deletes)
- [~] Tests: tree building, reassignment — reassignment tested; category tree building N/A (flat list)

### 1.11 Transactions (+ transfers, filters) ★

- [x] ★ Ledger list (newest first); virtualization for large sets later
- [x] ★ Add transaction (desc, amount, income/expense, category, account, date, member)
- [x] ★ Delete + inline edit transaction (desc, amount, category, date; non-transfers; sign preserved)
- [x] ★ Transfers between accounts (paired entries; excluded from income/expense); deleting one leg removes both
- [x] Tags input + tag display (income/expense); search matches tags
- [x] Filters: member, account, category, text, date range + sort (combine + clear) + persist last
      filter across reloads (`uistate.UseTxFilter` localStorage atom)
- [x] Sort options (date, amount, payee)
- [x] Export the filtered/sorted view to CSV (`applyTxFilter` shared with the list)
- [x] Filtered summary line: count + net total (base currency) of the shown set
- [x] Row component for actions; inline edit (incl. category) — `TransactionRow` edit mode
- [x] Bulk select + bulk delete (transfer-aware) + bulk recategorize + bulk mark cleared/uncleared
- [x] Repeat-last helper (pre-fills form from newest txn) + per-row duplicate (copies row to today)
- [x] Cleared/reconciled toggle per transaction + cleared-status filter (both/not/cleared, persisted)
- [ ] Tests: signed amounts, transfer pairing, filter + sort logic

### 1.12 Budgets (individual + group)

- [x] List budgets with spent vs limit + progress bar (current month)
- [x] Add + delete + inline edit budget (name, limit, period) + weekly/monthly/quarterly periods
      (engine `PeriodRange` + selector + per-budget evaluation honoring week-start)
- [x] Near/over-limit indicators (gentle, colored bar)
- [x] Period selector (month stepper) — view any month
- [x] Tests: spent/remaining, scope aggregation, thresholds (in `internal/budgeting`)

### 1.13 Goals

- [x] List with progress bar (% + remaining) + pace guidance + combined progress header (saved/target/%)
- [x] Add + delete + inline edit goal (name, target, target date, linked account) + contribute
- [~] Contribute-to-goal action done (prompt); auto-progress from linked account later
- [x] Tests: progress + projection (in `internal/goals`)

### 1.14 To-do (budgeting tasks)

- [x] List (open/done) with due + priority
- [x] Add + complete-toggle + delete + inline edit (title, priority, due, notes); linking later
- [~] Sort (open first, then due, then title) + hide-done filter done; more filters later
- [x] Create-from-insight (Insights "Save as task" → AI task) + create-from-nudge (freshness
      "Remind me" → nudge task) hooks
- [ ] Tests: ordering, status transitions

### 1.15 Freshness & friendly nudges

- [~] Dashboard nudge widget ("N balances could use a refresh") done; dismissible + one-tap update later
- [ ] One-tap "update balance" from nudge
- [ ] Per-account staleness badges
- [ ] Configurable windows in settings; recurring-bill exemption respected
- [ ] Tests already in `internal/freshness`; add dismissal-state tests

### 1.16 Custom fields (extensibility)

- [x] `CustomFieldDef{ID, EntityType, Key, Label, Type, Options, Required}` + store CRUD
      — `customfields.Def` (pure) + `customfielddefs` table CRUD + `CustomFieldDefsByEntity`; appstate accessors
- [x] Validate `custom{}` map against defs for the entity type — `customfields.Validate`, table-tested
- [x] Forms render core + custom fields by type (text/number/date/bool/select) — `CustomFieldInput`
      on all five entity forms (accounts, transactions, budgets, goals, members)
- [x] Custom field management UI (per entity type) — `CustomFieldsManager` on the Customize screen
- [x] Export/import round-trips custom field defs — dataset + Export/Import covered by tests
- [x] Tests: validation (value + Def), round-trip (store CRUD, dataset, export/import), save-path enforcement

### 1.17 Dashboard

- [x] Net worth + per-member/group rollups (Members screen "Net worth by member")
- [~] This-month income/expense (done); balance trend snapshot (later)
- [ ] Budget health summary; next goal; overdue tasks
- [ ] Freshness nudges block
- [x] Recent activity list
- [ ] Placeholder slots for AI insight + formula results (wired P2)

### 1.18 Settings

- [ ] Members management entry
- [ ] Base currency selector + editable FX rate table (add/edit/remove rate)
- [ ] Category management entry
- [x] Freshness window overrides editor — per-type day inputs in Settings writing
      `Settings.FreshnessOverrides`, applied via `appstate.FreshnessWindows`
- [x] OpenAI key + model fields persist to Settings (global panel) — used by Insights
- [ ] Data: export JSON, export CSV, import JSON, import CSV, load sample, wipe (confirm)
- [~] Preferences: theme/density, week-start, fiscal-month start, number/date formats
      — theme (dark/light/system) + accent + density + week-start + date format all complete &
        reload-persistent (engine + atom + Settings UI + `ApplyPrefs` + light/dark skins);
        only fiscal-month start remains
- [ ] Budgeting methodology selector (envelope / zero-based / simple tracking)
- [x] Module visibility toggles (show/hide screens) — end-to-end: pure `internal/modules` +
      localStorage atom + sidebar filter + Settings per-screen toggles, reload-persistent
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

- [x] Client over `fetch` with user key from settings; base URL configurable — `ai.SendChat`
- [ ] Chat/Responses call with JSON-schema **structured outputs** → Go structs
- [~] Vision input support (images/PDF pages) for document parsing — `ai.BuildVisionRequest` (pure) done
- [ ] Model selection; token + cost surfacing; "AI off until key set" state
- [ ] Error handling: auth, rate limit, network, CORS — plain-English messages
- [ ] Retry/backoff; request cancellation
- [x] Request build + response decode (pure codec, round-trip tested) — `internal/ai`

### 2.2 Documents — AI import

- [~] Upload UI (CSV paste + image picker) done; PDF + drag-drop later
- [x] Local CSV parse → import transactions (no AI needed) — Documents screen paste-and-import
- [x] Send PDF/image to vision model → structured transactions — `ai.BuildVisionRequest` +
      `ai.SendVisionChat` + `extract.ParseRows` + Documents image-import UI (choose → read → review → import)
- [ ] `Document{ID, Filename, Kind, UploadedAt, AccountID, MemberID, Status, Extracted[]}` lifecycle
- [x] Review screen: list + per-row edit + per-row remove → import to ledger (account-pick) +
      dedupe vs existing (skip same date+amount in account, reported)
- [ ] Monthly-spend extraction summary view
- [~] Tests: CSV parsing (store) + extraction parsing/dedupe (`extract`) done; extraction→txn mapping is UI

### 2.3 Insights & NL query

- [x] "Explain my month" generated narrative (Insights screen)
- [~] Natural-language query over data → answer (Insights "Ask about your money"); richer data context later
- [ ] Trend/anomaly highlights; advice cards
- [ ] Pin/save insights; show top insight on dashboard
- [ ] Guardrails: scope data sent, redact where possible
- [ ] Tests: prompt assembly, data-context selection (pure parts)

### 2.4 Auto-categorization & Rules

- [ ] `Rule{ID, Match, SetCategoryID, SetTags}` store + management UI
- [x] Rule matching engine (pure) + tests — `internal/rules` (Category/Tags/FirstMatch)
- [~] Rule-based category suggestion on entry (category-name match) done; AI suggestion + import rows later
- [ ] AI-proposed rules from history (review + accept)
- [ ] Apply rules on import/entry; conflict handling

### 2.5 Formula builder + sandboxed engine — `internal/formula`

- [x] ★ Tokenizer (numbers, strings, idents, operators, parens, commas) — `internal/formula.Tokenize`
- [x] ★ Parser → AST (precedence, unary, function calls) — `internal/formula.Parse`
- [x] ★ Evaluator with allow-list functions (`sum/avg/min/max/count/if/round/abs`) + arithmetic/compare — `internal/formula.Eval`
- [~] Variable resolution: live figures (net worth/income/expense/counts) done via `Env`; custom fields + filtered aggregates later
- [~] Typed results (number/bool/text) done; money/percent typing + formatting later
- [ ] `Formula{ID, Name, Target, Expr, ResultType, Format, Enabled}` store + CRUD
- [~] Builder UI: live preview + error messages + example chips done (Customize); guided insert later
- [ ] Surface results on dashboard / relevant entities
- [ ] ★ Extensive tests: tokenizer, parser, evaluator, errors, security (no escape), edge cases

### 2.6 Planning + Forecast

- [ ] `Recurring{ID, Kind, Label, Amount, Currency, Cadence, NextDate, AccountID, CategoryID, Autopost}` + CRUD
- [ ] `Plan{ID, Name, HorizonMonths, BaseScenario, Assumptions[]}` + `PlanItem{...}` + CRUD
- [~] ★ Forecast engine (pure): `internal/forecast.Project` over horizon from start + recurring + one-time items done; actuals-derived recurring later
- [x] Debt payoff math (`internal/payoff.Project`) + tests + extra-payment scenario (months/interest saved)
- [~] What-if scenarios: extra debt payment + trim-spending forecast done; add-recurring/rate-change later
- [ ] Planning screen: build scenario, compare vs actuals, push to forecast
- [~] Forecast visualization (net-worth curve) done on Planning; scenario comparison later
- [ ] ★ Tests: forecast projection, payoff math, scenario application

### 2.7 Capital-allocation engine — `internal/allocate`

- [~] ★ Criterion scorers: returns, stability, liquidity, debt reduction done (`internal/allocate`); goal-progress criterion later
- [x] ★ Weighted combination by profile; normalization; deterministic (`Score`/`Rank`)
- [ ] `AllocationProfile{ID, Name, Weights, Constraints, CustomCriteria[formulaID]}` + CRUD
- [~] Constraints: emergency buffer, max-per-destination, exclusions — applied/clamped
      — exclusions complete (engine + UI); emergency buffer + max-per-destination implemented in the
        `Distribute` split engine (tested); amount-split UI next
- [x] Candidate set assembly (asset accounts + high-interest liabilities + unfinished goals)
- [x] Ranked output with per-criterion breakdown (no black box)
- [x] Allocate screen: profile select → ranked suggestions + exclude/restore + amount-split input
      (amount + emergency buffer → per-destination dollar amounts via `Distribute`, with kept-back note)
- [x] Optional AI narrative ("Explain with AI" on the Allocate screen)
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

- [~] Web manifest done (`manifest.webmanifest` + theme-color/apple meta); icons later
- [x] Service worker (`sw.js`): network-first cache of shell + assets, offline fallback
- [~] Installability prompt done (beforeinstallprompt button); offline read works (sw); update flow later
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

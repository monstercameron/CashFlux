# CashFlux — Developer Journal

Narrative companion to `CHANGELOG.md`. Newest entries first. Capture decisions, trade-offs,
problems and fixes, and what's next.

## 2026-06-15 — Porting candidate C: design-system foundation

- Resumed the `/loop`, now executing §1.7c (port the chosen design into Go components), one feature
  per commit. First feature is the foundation everything else references.
- **Decision — adopt Tailwind (CDN) + the candidate-C custom CSS** rather than re-authoring every
  utility class as semantic CSS. The mockup was authored in Tailwind; faithful reproduction and low
  drift matter more here than shedding a CDN dependency, and it keeps the port mechanical. The
  palette/type scale live in `tailwind.config`; the bespoke component CSS (bento, widget header,
  drag/resize, flip panel, scrollbar, sidebar collapse, settings controls) is a single `<style
  id="design-system">` block ported verbatim from `design/candidate-c.html`.
- **Additive, not a switch-over:** the old semantic theme + top-nav shell still render so the build
  stays green at every commit; the new tokens just become available. The scroll-pane and body
  scrollbar selectors were namespaced (`main.cf-scroll`, `body.cf`) so they only apply once the new
  shell opts in, avoiding restyling the current screens mid-migration.

- Added the **accounting money formatter** as pure logic before any UI uses it (SDLC bottom-up):
  `money.Group` for thousands separators and `money.FormatAccounting` for the candidate-C figure
  style — symbol-prefixed, always two decimals, negatives in parentheses (`($240.55)`). Kept in
  `internal/money` (pure, native-tested) and currency-registry-free by taking the symbol as an
  argument, so the js/wasm screen layer composes `currency.Symbol(...)` + this without leaking the
  registry into money. Table-driven tests cover grouping boundaries, zero, sub-unit, and millions.

- Confirmed the framework's `html/shorthand` exposes SVG element constructors (`Svg`/`Path`/`Rect`/
  `Circle`/`G`/`Line`/`Polyline`/`Polygon`), a generic `Attr`/`Attrs` for arbitrary attributes, and a
  full pointer/drag event set (`OnPointerDown/Move/Up`, `OnDragStart/Over/Drop/End`) — so the SVG
  icons, charts, and the drag-reorder/resize interactions can all be expressed natively in Go.
- Started the shared design-system package `internal/ui` (js/wasm-tagged) with the first reusable
  primitive: **`Icon`** — the candidate-C stroked SVG set as a single props-driven component
  (`Icon(name, extra...)`), color via `currentColor`, size via caller classes. Builds clean for
  `GOOS=js GOARCH=wasm`.

- Ported the **app shell** to the candidate-C layout: `internal/app/shell.go` now renders a fixed
  left rail (`Sidebar`) + an independently scrolling `main.cf-scroll` pane with a sticky `TopBar`,
  replacing the old top-nav `Shell`/`NavBar`. The rail's primary nav is data-driven (`primaryNav()`)
  and each entry is rendered by a `navItem` component so its click hook stays stable (On*-in-loops
  rule). Imported the framework `ui` as `uic` to avoid colliding with our `internal/ui` (`ui`).
- Kept design data in the design layer: the route→icon mapping lives in `primaryNav()` (not the
  screen registry), so `internal/screens` stays free of presentation concerns. Phase-2 routes and
  Settings are reachable by URL but not yet in the rail (the My-pages/System groups come next).
- Full `GOOS=js GOARCH=wasm` build is green (~22 MB). Top bar's menu toggle, time-resolution control,
  and the Add action are present but static for now — wired in upcoming features.

- Completed the rail: **My pages** (example custom pages with colored page icons + a muted "New page"
  action), **System** (Settings), and a bottom **household card** that reads live member count + base
  currency from `appstate` and navigates to Settings (the global-settings flip panel replaces that
  navigate later). Generalized `navItem` into the one reusable rail primitive — optional `Path`
  (empty = non-navigating placeholder, used by the example pages until custom pages are real),
  `IconClass` for per-item icon tinting, and `Muted` styling. Section headers are direct `<div>`
  children of `<nav>` so the collapsed-rail CSS (`nav > div { display:none }`) hides them cleanly.

- Added `.gitattributes` (LF normalization + binary marks); `git add --renormalize` was a no-op
  (repo blobs were already LF), so the Windows CRLF warnings are gone with a one-file commit.
- **Collapsible rail**: the framework's `state.UseAtom` is global-by-id and re-renders every
  subscribed component, so a shared `rail:collapsed` bool atom cleanly coordinates the top-bar menu
  button (toggles) and the `Sidebar` (adds the `collapsed` class → the CSS does the 58px icon-only
  switch). No `syscall/js` needed. Collapse persists across navigation (atom is global); persisting
  across reloads waits on the prefs/settings wiring.

- Built the first **shared control primitives** in `internal/ui`: `Segmented` and `StepperPill`,
  both generic and props-driven. Each follows the export-thin-wrapper-over-CreateElement pattern so
  every call site is its own component instance with isolated hooks, and the per-option `segButton`
  is itself a component (the On*-in-loops rule). These back the time-resolution control next but are
  written for reuse anywhere (theme toggle, paging, etc.).

- Modeled the time-resolution control **bottom-up first**: new pure `internal/period` package wraps
  `dateutil` with a `Resolution` (week/month/quarter) and anchor math — `Truncate` (snap to unit
  start), `Step` (move by whole units), `Label` ("Jun 2026" / "Q3 2026" / "Jun 15 – Jun 21"), and
  `Range` (from/to anchors → half-open reporting range, with a to<from clamp). Table-driven tests
  green on native Go. The UI will just hold the resolution + two anchors in state and call this.

- Added an immutable `period.Window` (resolution + from/to anchors + week start) holding all the
  control's stepping/clamping rules as pure, tested methods (`SetResolution`, `StepFrom`/`StepTo`
  with the from ≤ to clamp, `Range`, labels). This is the value the UI will store in a single atom,
  so the top-bar control and dashboard share one source of truth and the view stays logic-free.

- Wired the **time-resolution control**: new `internal/uistate` package holds the shared dashboard
  window in one atom over `period.Window` (a neutral home so neither the app shell nor screens own
  it and there's no import cycle). The top-bar `ResolutionControl` composes `Segmented` +
  two `StepperPill`s; each action just stores the next immutable `Window` (no date math in the view).
  `Dashboard` now reads the same atom for its period range and re-renders on change — first proof the
  shared-state plumbing works end to end. Full js/wasm build green.

- Built the keystone **`Widget` shell** in `internal/ui`: the candidate-C bento cell (square outlined
  `.w`, unified `.wh` header = grip · centered title · gear, padded `.wbody`) as one generic
  props-driven component (title, body, grid span, draggable, resizable, `OnGear`). Every widget will
  be `Widget` + content, so the chrome is defined once. Grid placement is emitted as inline style per
  axis; the gear is its own component for hook stability in widget lists.

- Browser/DOM check: the gwc dev server is up at :8080 serving the current wasm, but the gwc MCP
  browser-driving tools (`gwc_dom`/`gwc_eval`/`gwc_screenshot`) aren't connected in this headless
  loop context, and the playwright lane isn't set up — so automated DOM assertions aren't available
  here. Staying on compile-green + review as the gate; the owner can eyeball the live server.
- Built the **`FlipPanel`** primitive (`internal/ui`): the candidate-C settings overlay — dimmed/
  blurred backdrop, a card that lifts and 3D-flips to a settings back face (centered title, close,
  scrollable body, dark Save/Cancel footer). Generic over title/body/size/handlers and reused by
  **both** per-widget and global settings (the reusability directive). The open animation runs once
  on mount: a `shown` `UseState` flipped to true inside a `UseEffect` (stable dep + guard against
  re-run), so the CSS transition animates from front→back rather than appearing pre-flipped.

- Added the remaining **control primitives** to `internal/ui`: `Toggle` (`.switch`) + `ToggleRow`
  (labeled `.toggle-row`), and `Swatch` (`.swatch`) + `SwatchPicker` (accent row). Each interactive
  element is its own component (hook stability), and `SwatchPicker` keys each chip by color. These
  complete the shared-control set the settings forms compose.

- Re-sequenced: built a **real bento dashboard** before the settings wiring, so the gear has a live
  widget to open. `Dashboard` now renders the `.bento` grid — a full-width header cell + four KPI
  widgets (Net worth, Income, Spending, Liabilities) composed from the `Widget` shell with
  accounting figures via new `fmtAccounting`/`figTone` helpers (`money.FormatAccounting` +
  `currency`). Income/Spending honor the shared time window; Net worth/Liabilities read
  `ledger.NetWorth`. Aliased `internal/ui` as `uiw` in screens (framework `ui` keeps the bare name).
  `recentTransactions` stays for the next widget (unused package funcs are legal Go; build confirms).

- Added the **Recent transactions** widget (2×2): newest six as a compact table (short "Jan 2" dates,
  payee, accounting amount with green/red tone) in the `Widget` shell. Display-only, so rows build in
  a plain loop (no per-row hooks needed). Reuses the existing `recentTransactions` helper.

- Added the `ProgressBar` primitive (`internal/ui`): a display-only helper (no hooks → plain
  function, not a component) rendering the candidate-C track + fill with a clamped percent and tone
  class. Reused by budgets/goals/savings-rate widgets next.

- Added the **Budgets** widget (1×2): current-month spend per budget via `budgeting.EvaluateAll`,
  each row a label + percent (toned green/amber/red by ok/near/over) over a `ProgressBar`. Kept it
  month-scoped on purpose (budgets are monthly) rather than following the dashboard window, so the
  percentages stay meaningful. Confirmed `appstate.Default` is `*appstate.App` (build passes).

- Added three more bento widgets reusing tested services: **Goals** (first goal's progress via
  `goals.Percent`), **To-do** (up to three open tasks, priority-toned dots), and **Accounts** (up to
  six active balances via `ledger.Balance`, negatives toned). All compose the `Widget` shell +
  `ProgressBar`; confirmed `appstate` exposes `Goals()`/`Tasks()`/`Accounts()`.

- Built chart geometry **bottom-up first**: new pure `internal/chart` package maps a value series to
  SVG coordinates (`Points`, y-inverted with padding; flat/single series centered) and emits
  `LinePath`/`AreaPath` strings with fixed precision for stable, testable output. Table-driven tests
  assert exact path strings. The view (`internal/ui`) will just feed these to an `<svg>`.

- Added `ledger.NetWorthSeries` (pure + tested): net worth as of each cutoff time by counting
  transactions strictly before it and reusing `NetWorth`, so first-of-month cutoffs give an
  end-of-month trend. Test walks a single account across Jan/Feb with a deposit and a withdrawal.

**Next:** the `AreaChart` ui helper (feeds `chart` paths to an `<svg>`) + Net-worth-trend widget, the
Cash-flow widget (div bars from `PeriodTotals`), then per-widget/global settings wiring.

## 2026-06-15 — Dashboard design direction chosen (candidate C)

- Paused screen porting to explore the dashboard visual design with the owner. Built 5 static
  HTML+Tailwind candidates in `design/` (served at `/design/candidate-*.html`); iterated heavily.
- **Selected candidate C**: flat neutral-dark palette, Fraunces serif headings + accounting figures
  (negatives in parentheses), a **bento grid** with one base cell unit and integer-scaling widgets,
  unified per-widget header (grip · title · gear), drag-to-reorder + edge resize handles, a
  gear→center+flip per-widget settings panel, a collapsible icon-only sidebar with a "My pages"
  (custom pages) section, a top-bar time-resolution control (Week/Month/Quarter + From/To), and a
  large global-settings flip panel off the household card.
- Decomposed the mockup into a granular component backlog in `TODOS.md` §1.7c, each item referencing
  `design/candidate-c.html`. Drag/resize/flip will need pointer/DnD via `syscall/js`/`interop`;
  computation stays in the tested logic packages and layout/settings persist to the store.

**Next:** resume porting — apply the candidate-C shell (sidebar + top bar + bento) and start with the
design tokens + app shell, then the widget shell and first widgets, per §1.7c.

## 2026-06-15 — Phase 1 begins: data model (money)

- Started executing the backlog at §1.1, SDLC bottom-up. First service: `internal/money` — a
  precise `Money{Amount int64, Currency string}` type (integer minor units, never float), with
  currency-checked `Add`/`Sub`/`Cmp`/`Neg`/`Abs`/`Sum`. Pure Go, no `syscall/js`; table-driven
  tests pass on native Go (`go test ./internal/money`).
- Renamed the master backlog to `TODOS.md` (project-wide tracking list).

- Added `internal/currency`: registry + manual `Rates` table + `Convert`/`ToBase` (cross-currency
  via base, mixed decimals, nearest-minor rounding). A rounding test surfaced a good lesson —
  `1.005` as float64 is `1.00499…`, so exact half-cents aren't representable; tests now use
  float-stable rounding cases and the conversion rounds to the nearest minor unit.
- Expanded `TODOS.md` to a granular per-entity/service/screen backlog (full spec coverage).

- Added `internal/id`: 128-bit hex IDs via crypto/rand, optional prefix, seedable source for
  deterministic tests. (Test helper lesson: a single-byte counter wraps at 256 and collides — the
  uniqueness test now uses real crypto/rand.)
- Running as a self-paced `/loop`: one feature per iteration, granular commit + CHANGELOG each, with
  a ~1-minute cooldown between features.

- Added `internal/dateutil`: canonical date parse/format, month/week/fiscal-month ranges,
  half-open `InRange`, and DST-safe `DaysBetween` (computed via UTC calendar dates).

- Added `internal/domain`: all core entity types with custom-field maps and JSON tags, plus
  validated enums (`Valid()`/`String()`/`All*`), `AccountType.Class()`/`IsLiability()`, and
  `Transaction.IsTransfer/IsIncome/IsExpense`. Scope uses individual|shared (shared == group-level,
  owner `GroupOwnerID`). Tests cover enum validity, class mapping, and transaction classification.

- Added `internal/ledger`: `Balance`, `RunningBalances`, `PeriodTotals` (income/expense, transfers
  excluded, base-converted), `NetWorth` (assets − liabilities, liabilities reported positive), and
  `NetByOwner` rollups. All cross-currency math routes through the `currency.Rates` base. Tests cover
  mixed currencies, transfers, archived accounts, and currency-mismatch errors.

- Added `internal/budgeting`: scope-aware `Spent` (individual budgets count only the owner member's
  expenses; shared/group budgets count everyone), `Evaluate`/`EvaluateAll` returning remaining,
  percent, and ok/near/over `State` (default near threshold 80%). Handles multi-currency and
  zero-limit edge cases. Tests cover scope, currency conversion, and all three states.

- Added `internal/goals`: `Remaining` (never negative), `Percent` (0..100 clamped), `IsComplete`,
  and `Project` (ceil-months estimate from an assumed monthly contribution; already-complete goals
  project to `from`; non-positive contribution yields no projection) via `Evaluate`. Tested.

- Added `internal/freshness`: default per-type staleness windows (debt-like balances 14d, checking
  30d, savings 45d, investment 60d), `Merge` for settings overrides, `IsStale` (archived/exempt/
  untracked never stale; never-confirmed = stale), `DaysSinceUpdate`, `StaleAccounts`. Recurring
  fixed bills are exempt by design (modeled as Recurring, not accounts; window 0 also exempts). Tested.

- Added `internal/validate`: `Validate{Member,Account,Category,Transaction,Budget,Goal,Task}`
  returning `Issues` (all problems at once, form-friendly). Covers required fields, enum validity,
  positive limits/targets, currency consistency, account class/type match, score/due-day ranges, and
  related-ref requirements. Tested. **§1.3 pure-logic services layer is complete** — 10 packages,
  all green on native `go test`.

## 2026-06-15 — Persistence: pure-Go SQLite (corrected course)

- Built the pure store core first: `store.Dataset` aggregate + `Settings` + schema-versioned JSON
  `Export`/`Import` with a lossless round-trip test.
- **I was wrong, and the owner was right.** I claimed pure-Go SQLite can't run in a browser tab. It
  can: `github.com/ncruces/go-sqlite3` (no cgo, SQLite via wazero) **compiles for `GOOS=js
  GOARCH=wasm`** and the full app wasm still builds. Lesson: test the claim, don't assume.
- Switched persistence from IndexedDB to an in-memory SQLite store (`store.SQLiteStore`): schema +
  `Load`/`Snapshot` for clean dataset ingress/egress. Native tests pass; the JSON Dataset stays the
  portable import/export + sync format. Single pinned connection so `:memory:` is shared.
- Clean architecture paid off: switching the storage engine touched zero logic packages.

- Added per-entity CRUD (`Put/Get/Delete/List`) and query helpers on the SQLite store. Equality
  filters use SQLite `json_extract` (confirmed working with ncruces); date-range filters in Go via
  `dateutil.InRange`. `Put` upserts via `ON CONFLICT`. Tests cover CRUD, missing-key, upsert, and
  all queries.

- Added `internal/money` `FormatMinor`/`ParseMinor` (plain decimal ↔ minor units, strict, validated,
  round-trip tested). Kept it currency-agnostic (takes `decimals`, not a currency) to avoid an import
  cycle with `internal/currency`. This unblocks human-readable CSV. Symbol/grouping is a UI concern.

- Added `internal/store` CSV: `TransactionsToCSV`/`TransactionsFromCSV`. Decimal amounts via
  `money.FormatMinor`/`ParseMinor` + `currency.Decimals`; import matches columns by header name
  (order-independent, extra columns ignored), generates ids when missing, reports errors per line.
  Round-trip is `reflect.DeepEqual`-stable.

- Added `Get/PutSettings`, atomic `Wipe`, and `SampleDataset` (a valid starter seed — checked by
  running `internal/validate` over every entity in tests). **§1.4 persistence is complete.**

**§1.3 + §1.4 done — 11 packages green** (money, currency, id, dateutil, domain, ledger, budgeting,
goals, freshness, validate, store).

- Added `internal/logging` (§1.5): a `log/slog` `Handler` (writes lines to an `io.Writer` + records
  into a bounded `Ring`), with level filtering and `With`/`WithGroup`. Kept pure — the wasm app will
  pass a console-backed writer. Ring eviction, attr capture, grouping, and filtering are tested.

- Added `internal/appstate` — the UI↔logic seam. Kept it **pure Go** (no syscall/js): it owns the
  in-memory SQLite store + slog logger, exposes typed read accessors and validated write-through
  (`Put*` run `internal/validate` first), and does JSON export/import. `Init` seeds sample data and
  sets a package `Default` the screens will read. Wired into `app.Run`; the wasm app still builds and
  appstate is native-tested. Logging goes to `os.Stderr`, which Go's wasm runtime routes to the
  browser console — so no platform code needed.

- Converted the **Accounts** screen from a stub to real data: reads `appstate.Default`, computes
  per-account balances and net worth via `internal/ledger`, groups assets vs liabilities, and shows
  a summary. Added shared display helpers (`fmtMoney`/`amountClass`/`humanizeType`). First visible
  end-to-end feature on the live view. (Read-only for now; add/edit + reactivity next.)
- **Note:** embedding SQLite (ncruces wasm) pushed the raw wasm to ~20 MB (was ~6.5 MB). It
  compresses well (gzip/brotli) but is a real first-load cost — track it; consider lazy-loading or
  the tinygo path later if needed.

- Wired the **Dashboard** to real data: net worth, this-month income/expense (`ledger.PeriodTotals`
  over `dateutil.MonthRange(time.Now())`), active-account count, and a sorted recent-activity list.
  `time.Now()` works at runtime in wasm. Two real screens now read the live store.

- Built the **Accounts add form** — the first mutating feature. Reactivity pattern that works with
  this framework: a screen-level `state.UseAtom("rev:accounts", 0)` subscribes the component; after a
  successful `appstate.PutAccount` the handler bumps the atom, re-rendering the screen against fresh
  store data. Form hooks (`UseState`/`UseEvent`) sit at stable top-level positions; option lists are
  built in plain loops (no `On*` there). Added the missing row/form/amount CSS to the host page.

- Added per-row account **delete**: converted `accountRow` (plain func) into an `AccountRow`
  component so its delete-handler `On*` hook is stable, and switched the lists to `MapKeyed` (keyed
  by account id). The parent passes a `func(string)` delete callback that calls
  `appstate.DeleteAccount` and bumps the revision. This is the canonical per-row pattern for the
  whole app. (Note: deleting an account currently leaves its transactions; cascade/cleanup later.)

- Built the **Transactions** screen: add form (description, amount, income/expense, account,
  category, date) where the amount's currency follows the chosen account and expenses are stored
  negative; newest-first list; per-row delete via `TransactionRow`. Member is inferred from the
  account's owner for individual accounts. Same reactive-revision pattern.

- Built the **Budgets** screen on `internal/budgeting`: current-month spend vs limit per budget with
  a colored ok/near/over progress bar (`Attr("style", "width:N%")` for the fill), plus add and
  delete. Limit is recovered for display as `Spent + Remaining` (both base currency).

- Built the **Goals** screen on `internal/goals`: progress bar (% + remaining), optional target date,
  add and delete. Projection is deferred until we capture an assumed monthly contribution. Reused the
  budget/bar CSS. (Imported the goals package as `goalsvc` to avoid shadowing the local `goals` var.)

**Next:** the **To-do** screen (task list + add + complete + delete), then transfers, then the
remaining Phase-1 screens (Members, Categories, Settings with import/export + load-sample/wipe).

## 2026-06-15 — Project kickoff & spec

- **Toolchain (fresh Windows machine):** installed GitHub CLI, portable Git, and Go 1.26.4 into
  `%LOCALAPPDATA%\Programs` and added them to the user PATH (no admin; MSI installs were blocked).
- **Repo:** created `CashFlux`, initialized git on `main`. Name chosen with the owner.
- **Framework study:** analyzed the local `GoWebComponents` checkout — confirmed the public API
  (shorthand element + control-flow funcs, `ui` hooks, `state` atoms, history `router`), the
  module wiring needed for a standalone app (local `replace` + mirrored `agenthub`/GoGRPCBridge
  replaces), and a key gotcha: `On*` prop options register hooks on wasm, so per-row handlers must
  live in their own row components.
- **Spec:** iterated with the owner and locked Phase 1. Highlights: local-first, household/group
  aware (members, individual pools, group budgets), full asset+liability accounts (incl. informal
  "loan shark" debts), multi-currency with a manual FX table, freshness nudges, custom fields +
  formula builder, planning + to-do, OpenAI client-side (BYO key) for document parsing/insights,
  and a capital-allocation suggestion engine.
- **Standards:** wrote `CLAUDE.md` — pure idiomatic Go, clean architecture (logic packages with no
  `syscall/js`, unit-tested on native Go), `log/slog` logging, readable plain-English UI,
  import/export, heavy configurability, and strict VCS/journaling (one feature per commit).

- **Dependency cleanup:** replaced the local `../GoWebComponents` `replace` with a real `go get`
  module pin (pseudo-version `v1.1.1-0.20260613162601-cad8af8`). `go mod tidy` + wasm build are
  clean — `agenthub` is pruned (core packages don't import it); only `cbor`/`float16`/`goldmark`
  come along indirect. Phase 0 wasm entrypoint builds (6.17 MB).
- **Tooling:** built `gwc` from the framework checkout and wired it as `.tools/gwc.exe` + the `gwc`
  MCP server (`.mcp.json`, 81 `gwc_*` tools). Wrote `docs/GOWEBCOMPONENTS.md` and a CLAUDE.md
  quick-reference for new sessions. Moved pre-spec draft files to `_scratch/` (Go-ignored).

- **Skeleton:** built the routed app shell (`internal/app`: router + `Shell` + `NavBar`) and stub
  screens for all 12 features (`internal/screens`), driven by a single screen registry. Verified on
  the live `gwc dev` server (HTTP 200 for `/`, wasm, and glue; hot reload active).
- **Layout cleanup:** moved web/build assets under `web/` so the project root holds only Go source,
  config, and docs — clean and standard.
- **Framework bug found (parked):** `gwc dev` resolves `-html` relative to the build/module root,
  not the serve `-root` (contradicts its flag help). Workaround: pass `-html web\index.html`. Proper
  fix is in GoWebComponents `tools/gwc/dev.go` — to be done, then rebuild + recopy `gwc`.
- **Planning:** wrote `TODO.md`, the priority-ordered master backlog, and made bottom-up SDLC
  (model → services → store → UI) an explicit rule in `CLAUDE.md`.

**Next (per SDLC + TODO §1.1):** start the data model — `internal/domain` types + `internal/money`
and `internal/currency` services with table-driven tests — before any feature UI.

**Note:** a few pre-spec exploratory Go files (model/persist/dashboard/transactions/components)
remain in the tree from early prototyping; they predate the locked spec and will be replaced to
match it during Phase 1.

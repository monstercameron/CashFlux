# Changelog

All notable changes to CashFlux are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/). Policy: **one feature per commit**,
and every commit updates this file under `Unreleased`.

## [Unreleased]

### Changed
- The sidebar now hides screens switched off via module visibility: the primary nav (Accounts,
  Transactions, Budgets, Goals, To-do) and the System items (Members, Categories) are filtered by the
  hidden-modules set, while locked screens (Dashboard, Settings) always stay. The Sidebar subscribes
  to the atom, so toggles take effect immediately.
- User-facing dates in the Transactions, Goals, and To-do lists now render using the chosen date
  format (via `prefs.FormatDate`) instead of always ISO. Each row component reads the preferences
  atom, so changing the format updates every list live.

### Added
- Account "locked until" date (assets): set on the add form, and the Allocate screen excludes an
  account locked until a future date from its suggestions (you can't add money to it yet).

### Changed
- The dashboard spending breakdown now rolls sub-category spend up to its top-level parent category,
  so e.g. Restaurants and Groceries are counted under Food — a cleaner high-level view.

### Added
- Re-parent categories: the category inline editor now has a parent picker too (same-kind, self
  excluded), so an existing category can be nested, moved, or promoted to top level.
- Sub-categories in the Categories screen: the add form has a parent picker (categories of the same
  kind, indented), and the category lists now display the parent/child hierarchy indented (via
  `categorytree.Flatten`). Lets you nest e.g. Restaurants and Groceries under Food.
- Category hierarchy engine (`internal/categorytree`): `Build` organizes a flat category list into a
  parent/child forest (siblings sorted by name) and `Flatten` returns a depth-tagged list for
  indented display, using the existing `Category.ParentID`. Defensive — orphans become roots and
  cycles are dropped rather than looping. Table-tested. Foundation for sub-categories.
- Cleared balance on accounts: a pure `ledger.ClearedBalance` (opening balance + only cleared
  transactions) is shown on each account row when it differs from the live balance — the figure to
  reconcile against a statement. Tested.
- Bulk mark transactions cleared/uncleared: the selection bar now has "Mark cleared" and "Mark
  uncleared" actions, so you can reconcile many at once.
- Filter transactions by cleared status (cleared / not cleared / both), persisted with the rest of
  the transaction filter — pairs with the cleared toggle to make reconciling against a statement
  easy (show only what's not yet cleared).
- Mark transactions cleared/reconciled: each transaction row has a toggle that flips the (now
  surfaced) `Cleared` flag, with the status shown in the row meta — useful for reconciling against a
  statement.
- Edit tasks inline: each to-do row has an Edit button to change the title, priority, due date, and
  notes. Every entity — including to-do — now supports inline edit.
- Edit categories inline: each category row has an Edit button to rename it and switch its kind
  (expense/income). With this, every entity supports inline edit.
- Edit members inline: each member row has an Edit button to change the name and color (members
  previously supported only add, delete, and set-default).
- Export filtered transactions to CSV: an "Export CSV" button on the transaction list downloads
  exactly the currently filtered and sorted set (shared `applyTxFilter` ensures the export matches
  the view), complementing the export-all in Settings.
- Change an account's owner: the account inline-edit form now has an owner picker (group or a
  member), updating ownership and scope. Ownership is now editable inline on accounts, budgets, and
  goals.
- Change a goal's owner: the goal inline-edit form now has an owner picker (group or a member),
  updating ownership and scope.
- Change a budget's owner: the budget inline-edit form now has an owner picker (group or a member),
  updating ownership and scope — so reassigning no longer requires deleting and recreating.
- Budget period selector in the UI: the add and inline-edit forms let you choose weekly, monthly, or
  quarterly, and each budget is now evaluated over its own period window (honoring the week-start
  preference) rather than a shared month. Each row shows its period.
- Budget periods (weekly / monthly / quarterly): the `Period` enum gained weekly and quarterly (with
  a `Label`), and `budgeting.PeriodRange` computes the current [start, end) window for a period
  containing a reference date (weekly honors the week-start preference; quarterly snaps to calendar
  quarters). Pure, table-tested. Foundation for per-budget periods in the UI.
- Link a goal to an account: the goal add and edit forms now have an optional "linked account"
  picker (populating `Goal.AccountID`), and the goal row shows "linked to <account>". Records which
  account a goal is funded from.
- Goal pace guidance: goals with a target date now show how much to save per month to hit it (via a
  new pure `goals.MonthlyNeeded` — remaining ÷ whole months left, rounded up). Shown only for
  incomplete goals with a future date.
- Edit accounts inline: each account row has an Edit button that swaps in a form for the name,
  opening balance, and the type-specific attributes (liabilities: credit limit, APR, minimum
  payment, due day, lender; assets: expected return, liquidity, stability), saving through the
  validated path. Completes inline edit across every main entity.
- Bulk transaction actions: each transaction row has a select toggle, and when any are selected a
  bar shows the count with a category picker + "Apply category" (recategorizes the selected
  non-transfer rows), "Delete selected" (transfer-aware, removing paired legs), and "Clear
  selection". Lets you reclassify or clean up many entries at once.
- Duplicate detection on document import: rows already present in the chosen account (same date and
  amount) are skipped, and the result reports how many duplicates were left out — so re-reading the
  same receipt won't double-enter transactions. Backed by `extract.Row.Signature`/`FilterNew` (tested).
- Edit rows in the document review list before importing: each extracted transaction has an Edit
  button to fix its date, description, amount, or category (e.g. correct a misread) prior to import.
- Remove rows from the document review list before importing: each extracted transaction has a ✕ to
  drop a misread, so only the rows you keep are imported.
- Document image import on the Documents screen: choose a receipt or statement image, "Read with
  AI" sends it to the OpenAI vision model (bring-your-own-key, client-side), and the extracted
  transactions appear in a review list — pick an account and import them through the validated path
  (categories matched by name, dates falling back to today). Ties together `ai.BuildVisionRequest`,
  `ai.SendVisionChat`, and `extract.ParseRows`. The CSV paste-import remains.
- Extraction parser (`internal/extract`): `ParseRows` turns an AI vision reply into reviewable
  `Row{Date, Description, Amount, Category}` values, tolerant of a bare array or an object wrapper
  (transactions/rows/items/data), numeric or string amounts, varied field names (merchant/payee/…),
  and a Markdown code fence; empties are skipped. Pure, table-tested. Bridges vision output to the
  import flow.
- Vision chat transport (`internal/ai`): `SendVisionChat` posts a multimodal request (system prompt
  + user text + one image) to OpenAI client-side with the user's key, same async one-callback
  contract as `SendChat`. The fetch promise chain is now shared via an internal `postCompletions`.
- Vision request codec (`internal/ai`): `BuildVisionRequest` marshals a multimodal OpenAI chat
  request — a system prompt plus a user message carrying text and an image (data/URL) part — for
  reading receipts and statements. The reply is plain text, read with the existing `ParseResponse`.
  Pure, table-tested. Foundation for document image import.
- Edit transactions inline: income and expense rows get an Edit button that swaps in a form for the
  description, amount, category, and date, saving through the validated path (the original
  income/expense sign and account are preserved). Transfers remain non-editable inline.
- Edit goals inline: each goal row has an Edit button that swaps in a form for the name, target
  amount, and target date (clearable), saving through the validated path. Complements the existing
  Contribute action.
- Edit budgets inline: each budget row has an Edit button that swaps in a form for the name and
  monthly limit, saving through the validated path. (Previously budgets could only be added or
  deleted.)
- Member delete now offers reassignment: deleting a member who owns accounts, budgets, or goals
  opens a panel to move everything to another owner (or the shared group) and then deletes them,
  instead of just refusing. Scope follows the new owner and transactions attributed to the member
  are re-attributed. Backed by `appstate.ReassignOwner` (tested).
- Category delete now offers reassignment: deleting a category that's still used by transactions or
  budgets opens a panel to move those records to another category and then deletes it, instead of
  just refusing. Backed by `appstate.ReassignCategory` (tested). Unused categories still delete
  immediately.
- Freshness reminders editor in the global Settings panel: per-account-type day inputs (credit
  cards, checking, savings, investments, loans, cash) that write `Settings.FreshnessOverrides`; 0
  means never flag that type. Changes apply immediately to the stale badges and dashboard widget.
- Freshness window overrides now take effect: `appstate.FreshnessWindows` layers the household's
  per-account-type overrides (from Settings) over the built-in defaults, and the Accounts stale
  badges and the dashboard Freshness widget both use it. Previously the stored overrides field was
  unused; overrides set via imported data now change which balances are flagged stale.
- The transaction list now remembers your filter and sort across reloads: the search text, account,
  category, member, date range, and sort are held in a single `uistate.UseTxFilter` atom persisted
  to localStorage. Clearing resets it. (Previously the filter reset on every reload.)
- Amount-split in the Allocate screen: enter an amount to allocate and an optional emergency buffer
  to keep back, and each ranked destination shows its suggested dollar amount (via `Distribute`),
  with a "Kept back" note for the buffer/leftover. Updates live as the amount, buffer, profile, or
  exclusions change.
- Allocation amount split (`internal/allocate`): `Distribute(ranked, total, SplitOptions)` spreads a
  total across ranked destinations in proportion to their scores, holding back an emergency-buffer
  `Reserve` and capping each at `MaxPer`, and returns per-destination `Plan`s plus the unallocated
  remainder. Even split when scores are absent. Table-tested (proportional, reserve, cap, edge
  cases). Turns the ranking into concrete dollar amounts.
- Exclude destinations in the Allocate screen: each ranked suggestion has an "Exclude" button that
  drops it from the ranking (via the new `RankWith` constraint), and an "Excluded" section lists the
  left-out destinations with "Restore". Updates live as you toggle.
- Allocation exclusion constraint (`internal/allocate`): a `Constraints` struct (currently an
  `Exclude` set of candidate IDs) with `Eligible`, plus `RankWith(candidates, weights, constraints)`
  that filters ineligible candidates before ranking. Zero-value constraints make it identical to
  `Rank`. Table-tested. Lets the user leave specific destinations out of the recommendation.
- Per-row "Duplicate" action on transactions: copies a transaction to today with a fresh id (tags
  and custom fields included), saving it through the validated path. Offered for income/expense rows
  only — a duplicate drops any transfer link, so it becomes a standalone entry rather than a broken
  transfer leg.
- Light theme: a `[data-theme="light"]` stylesheet that overrides the legacy palette variables, the
  shell's Tailwind utility colors (base/tile/hover/fg/dim/faint/line, the active-nav surface), and
  the widgets' hardcoded surface hexes (bento tiles, segmented/stepper pills, flip panel, settings
  inputs, switches, scrollbars). Choosing Light (or System on a light OS) in Settings now actually
  lightens the whole app, while the user accent stays applied on top. Completes the theme preference.
- Appearance preferences now apply to the page: `uistate.ApplyPrefs` writes `data-theme`
  (resolving "system" to the OS color scheme), `data-density`, and the `--accent` CSS variable onto
  the document root — applied on boot (before first paint) and on every change. The accent color
  retints buttons, bars, focus rings, and active states immediately; a new `[data-density="compact"]`
  stylesheet rule tightens cards, rows, and fields. (A full light-theme skin is still to come; the
  `data-theme` attribute is in place for it to hook.)

### Changed
- The Settings appearance controls (theme, accent, density) are now backed by the persistent
  preferences atom instead of throwaway local state, so the selections are remembered and saved to
  localStorage on change (they no longer reset when the panel closes).

### Added
- Theme, accent, and density added to the display-preferences engine (`internal/prefs`): `Theme`
  (dark/light/system), `Accent` (validated hex color), and `Compact` fields, with `Normalize`
  defaulting unknown themes to dark and invalid accents to the candidate-C green, plus an
  `isHexColor` check. Table-tested. Prepares the appearance controls to become real and persistent.
- Screen show/hide toggles in the global Settings panel: a "Screens" section with a Show toggle per
  hideable screen (Accounts, Transactions, Budgets, Goals, To-do, Members, Categories). Flipping a
  toggle updates the hidden-modules atom and persists to localStorage, hiding or restoring the
  screen in the sidebar immediately. Dashboard and Settings are omitted (locked visible). This
  completes module visibility end-to-end.
- localStorage-backed hidden-modules atom (`uistate.UseHiddenModules`/`PersistHiddenModules`): seeds
  the hidden-screen set from localStorage on boot and writes it back on change, so show/hide choices
  survive reloads. Loads are normalized (false/locked/stale entries dropped).
- Module-visibility engine (`internal/modules`): a `Hidden` set of hidden screen paths with
  `IsHidden`/`Toggle`/`Normalize`, plus locked core screens (home and settings) that can never be
  hidden. Toggle is immutable (returns a new minimal set) and a no-op for locked paths. Table-tested.
  Foundation for show/hide-screen settings.
- Preferences section in the global Settings panel: choose the week start (Sunday/Monday segmented
  control) and the date format (ISO / US / European / long), each showing a live example. Changes
  write the `UsePrefs` atom and persist to localStorage immediately, so they survive reloads.
- localStorage-backed preferences atom (`uistate.UsePrefs`/`PersistPrefs`): seeds the display
  preferences from localStorage on boot and writes them back on change, so week-start and date-format
  choices survive reloads — the same durable channel the dashboard layout uses (the dataset is
  re-seeded each boot). Loads are always normalized.
- Display-preferences engine (`internal/prefs`): a pure `Prefs` type (week start + date style) with
  `FormatDate` (ISO/US/EU/long), `WeekStartWeekday`, `WeekStartOf` (start-of-week honoring the
  configured first day), and `Normalize` (fills blank/unknown fields with defaults for forward
  compatibility). Table-tested. Foundation for reload-persistent user preferences.
- Custom fields on the Goals, Budgets, and Members forms — completing the rollout across all five
  entity types. Each add-form renders its registered custom fields via `CustomFieldInput`, types the
  values into the entity's `custom{}` map on save, and validates them through the matching appstate
  write path (`PutGoal`/`PutBudget`/`PutMember` now call `validateCustom`). Closes §1.16 form
  rendering: custom fields are available everywhere they're defined.
- Custom fields on the Transactions form: custom-field definitions registered for transactions now
  render in the add-transaction form (reusing `CustomFieldInput`) for income and expense entries,
  with values typed into the transaction's `custom{}` map on save and validated by
  `appstate.PutTransaction`. Transfer legs skip custom fields (an empty def list renders nothing).
- Custom fields on the Accounts form: any custom-field definitions registered for accounts now
  render as the right input (text/number/date box, yes-no or choice dropdown) in the add-account
  form via a reusable `CustomFieldInput` component (own event hook, safe in keyed lists). Values are
  typed into the account's `custom{}` map on save, and `appstate.PutAccount` validates them against
  the definitions (rejecting missing-required and wrong-typed values). Tested.
- Custom-field management UI on the Customize screen (`CustomFieldsManager`): add a field by picking
  the entity type (accounts/transactions/budgets/goals/members), a key and label, a data type
  (text/number/date/yes-no/choice), comma-separated options for choice fields, and optional vs
  required; saved through the validated `appstate.PutCustomFieldDef` path. Existing definitions list
  grouped by entity with type/required/options shown and per-row delete (own-component delete hook,
  honouring the loop-hook rule). Fulfils the Customize screen's "custom fields and formulas" promise.
- Persist custom-field definitions: `customfields.Def` now carries JSON tags and a `Validate`
  method (sound definition needs id/entity-type/key/label/known-type; choice fields need options);
  the store gains a `customfielddefs` table with full CRUD, a `CustomFieldDefsByEntity` query, and
  dataset Load/Snapshot + export/import round-trip; `appstate` exposes `CustomFieldDefs`,
  `CustomFieldDefsFor`, validated `PutCustomFieldDef`, and `DeleteCustomFieldDef`. Wipe clears the
  new table. Tested (store CRUD, dataset + export/import round-trip, wipe, Def validation).
- Custom-field definitions and validation engine (`internal/customfields`): typed `Def`
  (text/number/date/bool/select, required, select options) plus `Validate`, which checks an
  entity's `custom{}` value map against its definitions — flagging missing required fields, type
  mismatches, invalid dates, and out-of-list select values in plain English, while ignoring unknown
  keys so old data stays forward-compatible. Pure, table-tested. (Foundation for SPEC §1.16.)
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
- `.gitattributes`: normalize text to LF in the repo and on checkout (ends the Windows CRLF/LF
  warnings) and mark `*.wasm` and common asset types as binary.
- GitHub Actions CI (`.github/workflows/ci.yml`): on push/PR, runs `go vet`, `go test ./...` (the
  pure logic packages; js/wasm view packages are build-tagged out of native), and a `js/wasm` build.
- Project `README.md`: overview, feature highlights, stack, build/run, architecture, and doc links.

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
- `internal/store`: `Get/PutSettings` accessors, atomic `Wipe`, and a valid `SampleDataset` starter
  seed (validated in tests). Completes the Phase 1 persistence layer.
- `internal/logging`: `log/slog`-based `Handler` writing human-readable lines to any `io.Writer`
  plus a bounded, concurrency-safe `Ring` buffer for an in-app log viewer; supports level filtering
  and `With`/`WithGroup` contextual attrs + tests.
- `internal/appstate`: the UI↔persistence/logic seam — owns the in-memory store + slog logger, with
  typed read accessors, validated write-through (`Put*`/`Delete*`), JSON export/import, and
  `Init`/`Default`; wired into `app.Run` to seed sample data on boot. Pure Go + native tests.
- Accounts screen: first real, data-backed screen — assets/liabilities grouped with live per-account
  balances (`internal/ledger`) and a net-worth/assets/liabilities summary, reading from `appstate`.
  Shared money display helpers (`fmtMoney`, amount classes).
- Dashboard screen: real headline metrics — net worth, this-month income/expense (via
  `ledger.PeriodTotals` over the current month), active-account count, and a recent-activity list.
- Accounts add form: create an account (name, type, owner, currency, opening balance) with
  validated write-through and a reactive refresh (`state.UseAtom` revision bump). First mutating
  feature. Added row/form/amount styles to the host page.
- Accounts per-row delete via an `AccountRow` component (stable `On*` hook) with `MapKeyed` keyed
  rendering; deleting refreshes the screen and net-worth summary.
- Transactions screen: add income/expense (account-aware currency, category/date), newest-first
  list, and per-row delete (`TransactionRow`), all with validated write-through and reactive refresh.
- Budgets screen: current-month spend vs limit per budget via `internal/budgeting` with a colored
  ok/near/over progress bar, plus add and per-row delete.
- Goals screen: savings goals with a progress bar (% complete + remaining via `internal/goals`),
  optional target date, plus add and per-row delete.
- To-do screen: tasks with priority badges and due dates — add, complete-toggle, delete, sorted
  (open first, then by due date).
- Dashboard design exploration: five HTML/Tailwind candidates in `design/`; **candidate C selected**
  (flat neutral-dark, Fraunces serif headings + accounting figures, bento grid, per-widget
  grip/title/gear header, drag-reorder + edge resize, gear→flip settings, collapsible icon sidebar,
  global-settings flip).
- Granular, reusability-focused component backlog for the candidate-C dashboard UI (`TODOS.md` §1.7c),
  every item referencing `design/candidate-c.html`.
- Dashboard design-system foundation in the host page: Fraunces + Inter web fonts, the candidate-C
  Tailwind palette/type config, and the full candidate-C component CSS (bento grid, unified widget
  header, drag/resize handles, flip-settings panel, dark scroll pane, sidebar collapse, control
  primitives) — ported verbatim from `design/candidate-c.html`, ready for the Go component port.
- `internal/money`: `Group` (thousands separators) and `FormatAccounting` — accounting-style display
  (`$1,234.56`, negatives in parentheses like `($240.55)`, always `decimals` places, caller-supplied
  symbol) for the candidate-C figure style; table-driven tests. Pure, no currency-registry dependency.
- `internal/ui`: new shared design-system package (Go port of `design/candidate-c.html`) with a
  reusable, props-driven `Icon` primitive — the candidate-C stroked SVG icon set (dashboard, accounts,
  transactions, budgets, goals, to-do, settings, page, plus, menu) that inherits color/size from the caller.

- PWA web manifest (`manifest.webmanifest`) + theme-color/apple meta tags, making CashFlux installable
  as a standalone dark-themed app (Phase 3 start; icons and a service worker follow).
- PWA service worker (`sw.js`, registered on load): network-first caching of same-origin GETs (core
  shell pre-cached on install) so the app stays fresh online and loads offline; cross-origin calls
  (e.g. OpenAI) pass through uncached.
- PWA install prompt: an "Install CashFlux" button appears when the browser offers installation
  (`beforeinstallprompt`) and hides after install.

### Changed
- Retargeted the legacy screen palette (the shared CSS variables) to candidate-C values, so the
  non-dashboard screens (Accounts, Transactions, Budgets, Goals, To-do) — cards, stats, rows, forms,
  bars — match the new flat neutral-dark bento shell, with squared (4px) corners.
- App shell replaced the top-navigation chrome with the candidate-C layout: a fixed left rail
  (brand + icon-led primary navigation with active highlighting and router navigation) and an
  independently scrolling main pane with a sticky top bar (menu toggle, page title, Add action).
  `internal/app` now composes `internal/ui` primitives (`Icon`); screen bodies render inside the
  new `main` scroll pane.
- Rail completed with the candidate-C lower groups: a **My pages** section (example custom pages with
  colored page icons + a muted "New page" action), a **System** group (Settings), and a
  household card pinned to the bottom that shows live member count and base currency and opens
  Settings. `navItem` is now reusable across all groups (optional path for placeholders, custom icon
  class, muted styling).
- Collapsible rail: the top-bar menu button toggles the sidebar into 58px icon-only mode (labels,
  captions, brand text, and household summary hidden), coordinated by a shared `rail:collapsed`
  state atom so the button and rail stay in sync.
- `internal/ui`: reusable `Segmented` (mutually-exclusive option toggle) and `StepperPill`
  (label + prev/next chevrons) control primitives — generic and props-driven, each interactive
  child its own component so click hooks stay stable in lists.
- `internal/period`: pure time-resolution model for the dashboard control — `Resolution`
  (week/month/quarter) with anchor `Truncate`/`Step`/`Label` and `Range` (from/to anchors → a
  half-open reporting range, clamped). Table-driven tests cover quarter boundaries, week starts,
  cross-year stepping, and range spanning. Pure, native-tested.
- `internal/period`: immutable `Window` value (resolution + from/to anchors + week start) with the
  control's stepping rules — `SetResolution` (re-snaps anchors), `StepFrom`/`StepTo` (move one
  anchor, clamping the other so from ≤ to), `Range`, and from/to labels. Drops straight into UI
  state; clamp behavior table-driven tested.
- Time-resolution control in the top bar: a Week/Month/Quarter `Segmented` toggle plus From/To
  `StepperPill`s, backed by a shared `internal/uistate` window atom over `period.Window`. The
  dashboard now derives its income/spending period from this control (re-rendering on change) instead
  of a hardcoded current-month range; stat labels are now period-relative ("Income"/"Spending").
- `internal/ui`: reusable `Widget` shell — the candidate-C bento cell with the unified header (grip ·
  centered title · gear) and a padded body, props-driven (title, body, grid span, draggable,
  resizable, gear handler) so every dashboard widget is `Widget` + content. Optional edge resize
  handles; gear is its own component for stable hooks in lists.
- `internal/ui`: reusable `FlipPanel` settings overlay — the candidate-C dimmed/blurred backdrop with
  a card that lifts and 3D-flips to a settings back face (centered title, close button, scrollable
  body, dark Save/Cancel footer). Generic (title, body, size, Save/close handlers) and reused by both
  per-widget and global settings; the open animation runs once on mount via `UseState`/`UseEffect`.
- `internal/ui`: reusable `Toggle` (pill switch) + `ToggleRow` (labeled settings row), `Swatch`
  (color chip) + `SwatchPicker` (accent row) control primitives — the building blocks of the settings
  forms, generic and props-driven, each interactive element its own component.
- Dashboard rebuilt as the candidate-C **bento grid**: a full-width header cell plus four KPI widgets
  (Net worth, Income, Spending, Liabilities) built from the live ledger and shown as accounting
  figures (`$1,234.56` / `($240.55)`, green/red tone). Each KPI is the reusable `Widget` shell +
  content; Income/Spending follow the time-resolution window. New `fmtAccounting`/`figTone` helpers.
  The Net worth tile shows a real month-over-month change (▲/▼ %) via `ledger.NetWorthSeries`; the
  Income/Spending tiles show the period plus the deposit/transaction count for it.
- Recent transactions widget (2×2) on the dashboard: newest activity as a compact table with short
  dates and accounting amounts (green/red), in the reusable `Widget` shell.
- `internal/ui`: reusable `ProgressBar` primitive — the candidate-C thin rounded track + colored fill
  (clamped percent, tone class, extra spacing), reused by budgets, goals, and savings-rate widgets.
- Budgets widget (1×2) on the dashboard: current-month spend per budget with an ok/near/over
  `ProgressBar` and percent (green/amber/red), via `internal/budgeting`. Always month-scoped since
  budgets are monthly.
- Goals widget (1×1): the first goal's progress (saved / target + percent and target date) via
  `internal/goals`, in the reusable `Widget` shell with a `ProgressBar`.
- To-do widget (1×1): up to three open tasks, each with a priority-toned dot (high = amber).
- Accounts widget (2×1): a small grid of up to six active account balances (accounting figures,
  negatives toned red) via `ledger.Balance`.
- `internal/chart`: pure SVG path geometry for dashboard sparkline/area charts — `Points` (scale a
  series into a w×h box, y-inverted, padded, flat/single series centered), `LinePath`, and
  `AreaPath` (closed to a baseline). Table-driven tested; no rendering dependency.
- `internal/ledger`: `NetWorthSeries` — net worth as of each cutoff time (transactions strictly
  before the cutoff counted), in base currency, for the net-worth trend chart. Table-driven tested.
- `internal/payoff` (Phase 2 start): pure debt-payoff projection — `Project(balance, aprPercent,
  payment)` simulates monthly APR accrual and a fixed payment, returning months-to-zero, total
  interest, and total paid, with `ok=false` when the payment can't cover the interest. Table-driven
  tested.
- `internal/forecast`: pure balance/net-worth projection over a horizon — `Project(start, recurring,
  oneTimes, months)` applies the recurring monthly net plus any one-time events each month and
  returns the end-of-month balance series; `MonthlyNet` sums the recurring flows. Table-driven tested.
- `internal/ai`: OpenAI chat request/response shapes + a pure codec — `BuildRequest` marshals a
  chat-completions body; `ParseResponse` extracts the assistant content and surfaces API errors /
  empty responses; `ParseUsage` reads token counts. Round-trip tested (no network; the fetch
  transport is a separate js layer).
- `internal/ai`: browser `fetch` transport (`SendChat`) — posts a chat request with the user's key
  asynchronously and calls back with the content or a plain-English error; the only network spot.
- `internal/rules`: pure auto-categorization engine — `Rule{Match, SetCategoryID, SetTags}` with
  case-insensitive substring matching over payee+description, first-match-wins `FirstMatch`,
  `Category`, and `Tags`. Empty matches never fire. Table-driven tested.
- Insights screen (replacing the stub): an **"Explain my month"** AI narrative generated client-side
  from your live figures via OpenAI with your own key; prompts to add a key in Settings when absent,
  with loading and error states. Plus a **natural-language "Ask about your money"** box that answers
  questions using your figures as context.
- Planning screen (replacing the stub): a **debt-payoff calculator** — enter balance, APR, and
  monthly payment to see months-to-zero, total interest, and total paid, updating live via the
  `internal/payoff` engine, with a friendly message when the payment can't cover the interest, and an
  optional **extra-payment** input that shows how many months sooner it clears and how much interest
  it saves. Plus a **12-month net-worth projection** chart (current net worth + this month's net cash
  flow, via `internal/forecast` + the area chart) with a what-if "trim monthly spending by…" input
  that re-projects and reports the improved 12-month figure.
- `internal/allocate`: pure capital-allocation scorer — normalizes each candidate on returns,
  stability, liquidity, and debt-reduction, combines by a user `Weights` profile into an explainable
  `Score` + `Breakdown`, and `Rank`s candidates highest-first. Table-driven tested; deterministic.
- Allocate screen (replacing the stub): builds candidates from asset accounts, high-interest
  liabilities, and **unfinished goals**, ranks them by a chosen profile (Balanced / Maximize returns /
  Safety & access / Pay down debt), and shows each suggestion's score bar and per-criterion breakdown.
  An optional **"Explain with AI"** narrative summarizes why the ranking suits the profile (BYO key).
- `internal/formula`: tokenizer for the sandboxed formula language — numbers (incl. leading-dot),
  identifiers, double-quoted strings, arithmetic/comparison operators, parens, and commas; errors on
  unterminated strings, stray `=`/`!`, and unexpected characters. Table-driven tested.
- `internal/formula`: recursive-descent `Parse` → AST (NumberLit/StringLit/Ident/Unary/Binary/Call)
  with correct precedence (comparison < additive < multiplicative < unary), left-associativity,
  parens, and function calls. Errors on malformed input. Table-driven tested via a canonical s-expr.
- `internal/formula`: allow-list `Eval` (completes the sandboxed engine) — arithmetic, comparisons
  (numeric + string equality), variable resolution from an `Env`, and the functions `sum/avg/min/max/
  count/abs/round/if`. Errors on unknown var/function, arity, division/modulo by zero, and type
  mismatch; no host access. Table-driven tested.
- Customize screen (replacing the stub): a live **formula calculator** — write an expression over your
  figures (net worth, assets, liabilities, income, expense, account/transaction/member counts) and
  see the result instantly via the sandboxed engine, with the available variables and their current
  values listed, plus one-click example chips (savings rate, spending ratio, etc.). Variables now
  include budget/goal/task counts alongside the financial figures.
- `internal/ui`: `AreaChart` helper renders a filled gradient sparkline from a value series (feeding
  the pure `chart` geometry into an `<svg>`). Net worth trend widget (1×2) on the dashboard: the
  current figure over a six-month end-of-month area chart via `ledger.NetWorthSeries`.
- Cash flow widget (2×1): income (green, up) vs expense (red, down) bars for the last four months,
  scaled to the largest bar, with the current month's net figure — via `ledger.PeriodTotals`.
- Savings rate widget (2×1): the share of the period's income that wasn't spent, as a big figure and
  a `ProgressBar` (toned green/red).
- Spending breakdown widget (2×1): a segmented bar of the period's expenses by category (top three
  plus "Other") with a color-keyed legend; totals converted to base currency.
- Upcoming bills widget (2×1): the next due date and minimum payment for each liability account that
  has them, soonest first, with due dates within a week toned amber. Completes the candidate-C widget
  catalog (12 widgets).
- Per-widget settings: each widget's gear opens its settings in the `FlipPanel` (driven by a shared
  `settings:target` atom + a `SettingsHost` mounted at the shell root). The settings back face has an
  editable title and behavior toggles (show on dashboard, allow moving/resizing, compact), built from
  the `ToggleRow` primitive. The household card's global panel opens too (body coming next).
- The rail's household card now opens the global settings flip panel (via the shared settings atom)
  instead of navigating to the Settings route.
- Global settings: the OpenAI API key and model inputs now persist to the store (`Settings.OpenAIKey`/
  `OpenAIModel`), so the Insights screen can use them. The key stays on-device. The "+ Add member"
  button now closes the panel and opens the Members screen.
- Global settings panel body: a two-column flip-panel form with live household member chips, base
  currency, and editable FX rate rows (left) and AI (BYO key toggle + key + model), Appearance (theme
  `Segmented` + accent `SwatchPicker` + compact), and Data action buttons (right). Built from the
  shared control primitives; appearance controls hold local state and data actions are wired next.
- Export JSON data action: downloads the full dataset as `cashflux.json` (the portable export/import
  + sync payload) via `appstate.ExportJSON` and a small Blob/anchor browser-download helper.
- `internal/appstate`: `ExportCSV` (transactions → CSV), `ImportTransactionsCSV` (parse CSV rows →
  validated writes, best-effort), `LoadSample` (replace with the sample dataset), and `Wipe` (clear
  all data) — the data-action seams; tested natively.
- Documents screen (replacing the stub): paste a CSV of transactions and import them (no AI needed) —
  header-name column matching, decimal amounts, negatives for expenses; reports how many imported.
- Global settings Data actions wired: Export CSV (download), Import (file picker → replace dataset),
  Load sample, and Wipe (with a confirm dialog). A shared `data:revision` atom is bumped on bulk
  changes so the dashboard re-renders; added `pickFile`/`confirmAction` browser helpers.
- `internal/dashlayout`: pure bento layout model — `Placement` (column/row + spans with CSS grid
  string helpers), `Layout` with the candidate-C `Default` arrangement, immutable `Swap` (exchange
  two widgets' cells) and `Resize` (clamped spans). Table-driven tested; underpins drag-reorder/resize.
- The `Widget` shell now sources its grid placement from a shared `dashboard:layout` atom (falling
  back to caller defaults), so reorder/resize changes flow to every widget via state.
- Drag-to-reorder: dragging one bento widget onto another swaps their grid cells (`dashlayout.Swap`
  via a shared drag-source atom; `dragover` allows the drop with `Prevent`). The dragged widget dims
  (`.drag`) and the source clears on drag-end.
- Resize handles: a widget's right/bottom edge handles now cycle its column/row span
  (`dashlayout.Resize`, clamped to the 4×3 grid bounds) and re-place it live. Every dashboard widget
  is now both draggable and resizable.
- Bento layout persistence: the arrangement is saved to `localStorage` after every reorder/resize and
  reseeds the layout atom on load, so a customized dashboard survives reloads (falls back to the
  default arrangement when absent or invalid).
- Reset layout action in the dashboard header restores the default bento arrangement and clears the
  saved layout.
- Transactions: account-to-account transfers — a "Transfer" kind swaps the category picker for a
  "To account" picker and creates paired entries (debit + credit, each with `TransferAccountID`) that
  move both balances and are excluded from income/expense. Same-currency only for now; rows labelled
  "Transfer". Deleting either leg removes the reciprocal so balances stay consistent.
- Transactions: a filter bar (description search + account + category + member pickers + a From/To
  date range, with Clear) narrows the ledger list, with a distinct "No matching transactions" state.
- Transactions: a comma-separated tags field on income/expense entries; tags show on the row
  (`#tag`) and the search box matches tags as well as descriptions.
- Transactions: a sort selector (newest first / largest amount / payee A–Z).
- Transactions: auto-suggests a category as you type the description (matching against category names
  via `internal/rules`), without overriding a category you've already chosen.
- Transactions: a "Repeat last" button pre-fills the form from the most recent transaction (kind,
  amount, account, category, transfer destination).
- Goals: a "Contribute" action per goal adds an entered amount to its saved total (advancing the
  progress bar) via a quick prompt. The list now sorts incomplete goals first, then alphabetically.
- Top bar: the "+ Add" button now navigates to the Transactions screen (was inert).
- Budgets: a month stepper (‹ month ›) lets you view budget spend for any month, not just the current
  one. A health line summarizes how many budgets are over or near their limit.
- To-do: an optional notes field on tasks, shown in the task row.
- To-do: a "Hide done" / "Show all" toggle to filter completed tasks, with an "All done 🎉" state.
- Accounts: archive/restore an account from its row — archived accounts move to a separate "Archived"
  section and drop out of the assets/liabilities lists and net-worth totals (already excluded by
  `ledger`).
- Categories screen: add categories (name + income/expense), listed grouped by kind with per-row
  delete; reachable from a new "Categories" rail item (tag icon) under System. Deleting a category
  still used by transactions or budgets is blocked with a plain-English message.
- Members screen: add household members (name + color), list with a color swatch, set the default
  member, and per-row delete; reachable from a new "Members" rail item (users icon) under System.
  Deleting a member who still owns accounts/budgets/goals is blocked with a plain-English message.
  Also shows a "Net worth by member" rollup (each member + group-shared) via `ledger.NetByOwner`.
- Freshness nudge widget (full-width, dashboard): a friendly reminder of which account balances look
  stale (via `internal/freshness`) with days since each was last updated; the bento grew to 8 rows.
- Settings screen (replacing the stub): a household summary (base currency, member/account/category
  counts) and an in-app **debug log viewer** (newest first, with Refresh) reading the slog ring buffer.
- Accounts: a "Mark updated" action per (active) account sets its `BalanceAsOf` to today, clearing the
  staleness flag the freshness nudge surfaces.
- Accounts: a welcome card with a "Load sample data" button when there are no accounts yet
  (onboarding) — seeds the store via `appstate.LoadSample`.
- Accounts: a "Stale" badge on accounts whose balance is overdue for a refresh (via
  `freshness.IsStale`), complementing the dashboard nudge and the per-row "Mark updated" action.
- Accounts: liability rows with a credit limit show their credit utilization ("N% of limit used").
- Accounts: the add form reveals a **liability sub-form** (credit limit, interest APR, minimum
  payment, due day, lender) when a liability type is selected — feeding the Upcoming-bills widget and
  credit-utilization display, which previously had no data entry path.
- Accounts: for asset types, the add form reveals **allocation attributes** (expected return APR,
  liquidity, stability) — giving the Allocate engine real per-account scores instead of zeros.
- Persistence switched from IndexedDB to pure-Go in-memory SQLite (`ncruces/go-sqlite3`, no cgo, no
  dependency on browser web storage); the JSON `Dataset` remains the portable import/export and sync
  payload. (Confirmed pure-Go SQLite compiles for `js/wasm` and runs in the browser.)
- Expanded `TODOS.md` into a granular, per-entity/service/screen backlog covering the full spec.
- Serve web assets from `web/` (clean project root); restyled host page with a dark theme.
- Require bottom-up SDLC build order in `CLAUDE.md` (data model → services/logic with tests →
  persistence → state → UI last).

# CashFlux — Component Inventory (`internal/ui`)

All exported primitives live in `internal/ui` (build tag `js && wasm` unless noted).
Pure helpers (no build tag, native-testable): `SelectOption`, `OptionsFrom`, `IndentPx`, `JoinClass`.

---

## Primitives

### Card (`primitives.go`)
**Purpose:** Titled section card (`.card` + optional `.card-title` + optional header-action + body).
Absorbs the 170× hand-rolled `Section(css.Class("card"), H2(…), …)` scaffold.
```go
ui.Card(ui.CardProps{Title: "Accounts", HeaderAction: addBtn, Body: listNode})
```

### EntityListSection (`primitives.go`)
**Purpose:** Full card + title + header-action + empty-state-OR-body; the canonical CRUD section shell.
Wraps `Card`; screens supply title, optional add button, empty state, and list body.
```go
ui.EntityListSection(ui.EntityListSectionProps{Title: "Budgets", HeaderAction: addBtn, EmptyState: emptyNode, Body: rowsNode})
```

### FormField (`primitives.go`)
**Purpose:** Labeled field — visible caption above a control (`.labeled-field`). Fixes unlabelled
controls across C49–C65/B15.
```go
ui.FormField("Account name", Input(css.Class("field"), …))
```

### SelectInput (`select.go`)
**Purpose:** Native `<select>` from a typed `[]SelectOption`. Kills the 103× hand-rolled `Option`
loops; owns its `OnChange` hook (safe in loops via `uic.CreateElement`).
```go
ui.SelectInput(ui.SelectInputProps{Options: opts, Selected: cur, OnChange: fn, AriaLabel: "Category"})
```

### SelectOption / OptionsFrom (`select_pure.go` — no build tag)
**Purpose:** `SelectOption` is the typed value+label pair. `OptionsFrom[T]` converts any slice to
`[]SelectOption` with value/label extractor funcs.
```go
opts := ui.OptionsFrom(accounts, func(a domain.Account) string { return a.ID }, func(a domain.Account) string { return a.Name }, currentID)
```

### EntityRow (`primitives.go`)
**Purpose:** Generic list row (`.row`): optional leading node, `.row-desc` title, `.row-meta` lines,
trailing action nodes. Hookless — safe to call directly inside `MapKeyed` loops.
```go
ui.EntityRow(ui.EntityRowProps{Title: a.Name, Meta: []string{fmtBalance(a)}, Actions: []uic.Node{editBtn, deleteBtn}})
```

### IconButton (`primitives.go`)
**Purpose:** Accessible icon-only button (`aria-label` + `title`). Optional `.btn-del` danger style.
Owns its click hook — safe in variable-length loops.
```go
ui.IconButton(ui.IconButtonProps{Icon: icon.Edit, Label: "Edit account", OnClick: fn})
```

### DeleteButton (`primitives.go`)
**Purpose:** Destructive icon-only button (`.btn-del` + Close/Trash icon). Consolidates the 18×
hand-rolled delete-button pattern; owns its click hook (loop-safe).
```go
ui.DeleteButton(ui.DeleteButtonProps{AriaLabel: "Delete transaction", OnClick: fn})
```

### ExportButton (`primitives.go`)
**Purpose:** Labeled export/download button (inline-flex icon + span). Consolidates the 14× hand-rolled
export-button pattern; caller supplies `OnClick` (keeps primitive free of `downloadBytes`/`syscall/js`).
```go
ui.ExportButton(ui.ExportButtonProps{Label: "Export CSV", OnClick: func() { downloadBytes(…) }})
```

### StatGrid / Stat (`primitives.go`)
**Purpose:** `.stat-grid` of labeled figures with optional tone classes (`"pos"`, `"neg"`, `"dim"`).
Promotes the `stat()` helper repeated 9× across screens.
```go
ui.StatGrid([]ui.Stat{{Label: "Net worth", Value: "$12,400", Tone: "pos"}, …})
```

### OverflowMenu (`overflowmenu.go`)
**Purpose:** ⋯ trigger + popover menu (`add-wrap`/`add-menu` pattern). Consolidates 27× scattered
overflow/quick-add patterns. Each menu item may be `Hidden: true` for conditional visibility.
```go
ui.OverflowMenu(ui.OverflowMenuProps{Items: []ui.OverflowMenuItem{{Label: "Edit", Icon: icon.Edit, OnSelect: fn}}})
```

### InlineEditForm (`inlineeditform.go`)
**Purpose:** `.row-edit` + `.form-grid` + Save/Cancel chrome. The per-row edit wrapper repeated on
every CRUD screen; wires `OnSubmit`, `OnSave`, `OnCancel`, and optional extra content.
```go
ui.InlineEditForm(ui.InlineEditFormProps{Fields: []uic.Node{…}, OnSave: saveFn, OnCancel: cancelFn})
```

### TreeRows / IndentPx (`treerows.go` — no build tag)
**Purpose:** `IndentPx(depth int) string` returns the CSS `padding-left` value for a nesting depth
(16 px per level). Used by category-tree and nested-task rows to produce real indentation (not
em-dash prefixes per C63/C72).
```go
Style(map[string]string{"padding-left": ui.IndentPx(node.Depth)})
```

### DataTable (`datatable.go`)
**Purpose:** Sortable, paginated `<table>` shell. Caller renders `<tr>` rows; `DataTable` owns the
semantic table, sortable `<th>` headers (with `aria-sort` + caret), and pagination footer.
```go
ui.DataTable(ui.DataTableProps{Columns: cols, Body: rowNodes, Sort: sortKey, Dir: "asc", OnSort: fn, Page: p, PageSize: 25, Total: n, OnPage: pgFn})
```

### FilterToolbar (`filtertoolbar.go`)
**Purpose:** Search box + Filters popover trigger + active-filter chips + trailing action buttons.
Screen-agnostic — used by Transactions, Accounts and To-do.
```go
ui.FilterToolbar(ui.FilterToolbarProps{Search: q, OnSearch: fn, FilterFields: fieldNodes, Chips: chips, OnRemoveChip: removeFn})
```

### ControlStrip (`controlstrip.go`)
**Purpose:** Control/action slots for pages that do not need `FilterToolbar` search,
filter-panel and chip behavior. Goals and Assistant use it. The outer class is
`filter-strip`; `Class` adds page classes. `ControlsClass` and `ActionsClass` replace
the slots' default `filter-strip-controls` class. `Attrs` preserves page attributes.
Supply a translated `Label` for a named group. The component owns no hooks or
financial calculations; callers pass prepared nodes.
```go
ui.ControlStrip(ui.ControlStripProps{
    Label: controlsLabel,
    Controls: []uic.Node{periodPicker, viewPicker},
    Actions: []uic.Node{addButton},
})
```

### FlipPanel (`flippanel.go`)
**Purpose:** Dimmed/blurred backdrop + 3D-flip settings overlay. Generic props-driven shell used by
per-widget and global settings; caller supplies title, body, size, `OnSave`, `OnClose`.
```go
ui.FlipPanel(ui.FlipPanelProps{Title: "Widget settings", Back: bodyNode, OnSave: saveFn, OnClose: closeFn})
```

### Widget (`widget.go`)
**Purpose:** Bento dashboard cell shell: grip handle, centered title (drill-to-screen), gear button,
padded body, optional resize handles, drag-reorder. Every dashboard tile uses this.
```go
ui.Widget(ui.WidgetProps{ID: "budgets", Title: "Budgets", Body: contentNode, Draggable: true, Resizable: true})
```

### AreaChart (`chart.go`)
**Purpose:** Filled area sparkline SVG from a `[]float64` value series. Stretches to container width.
```go
ui.AreaChart(ui.AreaChartProps{Values: vals, Stroke: "#2e8b57", Label: "Cash-flow trend"})
```

### ProgressBar (`progress.go`)
**Purpose:** Horizontal fill bar (`.prog-track` + `.prog-fill`) with a tone token (`"bg-up"`,
`"bg-warn"`, `"bg-down"`, `"bg-dim"`, `"bg-fg"`). Clamps percent to `[0, 100]`.
```go
ui.ProgressBar(ui.ProgressBarProps{Percent: 68, Tone: "bg-warn"})
```

### Icon (`icon.go`)
**Purpose:** Compile-checked stroked SVG icon from the curated `icon.Name` set; inherits color from
`currentColor`, sized by caller classes.
```go
ui.Icon(icon.Accounts, css.Class(tw.W4, tw.H4))
```

### Segmented (`controls.go`)
**Purpose:** Row of mutually exclusive options (`.seg`). Roving tabindex + arrow-key navigation
(ARIA `radiogroup`). Reused for time resolution, theme selector, etc.
```go
ui.Segmented(ui.SegmentedProps{Options: []ui.SegOption{{Value: "month", Label: "Month"}, …}, Selected: cur, OnSelect: fn})
```

### SegmentedControl (`segmented.go`)
**Purpose:** Keyed pressed choices or tabs with caller-owned classes, optional
counts, disabled options and stable test IDs. To-do uses pressed choices for
views/date lenses; Notifications uses tabs for triage and Goals uses tabs for
Goals/Earmarks. `Segmented` above retains
its animated radio-group presentation.

Use stable, unique, nonempty keys. The group and button children own their hooks;
mount groups through `uic.CreateElement` in variable-length lists. Tabs support
wrapped Left/Right navigation, Home/End, disabled skipping and roving tab stops.
`Selected` and `OnSelect` keep selection caller-owned.
```go
ui.SegmentedControl(ui.SegmentedControlProps{
    Label: viewLabel, Selected: currentView, OnSelect: selectView,
    Class: "page-views", ButtonClass: "page-view", ActiveClass: "is-active",
    Options: []ui.SegmentOption{
        {Key: "list", Label: listLabel},
        {Key: "calendar", Label: calendarLabel},
    },
})
```

### surfaceTiles (`screens/surface_tiles.go` — screen package)
**Purpose:** Keyed widget placement host for Accounts, Budgets, Goals, To-do and
Notifications. It adds `bento`, accepts page classes/root attributes, copies the
render context per placement and delegates failure handling to `safeRenderSpec`.
Placement IDs must be stable, unique and nonempty. Event hooks stay in children.

Transactions retains specialized `txnPlacement` dispatch/memoization. Dashboard
and Reports retain composed grids/sections. Use the shared host when its placement
contract fits the page; preserve specialized performance boundaries.

### Native view models and bounded caches
Financial rules and aggregate preparation belong in native packages (`accountview`,
`assistantview`, `budgetview`, `dashboardview`, `goalview`, `notificationview`,
`reportview`, `taskview`). Screens format facts and own events. Cached maps/slices
are read-only; copy before sorting or editing.

`viewcache.Cache` bounds retained models. Screen `revKey` includes app identity,
stored revision and browser data revision; callers append scope, period, time and
other inputs. Large snapshots use capacity one. Tests cover context/data/FX/date
invalidation. See [Primary page performance](PRIMARY_PAGE_PERFORMANCE.md) for local
measurements and their limits.

#### Goals: prepare once, render children from facts

`goals.BuildTaskIndex(tasks)` scans linked tasks once and produces per-goal
`TaskSummary` values (counts plus at most three previews). `goalview.Build(Input)`
uses that index to build one `goalview.RowModel` per visible goal; each row carries
the canonical `goals.Evaluation`, task summary, growth projection and explicit
`Now`. The same model set supplies sections, summary figures and health ordering.
Pass the row model into `GoalRow`, then into `GoalFigures` and
`GoalTaskPreview`; those child components render prepared values and own their
interactive hooks. Keep the stored `domain.Goal` alongside the model for writes
and callbacks. This prevents list rows from independently re-running evaluation
or rebuilding task indexes. `GoalRow`'s nil-model fallback is for isolated use;
it rebuilds the task index, so do not use that fallback in a per-goal list.

The page-level view must be rebuilt when goals, tasks, accounts, transactions,
FX rates, member scope or the effective date changes. Stored writes flow through
the app/data revision; the date key must also account for day changes and goal
deadline/pause transitions (`goalview.TimeKey`). Do not mutate or sort the view's
shared slices/maps in render code; create a copy when a presentation needs a
different ordering. Child row components should be mounted as components (such
as `ui.CreateElement(GoalRow, ...)`) so per-row event hooks remain isolated and
stable.

Compact goal cards evaluate the shared row model but do not prepare interest,
trajectory or contribution-history details. `goals_card_details.go` owns that
expanded-only preparation boundary; preserve it when adding card details. The
contribution-history panel uses `Evaluation.Assignment` for the canonical
monthly plan (explicit plan converted to the goal currency, or evaluated
required pace when no explicit plan exists). It uses checked `goalhistory`
series/summary arithmetic; assignment failure, currency mismatch or overflow must
render as unavailable rather than a partial history. Contribution writes use
`Goal.RecordContributionChecked`, which preserves the original goal on checked
rollup failure and retains the established policy for a same-month currency
conflict. Keep contribution persistence in shared `appstate` hooks: an optional
ledger debit and the goal history are committed together, and notification,
workflow and observer effects run only after that commit succeeds. UI components
own the user action and feedback, not the transaction boundary or post-commit
effects.

#### Period totals: carry availability with the numbers

`periodview.BuildTotals` is the checked preparation boundary for Dashboard and
Assistant period activity. `Totals.Known` must gate use of `Income`, `Expense`
and `Net`: a failure converting any included transaction or checked subtraction
leaves those values unavailable rather than implying zero activity.
`SavingsKnown` is a separate check because a savings-rate calculation can fail
even when the money totals are known. Preserve the existing no-division display
convention for zero income. Dashboard widgets receive the complete
`RenderCtx.PeriodTotals`; legacy `Income`/`Expense` fields are meaningful only
when its `Known` flag is true. Do not drop the availability bit when building a
widget context or replace an error with zero-valued money.

#### Prepared dataset ingress

`store.PreparedDataset` is an opaque token created only by `PrepareDataset`,
which performs the ordinary full import validation and migration. App recovery
may retain and pass that token through `appstate.ParsedDataset` only while the
source checkpoint is unchanged. `LoadPrepared` skips only the normalizers
already covered by that validation; ordinary `Load` remains the ingress for
unprepared datasets. Keep the token's dataset private, reject its zero value,
preserve per-device credential replacement and run transaction-link netting
after a successful load. This avoids duplicate work without introducing a
caller-controlled validation bypass.

#### Reports: typed sections and lazy full-detail rendering

`reportController` owns the report's fixed hook sequence, state, scope/window
selection and shared prepared data. Its `reportSectionContext` explicitly
includes the identities that change how a section reads or formats facts:
revision, merged report/app scope, base currency, language, accent, date range,
current activity date and preferences. Section components receive typed props
containing this context and only their needed models/actions. Keep these display
identities in props when adding sections; callback identity or transaction count
alone does not invalidate translated, currency-formatted, themed or differently
scoped output.

Keep expensive full-detail sections as component elements (`ui.CreateElement`)
under the Full-mode branch. Summary mode should mount the compact executive view
without executing the long-form section builders. Do not call a section renderer
directly while constructing the controller's Summary branch, and do not put
controller hooks inside conditional Summary/Full branches or variable-length
loops. Any section with hooks must itself be a component with an unconditional,
fixed hook order. The report controller may prepare shared native aggregates for
the summary and full report, but each expensive section should stay separately
mountable so unopened detail isn't rendered speculatively.

For annual-review money-flow detail, use the checked native helpers
`reports.Per100Checked` and `reports.PositiveSavingsPer100`; they use integer
amounts, return overflow errors and let the UI show an explicit unavailable state
instead of partial or wrapped totals. Do not duplicate the per-$100 arithmetic in
screen code or substitute an unchecked `int64` multiplication/float percentage.

#### Accounts: disclose detail before preparing it

`accountSnapshot(app)` is the page's revision-keyed shared balance/ledger snapshot.
Pass it to one `accountDetailContext` for the rendered account rows. Compact
`AccountRow` renders balance and status without asking that context for history,
flows or projections. Its own disclosure state gates those reads; the selected
90-day/12-month/all history range is local, view-only row state. Keep this lazy
boundary when adding account detail: expensive facts belong inside the disclosure,
while the per-row component owns its hooks and child components own theirs.

The detail context's broad facts key includes app/data revision, rates, base
currency, as-of instant and time zone, plus account ID. This covers balance-derived
flow/projection and valuation history. Projection inputs are prepared once per
revision and reused across open rows. Historical account series are native-currency
facts, so their cache keys intentionally omit display FX/preferences and instead
include account/revision, day, zone and requested day count. Invalid exact balances
must remain unavailable; do not turn missing/invalid balances into zero-valued
history or forecast curves. A revision change invalidates the details while a
range change reuses the prepared history.

#### Goal allocation editor: a draft is not authority to save

`goalview.BuildAllocationDraft` is the native owner of the manual earmark draft:
it indexes other goals' reservations once, calculates checked account headroom from
the shared account snapshot and current rates, and exposes immutable rows. The UI
owns text/selection changes only. Unedited existing allocation records retain
their original currency and order, including legacy or unavailable sources;
display conversion failure is not permission to erase or rewrite them. Explicit
amount edits and new selections must pass current currency, eligibility and
headroom checks. Split refuses to silently omit selected unavailable reservations.

The editor's snapshot cache is keyed by app revision and goal ID. Re-read that
snapshot before splitting and again at save, and apply changes against the latest
goal and balances. This closes the gap between an open modal and concurrent store,
balance or FX changes. If the goal disappeared or the currency changed, show an
error and leave storage alone. Keep row controls in `goalAllocateRow` components
when mapping draft rows so each row owns stable input hooks; keep validation and
arithmetic in the native draft model.

#### Dashboard: cache layout separately from financial frames

The dashboard has two intentionally independent preparation layers. The native
`dashboardview.LayoutCache` prepares validated placements from instance, ready
state, layout items, widget configs, hidden IDs, stored placements and current
registry defaults. It retains two immutable configurations and freezes nested
inputs before caching. Financial data revisions are not layout inputs: a new
transaction should refresh frames without revalidating placement schemas. Layout
changes should rebuild the placements without forcing unrelated frame preparation.
Use this cache for the dashboard's configurable placement schema; do not use it as
a generic financial-data cache.

`dashboardview.FrameContexts` owns the other layer. Its identity includes app
instance/revision, preference revision, full scope, base currency, full FX-rate
identity, period bounds, clock minute and engine-variable timestamp. Each prepared
context caches up to 32 serialized pipeline requests; `Hydrate` returns cloned
frame fields/values so renderers cannot mutate cached results. Keep the identity
complete when adding a frame input, and keep source data read-only. Moving or hiding
a tile reuses the same financial context; changing data, scope, currency/rates,
period or time identity refreshes it. Both caches are bounded.

Dashboard's settled-state boundary also matters: the hero and direct headline
figures render first, while engine variables and configurable tile frames wait
until the surface settles. The drag-preview driver subscribes to drag atoms outside
the expensive controller, so pointer movement updates tile shells without
recomputing frames. Preserve that split when adding layout interactions or data
tiles. Reuse the existing `accountSnapshot`/view-model preparation and
`PreparedFrames` seams when another surface has the same inputs and immutable
pipeline contract; otherwise keep the cache page-specific rather than weakening
its identity.

### Toggle (`controls.go`)
**Purpose:** Pill on/off switch (`.switch`, ARIA `role="switch"`). Keyboard-accessible (Space/Enter).
```go
ui.Toggle(ui.ToggleProps{On: enabled, OnChange: fn, Label: "Dark mode"})
```

### ToggleRow (`controls.go`)
**Purpose:** Settings row (`.toggle-row`): label left + Toggle right. Standard building block for
preferences forms.
```go
ui.ToggleRow(ui.ToggleRowProps{Label: "Show archived accounts", On: prefs.ShowArchived, OnChange: fn})
```

### Swatch / SwatchPicker (`controls.go`)
**Purpose:** `Swatch` is one selectable color chip (ARIA `role="radio"`). `SwatchPicker` renders a
roving-tabindex radiogroup of swatches; used by accent-color and widget-style pickers.
```go
ui.SwatchPicker(ui.SwatchPickerProps{Colors: palette, Selected: cur, OnSelect: fn})
```

### StepperPill (`controls.go`)
**Purpose:** Centered label flanked by previous/next chevrons (`.rpill`). Generic stepped-value
control reused for period paging, budget-month navigation, etc.
```go
ui.StepperPill(ui.StepperPillProps{Label: "June 2026", OnPrev: prevFn, OnNext: nextFn})
```

### EmptyStateCTA (`screens/emptystate.go` — screen package)
**Purpose:** Friendly empty-state block: muted icon + message + CTA button. CTA can focus an
inline form field (`FocusID`), open a modal (`AddTarget`), or navigate (`Href`).
```go
ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: t("accounts.empty"), CTALabel: t("accounts.addFirst"), AddTarget: "account"})
```

---

## Porting Guide — Replacing Legacy Idioms

| Legacy idiom | Replace with |
|---|---|
| `Section(css.Class("card"), H2(css.Class("card-title"), title), body)` | `ui.Card(ui.CardProps{Title: title, Body: body})` |
| `Section(css.Class("card"), H2(…), addBtn, empty-or-rows)` | `ui.EntityListSection(ui.EntityListSectionProps{Title: …, HeaderAction: addBtn, EmptyState: …, Body: rowsNode})` |
| `Div(css.Class("rows"), MapKeyed(items, key, render))` | Pass as `Body` to `EntityListSection`; or keep as-is inside the Body arg |
| `for _, o := range items { Option(Value(o.ID), SelectedIf(cur == o.ID), o.Name) }` | `ui.SelectInput(ui.SelectInputProps{Options: ui.OptionsFrom(items, …), Selected: cur, OnChange: fn})` |
| `Button(css.Class("btn-del"), OnClick(…), Icon(icon.Close, …))` | `ui.DeleteButton(ui.DeleteButtonProps{AriaLabel: "Delete …", OnClick: fn})` |
| `Button(css.Class("btn", tw.InlineFlex, …), OnClick(…), Icon(…), Span(label))` (export) | `ui.ExportButton(ui.ExportButtonProps{Label: label, OnClick: fn})` |
| `Button(css.Class("btn"), Attr("aria-label", lbl), OnClick(…), Icon(…))` (icon-only) | `ui.IconButton(ui.IconButtonProps{Icon: ic, Label: lbl, OnClick: fn})` |
| `Div(css.Class("stat-grid"), stat(label, val), …)` / inline `stat()` helper | `ui.StatGrid([]ui.Stat{{Label: …, Value: …, Tone: …}, …})` |
| `Label(css.Class("labeled-field"), Span(css.Class("t-caption", …), lbl), control)` | `ui.FormField(lbl, control)` |
| `Style(map[string]string{"padding-left": fmt.Sprintf("%dpx", depth*16)})` (tree indent) | `Style(map[string]string{"padding-left": ui.IndentPx(depth)})` |
| Bare `P(css.Class("empty"), msg)` inside a card | `ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: msg, …})` |

### Migration order (Phase plan)
- **Phase 1 (forms):** Every add/edit form → `FormField` + `SelectInput`/`OptionsFrom`.
- **Phase 2 (lists):** `Div(.rows)` → `EntityListSection` (with existing body) or `DataTable`/`FilterToolbar` for sortable screens.
- **Phase 3 (rows):** Split `*Row` → `*DisplayRow` + `*EditForm`; fold display into `EntityRow`.
- **Phase 4 (done):** Super-screens decomposed (Planning, Documents, Allocate, Customize, settings).
- **Phase 5 (cleanup):** Delete residual bespoke markup; enforce ratchet via `scaffold_baseline_test.go`.

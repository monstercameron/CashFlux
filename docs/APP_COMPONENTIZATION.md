# App componentization

Scope requested on 2026-10-21: all routed app pages, separate business logic from
views, test the extracted logic, and optimize measured performance. This extends
the earlier work on the nine primary pages. It does not authorize a release.

## Boundaries

1. Domain and service packages own money, FX, filtering rules, eligibility,
   schedules, validation, write plans and aggregation. They compile natively
   without `syscall/js` and expose errors or per-fact availability.
2. Native prepared models compose service results, indexes and derived facts.
   Their inputs include scope, dates, currency, preferences and reference time.
   They do not read browser globals or mutate caller-owned slices.
3. Screen controllers acquire data, own UI state and dispatch approved commands.
   They compose child components from typed prepared props.
4. Presentational components format facts, display unavailable states and invoke
   supplied callbacks. Interactive rows own their hooks and stable entity keys.
5. Shared components live in `internal/ui` when interaction and semantics repeat
   across pages. Page-specific financial behavior stays in a native package.

Moving a large function into another screen file is organization, not completion
of the business-logic boundary. Conversely, a small domain-specific component
does not need a generic abstraction just to reduce file count.

## Current inventory

The registry in `internal/screens/screens.go` contains 47 routes: nine primary,
19 other rail pages and 19 off-rail or system routes. Route registrations remain
the coverage checklist. Existing primitives and component contracts are described
in [Component inventory](COMPONENTS.md).

The primary pages already use `accountview`, `budgetview`, `goalview`,
`taskview`, `txnview`, `txntable`, `notificationview`, `assistantview`,
`reportview`, dashboard preparation and shared surface hosts. Preserve those
boundaries and cache behavior while finishing remaining gaps.

## Current work sequence

| Slice | Current evidence/status | Next evidence needed |
| --- | --- | --- |
| First nine primary routes | Source and representative validation references are reconciled below; this is not a claim that every view fact has moved into a native model. | Continue the route boundary review and resolve verified residuals below. |
| Remaining routes | The 47-route registry and observed boundary classes are recorded in [Route coverage inventory](APP_ROUTE_COVERAGE.json). | Review each route's ownership and exception against the same checklist; aliases and transport routes are not separate financial models. |
| Performance | Native algorithm baselines and measured native changes are recorded below and in the linked performance notes. | Browser route/interaction before-and-after measurements are still required to claim page latency improvement. |
## Completion checks

- Review every registered route against the boundaries above; record remaining
  exceptions explicitly instead of declaring completion from the primary pages.
- No financial calculation in markup or row callbacks; native services own the
  calculation and retain truthful known/unavailable states.
- No event hook inside variable-length parent loops. Dynamic children own hooks
  and use stable IDs; sorting must not move edit/focus state to another entity.
- Prepare shared indexes once. Include every actual dependency in memo keys,
  while avoiding invalidation from unrelated settings or AI usage writes.
- Native tests cover meaningful domain edges and invalidation. Focused WASM
  tests cover wiring and identity; broad suites verify integration after a batch.
- Report measured performance effects. A code extraction by itself is not
  evidence that the page became faster. See
  [Primary page performance benchmark](PRIMARY_PAGE_PERFORMANCE.md).

The initial native model capture, with five samples per benchmark and the exact
environment/target caveat, is retained in
[Native baseline](performance/componentization-baseline-2026-10-21/app-refactor-baseline-native-summary.md).
These existing algorithm comparisons establish a reference; they are not a new
application before/after result.

## First nine primary routes: source and validation references

The table records current source seams and representative test files present in the
worktree. References identify coverage locations; this documentation reconciliation
did not run tests. The current full-screen suite passed 1,075 top-level tests after the lifecycle fixes. Call sites
show `surfaceTiles` on Accounts, Budgets, Goals, To-do and Notifications; `FilterToolbar`
on Transactions, Accounts and To-do; `ControlStrip` on Goals, Assistant and Reports;
`SegmentedControl` on Goals, To-do and Notifications. Transactions keeps specialized
`txnPlacement`; Dashboard uses `ui.Widget`; Reports has its own typed section composition.

| Route | Native preparation evidence | Shared/page components in source | Representative validation references | Verified boundary notes |
| --- | --- | --- | --- | --- |
| `/` Dashboard | `dashboardview` layout/frame preparation; `widgetrender` | `dashboardController`, `ui.Widget`, `DashboardLayoutControls` | `internal/screens/dashboard_preparation_wasm_test.go`, `dashboard_hero_wasm_test.go`, `dashboard_counts_wasm_test.go`; `internal/dashboardview/*_test.go` | Layout and financial-frame preparation are separate caches; configurable/detail tiles are deferred until settle. No browser latency improvement is established. |
| `/transactions` | `txnview`, `txntable`, `txnfilter`, `txnreview`, `txntoolbar` | `transactionsSurface`, specialized `txnPlacement`, `ui.FilterToolbar`, receipt preview overlay | `internal/screens/transactions_derive_test.go`, `txn_route_links_wasm_test.go`, `transactions_widget_r10w_wasm_test.go`, `transactions_tiles_gate4_wasm_test.go` | Keep specialized placement/memoization separate from `surfaceTiles`. |
| `/accounts` | `accountview`; shared goal reservation facts from `goalview` | `surfaceTiles`, `AccountRow`, `accountDetailContext`, `ui.FilterToolbar` | `internal/screens/accounts_details_lazy_wasm_test.go`, `reservations_primary_wasm_test.go`, `accounts_primary_wasm_test.go` | History/projections are disclosure-lazy and cache-keyed; invalid balances remain unavailable. |
| `/budgets` | `budgetview`, `budgeting` | `surfaceTiles`, `BudgetRow`, `BudgetAnnualGrid` | `internal/screens/budgets_bc4b1_wasm_test.go`, `budgets_summary_native_wasm_test.go`, `budgets_flex_native_wasm_test.go` | Native summary selection and flex preparation preserve availability; no additional route-specific residual recorded in the source audit. |
| `/goals` | `goalview`, `goals`, `taskview` | `surfaceTiles`, `goalListWidget`, `GoalRow`, `GoalFigures`, `GoalTaskPreview`, `ui.ControlStrip`, `ui.SegmentedControl` | `internal/screens/goals_primary_wasm_test.go`, `goals_sort_wasm_test.go`, `goals_segmented_tabs_wasm_test.go`; `internal/goalview/*_test.go` | One prepared row model feeds summaries and children; no additional route-specific residual recorded in the source audit. |
| `/todo` | `taskview`, `todoview` | `surfaceTiles`, `todoListWidget`, `TaskRow`, `ui.FilterToolbar`, `ui.SegmentedControl` | `internal/screens/todo_primary_wasm_test.go`, `todo_paging_memo_wasm_test.go` | View selection is page state while task preparation is native; no additional route-specific residual recorded in the source audit. |
| `/notifications` | `notificationview` | `surfaceTiles`, `notifSurfaceShell`, `notifListWidget`, `notifyRow`, `ui.SegmentedControl` | `internal/screens/notifications_primary_wasm_test.go`, `notifications_initial_wasm_test.go`; `internal/notificationview/*_test.go` | Prepared notification facts feed page triage; no additional route-specific residual recorded in the source audit. |
| `/assistant` | `assistantview`, `periodview`, insights services | `AssistantHub`, `assistantInsightsDataPanel`, typed `assistantAskHeader`/`assistantChatThread`, `ui.ControlStrip` | `internal/screens/assistant_chat_components_wasm_test.go`, `assistant_refactor_wasm_test.go`, `ag_reports_correctness_wasm_test.go` | Conversation state/actions remain controller-owned; financial answers use checked prepared facts. Component extraction is not a measured latency improvement. |
| `/reports` | `reportview`, `reports`, `periodview`, health/goal services | `reportController`, typed report section components, `ui.ControlStrip` | `internal/screens/reports_render_wasm_test.go`, `reports_sections_wasm_test.go`, `reports_tax_checked_wasm_test.go` | Summary leaves expensive sections unmounted; retain typed context identity and checked arithmetic. |

This is a source-and-reference inventory, not a completion certificate. Route coverage
references do not imply that a test passed during this documentation pass.

## Verified residuals

- **Allocate:** `internal/screens/allocate.go` ignores the error from
  `ledger.PeriodTotals` while preparing the income-prefill hint, so a failed
  conversion/aggregation can be presented as a zero-like hint. Preserve availability
  and suppress or label the hint on failure. `internal/screens/allocate_view.go`
  still derives the displayed allocatable amount as `TotalMinor - ReserveMinor` in
  `allocView.Allocatable`; move that fact into native preparation. These are separate
  from native candidate ranking and split preparation.
- **Events:** the page delegates totals to `events.Totals`, but
  `internal/events/events.go` accumulates `int64` totals without overflow checking.
  This is a native service correctness gap. Parsing typed date input and dispatching
  writes remain adapter/command responsibilities.
- **Sync route:** the registry comment describes `/sync` as a compatibility redirect,
  while `SyncScreen` returns the injected `SyncView`. Confirm boot injection behavior
  before documenting it as a redirect.

Categories period usage is prepared by `categoryview.PreparePeriod`; Rules coverage,
counts and conflict facts come from `rules.SummarizeCoverage`. Earlier claims that
these calculations remain assembled in their screen renderers are stale. Planning's
string parsing is a typed input adapter to `planningmodel.Prepare`, not forecast
arithmetic in markup.

## Performance evidence limits

The native baseline is retained in
[Native baseline](performance/componentization-baseline-2026-10-21/app-refactor-baseline-native-summary.md).
It compares existing algorithms and is not an application before/after result.
Measured native changes include shared category-tree union traversal,
[Unusual history buffer reuse](performance/unusual-buffer-reuse-2026-10-21/README.md),
and [checked Settings snapshots](performance/settings-snapshot-2026-10-21/README.md).
No browser route/interaction before-and-after evidence is recorded here; therefore no
page-latency improvement across the nine routes is claimed.

The route inventory was reconciled against the source audit at `e14066dc`, with
concurrent in-progress edits visible. Source references are file-level because active
refactors can move functions.

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

The registry in `internal/screens/screens.go` contains 47 static routes: nine
primary, 19 other rail pages and 19 off-rail or system routes. User-authored
custom pages are served separately through `/p/:slug` and are not included in
that registry count; their shared renderer and per-page widget composition still
need boundary review. Route registrations remain the static coverage checklist.
Existing primitives and component contracts are described in
[Component inventory](COMPONENTS.md).

The primary pages already use `accountview`, `budgetview`, `goalview`,
`taskview`, `txnview`, `txntable`, `notificationview`, `assistantview`,
`reportview`, dashboard preparation and shared surface hosts. Preserve those
boundaries and cache behavior while finishing remaining gaps.

## Current work sequence

| Slice | Current evidence/status | Next evidence needed |
| --- | --- | --- |
| First nine primary routes | Source and representative validation references are reconciled below; this is not a claim that every view fact has moved into a native model. | Run the root final gates for pending source seams and record any new gaps. |
| Remaining routes | The 47 static-route registry, dynamic `/p/:slug` custom-page route and observed boundary classes are recorded in [Route coverage inventory](APP_ROUTE_COVERAGE.json). | Review each route's ownership and exception against the same checklist; aliases and transport routes are not separate financial models. |
| Performance | Native algorithm baselines and measured native changes are recorded below and in the linked performance notes. | Browser route/interaction before-and-after measurements are still required to claim page latency improvement. |

## Current source seams pending root final gates

The following source and test files are present in the current worktree. Their
presence is not a pass claim: verification remains pending the root final gates.

| Area | Current source evidence | Validation evidence present in source |
| --- | --- | --- |
| Budget funding, row and pool | `internal/budgetview/funding.go`, `internal/budgetview/pool.go`, `internal/screens/budgets_row.go` and related row components | `internal/screens/budgets_funding_boundary_wasm_test.go`, `internal/screens/budgets_pool_host_wasm_test.go`, budget row host tests |
| Dashboard What changed | `internal/screens/dashboard_whatchanged.go`, `internal/dashboardview/trend_facts.go` | `internal/screens/dashboard_whatchanged_host_wasm_test.go`, `internal/screens/dashboard_trend_facts_host_wasm_test.go` |
| Retirement | `internal/planningmodel/retirement.go`, `internal/retirement/retirement.go`, `internal/screens/planning_retirement.go` | Native retirement tests and `internal/screens/planning_retirement_host_wasm_test.go` |
| Debt strategy | `internal/debtview/prepare.go`, `internal/debtplan/debtplan.go`, `internal/screens/debt_coach.go` | Native debt preparation/plan tests and `internal/screens/debt_loans_native_wasm_test.go` |
| Recurring agenda | `internal/recurringview/agenda.go`, `internal/screens/recurring_agenda.go` | Native agenda tests and `internal/screens/recurring_agenda_unavailable_host_wasm_test.go` |
| Custom fields | `internal/customfields/formvalues.go`, `internal/customfields/customfields.go`, `internal/screens/customfields.go` | Native custom-fields tests and `internal/screens/customfield_parse_wasm_test.go` |
| Subscription trailing spend and PYF | `internal/subscriptions/trailing_spend.go`, `internal/workflow/payyourself.go` | Native trailing-spend/PYF tests and `internal/screens/subscriptions_financial_wasm_test.go` |

## Completion checks

- Review every static registered route and the dynamic `/p/:slug` custom-page
  renderer against the boundaries above; record remaining exceptions explicitly
  instead of declaring completion from the primary pages.
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
did not run tests. At an earlier recorded checkpoint, the full-screen suite passed
1,075 top-level tests after lifecycle fixes; that is not the current root final-gate
result. Call sites
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

## Resolved boundary evidence

- **Allocate:** `allocateview.Prepare` owns the displayed allocatable
  amount. The screen retains checked transaction and income-total errors, labels
  unavailable income, and suppresses prefill while allowing manual amount entry.
  Native `allocateview` and `i18n` suites and all three Allocate host tests passed;
  fixtures cover missing FX, total overflow, malformed amounts and closed sources.

## Remaining source gaps

- **Events:** the page delegates totals to `events.Totals`, but
  `internal/events/events.go` accumulates `int64` totals without overflow checking.
  This is a native service correctness gap. Parsing typed date input and dispatching
  writes remain adapter/command responsibilities.
- **Settings:** the app injects `SettingsView`; the route registry cannot verify
  that calculations and validation in the injected view stay in native services.

No current source residual is recorded for Accounts delta arithmetic, Dashboard
trend/WhatChanged preparation, Goals Compare pooled contribution arithmetic,
Budget funding/row/pool arithmetic, or Reports arithmetic. The corresponding
source implementations are listed above or in the route table; their final
verification status is subject to the root gates. Reports still has a measured
browser latency regression, and the full app-wide componentization/performance
scope remains incomplete.

The `/sync` compatibility route receives `SyncRedirect` from
`internal/app/cloudtab.go` during boot and redirects to `/settings/cloud`.
The injected `SyncView` seam is transport wiring, not a missing financial model.

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
Current browser route evidence is retained in
[Primary-page browser captures](performance/primary-pages-2026-10-21/README.md).
The paired captures preserve exact visible-text parity and show lower local
settle medians on eight routes; Reports is slower. Interaction and cold-start
improvement remain unproven. This does not establish the full nine-page latency
objective.

The route inventory retains the historical source audit at `e14066dc` and the
follow-up first-nine review through `22dca667`. This document was reconciled
against current worktree sources on 2026-10-21; source and test-file presence do
not establish completion of the root final gates. References are file-level
because active refactors can move functions.

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

## Execution order

| Work | First targets | Required evidence |
| --- | --- | --- |
| Calculation correctness | Assistant local answers; tax-plan inputs; historical credit utilization; household totals; subscription monthly totals | Native overflow, FX, empty/unavailable, date and independent-factor tests; a focused screen adapter test |
| Remaining prepared models | Budget summary union, auto-budget scaling, allocation candidate/model assembly | Model parity, immutable inputs, error paths, native benchmark where the model is expensive |
| Large component controllers | Assistant conversation controls and content; Reports sections; Widget Builder panels; Dashboard composition | Stable hook order, keyed identity, route/state preservation, focused WASM interaction tests |
| Authoring and secondary pages | Rules, Categories, Workflows, Documents, Planning, Household, remaining tools/settings | Per-route component/data/command ownership review; reuse existing controls when semantics match |
| Performance | Cold model build, warm cache hit, invalidation, route navigation and representative interactions | Same-fixture before/after raw measurements, actual artifact hashes, allocations and route-specific conclusions |

Work in small vertical slices: prove the native service/model, wire it into a
screen, verify the interaction, then commit and push that slice. Do not combine
unrelated route rewrites into one commit.

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

## Initial remaining calculation gaps

The read-only inventory found unchecked or error-hiding paths in local Q&A,
tax-plan income reduction, historical credit utilization, household rollups and
subscription share totals. Budget summary selection and allocation preparation
still contain model composition in screen code. These are starting targets;
the route audit remains necessary for the rest of the application.

No production behavior or performance result is claimed by this plan.

## Completed slices

- Budget summaries use native checked union selection and independent summary
  availability; auto-budget tuning is the next separate slice.
- Household arithmetic prerequisites use checked owner shares, period income
  and member spending. The Household/Members adapter migration is in progress.
- Tax-plan net income uses a checked native helper and explicit household scope.
  The legacy estimator policy is unchanged; selected-fold optimization is in progress.
- Credit history is a native checked model with independent gaps and currency
  availability. Whole-page credit aggregate/advice migration is in progress.
- Subscriptions use native cadence, magnitude, annual/monthly totals and share
  facts. Checked failures do not become successful zero amounts.
- Setup money parsing and validation are native, exact and tested before UI writes.
- Investment growth headlines use independent checked value/change/percentage
  facts. Full investment preparation is still in progress.
- Net Worth account windows use indexed native balance/flow facts. Full Net Worth
  model preparation is still in progress.
- Rules coverage uses the canonical native matcher once per prepared summary,
  with stable row identity and lazy previews.
- Assistant Ask header and conversation thread are typed child components in
  `assistant_chat_components.go`. The controller owns state and actions; the
  children own their event hooks and render prepared props. Tests cover header
  callbacks/ARIA, edit-state identity through turn reordering, retry eligibility,
  deletion and thinking/streaming states. This extraction is not a measured
  latency improvement.

Focused WASM adapter checks pass for the committed slices above. Native package
tests and vet pass for their models. Two broad-run timing failures were isolated
to concurrent-machine contention; unchanged timing gates passed in quiet repeats.
This is a batch checkpoint, not completion of the 47-route audit.

## Remaining work and evidence

Native-first lanes are working on Household/Members, Artifacts, auto-budget
tuning, full Credit/Net Worth/Investments, Planning, Plan, Allocate, Debt/Loans,
Recurring, Categories, Custom Fields, Settings/Builder, Activity/Workflows and
Split input preparation. Bills/Health and Documents follow those bounded slices.
Small static or transport-only routes will be reviewed against the same boundary
checklist; they do not need financial models invented for them.

The initial per-route evidence is retained in
[Route coverage inventory](APP_ROUTE_COVERAGE.json). Source line numbers in this
capture are navigation hints from the initial audit and can move during refactors.
An existing native import or child component is evidence, not proof that the whole
route is complete.

Measured native changes include shared category-tree union traversal and
[Unusual history buffer reuse](performance/unusual-buffer-reuse-2026-10-21/README.md).
Moving calculations out of views is not itself a speed claim. Browser route and
interaction measurements remain a separate integration gate.

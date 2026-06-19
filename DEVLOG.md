# CashFlux — Developer Journal

Narrative companion to `CHANGELOG.md`. Newest entries first. Capture decisions, trade-offs,
problems and fixes, and what's next.

## 2026-06-19 — feat: spending anomaly heads-up (B21)

- Added pure `reports.SpendingAnomalies(txns, now, months, overPct, minMinor, rates)`: per category, compares
  the current month's spend to the trailing-`months` average (span-based denominator like SuggestLimit, so a
  young category isn't diluted) and flags those over by ≥ overPct AND ≥ minMinor absolute, biggest overage
  first. More robust than the existing vs-last-period delta; the floor skips noise on tiny categories.
- Surfaced as a "Heads up" card on Reports (top 3): "%s is %d%% above its usual." Placed the computation after
  `nameOf` is in scope (first attempt referenced it too early — moved it). Added reports.headsUp/anomaly keys.
- Table tests: a 200%-over category flagged while steady rent / sub-floor / no-baseline categories are not;
  steady-only and edge (zero months / empty) cases. gofmt clean, reports + i18n tests green, wasm build green.

## 2026-06-19 — feat: minimum-payment guidance on Planning (D9)

- Added pure `payoff.MinimumViablePayment(balance, apr)` = first-month interest + 1 minor unit (the first
  month's interest is the largest since the balance only falls, so anything above it reduces principal every
  month). 0 for non-positive balance, 1 for non-positive APR. Test asserts the invariant against Project: the
  returned figure clears the debt and one unit less does not.
- Surfaced in the Planning payoff calculator: the "payment too low" branch now uses
  `planning.paymentTooLowMin` to name the minimum ("Pay at least $X a month to start clearing it.") instead of
  a generic message. Kept the old key.
- Aside: confirmed backup-reminder wiring stays deferred — recording lastBackupAt lives in app/settings.go
  (export action, line ~715), a hot shared file the parallel session owns, and the cadence needs a Settings
  control there too. Not a surgical isolated change.
- gofmt clean, payoff + i18n tests green, wasm build green. Restored docs from HEAD first.

## 2026-06-19 — feat: no-spend days on Reports (B21)

- Added pure `reports.NoSpendDays(txns, start, end, now)`: iterates the period's days and counts those with no
  expense, capping the window at the end of `now`'s day so future days in the current period don't inflate it
  (past periods count fully; future periods return 0). Transfers/income don't count; multiple charges on one
  day collapse to a single spend day. Added a small `dayStart` helper.
- Surfaced as a "No-spend days" stat (pos tone) on the Reports grid, shown when > 0. Table tests: current
  partial period, past full month, future period. Added the reports.noSpendDays i18n key.
- gofmt clean, reports + i18n tests green, wasm build green. Restored docs from HEAD first.

## 2026-06-19 — feat: wire large-transaction alerts into catch-up (B19)

- Wired `notifyfeed.LargeTransactionCandidates` into `runNotifyCatchUp` (my app/notifyrun.go). Added
  `largeTransactionCandidates(app, now)`: reads the threshold from the `default-large` rule, uses a 30-day
  `since` window (so the first open doesn't replay all history; the txn-id key handles repeats), and renders
  notify.largeTitle/largeBody (with a largeNoDesc fallback for blank descriptions). default-large is already in
  DefaultRules, so CatchUp gates it.
- Backup reminders remain deferred: they need a persisted `lastBackupAt` (updated by the export flow) that
  doesn't exist yet — without it there's nothing to judge the cadence against.
- gofmt clean, i18n tests + wasm build green. Restored docs from HEAD first; committed app/notifyrun.go
  path-scoped (it's my file in app/, parallel session works elsewhere there).

## 2026-06-19 — feat: large-transaction notifications (B19)

- Completed the notify event coverage for `EventLargeTransaction` (the constant existed but had no generator or
  default rule). Added `notifyfeed.LargeTransactionCandidates(ruleID, txns, threshold, since, rates, text)`:
  base-currency expense magnitude ≥ threshold, on/after `since` (so the caller scopes it to the gap since last
  open), keyed `txn:<id>` so each big charge fires exactly once; non-positive threshold yields nothing.
- Added a `default-large` rule (threshold `defaultLargeTxnMinor` = 50000 = $500) and updated defaults_test
  (5→6 rules, added the event to the coverage list + a threshold assertion).
- The catch-up wiring in app/notifyrun.go (passing recent txns + the rule threshold) is the deferred piece
  (app/ collision); the rule sits inert until then, producing no candidates. Table tests cover the in-window
  big expense, small/old/income exclusions, the txn-id key, and zero-threshold. Fixed an unused `now` in the
  test. notify + notifyfeed tests green, wasm build green. Restored docs from HEAD first.

## 2026-06-19 — feat: settle-up CSV export (B24)

- Added pure `split.CSV(transfers, name, amount)` (From,To,Amount header; callback-based name/amount, FX/format
  agnostic, mirrors reports/bills CSV) + a Download CSV button on the Split settle-up card. The screen builds
  `[]split.Transfer` alongside the on-screen owes list (sharer→payer, share) and feeds it to split.CSV.
  Added split.downloadCsv/downloadCsvTitle i18n keys.
- Also ran a full health sweep this iteration: all 13 of my logic packages (budgeting/reports/payoff/split/
  subscriptions/bills/goals/backup/notify/notifyfeed/ledger/money/i18n) pass `-count=1`. gofmt clean, wasm
  build green. Restored docs from HEAD first.

## 2026-06-19 — feat: budget limit suggestions from history (D6)

- Added pure `budgeting.SuggestLimit(categoryID, txns, now, months, rates)`: averages the category's spend over
  the trailing full months, dividing by the span from the oldest month with spend through the most recent (so a
  one-month-old category isn't divided by the whole window, while genuine zero-spend months inside the span
  still pull the average down). Excludes the current partial month, transfers, and income.
- Surfaced it in the Budgets add form: "You've averaged $X/mo here recently." for the selected category plus a
  "Use this" button that sets the limit field (FormatMinor into major units). Added budgets.suggest/useSuggest
  i18n keys. Table tests: multi-month span average, new-category (denom 1), gap handling, and edge cases.
- gofmt clean, budgeting + i18n tests green, wasm build green. Restored docs from HEAD first.

## 2026-06-19 — feat: headline spending trend on Reports (B21)

- Added a one-line spending-trend summary under the Reports stat grid, comparing this period's expense to the
  prior comparable window (`reports.IncomeVsExpense` over `ps,pe` + `ledger.PercentChange`). Up/down phrased
  via two i18n keys (reports.spendUp/spendDown) showing the magnitude; nothing rendered when there's no prior
  baseline (PercentChange ok=false) or no change. UI-only. gofmt clean, i18n tests + wasm build green.

## 2026-06-19 — feat: subscriptions' share of spending (B25)

- Added a "Share of spending" stat to the Subscriptions grid: `subscriptions.MonthlyTotal(subs) * 100 /` this
  month's expense (from `ledger.PeriodTotals` over `dateutil.MonthRange(now)`). Rendered as a Fragment when
  there's no spending this month, so it simply drops out of the grid rather than dividing by zero. UI-only;
  added the subs.shareOfSpending i18n key. gofmt clean, i18n tests + wasm build green. Restored docs from HEAD.

## 2026-06-19 — feat: biggest deposits (B21)

- Added pure `reports.LargestIncome` mirroring LargestExpenses (gated on IsIncome, reuses ExpenseItem as the
  generic largest-item shape, top-n with deterministic tie-break). Surfaced a "Biggest deposits" card on the
  Reports screen above income-by-source, reusing the screen's nameOf/fmtMinor/pr helpers. Added the
  reports.biggestDeposits i18n key.
- Table test covers top-n + expense/transfer/out-of-range exclusions. gofmt clean, reports + i18n tests green,
  wasm build green. Restored docs from HEAD first.

## 2026-06-19 — feat: annual bill cost on the Bills screen (B22)

- Added a "Per year" stat to the Bills grid: the already-computed (FX-converted) monthly bill total × 12.
  Bills are monthly recurring (Upcoming derives one occurrence per liability account), so ×12 is the right
  annualization. UI-only — kept the sum screen-side because each bill's amount is FX-converted before summing
  (the bills package stays currency-agnostic via callbacks). Added the bills.annualCost i18n key. gofmt clean,
  i18n tests + wasm build green. Restored docs from HEAD first.

## 2026-06-19 — feat: net-worth breakdown on Reports (B21)

- Added a "Net worth" stat card (assets / liabilities / net, toned pos/neg/accent) to the Reports screen above
  the net-worth trend chart, using `ledger.NetWorth(accounts, txns, rates)` (already tested). Shown when there
  are any accounts. UI-only — no new pure logic or i18n keys (reused dashboard.netWorth, accounts.assets,
  dashboard.liabilities). gofmt clean, wasm build green. Restored docs from HEAD first.

## 2026-06-19 — feat: goal on-track / pace check (D12)

- Added pure `goals.OnTrack(goal, monthly, from) (onTrack, known, err)` plus `OnTrack`/`PaceKnown` fields on
  `Status`. Judges a dated goal against its target date at an assumed monthly contribution (reusing Project):
  complete → on track; no target date or no usable projection → known=false. Evaluate derives the fields from
  already-computed complete/projected/has (no extra Project call).
- Did NOT surface on the Goals screen this pass: that screen computes progress directly (not via Evaluate) and
  has no stored per-goal contribution rate to judge against — wiring it needs a contribution input / product
  decision. The pure check is immediately useful to Allocate's goal-progress scorer and the Planning forecast.
- Table tests: on-pace, behind, complete, undated (not judgeable), zero-contribution (not judgeable), plus the
  Evaluate field wiring. gofmt/vet/native tests green, wasm build green. Restored docs from HEAD first.

## 2026-06-19 — feat: proportional mode on the Split screen (B24)

- Surfaced `split.ByWeights` on the Split screen behind a "Split by weight" toggle. Added `weighted` +
  `weights` (per-member string map) state; in weighted mode shares come from ByWeights (blank weight → 1, so a
  fresh proportional split == even until adjusted; explicit 0 excludes), else from Equal as before. Settle-up
  already reads shareByID, so it follows either mode.
- Per-member weight inputs would put an OnInput hook inside the member loop (forbidden), so I extracted a
  `SplitMemberRow` component that owns its weight-input hook (toggle + weight field + share). Added
  split.byWeight/split.weight i18n keys.
- gofmt clean, i18n tests + wasm build green. Restored docs from HEAD first.

## 2026-06-19 — feat: weighted expense split (B24)

- Added pure `split.ByWeights(total, []WeightedMember)` next to the existing even `Equal`. Splits in
  proportion to int64 weights (share counts or incomes), using the largest-remainder (Hamilton) method:
  assign each `total*weight/sumW` floor, then hand the leftover (always < #weighted-members) one unit at a
  time to the biggest fractional remainders (ties by order). Shares sum to total exactly; zero/negative-weight
  members are kept with a zero share; nil when there's no positive-weight basis.
- Kept int64 throughout (documented the `total*weight` overflow assumption — fine for household sums). Table
  tests: 2:1, equal-weights, 60/30/10, remainder placement, zero-weight, no-basis, and a 7-way exact-sum.
- Logic-first per the SDLC; the Split-screen even/proportional toggle is a follow-up. gofmt/vet/native tests
  green, wasm build green. Restored docs from HEAD first.

## 2026-06-19 — feat: downloadable income & member breakdowns (B21)

- Added Download CSV buttons to the Reports "Income by source" and "Spending by member" cards. Income reuses
  the existing `reports.CategoryCSV` (income rows are CategorySpend with Amount only → Prior/Change blank). For
  members, added a new pure `reports.MemberCSV(rows, name, amount)` (Member,Amount header) with a table test
  covering the unassigned bucket. Both buttons reuse the existing downloadCsv/downloadCsvTitle i18n keys, so no
  new strings.
- gofmt clean, reports tests green, wasm build green. Restored docs from HEAD first.

## 2026-06-19 — feat: income by source (B21)

- Added pure `reports.IncomeByCategory` mirroring SpendingByCategory but gated on `IsIncome` (positive,
  non-transfer); reuses the CategorySpend result type (Amount only, no comparison). Surfaced as an "Income by
  source" card on the Reports screen between biggest-expenses and top-payees, reusing the screen's nameOf/
  fmtMinor helpers. Added the `reports.incomeBySource` i18n key.
- Renamed the test helper `income`→`incomeTxn` (collided with an existing `income` in the package tests).
  Table tests: sort + expense/transfer/out-of-range exclusions, empty input. gofmt clean, reports + i18n tests
  green, wasm build green. Restored docs from HEAD first (re-truncated to 0).

## 2026-06-19 — feat: debt payoff strategy comparison on Planning (D9)

- Surfaced `payoff.BuildPlan` on the Planning screen. Builds `[]payoff.Debt` from non-archived liability
  accounts with a positive owed balance (`ledger.Balance(a, txns).Abs()`), pulling `InterestRateAPR` and
  `MinPayment` straight off the account. An "extra per month" input feeds both BuildPlan(Snowball) and
  BuildPlan(Avalanche); the card shows each method's months + total interest, the avalanche payoff order, and
  the interest avalanche saves vs snowball.
- Empty state when there are no qualifying liabilities; an alert when neither plan is viable (minimums can't
  outpace interest). Added the planning.debtStrategy*/snowball/avalanche/strategy* i18n keys.
- Restored CHANGELOG/DEVLOG from HEAD first (re-truncated to 0). gofmt clean, i18n tests + wasm build green.

## 2026-06-19 — feat: debt snowball / avalanche planner (D9)

- Added pure `payoff.BuildPlan(debts, extra, strategy)` alongside the existing single-debt `Project`. Models
  the real debt-snowball mechanic: a constant monthly budget (sum of all minimums + extra), held flat as
  debts clear so freed minimums accelerate the rest. Each month: accrue interest, pay every active minimum,
  then dump the leftover on the strategy's focus debt (`Snowball` = smallest balance, `Avalanche` = highest
  APR), cascading to the next focus when one clears mid-month.
- Viability: rejects negative extra / non-positive budget, and returns ok=false the first month total balance
  fails to fall (interest outpaces the budget) — plus the existing maxMonths cap. Tracks payoff Order, and a
  conservation invariant (TotalPaid == principal + TotalInterest) is asserted in tests.
- Table tests: snowball vs avalanche pick different focuses (and avalanche pays ≤ interest — the optimality
  property), 0%/12-month exact, no-debts, already-paid skipped, not-viable, negative-extra, zero-budget.
- Logic-first per the SDLC; the Planning-screen strategy comparison UI is a follow-up. gofmt/vet/native tests
  green, wasm build green. Restored CHANGELOG/DEVLOG from HEAD first (re-truncated to 0 bytes).

## 2026-06-19 — feat: spending-by-weekday insight (B21)

- Added pure `reports.SpendingByWeekday` returning a `[7]int64` indexed by `time.Weekday` (Sunday=0), so
  callers read `totals[t.Weekday]` directly, plus `PeakWeekday` (highest-spend day, ok=false when all zero,
  ties to earliest). Transfers/income excluded. Table tests cover bucketing/exclusions, peak, and tie.
- Surfaced as a one-line muted insight under the category narrative: "Most spending happens on %s (%s)." using
  `time.Weekday.String()` for the day name (English; full weekday localization is a future refinement) and the
  formatted amount. Added the `reports.peakWeekday` i18n key.
- Docs were intact this iteration (no truncation). gofmt clean, reports + i18n tests green, wasm build green.

## 2026-06-19 — feat: spending by member (B21)

- Added pure `reports.SpendingByMember(txns, start, end, rates)` (mirrors SpendingByCategory: per-member
  expense totals, base currency, largest first, transfers/income excluded, empty MemberID = unassigned).
  Table tests: sort + exclusions, empty-member bucket, empty input.
- Surfaced it on the Reports screen as a "Spending by member" card, shown only when more than one member/bucket
  has spend (so a single-member household doesn't get a redundant card). Resolves names from app.Members(),
  labels the empty id "(unassigned)". Added `reports.byMember`/`reports.noMember` i18n keys.
- Shipped core + UI in one commit since the surfacing is small and cohesive. Restored CHANGELOG/DEVLOG from
  HEAD first (both re-truncated to 0 bytes). gofmt clean, reports + i18n tests green, wasm build green.

## 2026-06-19 — feat: cash runway on the Reports screen (B21)

- Surfaced `reports.EstimateRunway` on the Reports stat grid. Liquid balance = sum of `ledger.Balance` for
  non-archived cash-type accounts only (checking/debit/savings/cash), FX-converted to base. Burn = average of
  the last six FULL months (built monthly bounds from `dateutil.MonthStart(now)` back six months, excluding
  the current partial month so it doesn't understate spending), via `AverageMonthlyExpense`.
- Gated the stat on `burn > 0` so a no-spend dataset doesn't render a misleading "0 months". Added
  `accentForRunway` (under 3 mo → warning, 6+ → positive) and the `reports.runway`/`reports.runwayMonths`
  i18n keys.
- CHANGELOG was re-truncated to 0 bytes by the parallel session (DEVLOG intact this time); restored from HEAD
  before editing. gofmt clean, i18n tests + wasm build green.

## 2026-06-19 — feat: financial runway estimator (B21)

- Added pure `reports.EstimateRunway(balance, monthlyBurn) Runway` (Months/Days/Sustainable) + a companion
  `reports.AverageMonthlyExpense(flows)` that averages Expense across monthly PeriodFlows, skipping
  fully-inactive buckets (no income AND no expense) so empty months don't drag the burn toward zero. Built on
  the existing `IncomeExpenseSeries` shape so a caller does `EstimateRunway(liquidBalance, AverageMonthlyExpense(series))`.
- Edge handling: non-positive burn → Sustainable (balance never depletes); non-positive balance with burn →
  zero runway; leftover prorated into days against a 30-day month (always 0–29, no overflow at realistic
  amounts). Table tests cover all branches + an end-to-end average→runway.
- Logic-first per the SDLC; surfacing on the Reports screen needs a liquid-balance (cash-accounts-only)
  computation, deferred to a follow-up. gofmt/vet/native reports tests green, wasm build green.
- Restored CHANGELOG/DEVLOG from HEAD first (parallel session re-truncated both to 0 bytes again).

## 2026-06-19 — feat: budget pace warning on screen (D2)

- Surfaced `budgeting.ProjectPace` on the Budgets screen. In the per-budget evaluation loop I now also call
  ProjectPace(st, bs, be, now) with `now = time.Now()` and stash a formatted overspend in `paceOver[budgetID]`
  — but only when `!OnTrack && 0 < Elapsed < 1 && state != StateOver`, so a finished period (Elapsed clamps to
  1 → projected = actual) or an already-over budget doesn't show a redundant warning. Using real now (not the
  viewed period start) means past/future periods naturally show no pace line.
- Threaded it through `budgetRowProps.PaceOver`; BudgetRow renders a `budgets.paceOver` line (text-down) under
  the period sub-line. Added the i18n key.
- Restored CHANGELOG/DEVLOG from HEAD again first — the parallel session had re-truncated both to 0 bytes
  (now a recurring pattern; see the hazard memory). Verified size before committing. gofmt clean, i18n tests +
  wasm build green.

## 2026-06-19 — feat: show subscription price changes on screen (B25)

- Surfaced `subscriptions.DetectPriceChanges` on the Subscriptions screen as a read-only "Recent price
  changes" card (delta/percent/new amount/date, most-recent first, only shown when non-empty). Price-change
  rows have no per-row interactive elements, so they render inline via MapKeyed (no Row component needed).
  Added `subs.priceChangesTitle`/`subs.priceUp`/`subs.priceDown` i18n keys.
- Hazard event: while finishing this commit the parallel session truncated **both** CHANGELOG.md and
  DEVLOG.md to 0 bytes and didn't restore them for 60s+. I committed the code + i18n + sw.js path-scoped
  (8dfb5a5) WITHOUT the docs (committing empty docs would have wiped 180KB+186KB of history), then restored
  both from HEAD (`git checkout HEAD -- CHANGELOG.md DEVLOG.md`) and re-added these entries in a follow-up
  commit. Verified my prior B25/D2 entries survived the restore. wasm build + i18n tests green.

## 2026-06-19 - feat: bound sync lookup workspace ids

- Found one input-bound gap left in sync: `PutWorkspace` and `Delete` enforced the workspace id length cap,
  but `Get` only checked for an empty id before hitting the repository.
- `SyncService.Get` now trims the id and rejects anything over `maxWorkspaceIDLength` with
  `InvalidArgument`, matching the existing Put/Delete field-limit behavior.
- Extended `TestSyncServiceRejectsOversizedWorkspaceFields` to cover long Get ids; focused server test passed
  before the full gate run.

## 2026-06-19 - feat: enforce tls-safe browser config

- Tightened backend config validation for browser-facing URLs: `CASHFLUX_SERVER_APP_ORIGIN` must now be an
  HTTPS origin, with HTTP allowed only for localhost/loopback development, and wildcard origins are rejected.
- Applied the same TLS rule to OAuth redirect URLs after the existing `/v1/auth/{provider}/callback` path
  allow-list, so production OAuth callbacks cannot be configured over cleartext HTTP.
- Added config tests for wildcard origins, non-loopback HTTP origins, non-loopback HTTP OAuth redirects, and
  the allowed HTTPS/loopback cases. This closes the self-host security-defaults item; full TLS redirect/cipher
  policy remains tracked under the broader transport TODO.

## 2026-06-19 — feat: subscription price-change detection (B25)

- Added `subscriptions.DetectPriceChanges` to catch when a recurring charge's price changes. The existing
  `Detect` groups by name+amount (key = name|amount), so a price change would surface as two unrelated
  subscriptions — no good for this. The new detector groups by name only, sorts each series by date, confirms
  a regular cadence (reusing `classify`/`medianInt`), then walks back from the latest charge to the most
  recent charge with a different amount: that's the prior price, and the charge right after it dates the
  change.
- minCount floors at 3 (not 2): a change needs a before-run and an after-run, so two charges can't tell a
  change from a one-off. Returns OldAmount/NewAmount/Delta/PercentChange (rounded, guarded against a
  zero base)/ChangedAt, plus an `Increased()` helper. Sorted most-recent-change first.
- Documented the heuristic limit: it reports the latest distinct-amount transition, so usage-based/fluctuating
  charges can be noisy — but the cadence gate filters most of those out.
- Fixed a builtin shadow (`new` → `cur` param) and a missing `domain` import in the new test file. Table
  tests: increase, decrease, stable (no change), irregular-spacing (ignored), minCount floor, and
  most-recent-first ordering across two series. gofmt/vet/native tests green, wasm build green.
- Logic-first per the SDLC; surfacing it on the Subscriptions screen ("▲ $2/mo since April") is a follow-up.
  Path-scoped commit per the parallel-tree hazard memory.

## 2026-06-19 — feat: budget pace projection (D2)

- Added `budgeting.ProjectPace(status, start, end, now) Pace` — forecasts end-of-period spend by linear
  extrapolation (spent ÷ elapsed-fraction), the forward-looking complement to Status. Returns Elapsed,
  Projected, OverBy, OnTrack.
- Design choices: recover the limit as `Spent + Remaining` so no `currency.Rates` is needed and the currency
  always matches `Status.Spent`; `elapsedFraction` clamps to [0,1] and treats a degenerate/zero span as fully
  elapsed; before any time elapses we can't extrapolate, so projection = spend-so-far; clamp the float
  projection at MaxInt64 so a near-zero fraction can't overflow int64. Documented that early-period projection
  is noisy (one big day-one purchase projects huge) → present as a gentle heads-up, not a hard prediction.
- Table tests: half-period on-track vs over, before-start (no extrapolation), full-period (= actual),
  currency-follows-Spent, plus elapsedFraction boundaries + degenerate span. gofmt/vet/native tests green,
  wasm build green.
- Logic-first per the SDLC; the Budgets-screen "on pace / projected over by $X" badge is a follow-up (app/
  collision with the parallel session). Path-scoped commit per the parallel-tree hazard memory.

## 2026-06-19 — feat: wire backup reminders into notifications (B28)

- Bridged the B28 backup cadence core into the B19 notification system. Added `notify.EventBackupDue` plus a
  `default-backup` rule in `DefaultRules` (enabled, in-app), and updated `defaults_test.go` (4→5 rules,
  added the event to the coverage list).
- Added `notifyfeed.BackupCandidates(ruleID, cadence, lastBackupAt, now, text)`: returns one informational
  candidate when `backup.Due` is true, keyed `backup@<period>` (WeekKey for weekly, MonthKey for monthly) so
  it nudges once per cadence period and is idempotent across reopens via the existing DeliveredLog. nil when
  not due or Off. Kept it gentle (SeverityInfo) per the non-naggy ethos. Table tests cover not-due, off,
  due-monthly/weekly keying, and never-backed-up (fires immediately, 0-days body).
- Deferred to a follow-up (state/UI, app/ collision with the parallel session): persisting `lastBackupAt` in
  localStorage, calling BackupCandidates inside `runNotifyCatchUp`, and the dismissible export nudge +
  Settings cadence control.
- gofmt + vet clean; notify + notifyfeed native tests green; wasm build green.
- Process note: committing path-scoped (`git commit -F msg -- <paths>`) per the parallel-tree hazard memory,
  after last iteration's shared-index race split add from commit.

## 2026-06-19 — feat: budget rollover & sinking-fund math (B26)

- B26 wants envelope rollover + sinking funds. Verified first (per the spec): the budget engine already
  carries unspent forward across periods via `EnvelopeAvailable`, which re-derives the running balance from
  the whole transaction history. What was missing is the *single-step* recurrence and the sinking-fund math.
- Added pure `internal/budgeting/rollover.go`: `Carryover(prevRemaining, limit)` (advance one period — feed
  it last period's `Status.Remaining`, which is negative on overspend, plus this period's limit); and the
  sinking-fund trio `SinkingFundContribution` (ceiling division so the target is met on/before the deadline
  rather than a few cents short), `SinkingFundAccrued` (contribution × made, capped at target so the rounded
  final period doesn't overshoot, with currency + overflow guards), and `SinkingFundProgress` (capped 0–100).
- Did the int64 arithmetic directly on `Money.Amount` and re-wrapped via `money.New` — the money package has
  no Mul/Div, matching how budgeting already computes. Table tests cover even/remainder splits, the
  reaches-target invariant, the overspend-as-debt carry, capping, and the currency/overflow error paths.
- Logic-first per the SDLC. Deferred (domain + UI, parallel-session overlap in app/): the per-budget
  `Rollover bool` field, the "carried over $X" badge, and a sinking-fund budget type / methodology selector.
- Native budgeting tests green (-count=1), vet clean, gofmt clean, wasm build green.

## 2026-06-19 - feat: add backend blob garbage collection

- Added `cashflux-server gc-blobs` to sweep unreferenced blob metadata/files through the existing content-addressed store cleanup path.
- Added Prometheus counters for blob GC sweeps and deleted blobs, plus weekly systemd service/timer examples for self-host installs.
- Documented the GC schedule and the existing audit/snapshot list caps in self-hosting docs; deploy and repository tests cover the command artifacts and metrics.

## 2026-06-18 — feat: Split rail icon (B24)

- Added a Lucide "split" `icon.Split` and wired `/split` into `railMeta` (`nav.split`), so the Split screen
  shows a diverging-paths glyph instead of the neutral page icon. Updated the icon curated-set test
  (19→20). Completes the dedicated rail icons for all four new Tools screens (Reports, Subscriptions,
  Bills, Split). Native icon test green (forced -count=1), wasm build green.

## 2026-06-18 - feat: add backend structured logging

- Added `server.NewLogger` with text/json handlers, `CASHFLUX_SERVER_LOG_FORMAT`, and `CASHFLUX_SERVER_LOG_LEVEL`.
- Switched `cmd/cashflux-server` from package `log` to a configured `slog` logger for startup, shutdown, and fatal errors.
- Added redaction for sensitive attribute names and tests covering redaction, level filtering, and config validation.

## 2026-06-18 - feat: add backend liveness probe

- Added `GET /livez` so orchestration can distinguish process liveness from database readiness.
- Kept `/healthz` as the existing compatibility health endpoint and `/readyz` as the SQLite-backed readiness probe.
- Covered the split in server tests: live/health return 204 without a store, ready returns 503 without one.

## 2026-06-18 — feat: Split-a-shared-expense calculator screen (B24)

- Surfaced the pure `internal/split` core as a self-contained `screens.Split()` (`/split`, Tools group):
  amount input + per-member include toggles + a "who paid?" select. Computes `split.Equal(amt, ids)` live
  and shows each included member's share; with a payer chosen, lists "X owes Payer $share" (each non-payer
  owes their portion). No persistence or domain Split model needed — a handy household calculator that also
  exercises the core ahead of the full transaction-level split + settle-up.
- Hooks: amount input + payer select are stable single positions; member rows render `uiw.ToggleRow`
  (a component with a func `OnChange` prop, not an On* option) inside `MapKeyed`, so the per-row toggle is
  loop-safe. Deliberately kept to screens + screens.go route + i18n (the parallel agent is in
  app/server/theme). New `nav.split` / `screen.splitSub` / `split.*` keys; rail shows it with the neutral
  icon (a dedicated icon is an easy follow-up). wasm build green, gofmt clean.

## 2026-06-18 — feat: Reports savings-rate trend (B21)

- Added pure `reports.SavingsRateSeries(txns, bounds, rates)`: the whole-percent savings rate per bucket
  (reuses `IncomeExpenseSeries` + `PeriodFlow.SavingsRate`, so it matches the dashboard — 0 with no income,
  negative when overspending). Table-tested incl. the +50% / −50% / no-income cases.
- Wired a "Savings-rate trend" `ui.AreaChart` on the Reports screen over the same six-period `bounds` used
  by the cash-flow/net-worth charts (ints → float64; the chart handles negatives). New
  `reports.savingsTrend` key. Kept entirely in reports + reports_screen.go (parallel agent is in
  app/server/theme). wasm build green, gofmt + go vet clean.

## 2026-06-18 - feat: bound ai upstream retries

- Added `CASHFLUX_SERVER_AI_UPSTREAM_TIMEOUT` and `CASHFLUX_SERVER_AI_UPSTREAM_RETRIES` to cap OpenAI proxy calls.
- AIService now wraps upstream calls with a deadline and retries transient transport, 429, and 5xx failures with bounded jittered exponential backoff.
- Covered retry success, timeout mapping to `DeadlineExceeded`, validation of negative config, and retry backoff bounds in server tests.

## 2026-06-18 — feat: Reports largest-expenses report (B21)

- Added pure `reports.LargestExpenses(txns, start, end, rates, n)`: the period's biggest individual
  expenses, largest first (ties → most recent date, then description). Same conventions as the other
  reports (IsExpense, dateutil.InRange, base-currency convert). Table-tested incl. range/income/transfer
  exclusion and the n-limit + tie-break.
- Wired a "Biggest expenses" card on the Reports screen (top 8): description (or category name when blank)
  + date + amount. Bound a `pr := uistate.UsePrefs().Get()` for the date formatting (was only taking
  `WeekStartWeekday()` before). Kept the work in the reports package + my reports_screen.go to avoid the
  hot app/ layer (a concurrent push race this session confirmed the parallel agent is active there). New
  `reports.biggestExpenses` key. wasm build green, gofmt + go vet clean.

## 2026-06-18 — feat: weekly digest reminder — all four events live (B19, step 10)

- Added `weeklyDigestCandidates(app, now)`: summarizes the *previous* completed ISO-week's income vs
  spending via `reports.IncomeVsExpense` over `period.NewWindow(Week, now, weekStart).Shift(-1).Range()`,
  keyed by the *current* week (`notify.WeekKey(now)`) so the first open each week shows last week's recap.
  Emits nothing when the week had no activity (quiet weeks don't nag). Money formatted with a local
  `fmtBaseMoney` (accounting style). New `notify.digest{Title,Body}` keys.
- **All four recommended Phase-A notification events now fire end-to-end**: stale-balance, bill-due,
  budget-threshold, and the weekly digest — each idempotent via the delivered log, surfaced as the single
  on-open summary toast. B19's core user-facing behavior is complete; the in-app center (a list view) and
  the Settings rules UI (toggle channels/thresholds/quiet hours) are the remaining polish. wasm build
  green, gofmt clean.

## 2026-06-18 — feat: budget-threshold reminders in the catch-up (B19, step 9)

- Extended `runNotifyCatchUp` with the budget-threshold event. New `currentBudgetStatuses(app, now)` helper
  mirrors the Budgets screen: evaluates every budget over its own current period via
  `budgeting.PeriodRange(b.Period, now, weekStart)` + `EvaluateRollup(..., DefaultNearThreshold,
  categorytree.Descendants(...))` (parent budgets roll up sub-categories), skipping any that error. Feeds
  the statuses to `notifyfeed.BudgetCandidates` → near (warning) / over (critical) candidates.
- Three of the four events now fire live (stale-balance, bill-due, budget-threshold); only the periodic
  digest remains (needs the period summary text). Localized via new `notify.budget{Over,Near}{Title,Body}`
  keys. Still one summary toast, still recover-guarded. wasm build green, gofmt clean.

## 2026-06-18 — feat: wire notify catch-up on load (B19, step 8 — live!)

- Wired the pure notify pipeline into the wasm shell: new `internal/app/notifyrun.go` (`runNotifyCatchUp`)
  is called once at the end of `Run()`. It gathers the current stale-balance (via `app.FreshnessWindows()`)
  and bill-due (via `bills.Upcoming`) occurrences through the `notifyfeed` generators, runs
  `notify.CatchUp(notify.DefaultRules(), …)` against a delivered log persisted in localStorage
  (`cashflux:notify:delivered`, JSON of `DeliveredLog.Keys()`), and surfaces a single summary toast via the
  existing `uistate.UseNotice()` atom (the one reminder's title, or "N reminders waiting").
- Safety: the whole function is wrapped in `defer recover()` so a notification problem can never break app
  boot. Idempotency is the delivered log (the generators emit "current state" occurrences keyed by
  week/due-date, so they dedupe naturally); no `lastSeenAt` needed for these event types.
- Scope of this first wiring: stale-balance + bill-due (need only accounts). Budget-threshold needs the
  per-period budget-status assembly and the digest needs the period summary — both follow, as does the
  in-app center + Settings rules UI. New `notify.{staleTitle,staleBody,billTitle,billBody,summary}` keys.
  wasm build green, gofmt clean.

## 2026-06-18 — feat: notify.DefaultRules — recommended rule set (B19, step 7)

- Added `notify.DefaultRules()` — one Rule per supported event (bill-due, budget-threshold, stale-balance,
  digest), all enabled, in-app channel, no quiet hours, no frequency cap (the per-event occurrence keys
  already bound firing). Only bill-due carries a Threshold (7-day lead); the others read existing logic
  (freshness windows / budgeting near-over / digest period) so their Threshold stays 0. Pure (notify
  types only, no domain dep), table-tested for count, uniqueness, enabled+in-app, no-quiet-hours, and the
  bill-due lead.
- This is the last pre-wiring pure piece: the wasm shell can now seed rules from `DefaultRules()`, persist
  them + lastSeenAt + the delivered log, run the notifyfeed generators → `notify.CatchUp` on open, and
  surface the results. That wiring (localStorage + in-app center + the budget-status assembly) is the
  remaining focused task. `go vet` clean.

## 2026-06-18 — feat: notifyfeed — digest event evaluator; event set complete (B19, step 6)

- Added `notifyfeed.DigestCandidates(ruleID, periodKey, title, body, now)`: a periodic summary keyed
  `digest@<periodKey>` (caller passes `notify.WeekKey(now)` or `MonthKey(now)`), severity info. The
  summary text is rendered by the caller (figures + i18n in the UI), and an empty title yields no
  candidate. Returned as a one-element slice so it appends uniformly with the other generators.
- That completes the four recommended Phase-A events — stale-balance, budget-threshold, bill-due, digest —
  all as pure, table-tested generators. **The B19 notification logic is now complete end-to-end in pure
  Go** (types, quiet-hours/idempotency, the CatchUp engine, and all four event generators). What remains
  is purely the wasm surface: persist `lastSeenAt`, call CatchUp on open/visibility, and render the in-app
  center + browser Notifications — collision-prone appstate/UI work I'm leaving for a focused pass.
- SW cache rolls to v100. wasm build green, gofmt + go vet clean; full native suite green this iteration.

## 2026-06-18 — feat: notifyfeed — bill-due event evaluator (B19, step 5)

- Added `notifyfeed.BillDueCandidates(ruleID, upcoming, withinDays, now, text)`: from `bills.Upcoming`
  output, emits a `notify.Candidate` per bill due within `withinDays` (non-positive → 7-day default).
  Keyed `<accountID>@<due-date>` so each due occurrence fires once (idempotent across opens) and the next
  cycle's date is a fresh key. Due today/tomorrow → critical, else warning. Title/body via the `text`
  callback. Decision-light: "due soon" is just `bills.Bill.DaysUntil` against the window.
- Table tests cover the window filter, severity by days-until, the due-date occurrence key, and the
  default-window fallback. `go vet` clean. Three of the four recommended Phase-A events now have pure,
  tested generators (stale-balance, budget-threshold, bill-due); only the periodic digest remains, plus
  the wasm wiring (lastSeenAt + in-app center) gated on the broader scope decision.

## 2026-06-18 — feat: notifyfeed — budget-threshold event evaluator (B19, step 4)

- Added `notifyfeed.BudgetCandidates(ruleID, statuses, now, text)`: maps `budgeting.Status` values whose
  `State` is Near or Over into `notify.Candidate`s (Over → critical, Near → warning). Keyed
  `<budgetID>:<state>@<month>` so near and over are *distinct* occurrences — a budget that worsens from
  near to over fires a new, higher-severity alert that same month instead of being silenced by the prior
  near. Title/body via the `text(name, over)` callback (UI keeps i18n). Decision-light: "near/over" is
  exactly budgeting's existing classification, no new thresholds.
- Takes pre-computed statuses (not budgets+txns) so notifyfeed needn't pull the whole ledger — the caller
  already has them. Table tests cover over/near/OK filtering, severities, and the per-state month keys.
  `go vet` clean. Two event generators now feed `notify.CatchUp` (stale-balance, budget-threshold).

## 2026-06-18 — feat: notifyfeed — stale-balance event evaluator (B19, step 3)

- New pure `internal/notifyfeed` (imports notify + domain + freshness; notify stays domain-free).
  `StaleBalanceCandidates(ruleID, accounts, windows, now, text)` runs `freshness.StaleAccounts` and emits
  a `notify.Candidate` per stale account, keyed `<accountID>@<ISO-week>` so it nudges at most weekly
  (idempotent across opens via the delivered log), severity warning, At=now. Title/body come from a `text`
  callback so the strings stay localizable in the UI layer (same decoupling as the report narrative/CSV).
- Chose stale-balance as the first concrete event because it's decision-light: "stale" is exactly
  freshness's existing rule, so no new thresholds to invent. Table tests cover the stale/fresh/archived
  split, the candidate fields, and the weekly occurrence key. `go vet` clean.
- This is the first generator feeding `notify.CatchUp`; budget-threshold and bill-due generators + the
  wasm wiring (lastSeenAt persistence, in-app center) follow once the broader Phase-A event scope is set.

## 2026-06-18 — feat: Bills CSV export (B22)

- Added pure `bills.CSV(bills, amount)` (encoding/csv): header + name, due date (ISO), days until, amount.
  The amount callback receives each bill's `money.Money` so the screen can convert to base before
  formatting. Table-tested incl. a comma-in-name row (quoted by encoding/csv).
- Wired a "Download CSV" button into the Bills list card (shown when there are bills) via `downloadBytes`
  → `bills.csv`, with a per-bill base-currency conversion in the formatter. Completes CSV export across all
  three new Tools screens (Reports, Subscriptions, Bills). New `bills.downloadCsv*` keys; wasm build green.

## 2026-06-18 — feat: Subscriptions CSV export (B25)

- Added pure `subscriptions.CSV(subs, amount)` (encoding/csv): header + name, cadence, charge, normalized
  monthly + annual cost, and next-renewal (ISO date). Amounts via an `amount` callback (plain decimals).
  Table-tested incl. the monthly→annual (×12) and yearly→monthly (÷12) normalizations.
- Wired a "Download CSV" button into the Subscriptions list card (shown when there are subscriptions) via
  the existing `downloadBytes` → `subscriptions.csv`. New `subs.downloadCsv*` keys. Mirrors the Reports
  CSV export for consistency. wasm build green, gofmt clean.

## 2026-06-18 — feat: Reports CSV export (B21)

- Added pure `reports.CategoryCSV(rows, name, amount)` — builds the spending-by-category report as CSV
  bytes via `encoding/csv` (header + Category/Amount/Prior/Change%; blank change when no baseline).
  Decoupled from formatting via `name`/`amount` callbacks; the amount callback renders plain decimals
  (`money.FormatMinor`, no symbol/grouping) so the numbers are spreadsheet-friendly. Table-tested against
  exact rows incl. the blank-change case.
- Wired a "Download CSV" button into the Reports category card (shown when there are rows) that triggers
  the existing `screens.downloadBytes` with `spending-by-category.csv`. New `reports.downloadCsv*` keys.
  wasm build green, gofmt clean.

## 2026-06-18 - feat: add self-host docker quickstart

- Added a server Dockerfile and Docker Compose stack with Caddy TLS termination and a persistent data volume.
- Added `deploy/cashflux-server.env.example` for token-mode self-hosting, billing disabled by default, optional OAuth providers, and server limits.
- Added `docs/SELF_HOSTING.md` and linked it from the README with token setup, local dev, TLS, backup/restore, and upgrade notes.

## 2026-06-18 — feat: Reports top-payees report (B21)

- Added pure `reports.TopPayees(txns, start, end, rates, n)`: groups expenses by description
  (case-insensitive, keeping the first spelling), sums base-currency amounts, ranks largest-first (ties by
  name), returns top n. Mirrors the category-totals conventions (IsExpense, dateutil.InRange, rates.Convert).
  Table tests: case-insensitive merge, range/income/transfer exclusion, and the n limit.
- Wired it into the Reports screen as a "Top payees" card (top 8), shown when there are payees; blank
  descriptions render as "(no description)". Small cohesive report, so core + UI in one commit. New
  `reports.{topPayees,noPayee}` keys. wasm build green, gofmt + go vet clean.

## 2026-06-18 — refactor: dashboard bills widget reuses bills.Upcoming (B22)

- The dashboard "Upcoming bills" widget had its own inline liability→bill derivation (a local struct +
  `dateutil.NextMonthlyDue` + manual sort/top-4). Replaced it with `bills.Upcoming(app.Accounts(), now)`
  so the widget and the Bills screen agree exactly, and the widget picks up the pure core's month-end
  clamping (`dateutil.NextMonthlyDue` didn't clamp). Mapped `Bill.DaysUntil <= 7` to the warn tone and
  `Bill.Amount.Neg()` to the red figure (unchanged rendering). Dropped the now-unused `sort` import.
- Net: less duplicated logic, one source of truth for "what bills are coming up." wasm build green, gofmt
  clean; full native suite was green this iteration too.

## 2026-06-18 — feat: split / settle-up pure core (B24, step 1)

- New pure `internal/split` (no syscall/js, table-tested): `Equal(total, members)` even split with exact
  remainder distribution (1000¢/3 → 334/333/333, shares always sum to total); `Expense` + `NetBalances`
  (credit the payer the total, debit each participant their share → per-member net, sums to zero);
  `SettleUp(balances)` greedy debtor↔creditor matching (descending, ties by id) yielding a small,
  deterministic set of `Transfer`s that fully settle when balances sum to zero.
- Tests: even-split rounding (incl. exact-sum invariant + nil for no members), net-balance computation
  and zero-sum, settle-up correctness (apply transfers → everyone at zero) and the simple two-party chain.
  `go vet` clean. The Split-on-a-transaction action + Settle-up view (UI, ties to members + transfers)
  build on this next.

## 2026-06-18 - fix: force AI proxy through grpc

- Retired the legacy `/v1/ai/key`, `/v1/ai/chat`, and `/v1/ai/vision` HTTP routes from the backend mux.
- Kept AI key upload, model listing, chat, and vision on the authenticated `AIService` gRPC contract over the
  GoGRPCBridge `/grpc` tunnel.
- Added an HTTP regression test that asserts the old `/v1/ai/*` routes return 404, so a stale client path fails
  loudly instead of looking like a supported CORS path.
- Follow-up commit keeps the log/TODO marker with the actual mux removal after the shared worktree moved forward.

## 2026-06-18 - feat: add self-host token rotation command

- Added `server.GenerateAccessToken`, returning a random bearer token plus the SHA-256 digest expected by
  `CASHFLUX_SERVER_TOKEN_SHA256`.
- Added `cashflux-server rotate-token` so self-host operators can generate a replacement token without starting
  the HTTP server.
- Verified the command output shape with `go run ./cmd/cashflux-server rotate-token` and covered digest correctness
  with a server unit test.

## 2026-06-18 - feat: add cloud entitlement seam

- Added `IsCloudActive` as the single server-side seam for future Sync/AI/blob entitlement checks.
- Billing-disabled deployments, including self-host token mode, return active by default so self-host remains
  always-on while Stripe/subscription work is still absent.
- Billing-enabled deployments currently return inactive until subscription state lands, keeping the future gate
  explicit instead of silently allowing a half-wired paid mode.
- Added focused tests for billing-disabled active, missing-user rejection, and billing-enabled inactive behavior.

## 2026-06-18 - feat: harden self-host tokens

- Added `CASHFLUX_SERVER_TOKEN_SHA256` so self-host deployments can store only a SHA-256 digest in config while
  clients still authenticate with the bearer token.
- Token comparison now supports both local-development plaintext tokens and digest-backed tokens using
  constant-time comparisons.
- In token auth mode, when neither token nor digest is configured, startup generates and prints a high-entropy
  one-time token plus instructions to persist it via `CASHFLUX_SERVER_TOKEN_SHA256`.
- Covered digest validation, generated-token config, and hashed bearer authentication in server tests.

## 2026-06-18 - feat: start oauth login

- Added `GET /v1/auth/{provider}` for configured Google/GitHub providers.
- The start endpoint generates a random OAuth `state` plus PKCE verifier, stores them in a short-lived HttpOnly
  SameSite=Lax cookie scoped to the callback path, and redirects to the provider with an S256 challenge.
- Covered the redirect shape and cookie behavior in server tests, including rejection for an unconfigured provider.
- Remaining OAuth work: callback code exchange, user upsert, access/refresh token issue, refresh, and logout.

## 2026-06-18 - feat: configure oauth providers

- Added `OAuthProviderConfig` to backend config, loaded from `CASHFLUX_SERVER_OAUTH_GOOGLE_*` and
  `CASHFLUX_SERVER_OAUTH_GITHUB_*` environment variables.
- Config validation now rejects `oauth` auth mode without at least one complete provider, and rejects partial
  provider configs early at boot.
- `/v1/version` now returns sorted configured `authProviders` and supports CORS/preflight so the wasm settings
  test-connection flow can discover whether a server supports OAuth or token mode.

## 2026-06-18 - feat: list ai models over grpc

- Added `AIService.ListModels` to the shared JSON gRPC contract and dynamic service registration.
- The server returns a deterministic sorted configured allow-list when `CASHFLUX_SERVER_AI_MODELS` is set, and
  otherwise returns the same known model set shown by the app's AI settings picker.
- Extended the real GoGRPCBridge AI integration test to verify `SetKey`, `ListModels`, and `Chat` on one
  authenticated tunnel connection.

## 2026-06-18 - fix: reject oversized sync snapshots

- Added a typed repository size-limit error for snapshot datasets so the transport can distinguish payload limits
  from generic storage failures.
- `SyncService.PutWorkspace` now maps an over-limit dataset to gRPC `ResourceExhausted` instead of leaking a
  generic server error.
- Added a real GoGRPCBridge integration test that sends `defaultSnapshotMaxBytes+1` bytes through
  `PutWorkspace` and verifies the `ResourceExhausted` status.

## 2026-06-18 - feat: subscribe browser to sync watches

- The wasm sync bootstrap now opens a long-lived `WatchWorkspaces` stream through the GoGRPCBridge tunnel when
  backend URL/token preferences are configured.
- Added a persisted browser device id (`cashflux:sync-device-id`) and sends it on workspace pushes so the watch
  loop can ignore same-device echoes.
- Active-workspace watch events from other devices trigger the existing `GetWorkspace` pull path, reusing the
  newest-by-`updatedAt` metadata guard before importing/reloading.
- Moved server watch publication to after the RPC snapshot write so a watch-triggered pull can see the latest
  dataset instead of racing the snapshot store.

## 2026-06-18 - feat: stream workspace sync watches

- Added the `WatchWorkspaces` SyncService RPC to the JSON gRPC bridge contract and registered it as a
  server-streaming method on `/grpc`.
- Added in-process per-user watcher fan-out in `SyncService`; accepted workspace puts and tombstone deletes
  publish workspace events without crossing user boundaries.
- Covered the path with a pure per-user fan-out test and a real websocket bridge integration test that opens
  the watch stream on one connection and receives a put event from another connection.
- Remaining client work: keep a long-lived browser watch open, ignore same-device echoes, and trigger a pull
  or status update when another device changes a workspace.

## 2026-06-18 - feat: sync browser autosave over grpc

- Wired the wasm autosave loop to push changed active-workspace snapshots through `SyncService.PutWorkspace`
  over the `/grpc` GoGRPCBridge tunnel when backend URL/token preferences are configured.
- Added boot/focus pulls via `SyncService.GetWorkspace`; newer server snapshots import into the local store,
  update localStorage, and reload after focus-time applies so mounted views show the server copy.
- Added per-workspace sync metadata (`cashflux:sync-meta:<workspaceID>`) with server `updatedAt`, version, and
  dataset hash so first-run and stale-overwrite decisions are explicit.
- Added pure `internal/syncstate` tests for the remote-apply rule: fresh browsers can accept server data, while
  existing local datasets without sync metadata are not silently overwritten.

## 2026-06-18 - feat: sync dataset snapshots over grpc

- Extended SyncService `PutWorkspace` and `GetWorkspace` RPC envelopes with opaque dataset bytes, matching the
  backend design: the server stores the exported client dataset as a blob without interpreting entities.
- Wired accepted workspace puts into the existing snapshot table with current/history retention, and returns the
  current server snapshot when a stale LWW push is rejected.
- Expanded the real websocket bridge integration test to prove dataset bytes round-trip through Put/Get and stale
  rejection recovery.

## 2026-06-18 - feat: expose sync service over grpc

- Registered `cashflux.v1.SyncService` on the backend gRPC server behind `/grpc`, exposing workspace list, get,
  put, and delete methods over the GoGRPCBridge tunnel.
- Added JSON RPC request/response envelopes for workspace metadata while the `.proto`/generated-code atom remains
  open, keeping the transport real without blocking on local `protoc` setup.
- Made token-backed sync writes idempotently ensure the authenticated user row exists before inserting workspace
  records, avoiding foreign-key failures for first-time token users.
- Added a bridge integration test that opens a real websocket tunnel and verifies put/list/get, stale LWW reject,
  delete tombstone behavior, and post-delete active listing.

## 2026-06-18 - feat: route backend ai over grpc bridge

- Registered `cashflux.v1.AIService` on the backend gRPC server behind `/grpc`, with unary `SetKey`, `Chat`,
  and `Vision` methods using the existing token interceptor and encrypted-key store.
- Moved the wasm backend AI transport and Settings key upload from browser `fetch` calls to
  `syncbridge.Dial(...).Invoke(...)`, so the client reaches the backend through GoGRPCBridge.
- Added a native bridge integration test that opens a real websocket tunnel, invokes `SetKey`, then invokes
  `Chat` against a mock OpenAI upstream to prove the key is stored and reused through gRPC.
- Kept the existing HTTP AI endpoints in place as a compatibility/debug surface; the app client path no longer
  depends on them.

## 2026-06-18 - fix: allow backend ai proxy cors

- Added explicit `OPTIONS` handlers for `/v1/ai/chat` and `/v1/ai/vision`; browser preflight no longer falls
  through without `Access-Control-Allow-Origin`.
- Expanded the shared CORS helper to expose `Content-Length`, `Content-Type`, and `ETag`, and to cache
  successful preflight checks for 10 minutes.
- Verified the allowed-origin preflight path for both AI proxy routes with server tests.

## 2026-06-18 - feat: add backend readiness and graceful shutdown

- Changed `/readyz` from a static liveness response into a real readiness probe backed by the server store:
  it now requires a configured SQLite handle, a successful ping, and a readable schema version.
- Kept `/healthz` as the cheap process-liveness check, so orchestration can distinguish "process is alive"
  from "backend dependencies are ready to serve traffic."
- Reworked `cmd/cashflux-server` to run an explicit `http.Server` and call `Shutdown` on interrupt/SIGTERM,
  giving HTTP, websocket bridge, and in-flight requests a drain window before the process exits.
- Verified the new readiness behavior with server tests for healthy stores, missing stores, and closed stores.

## 2026-06-18 - docs: document backend auth handshake

- Updated `docs/BACKEND_PLAN.md` from plan-only language to backend-foundation-in-progress and documented the
  concrete transport split: HTTP keeps the configured backend base URL, while gRPC converts it to the `/grpc`
  ws/wss bridge target.
- Documented the bearer-token contract shared by HTTP and gRPC: `Authorization: Bearer <token>` for HTTP,
  `authorization: Bearer <token>` metadata for unary/stream RPCs, and the current token-mode server validation.
- Brought the artifact endpoint docs in line with the implemented `PUT`/`GET`/`HEAD /v1/blobs/:hash` shape.

## 2026-06-18 - feat: add authenticated blob endpoints

- Added bearer-protected `PUT`, `GET`, and `HEAD /v1/blobs/{hash}` routes. Uploads are capped by
  `CASHFLUX_SERVER_BLOB_MAX_BYTES`, must hash to the URL `:hash`, and are stored through the existing
  content-addressed blob repository under the server data dir.
- Downloads return the stored MIME type, content length, ETag, and immutable cache headers. CORS now allows the
  blob methods alongside the existing AI proxy methods.
- Verified upload/download/head, missing auth, oversized uploads, hash mismatch, invalid hashes, CORS preflight,
  and negative blob-limit config with server tests. Workspace blob linking remains for the sync/artifact atom.

## 2026-06-18 - feat: add client grpc bridge transport

- Added `internal/syncbridge`, a small client transport layer that turns the configured backend HTTP(S) URL into
  the `/grpc` ws/wss tunnel URL and opens it through `grpctunnel.BuildTunnelConn`.
- Added unary and stream client interceptors that attach `authorization: Bearer <token>` metadata, matching the
  server auth interceptor contract. Sync RPC wiring remains the next 7.7 atom.
- Verified URL normalization, required-token validation, and metadata injection with native tests.

## 2026-06-18 - feat: mount backend gRPC bridge

- Mounted GoGRPCBridge at `/grpc` around the backend `grpc.Server`, with the configured SPA origin check,
  keepalive/idle timers, read limit, active/per-client connection caps, and per-client upgrade-rate cap.
- Shared the token auth derivation between HTTP bearer calls and gRPC metadata validation so both surfaces map
  a server token to the same authenticated user id.
- Verified with focused server tests for config validation, endpoint mount/origin rejection, and token mapping.

## 2026-06-18 - test: pin backend AI cancellation

- Added a cancel-aware mock HTTP client around `server.AIService.Chat` to prove the request context reaches the
  upstream OpenAI call and that canceling the client context returns a gRPC `Canceled` error.
- This closes the cancellation-propagation checklist item for the current HTTP proxy path; streaming chunk cancel
  behavior remains tied to the later gRPC/server-streaming transport.

## 2026-06-18 - feat: route AI screens through backend proxy

- Added a wasm backend AI transport in `internal/ai` for `/v1/ai/chat` and `/v1/ai/vision`, with the same
  callback/cancel shape as the direct OpenAI transport.
- Insights, Allocate, and Documents now prefer the backend URL/token saved in Settings; direct browser OpenAI calls
  remain as the local-only fallback when no backend token is configured.
- Verified pure proxy request builders, full native tests, wasm build, and server build. Final gRPC/server-streaming
  client replacement remains a backend TODO.

## 2026-06-18 — feat: Bills month-calendar view (B22)

- Rendered the calendar on the Bills screen from `bills.MonthCalendar(upcoming, year, month, weekStart)`:
  a CSS `.cal-grid` (7 columns) with weekday headers, a cell per day (out-of-month dimmed, today outlined
  via `.today`), and a `.cal-dot` on days with bills due (the dot's `title` lists the bill names). Built
  the children as a `[]any` accumulator (`Div(args...)`) since the shorthand funcs take `...any` and a
  `[]ui.Node` can't be spread directly.
- Added `.cal-*` CSS to index.html and a `bills.calendar` ("June 2026 calendar") i18n key. Shown only when
  there are upcoming bills. Completes B22's headline calendar view; mark-paid (State layer) remains the
  one deferred piece. wasm build green, gofmt clean.

## 2026-06-18 — feat: bills month-calendar layout helper (B22)

- Added `bills.MonthCalendar(bills, year, month, weekStart) [][]CalendarDay` — the pure month-grid layout
  the calendar UI needs. Computes the week-start offset, ceils to whole weeks (`(offset+daysInMonth+6)/7`),
  pads leading/trailing cells from the adjacent months (`InMonth=false`, never carrying bills), and buckets
  each bill onto its due day via a date-only key. Reuses the existing `daysInMonth`.
- Tests: grid shape (7/row, first cell = weekStart on/before the 1st), Monday vs Sunday week-start, the 1st
  appears once and in-month, bill placement on the right cell, and that out-of-grid / out-of-month days
  carry no bills. `go vet` clean. The calendar view (rendering this grid with due-day dots) is next.

## 2026-06-18 — feat: bill payment reminder → to-do (B22)

- Added a **Remind me** action to each bill row (extracted a `BillRow` component owning its click hook).
  It creates a `domain.Task` ("Pay bill: <name>", priority medium, source nudge) due on the bill's due
  date via `app.PutTask`, then toasts through `uistate.UseNotice()`. Same pattern as the subscription
  reminder; reuses the existing to-do system.
- Deferred mark-paid: per the spec it "creates/links a transaction" + persists paid-this-cycle status —
  that's the State layer (a new store field) plus liability-payment accounting, too much to infer safely
  here. The reminder gives immediate value via to-do without those decisions. New `bills.{remind,
  remindTitle,reminderTitle,reminderNote,reminderAdded}` keys. wasm build green, gofmt clean.

## 2026-06-18 — feat: Bills rail icon (B22)

- Added a Lucide calendar `icon.Bills` and wired `/bills` into `railMeta` with the `nav.bills` key, so the
  Bills screen shows a calendar glyph instead of the neutral page icon. Updated the icon curated-set test
  (18→19). Native icon test green, wasm build green.

## 2026-06-18 - feat: guard backend AI proxy calls

- Added AI proxy abuse guards to `server.AIService`: configured model allow-list, request JSON size cap, and
  per-user daily request/token ceilings. The guards run before loading the encrypted OpenAI key or calling the
  upstream provider, so rejected requests do not expose secrets or spend tokens.
- Added env-backed config: `CASHFLUX_SERVER_AI_MODELS`, `CASHFLUX_SERVER_AI_REQUEST_MAX_BYTES`,
  `CASHFLUX_SERVER_AI_REQUESTS_PER_DAY`, and `CASHFLUX_SERVER_AI_TOKENS_PER_DAY`.
- Tests cover disallowed models, oversized requests, daily request/token caps, and the HTTP endpoint mapping.

## 2026-06-18 — feat: Bills screen — wire the B22 core into the UI (B22, step 2)

- New `screens.Bills()` registered as `/bills` in the Tools group: runs `bills.Upcoming(accounts, now)`,
  converts each min-payment to base currency, and renders a stat grid (total due soon, count, next due
  date) plus a per-bill list (name, due date + friendly days-until via `daysUntilLabel`, amount). Rows are
  plain text (no hooks), so the loop is safe.
- Followed the parallel session's screens.go refactor: Label/Title/Subtitle now hold **i18n keys** (not
  English), so the route uses `nav.bills` / `screen.billsSub`, and I added those plus `bills.*` keys. The
  rail auto-shows it via navGroup (B7) with the neutral icon for now (dedicated icon is a quick follow-up).
- Deferred: month calendar view + mark-paid (logs a payment) + Planning-recurring-derived bills. wasm
  build green, gofmt clean.

## 2026-06-18 — feat: bills tracker — pure core (B22, step 1)

- New pure `internal/bills` (no syscall/js, table-tested), derived-first per the spec's recommendation:
  `Upcoming(accounts, now)` returns the next bill for each active liability account that has a
  `DueDayOfMonth` and a non-zero `MinPayment` — amount, next due date, and days-until — soonest first.
  `NextDue(dueDay, from)` finds the next occurrence on/after `from`'s date with month-end clamping
  (`daysInMonth` via the day-0-of-next-month trick), so a 31st-due bill lands on Feb 28/29 and rolls the
  year correctly. Assets, archived, and no-due-day / no-min-payment accounts are skipped.
- Tests: NextDue (this-month / today / passed→next / Feb clamp non-leap+leap / 31-day month / year
  rollover) and Upcoming (filtering, ordering, amount, days-until). `go vet` clean.
- Deferred to later steps: Planning-recurring-derived bills, paid-this-cycle status + mark-paid (the State
  layer), and the Bills screen + month calendar (UI). Pure, not yet wired.

## 2026-06-18 — feat: subscription cancel-reminder → to-do (B25)

- Added the B25 "cancel reminder → task" action. Extracted a `SubscriptionRow` component (owns its click
  hook per the On*-hooks-in-loops rule) with a **Remind me** button; the list now renders via `MapKeyed`
  (key = name + amount). The handler creates a `domain.Task` (status open, priority medium, source nudge)
  titled "Review subscription: <name>", due on the subscription's `NextRenewal`, via `app.PutTask`, then
  toasts a confirmation through the shared `uistate.UseNotice()` atom. No B19 dependency — reuses the
  existing to-do system.
- New `subs.{remind,remindTitle,reminderTitle,reminderNote,reminderAdded}` i18n keys. wasm build green,
  gofmt clean.

## 2026-06-18 — feat: Reports net-worth trend chart (B21)

- Added a second sparkline to the Reports screen: net worth as of each of the same `bounds` boundaries
  used for the cash-flow trend, via the existing `ledger.NetWorthSeries(accounts, txns, bounds, rates)`
  (cumulative running total, not per-period flow). Rendered with `uiw.AreaChart` in an accent stroke and
  its own gradient id, shown only when there are ≥2 points. Reused the `dashboard.netWorthTrend` label.
  No new logic — pure reuse of the ledger series + the chart shim. wasm build green, gofmt clean.

## 2026-06-18 — feat: Subscriptions rail icon (B25, step 3)

- Added a Lucide "repeat" `icon.Subscriptions` and wired `/subscriptions` into `railMeta` with the
  `nav.subscriptions` key, so the screen shows a recurring-cycle glyph instead of the neutral page icon.
  Updated the icon curated-set test (17→18). gofmt realigned the (now wider) `railMeta` and icon maps.
  Native icon test green, wasm build green.

## 2026-06-18 — chore: route screen registry through i18n (copy pass, file 4)

- The deferred `internal/screens/screens.go` registry: every Route hardcoded Label, Title, and Subtitle in
  English, displayed raw by the shell (page heading + subline) — the last real i18n gap in the UI.
- Design: the registry now carries **keys, not English**. Label/Title reuse the existing `nav.*` keys (the page
  title equals the nav label for every screen); Subtitle uses new `screen.*Sub` keys. The shell already resolves
  rail labels via `T()`, so only the Shell-title path needed wrapping: `app.go` now passes
  `uistate.T(route.Title)` / `uistate.T(route.Subtitle)` at all three Shell construction sites, and the
  custom-page fallback title ("Page") became `custompage.fallbackTitle`.
- Verified: 18 `screen.*Sub` + 1 fallback key added; `nav.artifacts`/`nav.workflows` already existed; registry
  has no display English; builds under `GOOS=js GOARCH=wasm`. Copy kept verbatim (already high quality).
- This closes the UI i18n sweep: remaining hardcoded strings are AI system prompts (English by design) and the
  "CashFlux" brand wordmark (handled by the rebrand backlog §7, not i18n).

## 2026-06-18 — chore: route dashboard empty states through i18n (copy pass, file 3)

- `internal/screens/dashboard.go` had three hardcoded display strings: the "App state is not ready yet."
  fallback, "No upcoming bills.", and "Nothing near or over budget." The first now reuses the existing shared
  `common.notReady` key (no duplicate); the other two got new `dashboard.noUpcomingBills` /
  `dashboard.noBudgetAlerts` keys, with the budget one nudged friendlier ("Nothing's near or over budget.").
- Scan note: insights.go / allocate.go "hardcoded" strings are AI **system prompts** (prompt engineering), not
  display copy — intentionally left in English; localizing them would degrade AI output. The rest of the UI is
  already i18n'd. Remaining genuine gap is the `screens.go` route registry (deferred, needs a key-based refactor).

## 2026-06-18 — chore: route lock-screen greeting through i18n (copy pass, file 2)

- `internal/app/applockgate.go` set the lock-screen greeting from hardcoded English ("Good evening" default,
  "Good morning"/"Good afternoon" by hour). Added `applock.greetingMorning/Afternoon/Evening` to en.go and
  resolved them via `uistate.T`, so the greeting localizes with the rest of the lock screen. Builds under
  `GOOS=js GOARCH=wasm`.
- Deferred `internal/screens/screens.go`: its route registry hardcodes every screen's Label/Title/Subtitle and
  is consumed in several places (app.go Shell props, shell.go rail) — needs a registry-level i18n refactor
  (store keys, resolve centrally), not a quick literal swap. Flagged for a dedicated pass.

## 2026-06-18 — chore: route settings.go strings through i18n (copy pass, file 1)

- First file in the "all text through i18n + improve copy" sweep. `internal/app/settings.go` had ~16 hardcoded
  English strings outside the language store: every export/import/load-sample/wipe `notify(...)` toast, the wipe
  `confirmAction` text, the FX-rate row label (`"1 USD ="`), and the freshness unit hint (`"days (0 = never)"`).
- Added `settings.*` keys to `internal/i18n/en.go` and replaced the literals with `uistate.T(...)`. Copy
  improvements: success toasts now take the filename as a `%s` arg (one place, rebrand-friendly), and the
  freshness hint reads "days · 0 means never" (clearer than the parenthetical). Error toasts keep the existing
  "Couldn't … : %s" voice for consistency.
- Note: the en.go additions were picked up by the other session's commit; this commit lands the settings.go
  half. Both packages build under `GOOS=js GOARCH=wasm`. Next file in the sweep: the next-highest hardcoded-copy
  view (e.g. `internal/screens/screens.go`).

## 2026-06-18 — feat: Subscriptions screen — wire the B25 core into the UI (B25, step 2)

- New `screens.Subscriptions()` registered as `/subscriptions` in the Tools group: runs
  `subscriptions.Detect(txns, rates, 2)` and renders a stat grid (monthly burden via `MonthlyTotal`, yearly
  via summed `AnnualAmount`, active count) plus a per-subscription list (name, cadence + next-renewal meta,
  normalized $/mo, and the charge). Rows are plain text (no On* hooks), so the loop is safe.
- Gotcha: `cadenceLabel` already exists in planning.go (for `domain.RecurringCadence`) — renamed mine to
  `subscriptionCadenceLabel` to avoid the package-level redeclaration. Added `subs.*` + `nav.subscriptions`
  i18n keys. Left the rail icon as the neutral fallback for now (a dedicated icon is an easy follow-up,
  like Reports). wasm build green, gofmt clean.

## 2026-06-18 — feat: subscriptions detection — pure core (B25, step 1)

- New pure `internal/subscriptions` (no syscall/js, table-tested): `Detect(txns, rates, minCount)` finds
  recurring charges. It considers non-transfer expenses, converts to base currency, groups by normalized
  description + identical converted amount, and infers a cadence from the **median** gap between dates
  (weekly 6–8d, monthly 26–33d, yearly 350–380d; anything else is not a subscription). Each result carries
  count, last date, next-renewal (last + one interval), and `MonthlyAmount`/`AnnualAmount` normalizers;
  `MonthlyTotal` sums the burden. Sorted by monthly cost.
- Used the median (not mean) gap so a single off-cycle charge doesn't break detection, and required ≥2
  occurrences (one gap) minimum. Tests: monthly/weekly/yearly classification, irregular + sparse + income
  exclusion, ordering, and the monthly total. `go vet` clean; pure, not yet wired (UI is the next step).
- B25 is a "SPEC" item but the detection algorithm is unambiguous and the standing directive is to
  implement features; built bottom-up so the core is reusable regardless of the eventual UI/scope.

## 2026-06-18 — feat: Reports rail icon (B21, step 7)

- Reports already auto-appeared in the rail (navGroup shows any registered Tools screen, B7) but with the
  neutral Page icon. Added a dedicated Lucide-style bar-chart `icon.Reports`, wired `"/reports"` into
  `railMeta` with the `nav.reports` key. Updated the icon package's curated-set test (16→17). Native icon
  test green, wasm build green, gofmt clean.

## 2026-06-18 — feat: Reports cash-flow trend chart (B21, step 6)

- Added a cash-flow trend to the Reports screen using the existing pure `ui.AreaChart` sparkline (no D3
  dependency). Builds `trendBuckets` (6) consecutive period bounds ending at the viewed period via
  `period.Truncate` + `period.Step` on the current resolution, feeds them to
  `reports.IncomeExpenseSeries`, and plots each bucket's `Net()`. Shown only when there are ≥2 points.
- Note on aliases: in `screens` the framework hooks are `ui` and the internal design system is `uiw`
  (matching dashboard.go), so the chart is `uiw.AreaChart`. New `reports.trendHint` i18n key (formatted
  with the bucket count). wasm build green, gofmt clean.

## 2026-06-18 - feat: proxy AI calls through backend

- Added `server.AIService` for backend OpenAI calls: it authenticates via context, decrypts the user's stored
  OpenAI key from SQLite with the server master key, reuses the existing chat/vision request builders, maps
  upstream HTTP errors to status codes, and records request/token usage.
- Wired `/v1/ai/chat` and `/v1/ai/vision` into the HTTP backend behind the same bearer-token, master-key, and
  CORS checks as key upload. `CASHFLUX_SERVER_OPENAI_BASE_URL` lets tests and dev runs point at a mock upstream.
- This is the secure proxy call path, not the final gRPC/SSE streaming surface; the remaining 7.4 work is true
  streaming chunks, model allow-list/rate limits, and cancellation polish.

## 2026-06-18 — feat: Reports screen — wire the reports core into the UI (B21, step 5)

- The UI-last step for the first reports: a new `screens.Reports()` (`reports_screen.go`) registered as
  `/reports` in the Tools group. It reads the shared top-bar period window (`uistate.UsePeriod().Get()`,
  `.Range()` for the period and `.Shift(-1).Range()` for the prior comparison), then renders
  `reports.IncomeVsExpense` as a stat grid (income/spending/net/savings-rate), `reports.SpendingNarrative`
  as a plain-English paragraph, and `reports.SpendingByCategory(compare=true)` as a category list with a
  green ▼ / red ▲ percent-change badge. Category rows are plain text (no On* hooks), so the loop is safe.
- Reused existing helpers (`fmtMoney`, `stat`, `accentFor`, the `text-up/down` tones) and labels
  (`dashboard.income/spending/savingsRate`); added `reports.{net,byCategory,empty,uncategorized}`. No
  charts yet (follow-up). Surgical one-line route registration to keep the shared `screens.go` merge-safe.
  wasm build green, gofmt clean, full native suite still green.

## 2026-06-18 — feat: reports — top movers + narrative DRY (B21, step 4)

- Added `reports.TopMovers(rows, n)`: ranks compared categories by absolute change vs the prior period
  (largest first, ties by id, n<=0 = all), excluding unchanged/no-delta rows — the "top movers" catalog
  item. Refactored the narrative's `topMover` to delegate to it so the ranking rule lives in one place.
- Tests cover ranking, tie-break determinism, exclusion of unchanged/no-delta rows, the n limit, and the
  empty case. `go vet` clean. The reports engine now covers spending-by-category, cash-flow, top-movers,
  and narrative as pure tested cores; the Reports screen/charts are the remaining (UI-last) piece.

## 2026-06-18 — feat: reports — deterministic spending narrative (B21, step 3)

- Added `reports.SpendingNarrative(rows, compared, format, name)` — the "narrative descriptions" piece of
  B21, template-based (no AI) so identical numbers always yield identical text. Headline (total + category
  count with correct singular/plural), biggest expense, and — when comparing — the single biggest mover
  vs the prior period (`topMover` = largest absolute change, with rose/fell + signed %). Stays decoupled
  via `format`/`name` callbacks (empty/unknown name → "uncategorized"). Zero-spend (incl. all-dropped-to-
  zero movers) reads "No spending in this period."
- Table tests: empty, no-comparison headline+biggest, singular + uncategorized, and top-mover selection
  (a drop-to-zero beating a smaller increase). `go vet` clean; pure, not yet wired.

## 2026-06-18 — feat: reports — income-vs-expense / cash-flow (B21, step 2)

- Added `internal/reports/cashflow.go`: `PeriodFlow` (income/expense + `Net()` + `SavingsRate()` via the
  shared `ledger.SavingsRate`), `IncomeVsExpense` for one period, and `IncomeExpenseSeries` for the
  cash-flow trend across `bounds` buckets. Reuses `ledger.PeriodTotals` so the numbers match the
  dashboard exactly (transfers excluded, base-currency converted). Table tests cover the single-period
  totals/net/savings-rate, the multi-bucket series (incl. a net-negative month), and the too-few-bounds
  guard. `go vet` clean, pure package, not yet wired.

## 2026-06-18 — feat: reports engine — pure core (B21, step 1)

- Started the approved B21 reports engine bottom-up per the SDLC: a pure `internal/reports` (no
  syscall/js, table-tested). First report `SpendingByCategory(txns, start, end, compare, priorStart,
  priorEnd, rates)` → `[]CategorySpend` sorted largest-first, with optional prior-period comparison
  (prior amount + percent change via `ledger.PercentChange`). Built on the existing ledger conventions:
  base-currency conversion through `currency.Rates`, transfers/income excluded via `domain.IsExpense`,
  `dateutil.InRange` bounds. Took the union of current+prior category ids so a category that fell to zero
  this period still surfaces as a top mover. Added a `Total` helper for the headline figure.
- Tests cover sorting, range/income/transfer exclusion, the headline total, and the comparison cases
  (increase %, drop-to-zero -100%, new category with no baseline → HasDelta false). `go vet` clean. Pure
  package, not yet imported — no behavior change; the Reports nav/screen + richer chart kinds come next.

## 2026-06-18 - feat: upload OpenAI key to backend

- Connected the wasm Settings panel to the local backend with device-local backend URL/token prefs and an
  "Upload key to backend" action for the current OpenAI key.
- Added `/v1/ai/key`: it accepts localhost/CASHFLUX_SERVER_APP_ORIGIN CORS, requires bearer auth and
  `CASHFLUX_SERVER_MASTER_KEY`, hashes the bearer into a server user, and stores the OpenAI key via the existing
  AES-GCM `ai_keys` repository path without ever returning the secret.
- Verified native tests, wasm build, server build, full `go test ./...`, live POST to `127.0.0.1:8081`, and that
  the dev SQLite file does not contain the uploaded test key plaintext.

## 2026-06-18 — feat: notify catch-up engine (B19 Phase A, step 2)

- Added `notify.CatchUp(rules, candidates, now, log) → []Notification` — the event-agnostic heart of
  catch-up-on-wake. A `Candidate` is a potential occurrence the (future) per-event evaluators emit;
  CatchUp gates by rule (exists + enabled + ≥1 channel), drops already-delivered occurrences (the
  DeliveredLog), and applies each rule's `FrequencyCap` (keep the most recent N, collapse the rest), then
  marks *every* fresh occurrence considered — including capped-out ones — as delivered so a long gap
  collapses and never replays. Results are newest-first with a key tiebreaker for determinism.
- Design call: quiet hours are NOT applied in CatchUp — a summary shown when the user actively opens the
  app isn't an interruption; quiet hours gate live in-session firing (`Rule.CanFireAt`). Documented in
  the func.
- Keeping the engine candidate-driven means the still-open "which events ship first" decision lives in
  the per-event candidate generators, not here — so this layer is buildable and testable now. Table tests
  cover gating, idempotency (re-run → empty), frequency-cap collapse, newest-first order, and nil-log
  safety. `go vet` clean. Still no UI / not yet wired into the shell.

## 2026-06-18 — feat: notify package — pure notification core (B19 Phase A, step 1)

- Started the approved B19 Phase A (client-only notifications) bottom-up per the SDLC: model + logic +
  tests first, no UI. New pure `internal/notify`:
  - Types: `Channel` (in-app/browser; email/SMS are deferred Phase B and absent here), `Event` (the
    recommended first slice — bill-due, budget-threshold, goal-milestone, stale-balance, large-txn,
    digest), `Severity`, `Rule` (enabled + channels + threshold + quiet hours + frequency cap),
    `Notification`.
  - Logic: `Rule.HasChannel`, `Rule.InQuietHours` (half-open window, handles past-midnight wrap),
    `Rule.CanFireAt` (enabled ∧ has-channel ∧ not-quiet), `DedupeKey` + `Day/Week/MonthKey` occurrence
    keys, and a `DeliveredLog` map for idempotent catch-up. All table-tested; `go vet` clean.
- Deliberately did NOT build event-specific evaluation or `CatchUp` yet — the TODO leaves "which events
  ship first / is cost-tracking in scope for Phase A" open to confirm, so I kept this first step to the
  event-agnostic core (types + gating + idempotency) that those decisions don't affect. Not yet imported
  by the wasm shell, so no behavior change; bumped SW cache for ritual consistency only.

## 2026-06-18 — a11y: label the top-bar "Jump to…" select (C47 tail)

- Closed the last concrete C47 gap in the app chrome: the resolution control's "Jump to…" `<select>`
  (shell.go) had only a `title`, no programmatic name. Added `aria-label`. Audited the rest of the top
  bar — the segmented Week/Month/Quarter control and the from/to steppers are labeled buttons, and the
  jump-to select was the only bare control — so the dashboard chrome is now clean.
- While here, confirmed C41's "one-tap This period reset" (asked for in #64) already exists in the
  resolution control (shown when the window isn't the current period), and that C34's overflow fix
  already un-clips the "+ Add" menu (C43's remaining z-index portal is a separate, larger refactor —
  left for now). Build green.

## 2026-06-18 — fix: resolution switch re-anchors to now, not the window start (C41)

- `Window.SetResolution(r)` re-snapped `w.From`/`w.To` (the existing window start, which is ≤ now) to the
  new unit, so each switch drifted backward: Month "Jun" → Week gave June's first week (May 31–Jun 6),
  Week → Month gave May, Week → Quarter gave Q1 — compounding into the past.
- Changed it to `SetResolution(r, now)` returning `NewWindow(r, now, weekStart)` — the single period that
  contains now (matching the "This period" preset that #64 confirmed works). Only caller is shell.go's
  resolution control (passes `time.Now()`). Rewrote `TestSetResolutionResnaps` → `…ReanchorsToNow`,
  asserting every Week/Month/Quarter switch yields a single window whose half-open range contains now.
- The period engine itself was already correct (per C40/#61); this was purely the re-anchor policy on a
  resolution change. Native period tests pass; wasm build green. (C40 — Budgets-screen SPENT under
  Quarter — is a separate, still-open bug.)

## 2026-06-18 — fix: workflow Save folds in a pending action (C37)

- Root cause of the "Save workflow is a no-op" report: the action builder stages an action only on *Add
  action*; if the user typed an action and went straight to *Save*, `actions` was empty, `Validate`
  returned "Add at least one action", and the typed action was discarded. (Validation feedback already
  existed via the `msg`/`role=alert` line — the report predated it — but the silent loss was the real bug.)
- Extracted `buildDraft()` (draft fields → Action + ok), used by both *Add action* (now refuses an empty
  draft with a message instead of staging a blank) and *Save* (folds a valid pending draft into the saved
  actions). Also reset the draft fields after a successful save. Added `workflows.actionNeedsValue` key.
- Left Enter-to-submit out: it's a multi-step builder (Enter in the action field should mean "add action",
  not "save"), and the Save button is already Tab+Enter reachable. Build green, gofmt clean.

## 2026-06-18 — a11y: label planning + settings controls — C47 complete

- Final C47 batch: the planning recurring-item selects (cadence/account/category) and all six settings
  selects (base currency, budget method, AI model, display scale, date format, language). Settings rows
  use a `set-label` *div* (not a `<label for>`), so the selects were programmatically nameless despite the
  visible text — added `aria-label` from the same key (kept the visible div as the visual label; no double
  announcement since the div isn't associated).
- Dashboard's audit-reported "2" controls: the dashboard's interactive chrome (resolution control) is
  segmented buttons + steppers that already carry names; no bare select/date input remained in
  dashboard.go — treating C47 as complete for the form-control gap. Build green, gofmt clean.

## 2026-06-18 — a11y: label budgets/goals/accounts form controls (C47, batch 2)

- Extended the C47 aria-label pass to the budgets, goals, and accounts add + inline-edit forms: category,
  owner, period, account-type, linked-account selects and the target-date / lock-until date inputs.
  Reused existing label/tooltip keys where present; added `budgets.categoryLabel`, `goals.dateLabel`,
  `accounts.typeLabel`.
- Remaining C47 screens: planning (projection/plan selects), dashboard (2 controls), settings (1). Build
  green, gofmt clean.

## 2026-06-18 — a11y: label transactions form controls (C47, batch 1)

- The C47 audit flagged ~12 unlabeled controls on /transactions (the worst offender). Added `aria-label`
  to every bare `<select>` and `<input type=date>` in the add form, the filter/sort bar, the bulk-action
  row, and the inline-edit row. Where a `Title` tooltip existed I reused its text for the label (and kept
  the tooltip); added `transactions.{kindLabel,categoryLabel,dateLabel,filterAccount,filterCategory}`
  i18n keys for the rest.
- Note: `<select>` can't use a placeholder as a name, so aria-label is the right tool here. Remaining C47
  screens (budgets, planning, accounts, goals, dashboard, settings) follow in subsequent commits. Build
  green, gofmt clean.

## 2026-06-18 — feat: dashboard tiles drill into their data screen (C30)

- Tiles had no click behavior (no href/role, cursor:auto). Made each tile's **title** a navigation link
  to the screen that owns its data. Chose the title-as-link over whole-body click (the TODO's open
  decision): whole-body would fight the tile's HTML5 drag/resize; the title is unambiguous, keyboard-
  activatable, and visually distinct from the grip and gear.
- Implemented centrally in the `ui.widget` shell via a `widgetRoute(id)` map + a `viewTitle` sub-component
  (its own click hook, like `gearButton`, per the On*-hooks-in-loops rule) — one file, instead of
  threading an `OnView` prop through all ~18 `ui.Widget(...)` call sites in dashboard.go.
- `ui` now imports `GoWebComponents/router` and calls `router.Navigate`. Custom-page tiles use a separate
  `customTile` header and are unaffected. CSS: `.wh-title` shares the h3 font, adds pointer + hover
  underline. New `widget.open` / `widget.openNamed` i18n keys. Build green, gofmt clean.
- Deferred (noted in C30): deep-linking with a pre-applied filter (e.g. Spending → Transactions filtered
  to expenses for the period) — needs the transactions screen to accept filter query params.

## 2026-06-18 — fix: top bar wraps instead of scrolling (C34)

- `.topbar` had `overflow-x: auto`, so at the awkward ~1000–1100px band (esp. Custom-range mode, which
  adds two date steppers) `scrollWidth > clientWidth` turned the fixed-height header into a scroll
  container. Replaced `overflow-x:auto` with `flex-wrap: wrap; row-gap; min-height:3.5rem`, added
  `.topbar-controls { flex-wrap: wrap }`, and dropped the fixed `h-14` utility in `shell.go` (CSS
  `min-height` now governs) so a wrapped second row isn't clipped.
- Chose wrap over hide-the-scrollbar (the C31 trick): a horizontal hidden-scroll header would leave
  controls off-screen with no indicator. Wrapping is the better UX and matches the existing <1000px
  media query, just extended to all widths. CSS + one class change; build green.

## 2026-06-18 - feat: add backend sync LWW puts

- Added the `SyncService.PutWorkspace` path for last-write-wins workspace updates: it scopes writes to
  `AuthUser.ID`, rejects cross-user ID takeover, server-stamps `updatedAt`, bumps versions, and returns current
  state on stale rejection so clients can re-pull.
- Covered initial create, stale reject, fresh accept, force override, version increments, and cross-user collision
  protection with native server tests.

## 2026-06-18 — feat: empty-state CTAs for transactions + categories (§6.5, batch 2)

- Extended `EmptyStateCTA` to transactions (truly-empty case only — the filtered no-match case keeps a
  plain line, since adding wouldn't help) and to both category lists (expense/income), each focusing the
  shared add-name field (`txn-add`, `cat-add`). New `transactions.addFirst` and
  `categories.addFirst{Expense,Income}` i18n keys.
- That covers the meaningful "add your first" empty states. Remaining bare empties (artifacts, custom
  fields, workflows, planning, documents history, allocate) are either tool outputs or have no single
  add-form to jump to — left as plain lines intentionally. Build green, gofmt clean.

## 2026-06-18 — feat: empty-state call-to-action blocks (§6.5, batch 1)

- Empty lists were a bare `P(Class("empty"), …)` line. Added a reusable `EmptyStateCTA` component
  (`internal/screens/emptystate.go`): centered message + a primary button that calls `focusByID` on the
  screen's add form (reusing the §6.7 plumbing). Add-form first inputs got stable ids (`goal-add`,
  `budget-add`, `task-add`, `member-add`, `rule-add`).
- Made it a component (not a helper returning a node with an inline OnClick) so the click-handler hook
  lives inside its own lifecycle — safe to mount/unmount as the list toggles empty↔non-empty, rather than
  conditionally registering a hook in the screen body.
- Added `.empty-cta` CSS (flex column, centered) in index.html; new i18n `*.addFirst` keys.
- Batch 1: goals, budgets, to-do, members, rules. Next: transactions (truly-empty vs filtered no-match),
  categories (expense/income), accounts (sample loader already covers the welcome state). Build green.

## 2026-06-18 — feat: focus-on-edit for todo/rules/documents/custom pages — §6.7 complete

- Final batch of the §6.7 focus-on-edit feature: `TaskRow`, `RuleRow`, `DraftRow`, and the custom-page
  `editWidgetForm`. Same open/closed-keyed `ui.UseEffect` → `focusByID` on the first input.
- `DraftRow` has no stable entity id (it's an `extract.Row`), so it keys the field by list index
  (`draft-edit-<i>`); added a `strconv` import for that.
- `editWidgetForm` mounts only while editing, so it focuses on mount with a stable `w.ID` dep (runs once).
- §6.7 now covers every inline editor in the app. Build green, gofmt clean.

## 2026-06-18 — feat: focus-on-edit for categories + members (§6.7 cont.)

- Same pattern applied to `CategoryRow` and `MemberRow`: open/closed-keyed `ui.UseEffect` → `focusByID`
  on the first input (name), which now carries a stable `Attr("id", ...)`.
- Remaining inline editors: todo, rules, documents, custom pages. Build green, gofmt clean.

## 2026-06-18 — feat: focus-on-edit for transactions + budgets (§6.7 cont.)

- Extended the §6.7 focus-on-edit pattern to `TransactionRow` and `BudgetRow`: a `ui.UseEffect` keyed on
  the editor's open/closed state calls `focusByID` on the first input (desc / name), which now carries a
  stable `Attr("id", ...)`. Used a plain "open"/"closed" string dep key (no `fmt` dependency added).
- Remaining inline editors: categories, members, todo, rules, documents, custom pages. Build green.

## 2026-06-18 - feat: add scoped backend sync helpers

- Added a `SyncService` wrapper over the backend repository for the first RPC-shaped workspace operations:
  authenticated `List`, `Get`, and soft-delete `Delete`, all scoped exclusively to `AuthUser.ID`.
- Covered per-user list/get isolation, cross-user delete no-op behavior, own tombstone propagation, and
  unauthenticated / invalid workspace-id errors with native server tests.

## 2026-06-18 - feat: add backend rpc auth middleware

- Added reusable gRPC auth middleware for the upcoming SyncService and AIService: bearer token extraction
  from incoming metadata, a validator hook, authenticated-user context helpers, and unary/stream interceptors.
- Covered missing/malformed auth, invalid-token rejection, unary context propagation, and stream context wrapping
  with native server tests.

## 2026-06-18 - feat: add backend usage counters

- Added `usage` repository helpers for per-user UTC-day request/token increments, day lookups, and
  max-request/max-token checks for future AI relay rate limiting.
- Covered accumulation, timezone normalization, per-user/day isolation, empty-user allowance for limit
  checks, and invalid increment/limit rejection with native server tests.

## 2026-06-18 - feat: backend encrypted AI keys

- Added AES-GCM helpers for `ai_keys`: env config accepts a 16/24/32-byte master key, and storage encrypts
  per-user provider keys with user/provider authenticated data before writing SQLite ciphertext.
- Covered plaintext avoidance, decrypt/rotate flow, bad master-key rejection, cross-user miss, and wrong-key
  decrypt failure with native server tests.

## 2026-06-18 - feat: backend blob store

- Added content-addressed blob storage for the backend: sha256 hashing, path-sharded files, metadata
  upsert, verified reads, workspace links, and a GC sweep for blobs with no workspace references.
- Covered dedupe/idempotence, linked-vs-unlinked sweep behavior, oversized rejection, and hash-mismatch
  detection with native server tests.

## 2026-06-18 - feat: backend snapshot store

- Added server snapshot methods for opaque gzipped dataset payloads: put current, get current, list
  retained history newest-first, and trim prior versions to a configured limit.
- Covered last-N recovery history, history drop mode, current snapshot replacement, and dataset size-cap
  rejection with native server tests.

## 2026-06-18 - feat: backend repository layer

- Added typed server repository structs and methods for OAuth users and workspace registry rows:
  upsert user, put/list/get workspace, and soft-delete workspace tombstones scoped by user.
- Covered ordering, cross-user isolation at the repository boundary, validation errors, and deleted-vs-active
  list behavior with native server tests. Snapshot payload/LWW methods stay as the next storage atom.

## 2026-06-18 - feat: backend storage schema

- Added `internal/server.Store`, `OpenStore`, and schema-version migration handling for the backend DB.
  The opener enables SQLite foreign keys and WAL, creates the Cloud schema, and rejects databases newer
  than this binary supports.
- Pinned the first schema in tests: users, workspaces, current snapshots, snapshot history, blobs,
  workspace-blob links, encrypted AI keys, usage counters, and schema metadata.

## 2026-06-18 - feat: backend server foundation

- Started the backend series with an in-module layout: `cmd/cashflux-server` for the binary and
  `internal/server` for native-tested server pieces. No gRPC/proto dependencies yet.
- Added env-driven config, auth-mode validation (`token` or `oauth`), health/readiness endpoints,
  and `/v1/version` so the future self-host Settings "Test connection" can verify API compatibility
  and discover token-vs-OAuth plus billing mode.

## 2026-06-18 - test: D21 document import flow

- Extracted reviewed document-row import from the wasm Documents screen into `appstate.ImportReviewedDocumentRows`,
  preserving duplicate detection, category-name matching, rule fallback, bulk-trigger suspension, and import-history
  recording.
- Added D21 coverage for CSV header import (existing test), duplicate skipping, reviewed image-row import history,
  monthly statement summary, period spending totals, and budget impact. The OpenAI-key vision extraction checkbox
  remains open because it needs a live key/browser path.

## 2026-06-18 - test: D20 rules auto-fill flow

- Moved transaction draft auto-fill and CSV import auto-categorization through shared appstate helpers so
  entry and import use the same first-match rule behavior without overriding manual category/tags.
- Added D20 coverage for entry suggestions, CSV import categorization, coffee-budget spend impact,
  Apply-to-existing, and shadowed-rule conflict detection; wired Transactions to the helper.

## 2026-06-18 - test: D19 member ripple behavior

- Added appstate helpers for setting exactly one default member, resolving the member for new transaction
  forms, and deleting a member after reassigning owned records.
- Wired Members and Transactions to those helpers so the tested paths are the UI paths. The regression covers
  default-member attribution, account/budget/goal/transaction reassignment, deleting the old member, no orphaned
  owner/member ids, and recomputed `ledger.NetByOwner` rollups.

## 2026-06-18 - test: D18 net-worth assembly behavior

- Added explicit D18 coverage that `ledger.NetWorth` satisfies assets-minus-liabilities and that
  `NetByOwner` per-owner rollups sum to the household net-worth total.
- The same test keeps an account archived, verifies it is excluded from the rollups, then restores it and
  proves the account re-enters both household net worth and the owning member's rollup.

## 2026-06-18 - test: D17 freshness reminder task

- Moved the dashboard stale-balance "Remind me" task shape into `appstate.CreateFreshnessReminderTask`,
  keeping the localized title in the UI while making the generated task native-testable.
- Added appstate coverage that the reminder gets an ID, persists, and is open/medium/source=nudge. The
  D17 browser assertions for stale badges/counts and mark-updated behavior remain open.

## 2026-06-18 - test: D16 FX aggregates

- Added one aggregate-level FX regression that uses the same EUR account and EUR transactions across
  `ledger.NetWorth`, `ledger.PeriodTotals`, and `budgeting.Spent`.
- The test runs those aggregates at two different EUR rates, proving an edited rate recomputes every figure.
  Existing currency and aggregate error tests already cover missing/zero rates and stable target-minor-unit
  rounding, so the D16 behavior checklist can close without duplicating those paths.

## 2026-06-18 — feat: focus the first field when an inline editor opens (§6.7, goals + accounts)

- Inline edit forms opened with focus stranded on `<body>` (the Edit/Contribute button that opened them
  was removed from the DOM), so the user had to click into the first field. Added `screens.focusByID`
  (a `syscall/js` `getElementById(...).focus()` helper) and a `ui.UseEffect` per row keyed on the
  editor's open/closed state that focuses the first field when it opens.
- `autofocus` was rejected: browsers only honour it on initial page load, not on dynamic insertion, and
  it loses to whatever already has focus. The post-render UseEffect is the reliable path (same hook the
  dashboard uses to run the FLIP after layout).
- Gotcha: in the `screens` package `ui` is the framework `GoWebComponents/ui` (hooks), not `internal/ui`.
  First put the helper in `internal/ui` (`ui.FocusByID`) — undefined at the call sites. Moved it to a
  package-local `focusByID` in `internal/screens/focus.go`.
- Applied to goals (edit + Contribute) and accounts (edit + Update balance) this commit; first inputs got
  stable `Attr("id", ...)`. Remaining inline editors (transactions, budgets, categories, members, todo,
  rules, documents, custom pages) follow in subsequent commits. Disk wasm build green, gofmt clean.

## 2026-06-18 — feat: complete custom-page widget management (reorder/resize/edit)

- Closed the last custom-page gaps from the plan: widgets were add/delete only. Added, in `custompage.go`:
  - **Reorder:** each tile header is a drag handle; dropping on another tile calls `dashlayout.Move` over
    the page's layout (drag source held in `CustomPage`, drop callbacks passed per tile). Persisted.
  - **Resize:** ↔ / ↕ header buttons cycle width (max 4) / height (max 3) via `dashlayout.CycleSpan` +
    `ResizeItem`. `ensureLayout` synthesizes default 1×1 entries for widgets missing a layout row so
    reorder/resize always have something to operate on.
  - **Edit:** an ✎ button toggles an inline `editWidgetForm` in the tile body — edit title + the type's
    binding (KPI formula+format, list source, text, artifact) and save back into the page. All field hooks
    run unconditionally (per-type control just picks which to show), so the hooks-in-loops rule holds.
- All pure layout ops were already tested (`dashlayout`); browser-verified the tile chrome (grip/↔/↕/✎/✕)
  renders on a seeded page. wasm builds, `go test ./...` green.
- This completes the custom-page feature: pages (create/rename/hide/delete/reorder), widgets (add/edit/
  delete/reorder/resize) of all six types, artifacts, and the workflow engine.

## 2026-06-18 - test: transfer paired delete

- Moved reciprocal transfer-leg deletion from the wasm Transactions screen into appstate so the behavior is
  native-testable and reusable from any delete caller.
- Added a regression that deletes one transfer leg, removes only its exact reciprocal, and keeps a same-account
  decoy on another date plus a standalone transaction.

## 2026-06-18 - test: freshness dismissal state

- Added pure freshness dismissal state keyed by account ID. A dismissed stale nudge stays hidden for the
  current balance timestamp, then becomes eligible again once the balance is marked updated and later ages stale.
- Wired the Dashboard Freshness widget to persist dismissals in localStorage and added a Dismiss action beside
  the existing Remind flow.

## 2026-06-18 - test: D14 transfer behavior

- Added ledger coverage for a two-leg transfer: checking decreases, savings increases, household net worth
  stays unchanged, and `PeriodTotals` reports zero income and zero expense.
- Added budgeting coverage that a transfer leg with a budget category is still ignored by `Spent`, closing
  the D14 budget/KPI exclusion assertion alongside the paired-delete appstate regression.

## 2026-06-18 — fix: hide the left-rail scrollbar (C31)

- The rail's `nav` (`overflow-y-auto`) had no scrollbar styling, so it showed the default OS bar once it
  overflowed. Added `aside.rail nav { scrollbar-width:none }` + `::-webkit-scrollbar{width:0;display:none}`
  — hidden but still scrollable by wheel/trackpad/keyboard. CSS-only (web/index.html). (Skipped the
  optional edge-fade mask to avoid clipping the top/bottom nav items.)
- Disk wasm build not needed (CSS); served via index.html. Committed by pathspec.

## 2026-06-18 — fix: in-app "Set balance" form; remove the last native prompt (§6.8 complete)

- Converted Accounts "Set balance" to an inline form in `AccountRow` (settingBal state + amount input +
  Save/Cancel, mirroring inline edit), like the goals contribute one. That was the last `promptText` caller,
  so removed `promptText` + the now-unused `syscall/js` import from goals.go. §6.8 "replace native dialogs"
  is done — no `window.prompt` left in the screens. 1 new i18n key.
- gofmt clean, disk wasm build green (served wasm rebuilt). Committed by pathspec.

## 2026-06-18 — fix: in-app goal-contribute form (§6.8, branched off lock-screen)

- Branched for breadth per the user's "tonnes of todos". Replaced the Goals "Contribute" native
  `window.prompt` with an inline form in `GoalRow` (mirrors the existing inline-edit toggle: a `contributing`
  state + amount input + Add/Cancel). Removed the prompt path there.
- Gotcha: `promptText` was a SHARED helper — accounts.go's "Set balance" also calls it — so removing it broke
  the build (`accounts.go:435 undefined: promptText`). Restored `promptText` + the `js` import; accounts
  keeps it for now (its in-app form is the next §6.8 follow-up). 1 new i18n key.
- gofmt clean, disk wasm build green (served wasm rebuilt). Committed by pathspec. Screens were clean (the
  parallel session is on tests), so no collision.

## 2026-06-18 — feat: suspend/resume the lock without wiping creds (B17)

- Implemented the spec's "Lock screen toggle that flips Enabled without touching the credentials."
  `applock.Config.Suspended` + `Active() = Enabled && !Suspended`; `ShouldAutoLock` now keys off `Active`.
  All gate-show paths (maybeLockOnBoot, showAppLockGate, the auto-lock timer) check `Active()`, so a paused
  lock keeps its passcode but never appears. `setLockSuspended` flips it; Settings → App lock gains a "Lock
  screen" toggle + a "Paused" status, and hides "Lock now" while paused. Remove still fully clears.
- Table test covers active→suspend (not active, no auto-lock, still verifies passcode) and disabled.
  applock tests green, gofmt clean, disk wasm build green (served wasm rebuilt). Committed by pathspec.

## 2026-06-18 — feat: lock-screen unlock animation (B17.1)

- On a correct passcode the gate now fades out with a blur+scale (opacity/filter/transform CSS transition,
  ~0.35s) via a new `unlockGate`, then sets display:none and resets the styles (self-releasing setTimeout
  callback). Honors `prefers-reduced-motion` (instant hide). Gate-only change.
- gofmt clean, disk wasm build green (served wasm rebuilt). Committed by pathspec.

## 2026-06-18 — feat: toggle lock-screen quote/metadata in Settings (B17.1)

- Made the lock-screen content configurable. Added `HideQuotes`/`HideMeta` to `applock.Config` (stored as
  "hide" flags so the zero value / older configs default to shown = ON, per spec). Two `ui.ToggleRow`s in
  the App-lock settings section flip them via `setLockHideQuotes`/`setLockHideMeta`; `refreshLockMeta` now
  shows/hides the greeting+date and quote elements accordingly (display none when off). 2 new keys.
- gofmt clean, applock tests green, disk wasm build green (served wasm rebuilt). Committed by pathspec.

## 2026-06-18 — feat: passcode hint (revealed after failed attempts) (B17)

- Optional hint that surfaces only after 3 wrong tries, never sitting on the gate for a passer-by. Pure
  `applock.ValidHint(hint, passcode)` rejects any hint containing the passcode (case-insensitive, trimmed);
  `WithPasscode` now takes a hint and drops a leaky one (Config gains a `Hint` field). Table tests cover
  empty/safe/leaky/case-insensitive + storage. `enableAppLock` threads the hint.
- Gate: a `fails` counter in the attempt closure reveals a hidden "Show hint" button at ≥3 misses (only if
  a hint exists); clicking shows `Hint: <text>` in the message line. Reset on success / each show. The
  setup form gained a hint input with inline "can't contain your passcode" validation. 5 new applock.* keys.
- applock tests green, gofmt clean (realigned en.go), disk wasm build green (served wasm rebuilt).
  Committed by pathspec.

## 2026-06-18 — feat: lock-screen greeting + date + daily quote (B17.1)

- Started the B17.1 lock-screen experience: new pure `internal/lockquotes` package (12 curated finance/
  motivation lines; `ForIndex` wraps deterministically — no randomness, table-tested for wrap/negative/
  determinism). The gate now renders a time-of-day greeting, locale date (`toLocaleDateString`), and the
  day's quote (rotated by `Date.now()/86400000`), via a `refreshLockMeta` recomputed on every show. All
  privacy-safe — nothing financial. Hardcoded greeting strings for now (English; the quotes are English
  anyway) — i18n + Settings toggles + opt-in glanceable data are follow-ups.
- New package + gate-only change (applockgate.go); lockquotes test green, gofmt clean, disk wasm build
  green (served wasm rebuilt). Committed by pathspec.
- Re-engaged the broader backlog (454 open items) after the user flagged it — dropping the
  collision-avoidance over-caution; using pathspec commits to coexist with the parallel session.

## 2026-06-18 — fix: focus-trap the lock gate (B17 modality complete)

- Closed the residual from the previous commit: the unlock gate now traps Tab within its controls
  (input → Unlock → Forgot, wrapping at both ends via a keydown listener on the gate, mirroring
  FlipPanel's trap), so Tab can't move focus to the covered background. Combined with the opaque
  click-blocking overlay and the shortcut suppression, the locked state is now fully modal.
- gofmt clean, disk wasm build green (served wasm rebuilt). Committed by pathspec. B17 lock is now
  robust end-to-end.

## 2026-06-18 — fix: lock gate suppresses global keyboard shortcuts (B17 hardening)

- Spotted a bypass: the unlock gate is a visual/click overlay, but the keyboard shortcuts are document-level
  listeners, so while "locked" you could still Alt+1–9 navigate or Cmd+K the palette (behind the gate).
  Added `appLockActive()` (gate exists + display != none) and made `wireKeyboardShortcuts` bail when it's
  true — the gate's own input listeners are on the gate elements, so passcode entry still works.
- Residual (noted, not done): no full focus-trap, so Tab could still move focus to covered background
  controls; for a soft gate the click + shortcut blocking is the main protection. gofmt clean, disk wasm
  build green (served wasm rebuilt). Committed by pathspec.

## 2026-06-18 — test: D13 forecast net-worth feed

- Closed the D13 pure forecast unit-test checklist item by adding a net-worth-feed bridge in
  `internal/forecast/forecast_test.go`.
- The test derives a starting net worth from `ledger.NetWorthSeries`, then projects recurring and one-time
  future changes with `forecast.Project`, covering the dashboard/planning overlap at the pure layer.
- Rechecked after the B17 shortcut hardening commit landed and kept this docs delta attached to the
  forecast test/TODO commit.
- Rechecked again after the lock-gate focus-trap commit landed; the final D13 commit now carries the
  test, docs follow-up, and completed TODO checkbox together.
- Verification: `go test ./...`, wasm build, and `gwc verify` passed.

## 2026-06-18 — test: ReassignOwner entity coverage

- Tightened `TestReassignOwnerAllEntityTypes` so it verifies the final owner/scope for budget and goal
  records as well as accounts and transactions.
- This closes the ReassignOwner unit TODO with explicit assertions across all four entity categories.

## 2026-06-18 — test: recurring cadence catch-up

- Expanded `Recurring.Advance` coverage from monthly-only to weekly, monthly, quarterly, and yearly.
- Made `PostDueRecurring` deterministic with fixed due/as-of dates, asserting the exact catch-up count,
  advanced `NextDue`, and zero new transactions on a second run.

## 2026-06-18 — test: rules retroactive apply path

- Extended `TestApplyRules` beyond the existing first-match update case to assert transfers are skipped and
  pre-existing transaction tags are preserved.
- The pure `rules.FirstMatch` and `rules.Conflicts` tests already covered ordering/shadowing, so this closes
  the remaining retroactive `ApplyRules` path.
- Rechecked after the B17.1 quote/greeting commit landed and kept a fresh docs delta for the atomic rules
  test/TODO commit.

## 2026-06-18 — test: formula/custom-field round trip

- Added an appstate integration test that saves a numeric custom field, an account value, and a saved
  formula, then export/imports the dataset.
- The test validates the imported custom map and evaluates the imported formula against the imported custom
  value, tying the already-covered formula sandbox to custom-field persistence.
- Rechecked after the B17 passcode-hint commit landed and kept a fresh docs delta for this test/TODO commit.
- Rechecked again after the B17.1 Settings toggles landed; this commit keeps the formula/custom-field test,
  docs note, and checkbox together.
- Rechecked after the B17.1 unlock animation landed; keeping the staged formula/custom-field test and TODO
  checkbox paired with fresh docs.

## 2026-06-18 — test: extract/CSV column mapping

- Extended the CSV import round-trip test with a hand-written, reordered header using friendly
  `account`/`category`/`member` names.
- The test asserts those names resolve to IDs and the imported row keeps amount, date, cleared state, and
  semicolon-separated tags, complementing the existing `extract.ParseRows` parsing/dedupe tests.

## 2026-06-18 — test: config layering current contract

- Added a resolver-layer test for budget methodology: empty settings parse to the simple default, household
  settings override to zero-based, and member records do not change methodology because there is no
  member-level override model today.

## 2026-06-18 — test: D15 cleared balance adjustment math

- Closed the D15 pure ledger unit-test checklist item by extracting `ledger.AdjustmentToTarget` and covering
  both adjustment and no-op cases in `internal/ledger/ledger_test.go`.
- The Accounts reconcile flow now calls the helper before posting a cleared balance-adjustment transaction,
  keeping the UI behavior unchanged while making the math testable.
- Verification: `go test ./...`, wasm build, and `gwc verify` passed.

## 2026-06-18 — test: D18 net-worth rollups

- Closed the D18 pure ledger unit-test checklist item with a combined `NetWorth`/`NetByOwner` case in
  `internal/ledger/ledger_test.go`.
- The test mixes two individual owners, the household group owner, a EUR asset converted to USD, a liability,
  and an archived account that must be excluded from both total net worth and owner rollups.
- Verification: `go test ./...`, wasm build, and `gwc verify` passed.

## 2026-06-18 — test: D9 payoff final-month boundary

- Closed the D9 pure unit-test checklist item by adding an exact final-payoff-month case in
  `internal/payoff/payoff_test.go`.
- Existing tests already covered payment-equals-interest as non-viable and `allocate.Distribute` reserve/cap;
  this pins the remaining payoff-month boundary where the last payment is capped at the amount owed.
- Verification: `go test ./...`, wasm build, and `gwc verify` passed.

## 2026-06-18 — test: D12 goal pace projection

- Closed the D12 pure unit-test checklist item by linking `goals.MonthlyNeeded` to `goals.Project` in
  `internal/goals/goals_test.go`.
- The new test computes the required monthly contribution and proves using that amount projects exactly to
  the target date; existing allocate tests already cover the `GoalProgress` scorer/ranking behavior.
- Verification: `go test ./...`, wasm build, and `gwc verify` passed.

## 2026-06-18 — fix: make the multi-currency (FX) editor functional (D16)

- Discovered the Settings FX editor was a dead stub: base-currency `<select>` had no `OnChange`, `rateRow`'s
  input had no handler, and rows only appeared for currencies already in `FXRates` (so an empty table had
  no way to add one). The whole pipeline below it already worked — `store.Settings{BaseCurrency,FXRates}`
  exists, every screen builds `currency.Rates` from it, and `ledger.*` converts — so this was purely a
  broken editor UI.
- Fix (settings.go): `onBase` saves the base via `PutSettings`+bump; `setRate` writes/clears a rate;
  `rateRow` → `fxRateRow` component (stable change hook) with an editable number input that commits on
  change (blur) so decimals aren't reparsed mid-typing; rows now render for every registered currency
  (`currency.Codes()`) except the base, so any rate is addable. Base selector lists all registered
  currencies (was a hardcoded 3). Swapped the now-unused `sort` import for `currency`.
- Added pure `currency.Codes()` (sorted, table-tested). gofmt clean, currency test green, worktree wasm
  build exit 0. Committed by pathspec. This effectively completes the D16 multi-currency feature's UI.

## 2026-06-18 — test: D11 planning one-time end balance

- Closed the D11 pure planning unit-test checklist item with an `EndBalance` case that includes a one-time
  future item.
- Existing tests already covered `Project` and `MonthlyNet` with one-time items; this pins the final
  projected balance helper against the same behavior.
- Rechecked after the FX editor commit landed and kept this docs follow-up attached to the test/TODO commit.
- Verification: `go test ./...`, wasm build, and `gwc verify` passed.

## 2026-06-18 — test: D4 budget scope aggregation

- Closed the D4 pure budgeting unit-test checklist item with a mixed-member scenario in
  `internal/budgeting/budgeting_test.go`.
- The test evaluates the same food transactions through an individual member budget and a group budget,
  proving owner-only spend and household-wide spend stay distinct.
- Verification: `go test ./...`, wasm build, and `gwc verify` passed.

## 2026-06-18 — feat: app-lock "Forgot passcode?" recovery (close lockout gap)

- A forgotten passcode previously meant permanent lockout (no recovery — the B17 spec called for one).
  Added a "Forgot passcode?" link on the unlock gate → `confirmAction` → `wipeAllLocalData()` (removes every
  `cashflux:*` localStorage key) → reload (boot re-seeds a fresh sample). For a soft, unencrypted gate,
  wipe-to-reset is the honest recovery — "just remove the lock" would let anyone bypass it. 2 new
  `applock.*` keys. In `applockgate.go` + en.go only.
- gofmt clean; disk build green this time so I rebuilt the served `web/bin/main.wasm` directly. Committed by
  pathspec. B17 is now genuinely complete (no lockout dead-end).

## 2026-06-18 — fix: surface app lock in Settings (user couldn't find it)

- User feedback: no way to set up / toggle the lock from Settings — it was only in the Cmd+K palette, which
  they didn't discover. Added a **Settings → App lock** section (`appLockSection` in new
  `applocksettings.go`, wired into globalSettingsForm after the Workspaces section): status line + adaptive
  buttons (Set when off; Lock now / Change / Remove when on), all reusing the existing handlers.
- Made the setup form refresh-aware: `showAppLockSetup(onDone func())` stores a package-level callback run
  on successful enable, so the Settings section updates immediately after you set a passcode. `setPasscodeFlow`
  (palette) passes nil. 5 new `applock.*` keys (section/hint/status×3, statusOnAuto uses %d).
- Had to touch shared `settings.go` (3 lines) — the collision I'd been avoiding, but the user explicitly
  wants it there. gofmt clean, worktree build exit 0, committed by pathspec.

## 2026-06-18 — test: D16 currency conversion edge coverage

- Closed the D16 pure currency unit-test checklist item in `internal/currency/currency_test.go`.
- Added coverage for missing target-currency rates, negative amount rounding, and repeated cross-rate
  conversion stability so render loops do not hide conversion drift.
- Rechecked after the app-lock Settings commit landed; this follow-up keeps the currency commit's own docs
  delta attached to the test and TODO checkbox changes.
- Verification: `go test ./...`, wasm build, and `gwc verify` passed.

## 2026-06-18 — fix: UX polish §6.6 — segmented control arrow keys

- Closed the shared segmented-control keyboard item in `internal/ui/controls.go`: radiogroups now handle
  Arrow Left/Up and Arrow Right/Down, wrapping through options and calling the existing `OnSelect`.
- Kept the change at the `Segmented` container so every existing caller gets the behavior without new
  screen-specific code.
- Browser verification focused the period selector and confirmed ArrowRight moved `Month` to `Quarter`,
  then ArrowLeft moved back to `Month`; `go test ./...`, wasm build, and `gwc verify` passed.

## 2026-06-18 — fix: UX polish §6.4 — workspace switcher divider spacing

- Closed the workspace-switcher separator spacing item in `internal/app/wsswitcher.go`: the divider before
  menu management actions now uses `my-2 pt-2` instead of only `my-2`.
- This keeps the existing border-line separator but gives the action group the requested breathing room.
- Browser verification opened the workspace switcher and confirmed the divider renders as
  `border-t border-line my-2 pt-2`; `go test ./...`, wasm build, and `gwc verify` passed.

## 2026-06-18 — fix: UX polish §6.9 — collapsed rail flyout clicks

- Closed the collapsed-rail flyout clickability item in `web/index.html`: the hover/focus label now uses
  `pointer-events:auto` instead of `none`.
- Because the label is an absolutely positioned child of the nav item, hovering/clicking the label keeps
  the parent nav item's interactive state available.
- Browser verification checked the hovered flyout label (`display:block`, `position:absolute`,
  `pointer-events:auto`); `go test ./...`, wasm build, and `gwc verify` passed.

## 2026-06-18 — feat: export-data commands in the command palette

- Added "Export JSON" / "Export CSV" to the Cmd+K palette (reusing the existing `exportJSON`/`exportCSV`
  helpers + the `settings.exportJSON/CSV` i18n keys — no new keys). New `paletteNotify` posts the
  helpers' success/error via the global Notice/toast atom (works outside a render). Export is read-only
  (download), so no refresh/reload plumbing needed — kept import/sample/wipe out of the palette since those
  need a re-render and wipe has reload subtleties.
- All in `shortcuts.go`; gofmt clean, worktree build exit 0, committed by pathspec.

## 2026-06-18 — feat: in-app passcode setup form (B17 final; UX audit §6.8)

- Replaced the set-passcode native `prompt()`s with an in-app modal form (`showAppLockSetup`/
  `buildAppLockSetup` in `applockgate.go`): passcode + confirm password inputs, an auto-lock-minutes
  number field, inline error (empty / mismatch), Cancel/Enable, Enter-to-submit. `setPasscodeFlow` is now
  a thin wrapper that opens it, so the palette's Set/Change commands route here. 6 new `applock.*` form
  keys (escaped via a small `escT` helper). This closes the §6.8 "replace native dialogs" item for app-lock
  and finishes B17 (the old `applock.setPrompt/confirmPrompt/autoPrompt/enabled` keys are now unused data —
  harmless, left in place).
- gofmt clean, worktree build at HEAD exit 0, committed by pathspec. B17 done: core+tests → gate → palette
  → auto-lock → i18n → in-app form.

## 2026-06-18 — refactor: i18n the app-lock strings (B17 polish)

- Routed the passcode-lock UI through the catalog (13 new `applock.*` keys): the unlock gate, the
  set-passcode flow (set/confirm/auto-lock prompts, mismatch + enabled alerts), and the palette commands
  (Set/Lock now/Change/Remove). Added a `uistate` import to `applockgate.go`. The whole keyboard + app-lock
  UI is now translatable; only a proper in-app passcode form (vs native prompts) remains.
- gofmt clean, worktree build at HEAD exit 0, committed by pathspec.

## 2026-06-18 — fix: UX polish §6.1 — delete button hit area

- Closed the `.btn-del` target-size item in `web/index.html`: delete icon buttons now have an explicit
  `min-width:32px` and `min-height:32px`, while keeping their existing transparent visual treatment.
- This leaves the shared 24px minimum for smaller non-delete icon controls intact.
- Browser verification initially caught the cascade order overriding width back to 24px; the final rule now
  computes to 32×32px.
- Re-ran `go test ./...`, the wasm build, and `gwc verify` after the app-lock setup form landed; all passed.

## 2026-06-18 — feat: app-lock idle auto-lock (B17 cont.)

- Wired the pure `ShouldAutoLock` into a real timer. `setPasscodeFlow` now also prompts for an auto-lock
  window (minutes; 0 = off). New `wireAutoLock` (armed once in `app.go` after `maybeLockOnBoot`) tracks a
  `last`-activity timestamp reset by mousemove/keydown/click/touchstart/scroll, and a 30s `setInterval`
  re-shows the gate once `ShouldAutoLock(idleMinutes)` is true (guarded against re-showing an already-open
  gate, then resets `last`). Uses JS `Date.now()` for timing (Go's monotonic time isn't needed here).
- Still in my/new files only (`applockgate.go` + one `app.go` line) — collision-free. gofmt clean, worktree
  build exit 0. The B17 lock is now functional end-to-end: set (with optional auto-lock) → boots/locks/idles
  behind the gate → unlock. Remaining: in-app passcode form (vs prompts) + i18n.

## 2026-06-18 — feat: passcode lock MVP (B17) — gate + palette management, collision-free

- Built the functional lock on top of the pure `applock` core, deliberately keeping it out of the parallel
  session's hot files: persistence in a new `internal/app/applock.go` (load/save Config at user-global
  `cashflux:applock`, `crypto/rand` salt, enable/disable); a self-contained DOM unlock gate in new
  `internal/app/applockgate.go` (full-screen `var(--bg)` cover, password input, Enter/Unlock → constant-time
  Verify → hide); and management via the Cmd+K palette (adaptive: Set / Change / Lock now / Remove) rather
  than settings.go. Boot wiring is a single `maybeLockOnBoot()` line in `app.go`.
- MVP scope: lock-on-reload + manual "Lock now"; passcode set via native prompt×2 (confirm). Deferred:
  idle auto-lock (the pure `ShouldAutoLock` is ready), an in-app passcode form, focus-trap, and i18n of the
  app-lock strings (hardcoded for now — flagged).
- New files were untracked, so committed via `git add` then `git commit -- <paths>` (learned that from the
  pure-core commit). Verified gofmt + worktree build at HEAD (exit 0); disk build red only on parallel WIP.

## 2026-06-18 — feat: make workflows genuinely valuable (acted on a value critique)

- A product critic judged the workflow engine "a demo with one real button": txn-added conditions/actions
  were blind to the triggering transaction (only month aggregates), notify just logged, applyRules
  duplicated rules. Implemented the Tier-1 fixes that turn it into real transaction automation:
  - **formula engine:** `Env` gained `Strs` (string variables) and a `contains()` (case-insensitive) +
    `lower()` function — so conditions can match text, not just numbers.
  - **workflow engine:** `Eval`/`Plan` now take a `Context{Vars, Strs, TxnID}`. New write-safe actions
    `setCategory`/`addTag`/`flagReview` carry the triggering txn id. Validate covers them.
  - **appstate:** `txnContext` builds per-txn vars (`txn_amount`/`txn_abs` major + `txn_payee`/`txn_desc`/
    `txn_category`/`txn_account`/`txn_tags`); `RunWorkflowOn` runs with that context; `RunTriggered`
    threads the triggering txn; `applyEffect` mutates the txn via a below-trigger `mutateTxn` (loop-safe).
    `notify` now calls an injectable `Notifier` hook, wired from the `Toast` component to a real toast.
  - **UI:** the action picker offers the new actions with a category picker / tag field; the condition
    hint shows per-txn examples.
  - Tests: per-txn Eval (amount + contains), txn-mutating Plan, and an appstate routing test (bistro→
    Dining; big spend→needs-review) + a notify-hook test. All green.
- **Verification rabbit hole worth recording:** the browser showed "no workflows" even with a fresh
  binary. Native tests passed, so I traced it: my throwaway seed harness embedded the exported JSON in a
  **JS backtick template literal**, which un-escaped the `\"` inside a condition string
  (`contains(txn_payee, "bistro")`), corrupting the JSON; `ImportJSON` correctly rejected it and fell back
  to the sample (a 1-member household — which mimicked "members load, workflows don't"). Switched the
  harness to `fetch()` a `.json` file (no escaping) and the workflows render correctly. App was never at
  fault. Also confirmed via the debug page that custom pages/artifacts persist fine in wasm.

## 2026-06-18 — feat: app-lock pure-logic core (B17 groundwork)

- The user kept re-invoking /loop (wants continuous output), and the isolated app/keyboard/workspace work
  is done — so I started B17 from the bottom with a NEW, zero-collision package `internal/applock`.
  Deliberately built only the UX-agnostic pure layer (no modal/recovery-flow assumptions, respecting the
  confirm-specs rule): `Config{Enabled,Salt,Hash,AutoLockMinutes}`, `HashPasscode` (salted SHA-256, hex),
  `WithPasscode`/`Cleared`, constant-time `Verify` (crypto/subtle), and `ShouldAutoLock(idleMinutes)`.
- Table tests cover determinism + salt/passcode sensitivity, verify correct/wrong/empty/disabled, empty
  passcode-or-salt rejected, negative window clamped, cleared-can't-verify, and the auto-lock thresholds.
- Pure native package (no wasm, no served-asset change → no SW bump); `go test ./internal/applock` green,
  gofmt clean. Committed by pathspec. Next (needs a quick UX nod): persist the config, a lock-screen
  overlay + Settings toggle, and wire crypto/rand salt + idle timing.

## 2026-06-18 — fix: UX polish §6.4 — selected transaction checkbox state

- Closed the selected-transaction checkbox item across `internal/screens/transactions.go` and
  `web/index.html`: selected rows now carry `row selected`, and `.row.selected .check` gets an accent
  background/border.
- The base `.check` now has a transparent border to avoid layout shift when selection turns the border on.
- Browser verification confirmed the selected checkbox computes to the accent background, border, and text
  color with `box-sizing:border-box`.
- Re-ran `go test ./...`, the wasm build, and `gwc verify` after the app-lock commit landed; all passed.
- Final commit was held until the five-minute cooldown after the idle auto-lock commit cleared.

## 2026-06-18 — fix: UX polish §6.11 — light-theme soon badge

- Closed the fixed dark-blue `.badge-soon` item in `web/index.html`: light theme now overrides the badge
  background, text, and border colors instead of reusing the dark palette.
- This keeps existing dark-theme styling intact and only adds the missing light-theme treatment.

## 2026-06-18 — fix: UX polish §6.1 — form field target height

- Closed the shared form-field target-height item in `web/index.html`: `.field` now has a 44px minimum
  height, while compact density still keeps a 40px floor instead of dropping to 36px.
- This covers the app's shared text/select/date/number field class without changing individual form code.

## 2026-06-18 — fix: UX polish §6.2 — segmented button label size

- Closed the segmented-button part of the tiny-type item in `web/index.html`: `.seg-btn` labels moved from
  `0.8rem` to `0.85rem` without changing the existing compact padding or active-state styling.
- This covers the shared segmented control CSS used by the dashboard/resolution controls.

## 2026-06-18 — fix: UX polish §6.11 — accent swatch hit area

- Closed the settings accent-swatch target-size item in `web/index.html`: `.swatch` moved from 22×22px to
  24×24px, preserving the existing 6px radius and selection border.
- This is CSS-only because the shared `ui.Swatch` component already emits the `.swatch` class for accent
  choices.

## 2026-06-18 — refactor: i18n the command palette (§6.6, finishes the keyboard-UI pass)

- Routed the Cmd+K palette through the catalog: search placeholder/aria + "No matching commands" + the
  action labels (toggle theme/sidebar, switch/new/export workspace) now use `uistate.T` (new `cmd.*` keys;
  import reuses `ws.import`). With the prior help-overlay commit, the whole keyboard UI is translatable.
- gofmt clean, worktree build at HEAD exit 0, committed by pathspec.

## 2026-06-18 — refactor: i18n the keyboard help overlay (§6.6)

- Routed the `?` cheat-sheet through the i18n catalog per the engineering standards: `helpHTML` went from a
  const to a builder func that interpolates `uistate.T("shortcuts.*")` for the title + row labels (8 new
  keys after `rail.tools`); chords stay literal. The palette's "Keyboard shortcuts" command now reuses
  `shortcuts.title`. (Command-palette chrome strings — search placeholder, no-match, action labels — remain
  a follow-up.)
- en.go is the hottest shared file; took a couple of Read→Edit retries as the parallel session wrote it.
  Verified via worktree build at HEAD with my shortcuts.go + en.go (exit 0; disk build red only on the
  parallel session's WIP). Committed by pathspec.

## 2026-06-18 — fix: UX polish §6.2 — priority badge legibility

- Closed the To-do priority badge part of the tiny-type item in `web/index.html`: `.badge-prio` moved from
  `0.68rem` to `0.75rem`, and `.task-meta` gap loosened from `0.5rem` to `0.6rem`.
- Kept this CSS-only because the rendered badge markup already uses the shared `.badge-prio` classes.
- Browser verification confirmed the priority badge computes to 12px and the metadata gap to 9.6px.

## 2026-06-18 — fix: UX polish §6.4 — disabled button state

- Closed the shared disabled-button style item in `web/index.html`: `.btn:disabled` and
  `.btn[aria-disabled="true"]` now dim, use `cursor:not-allowed`, and suppress hover brightening.
- Targets existing real disabled buttons such as Insights' "Thinking" state without changing screen logic.

## 2026-06-18 — feat: theme + sidebar toggle commands in the palette

- Added "Toggle light / dark theme" and "Collapse / expand sidebar" to the Cmd+K palette — standard
  palette affordances. `toggleTheme` flips `prefs.Theme` (non-light → dark) then Set/Persist/ApplyPrefs
  (CSS reacts via the data-theme attr immediately); `toggleSidebar` flips `UseRailCollapsed` + persists.
  Both use global uistate atoms, so they work from the palette callback. All in `shortcuts.go` (+prefs
  import); no en.go, no screen files — zero collision.
- The parallel session keeps churning many files (custompage.go, then formula/eval.go unused imports on
  disk); verified my change via worktree build at HEAD (exit 0) rather than the red disk build. Committed
  by pathspec.

## 2026-06-18 — harden custom pages + workflows (acted on an adversarial test critique)

- Ran a critic subagent against the e2e suite; it found real gaps. Acted on them:
  - **M1 (real bug):** the txn-added trigger was wired into quick-add only. Centralized it in
    `appstate.PutTransaction`, firing on genuinely-new txns (GetTransaction existence check, so edits don't
    fire) — now every add path (inline editor, transfer, duplicate, CSV + image import) honors it. Added a
    `triggersSuspended` flag + `WithoutTriggers` so bulk imports fire once, and so a workflow applying its
    own effects can't recursively re-fire. (ApplyRules uses `store.PutTransaction`, below the trigger, so
    it was already loop-safe; the guard is belt-and-suspenders.)
  - **M2:** `applyEffect` createTask is now idempotent — skips when an open task with the same title
    exists, so a per-add workflow doesn't spawn a task storm.
  - **C1/C2 (false confidence):** the widget renderers were `js && wasm`-only and untested; the "story"
    tests re-implemented their logic. Extracted the real logic into a pure `internal/widgetdata` package
    (`ListRows` newest-first + cap + formatting, `KPIText`, `ChartWindow`) and made the renderers thin
    shells over it. Now the actual code path users see is table-tested natively (ordering, truncation,
    currency rounding, unknown-source).
  - **M4/m1:** added an injectable clock seam (`App.now`/`clock()`) so month-scoped figures are
    deterministic in tests; added trigger/dedupe/disabled/multi-action/notify tests; tightened the
    rename test to assert the exact `gamma-2` slug.
- Discovered while writing tests: `ValidateTransaction` requires a non-empty `desc`, so seed txns must set
  it (was silently skipping rows). Browser re-verified the refactored render path (KPI/list/chart correct).

## 2026-06-18 — fix: UX polish §6.3 — upcoming bill dates use preferences

- Closed the dashboard upcoming-bills date-format item in `internal/screens/dashboard.go`: the widget now
  reads `uistate.UsePrefs().Get()` and renders due dates with `Prefs.FormatDate`.
- This keeps the widget aligned with the existing Settings date-style preference instead of hardcoding
  `Jan 2`.
- Browser verification set `cashflux:prefs.dateStyle` to `us` and confirmed the bills widget rendered
  slash-form dates such as `06/18/2026`.
- Kept the implementation isolated to `dashboard.go`; the parallel workflow commit landed while this was
  waiting on cooldown.

## 2026-06-18 — feat: workspace management commands in the palette + HEAD health check

- Health check (repo is churning fast under the parallel session): `go test ./...` on disk showed 3
  appstate FAILs, but those are the parallel session's UNCOMMITTED WIP — committed HEAD passes appstate in
  a clean worktree (exit 0), and all pure-logic packages are green. No pushed regression.
- Feature: added New / Export current / Import workspace commands to the Cmd+K palette (alongside the
  per-workspace Switch entries), all via existing app-package funcs (createWorkspace / exportWorkspace /
  importWorkspace / pickFile / promptName) — so full workspace management is keyboard-reachable. Entirely
  in `shortcuts.go`, no en.go, zero collision.
- Verified gofmt clean + worktree build exit 0 (disk build still red only on the parallel session's
  custompage.go unused-imports WIP). Committed by pathspec.

## 2026-06-18 — fix: UX polish §6.2 — rail section label legibility

- Closed the non-custom rail label part of §6.2 in `internal/app/shell.go`: rail section labels moved from
  `text-[10px] tracking-[0.16em]` to `text-[11px] tracking-[0.08em]`.
- Left custom-page label contrast work untouched because another agent owns custom-page TODOs.

## 2026-06-18 — feat: switch workspaces from the command palette

- Enriched the Cmd/Ctrl+K palette with "Switch to workspace: <name>" entries (one per non-active
  workspace) that call `switchWorkspace(id)` — integrating two features I own, entirely within
  `shortcuts.go`. Moved `cmdPaletteCmds = buildPaletteCommands()` from build-time into `openCommandPalette`
  so the command list (and thus the workspace entries) rebuilds on every open and never goes stale.
- Build note: a full wasm build currently fails ONLY on the parallel session's `internal/screens/
  custompage.go` (unused imports — their mid-edit WIP); my `shortcuts.go` change has zero errors. Verified
  HEAD+my-change compiles by copying it into a detached worktree at HEAD and building (`GOOS=js` exit 0).
  Committed by pathspec, so their broken WIP isn't included. Couldn't refresh the served wasm (their break
  blocks `gwc dev`'s rebuild) — confirm in-browser once their build is green.

## 2026-06-18 — feat: §6.6 command palette (Cmd/Ctrl+K) — the capstone keyboard item

- The remaining [H] §6.6 item. Self-contained DOM overlay in `shortcuts.go` (same pattern as the help
  overlay): a search input + a filtered result list, built once and toggled via `display`.
- Commands = every screen from `primaryNav()`/`toolsNav()`/`systemNav()` (→ `router.Navigate(path)`) plus
  two actions (Add a transaction → `UseQuickAdd().Set(true)`; Keyboard shortcuts → `toggleHelpOverlay`).
- Keyboard model: type to substring-filter; ↑/↓ wrap-move the highlight (re-styling rows + scrollIntoView);
  Enter runs the highlighted command; Esc/backdrop close. Row clicks use a single delegated listener that
  reads `data-cmd-row` (the original command index) via `closest`, so the dynamically rebuilt rows need no
  per-row js.Funcs (no leak). The handful of listeners (input/keydown/click) are created once with the
  overlay and live for the app's lifetime.
- Wired Cmd/Ctrl+K into the global keydown (works even from a field, since it's a modifier chord); Esc in
  the global handler now closes both the help and command overlays. Labels HTML-escaped before innerHTML.
- gofmt clean, wasm build exit 0, probe ok/200. Can't drive Cmd+K + typing headlessly — logic-verified;
  confirm in-browser. §6.6 keyboard set complete: Alt+1..9, Alt+N, Enter, ?, Cmd/Ctrl+K.

## 2026-06-18 — feat: §6.6 quick-add hotkey (Alt+N)

- Alt+N opens the quick-add transaction panel by setting `uistate.UseQuickAdd().Set(true)` from the global
  keydown handler. Confirmed safe: `UseAtom` → `runtime.GoUseAtomGlobal` is a global id-keyed atom (not
  component-bound), and the +Add menu already calls the identical `quickAdd.Set(true)` from an OnClick
  (also a non-render callback), so opening it from a keydown behaves the same.
- Chord choice: the audit suggested Ctrl/Cmd+Shift+A, but that's reserved (Chrome tab-search, Firefox
  add-ons). Used Alt+N instead — conflict-free and consistent with the Alt+1..9 family. Added to the `?`
  overlay. Lives entirely in `shortcuts.go` (+uistate import); low collision while the parallel session
  works other §6 items.
- gofmt clean, wasm build exit 0, probe ok/200. Keystroke can't be sent headlessly — logic-verified.

## 2026-06-18 — fix: UX polish §6.1 — rail nav hit areas

- Closed the §6.1 rail-nav touch-target item in `internal/app/shell.go`: all `navItem` variants now carry
  `min-w-10 min-h-10`, so expanded and collapsed rail items keep a stable 40×40px minimum hit area.
- Kept this separate from custom-page nav work because another agent owns custom-page TODOs.
- Verified the rendered Dashboard nav row exposes both `min-w-10` and `min-h-10` in the browser.

## 2026-06-18 — fix: §6.9 toast — error notices linger + labelled dismiss

- `toast.go` already had the role/aria-live work (status/polite for info, alert/assertive for errors, plus
  a persistent live region). Closed the remaining §6.9 item: split the auto-dismiss timeout —
  `toastErrTimeoutMS` 7500ms for errors vs 4500ms for ordinary notices — so error text isn't whisked away
  before it's read. Added `aria-label="Dismiss"` to the close button (§6.9 [L], to go with its title).
- Single file (`internal/app/toast.go`); gofmt clean, wasm build exit 0, probe ok/200.
- Process: committing via `git commit -- <paths>` now (not `git add` then commit) so the parallel session's
  pre-staged files don't get swept into my commit — last time my "?" overlay commit absorbed their
  en.go/allocate.go because `git commit` writes the whole shared index.
- §6 progress (mine): 6.1, 6.2, 6.3, 6.4(switcher), 6.6(Alt+1..9/Enter/?-overlay), 6.9(toast), 6.11.

## 2026-06-18 — test: 10 user stories e2e for custom pages + workflows

- Wrote `docs/CUSTOM_PAGES_STORIES.md` (10 stories + acceptance + edge cases) and
  `internal/appstate/scenarios_test.go` driving every story through the real appstate API: KPI dashboards
  (formula eval), recent-activity list source, page persist + non-overlapping `Pack` + widget delete,
  artifacts (CSV parse → table rows, image data URL), page organize (reorder/hide/unique re-slug),
  overspend alert firing on a qualifying txn (and not otherwise), manual apply-rules workflow with dry-run
  (no change) then real run (categorizes), plus edge cases (bad formula errors, false condition no-ops,
  export→import round trip). All green.
- Browser e2e: seeded a multi-page workspace and screenshotted a stress page with all six widget types
  (4 KPIs + list + chart + table + text + image) packed without overlap, a second page (accounts list),
  the hidden-pages section, and inter-page navigation — all correct.
- Bug found + fixed: currency KPIs truncated `value×100` to int, so float error could drop a cent
  ($15,343.50 → $15,343.49). Now `math.Round`. No other defects surfaced.

## 2026-06-18 — fix: UX polish §6.10 — custom-field key validation

- Moved custom-field key format rules into the pure `internal/customfields` layer: keys must be ASCII
  letters/numbers/underscores and cannot shadow reserved metadata names (`id`, `entityType`, `type`,
  `custom`, etc.). `Def.Validate` now enforces this before persistence; table tests cover valid keys,
  spaces, punctuation, and case-insensitive reserved names.
- Added a matching browser hint on the Custom Fields add form (`pattern="[A-Za-z0-9_]+"` + localized
  title), so users see the allowed format before appstate rejects the save.

## 2026-06-18 — feat: §6.6 "?" keyboard help overlay (self-contained DOM)

- Added `?`-to-toggle (and Esc/✕/backdrop-to-close) a shortcuts cheat sheet, documenting the Alt+1..9,
  Enter, Esc, and Shift bindings. Built as a pure-DOM overlay created/toggled entirely in `shortcuts.go`
  (createElement + innerHTML + display toggle) — deliberately NOT a framework component, so it needs no
  shell mount point and stays zero-collision while the parallel session churns §6 (it just shipped §6.10).
- Wiring: extended the existing global keydown handler — Esc calls `closeHelpOverlay` (no-op if closed;
  FlipPanel still owns Esc for panels), and `?` (guarded by isEditableTarget) toggles. The overlay is
  built once and thereafter shown/hidden via `style.display`; its click/close js.Funcs live for the app
  lifetime (like the boot listeners), so no release bookkeeping.
- English strings hardcoded for now (consistent with FlipPanel's hardcoded Save/Cancel); noted an i18n
  follow-up in the source. Verified gofmt clean, wasm build exit 0, probe ok/200. The `?` toggle can't be
  exercised headlessly — logic-verified; confirm in-browser.
- §6.6 keyboard set now: Alt+1..9 jump, Enter-save, and this help overlay. Remaining 6.6: quick-add hotkey
  (touches quick-add state) and Cmd/Ctrl+K palette (bigger). Next I'll likely step to other low-collision
  §6 items or a quick-add hotkey.

## 2026-06-18 — fix: UX polish §6.10 — Allocate score bar labelling

- Picked a narrow clean-screen item from §6.10: allocation score bars were visual-only fills. `AllocRow`
  now computes one clamped whole-percent score, shows it as a localized inline `Score N%` label, and
  exposes the track as `role="progressbar"` with `aria-valuenow/min/max` and an `aria-label`.
- Kept the existing right-aligned score/amount header unchanged; the new label makes the score readable
  next to the bar and gives assistive tech the same value. Added `allocate.scoreLabel` so the new visible
  text stays in the language catalog.

## 2026-06-18 — feat: §6.6 FlipPanel Enter-to-submit

- Added an Enter case to the FlipPanel's existing document keydown handler (the one that already does
  Esc-close + Tab-trap), captured `onSaveRef`/`closeOnly` alongside the existing `onCloseRef`. Enter runs
  save→close; skipped when `activeElement` is TEXTAREA (multi-line), BUTTON (let it click natively), or
  SELECT, and when CloseOnly (nothing to save). One file, `internal/ui/flippanel.go`.
- Picked an `internal/ui` item deliberately: the parallel session is also in §6 now, so I'm taking the
  new-file / ui-package keyboard items it's unlikely to touch, leaving it the screen/CSS one-liners.
- Verified: gofmt clean (ran `gofmt -w` after the insert), wasm build exit 0, probe ok/200. Keystroke can't
  be sent headlessly — Enter-save is logic-verified; confirm in-browser.
- Next §6.6: a "?" help overlay (needs a shell mount + small overlay component) and a quick-add hotkey.

## 2026-06-18 — feat: §6.6 keyboard shortcut — Alt+1..9 section jump

- First of the §6.6 keyboard-shortcut items. New file `internal/app/shortcuts.go` +
  `wireKeyboardShortcuts()` wired in `Run()` after `wireResizeReveal()` (same once-at-boot global-listener
  pattern). Navigates via the package-level `router.Navigate` (UseNavigate just wraps it, so no component
  context is needed) to `primaryNav()[n-1].Path`.
- Robustness: keys off `KeyboardEvent.code` ("Digit1".."Digit9") — layout-independent and excludes the
  numpad (Alt+numpad = OS alt-codes, which report "Numpad1"); requires Alt without Ctrl/Meta; and bails
  when focus is in INPUT/TEXTAREA/SELECT/contentEditable so it never eats a keystroke mid-typing.
- Chose canonical `primaryNav()` order (not the user's reordered/hidden-filtered visible list) — that
  ordering logic lives in Sidebar via hooks I can't call from a global handler, and canonical order is
  stable/predictable. Navigating to a hidden screen still works (the route exists).
- Coordination note: the parallel session is ALSO picking §6 items now (it shipped a §6.4 add-menu radius
  fix). To avoid duplicate/colliding work I'm steering to the larger new-file 6.6 features rather than the
  CSS one-liners it's grabbing. New file = minimal collision surface; only `app.go` is shared (one added
  line). Atomic commit.
- Verified: gofmt clean, wasm build exit 0, probe ok/200. Can't send Alt+1 headlessly, so the nav itself
  is logic-verified — confirm in-browser.
- Next §6.6: a "?" help overlay documenting shortcuts (needs a shell mount point + a small overlay
  component), and a quick-add hotkey.

## 2026-06-18 — fix: UX polish §6.4 — add-menu radius utility

- Completed another low-collision §6.4 item in `internal/app/addmenu.go`: the top-bar **+ Add** button
  no longer uses an inline `Style{"border-radius": "4px"}` and now carries `rounded-[4px]` in its class
  list with the rest of the app styling. This keeps visual shape in the shared utility path and avoids
  clobbering focus/hover styling with ad-hoc inline CSS. The docs text was swept into the parallel
  keyboard-shortcut commit first; this atom carries the implementation and a small wording refinement so
  the commit still updates both journals.
- Scope is intentionally one-line UI cleanup + docs. `TODOS.md` is dirty from the parallel backlog/spec
  expansion, so I did not edit or stage it for this atom.

## 2026-06-18 — fix: UX polish §6.3/§6.4 — progress bar height + switcher separator

- Two tiny low-collision §6 items in non-screen files: `ui/progress.go` track `h-1.5`→`h-2`, and the
  workspace-switcher dropdown separator `my-1`→`my-2` (`wsswitcher.go`). Go changes, so wasm rebuilt;
  gofmt clean, build exit 0. Atomic commit.
- Remaining §6 is mostly in the parallel session's hot screen files (transactions/accounts/dashboard/
  shell) and the bigger 6.6 keyboard-shortcut features (command palette, ? overlay, Alt+1..9) which need
  shell/router wiring — those next, checking each target file is clean before editing.

## 2026-06-18 — fix: UX polish §6.11 — light-theme contrast + toggle size (CSS)

- Continued section 6, again pure `web/index.html` CSS (single shared file, no rebuild). Light-theme idle
  icon controls `.gear-inline`/`.gear-abs`/`.menu-btn`/`.set-close` were `#8a8a90`/`#8a8a92` on `#f7f6f3`
  (~2.7:1) → `#6a6a72` (verified ~5:1, clears the 3:1 UI / 4.5:1 text thresholds). Note `.set-close`
  wasn't in the existing light-theme override group, so I added it.
- Settings `.switch` 36×21 → 40×24 (24px clears the touch-target minimum); knob 17→18px, travel recomputed
  (left 3→19). Both `.switch::after` and `.switch.on::after` updated together.
- Verified probe ok/200. Atomic commit. Next §6: 6.4 [L] switcher separator spacing (my own wsswitcher.go,
  low collision), then the 6.6 keyboard-shortcut features (new files).

## 2026-06-18 — fix: UX polish §6.1–6.2 — WCAG contrast + touch targets (CSS)

- User redirected the loop to TODOS section 6 (the 2026-06-18 UX/UI polish audit). Started with the two
  cheapest-but-high-value, lowest-collision clusters: both are pure `web/index.html` CSS, so a single
  shared file, no Go/wasm rebuild.
- 6.2 contrast: `faint` #6c6c72→#7d7d85 (verified ~4.7:1 on #0e0e0f, was ~3.1:1) and `dim`
  #a6a6ac→#ababb3, in BOTH the Tailwind config block and the `:root` vars (two color systems coexist).
  `.insight-dot` 1.05rem→1rem.
- 6.1 touch targets: `.field` padding up + 38px min-height (36px floored under compact density); to-do
  `.check` made a centered 24×24 inline-grid with a right margin; `.btn-del` padding bumped within its
  existing 24×24; color picker 46×34→44×44. (`.btn-del`/`.toast-x`/`.set-close` already had the 24×24 hit
  area from a prior fix.) Skipped the screen-file parts of 6.1 (transactions.go checkbox markup,
  custompagesnav ⋯, shell rail min-size) — those are the parallel session's hot files; deferred.
- Did NOT touch TODOS.md to check the items off — it's perpetually dirty from the parallel session; the
  items addressed are 6.2 (faint/dim/insight-dot) and the CSS parts of 6.1 (field/check/btn-del/color).
- Verified: probe ok=true / 200 (CSS can't break the wasm; confirmed it still boots). Visual contrast/size
  needs an eyeball pass in-browser (probe can't read computed styles). Atomic commit per the git-tree memory.
- Next in §6: 6.11 light-theme contrast (index.html only, low collision), then the keyboard-shortcut
  features in 6.6 (new files, e.g. Alt+1..9 section jump / Cmd+K palette).

## 2026-06-18 — feat: reorder workspaces (arrange the switcher)

- `Registry.Move(id, toIndex)` — clamps toIndex into range, order-preserving, no-ops on unknown id /
  single-element list / same-position. Crucially leaves ActiveID and StartupID alone (they're id-tracked,
  not index-tracked) — covered explicitly in the table test alongside first↔last, middle, clamp, and
  no-op cases. Used a full-slice-expression (`out.Workspaces[:from:from]`) on remove to avoid aliasing the
  cloned backing array before re-inserting.
- App: `moveWorkspace(id, toIndex)` persists; no reload (switcher + management list re-read the registry).
- UI: up/down arrows in each `wsManageRow` (new Index/Total props; workspacesSection passes them). Both
  arrows always render — Move clamps, so a keyboard activation of a dimmed boundary arrow is a harmless
  no-op; the dim (`opacity-30 pointer-events-none`) is just a hint. Two i18n keys.
- Clean window this time: tree was idle (only TODOS.md), so the atomic add+commit went through without the
  en.go write-races of the previous feature. Tests pass, gofmt clean, wasm build exit 0.

## 2026-06-18 — feat: workspace export / import (move a whole context as a file)

- Another isolated workspace-layer feature (the parallel session is churning store/domain/screens, so I
  stayed out). Reused the existing `downloadBytes`/`pickFile` DOM helpers — no new file plumbing.
- `wsExport` envelope `{version, name, color, bundle}` where bundle is the workspace's perWorkspaceKeys
  snapshot (live `bundleCurrent()` for the active one, else `loadBlob`). No secrets: the OpenAI key is
  user-global, not in perWorkspaceKeys. `exportWorkspace` marshals + downloads `workspace-<slug>.json`
  (new `slugify` helper). `importWorkspace` parses, rejects a nil/!bundle file (returns false → caller
  alerts `ws.importErr`), else adds a new workspace from the envelope (palette color if none), saves its
  blob, switches + reloads (current bundled out first). applyBundle only writes known perWorkspaceKeys, so
  stray keys in an import are harmlessly ignored.
- UI: per-row **Export** in `wsManageRow`; a section-level **Import workspace** button. 4 i18n keys.
- Process: committed atomically (single `git add <files>; git commit` shell call) per the new
  [[cashflux-parallel-git-tree-hazard]] memory, after the last feature got swept into the parallel
  session's commit. en.go required three Read→Edit retries — the other session was writing it live.
- Verified: i18n + workspace tests pass, gofmt clean, `GOOS=js GOARCH=wasm` build exit 0, probe healthy.
  File download/upload can't be driven headlessly (probe is read-only), so export/import round-trip is
  logic-verified — confirm in-browser.

## 2026-06-18 — feat: per-workspace color (tell contexts apart at a glance)

- Followed the user's "add other useful settings like this" invite with a self-contained, isolated
  workspace feature (deliberately avoided the obvious "open to screen X" startup setting — its boot-time
  routing is entangled with the parallel session's just-landed base-href/deep-link work, high collision
  risk). Color lives entirely in my layer: model + app state + the switcher UI I own.
- Model (`internal/workspace`): `Workspace.Color` (CSS color string, omitempty) + `Registry.SetColor`
  (unknown-id no-op; clone copies it by value so it survives Rename/SetActive). Table test covers set,
  isolation to one workspace, no-op, clear, and rename-preserves-color.
- App: `setWorkspaceColor`; a six-swatch `workspacePalette` + `paletteColor(i)` cycling helper. New
  workspaces (createWorkspace/duplicateWorkspace) and the initial Default auto-get a distinct palette
  color by creation order, so they're visually distinguishable out of the box.
- UI (`wsswitcher.go`): a `wsColorDot` helper; the expanded switcher trigger and each dropdown row show
  the dot; the collapsed-rail glyph tints its border with the color; `wsManageRow` gains the reusable
  `ui.SwatchPicker` to change a workspace's color (writes through `setWorkspaceColor`, re-renders via the
  panel's bump). No new i18n key (the swatch is inline). No `en.go`/`settings.go`/`app.go` changes —
  smaller shared-file surface than the last two commits.
- Verified: workspace tests pass, gofmt clean, full `GOOS=js GOARCH=wasm` build exit 0 (the parallel
  `internal/screens` build is healthy again), probe ok=true / status 200 / switcher + dashboard render.
  Couldn't headlessly open the switcher dropdown or settings to eyeball the swatches (probe is read-only),
  so the dot/picker are build-verified — confirm visually after a hard reload.

## 2026-06-18 — feat: custom pages — widget engine + grid rendering (Phase B)

- `internal/screens/custompage.go` now renders a page's `[]PageWidget` as a bento grid: `dashlayout.Pack`
  on the page's `[]dashlayout.Item` gives each tile its `grid-column/row`; a local `customTile` component
  (its own delete hook) wraps each body. Did NOT reuse `ui.Widget` — it's wired to the dashboard's global
  layout atom, so custom pages get their own lightweight tile.
- Widget bodies: KPI (`widgetspec.EvalKPI` over `engineenv.Vars`, currency/number/percent formatting),
  List (transactions/accounts/budgets/goals/tasks, N rows, using `ledger.Balance`/`goals.Percent`), Chart
  (net-worth trend via `ledger.NetWorthSeries` + `chartspec` + `ui.Chart`), Text (authored config text).
- `addWidgetBar`: a single stable component (form hooks not in a loop) — pick type, title, and one binding
  (KPI formula / list source / text), appends a `PageWidget` + a `dashlayout.Item` and persists. Per-tile
  remove. Re-render via a version-counter refresh callback threaded to tiles + the bar.
- Name clash: `kpiBody` already existed in dashboard.go → renamed mine `cpKPIBody`.
- Verified end-to-end with a Go-generated seed dataset (a page with all four widget types) loaded into
  localStorage, then screenshotted /p/demo: KPI showed the live net worth ($14,120), the list showed recent
  transactions, the trend chart drew (D3 SVG — works because of the earlier createElementNS fix), and the
  text note rendered. The seed/generator were throwaway and removed.
- Next: Phase C (artifacts: images + datasets persisted, Image/Table widgets) and Phase D (workflow engine).

## 2026-06-18 — feat: startup workspace preference (pin a workspace to open with)

- Requested: configure whether the app starts with a specific workspace or resumes the last-used one.
- Model (`internal/workspace`): added `Registry.StartupID` (empty = resume last-active; set = pin) plus
  `SetStartup` and `StartupTarget` (resolves the boot target: pinned-if-still-exists else active).
  Subtlety caught: `clone()` and `Remove()` had to be updated to carry `StartupID`; `Remove()` also
  clears the pin when the pinned workspace is deleted, so launch never targets a ghost. Table tests cover
  pin/unpin, unknown-id no-op, dangling-pin fallback, clone preservation across Rename/SetActive, and the
  remove-clears-pin / remove-other-keeps-pin cases.
- Boot (`internal/app`): new `applyStartupWorkspace()` runs in `Run()` between `ensureWorkspaceRegistry`
  and `hydrateDataset`. If a workspace is pinned and isn't the one whose data sits in the canonical keys,
  it bundles the last-active workspace out and the pinned one in — no reload needed because nothing has
  mounted/read the keys yet, so `hydrateDataset` just loads the swapped-in context. `setStartupWorkspace`
  persists the choice (takes effect next launch).
- UI: `workspacesSection` gained a `wsStartupSelect` component (its own component for a stable OnChange
  hook) — an "On launch, open" dropdown listing *Last used workspace* + every workspace; mirrors the
  language `Select`/`Option`/`SelectedIf` pattern. Two i18n keys added.
- Concurrency note: the parallel custom-pages/widgets session is mid-refactor and currently has the
  `internal/screens` package non-compiling on disk (moving `kpiBody` into `custompage.go` with a new
  signature while `dashboard.go` still declares/calls the old one). That breaks a full local wasm build,
  but it's entirely their uncommitted WIP — my changes have zero wasm build errors. Verified my commit
  yields a buildable HEAD by copying my six files into a detached worktree at HEAD and building
  `GOOS=js GOARCH=wasm` there: exit 0, workspace tests green. Could NOT refresh the locally-served wasm
  (the on-disk screens break blocks `gwc dev`'s rebuild); CI builds HEAD fresh, so the deploy is fine.
- Other useful settings to consider next (suggested, not yet built): "On launch, open screen" (last
  screen vs. a fixed landing route), and confirm-before-switch when a workspace has unsaved edits.

## 2026-06-18 — feat: collapsed-rail variant for the workspace switcher

- Re-applied (cleanly, mine-only) the previously-reverted polish: `WorkspaceSwitcher` now reads
  `uistate.UseRailCollapsed()` and, when collapsed, renders a compact 36px icon-only square showing the
  active workspace's initial (`workspaceInitial`) instead of the full-width name + ▾ button, which was
  cramped/clipped in the 58px rail. The dropdown menu flies out to the right (`left-full ml-1 w-48`)
  rather than stretching edge-to-edge, so workspace names stay readable. Hover title carries the full
  name + "Switch workspace". Expanded rail unchanged.
- Kept the change entirely inside `internal/app/wsswitcher.go` (inline Tailwind utilities, no CSS) to
  avoid touching `web/index.html`, which the parallel custom-pages session has dirty (base-href work).
- Hooks: `UseState` then `UseRailCollapsed` are both called unconditionally before any early return, so
  hook order stays stable across the collapsed/expanded branches.
- Verified: wasm builds, gofmt clean, probe healthy (`ok:true`, status 200, 0 console errors, "Default"
  switcher + $354,070 dashboard render). Note: the headless probe can't toggle the rail, so the collapsed
  branch is build-verified; confirm the compact glyph + right-flyout in-browser by collapsing the rail.
- Tooling note: `.tools/gwc.exe` was rebuilt and `probe` now takes the URL **positionally**
  (`gwc probe http://127.0.0.1:8080/index.html`), not via `-url=`.

## 2026-06-18 — fix: new workspace starts empty (was cloning the current sample)

- Bug (user-reported): "the profile switcher works but it clones all the data from the prior selected
  account instead of showing empty values." Creating a workspace showed the same accounts/net worth as
  the one you were on.
- Root cause: `createWorkspace` cleared the per-workspace UI keys and the `cashflux:dataset` key, then
  reloaded. Boot's `hydrateDataset` treats an *empty* dataset key as first-run and re-seeds the demo
  sample — so the "new" workspace came up as the Michael Brooks sample, indistinguishable at a glance
  from the current sample-based workspace. (Confirmed it was the re-seed path, not an autosave clobber:
  the `suspendAutosave` guard is correctly set before the reload.)
- Fix: added `store.EmptyDataset()` (one default "You" member, USD base, no financial data) and made
  `createWorkspace` persist `store.Export(store.EmptyDataset())` into the dataset key instead of leaving
  it empty. Now a new workspace boots to a genuine clean slate. `duplicateWorkspace` is untouched — that
  remains the intentional "copy this workspace's data" action.
- Tests: `TestEmptyDataset` asserts no accounts/txns/budgets/goals, exactly one member, USD base, and a
  lossless export→import round-trip (the same path boot takes). `go test ./internal/store/...` +
  `./internal/workspace/...` green; gofmt clean; wasm rebuilt + probe shows the app healthy (Default ▾,
  net worth, 0 console errors).
- Note: the headless probe can't click "+ New workspace" (native prompt + no click driver), so the
  empty-workspace flow is logic/test-verified; confirm in-browser with a hard reload.
- Next: re-apply the collapsed-rail polish to `WorkspaceSwitcher` (hide it when the rail is collapsed)
  once the tree is clear of the parallel custom-pages work.

## 2026-06-18 — feat: custom pages — "My pages" rail group (Phase A cont.)

- Added `internal/app/custompagesnav.go` (`CustomPagesNav`): renders the "My pages" rail section from
  `appstate.CustomPages()` via `pages.Visible`, each row a `navItem` to `/p/<slug>` (so click/drag/flyout
  reuse the built-in nav), plus a "New page" action (prompt → `pages.UniqueSlug` + `id.New` + `NextOrder` →
  `PutCustomPage` → navigate). Drag-reorder persists via `pages.Reorder` + `PutCustomPage`. Wired into
  `Sidebar` with one line after the System group.
- Gotcha: a component rendered via `CreateElement` that returns a **bare `Fragment`** did NOT keep its
  sibling position among the `MapKeyed` rail groups — "My pages" jumped to the top of the rail. Returning a
  single root `Div` fixed the ordering (verified: section now sits after System, above the household card).
  Worth remembering for any rail/section component.
- Verified in a headless browser (built to `web/bin/main.wasm`, `gwc serve`, Edge screenshot): "My pages"
  + "New page" render with icons in the correct spot. (Create flow uses `window.prompt`, which headless
  blocks, so that path is logic-verified rather than click-verified.)
- Concurrency: the parallel workspaces agent had already committed its rail switcher + i18n; my
  `shell.go`/`en.go` hunks were cleanly just-mine on top, so this committed without entangling its code.
- Next (Phase A wrap): rename/delete/hide page management; then Phase B (widget engine + grid rendering).

## 2026-06-18 — feat: workspaces — Phases 2+3 (engine + UI), wired & browser-verified

- Phase 2 (engine, `internal/app/workspace.go`): the active workspace keeps using the canonical `cashflux:*`
  keys exactly as before; inactive ones are bundled into `cashflux:ws-data:<id>`. `switchWorkspace` bundles
  the current keys out, restores the target's, marks it active, and `location.reload()`s so boot rehydrates
  everything — no per-atom re-seeding and zero changes to the 12 uistate stores. `createWorkspace` clears
  the per-workspace keys (boot seeds the sample), `duplicateWorkspace` copies the current bundle,
  `rename`/`delete` operate in place (delete-active reloads to a survivor). `ensureWorkspaceRegistry` (called
  in `Run` before hydrate) migrates an existing single dataset into a "Default" workspace.
- Race guard: added `suspendAutosave` in persist.go — set before a switch's reload so the dying page's
  pagehide/ticker save can't clobber the swapped-in `cashflux:dataset`.
- Phase 3 (UI): `WorkspaceSwitcher` at the top of the rail (active name + ▾ → menu: switch rows + New +
  Duplicate; per-row `wsMenuItem` for stable hooks) and a Settings → Workspaces section (`wsManageRow`:
  rename via prompt, delete via confirm, hidden when only one remains). New `ws.*` i18n keys; names via a
  browser prompt. The swap reloads the page (sub-second, cached wasm) rather than hot-swapping atoms.
- Verified via `gwc probe` on the live dev server: fresh session auto-creates "Default", the rail shows
  "Default ▾", the dashboard renders the seed, zero console errors. Couldn't headlessly click the menu
  (probe loads+captures only), so the switch/new/duplicate and rename/delete click-throughs rest on the
  unit-tested registry + a healthy render — flagged for manual confirmation. Full suite 43 green.

## 2026-06-18 — feat: workspaces — Phase 1 (pure registry)

- User wants multiple independent "workspaces" (e.g. real money vs. an experimental sandbox) with quick
  switching. Decided: terminology "workspace"; a swap changes *everything* (dataset + all UI/layout/settings);
  the OpenAI key/env stays user-global (the per-workspace OpenAI model lives in the dataset's Settings).
- Plan (bottom-up, commit per layer): (1) pure registry — this commit; (2) workspace-scoped localStorage
  (namespace every `cashflux:*` key by the active workspace id, except the user-global `cashflux:openai-key`;
  migrate existing keys into a "Default" workspace); (3) switch/create/duplicate/delete wiring + tests;
  (4) rail quick-switch dropdown + Settings management.
- Phase 1: new pure `internal/workspace` package — `Workspace{ID,Name}` + `Registry{Workspaces, ActiveID}`
  with `Add`/`Rename`/`SetActive`/`Remove`/`Active`/`Has`/`Get`. Rules encoded + table-tested: first add
  becomes active, duplicate/empty ids ignored, `Active()` falls back to the first when ActiveID dangles,
  removing the active promotes the first survivor, the last workspace can't be removed. ID generation is the
  caller's job so the package stays deterministic. Tests green, gofmt clean.

## 2026-06-18 — feat: custom pages — persistence (Phase A cont.)

- Wired `CustomPage` through the store with the existing JSON-in-SQLite seam: `custompages` table,
  `Dataset.CustomPages`, `Load`/`Snapshot`, the 4-method CRUD in `crud.go`, and appstate
  `CustomPages`/`PutCustomPage`(validates id+name+slug)/`DeleteCustomPage`. Exactly the documented 7-step
  pattern — no new mechanism.
- Extended `sampleDataset()` with a page (layout + bound KPI widget) so both round-trip tests
  (`Export→Import` byte-equality and SQLite `Load→Snapshot`) prove pages are lossless, including the nested
  `dashlayout.Item` layout and `widgetcfg.Config`/`WidgetBinding`. Green; vet/gofmt clean.
- Note: a parallel "workspaces" agent was committing at the same time and its `git add -A` swept my
  uncommitted `sqlitestore.go`/`dataset.go` edits into its commit (`fb689b2`); committed the remainder
  (crud/appstate/test) with scoped `git add` to avoid clobbering its in-flight files.
- Next: `/p/:slug` routing + an empty page screen.

## 2026-06-18 — feat: custom pages — Phase A starts (data model + pure logic)

- Kicked off the big "custom pages + widget engine + workflow engine + artifacts" feature (plan agreed
  with the user first, per the spec rule; decisions: widgets config-driven first then scripting, workflows
  = rules AND sequences, artifacts in the SQLite dataset, read+write access with dry-run+audit).
- Bottom-up, started at the data model. Added `domain.CustomPage` (id/slug/name/icon/order/hidden + a
  per-page `[]dashlayout.Item` layout + `[]PageWidget`), `PageWidget` (type/title/`widgetcfg.Config`/
  binding), and `WidgetBinding` (declarative data source: source/filter/expr/artifactId/columns). Chose to
  store page layout/widgets **in the dataset** (not localStorage like the built-in dashboard) because a
  custom page is user content that must export/import — keeps it consistent with "persist artifacts."
- `domain` now imports `dashlayout` + `widgetcfg`; both are dependency-free leaf packages, so no import
  cycle. Verified the dependency direction before wiring it.
- New pure `internal/pages` package mirrors the `navorder` style: `Slug`/`UniqueSlug` (deterministic,
  collision-suffixed), `Ordered`/`Visible`/`NextOrder`, `Reorder` (move + renumber Order so the caller can
  persist every page), `BySlug`/`ByID`, `Validate`. Avoided pulling in `strconv` for one int format.
- Tests: table-driven, green; `gofmt`/`go vet` clean; full `go test ./...` still passes (the domain change
  is additive). Committed as one feature.
- Next (Phase A cont.): persist pages in the store (custompages table + CRUD + roundtrip), then `/p/:slug`
  routing + an empty page screen, then the "My pages" nav group with create/rename/delete/reorder/hide.

## 2026-06-18 — fix: invisible icons (SVG namespace bug in the framework)

- Report: can't see the left-rail icons, and can't see the button that collapses the sidebar.
- Diagnosis: the collapse toggle (`app.TopBar`, `.menu-btn`) is icon-only, so when its icon doesn't
  paint the whole control is invisible — same root cause as the rail icons. Traced to the framework: the
  wasm DOM adapter (`internal/platform/jsdom/adapters.go`) created every node via
  `document.createElement`, i.e. the HTML namespace. SVG elements only render when created with
  `document.createElementNS(...)`, so all inline `ui.Icon` SVGs were invisible. The SSR/string path
  serializes `xmlns` correctly, which is why the framework's SVG tests passed while the live app showed
  nothing. The `DOMAdapter` interface had no namespace-aware creation, and a deliberate guard
  (`TestDOMAdapterHasNoRawHTMLSink`) rules out an `innerHTML` escape hatch.
- Decision (asked the user): fix the root cause in the framework AND re-pin, rather than work around it in
  CashFlux. The CSS-mask data-URI workaround was therefore unnecessary — adding it would have been dead,
  redundant code on top of a working fix, so it was deliberately skipped.
- Framework fix: routed SVG-only tags through a cached `createElementNS`, and made
  `CreatePreparedElement` skip the HTML-string template fast path for SVG (it can't produce SVG nodes).
  No raw-HTML sink introduced — attrs/text still set via `setAttribute`/`textContent`. Committed +
  pushed to GoWebComponents (`bfe3011d`), then `go get`-bumped the pin here.
- Gotcha that caused a false "still broken": the served wasm is `web/bin/main.wasm`, but the CLAUDE.md
  "build wasm directly" snippet targets `static/bin/` — rebuilding there left the live app on the old
  binary. Rebuilt into `web/bin/` and screenshot-verified (headless Edge against `gwc serve`): every rail
  icon, the collapse toggle, the household gear, and the tile grips/gears now render.
- Next: the build-output path mismatch (`static/bin` vs `web/bin`) is a footgun — worth reconciling the
  CLAUDE.md snippet / dev flow so there's one canonical wasm output location.

## 2026-06-18 — seed data: a middle-aged single homeowner persona

- User request: seed the data for a middle-aged single man. Confirmed three choices up front (AskUserQuestion):
  replace the existing sample (vs. add a second seed), homeowner-with-mortgage (vs. renter), and several
  months of history (vs. one month).
- Rewrote `store.SampleDataset()` as Michael Brooks, 46, single homeowner. Balance sheet: Everyday Checking,
  High-Yield Savings, Brokerage/401(k), Home (TypeOther asset), Mortgage, Auto Loan, Credit Card — with
  liquidity/stability/expected-return scores set so the Allocate screen is meaningful. Three months
  (Apr–Jun 2026) of recurring activity generated in a loop: salary in, ~14 categorized bills/expenses from
  checking, and monthly transfers to savings and the brokerage (both legs, so those balances and the
  net-worth trend climb). Small per-month variation (`v`) keeps charts from being flat; June's later
  activity is left uncleared to look like a real mid-month ledger.
- Kept the app's existing convention: liabilities are static balances and their payments are categorized
  expenses (no payment-transfer plumbing). Five monthly budgets (groceries near-limit, others comfortable),
  three goals linked to savings/brokerage, three tasks.
- Net worth lands ~\$354k (≈\$640k assets − \$286k debt) — plausible for the persona. All ids are stable so
  reload stays idempotent. `TestSampleDatasetIsValid` (validates every entity) passes; full suite 41 green,
  gofmt clean, wasm builds.

## 2026-06-18 — fix stray nested wasm re-tracking (.gitignore mid-slash anchoring)

- A clean-state check caught `internal/screens/static/bin/main.wasm` tracked *again* after I thought I'd
  untracked it. Cause: a `.gitignore` pattern with a slash in the middle (`static/bin/`) is **anchored to
  the repo root**, so it never matched the nested path — and the untracking commit's `git add -A` re-added
  the on-disk file. Classic gitignore gotcha.
- Fixed the pattern to `**/static/bin/` (the leading `**/` matches at any depth; `git check-ignore` now
  confirms both the root and the nested wasm are ignored) and deleted the stray
  `internal/screens/static/` dir outright (it only ever held the accidental build artifact). No tracked
  `.wasm` remains and it can't be re-added now.

## 2026-06-18 — untrack stray bin/ screenshots; ignore bin/ wholesale

- Follow-on to the wasm untracking. The four `bin/*.png` review screenshots (dash/dash2/mobile/mobile2)
  were the only remaining tracked things under `bin/`. Grepped the repo — nothing references them (only my
  own prior DEVLOG note), and review captures are meant to live in the already-ignored
  `.review-screenshots/`. So they're misplaced strays. Untracked them (`git rm --cached`, local kept) and
  simplified `.gitignore` to ignore `/bin/` wholesale (matching CLAUDE.md's "bin/ is git-ignored"),
  replacing the earlier per-extension `/bin/*.wasm` patterns. `bin/` now has nothing tracked.

## 2026-06-18 — stop committing wasm build artifacts (stale .gitignore)

- The full-suite check's `git diff --stat` kept showing `static/bin/main.wasm | Bin 27MB` change every
  commit — I'd been re-committing a 27 MB binary each cycle via `git add -A`. Root cause: `.gitignore`
  ignored `/web/bin/` (an old path) but the local build command writes to `static/bin/main.wasm`, which
  wasn't ignored, so it got tracked.
- Verified deployment safety before touching it: `.github/workflows/deploy-pages.yml` builds the wasm fresh
  in CI (`go build -o web/bin/main.wasm`), copies wasm_exec.js, and uploads `web/` as the Pages artifact —
  it never uses the committed `static/bin` or root `bin` wasm. So those are local-only and safe to untrack.
- Found four tracked wasm artifacts: `static/bin/main.wasm`, `bin/main.wasm`,
  `bin/main.wasm.hotreload-manifest.json`, and a stray `internal/screens/static/bin/main.wasm` (collateral
  from the earlier persisted-`cd` mishap). `git rm --cached` all four (local files kept), and fixed
  `.gitignore`: `static/bin/` (unanchored, so it also catches the nested stray), `/static/wasm_exec.js`,
  `/bin/*.wasm`, `/bin/*.hotreload-manifest.json`. Left `bin/*.png` (screenshots — possibly referenced).
- Net: no tracked `.wasm` remains; future commits won't carry the binary. Full suite still 41 green, gofmt
  clean, wasm builds. (Process note to self: gofmt every hand-edited .go file before committing — two prior
  commits slipped through dirty.)

## 2026-06-18 — extract spend-breakdown ranking into ledger.RankSpending (+ i18n the labels)

- The broader scan found one more real chunk of view-embedded logic: the dashboard breakdown widget's
  rank-categories-by-spend → top-N → collapse-the-rest-into-"Other". Extracted the pure part to
  `ledger.RankSpending(totals, n) (top []CategoryTotal, other int64)`, table-tested (sort order, the
  n+1-keep-all threshold, tail collapse sum, n<=0, empty). The view keeps name resolution + bar/legend
  rendering, now calling RankSpending.
- Bonus: the breakdown's "Other" and "Uncategorized" were hardcoded English — added `dashboard.other` /
  `dashboard.uncategorized` keys and the view passes localized labels (the extraction made the label
  injection natural). i18n + ledger tests + wasm build green.
- The other view helpers scanned (uistate's loadItems/resolveTheme/loadRailCollapsed, the dashboard
  bills/segment *rendering*) are platform glue (localStorage/JS) or display — correctly staying put.

## 2026-06-18 — consolidate account-by-id lookup into domain.AccountByID

- `accByIDFrom` (documents, 3 uses) and `accountName` (goals) each had their own linear "find account by id"
  scan. Added `domain.AccountByID(accounts, id) (Account, bool)` — pure, table-tested; chose `domain` as the
  home because both files already import it (no new imports) and it's a query over a core type. documents
  calls it directly (local `accByIDFrom` removed); goals' `accountName` now delegates to it.
- Gotcha: the test first compared the not-found result with `!= Account{}`, which doesn't compile —
  `Account` has a `map[string]any` (Custom) field and so isn't comparable. Switched to checking `a.ID == ""`.
  domain tests + wasm build green.

## 2026-06-18 — move firstNonEmpty to textutil; view-logic extraction wrapping up

- Moved the documents view's `firstNonEmpty(a,b)` to `textutil.FirstNonEmpty` (pure, table-tested,
  whitespace-as-empty); rewired its two call sites, removed the local def. wasm build + tests green.
- This effectively wraps the view-logic extraction vein. The genuinely-pure, logic-bearing helpers that were
  buried in `screens` are now extracted and tested: `recentTransactions`→`ledger.Recent`,
  `parseTags`/`parseOptions`→`textutil.CommaFields`, `parseFloatOrZero`/`parseIntOrZero`/`parseWeight`→
  `textutil.ParseFloat`/`ParseInt`, and `firstNonEmpty`→`textutil.FirstNonEmpty`. What remains in the view
  (`fmtMoney`, `figTone`, `amountClass`, `accentFor`, `humanizeType`, `*Label`, `catColor`, `indentLabel`,
  the insights `highlight*` mappers, `toDocumentRows`' 1:1 field copy) is legitimately presentation/glue —
  extracting it would add coupling for no real logic, so it stays.

## 2026-06-18 — consolidate numeric form parsing into textutil

- Continued the view-logic extraction. `parseFloatOrZero`/`parseIntOrZero` (accounts.go) and `parseWeight`
  (allocate.go) were three untested view-layer takes on "parse a number from a form field, tolerate junk".
- Added `textutil.ParseFloat`/`ParseInt` (TrimSpace + parse, 0 on error), table-tested (blank, spaces,
  garbage, negatives, non-integer for ParseInt). accounts' five call sites now use them and the two local
  defs are gone; allocate's `parseWeight` delegates to `textutil.ParseFloat` while keeping its
  non-negative clamp. `strconv` stays imported in both screens (other uses); `strings` too. gofmt + wasm
  build + textutil tests green.
- (`trimWeight` left in allocate — it's float→string display formatting, a view concern, not logic.)

## 2026-06-18 — unify comma-list parsing into tested textutil.CommaFields

- Continuing to pull pure logic out of the view layer. Found `parseTags` (defined in transactions.go, used
  there and in rules.go) and `parseOptions` (customfields.go) were *functionally identical*: split on commas,
  trim each, drop empties, nil when none. Two untested copies of the same logic.
- Created `internal/textutil` with `CommaFields(s) []string` (pure, table-tested: empties, all-separators,
  trimming, order, inner spaces) and rewired all four call sites to it; removed both local copies. gofmt
  reordered the new imports. `strings` stays imported in both screens (still used elsewhere). wasm build +
  textutil test green.
- (`validateRuleInput` is also pure but returns i18n keys — more view-coupled — so left for now.)

## 2026-06-18 — extract dashboard "recent transactions" into tested ledger.Recent

- New vein of work the user's "keep going" surfaced: pure logic still embedded in the wasm-only view
  packages (0% native coverage), against CLAUDE.md's "never put computation in view code". Swept `screens`
  for pure helpers; `recentTransactions` (copy → sort newest-first → take N) was the most logic-bearing.
- Moved it to `ledger.Recent(txns, n)` (`recent.go`) — ledger is pure and transaction-focused, and `screens`
  already imports it (no cycle). Added a negative-n guard (the inline version would panic on n<0). Table
  tests in `recent_test.go`: ordering, top-N limit, n>len returns all, n≤0 empty, and input not mutated.
  Rewired `dashboard.go` to call it; removed the local copy. ledger tests + wasm build green.
- More such helpers remain in `screens` (parseTags, parseWeight/trimWeight, validateRuleInput, plural, …);
  I'll extract the logic-bearing ones over subsequent commits.

## 2026-06-18 — widgetcfg IDs() to 100%

- Last pure package under 90%. `widgetcfg` was 88.1% with one untested function, `IDs()` (the sorted
  registered-widget list). Added `ids_test.go` asserting it's non-empty, sorted, every id is a real schema
  (consistent with Has/SchemaFor), and includes the known widgets. `widgetcfg` → **100%**.
- With this, every pure logic package is ≥90% (the great majority 95–100%); only `store` sits lower (84%),
  capped by partial-failure injection paths in Load/Snapshot that aren't cleanly reachable. The coverage
  sweep is genuinely complete.

## 2026-06-18 — custom-field validation enforced on every write (91% → 92%)

- Verified the data-integrity guarantee that a required custom field is enforced on every entity write that
  supports custom fields, not just accounts. Added `custom_validation_test.go`: registers a required custom
  def for member/transaction/budget/goal, then confirms an otherwise-valid entity that omits the value is
  rejected — the `validateCustom`-rejection branch each of those `Put*` methods had left uncovered (they were
  at 75%; PutAccount was already covered by `TestPutAccountValidatesCustomFields`). Those methods → 87.5%;
  `appstate` → **91.8%**.
- **Coverage work concluded.** Every native package now sits at 84–100% (domain/validate ~100; insights 98;
  budgeting/goals/payoff 95–96; ledger 94; formula 93; id 91; appstate 92; store 84), error paths included
  (via the closed-DB/store technique). The remaining slivers are duplicative base-validation branches,
  construction-error injection, and unreachable safety backstops — not worth further churn. wasm/UI packages
  still need the browser lane (unavailable here).

## 2026-06-18 — appstate resolver + freshness-override paths (90% → 91%)

- Two more reachable feature paths: `idResolver` (the C27 name→id CSV resolver, callable directly since the
  test is in-package) and `FreshnessWindows`'s per-type override loop.
- Added `resolver_test.go`: `idResolver` across all three branches (exact-id passthrough, case-insensitive
  name match, unresolved passthrough) and a `FreshnessWindows` test that layers a household "checking" → 5
  override over the defaults. Both functions → **100%**; `appstate` → **90.7%**.

## 2026-06-18 — appstate business-logic branches (87% → 90%)

- The reachable business-logic gaps after the error-path pass: `PutCustomFieldDef`'s rejection of an
  invalid definition (55.6%) and `ReassignOwner`'s per-entity move loops + individual-target path (62.5% —
  the existing test only reassigned an account+goal to the group owner).
- Added `logic_test.go`: rejecting an incomplete custom-field def (and asserting it isn't stored), and a
  full `ReassignOwner` from one member to another with an account, budget, goal, and transaction (moved=4,
  ownership/scope flipped). `appstate` → **89.7%** (`PutCustomFieldDef` 100%, `ReassignOwner` 87.5%). The
  residual is mid-loop store-error returns (error injection partway through a reassign) — marginal.

## 2026-06-18 — appstate error-path coverage via a closed store (82% → 87%)

- Same seam-free technique as store: close the App's underlying store (`a.Store().Close()`) and the
  store-backed methods hit their error arms. Valid entities pass validation first, so each reaches the
  failing store call; accessors return nil but exercise `logErr`'s error branch.
- Added `errors_test.go`: a table of `Put*`/`Delete*`/`PutSettings` calls plus `ImportJSON` (fed valid JSON
  from a second open app so the *Load* fails, not the parse), `LoadSample`, `Wipe`, and `ExportJSON`, then
  the nil-returning accessors. `appstate` → **86.6%**. The residual is reassign/validateCustom partial
  branches and per-loop error arms — genuinely marginal now.

## 2026-06-18 — store error-path coverage via a closed DB (81% → 84%)

- Revisited the store error arms I'd earlier written off as needing a fault-injecting mock. They don't: a
  **closed** `*sql.DB` makes every Exec/Query/QueryRow fail, and `Close()` is already public — so the error
  branches are reachable with no production seam.
- Added `errors_test.go` running the helpers against a closed store: `putJSON`/`getJSON`/`deleteRow`/
  `queryRows`/`loadRows` (via PutMember/GetMember/DeleteMember/ListMembers/TransactionsByAccount),
  `GetSettings`/`PutSettings`/`Wipe`, and `Snapshot`/`Load`. `store` → **83.7%** (`deleteRow` 100%, the JSON
  helpers ~85–91%). The residual is per-table error branches in Load/Snapshot (only the first table's arm
  fires before returning) and unreachable `json.Marshal` failures.

## 2026-06-18 — payoff negative-APR branch (92% → 96%); coverage sweep complete

- Last reachable-gap package. `payoff` was 91.7%; the untested branch was the interest floor (a negative
  APR implies negative monthly interest, floored to 0). Added `payoff_edge_test.go` covering
  Project(100000, -12, 10000) → 10 months, no interest. `payoff` → **95.8%**; the only residual is the
  `maxMonths` safety cap, unreachable without a contrived >100-year simulation.
- **Coverage sweep summary.** Pure logic packages after the sweep: domain 100, validate 99, insights 98,
  budgeting/goals 95, payoff 96, ledger 94, formula 93, id 91; plus the already-high money/currency/
  customfields/i18n/etc. The infra packages (store 81, appstate 82) are capped by error-injection paths
  that would need a fault-injecting store mock — deliberately not added just for coverage. The wasm/UI
  packages (ui/screens/app/uistate) can't be measured natively and need the browser lane (unavailable here).

## 2026-06-18 — insights down-anomaly coverage (92% → 98%)

- Coverage sweep, anomaly detection. `insights` was 92.0%; the untested branches were the *down* path of
  `Detect` (a category whose spend fell) and `abs64`'s negative arm — the latter only runs inside the sort
  comparator, which needs ≥2 anomalies to fire.
- Added `insights_edge_test.go` with a three-series case: a +400% rise (Up), an −80% drop (Down), and a
  −10% drop that's below the 50% threshold (skipped). Two flagged anomalies exercise the sort/`abs64` path
  and the equal-magnitude tie-break by name. `insights` → **98.0%** (`abs64` 100%, `Detect` 96.7%).

## 2026-06-18 — validate to 99% (edge rules)

- Coverage sweep on the entity validator (it guards every write). `validate` was 92.2%; the existing tests
  covered the common problems, leaving small edge branches: `Error()` on empty Issues, `validCode`'s
  character-range arm (a 3-letter *lowercase* code), `ValidateAccount`'s invalid-type / negative-stability /
  negative-APR checks, and `ValidateTask`'s non-empty-but-invalid RelatedType.
- Added `validate_edge_test.go` for exactly those. `validate` → **98.9%** (Error/validCode/ValidateTask now
  100%, ValidateAccount 95.7%). The tiny residual is the class/type-mismatch `else if` and a couple of
  single `< 0` operands already covered on their `> max` side.

## 2026-06-18 — Cover appstate accessors, deletes, settings, CSV (64% → 82%)

- Largest remaining sweep target. `appstate` was 63.8%; the existing tests covered the rule/recurring/
  validation paths, but many state-seam methods were at 0%: the entity accessors (Categories/Tasks/
  CustomFieldDefs/CustomFieldDefsFor/FreshnessWindows), the `Store`/`Log`/`LogRing` handles, `PutTask`/
  `DeleteTask`, the `Delete*` family (member/category/transaction/budget/goal/customFieldDef), `PutSettings`,
  `ExportJSONRedacted`, and `TransactionsCSV`/`ImportTransactionsCSV` (with `idResolver`).
- Added `appstate_more_test.go`: accessor/handle checks on an empty app; a Task put→delete round-trip plus a
  validation-failure; a delete-each-entity sweep (mirroring the existing tests' valid literals — accounts
  need owner/scope/type/class, tasks need a valid priority, etc.); a settings round-trip asserting the
  manual export keeps the OpenAI key while the redacted export strips it; and a CSV export→import round-trip
  that exercises the name→id resolver. `appstate` → **81.7%**.
- The residual is store-error and ImportJSON/validateCustom error arms that need fault injection, plus parts
  of ReassignOwner/ReassignCategory — diminishing returns for now.

## 2026-06-18 — Cover the untested store CRUD wrappers (76% → 81%)

- Coverage sweep on persistence. `store` was 76.2%; the per-function profile showed the Get/Delete/List (and
  some Put) wrappers for **Member, Category, Transaction, Budget, Goal, Task** at 0% — the existing CRUD
  tests covered Account/Rule/Document/SavedInsight/Recurring/AllocProfile/Formula/Plan/CustomFieldDef but not
  these six core entities.
- Added `crud_more_test.go`: a Put → Get → List → Delete → Get-absent round-trip per entity, following the
  existing `TestAccountCRUD` shape (and `newStore(t)` helper). `store` → **81.4%**.
- The residual is `sqlitestore.go` infra (NewMemory/Load/Snapshot/replaceRows) and the generic helpers'
  marshal/scan error arms — DB-failure paths that need fault injection to hit; deferred as low-value.

## 2026-06-18 — Raise id test coverage (73% → 91%)

- Coverage sweep. `id` was 72.7%; the gaps were the **package-level** `NewWithPrefix` (0% — the existing
  tests only exercised the `Generator` method and package-level `New`) and the method's error path.
- Added `id_edge_test.go` (reusing the existing `errReader`/`errBoom` helpers): the crypto/rand-backed
  `NewWithPrefix("acc")` happy path and the generator's `NewWithPrefix` error propagation. `id` → **90.9%**.
  The residual is the two `panic`-on-randomness-failure branches in the package-level `New`/`NewWithPrefix`,
  unreachable without injecting a failing source into the fixed default generator (not worth a test seam).

## 2026-06-18 — domain enums to 100% coverage

- Coverage sweep, the foundational types. `domain` was 85.5%; the misses were the default (invalid) arms of
  several `Valid()` methods and `Period.Label`, which had no test at all.
- `TestEnumInvalid` already covered the invalid branch for AccountClass/AccountType/CategoryKind/TaskPriority;
  added `enums_edge_test.go` for the rest — invalid Scope/Period/TaskStatus/RelatedType/TaskSource — plus
  `TestPeriodLabel` over all three periods and the unknown→Monthly default. `domain` → **100.0%**.

## 2026-06-18 — Raise formula test coverage (89% → 93%)

- Coverage sweep, the sandboxed expression engine. `formula` was 88.7%; the existing tests covered the
  happy paths well, but several type-coercion/edge branches were untested: `asNumber`'s bool case,
  `truthy`'s string case, `evalBinary`'s string `!=` and the cannot-compare error, the unary `+`, and a few
  function arity/type errors (avg/min/max empty, round arity, a string passed to a numeric function).
- Added `eval_edge_test.go` driving each through `Eval`: bool→number coercion via `(2>1)+10`, unary `+`,
  `if("x",…)`/`if("",…)` for the string truthiness, `"a" != "b"`, and the error set (`"a" == 1`, `avg()`,
  `min()`, `max()`, `round(1,2)`, `sum("x")`). `formula` → **92.7%** (`asNumber` 100%). The residual is
  genuinely unreachable: `truthy`/`eval` default arms and the unknown-operator branch can't fire because
  Values are only float/string/bool and the parser only emits known operators.

## 2026-06-18 — Raise ledger test coverage (87% → 94%)

- Next package in the coverage sweep. `ledger` was 86.5%; the misses were the error branches of the
  money-aggregation functions — opening-balance currency mismatch, a transaction currency that differs from
  its account (failing `Add`), and a balance the base-only rate table can't convert.
- Added `ledger_edge_test.go`: opening-mismatch + Add-mismatch errors for `ClearedBalance`/`RunningBalances`;
  the unconvertible-currency error in `PeriodTotals`; both `NetWorth` and `NetByOwner` error paths
  (balance-compute and convert); plus a positive `NetByOwner` test for same-owner accumulation and the
  archived-account skip. `ledger` → **93.6%** (`ClearedBalance` 100%). The residual uncovered lines are
  defensive `Add`/`Sub` mismatch branches that can't trigger — conversions always return base currency, so
  the accumulator and operand always share it.

## 2026-06-18 — Raise budgeting test coverage (83% → 95%)

- Continued the pure-package coverage sweep. `budgeting` was 82.9%; per-function profiling showed two
  functions at 0% (`matches`, `EvaluateAll`) plus partial error/edge branches in `normalizedLimit`,
  `spentCovered`, `evaluateWith`, and `EnvelopeAvailable`.
- `matches` (the exact-category helper) turned out to be **dead** — superseded by the inline cover
  predicates in `Spent`/`Evaluate`, no callers. Removed it (same call the coverage audit made for `stub`).
- Added `budgeting_edge_test.go`: `EvaluateAll` happy + error path, the empty-limit-currency default
  (`normalizedLimit` → base), and the currency-conversion error path threaded through `Spent`, `Evaluate`,
  `EvaluateAll`, and `EnvelopeAvailable` (a covered expense in a currency the rate table can't resolve).
  `budgeting` → **95.1%** (`EvaluateAll`/`normalizedLimit` now 100%). The remainder is the defensive
  `limit.Sub(spent)` mismatch branch, unreachable since spend is always computed in the limit's currency.

## 2026-06-18 — Repo health check + raise goals test coverage

- Full-repo health pass after the session's commits: `gofmt -l` clean, native `go vet ./...` clean, native
  `go test ./...` all 40 packages green, wasm `go vet`/build green. No regressions, nothing to fix.
- Ran `go test -cover ./...` to find genuine gaps. Coverage is high across the board (most >85%); the lowest
  pure-logic package was `goals` at 82.1%. Per-function profiling showed the misses were error/edge branches
  (currency-mismatch errors in `Remaining`/`IsComplete`/`Project`/`Evaluate`, and `MonthlyNeeded`'s
  partial-final-month bump).
- Added `goals_edge_test.go`: currency-mismatch error paths for all four, the `Project`→`Remaining` error
  propagation, both `Evaluate` error branches (Remaining-first and Project-via-mismatched-monthly), the
  partial-final-month month-count round-up, and `MonthlyNeeded`'s mismatch path. `goals` → **94.6%**. The
  remaining ~5% is defensive/unreachable code (negative-`current` clamp; the `months < 1` guard that the
  `TargetDate.After(from)` check already precludes), not worth contorting tests for.

## 2026-06-18 — Remove dead `stub` screen placeholder

- Swept the Go source for `TODO`/`FIXME`/`HACK`/`XXX`/`BUG` markers — none (clean). The one real find was
  `screens.stub(...)`, the "Planned · Phase N" placeholder for not-yet-built screens. Grepped its call
  sites: zero (every screen is now a real component). Unused package-level funcs don't fail the Go build,
  so it had quietly become dead code, which CLAUDE.md forbids.
- Deleted it. `stat(...)` next to it is still used widely, so it stays. The dot-imported shorthand helpers
  it used (`Textf`/`Ul`/`Li`/`If`) are still imported for other functions, so no import churn. wasm build +
  `go vet ./internal/screens` green. (Left the now-orphaned `.badge-soon` CSS class in place — harmless and
  cheap to keep for any future placeholder.)

## 2026-06-18 — Move dashboard span math into pure dashlayout (with tests)

- The tile resize arithmetic (`cycleSpan` grow/shrink/wrap, `clampSpan` bound) lived as unexported helpers
  in `internal/ui/widget.go` — a wasm-only view package, so they had no native unit tests. That's
  computation in view code, which CLAUDE.md explicitly forbids ("never put computation in view code";
  "logic packages … unit-tested").
- Promoted them to `dashlayout.CycleSpan(cur, max, shrink)` and `dashlayout.ClampSpan(v, max)` in a new
  `span.go` (the package is already pure, no build tag, and is where the rest of the grid math lives), with
  table tests in `span_test.go` covering grow/wrap/shrink-floor and clamp bounds. `widget.go` now calls
  them and the local copies are deleted.
- Pure-Go refactor, no behavior change: `go test ./internal/dashlayout` passes, wasm build + `go vet
  ./internal/ui` green.

## 2026-06-18 — i18n: route the last hardcoded user-facing messages through the catalog

- Audited the screens for user-facing strings that bypass `uistate.T`. Grepped the error/notice setters
  (`errMsg.Set`, `notifyErr`, `noticeAtom…With`, `promptText`). Almost all were already i18n'd or just
  `errMsg.Set("")` clears; three real offenders remained:
  - `accounts.go` validation `"Enter a valid opening balance."` → new key `accounts.invalidOpening`.
  - `dashboard.go` reminder failure toast `"Couldn't create the reminder: "+err` → `dashboard.reminderErr`
    (`"Couldn't create the reminder: %s"`).
  - the two dashboard-tile resize-handle tooltips I'd added in the #1032 commit (kept hardcoded then for
    consistency with the surrounding tooltips) → `widget.resizeWidth` / `widget.resizeHeight`.
- All now resolve via `uistate.T`. i18n catalog test + wasm build + `go vet ./internal/ui ./internal/screens`
  green. (`promptText` call sites were already i18n'd; the `aria-keyshortcuts` token list is literal key
  names by spec, not translatable, so left as-is.)

## 2026-06-18 — B15: announce account balance updates (reconcile was silent)

- Last open live-region sub-item: announce inline balance updates. Found that `accounts.go`'s `setBalance`
  (the reconcile / Update-balance flow) succeeded *silently* — it posted the adjustment txn, set
  `BalanceAsOf`, cleared the error, and bumped, but gave no confirmation. Mark-updated already posted a
  notice; reconcile didn't.
- Reused the existing `noticeAtom` toast (already a persistent polite live region) — on success it now sets
  `…With(uistate.T("accounts.balanceUpdated", ac.Name, fmtMoney(money.New(target, ac.Currency))), false)`
  (the `false` = not-an-error → polite, not assertive). New i18n key `accounts.balanceUpdated` = "Updated
  %s to %s." So the new balance is both visibly acknowledged and announced to screen readers.
- i18n catalog test + wasm build + `go vet ./internal/screens` green. Closes the live-regions checklist item.

## 2026-06-18 — C28 (blank icons): root-caused to the framework, not app code

- Tried to fix C28 (every `ui.Icon` SVG renders blank; the live DOM shows `viewbox` lowercase). Audited
  both ends. App side is correct: `internal/ui.Icon` passes `Attr("viewBox", "0 0 24 24")`, the framework
  auto-injects `xmlns`, and the framework's *SSR string* renderer preserves the camelCase (its own
  `shorthand_more_test.go` asserts `viewBox="0 0 16 16"`).
- Framework side is the defect: grepped the entire GoWebComponents module — **there is no `createElementNS`
  anywhere**. So the wasm renderer makes `<svg>` in the HTML namespace. Two consequences: (1) per the DOM
  spec, `setAttribute("viewBox", v)` on an HTML-namespaced element lowercases the qualified name to
  `viewbox`; (2) the node isn't a real `SVGSVGElement`, so SVG geometry doesn't render at all. Text nodes
  still paint — which explains why chart *axis labels* looked fine (C16) while icon glyphs are blank: same
  bug, only the geometry is lost.
- No app-level workaround: `Attr` is the lowercasing path; `Props.Raw` can set odd attribute keys but
  can't add an element namespace; and the framework exposes no raw-HTML/`innerHTML`/dangerouslySetInnerHTML
  node that would let me hand the browser a pre-parsed (correctly-namespaced) SVG string. The fix has to be
  upstream (create `svg`/`math` subtrees via `createElementNS`, keep SVG camelCase attrs).
- Marked C28 **framework-blocked** (alongside B1/B3 SPA routing). Couldn't live-verify the current state
  either — the gwc browser oracle isn't available in the headless loop environment. No code change made;
  fabricating an app-side "fix" for a renderer bug would be dishonest and wouldn't work. Documented the
  root cause so it's actionable the moment the framework dependency or a browser lane is available.

## 2026-06-18 — #1032: explicit mouse shrink on the dashboard resize handle

- Open grid item: the resize handle only *grew* (click cycles span up, wraps to 1 at the max), so the only
  mouse way to shrink was to cycle all the way around. The keyboard path already had a direct shrink
  (Shift+Arrow); the mouse lacked the equivalent.
- Added Shift+click to shrink one step. Checked the framework first: `OnClick`/`OnKeyDown` both take
  `any` and the runtime dispatches `func(GoEvent)` (= `uic.MouseEvent`, an alias), exposing `JSValue()`.
  So the handle's `OnClick(func(e uic.MouseEvent))` reads `e.JSValue().Get("shiftKey").Bool()` — same
  technique the keyboard handler already uses for `shiftKey`.
- Factored the span math into a `cycleSpan(cur, max, shrink)` helper: shrink → `cur-1` clamped at 1;
  otherwise grow → `cur+1` wrapping to 1 past max. Both edge handles (width/height) use it. Tooltips now
  read "click grows, Shift+click shrinks".
- (Kept the tooltips as plain hardcoded English, consistent with the existing resize tooltips and the
  `aria-keyshortcuts` string in this same component; a full title-i18n sweep would be a separate change.)
  wasm build + `go vet ./internal/ui` green.

## 2026-06-18 — Closed the Members/Accounts "Add button no-op" finding (not a defect)

- Browser-oracle findings #4–#8 reported that the Members and Accounts **Add buttons** were silent no-ops
  on click while Enter worked, and pinned it on "the button's click handler reads stale state."
- Audited the actual code: in both `members.go` and `accounts.go` the Add button is `Type("submit")`
  *inside* `Form(Class("form-grid"), OnSubmit(add), …)`, and `add` is `ui.UseEvent(Prevent(func(){ … name.Get() … }))`.
  A submit-button click and an Enter keypress both dispatch the form's `submit` event → the same `add`,
  reading the same live `name` atom. There is no separate click handler and no stale-state path.
- The structure is **uniform across all six add forms** — Budgets and Goals have the identical
  `MapKeyed(defs, …)` custom-field block and were reported *working*, so there's nothing structurally
  different about Members/Accounts to fix.
- Root cause of the report: a synthetic-input harness artifact. Setting an input's `.value` without
  dispatching an `input` event leaves the bound state empty, so neither path truly commits; the flaky
  "Enter adds, click doesn't" split confirms it wasn't deterministic. The TODO's own caveat already
  suspected this ("if a human's typing updates the bound state, the button may work for them").
- Action: closed the #4 and #8 checkboxes with the analysis. No code change (the code is already correct),
  hence no CHANGELOG entry. A genuine regression assert needs the Playwright lane and must type via real
  key events, not value-set — tracked there.

## 2026-06-18 — B7: derive the rail nav from the screen registry

- B7's last item: the rail's three groups (primaryNav/toolsNav + an inline System block) were
  hand-maintained lists in shell.go, separate from `screens.All()` — so a newly routed screen could be
  reachable by URL yet silently missing from the menu (exactly the bug B7 was about).
- Added a `Group` field to `screens.Route` (`GroupPrimary`/`GroupTools`/`GroupSystem`) — pure data, no
  design dependency, so the registry stays presentation-free. `shell.go`'s new `navGroup(group)` filters
  `screens.All()` by it, in registry order; `primaryNav`/`toolsNav`/`systemNav` are now one-liners over it.
- Kept the design data (icons + i18n label keys) in shell.go as a `railMeta` path→{Key,Icon} map, honoring
  the earlier "icons live in the design layer, not the registry" decision. Membership = registry; appearance
  = railMeta. A path not in railMeta still renders (its registry Label + a default `icon.Page`) instead of
  vanishing — fail-safe, which is the whole point of B7.
- Replaced the hardcoded System block (members/categories/rules `If(!hidden…)`) with a `MapKeyed` over
  `visibleSystem`, and wrapped its header in `If(len>0)` so an all-hidden System group drops its label too.
- Behavior-preserving: the derived order is identical to the old hardcoded order, so no visual change.
  wasm build + `go vet ./internal/app ./internal/screens` green.

## 2026-06-18 — B15 a11y: announce filtered transaction count via live region

- The Transactions list already rendered a visible count+net summary, but it wasn't a live region and it
  unmounted at zero results, so a screen-reader user changing filters heard nothing — and never learned a
  filter produced no matches.
- Added an always-mounted `P(Class("sr-only"), role=status, aria-live=polite, aria-atomic=true)` carrying a
  `filterStatus` string: the count+net summary when there are matches, the localized `transactions.noMatch`
  text at zero, empty when there are no transactions at all. Staying mounted is the key — `aria-live`
  announces on *change*, so toggling the container in/out with `If(...)` would miss updates.
- Marked the existing visible summary `aria-hidden="true"` so SR users get the live region's announcement
  once, not the static summary too. Reused existing i18n keys (`transactions.summary`/`transactions.noMatch`)
  — no new strings. Scoped to Transactions (the only screen with a real filter bar). wasm build + vet green.
- Remaining live-region item: announcing inline balance updates after an edit.

## 2026-06-18 — B15 a11y: per-field aria-describedby for form errors

- Last forms-a11y item: tie each form's error to its input via `aria-describedby` so a screen reader
  re-announces it on focus (the `role="alert"` only fires once, when the error first appears).
- The architecture has **one form-level error string per add-form** (an `errMsg`/`rErr`/`plErr` atom), not
  per-field errors, so the honest association is error ↔ the form's **primary input** (the name/first field).
- Built a shared pair in `internal/screens/aria.go` (package `screens`, not `internal/ui` — the screens
  import the *framework's* `ui` package under that name, so a helper there would collide): `errAttrs(id,msg)`
  returns the `aria-describedby`+`aria-invalid` options (nil when no error — so spreading it after the input's
  `OnInput` keeps the hook count stable), and `errText(id,msg)` is the drop-in for the old
  `If(msg!="", P(Class("err"), role=alert, …))` but adds the matching `id`.
- Wired into all 11 add-forms: accounts, budgets, categories, custom-fields, goals, members, rules, to-do,
  transactions, and planning's recurring + plan forms, each with a stable error id (acct-err, budget-err, …).
  Spread pattern: `Input(append([]any{…, OnInput(on)}, errAttrs(id, msg)...)...)`.
- wasm build + `go vet ./internal/screens ./internal/ui` green. (Gotcha: a stray `cd` into a subdir persisted
  across PowerShell calls and broke the relative vet paths; reset with Set-Location to the repo root.)

## 2026-06-18 — B15 a11y: a light-theme-safe default accent

- The last open contrast item: the default accent `#54b884` (a light mint) only cleared ~2.1:1 against the
  light surface, failing WCAG AA for UI/large elements (3:1). The accent isn't decorative — it paints the
  focus ring and large strokes — so the default has to pass on whichever theme the user runs.
- Considered a **per-theme default** (keep the mint on dark, darker on light) but rejected it: `--accent` is
  applied **inline by JS** from prefs, and for `ThemeSystem` the effective theme isn't known at apply time,
  so a per-theme default would need a CSS `@media`/`[data-theme]` override path that fights the inline var.
  A single default that passes everywhere is simpler and correct.
- Drove the choice with data: a throwaway test over `internal/contrast.Ratio` against the dark elev `#1a1a1d`
  and light elev `#efede8`. Seagreen **`#2e8b57`** clears both comfortably (dark 4.09:1, light 3.63:1) and
  stays an obviously-green brand color. Set it as `prefs.defaultAccent`, the first swatch in the picker, the
  contrast-note fallback, the CSS `--accent` default in index.html, and the chart-stroke fallbacks
  (chart.js, ui/chart.go, planning.go). Bumped the SW cache to v15. prefs/contrast tests + wasm build green.
- The settings contrast note now reads OK on light with the default (it computed the warning before). Closes
  the B15 contrast item.

## 2026-06-18 — B15 a11y: route the last hardcoded aria-labels through i18n

- Grepped for hardcoded `aria-label`/`aria-roledescription` literals; only two remained (the widget gear
  and the SwatchPicker). Routed both through `uistate.T()` (new `widget.settings`, `a11y.accentColor`
  keys); controls.go gained the uistate import (no cycle — widget.go already imports it). The gear's
  title attr now uses the same key. Closes the B15 i18n-aria item. Build + i18n catalog test green.

## 2026-06-18 — B15 a11y: keyboard resize (completes bento keyboard control)

- Extended the tile `OnKeyDown` with Shift+Arrow resize: ←/→ adjust width, ↑/↓ adjust height via
  `dashlayout.ResizeItem`, clamped to [1,maxColSpan]/[1,maxRowSpan] (added a `clampSpan` helper). Read
  Shift off the underlying event (`e.JSValue().Get("shiftKey").Bool()`). Plain arrows still move.
- Verified live: Shift+ArrowRight on kpi-networth grows it from grid-column "1" to "1 / span 2".
- The dashboard bento is now fully keyboard-operable (move + resize), closing the WCAG 2.1.1 gap that the
  pointer-only drag/resize left.

## 2026-06-18 — B15 a11y: keyboard reorder for the bento

- Closed the biggest remaining keyboard gap: the bento was drag-only (pointer). Made each draggable tile
  `tabindex=0` with `aria-keyshortcuts`, and added an `OnKeyDown` that on Arrow keys moves the tile one
  slot earlier (Left/Up) or later (Right/Down) via `dashlayout.Move` on the arranged order — persisting
  and switching to Custom mode, exactly like a drag. The FLIP animates it; reduced-motion still applies.
- Verified live: focusing kpi-networth and pressing ArrowRight moves it from grid 1/2 to 2/2.
- Remaining keyboard a11y: an arrow-key *resize* alternative (resize is still Shift+click) and
  inline-edit focus-on-enter/exit.

## 2026-06-18 — B15 a11y: icon-button labels + closing satisfied items

- Added explicit `aria-label`s to the icon-only buttons the spike flagged: the widget gear ("Widget
  settings", glyph wrapped in `aria-hidden`), the accounts "⋯" overflow ("More actions"); made the
  decorative drag grip `aria-hidden`. Verified live (gear aria-label present, grip aria-hidden=true).
- Closed B15 items now satisfied by recent work: reduced-motion covers the dashboard FLIP animations
  (flip.js checks matchMedia), 200% zoom reflows (C26, verified), and the D3 `ui.Chart` is already
  role=img + aria-label (chartd3.go) alongside the sparkline. Updated the checklist.
- Remaining B15: the bento drag/resize keyboard alternative, per-field `aria-describedby`, the
  light-theme accent brand decision, and the axe-in-CI tooling (needs the browser lane).

## 2026-06-18 — feature D10: forecast scenario comparison + dollar axis

- The Planning 12-month forecast used the axis-less sparkline and showed the trim scenario as text only.
  Switched it to the D3 `ui.Chart`: plots major units with a compact-currency Y axis ($0/$10k/…, the C16
  fix), and overlays a second series (the trimmed scenario, gold) beside the baseline when a trim amount
  is entered — so the two net-worth curves compare directly (the §2.6 comparison gap).
- Added legend rendering to the D3 shim (`web/chart.js`): when `spec.legend` and >1 series, a small
  top-right colored-dot + name list, so the baseline/scenario lines are labeled. SW cache v13→v14.
- Unit: `forecast.TestProjectSpendingDeltaShiftsEndBalance` — trimming by `delta`/month pulls the curve
  ahead by delta each month (end = delta×months higher), backing the what-if.
- Verified live on /planning: D3 svg with `$0/$10k/$20k/$30k` ticks; entering a trim adds the scenario
  line (stroked paths 3→4).

## 2026-06-18 — feature B2: live drag-over preview

- The reorder previously only happened on drop; added a live preview during the drag. Clean,
  low-regression design: a new `uistate.UseDragPreview` atom (the tile under the cursor, set in
  `OnDragOver`) drives a *render-time* `Move(arranged, dragSrc, indexOf(previewTarget))` in `ui.widget`,
  so the grid reflows live. Crucially this never touches the persisted `items` — so the drop path is
  unchanged (still bakes + persists), and a drag-end-without-drop just clears the atoms and the render
  reverts. No revert-of-persistence bookkeeping needed.
- Made the FLIP fire during the preview by adding the drag atoms to the dashboard's flip signature, so
  each dragover animates the reflow.
- Verified with synthetic DnD: dragstart on kpi-income + dragover on kpi-networth moves income to column
  1 (preview); dragend without drop reverts it to column 2. Drop persistence path untouched.
- Remaining B2: pointer-events over HTML5 DnD for touch.

## 2026-06-18 — B10 closed: resolution control verified + responsive

- Picked up B10 (the "drastic" resolution-control redesign) expecting a big build, but found the redesign
  was already implemented (single-period stepper, This-period reset, Custom-range toggle, Jump-to presets)
  and the remaining "responsive collapse" was already handled by my C19 work (`.reso-control` wraps; the
  control cluster drops to a full-width row below 1024px). So B10 was really down to verification.
- Verified live on the dashboard: the control shows a single label "Jun 2026" (not the old
  "Jun 2026 – Jun 2026"); the "Last period" preset shifts the window to May 2026; there's one stepper
  that "Custom range" expands to two From/To steppers. Decision recorded: full range power stays behind
  Custom range (recommended), not dropped.
- Docs/verification only (no code change this iteration). B10 complete.

## 2026-06-18 — feature B2: FLIP animations for the dashboard bento

- Grid placement changes don't transition, so animating tile reorder/resize needs FLIP. Did it with a
  stateful JS shim `web/flip.js` (`cashfluxFlipBento`): it remembers each `.bento > .w[data-widget]`
  tile's screen position; on the next call it measures the new position, jumps the tile back to the old
  spot (transition:none + translate), forces a reflow, then on the next frame transitions the offset to
  zero — the classic FLIP. State lives in JS, so there are no per-move Go `js.FuncOf` callbacks to leak.
  Honors prefers-reduced-motion (records positions only).
- Go side: a `ui.UseEffect` in `Dashboard` invokes the shim, keyed on a layout signature (mode + each
  item's id/spans/importance) so it fires exactly when the arrangement could change — not on every data
  tick. Loaded flip.js in index.html + SW CORE (cache v12→v13).
- Covers both "animate reorder" and "animate resize" (any reflow animates), and the auto-layout switch.
- Verified the mechanism in the oracle: `cashfluxFlipBento` is loaded; after forcing a tile to a new
  grid-row and calling it, the tile gets the inverse `translate(250px,-1221px)` with transition:none (the
  FLIP invert) — the rAF then plays it to zero. (The time-based glide itself isn't headless-observable.)
- Still open under B2: a live drag-over *preview* (reflow lands on drop) and pointer-events for touch.

## 2026-06-18 — C9: accounts-row overflow menu

- Account rows exposed six actions each — visually busy. Kept the primary three inline (Transactions /
  Edit / ✕) and moved Update balance / Mark updated / Archive into a "⋯" overflow menu, reusing the C23
  popover CSS (.add-wrap/.add-menu/.add-item/.add-backdrop/.hidden-menu) — no new styles.
- AccountRow is its own component, so the menu state (`menuOpen` UseState) + toggle/close/secondary
  handlers register as hooks at the top (unconditional); the menu is always rendered and CSS-toggled, and
  the secondary handlers now also close the menu. Made `archTitle` the archive item's tooltip.
- Verified live (in-app nav to /accounts): rows show a ⋯ that opens with [Update balance, Mark updated,
  Archive]; hidden→shown on click.

## 2026-06-18 — feature D6 (envelope): carry-forward budgeting view

- User said to grind until every TODO is done and stop pausing for direction (recorded in
  [[dont-ask-which-ticket-next]]). Took the envelope view — the decision-gated item I'd flagged — and
  made the spec call myself: since `domain.Budget` has no start date, the carry-forward window runs from
  the **first covered transaction** through the current period.
- Pure model: `budgeting.EnvelopeAvailable` accumulates `limit − spent` per period from that first
  transaction to now (bounded at 240 periods so bad dates can't loop), reusing `spentCovered` so it
  honors the period window, owner scope, and sub-category rollup. Table-tested: no-spend = one period's
  limit, current-period-only, carries unspent forward, overdraw nets against carryover, scope respected.
- UI: Settings offers Envelope (3rd option); the Budgets screen, in envelope mode, shows a note plus a
  per-budget "Envelope balance: $X" line (danger tone when negative). Computed per budget via
  `EnvelopeAvailable` with `categorytree.Descendants`.
- Verified live: switching to Envelope and visiting Budgets shows the note + "Envelope balance: $359.45".
  D6 is now complete across all three methodologies (Simple / Zero-based / Envelope).

## 2026-06-18 — tests D7: month/week/quarter boundary membership

- Hardened the period-boundary correctness that C1 was about, at the `ledger.PeriodTotals` (totals)
  level. `internal/ledger/boundary_test.go`: income txns placed on the first and last day of each window
  must land in exactly one period across consecutive windows — no drop, no double-count.
  - Month: May 31 / Jun 1 / Jun 30 / Jul 1 → May=100, June=600, July=800, and the three windows sum to
    every amount once (1500). This is the totals-level regression home for C1.
  - Week (Sunday-start): the week-start day and the Saturday before the next start are in; the next
    Sunday rolls to the following week.
  - Quarter: Q2 [Apr 1, Jul 1) includes Apr 1 and Jun 30; Jul 1 rolls to Q3.
- All pin the half-open `[start, end)` semantics. The underlying UTC-convention fix + a non-UTC-zone
  membership test landed earlier (C1). Test-only.

## 2026-06-18 — feature D5: parent-category budgets roll up sub-category spend

- Found a real gap behind the D5 "rollup test" item: `budgeting.matches` compared `t.CategoryID ==
  budget.CategoryID` exactly, so a parent-category budget ("Food") did NOT count spend on its
  sub-categories ("Groceries"). Implemented the rollup.
- Pure foundation: `categorytree.Descendants(cats, rootID)` returns rootID + every nested id (cycle-safe),
  so callers can roll a subtree up. Tested: multi-level, mid-tree, leaf, reparent (rollup follows the new
  parent), empty/unknown root, and a cycle.
- Budgeting: refactored the matcher around a `covers func(string) bool` predicate (`matchesCovered` /
  `spentCovered` / `evaluateWith`), kept `Spent`/`Evaluate` byte-for-byte (exact predicate), and added
  `EvaluateRollup(..., covers map[string]bool)` — counts spend in the budget's category OR any covered
  category, still applying the period window and individual-owner scope. Tests: descendants counted,
  empty covers == own category, scope respected under rollup.
- Decoupling: budgeting takes the covered-id *set*, not the category tree, so it stays independent of
  `categorytree`. The callers (Budgets screen + dashboard Budgets widget) build the set via
  `categorytree.Descendants(app.Categories(), b.CategoryID)` and call `EvaluateRollup`.
- For a budget whose category has no sub-categories, `Descendants` = {id} and `EvaluateRollup` is
  identical to `Evaluate` — so no behavior change on flat category sets (verified the dashboard still
  renders cleanly). The rollup itself is proven by the unit tests. (The spending-breakdown widget already
  rolled up; this aligns budgets with it.)

## 2026-06-18 — tests D2: budgeting threshold boundaries

- With the tractable feature backlog thin (remaining items are the B10 redesign, B2 FLIP animations, and
  the envelope carry-forward view — all large or hard to verify headlessly), spent this iteration
  hardening the package I just extended. Added `internal/budgeting/threshold_test.go`: white-box boundary
  tests for `classify` (`==limit` → Over, `==near%` → Near, one cent below → OK; two thresholds) and
  `percent` (including the zero-limit guard that returns 100/0 instead of dividing by zero). 12 cases.
- Test-only, native — guards the exact-boundary behavior so a future tweak to the `>=` comparisons can't
  silently shift a budget's state off by a cent. Closes the D2 "unit" item.

## 2026-06-18 — feature D6: budgeting method (Simple / Zero-based)

- Built the budget-methodology selector. Model: `budgeting.Methodology` (simple/zero-based/envelope) +
  `Valid`/`ParseMethodology` (unknown/empty → simple) + `ToAssign(income, totalBudgeted)`, all pure and
  table-tested. Config: `store.Settings.BudgetMethodology` (household-level, so it persists with the
  dataset and now survives reload via the autosave).
- UI: a Settings → household "Budgeting method" selector (Simple · Zero-based; Envelope reserved in the
  type but not offered yet — it needs the carry-forward view). On the Budgets screen, when zero-based,
  a banner shows income-for-the-month minus total budgeted: "$X left to assign" / "Every dollar is
  assigned" / "Over-assigned by $X". Income comes from `ledger.PeriodTotals` over the month range of the
  globally-selected period (`budgeting.PeriodRange(PeriodMonthly, viewMonth, weekStart)`).
- Verified live end-to-end: set Zero-based in Settings, navigated to /budgets (client-side), and the
  banner read "$3,600.00 left to assign this month" (sample income − total budgeted).
- Deferred: the Envelope view (carry-forward) and the per-member config-layering — noted in TODOS.

## 2026-06-18 — cleanup C7: one period control for Budgets

- The Budgets card carried its own `‹ January 2006 ›` month stepper (a `monthOffset` UseState +
  prev/next handlers) that competed with the global top-bar resolution control — two controls, two
  formats ("Jun" vs "June"). Since C4 already shows the resolution control on /budgets, the in-card
  stepper was redundant.
- Removed the stepper UI + `monthOffset`/`prevMonth`/`nextMonth`, and pointed `viewMonth` at the shared
  window (`uistate.UsePeriod().Get().From`) — the same atom the top-bar writes — so stepping the period
  up top now drives the budgets view. Dropped the now-unused `dateutil`/`time` imports.
- Default behavior unchanged (the default window is the current month). Route-gated screen; verified by
  reasoning + build.

## 2026-06-18 — feature B8: drag-reorder the sidebar

- Built the sidebar nav reorder bottom-up. Pure `internal/navorder` (Move/Apply, 10 table-test cases):
  `Move` relocates an id like `dashlayout.Move`; `Apply` layers a saved path sequence over the live nav
  list (saved order first, new screens appended, hidden/removed dropped). `uistate/navorder.go` persists
  the order (`cashflux:nav-order`) + a `UseNavDragSource` atom. `navItem` gained Draggable/OnDragStart/
  OnDrop props (the item is already its own component, so the drag hooks are at stable positions); the
  Sidebar applies the saved order to `visibleNav` and wires per-item drag callbacks.
- **Design call:** implemented *always-draggable* rather than the TODO's Shift-gating. Shift-gating the
  HTML `draggable` attribute reactively would need a new shift-held *atom* (the resize-reveal uses a
  non-reactive `data-resize` DOM attribute, which CSS can read but the Go components can't react to), and
  that re-renders the whole rail on every Shift press. Always-draggable is simpler and fully functional —
  click still navigates (a separate event from drag). Noted Shift-gating as a later refinement.
- Verified live by dispatching real DragEvents (dragstart/dragover/drop with a DataTransfer): dragging
  Accounts onto Dashboard reordered the rail to `[Accounts, Dashboard, …]` and persisted the path order.
  Apply-on-boot is reasoned (tested Apply + the atom seeds from localStorage like the other prefs).

## 2026-06-18 — feature: per-widget "add" affordances on empty dashboard tiles (C23)

- Completed the open part of C23: empty Accounts/Goals/Budgets/To-do dashboard tiles now show an
  in-context "Add a …" button. Added a reusable `emptyAddCTA` component (its own component so the
  `router.UseNavigate` hook stays stable) that renders the message + a primary button routing to the
  screen. The Budgets widget distinguishes genuinely-empty (`len(app.Budgets()) == 0` → CTA) from the
  at-risk filter being empty (budgets exist → plain "Nothing near or over budget.").
- Verified: i18n + wasm build green; the sample dataset populates all four tiles so the CTA isn't visible
  by default, but I confirmed the navigation mechanism live (clicking a nav item routes client-side —
  /goals renders the Goals screen), which is exactly what the CTA does.

## 2026-06-18 — feature: opt-in "remember my key on this device" (C27 closed)

- Followed the dataset-persistence feature with the small, isolated piece it left: an opt-in to persist
  the OpenAI key (the autosave redacts it, so it's session-only by default).
- `prefs.RememberAIKey` (new bool, off by default). `uistate/aikey.go`: `PersistAIKey`/`ClearAIKey`/
  `LoadAIKey` over a dedicated `cashflux:openai-key` localStorage entry (kept separate from the dataset).
  `app/persist.go` `hydrateAIKey()` restores it on boot when the toggle is on (called after
  hydrateDataset). Settings → AI gained a ToggleRow + a plain-English unencrypted-storage note; the key
  input's onKey re-persists when the toggle is on so edits stay in step.
- Secure-by-default: off → nothing stored; on → user has explicitly opted in with a clear notice.
- Verified live by driving the UI: open settings → type a key → toggle Remember on writes
  `cashflux:openai-key`, toggle off clears it. Boot-restore is reasoned (mirrors hydrateDataset + tested
  PutSettings). Closes the C27 AI-key item entirely.

## 2026-06-18 — feature: local dataset persistence (data survives reload)

- Re-armed loop ("implement all features"). Picked the highest-value gap I'd surfaced while investigating
  the C27 AI-key bug: the dataset wasn't persisted locally at all — `appstate` seeded the *sample* on
  boot and only manual Export/Import saved anything, so a reload lost the user's data.
- Design choice: rather than instrument all ~25 mutation methods with an OnChange hook (error-prone —
  Puts don't share a helper, only Deletes route through `a.del`), autosave on a **4s ticker + page-hide**.
  The ticker snapshots the current dataset and writes only when the serialized bytes change, so it catches
  every mutation regardless of code path without touching appstate's write methods.
- `internal/app/persist.go` (js&&wasm): `hydrateDataset()` loads `cashflux:dataset` from localStorage on
  boot (ImportJSON), or seeds the sample on first run; `startDatasetAutosave()` wires the ticker +
  `pagehide`/`visibilitychange` listeners. Boot changed to `appstate.Init(nil, false)` (empty store) then
  `hydrateDataset()` before mount, and `startDatasetAutosave()` after mount.
- Security: added `appstate.ExportJSONRedacted()` — snapshots and zeroes `Settings.OpenAIKey` before
  marshaling, so the autosave never writes the secret (it stays session-only). The manual `ExportJSON`
  keeps the key so a user's own backup is complete. Guarded `save()` with `recover()` so a quota-exceeded
  `localStorage.setItem` (large dataset) logs instead of crashing.
- Verified live: within ~5s a redacted dataset blob (4627 bytes, has transactions, **no openAiKey**) is in
  localStorage, and the app boots with its data ($20,749.25). Cross-reload load couldn't be exercised in
  the static oracle (fresh browser profile per launch), but ImportJSON + the localStorage read are both
  proven, mirroring the resolution/rail persistence pattern.
- This also shrinks the C27 AI-key item to just an opt-in "remember my key" toggle (secure-by-default).

## 2026-06-17 — bugfix C27: vision category near-name matching

- The receipt-import path matched the AI's category string to a household category by exact (lowercased)
  name, so "Food & Drink" missed "Food" and imported uncategorized. Added a fuzzy fallback between the
  exact match and the existing auto-rules fallback: substring match either direction (min length 3 to
  avoid spurious hits like a one-letter category), scanning `app.Categories()` in order for determinism.
- Chose the contained post-processing fix over constraining the vision prompt to the category list (a
  bigger AI-call change) — noted prompt-constraint and a per-row picker as further hardening. Route-gated
  screen + AI call, so verified by reasoning + build.

## 2026-06-17 — bugfix C27: "Save as task" title

- Saving an AI insight as a to-do used the whole first sentence of the answer (truncated to 80 runes) as
  the title. Now the title is the question the user asked (the `question` state) when present, else a
  short generic "Money insight" label; the full answer stays in the task notes. Small, route-gated UI fix.

## 2026-06-17 — bugfix C27: document-review amounts in accounting style

- The receipt-import review list showed the AI's raw amount string ("−4.50"), out of step with the
  app's accounting format. Passed the chosen import account's currency (falling back to base) into
  `DraftRow` and formatted the display amount via `fmtMoney` (parse → money → format), with a raw-string
  fallback when the value won't parse (the user may still be correcting it). The edit field keeps the
  raw editable string. Route-gated screen, so verified by reasoning + build; `fmtMoney` itself is tested.

## 2026-06-17 — bugfix C27: CSV import accepts its documented format

- The starred C27 bug: pasting the on-screen `date,payee,amount,account` example failed with a leaked
  `store: csv line 2: amount and currency are required` — the parser hard-required a `currency` column the
  docs never mentioned, and it read `account_id`/`category_id`/`member_id` while the docs say
  `account`/`category`/`member`.
- Store fix (`TransactionsFromCSV` gained a `defaultCurrency` param): only amount is required now; a
  missing currency falls back to the passed default; a new `colID` helper reads `<base>_id` (export) or
  `<base>` (friendly), preferring the explicit id. Pure + table-tested (default-currency + friendly
  columns; id-wins-over-name; still errors on no-currency-and-no-default, bad amount/date, missing amount).
- appstate fix: `ImportTransactionsCSV` passes the household base currency and resolves account/category/
  member cells that were given as **names** to their ids (case-insensitive, via a small `idResolver`),
  so a hand-written CSV with `account=Checking` lands on the right account. Unknown values pass through to
  the validated write path, which skips genuinely-invalid rows.
- UI: strip the internal `store:` prefix from the import error, and updated `documents.csvDesc` to say
  currency is optional and account/category/member accept a name or an ID.
- All logic-package tests + the wasm build green. (Route-gated screen, but this is a logic fix proven by
  unit tests.) Remaining C27 items: vision category aliasing, review-row accounting format (= C2),
  AI-key persistence, and the save-as-task title trimming.

## 2026-06-17 — C26: text/display size to 200% for accessibility

- The Display scale (B6) topped out at 130% and the TODO flagged that `zoom` would overflow the
  non-responsive layout at large values — so C26 was blocked on C10. C10/C19 are now fixed, which
  unblocks the TODO's option (b): keep the `zoom`-on-`#app` mechanism and just raise the ceiling.
- Raised `prefs.ScaleMax` 130 → 200 (WCAG 2.1 SC 1.4.4), updated the clamp test (200 valid, 250 clamps),
  and relabelled the setting "Text & display size" so it reads as an accessibility control. The
  `scaleOptions` builder derives from the constants, so the dropdown now offers 70–200% automatically.
- **Empirically validated the reflow** (the part the TODO was unsure about): set `--ui-scale: 2` on a
  1280px window — Chromium's `zoom` makes the content lay out at the effective ~640px width, so the
  phone responsive rules engage and `document.body.scrollWidth == viewport` with no horizontal scroll.
  So 200% text resize works without loss of content, exactly because the responsive pass landed first.
- Composes with C25: density rebalances the base tokens; this is a zoom multiplier on top — independent.
- Wasm-only change (no index.html), so no SW cache bump (main.wasm is network-first).

## 2026-06-17 — C25: rebalance the default density down

- Picked C25 next ("UI too fat/chunky"). The TODO said to confirm the approach first, but the user told
  me to stop asking and just pick — so I made the call: rebalance the *default* density down rather than
  introduce new Cozy/Compact presets. Simpler, lower-risk, and the existing Compact toggle + Display
  scale still layer on top.
- Trimmed the shared tokens in index.html: body 16px/1.5 → 14.5px/1.45 (Fraunces display figures keep
  their explicit sizes, so the data stays prominent); `.field` 0.5/0.6→0.4/0.55rem padding + radius 8→6;
  `.btn` 0.55/0.9→0.4/0.8rem + radius 8→6; `.wbody` 0.85→0.7rem.
- Checked the a11y angle: the B15 touch-target rule enforces a 24px minimum on small controls; fields are
  now ~34px and buttons ~30px, both above it — no regression. (The app's standard is 24px, not the 44px
  the TODO aspired to; I didn't make that worse.)
- Verified live on the dashboard + the quick-add form: body computes to 14.5px, `.field` is 34px with no
  text clipping, and the KPI figures still fit (0 clipped). Other screens are route-gated in the static
  oracle but share these tokens, so the effect is uniform. SW cache v11→v12.

## 2026-06-17 — feature C23: "+ Add" multi-entity add menu

- Picked the next ticket myself (the user said to stop asking which to do — see the new memory). Chose
  C23 as the most self-contained, clearly-valuable next feature.
- "+ Add" opened only the quick-add transaction panel; every other entity could be added only from its
  own screen. New `app.AddMenu` component turns "+ Add" into a popover: New transaction (opens the inline
  quick-add) · New account · New budget · New goal · Scan a document (the entity items route via the
  router to the screen where the add form lives).
- Framework-safe rendering: the popover + a click-catching backdrop are always in the DOM and shown/
  hidden with a `.hidden-menu` CSS class (not `If`), so the menu items' OnClick hooks stay at stable
  positions (the On*-hooks-in-loops rule). The 5 items are built by a fixed-count helper, not a loop.
  Moved `UseQuickAdd` out of TopBar into AddMenu.
- Verified live: the menu is hidden initially, opens on click with 5 items, "New transaction" opens the
  quick-add panel, and the menu closes on select. Entity navigation uses the same router.Navigate the
  nav rail uses. SW cache v10→v11.
- Left open (enhancement): per-widget in-context "add" affordances (e.g. an empty Budgets tile offering
  "Add a budget").

## 2026-06-17 — feature C24 (importance UI): rank tiles from the gear — C24 done

- Final piece: a per-tile Importance control. Added pure `dashlayout.SetImportance`/`ImportanceOf`
  (+ tests), then an `importanceRow` component in the gear settings panel (Highest/High/Normal/Low →
  2/1/0/−1) that writes the layout items atom and persists on change.
- Resolved the C21 tension cleanly: importance is a *universal* per-tile setting, so the settings panel
  is never empty. That let me show the gear on **every** tile while in Auto-importance mode (the widget
  gear gate gained `|| mode == ModeAutoImportance`) without bringing back the empty "no settings" panel
  — in the other modes the gear still only shows on schema'd tiles (C21 preserved).
- Hit (and fixed) an ordering bug: the gear gate referenced `mode` before it was computed; hoisted the
  mode read above the gate.
- **End-to-end live verification** (the satisfying one): switched the header selector to Auto-importance
  → the no-schema freshness tile now shows a gear → opened it → set Importance to Highest → the freshness
  tile moved from grid-row 8 (bottom) to row 2 (top), and `cashflux:layout` now contains the Importance
  field. The whole feature works front to back.
- C24 is complete (model + state + selector + importance editing). Remaining decision-gated C-items:
  C17 (→ B10 redesign), C23 (add-menu feature), C25 (density — confirm tokens), C26 (text-resize).

## 2026-06-17 — feature C24 (wiring): layout-mode state + selector

- Wired the auto-layout engine into the dashboard. State: `uistate.UseLayoutMode` /
  `PersistLayoutMode` / `loadLayoutMode` (default Custom, mirrors the resolution-pref pattern). Render:
  `ui.widget` now packs `Arrange(items, mode)` instead of the raw items, so each tile's grid placement
  follows the active mode. UI: a mode `<select>` in the dashboard header (Custom / Auto: default / Auto:
  importance) with i18n keys.
- Two ordering subtleties handled: (1) a manual drag in an auto mode bakes the current *arranged* order
  into the stored sequence and flips to Custom — otherwise the drop index (computed against the visual
  order) wouldn't match the stored order. (2) Switching the selector to Custom likewise bakes the auto
  order, so tiles don't jump back to an older hand-arrangement. Resize is order-independent, so it stays
  untouched and works in every mode.
- Verified live: the selector shows 3 options, defaults to Custom, persists `cashflux:layout-mode` on
  change, and the dashboard re-renders cleanly (16 tiles) in each mode. Actual reordering is covered by
  the model's table tests; it'll be visually demonstrable once importance is settable (next commit) —
  on a fresh default layout all three modes coincide (canonical order), so there's nothing to see yet.
- Note: tiles render in a fixed DOM order (dashboard.go source order); modes change CSS grid placement
  (via Pack), not DOM order.

## 2026-06-17 — feature C24 (model): dashboard auto-layout engine

- After clearing the whole C-series bug backlog (C1–C22 this session), asked the user which decision-
  gated item to take next; they chose **C24, the auto-layout engine**, and confirmed the two open
  decisions: importance is set **per-tile via the gear**, and tile **size stays user-set** (auto-layout
  only reorders). Noted the C21 tension (KPI tiles have no gear) — the resolution is to make importance a
  universal per-tile setting so the gear panel is never empty, letting the gear show on every tile in
  importance mode without bringing back C21's empty panel. That's a UI-stage concern; recorded for then.
- Built the **model first** (bottom-up): pure `dashlayout.Arrange(items, mode) []Item` that reorders the
  sequence by `Mode` (Custom = no-op, AutoDefault = canonical `DefaultItems` order, AutoImportance =
  importance desc with canonical-order tiebreak); the existing `Pack` still derives positions. Added an
  additive `Importance int` to `Item` (json omitempty, so older saved layouts load unchanged). Arrange
  never touches spans — sizes remain user-set per the decision.
- 8 table tests: Mode.Valid, Custom no-op, AutoDefault restores canonical order from any start,
  AutoImportance high-first then canonical tiebreak, ties stable, no input mutation + spans preserved,
  unknown ids sort after known, and Arrange+Pack has no overlap in every mode. Native suite + wasm build
  green.
- Next (separate commits): persist a `LayoutMode`; then the render path applies Arrange before Pack, a
  mode selector on the dashboard header, and an Importance control in the gear panel.

## 2026-06-17 — bugfix C19 (list rows): action buttons wrap instead of overlapping

- The last C19 sub-item: at narrow widths the transaction row's buttons overlapped the description/date
  because `.row` is a nowrap flex. Added `.row { flex-wrap: wrap; row-gap: .4rem }` to the ≤1024px block.
  `.row-main` keeps `flex:1`, so it claims the first line and the buttons flow underneath; it's a no-op
  whenever the row still fits on one line. Shared by every list screen (transactions/accounts/budgets/…),
  and wrapping is graceful everywhere.
- /transactions can't be loaded in the static oracle (no SPA fallback), so I verified the *mechanism* the
  same way as C18: injected a representative `.row` (check + row-main + Mark cleared/amount/Edit/
  Duplicate/✕) at 360px and measured — the row wrapped (height ~204px) with 0 of 5 buttons overlapping
  the text rect. SW cache v9→v10.
- C19 is now fully done (top bar + KPI clip + list rows).
- C22 is verified resolved by the C14/B2 Pack migration (widget.go uses Pack/Move/ResizeItem, no Swap):
  moving a tile reflows the rest and resize re-packs without overlap. Only the live drag *preview*
  remains, tracked under B2.

## 2026-06-17 — bugfix C19 (KPI clip): 2-column tablet bento

- Measured the reported KPI clip live: at 900px the bento was still the desktop 4-column grid, tiles
  ~153px, and one figure clipped (`scrollWidth > clientWidth`). The phone block (<768px, 1 column)
  didn't reach this range, so 768–1024px fell through to the cramped desktop grid.
- Added a tablet bento media block (`min-width:768px and max-width:1024px`): 2 columns, tiles flow with
  `grid-column:auto`, and `.bento > *:first-child` keeps the dashboard header spanning both columns
  (it carries an inline `grid-column: 1 / -1` that the auto override would otherwise drop).
- Verified live at 900px: 0 clipped figures, KPI tiles ~315px (was 153), header ~640px (full content
  width = both columns), no horizontal page scroll. SW cache v8→v9.
- C19 now done except transaction-row action-button wrapping at narrow widths — that's route-gated
  (can't drive /transactions in the static oracle) and a list-row concern, left open under C19.

## 2026-06-17 — bugfix C20: persist the sidebar collapsed state

- C20 ("collapsible panel reads as missing") had three parts. Two are now resolved: C15 fixed the
  empties-on-collapse problem (icons survive), and this change persists the collapsed/expanded choice
  across reloads — it was a transient `state.UseAtom` in app/shell.go, lost on refresh.
- Moved the atom into `uistate.UseRailCollapsed()` seeded from localStorage via `loadRailCollapsed`, with
  `PersistRailCollapsed` written in the menu-button click. This mirrors the existing
  `UsePeriod`/`PersistResolution`/`loadResolution` pattern exactly. Removed the now-unused `state` import
  and the old const from shell.go.
- Verified live: clicking the toggle writes `cashflux:rail-collapsed` = `1` then `0` and the rail width
  goes 58↔240. The oracle uses a fresh browser profile per launch (confirmed: localStorage set in one
  launch reads null in the next), so the load-on-reload leg can't be exercised end-to-end there — but it
  is structurally identical to the resolution-pref load path that's already in use.
- Remaining C20: an on-panel collapse affordance (chevron) — a placement/design call; the working
  top-bar toggle stands in the meantime, so I left it for a spec decision rather than guessing.

## 2026-06-17 — bugfix C21: widget gear only where there are settings

- The gear opened real, persisted settings for 8 widgets but also showed on the 4 KPI tiles and
  cashflow/bills/freshness, which have no schema — there it opened the C11 close-only "no settings yet"
  panel, reading as broken. Gated the gear in `ui.widget`: render the `gearButton` only when
  `props.OnGear != nil || widgetcfg.Has(props.ID)`; otherwise render an inert `span.gear-inline`
  (visibility:hidden, aria-hidden) so the grip · title · gear header stays balanced.
- Also strengthened discoverability (the gear was a very faint glyph): added a color transition and a
  `.w:hover/.w:focus-within .gear-inline` brighten so it surfaces on the tiles that have settings.
- Verified live: 16 tiles → 8 real `button.gear-inline` (exactly the schema'd widgets) and 8 hidden
  `span.gear-inline`; the net-worth KPI's gear slot is a span, not a button. SW cache v7→v8.
- C22 ("layout doesn't reflow on move/resize") is the same root cause as B2/C14 and is already resolved
  by the Pack migration (widget.go uses Pack/Move/ResizeItem, no Swap) — only the live drag *preview*
  remains, tracked under B2.

## 2026-06-17 — bugfix C19 (top bar): controls were unreachable on tablet/phone

- At ≤768px the top-bar control cluster (resolution segmented + jump + stepper + custom range + Add) ran
  off the right edge with no wrap, so some controls were unreachable and the breadcrumb shrank to "D".
- First attempt — `flex-wrap: wrap` + `width: 100%` on the control cluster — *didn't* work, and the
  oracle showed why: flexbox preferred to *shrink* every item onto one line (squashing the breadcrumb to
  ~48px) rather than wrap, and Tailwind's `.h-14` was overriding my `height:auto` so even a wrapped row
  would have been clipped. Measured it live: barH stayed 56, breadcrumb h1 width ~96 but controls still
  shared the row.
- Fix that worked: `.topbar-controls { flex: 1 0 100% }` (no shrink, full-basis → forced onto its own
  row) plus `.topbar { height: auto !important }` to beat `h-14`, and `flex-wrap` on the bar and the
  resolution control so the cluster wraps internally. Gave the bar a `topbar` class, the cluster a
  `topbar-controls` class, and the resolution control a `reso-control` class to target them.
- Verified live with the oracle's viewport flags: 768px → bar 175px, breadcrumb readable (96px), every
  control's right edge ≤ 768, no page h-scroll; 390px → all controls reachable, no h-scroll. SW v6→v7.
- Remaining C19 (separate concerns, left open): transaction-row action buttons overlapping at narrow
  widths (route-gated — can't drive /transactions in the static oracle) and KPI figure clipping when the
  desktop bento is squeezed between 768–1024px.

## 2026-06-17 — bugfix C18: inline-edit stacked fields vertically

- Editing a Transactions or Accounts row rendered its fields in a tall single column with dead space to
  the right, while Budgets edited horizontally. Surprise: all three edit forms already used
  `Form(Class("form-grid"))` — the difference was the *wrapper*. Transactions/Accounts wrapped the form
  in the flex `.row` (`display:flex; justify-content:space-between`), so the single grid child shrank to
  its min content and `auto-fit minmax(150px,1fr)` collapsed to one column. Budgets wrapped it in a
  block (`.budget`), giving the grid a definite full width and thus multiple columns.
- Fix: added a `.row-edit` block class (same top-border/padding as `.row`, density-aware) and switched
  both edit wrappers to it. No change to the forms themselves.
- Verified the mechanism in the oracle (the routes don't load there, but the CSS does): injected a
  `form-grid` with four fields into a 600px `.row-edit` vs `.row` — 3 columns vs 1. SW cache v5→v6.

## 2026-06-17 — bugfix C15: collapsed rail hid every nav icon

- Collapsing the sidebar left only the brand mark and the active-item highlight — no nav icons — so you
  couldn't navigate while collapsed, and B5's hover-flyout had nothing to reveal. The collapse CSS hid
  `aside.rail.collapsed nav > div` to drop the "TOOLS"/"SYSTEM" section labels, but the framework wraps
  each `uic.CreateElement(navItem, …)` in its own `<div>`, so the selector matched every item too.
- Fix: gave `railHeader` a `rail-section` class and retargeted the rule to `nav .rail-section`. The
  `<768px` mobile rail had a copy of the same `nav > div` rule (C10) — fixed it there as well, so the
  phone rail won't lose its icons either.
- Bumped SW cache v4→v5 (index.html is a precached CORE asset and changed).
- Verified live by adding `.collapsed` to the rail in the oracle and reading computed styles: railW=58,
  14/14 `.nv` items visible, 2/2 `.rail-section` headers hidden.

## 2026-06-17 — bugfix C1: period totals dropped first-of-period UTC-dated transactions

- The Dashboard Income KPI read `$0.00` for June even though a $4,200 salary was dated 2026-06-01. Root
  cause was a timezone mismatch: transaction dates are stored at **UTC midnight** (`ParseDate` parses in
  UTC, and the add flow round-trips the date string through it), but the period boundary builders in
  `dateutil` constructed the window start in the **machine's local timezone** (`t.Location()`). On any
  machine behind UTC, the local month-start (e.g. `Jun 1 00:00 −05:00` = `Jun 1 05:00Z`) sorts *after* a
  `Jun 1 00:00Z` transaction, so `InRange` (`!Before(start) && Before(end)`) excluded it. The Jun 2–5
  expenses survived because they're a day clear of the boundary.
- **Fix — one canonical convention: UTC-midnight calendar dates.** `midnight`, `MonthStart`,
  `FiscalMonthRange`, and `NextMonthlyDue` now take the calendar date from the reference instant (in its
  own location — the user's wall calendar) but emit the boundary at `time.UTC` midnight; `WeekStart`
  inherits it via `midnight`, and `period.quarterStart` got the same treatment. Now both sides of every
  in-range comparison are UTC calendar dates.
- Why take the date in local but emit in UTC: "what month is it" should follow the user's wall calendar,
  while the boundary must align with UTC-stored dates. Worked the edge cases (late-night behind-UTC, the
  1st in a far-behind zone, far-ahead zones) — a same-day-1 UTC transaction is now always counted.
- Added `TestPeriodBoundariesAreUTCRegardlessOfZone` (UTC-5 / UTC-11 / UTC+13) asserting the boundary is
  UTC midnight and a first-of-month `00:00Z` txn is in range. Existing date/period/ledger suites stay
  green (their fixtures were already UTC). Verified live: Income KPI now shows `$4,200.00`.

## 2026-06-17 — bugfix C16: net-worth trend chart plotted cents

- The net-worth trend D3 chart fed `Y: float64(m.Amount)` — raw minor units (cents) — so the Y axis
  ticked in millions of cents and the labels clipped/looked non-monotonic ("000,000 / 500,000") in the
  narrow 44px-margin widget. The big figure above was fine because it used the money formatter.
- **Fix, two parts.** (1) `dashboard.go` now divides by the currency's decimal factor (same `div` loop
  idiom as `customize.go`) so the chart plots dollars, and sets `Y.Format` to a compact-currency d3
  spec (`$.2~s` for `$` currencies, `.2~s` otherwise). (2) `web/chart.js` now honors the per-axis
  `format` hint (`chartspec.Axis.Format`) — it builds a `d3.format` tick formatter for X and/or Y when a
  spec is present, falling back silently on an invalid spec. The `Axis.Format` field already existed in
  the spec; nothing was reading it.
- Bumped the service-worker cache (`cashflux-v3` → `v4`) because `chart.js` is a cached CORE asset —
  otherwise returning users keep the old shim.
- **Audited the other chart feeds** as the TODO asked: `customize.go` already converts to major units;
  the planning `AreaChart` is the pure-Go sparkline (`internal/ui/chart.go`) — a normalized path with no
  numeric axis, so cents-vs-dollars only ever affected its (identical) shape, not any label. No other
  `*.Amount`-fed chart had the bug.
- Verified live in the headless oracle: Y-axis now reads `$0, $5k, $10k, $15k, $20k` (monotonic, fits).
- Note: there were still-open C-series items beyond C2–C14 (C1, C15–C19). C16 done here; C1 (Income $0
  correctness) and C15 (collapsed rail loses nav) are the next high-value bugs.

## 2026-06-17 — bugfix C2 (negatives): one money formatter, parentheses everywhere

- Closed the last open part of C2. `fmtMoney` rendered negatives with a minus sign (`-$60.20`) while
  `fmtAccounting` used accounting parentheses (`($60.20)`), so the Transactions rows disagreed with the
  Dashboard KPIs. The CLAUDE.md standard is accounting format, so I unified on parentheses.
- **Approach — collapse to one formatter rather than migrate 38 call sites.** The two functions differed
  only in negative style (both already grouped thousands). I rewrote `fmtMoney` to delegate to
  `money.FormatAccounting`, making it identical to `fmtAccounting`, then deleted `fmtAccounting` and
  pointed its 11 Dashboard call sites at `fmtMoney`. One canonical display formatter, no redundant twin.
- **The risk the TODO flagged — parentheses must never reach an input value — does not apply.** Grepped:
  there is no `Value(fmtMoney(...))` anywhere; editable inputs prefill with `money.FormatMinor` (plain
  minus, no symbol) and parse with `money.ParseMinor`. `fmtMoney` is display-only, so the change can't
  leak a parenthesized string into a form field.
- Verified live in the headless oracle: the Dashboard (which already used parentheses, now via the
  collapsed `fmtMoney`) still renders correctly — `$20,749.25 | $1,800.75 | ($60.20) | ($240.55) |
  ($1,500.00)` — confirming the collapse caused no regression. The other screens route through the same
  tested formatter, so their rows now match.
- With this, every substantial C-series bug (C2–C14) is fixed. What remains is B2 *enhancement* polish
  (live drag-over preview, FLIP animations) — beyond the reported bug, which is already fixed.

## 2026-06-17 — bugfix C13: quick-add panel height fits its content

- The "+ Add" quick-add flip panel used the default FlipPanel height (470px), leaving the compact
  6-field "Add a transaction" form floating in a tall, mostly-empty card. Set `Height: "420px"` on the
  panel so it fits its content; the `.set-body` keeps `overflow-y:auto`, so a too-short panel just
  scrolls — there's no risk of clipping content with a fixed reduction.
- Verified live: drove the headless browser oracle to click "+ Add" (async re-render handled with a
  Playwright-awaited async IIFE + 500ms settle), then measured `.flip-wrap` — `panelH=420 fields=5`.
  Confirms the panel both opens correctly and renders at the new height.
- The *richer quick-add* part of this item (scan-bill / scan-document / custom-workflow add cards)
  stays open under **B11**; only the empty-space/height complaint is resolved here.
- Next: the remaining C-series items are minor/debatable — C2-negatives (parentheses-vs-minus
  unification across screens, risk of parentheses leaking into input values) and the B2 drag polish
  (live drag-over preview, FLIP animations). Assess whether to wrap the bug pass.

## 2026-06-17 — bugfix C9: Accounts asset-input placeholders clipped

- The asset inputs on the Accounts add/edit form had long placeholders ("Expected return APR %",
  "Liquidity 0–100", "Stability 0–100") that clipped in the ~150px grid columns. Shortened the
  placeholders to "Return %"/"Liquidity"/"Stability" and added `title` attrs with the full label + range
  (e.g. "Liquidity score (0–100)") on both the add form and the edit row. Route-gated (the static serve
  has no SPA fallback so I can't load /accounts directly), but shorter placeholder text definitively
  can't clip and the titles preserve the detail. i18n + wasm green.

## 2026-06-17 — C14/B2: migrate dashboard onto the Pack model

- Discovered `dashlayout/pack.go` (the ordered-sequence + bin-packing model: `Item`, `DefaultItems`,
  `Pack`, `Move`, `ResizeItem`) already existed and is table-tested — the remaining B2 work was wiring
  the UI onto it. (Reverted a duplicate `Pack`/`Move` I'd started adding to dashlayout.go before
  spotting it.)
- Migrated the UI: `uistate` layout atom is now `[]dashlayout.Item` (`UseLayoutItems`/`PersistItems`;
  legacy `[]Placement` localStorage migrates for free since unmarshaling into `Item` ignores col/row).
  `widget.go` now `Pack(items, 4)`s for rendering (row offset +1 for the fixed header), drag-drop calls
  `Move(items, src, targetIndex)` (reorder → reflow, not pairwise Swap), and resize calls `ResizeItem`.
  Dashboard "Reset layout" uses `DefaultItems`. Added `grid-auto-rows` to `.bento` so packed layouts
  taller than 8 rows still render.
- Fixes C14's core: a widened tile no longer overlaps its neighbor (Pack reflows), so the resize handle
  is never painted-over / "stuck"; the wrap-at-max is now a clean reflow (and tooltips say "cycles
  1→4/1→3"). **Verified in a headless browser**: the packed default arrangement is pixel-identical to
  before (dumped every tile's grid-area; matches), full test suite + wasm green. Remaining B2 polish
  (deferred): live drag-over reflow preview, FLIP animations, pointer-drag over HTML5 DnD, and a direct
  one-click shrink (vs the wrap-cycle).

## 2026-06-17 — browser oracle online; verified D3 + fixed C10 (responsive)

- The engineer pointed out the gwc MCP browser tools. The shipped `.tools/gwc.exe` lacked the
  `playwrightgo` build tag, so I rebuilt the runner from the GoWebComponents checkout
  (`go build -tags playwrightgo -o .tools/gwc-pw.exe ./tools/gwc`); Playwright/Chromium was already
  installed. Now I can `serve` the app and drive it with `gwc-pw screenshot/eval` (headless). PowerShell
  5.1 mangles double-quotes in native args, so JS exprs must use single quotes wrapped in a PS
  double-quoted string.
- **Verified B14 D3 charts**: `eval` on the live dashboard reports `d3=object`, `.cf-chart svg` present
  → the net-worth trend renders via D3 with axes. The boot overlay seen in a screenshot is just the
  0.45s fade (`#boot` already has class `hidden`), not a stuck overlay.
- **Fixed C10** (was flagged blocked-on-browser): a `@media (max-width:767px)` block forces the rail to
  its icon-only 56px, hides rail text, stacks the bento into one column (overriding tiles' inline
  grid-column/row with `!important`), and clamps `overflow-x`. Verified at 390px: `scrollWidth ==
  clientWidth == 390`, `overflow=false`, rail 56px. Desktop (1280/1366) unaffected. Note: the top-bar
  resolution control can still overflow on a phone (no page-wide scroll though) and there's no slide-in
  drawer (icon nav is tappable) — minor follow-ups.

## 2026-06-17 — bugfix C9: Insights bare without a key

- Verified: the offline Spending-highlights card already renders unconditionally (line 205), so that
  half of C9 was done. The "Ask about your money" card, though, was `If(key != "", …)` — invisible to
  keyless users. Restructured so the Ask card always renders: the working `Form` stays under
  `If(key != "")` (identical OnSubmit/OnInput hook conditionality — no hook-order change), and a
  `If(key == "")` branch shows a disabled input preview + the existing `insights.keyHint` so the
  feature is visible and self-explanatory. wasm + vet green.

## 2026-06-17 — bugfix C12: last settings row clipped by footer

- The global settings flip panel is a flex column (header / `.set-body` flex:1 overflow-auto / sticky
  `.set-foot`), so the last row could sit flush against the footer fold and read as clipped. Per the
  TODO's prescribed fix, bumped `.set-body` bottom padding (1rem → 1.5rem) so the final row clears the
  footer and scrolls fully into view. Pure CSS; low-risk regardless of the exact rendering cause (I
  can't browser-verify, but extra bottom padding only helps).

## 2026-06-17 — bugfix C3: household card (verified resolved + tidy)

- Re-checked C3 against current code: the reported "GWC avatar overlapping/clipping its own text" is
  gone — `HouseholdCard` is now a clean flex Button (gear icon + a two-line text span), and the only
  `.hh` CSS is the collapsed-rail rule (no absolute positioning, no avatar). So the layout bug is
  resolved by the redesign since the review. Minor tidy: dropped the redundant trailing "· Settings"
  from the visible summary (the gear icon + tooltip already signal it), keeping it in the Title tooltip.
  wasm + vet green.

## 2026-06-16 — bugfix C2 (part): grouped fmtMoney

- `fmtMoney` rendered ungrouped amounts ("$20749.25"), so every screen using it (Accounts/Budgets/
  Goals/Allocate/Documents/Plans…) looked locale-naive next to the Dashboard's grouped accounting
  figures. Wrapped its `FormatMinor` in the existing `money.Group`, so all `fmtMoney` output is now
  comma-grouped — fixed in one place. Kept the **minus-sign** negative style (not parentheses) so
  there's zero risk to any input pre-filled via `fmtMoney` (parentheses wouldn't re-parse) and no
  jarring inline parentheses. The remaining C2 sub-point — unifying negatives to parentheses (Transactions
  rows minus vs Dashboard parentheses) — is a visual-standard call best verified in a browser and is
  left as a follow-up (the row-level `fmtAccounting` migration). wasm + money tests green.

## 2026-06-16 — bugfix C4: resolution control on non-period screens

- The TopBar rendered `ResolutionControl` on every route, so a Week/Month/Quarter stepper showed on
  Categories/Members/Rules/etc. where it does nothing. Gated it behind a `periodAware` set
  (`/`, `/transactions`, `/budgets`, `/planning`, `/insights`) derived from
  `router.InspectCurrentRoute().Path`. Left `+ Add` everywhere — logging a transaction is a valid action
  on any screen, so it has an obvious target (C4's second bullet is a no-op by that reasoning). wasm +
  vet green.

## 2026-06-16 — bugfix C9: category color never surfaced

- `domain.Category.Color` existed but the Categories screen never read or set it. Wired it end to end:
  the Add form and the inline Edit row got a `.color-input` (reusing the C8 class), threaded color
  through `saveCat` (new param) and `categoryRowProps.OnSave` (signature widened), and each display row
  now shows an 11px `.cat-swatch` colored dot before the name. A `catColor` helper falls back to a
  neutral default for categories created before colors existed. Persistence was already free (the store
  JSON-blobs the whole Category). New `categories.color` key. i18n + wasm green.

## 2026-06-16 — bugfix C8: member color picker was a bare line

- The Add/Edit member forms used `<input type="color">` with the `.field` class (tuned for text), so
  the native color control collapsed to a thin line. Gave it a dedicated `.color-input` class
  (46×34, padding, swatch-wrapper resets for webkit) so it renders as a proper clickable color swatch,
  plus a `title`/`aria-label` "Member color". Chose styling the native input over swapping to the fixed
  `SwatchPicker` palette — it keeps full color choice, avoids an off-palette-selection edge case, and is
  a smaller, reliable change (native color inputs honor explicit width/height). New `members.color` key.

## 2026-06-16 — bugfix C5: two "Net worth" tiles read as duplicates

- The dashboard has both the `kpi-networth` KPI (figure + ▲/▼% this month) and the `trend` tile (figure
  + D3 chart), and both were titled "Net worth", so they looked like a redundant duplicate. They're not
  truly redundant (one's a number, one's a curve), so per C5's "differentiate it" option I retitled the
  trend tile to **"Net worth trend"** (new `dashboard.netWorthTrend` key) rather than deleting it. Now
  the KPI and the trend chart read as distinct widgets. i18n + wasm green.

## 2026-06-16 — bugfix C6: Allocate zero-score noise (+ labels verified)

- C6 part 1 (unlabeled weight inputs) was already fixed — the five weight inputs carry
  `Title`+`Placeholder` ("Returns weight", … "Goal-progress weight"). Part 2 (zero-score candidates):
  `RankWith` returned accounts with all-zero criteria, which `AllocRow` rendered as
  "0% · returns 0 · stability 0 · liquidity 0" noise. Now filter `ranked` to `Score > 0` before both
  the amount-split `Distribute` and the list; track `hiddenZero` so when filtering empties the list we
  show a "set expected return / stability / liquidity on your accounts" hint instead of the generic
  empty message. New `allocate.setAttributes` key. i18n + wasm green.

## 2026-06-16 — bugfix C11: empty widget panel showed Save

- Added a `CloseOnly` option to `ui.FlipPanel`: when set, the footer is a single "Close" button instead
  of Cancel/Save (refactored the footer into a precomputed node + shared save/cancel closures).
  `SettingsHost` sets `CloseOnly: !widgetcfg.Has(target.ID)` for the per-widget panel, so a widget with
  no settings schema no longer shows a Save button implying there's something to commit. Verified by
  inspection (the placeholder body "This widget doesn't have any settings yet." now pairs with Close).
  Also checked C9's "goals unlabeled current-amount field" — already fixed (it has a "Saved so far"
  placeholder), so that one was stale. wasm + vet green.

## 2026-06-16 — bugfix C7: budget row "Food · Food"

- Shifted to the C-series UI bugs. `BudgetRow` built its label as `Name + " · " + Category`
  unconditionally, so a budget named after its category read "Food · Food", and an unnamed budget read
  "· Food". Replaced with a switch: empty name → show the category alone; name == category
  (case-insensitive) → show one; otherwise "name · category". Code-logic fix verified by inspection
  (no browser needed). C7's second part (the Budgets card's own month stepper duplicating the global
  resolution control) is a separate UX concern tied to C4/B10 — left for its own slice. wasm + vet green.

## 2026-06-16 — B14: migrate net-worth trend widget to ui.Chart (proof)

- Migrated `netWorthTrendWidget` from the pure-SVG `uiw.AreaChart` to `uiw.Chart`: build a
  `chartspec.Spec{Kind: Area, Series: [{Points: (i, netWorth[i])}]}` (empty Color → theme accent) and
  render it at 120px. This is the B14 proof that the Go → JSON → D3 pipeline works end to end. Kept the
  blast radius to one widget — `AreaChart` still renders the planning forecast and the plan-row
  sparklines — so if D3 misbehaves only this widget is affected. wasm + vet green. **Needs an
  in-browser smoke test** (D3 render correctness, theme colors, resize) before declaring parity and
  migrating the rest.

## 2026-06-16 — B14: ui.Chart Go component (drives the D3 shim)

- Added `internal/ui/chartd3.go` — `ui.Chart(ChartProps{Spec, Height, Class, Label})`. It renders a
  managed container `Div` with a stable `UseId` id, marshals the `chartspec.Spec` to JSON, and in a
  `UseEffect` keyed on that JSON resolves the element via `getElementById` and calls
  `window.cashfluxRenderChart(el, json)` (guarded by `fn.Type()==TypeFunction`). Cleanup clears the
  box's innerHTML on unmount / before a redraw, so no stale SVG lingers — the ref/portal pattern for
  letting D3 own that subtree without fighting the vdom. `role="img"` + `aria-label` for a11y. Renamed
  the internal component func to `chartD3` to avoid colliding with the imported `internal/chart`
  package. wasm + vet green; D3 render still needs a browser check.
- Next: migrate one widget (net-worth trend) from the pure-SVG `AreaChart` to `ui.Chart` as the proof.

## 2026-06-16 — B14: D3 chart shim + offline caching (renderer foundation)

- User chose D3 for B14 (via AskUserQuestion). Started the integration bottom-up: JSON-tagged the
  `chartspec` types (lowercase wire keys), added `web/chart.js` — a self-contained `cashfluxRenderChart(el, specJSON)`
  shim that parses a Spec and draws line/area/bar (with D3 axes) or donut, reading axis/grid/default
  colors from the app's CSS custom properties so it's theme-aware and redraws on each call. Pinned D3
  v7.9.0 via CDN and `./chart.js` in index.html before the wasm boot, and added both to the service
  worker's CORE cache (bumped CACHE → v3) for offline use.
- Verified: chartspec tests green, wasm builds, `node --check` passes on chart.js. NOT verified: actual
  D3 rendering (needs a browser — flagged). Next slices: the Go `ui.Chart` component (managed container
  + effect that serializes the spec and calls the shim, with cleanup) and migrating one widget
  (net-worth trend) as the proof.

## 2026-06-16 — a11y: aria-required across all add forms

- Completed required-field marking: added `aria-required="true"` to each add form's primary required
  input — account name, category name, budget limit, goal name + target, member name, rule match,
  to-do title, transaction amount (the add forms only; edit-row inputs use the S-suffixed vars and were
  left). Done via a targeted per-file replace keyed on each input's unique `Placeholder/Value` segment
  (verified exactly one match per target). Still `aria-required` (semantic) not native `required`, to
  avoid clashing with the app's own validation. wasm + vet green. Closes the required-field-marking
  part of the forms a11y item; per-field `aria-describedby` remains.

## 2026-06-16 — a11y: aria-required on key form fields

- Marked the genuinely-required inputs with `aria-required="true"` on the forms I recently built:
  quick-add amount, and the plan name + horizon. Deliberately used `aria-required` (semantic) rather
  than the native `required` attribute, which would trigger the browser's own validation popup and
  conflict with the app's custom Go-side validation + the `role="alert"` error messages. Contained to
  two forms (not a scattered sweep). wasm + vet green. Marking required fields across every add form is
  a broader follow-up.

## 2026-06-16 — a11y: announce form validation errors (role=alert)

- Added `role="alert"` to all 17 inline error-message `P(Class("err"), …)` elements across the screens
  (accounts, allocate, budgets, categories, customfields, customize, documents, goals, insights,
  members, planning ×4, rules, todo, transactions) via a safe mechanical replace. `role="alert"` is an
  implicit assertive live region, so a validation failure is spoken when it appears rather than being a
  silent style change (WCAG 3.3.1). wasm + vet green. (Per-field `aria-describedby` association is a
  larger follow-up; this gives the announce-on-appear win broadly with one pass.)

## 2026-06-16 — planning: projected-balance sparkline on each plan row

- `PlanRow` now renders a compact `uiw.AreaChart` of `planning.Project(p)` (int64 curve → float64),
  toned green/red by whether `EndBalance` is above/below `StartBalance`, with a per-plan `GradientID`
  ("cf-plan-"+ID) to avoid SVG gradient id collisions across rows, a `role="img"` accessible label
  (`plans.chartLabel`), and an `If(len>1)` guard. Reuses the existing tested chart geometry + planning
  engine — no new logic. i18n + wasm green.

## 2026-06-16 — settings: accent-contrast indicator (uses contrast pkg)

- Added `accentContrastNote` under the accent SwatchPicker in `globalSettingsForm`: computes the
  selected accent's WCAG ratio against the theme's elevated surface via `contrast.Ratio` and shows a
  muted "Contrast X.X:1 — passes AA" line, or a danger-colored "low; may be hard to see" warning when
  it fails AA for UI/large elements (3:1 — accent is used for fills/active states/focus ring). For the
  system theme it checks both dark and light surfaces and reports the worst. This puts the earlier
  contrast audit in front of the user (the default green flags as low on light) and lets them choose a
  safer swatch. New `settings.accentContrast*` keys. i18n + wasm green.

## 2026-06-16 — planning: one-time item in the plan create form

- Extended the Plans create form with an optional one-time amount + "in month #" pair. When both are
  given (and the month is within the horizon — else a validation error), `addPlan` appends a
  `PlanItemOneTime` alongside the recurring monthly item, switching `p.Items` to `append` so a plan can
  carry both. The model/engine/persistence already supported one-time items (engine + projection were
  table-tested earlier), so this is pure form wiring + validation. New `plans.once*` i18n keys. The
  projected end balance reflects the windfall/expense. i18n + wasm green. A full per-plan item
  add/remove editor (editing existing plans' items) remains a later option.

## 2026-06-16 — a11y: keyboard operability for div-based switch/swatch

- Verified the framework exposes a declarative `OnKeyDown` (shorthand → `html.OnKeyDown`) whose callback
  gets a `ui.KeyboardEvent` with `GetKey()`/`PreventDefault()` — so no per-instance js wiring is needed.
  Made the `Toggle` (role=switch) and `Swatch` (role=radio) `<div>`s keyboard-operable: added
  `tabindex="0"` and an `OnKeyDown` that toggles/selects on Space/Enter (PreventDefault on Space to stop
  page scroll). The existing `:focus-visible` rule already targets `[role=switch]`/`[role=radio]`, so
  they get a focus ring for free. (The Segmented control is real `<button>`s — already operable.)
  This resolves the earlier "needs a framework key handler" blocker. wasm + vet green.

## 2026-06-16 — a11y: theme-token contrast audit + text-faint fix

- Audited the suspect tokens with the new `contrast` package (throwaway test, not committed). Findings:
  `text-faint` failed AA-normal on both themes — dark #6c6c72 = 3.70/3.33 (bg/elev), light #8a8a90 =
  3.18/2.93 (the light-on-elev even failed AA-large). text-dim, danger, and accent-on-dark all passed.
  The shared **accent** #54b884 fails on light (2.27 on bg / 2.45 on card) — but it's mostly fills/large
  UI and changing the brand hue is a visual decision, so I flagged it rather than altering it.
- Fix (data-driven): lightened dark `--text-faint` to **#888890** (5.49/4.94) and darkened light
  `--text-faint` to **#686870** (5.11/4.72) — both clear AA-normal (4.5) on base and elevated surfaces,
  picked by iterating candidates through `contrast.Ratio`. Updated the dark var, the light var, and the
  light `.text-faint` utility override. Pure CSS; no Go change.

## 2026-06-16 — contrast: WCAG luminance/ratio utility (pure)

- New pure package `internal/contrast`: `ParseHex` (3/6-digit, optional #), `RelativeLuminance`, `Ratio`
  (1..21, symmetric), and `PassesAA`/`PassesAAA` with the standard thresholds (4.5/3.0, 7.0/4.5). Exact
  sRGB linearization per WCAG. Table tests: parse incl. shorthand/error cases, black=0/white=1 luminance,
  black/white=21:1 + symmetry, identical=1, the canonical #767676-on-white ≈4.54 boundary, and the
  pass-predicate boundaries. Directly useful for the user-configurable accent (a future "this accent is
  hard to read" check) and for auditing the theme tokens. `go test` green.
- Note: built the calculator now; using it to validate the accent swatch (or warn on low-contrast custom
  accents) is a natural follow-up UI slice.

## 2026-06-16 — a11y: FlipPanel focus trap + restore

- Extended the FlipPanel modal effect (which already did Esc-to-close) into a full focus trap: on mount
  it captures `document.activeElement`, moves focus to the dialog's first focusable (querying
  `.flip-wrap` for tabbable elements, skipping `tabindex="-1"` and disabled), and traps Tab/Shift+Tab to
  cycle within the dialog (wrapping at the ends via `preventDefault` + focus). On unmount it restores
  focus to the remembered trigger. Uses `js.Value.Equal` to compare the active element to first/last.
  One change covers every overlay (quick-add + both settings panels). Completes the B15 dialogs item
  (role/aria-modal/label + Esc + focus trap + initial focus + restore). wasm + vet green.

## 2026-06-16 — a11y: minimum touch targets (24px, WCAG 2.5.8)

- Gave the small icon-only buttons (`.btn-del`, `.toast-x`, `.rstep`, `.set-close`) a `min-width`/
  `min-height` of 24px with `inline-grid` + `place-items:center` so the glyph centers in a comfortably
  tappable box — meeting WCAG 2.5.8 (AA, 24×24) without inflating the dense desktop layout. The
  `.menu-btn` already gets ≥24px from its `w-7 h-7` utility classes. 44×44 (AAA) noted as aspirational.
  Pure CSS; no Go change.

## 2026-06-16 — a11y: prefers-reduced-motion for interaction animations

- Extended the existing `@media (prefers-reduced-motion: reduce)` block to neutralize the remaining
  interaction animations: `.flip-inner`/`.flip-backdrop` transitions (dialog appears in place, no
  flip/lift), the `.toast` slide-in, and the `aside.rail` width transition. The boot animation and the
  rail flyout were already handled (flyout is gated behind `no-preference`). Pure CSS in index.html —
  no Go/wasm change. Minor residual transitions (toggle knob, button filter) left as-is (low motion).

## 2026-06-16 — B14: pure chartspec package (decision-independent half)

- New pure package `internal/chartspec`: `Kind` (line/area/bar/donut) + `Point`/`Series`/`Axis`/`Spec`,
  with `Kind.Valid`, `Spec.Validate` (unknown kind, no series, empty series, multi-series donut — all
  as sentinel errors for `errors.Is`), and `Spec.Extent` (min/max X/Y across all points, with an `ok`
  flag so callers don't scale a zero-width range). Table tests cover valid/invalid specs and extent
  (incl. empty). This is B14's framework-agnostic foundation — a renderer (pure-Go SVG or D3) consumes
  a Spec.
- Flagged decision (genuinely the user's): whether to adopt **D3** for the renderer (large JS dep +
  offline SW caching + vdom-portal complexity) or **keep growing the pure-Go SVG** helpers. The
  chartspec package is useful either way, so I built it without committing to that choice; the renderer
  (`ui.Chart`) waits on the decision. Pure, no `syscall/js`; `go test` green.

## 2026-06-16 — B13: rewire ui.Icon to icon.Name (call sites migrated)

- `ui.Icon`/`iconBody` now take `icon.Name` and switch on the `icon.*` constants; migrated every call
  site (railItem.Icon and navItemProps.Icon fields → `icon.Name`; nav literals, members/categories/
  rules, the household `Settings` and top-bar `Menu` icons). Checked the framework for a raw-SVG inject
  primitive (dangerouslySetInnerHTML/innerHTML) to consume `icon.Inner()` directly — none exists, so
  the renderer keeps the typed shorthand shapes and the icon package remains the compile-checked name
  vocabulary (its `Inner()` stays available for non-render consumers and a future Lucide-string
  renderer). Net effect: a mistyped icon is now a build error, not a blank SVG. Glyphs unchanged; wasm
  + full native suite green.

## 2026-06-16 — B13: pure icon registry (type-safe Name)

- New pure package `internal/icon`: a `Name` type with compile-checked constants for the 16 curated
  icons, each mapping to its inner SVG markup (lifted verbatim from the hand-rolled `ui.iconBody`), plus
  `Inner()`/`Valid()`/`All()`. Kept the existing string names (not Lucide ids) so the later `ui.Icon`
  rewire is mechanical and the glyphs stay pixel-identical. Tests: every constant resolves to non-empty
  inner-only markup, unknown names are invalid/empty, `All()` is sorted and matches the curated set.
- Scope/flag: this is B13's pure interface + data half. Deferred (separate slices): rewire `ui.Icon`
  to take `icon.Name` and migrate call sites; and the optional generator to fetch real Lucide path data
  (the current curated glyphs are already Lucide-format stroked SVGs, so that's a refinement not a
  blocker). No `syscall/js`, so it's fully `go test`-verifiable.

## 2026-06-16 — allocate: max-per-destination input (§2.7)

- Surfaced the `Distribute` engine's already-tested `MaxPer` cap in the Allocate amount-split UI: a new
  "Max per destination" number input (`maxPerStr` state + handler), parsed to minor units and passed as
  `SplitOptions.MaxPer`. Overflow from caps falls into the existing kept-back remainder note, so no new
  display logic was needed. Pure wiring of an existing capability — no logic/test changes. Closes the
  "amount-split UI next" item under §2.7 constraints. i18n + wasm green.

## 2026-06-16 — B10: presets dropdown

- Added a "Jump to…" quick-pick to `ResolutionControl`: a native `<select>` (keyboard-accessible, no
  custom menu state) with This period / Last period / This quarter / Year to date, wired to
  `period.NewWindow`/`Previous`/`YearToDate`. It's an action menu — the placeholder option is always
  `SelectedIf(true)` so the control snaps back after applying. Quarter/YTD also persist the resolution
  change. The `onPreset` handler is a `uic.UseEvent` at a stable position (alongside the rangeMode
  state). New `resolution.preset*`/`jumpTo` keys. With this, B10's UI redesign is essentially done
  (single stepper + reset + custom range + presets); only narrow-width responsive behavior remains.
  i18n + wasm green.

## 2026-06-16 — B10: ResolutionControl rebuilt (single stepper + reset + custom range)

- Rebuilt the top bar's `ResolutionControl`. Default is now a single-period stepper using
  `Window.Label()`/`Shift(±1)` (pages the whole window, reads as one label). A local `uic.UseState`
  `rangeMode` toggles to the old dual From/To steppers (`Custom range` ↔ `Single period`); leaving
  range mode calls `Window.Single()` to collapse cleanly. A `This period` button appears only when
  `!w.IsCurrent(now)` — the off-now cue + one-tap reset to `NewWindow(res, now)`. Granularity segmented
  unchanged. Safe to render the steppers conditionally because `StepperPill`/`Segmented` are
  `CreateElement` components (isolated hooks); the only hook in ResolutionControl is the rangeMode
  state. New `resolution.*` keys. Deferred to a follow-up: a presets dropdown (This/Last/YTD) and
  narrow-width responsive behavior. i18n + wasm green.

## 2026-06-16 — B10: single-period Window helpers (pure logic)

- Added to `period.Window`: `IsSinglePeriod()` (From == To), `Single()` (collapse To := From), and a
  combined `Label()` that returns one unit label when single ("Jun 2026") or "from – to" for a range.
  This is the pure-logic prerequisite for the redesigned single-stepper resolution control — the label
  collapses correctly so the common "this month" case stops reading as "Jun 2026 – Jun 2026". Three
  table tests (single vs range predicate, collapse, label both ways). Next B10 slices: rebuild
  `ResolutionControl` as single-stepper + presets dropdown + this-period reset (UI).

## 2026-06-16 — B12: goals widget config schema (sweep complete)

- Registered a `goals` widget schema: "byProgress" Toggle (default false) and "showDate" Toggle
  (default true). Wired `goalsWidget` — byProgress features the goal with the highest `goals.Percent`
  (first wins ties) instead of `list[0]`; showDate gates the "· by <date>" caption. A dynamic
  goal-picker `Select` doesn't fit the static-at-init schema model, so "nearest completion" is the
  pragmatic, explainable knob. Registration/default test added.
- With this, every dashboard widget that has feasible settings now exposes them (savings, recent,
  trend, breakdown, to-do, accounts, budgets, goals) — B12's incremental-schema item is effectively
  complete. wasm + tests green.

## 2026-06-16 — B12: budgets widget config schema

- Registered a `budgets` widget schema: "count" Number (default 6, clamped [3,20]) and "atRisk" Toggle
  (default false). Wired `budgetsWidget` to optionally filter to Near/Over statuses (in-place filter on
  the fresh EvaluateAll slice) and cap the list; the empty state messages differ ("No budgets yet." vs
  "Nothing near or over budget."). Registration/clamp/bool test added. wasm + tests green.

## 2026-06-16 — B12: accounts widget config schema

- Registered an `accounts` widget schema: a "count" Number (default 6, clamped [3,12]) and a "cleared"
  Toggle (default false). Wired `accountsWidget` to read both — the limit caps the grid, and when
  cleared is on it shows `ledger.ClearedBalance` instead of `ledger.Balance` (reconciled vs current).
  Both criteria are real and explainable. Registration/clamp/bool test added. wasm + tests green.

## 2026-06-16 — B12: to-do widget config schema

- Registered a `widgetcfg` schema for the `todo` widget (a "Tasks to show" Number field, default 3,
  clamped [1,10]) and wired `todoWidget` to read it via `SchemaFor("todo")` + `f.Int(cfg)`, replacing
  the hardcoded `open[:3]`. The per-widget flip-panel renders the field generically, so no UI change
  was needed beyond passing the config through. Added a registration/clamp test. First of the B12
  "more widget schemas" additions; goals/accounts/etc. can follow the same pattern. wasm + tests green.

## 2026-06-16 — a11y: FlipPanel is a real modal dialog

- Gave `ui.FlipPanel` `role="dialog"` + `aria-modal="true"` + `aria-label` (its title) on the flip-wrap,
  and an Esc-to-close listener via a `UseEffect` keyed `true` (added on mount, removed + `cb.Release()`
  on unmount — matches the panel's open/close lifetime since it mounts fresh each open). One change
  covers every overlay: quick-add and both settings panels. Still TODO for the dialogs item: a focus
  trap + moving initial focus into the dialog (needs a DOM ref to the panel node). wasm + vet green.

## 2026-06-16 — B11: quick-add transaction flip panel

- The top bar "+ Add" no longer navigates to /transactions; it opens a quick-add flip panel via a new
  `uistate.UseQuickAdd()` bool atom + a `QuickAddHost` mounted at the shell root (mirrors SettingsHost).
  Form: account, expense/income segmented, amount, description, optional category, date (defaults to
  today). Save builds the transaction (expense → negative) and calls `app.PutTransaction`, bumps the
  data-revision atom so screens refresh, and toasts the outcome.
- Hook-ordering care: all hooks (atoms, 6×UseState, 5×UseEvent) run unconditionally *before* the
  open/closed guard so hook order is stable across opens. Avoided setting state during render — used
  effective fallback values (first account, today) for both display and save instead, so an immediate
  Save works without a pre-render Set. FlipPanel always closes after Save, so validation failures
  report via an error toast (the now-persistent live region) rather than inline. New `quickAdd.*`
  i18n keys. i18n + wasm green. Closes B11.

## 2026-06-16 — Planning UI: plans card (§2.6)

- Added a "Savings & spending plans" card to the Planning screen: create (name, horizon, start
  balance, monthly change), list, and delete. The create form captures one steady monthly change as a
  recurring `PlanItem` (the common case); one-time items can be a later slice. Each saved plan renders
  via a `PlanRow` component (own delete hook, no-hooks-in-loops) showing horizon/start/monthly meta
  and the projected end balance from `planning.EndBalance`, toned by `figTone`. New `plans.*` i18n
  keys. Closes §2.6's Plan/PlanItem feature across model → logic → persistence → UI. i18n + wasm green.

## 2026-06-16 — Plan persistence (§2.6)

- Wired `domain.Plan` through the store exactly like the other JSON-blob entities: a `plans` table
  (id + data TEXT), `Dataset.Plans` field, replaceRows/loadRows in the snapshot path, and
  Put/Get/Delete/List CRUD. Added validated appstate accessors `Plans()`/`PutPlan` (requires id, name,
  positive horizon) /`DeletePlan`. No SchemaVersion bump — additive, the blob carries everything.
  Tests at every layer: store CRUD (incl. nested PlanItem round-trip), dataset export/import lossless
  check, and appstate validation (rejects no-id/no-name/zero/negative horizon). Full suite green.
- Next: the Planning screen UI to create/list/project/delete plans (UI last).

## 2026-06-16 — Plan model + planning engine (§2.6)

- Added `domain.Plan{ID, Name, HorizonMonths, StartBalance, Items}` and `domain.PlanItem{ID, Label,
  Kind, Amount, Month}` with `PlanItemKind` (recurring | one_time). Interpreted the TODO's vague
  "BaseScenario" as `StartBalance` (the base starting point) and "Assumptions[]" as `Items` — plain
  data, JSON-tagged, `omitempty` on optional fields. Kept `domain` a pure data leaf: no forecast
  import there.
- New pure package `internal/planning` composes domain + forecast: `Project` (balance curve over the
  horizon), `MonthlyNet` (recurring-only steady change), `EndBalance` (last projected, or StartBalance
  if no horizon). Layering keeps both `domain` and `forecast` as leaves; planning is the thin glue.
  Seven table tests (recurring+one-time projection, monthly-net excludes one-time, end balance with/
  without horizon, empty horizon, unknown-kind ignored). Fixed a miscalculated expected-curve fixture
  (net is +40000/mo, not +400). Next: Plan persistence (store + dataset + appstate), then the UI.

## 2026-06-16 — documents UI: monthly-spend summary view (§2.2)

- The Documents screen renders `spendsummary.Summarize` over the draft (awaiting-import) rows as a
  per-month out/in/net card, shown between the review list and the CSV card and only when rows exist.
  Amounts read at the selected import account's currency precision (falls back to base/USD), formatted
  via `fmtMoney`; the undated bucket shows "No date". New i18n keys documents.summaryTitle/Desc/
  Undated/OutIn. Closes §2.2's monthly-spend summary across logic + UI. i18n catalog + wasm green.

## 2026-06-16 — spendsummary: monthly-spend summary logic (§2.2)

- New pure package `internal/spendsummary`: `Summarize([]extract.Row, decimals) []MonthSpend` buckets
  rows by "YYYY-MM" and totals Out (spend = negative amounts) vs In (positive), matching the import
  flow's sign convention (vision prompt: negative = expense). Design choices: tolerant date parsing
  across ~10 layouts; reuses `money.ParseMinor` for exact minor-unit amounts (after stripping $ and
  commas) — no float rounding; undated rows collect under an empty Month sorted last (surfaced, not
  dropped); a row with an unparseable amount still counts toward Count but adds 0 (honest totals).
  Seven table tests (buckets/totals/Net, ordering, mixed formats, undated+garbage, currency symbols,
  empty). Kept `extract` slim by putting this in its own package rather than extending Row's package.
- Next (UI last): a Documents-screen summary view over the current extracted rows.

## 2026-06-16 — allocate UI: goal-progress weight wired end-to-end

- Final (UI-last) slice of the goal-progress criterion. The Allocate screen now: populates each goal
  candidate's `GoalProgress` from `goals.Percent(g)/100`; adds a "Goal-progress weight" number input
  and a "Finish goals" preset (GoalProgress 4); threads the weight through setWeights / resolveWeights
  (saved profiles) / the live `weights` struct / saveProfile; and the breakdown line appends a "· goal
  N%" note for goal candidates (built alongside the existing pays-debt note, so the i18n arg count is
  unchanged). New keys: `allocate.wGoal`, `allocate.goals`, `allocate.goalNote` (literal `%%`). i18n
  catalog + wasm green. Completes §2.7's goal-progress criterion across all layers.

## 2026-06-16 — persistence: goal-progress weight on saved profiles

- Added `GoalProgress float64` (`json:"goalProgress,omitempty"`) to `domain.AllocationProfile`. Because
  the store serializes each profile as a JSON `data` blob (id + data TEXT), no SQL/schema change is
  needed — the field round-trips for free, and `omitempty` keeps old rows clean. Extended the CRUD and
  dataset round-trip tests to assert the weight survives save/load and export/import. Older profiles
  without the field load as 0 (goal progress simply doesn't influence their ranking). This is the
  persistence layer beneath the next slice — the Allocate screen's goal-progress weight input/UI.

## 2026-06-16 — allocate: goal-progress criterion (§2.7)

- Added a fifth allocation criterion, goal progress, to `internal/allocate`: `Candidate.GoalProgress`
  (0..1 completion of a linked goal), a `Weights.GoalProgress` knob, and a `Breakdown.GoalProgress`
  term. The score is the clamped completion fraction, so a goal-progress weighting ranks goals nearest
  done first ("finish what's almost finished"). Deliberately additive and backward-compatible: with
  zero weight the normalized score is byte-identical to before (proven by a dedicated test), so all
  existing allocate tests stay green. Table tests cover clamping, ordering, and the zero-weight
  invariant.
- Bottom-up split: this is the tested logic only. The Allocate screen still needs a weight input, a
  profile-preset entry, populating `GoalProgress` from each goal's pace, and a breakdown line — that
  UI wiring is the next slice (UI last, per the SDLC rule).

## 2026-06-16 — B15 (slice): single h1 per screen

- Promoted the top bar's current-page breadcrumb title from a `Span` to an `<h1>`, so every screen
  has exactly one top-level heading (it lives in `<main>`, where the topbar renders). Demoted the
  dashboard's in-canvas header `<h1>` → `<h2>` so it no longer double-h1s. Other screens' section
  headings were already `<h2>`/`<h3>`, so they now sit correctly under the page h1. Closes the B15
  "single h1 + heading order" item; per-screen h2/h3 nesting can be tightened later if needed.

## 2026-06-16 — B15 (slice): color-not-only-cue audit

- Audited the UI for status conveyed by color alone. Most spots already pair color with text/shape:
  budget bars carry "On track / Near limit / Over budget" labels, the net-worth and highlight widgets
  use ▲/▼ arrows, stale accounts show a "Stale" badge. The one offender was the dashboard To-do
  widget, where high and medium priority were both `●` differing only by tone (and the dot was a
  silent glyph). Fixed: high/medium/low now use distinct shapes (▲/●/○) plus a `title`/`aria-label`
  naming the priority. Closes the B15 color-only-cue item.

## 2026-06-16 — B15 (slice): persistent live region for notices

- Reworked `Toast` so the live region is always mounted: when idle it renders an empty `.sr-only`
  div carrying `role`/`aria-live`, so the next post mutates an existing region (the reliable pattern)
  instead of mounting region+text together (which many SRs skip). Errors now use
  `aria-live="assertive"` + `role="alert"` to interrupt; ordinary notices stay `polite`/`status`.
  Added a `.sr-only` utility to `index.html`. Covers the B15 "live regions for async results" item —
  every notice flows through this one surface (saves, imports, AI, failures).

## 2026-06-16 — document.title per screen

- Set `document.title` to "<Screen> · CashFlux" on each route change (and initial load) from the same
  `Shell` route-change effect, via a new `setDocumentTitle` helper. Title is set unconditionally
  (including first render) — unlike the focus move, which still skips first render. Completes the
  document-title half of the SPA route-change a11y item; the focus half shipped in the prior slice.

## 2026-06-16 — B15 (slice): route-change focus

- On navigation, move focus into `<main>` so SPA route changes behave like a real page load for
  keyboard/SR users instead of leaving focus on the old screen's control. Implemented with a
  `UseEffect` in `Shell` keyed on `InspectCurrentRoute().Path`, guarded by a `UseRef(true)`
  first-render flag so the initial load does NOT steal focus (keeps the first Tab landing on the
  skip link). New `focusMain()` helper calls `getElementById("main").focus({preventScroll:true})`.
- Closes the B15 "route-change focus" item. Remaining B15: live regions for async results, the
  color-only-cue audit, real keyboard operability for the div-based switches/swatches and bento
  drag/resize (needs framework key handlers), and single-h1-per-screen heading order.

## 2026-06-16 — B15 (slice): skip link + focus rings

- Added a `.skip-link` as the Shell's first child (`href="#main"`), off-screen until focused, and made
  `<main id="main" tabindex="-1">` a focus target — standard skip-to-content. Plus a global
  `:focus-visible` outline (accent, 2px, offset) scoped to interactive elements/roles in `index.html`,
  honoring reduced-motion for the skip-link transition.
- Closes the "Semantics & landmarks → skip link" and "Focus visibility" B15 items. Remaining big B15
  pieces: real keyboard operability for the div-based switches/swatches and the bento drag/resize
  (needs framework key handlers), live regions, the color-only-cue audit, and the route-change
  title/focus move. New `a11y.skipToContent` key; catalog + wasm green.

## 2026-06-16 — B15 (slice): stepper + swatch labels

- `StepperPill` gained `PrevLabel`/`NextLabel` props (default "Previous"/"Next"), wired to the
  resolution control's from/to steppers with localized labels ("Move start earlier", …) so the bare
  ‹/› buttons announce. `SwatchPicker` is now a `role="radiogroup"` (aria-label "Accent color") of
  `role="radio"` swatches, each `aria-label`led by its hex with `aria-checked` for the selection.
- Same as the Toggle decision: didn't add tabindex to the div-based swatches (focusable-but-not-operable
  trap) — keyboard ops is the separate item. Closes the icon-button-label part of B15's "Custom
  controls". New `resolution.*` keys; catalog + wasm green.

## 2026-06-16 — B15 (slice): central control ARIA

- Roles/state on the shared controls (one edit → every usage): `Toggle` gets `aria-checked` + an
  `aria-label` (threaded from `ToggleRow`'s label so the unlabeled switch has an accessible name);
  `Segmented` becomes a `role="radiogroup"` (optional `Label` for the group name) of `role="radio"`
  buttons with `aria-checked`. The seg buttons are already real `<button>`s, so they're keyboard-
  operable — this just adds the semantics AT needs.
- Deliberately did NOT add `tabindex` to the div-based Toggle: making it focusable without a key
  handler (Enter/Space) would be focusable-but-not-operable (a WCAG 2.1.1 fail). Real keyboard
  operability for the div switches is the separate "Keyboard" B15 item (needs a framework key handler).
  StepperPill/Swatch icon-button labels are the next slice. wasm green.

## 2026-06-16 — B15 (slice): chart + landmark labels

- Started the accessibility program with low-risk wins: `ui.AreaChart` gained a `Label` prop and now
  renders `role="img"` + `aria-label` (an SVG without it is an unlabeled graphic to AT). The two
  callers (dashboard net-worth trend, planning forecast) pass a summary with the live figure. Labelled
  the sidebar `<nav>` landmark ("Main navigation") so it's distinct from the breadcrumb nav.
- These are the "Charts: role=img + aria-label" and part of the "Semantics & landmarks" B15 items.
  More slices to come (icon-button aria-labels, dialog focus-trap, segmented=radiogroup, live regions)
  — each its own small commit, no big-bang. New i18n keys; catalog + wasm green.

## 2026-06-16 — B8: menu visibility covers all nav items

- Extended `hideableScreens` (Settings → Screens toggles) to the Tools group (Planning/Allocate/
  Insights/Documents/Customize) and Rules — they already respected `hidden.IsHidden` in the sidebar
  filter but weren't in the toggle list, so now every routed main-line screen is toggleable (dashboard
  stays locked). Closes the B8 "menu visibility settings" sub-item; only shift+drag reorder remains.
- (Removed the stray `/settings` from the comment too, since it's no longer a screen.) wasm green.

## 2026-06-16 — B8 (partial): drop the "My pages" segment

- Removed the mockup "My pages" rail group (the three example custom pages + the "New page" item) and
  the now-dead `customPage` type / `myPages()` func, per the user's "remove the my pages segment as
  custom pages integrate directly into the page". The rail is now just the real navigable screens
  (primary + Tools + System).
- The other B8 sub-items: menu **visibility settings** already exist (Settings → Screens module
  toggles, `hideableScreens`); **shift+drag reorder** of nav items remains (the heavy part — needs a
  drag-order atom + persisted sequence) and is the open piece. `rail.myPages`/`rail.newPage` i18n keys
  left in place (harmless). wasm green.

## 2026-06-16 — B4: consolidate the duplicate Settings

- There were two settings entry points: the household-card → global panel (the real editor) and a
  `/settings` screen (just a household summary + the debug-log viewer). Folded the debug-log viewer
  into the global panel (`globalSettingsForm`: last 25 ring entries, newest first, refresh button) and
  deleted `screens/settings.go`, the `/settings` route, and the Settings sidebar item. The household
  card is now the single way into settings.
- Cleanup: removed `/settings` from `modules.locked` (only `/` stays locked now) and repointed the
  modules tests' locked-path example to `/`. The dropped household-summary card was redundant with the
  panel's own currency/members editors, so not worth re-homing. Full `go test ./...` + wasm green.
- The `nav.settings` i18n key is now unused but left in place (harmless). This is B4.

## 2026-06-16 — B9: top-bar breadcrumb

- Replaced the plain title in `TopBar` with a `<nav aria-label="Breadcrumb">`: a Dashboard crumb
  (a real `<button>`, so it's keyboard-operable) + a `›` separator + the current screen title marked
  `aria-current="page"`. On the dashboard route only the title shows. The home crumb navigates via the
  existing `nav`; current route comes from `router.InspectCurrentRoute().Path`.
- The app's routes are flat (one screen per route), so the trail is at most two deep — no nested-route
  derivation needed. Reused existing utility classes (no new CSS); works in both themes. wasm green.

## 2026-06-16 — B5: collapsed-rail hover labels

- Added `Title(props.Label)` to `navItem` (and a title on the household card) — a native tooltip that
  doubles as the accessible name when the rail collapses to icons. Plus a CSS flyout: in
  `.rail.collapsed`, `.nv:hover/:focus-visible/:focus-within > span` overrides the `display:none` and
  positions the label as an absolute pill to the right (z-index above content, pointer-events none),
  with a reduced-motion-gated fade-in. Keyboard focus reveals it via `:focus-within`/`:focus-visible`.
- Specificity: the reveal selectors carry an extra pseudo-class over the base `.nv span { display:none }`
  so they win without `!important`. wasm green.

## 2026-06-16 — B6: display-scale setting

- Now green-lit ("do all the todos"), built B6 — the font/UI scale the user had earlier asked me to
  hold. Pure `prefs.Scale` (int percent, `ScaleMin/Max/Default` = 70/130/100, clamped in `Normalize`,
  `ScaleFraction()` → CSS multiplier; table-tested). `uistate.ApplyPrefs` sets `--ui-scale`;
  `index.html` adds `#app { zoom: var(--ui-scale, 1) }`; Settings → Appearance gets a "Display scale"
  `<select>` (70–130% in 10s, 100% marked default) that persists + applies live.
- Used CSS `zoom` (simple, scales the whole app cleanly incl. layout) over a font-size cascade. Fixed
  the pre-existing `TestNormalize` fixture (added a valid `Scale: 110` so the preserve-valid assertion
  accounts for the new field). prefs + i18n + wasm green.

## 2026-06-16 — saved formulas: Customize UI

- Added save/list/edit/delete to the Customize screen: a name input + Save below the calculator, and a
  "Saved formulas" card (`savedFormulasCard`) listing each formula via a `SavedFormulaRow` component
  (own handlers) showing name, expression, and its live-evaluated result, with Edit (loads back into
  the editor) and delete. `evalFormulaDisplay` evaluates each against the current vars.
- §2.5's remaining piece is now just surfacing enabled formulas' results on the dashboard (a widget) —
  the harder design call. The Customize save/list loop is complete. New `customize.*` keys; catalog +
  wasm green.

## 2026-06-16 — saved formulas: model + persistence

- User said "do all the todos" — scope opened up to the full backlog (gated UX items included). Picked
  up §2.5's `Formula{...}` + CRUD first: `domain.Formula{ID, Name, Expr, Enabled}` persisted like the
  other entities (formulas table, store CRUD, dataset round-trip, validated appstate accessors;
  table-tested). Kept it minimal (Target/ResultType/Format from the spec deferred — ResultType is
  computed at eval, the rest are display extras).
- Next on §2.5: a save/list/delete UI on the Customize screen and surfacing enabled formulas' results
  on the dashboard. all layers + wasm green.

## 2026-06-16 — nav: Tools group for the missing main-line screens (B7)

- Planning, Allocate, Insights, Documents, Customize were routed but absent from the sidebar — only
  reachable by URL. Added a `toolsNav()` group rendered as a "Tools" rail section (module-visibility
  filtered like primary nav), and four new SVG icons (planning/allocate/insights/customize; documents
  reuses `page`). New `nav.*` + `rail.tools` keys.
- Scoping note: this is the uncontroversial reachability half of the menu work (B7) — making routed
  screens reachable. The B8 redesign (shift-drag reorder, drop "My pages", per-item visibility
  settings) stays as analysis-only per the user's earlier note. Catalog + wasm green.

## 2026-06-16 — allocation profile picker + weight editor

- Allocate now drives the ranking from four editable weight inputs (returns/stability/liquidity/debt)
  instead of a fixed preset lookup. The profile select merges the built-in presets with saved profiles
  ("saved:<id>" keys); picking one loads its weights into the inputs (`resolveWeights`/`setWeights`).
  "Save profile" persists the current weights as a `domain.AllocationProfile`; a Delete appears for
  saved selections.
- Helpers `parseWeight` (blank/invalid/negative → 0) and `trimWeight` (no trailing zeros). New
  `allocate.weights*`/`saveProfile`/etc. i18n keys. This completes §2.7's AllocationProfile picker UI
  on top of the persisted model. Catalog + wasm green.

## 2026-06-16 — saved allocation profiles: model + persistence

- Added `domain.AllocationProfile` (name + four float weights) persisted like the other entities
  (allocprofiles table, store CRUD, dataset round-trip, validated appstate accessors). Table-tested.
- Kept domain a leaf: the weights are inline floats, not `allocate.Weights`, so domain doesn't import
  the scoring engine; the Allocate UI maps profile → `allocate.Weights` at the call site (it already
  builds Weights from its presets). Fits the "highly configurable" project value — custom profiles
  beyond the four presets.
- This is §2.7's `AllocationProfile + CRUD`. Next: a profile picker/manager on the Allocate screen
  (merge saved profiles with the presets). all layers + wasm green.

## 2026-06-16 — recurring autopost

- Added `appstate.PostDueRecurring(asOf)`: for each autopost recurring with an account, post a
  transaction (date = NextDue, amount/label/category from the recurring) and `Advance()` until NextDue
  is past asOf — a bounded catch-up loop (guard 600) handling multiple missed periods. Skips
  non-autopost or account-less ones. Returns the count. Table-tested (catch-up count, skip rules,
  idempotent re-run, posted-txn shape).
- Extended the Planning recurring form with account/category selects + an Auto-post toggle (so the
  model's fields are settable), and added a "Post due now" button reporting the count. Used
  `uiw.ToggleRow` for the toggle.
- This makes recurring genuinely functional — schedules turn into real ledger entries on demand. (A
  background/auto trigger on app load could come later; manual "post due now" is the safe explicit
  version.) New `recurring.*` keys; appstate + i18n + wasm green.

## 2026-06-16 — recurring monthly-equivalent total

- Added pure `domain.Recurring.MonthlyEquivalent()` (weekly ×52/12, quarterly ÷3, yearly ÷12, monthly
  as-is; integer truncation) and showed the summed net monthly equivalent in the Planning recurring
  card. Table-tested.
- Chose a display-only summary over wiring recurring into the 12-month forecast: the forecast's
  monthlyNet is derived from this month's actuals, so adding recurring there would double-count a
  salary/bill that already shows in actuals — a semantic decision worth leaving for explicit design.
  The monthly-equivalent total is unambiguous and useful on its own. domain + i18n + wasm green.

## 2026-06-16 — Recurring management UI on Planning

- Added a "Recurring cash flows" card to the Planning screen (no new route — avoids the gated nav
  redesign, and recurring flows belong with forecasting anyway): an add form (label, signed amount,
  cadence select) and a list rendered via a `RecurringRow` component (own delete handler), amounts
  colored by sign with cadence + next-due meta. `cadenceLabel` localizes the cadence.
- Validation mirrors the others (label + non-zero amount client-side; appstate enforces the rest).
  New `recurring.*` i18n keys. Catalog + wasm green.
- Gives the Recurring model a real producer/consumer. Next: feed these into the forecast
  (monthly-equivalent) and optional autoposting of due ones.

## 2026-06-16 — Recurring cash-flow model + persistence

- Added `domain.Recurring` (label, signed `money.Money` amount, `RecurringCadence`, NextDue,
  account/category, Autopost) with `Cadence.Next(from)` (weekly = +7d, monthly/quarterly/yearly via
  `dateutil.AddMonths`, unknown→monthly) and `Recurring.Advance()` (value receiver, returns a copy).
  Persisted like the other entities (recurring table, store CRUD, dataset round-trip, validated
  appstate accessors). Table-tested at domain/store/appstate layers.
- Modeling call: followed Transaction's convention — signed amount + currency carried in
  `money.Money`, no separate Kind/Currency enum from the spec's field list — so income/expense is the
  amount sign, consistent with the rest of the domain. domain now imports dateutil for cadence math
  (acyclic: dateutil is stdlib-only).
- This is §2.6's `Recurring{...}` + CRUD item. Next: a management UI and (later) autoposting due ones
  into real transactions; the forecast engine can also read these. wasm + all layers green.

## 2026-06-16 — forecast + payoff: edge-case tests

- forecast: added tests for one-times outside the horizon (month 0, beyond, negative → ignored),
  multiple one-times in the same month summing, negative horizon → nil, and balances going negative
  (reported truthfully, no flooring).
- payoff: added single-month clear (final payment capped → TotalPaid = principal + one month
  interest), payment-exactly-equals-interest is non-viable (the `payment <= interest` boundary),
  negative balance treated as already paid, and a TotalPaid = principal + interest invariant over a
  table of viable inputs.
- Completes §2.6 "Tests: forecast projection, payoff math"; both already had happy-path coverage, this
  pins the boundaries. No engine changes; both suites green.

## 2026-06-16 — allocate: determinism + clamping tests

- Filled the §2.7 "extensive tests" gap that mattered most: an explicit determinism test (Rank +
  Distribute identical across 25 runs — guards against map-iteration or unstable-sort creeping in),
  a tie-stability test (equal scores keep input order, since the UI shows a ranked list), and a
  breakdown-clamping test (negative APR → 0, stability > 100 → 1, negative liquidity → 0).
- The package was already well-covered for scoring/weights/constraints/Distribute; this pins the
  "deterministic & explainable" core guarantee specifically. No engine change. Full suite green.

## 2026-06-16 — formula engine: security + edge-case tests

- Added `eval_security_test.go` to nail down the §2.5 "extensive tests incl. security (no escape)"
  item: a blocklist of host/arbitrary function names (and wrong-case allow-list names) all error;
  undeclared variables error (no silent zero); evaluation only ever produces float64/string/bool (no
  host type can leak out); 300-deep nesting evaluates; repeated eval is deterministic; numeric edge
  cases (chained/parenthesized unary, negative modulo via math.Mod, float division, round-half-away);
  malformed inputs error rather than panic.
- No engine changes needed — the existing design already enforces the allow-list and scalar-only
  values; this pins those guarantees with tests so a future change can't regress the sandbox. Whole
  formula suite green.

## 2026-06-16 — rules: shadowed-rule detection

- Added pure `rules.Conflicts(rs)`: a rule is shadowed when an earlier rule's match phrase is a
  substring of its own (case-insensitive) — first-match-wins means the later one can never fire.
  Empty-phrase rules report ShadowedBy -1. Returns the first shadower per rule. Table-tested.
- Surfaced it on the Rules screen: each row carries a `Warning` (computed via `Conflicts` keyed by
  rule ID) shown as a `text-warn` meta line — "Never runs — an earlier rule (…) already matches it."
  This is the "conflict handling beyond first-match" piece of §2.4. Catalog + rules tests + wasm green.

## 2026-06-16 — pin/save insights: UI

- Added a Pin button beside "Save as task" on the Insights answer (saves the text via
  `app.PutSavedInsight` with a "Pinned." confirmation) and a "Pinned insights" card listing
  `app.SavedInsights()` newest-first, each a `PinnedInsightRow` component with a remove button. A local
  rev counter re-renders the list on pin/unpin.
- Completes §2.3's pin/save-insights item (storage + UI). New `insights.pin*`/`unpinTitle` keys;
  catalog + wasm green.

## 2026-06-16 — pin/save insights: model + persistence

- Added `domain.SavedInsight{ID, Text, CreatedAt}` and persisted it like the other entities
  (savedinsights table, store CRUD, dataset round-trip, validated appstate accessors requiring id +
  non-empty text). Table-tested at store, dataset, and appstate layers.
- Kept it a separate entity from `Task` deliberately: "Save as task" already exists for actionable
  items, but pinning an explanation to revisit shouldn't pollute the to-do list — different intent,
  different store. The UI (a Pin button on the Insights answer + a pinned-insights card) is next.

## 2026-06-16 — AI request cancellation

- `postCompletions` now creates an `AbortController`, passes its signal to fetch, and returns a cancel
  closure that aborts the controller, clears any pending retry `setTimeout`, and flips a `cancelled`
  flag the onText/onCatch callbacks check so a cancelled request reports nothing. The Send* funcs now
  return that cancel; Insights captures it in state and shows a Cancel button (with a disabled
  "Thinking…") while loading.
- One AbortController is created once and reused across retry attempts, so a single abort kills the
  whole retry chain. Build-error paths return `noopCancel` so callers can always call the result
  safely. Existing callers that ignore the return value (allocate, documents) compile unchanged — Go
  discards an unused return value.
- Completes §2.1 "retry/backoff; request cancellation". Can't browser-test the abort here, but it's the
  standard AbortController shape; ai + i18n tests + wasm green.

## 2026-06-16 — B1 triage: deep-link 404 is now framework-side only

- Worked the Section-B bug backlog starting at B1 (deep-link refresh 404). The CashFlux-side fixes
  were already landed in prior sessions and are marked done: the service worker serves the cached app
  shell for `navigate` requests (`web/sw.js`, cache `cashflux-v2`), and the GitHub Pages deploy
  generates a `404.html` shell — so the **deployed/offline PWA already routes deep links correctly**.
- Verified the current build: `GOOS=js GOARCH=wasm go build` is green and `go test ./...` passes.
- Empirically probed the dev server (`gwc dev -port 8099`): `/` → **404**, `/index.html` → **404**,
  `/accounts` → **404**, but `/bin/main.wasm` → **200**. So `gwc dev` isn't serving the HTML shell at
  *any* route — this is the §0 "`gwc dev -html` resolution" bug, and on top of it there's no SPA
  history fallback. Both gaps live in the **GoWebComponents framework** (the dev tool), not in this
  repo. Documented the dev-server caveat + workaround in the README hosting section.
- **Decision needed before B1 can fully close:** the only remaining buildable piece is the framework
  dev-server fix (serve `index.html` for the root + unknown non-asset history paths), which is a commit
  in the GoWebComponents sibling repo + a `.tools/gwc.exe` rebuild — a separate repo, so flagging it
  rather than assuming scope. Until then, browser end-to-end verification also waits on Playwright (§0,
  not installed). Next: B2 (dashboard reflow) is fully in-repo and unblocked.

## 2026-06-16 — model picker + AI-off-until-key hint

- Expanded the Settings → AI model `<select>` from two options to the six the cost table knows
  (gpt-4o-mini, gpt-4.1-nano/mini, gpt-4o, gpt-4.1, o4-mini), kept deliberately aligned with the
  pricing table so the new token-cost line stays accurate for whatever the user picks.
- Added an inline hint under the key input shown only when no key is set: "AI features stay off until
  you add a key…", restating the bring-your-own-key/local-first promise right where it matters. The
  enable toggle + empty-key gating already existed; this makes the off state explicit.
- Completes §2.1's model/cost item (cost surfacing + model picker + AI-off-until-key). i18n + wasm
  green.

## 2026-06-16 — surface AI token usage + cost in Insights

- Threaded `Usage` through the transport: `SendChat`/`SendVisionChat`/`postCompletions` now call
  `onResult(content, Usage)` (the body is parsed once for content, once via `ParseUsage`). Updated all
  four callers — allocate + documents ignore it with `func(c string, _ ai.Usage)`, Insights captures
  it.
- Insights renders a faint "Used N tokens · about $X" line under the answer, using `EstimateCostUSD`
  (falls back to tokens-only when the model's pricing is unknown). Usage state resets at the start of
  each explain/ask. New `insights.usageCost`/`insights.usageTokens` keys.
- This completes the "token + cost surfacing" half of §2.1's model/cost item; the model picker and the
  explicit "AI off until key set" state remain. ai + i18n tests + wasm green.

## 2026-06-16 — docs: static-hosting SPA rewrite

- Added a README "Hosting (SPA history fallback)" section: explains why clean (non-hash) routes need
  the host to rewrite unknown non-asset paths to index.html, and gives the snippet for GitHub Pages
  (404.html, auto), Netlify (_redirects), Vercel (rewrite), nginx (try_files), Caddy. Closes the
  prod-hosting layer of B1; the SW + 404.html layers were already done. Docs-only.

## 2026-06-16 — service-worker SPA navigation fallback

- The SW was network-first with `.catch(() => caches.match(req))` — but `.catch` only fires on a
  network *failure*, not a resolved 404. So a deep-link refresh at `/accounts` (which has no file on a
  static host) served the 404. Added a dedicated `req.mode === "navigate"` branch: on a non-ok
  response or a network error, serve the cached app shell (`appShell()` = index.html, then `./`), which
  boots and lets the router resolve the path.
- Successful navigations cache their document under the `./index.html` key (every SPA navigation is the
  shell), so the offline shell stays fresh. Bumped CACHE to v2 so the new logic + a clean cache
  activate on next load.
- This is the SW half of deep-link refresh (B1); the deploy workflow's generated 404.html covers the
  cold first load on GitHub Pages. JS-only; can't unit-test the SW here, but it's the standard SPA
  navigation-fallback shape.

## 2026-06-16 — AI guardrails: explicit FinancialContext

- Added pure `ai.FinancialContext{NetWorth, Income, Spending, Accounts}` + `Line()`, and routed both
  Insights prompts (explain + ask) through it. The guardrail is structural: the only thing that can go
  into the prompt's context is those four aggregate fields — there's no field for payees, account
  numbers, or per-transaction rows, so a future edit can't accidentally widen what's sent. The scope
  now lives in one reviewable, tested place instead of two inline Sprintf strings.
- Dropped the now-unused `fmt` import from insights.go. ai test (incl. a no-leak assertion) + wasm
  green. This is the §2.3 "guardrails + pure prompt-assembly tests" item.

## 2026-06-16 — documents import-history list UI

- Added an "Import history" card to the Documents screen: `app.Documents()` sorted newest-first,
  rendered as `DocHistoryRow` components (own delete handler) showing kind · date · status · row count
  · account, each removable via `app.DeleteDocument`. Empty state when there's nothing yet.
- Localized kind/status via `docKindLabel`/`docStatusLabel` helpers + new `documents.kind*`/`status*`
  keys; date stays a display-format literal (consistent with the app's other date examples).
- This closes the §2.2 Document lifecycle end-to-end: model → persistence → recorded on import →
  visible/auditable history. Catalog + wasm green.

## 2026-06-16 — record Documents on import

- Wired the Document model into the Documents screen: both import paths now call a `recordDocument`
  helper (declared before the handlers, since Go closures can't reference a later same-scope var) that
  PutDocuments a `DocImported` record when ≥1 transaction lands. Image imports carry the rows (mapped
  extract.Row → domain.DocumentRow via `toDocumentRows`); CSV records the metadata (the store parses
  CSV, so the rows aren't surfaced to the UI).
- Best-effort: the audit record's error is ignored (appstate logs it) so a history hiccup never blocks
  the actual import the user cares about.
- Now the §2.2 lifecycle has a real producer. Remaining: a documents history/audit list UI to view
  them (the records persist + export already). wasm green.

## 2026-06-16 — Document lifecycle model + persistence

- Added `domain.Document` (ID, Filename, Kind, UploadedAt, AccountID, MemberID, Status, Extracted[])
  with `DocumentKind` (csv/image), `DocumentStatus` (pending/extracted/imported/failed), and a
  `DocumentRow` for the reviewed lines. Persisted exactly like rules: a `documents` table, store CRUD,
  dataset export/import, and validated appstate accessors. Table-tested (store CRUD + status
  transition, dataset round-trip, appstate id-required + round-trip).
- Layering call: gave Document its own `DocumentRow` rather than importing `extract.Row`, keeping
  `domain` a pure leaf (nothing it imports knows about the parser). The Documents screen maps
  extract.Row ↔ domain.DocumentRow at the edge (same four string fields). Avoids a domain→extract
  dependency and the can't-define-methods-on-another-package's-type problem with `extract.Row`.
- This is the model/persistence half of §2.2's lifecycle item; recording a Document when the user
  imports (CSV or image) and a documents history list are the UI follow-ups. wasm green.

## 2026-06-16 — vision extraction via structured outputs

- Gave the structured-outputs codec a real consumer: `BuildStructuredVisionRequest` (extracted shared
  `visionMessages` so plain + structured vision builds don't duplicate the multimodal message) +
  `SendStructuredVisionChat`, and switched the Documents image read to send a strict `transactions`
  JSON schema. The system prompt no longer has to beg for "ONLY a JSON array, no code fence" — the
  schema enforces shape — though `extract.ParseRows` still parses the `{"transactions":[…]}` result
  (and remains the fallback for non-structured replies).
- Schema follows strict-mode rules (every property required, additionalProperties:false). Round-trip
  test asserts the image part survives alongside response_format. ai tests + wasm green.

## 2026-06-16 — AI structured-outputs codec

- Added `ai.BuildStructuredRequest(model, messages, temp, schemaName, schema)` + `ResponseFormat`/
  `JSONSchema` types: emits a chat request with `response_format: {type: json_schema, json_schema:
  {name, schema, strict: true}}`, so the reply is schema-constrained JSON decodable into a Go struct.
  Round-trip tested (response_format shape + schema preserved as RawMessage).
- Decision: pure codec only, consistent with how the rest of `ai` separates request/response shaping
  (pure, tested) from the js fetch transport. The obvious first consumer is document extraction
  (replace the free-form-JSON-then-`extract.ParseRows` path with a strict schema) — left as a
  follow-up so this stays a focused, low-risk building block. Scope note: I'm staying on spec-backed
  Phase 2 backend and avoiding the user's analysis-only UX items (animations, display scale, route/nav
  redesign, breadcrumb) per their "these are analysis and todo adding" / "stop implementing" feedback.
- ai tests + wasm green.

## 2026-06-16 — suggested-rules review UI

- Wired `rulesuggest.Suggest(transactions, existingRules, 3)` into the Rules screen as a "Suggested
  rules" card (above the rules list, hidden when empty). Each suggestion is a `SuggestionRow` component
  (owns its own Add handler, per the no-hooks-in-loops rule) showing "Categorize X as Y · Seen in N
  transactions" and an Add button that assigns an id and `PutRule`s it.
- After accepting, `bump()` re-renders: the new rule appears in the list and that suggestion drops off
  (Suggest skips keys an existing rule already covers) — a satisfying review-and-accept loop with no
  extra bookkeeping. New `rules.suggested*`/`rules.accept*` i18n keys. Catalog + wasm green.
- Completes the review-and-accept half of §2.4's "rules from history".

## 2026-06-16 — rulesuggest: deterministic rule suggestions

- New pure `internal/rulesuggest.Suggest(txns, existing, minCount)`: groups categorized non-transfer
  transactions by a normalized key (payee, or description when there's no payee), and where a key
  appears ≥ minCount times with ≥80% category agreement and isn't already covered by an existing rule,
  proposes `rules.Rule{Match, SetCategoryID}` with its support/total counts, sorted by support.
- Design call: did this as a deterministic heuristic rather than the backlog's literal "AI-proposed
  rules". It's free, instant, explainable (carries the evidence), and aligns with the project's
  determinism/explainability rule — strictly better than an AI round-trip for this. AI proposals could
  layer on later for fuzzier patterns. Lives in its own leaf package (imports domain + rules) so the
  rules engine stays domain-free.
- Edge handling tested: min-count gate, the 80% consistency gate (mixed key skipped), existing-rule
  skip, payee-vs-desc key, transfers/uncategorized ignored, sort by support. UI (a "suggested rules"
  list with Add buttons on the Rules screen) is the next step.

## 2026-06-16 — AI retry with backoff

- Added pure `ai.IsRetryable(status)` (429/5xx/0) and `ai.RetryDelayMS(attempt)` (500ms doubling, up
  to `MaxRetries`=3), table-tested. Rewrote `postCompletions` into a self-recursive `attempt(n)`: on a
  retryable HTTP status or a network reject it schedules `attempt(n+1)` via `setTimeout` after the
  backoff, releasing that attempt's js.Funcs first; otherwise it finalizes via onResult/onError.
- Care points: each attempt allocates its own onResp/onText/onCatch and releases them before
  retrying (so funcs don't leak across attempts), and the retry timer func releases itself. Client
  errors (400/401/404) short-circuit to onError — no point retrying a bad key.
- Request cancellation (the other half of the §2.1 line) is still open; it needs an AbortController
  wired to a caller-held cancel handle, which is a bigger surface. Retry/backoff stands alone. Pure
  tests + wasm green.

## 2026-06-16 — AI cost estimation (pure)

- Added `ai.EstimateCostUSD(model, usage)` over a small per-1M-token price table, plus `pricingFor`
  with longest-prefix matching so `gpt-4o-mini-2024-07-18` resolves to gpt-4o-mini (not gpt-4o — that
  ordering bug is exactly why I avoided map-iteration prefix matching). `FormatCostUSD` shows sub-cent
  costs to 4 decimals so a fraction of a cent is still visible. Table-tested.
- Scope: pure logic only this commit. Surfacing it in the UI ("this used ~N tokens, ~$0.000x") needs
  the response's `Usage` threaded back through the transport's `onResult` callback (currently it only
  passes the content string) — a transport-signature change touching Insights + Documents callers, so
  it's a separate follow-up rather than bundled here. The codec already has `ParseUsage`.

## 2026-06-16 — AI error handling: plain-English messages

- Added pure `ai.ErrorMessage(status, body)`: maps an OpenAI HTTP failure to an actionable message —
  401 (key rejected), 403 (no model access), 429 split into rate-limit vs. out-of-quota (by sniffing
  the error body), 404 (unknown model), 5xx (server trouble), 400 (shows OpenAI's own detail), with a
  status-named fallback. `apiErrorMessage` pulls `error.message` from the body via the existing
  `ChatResponse.Error`. Table-tested.
- Wired the fetch transport to capture `response.status` in the first `.then` and route any `>= 400`
  through `ErrorMessage` (previously a 401/429 body just failed `ParseResponse` with a generic line).
  Also rewrote the `catch` (network/CORS) message to "Couldn't reach OpenAI. Check your internet
  connection…" — friendlier than echoing the raw JS error.
- Both AI surfaces (Insights, Documents image import) already display the transport's `onError`
  string, so they inherit the better messages with no screen changes. wasm green.

## 2026-06-16 — retroactive Apply-to-existing for rules

- Added `appstate.ApplyRules() (int, error)`: walks transactions, and for each uncategorized,
  non-transfer one runs `rules.FirstMatch` over payee+desc, assigning the matched category (and tags
  when the txn has none). Leaves categorized transactions alone; returns the count updated. Uses only
  the user's saved rules (not the implicit category-name rules) — this is an explicit user action, so
  it should do exactly what the rules say.
- Wired an "Apply to existing" button into the Rules screen list card (shown only when rules exist),
  reporting the count via the toast. This is the clean answer to the CSV-import gap: rather than
  reaching into the pure store CSV parser, the user (or a future post-import hook) runs the rules over
  whatever's uncategorized.
- Table-tested at the appstate layer (match→update, already-categorized untouched, no-match left
  alone, tags applied). New `rules.applyExisting*`/`rules.applied*` i18n keys. wasm green.

## 2026-06-16 — apply rules on image import

- The image-review import loop now builds the same `autoRules` (user rules ++ implicit category-name
  rules) and uses it as a fallback: an imported row keeps its own category when the name matches one
  of yours; otherwise `rules.FirstMatch` against the description fills the category and tags. Explicit
  beats inferred, which is the right precedence for AI/vision-provided categories.
- Scope: the vision/AI image-import path (where UI code turns rows into transactions). The CSV-paste
  path goes through `app.ImportTransactionsCSV` → the pure `store` CSV parser, which has no rules
  dependency; routing that through rules would mean a store-layer or post-import pass — left as a
  separate follow-up. wasm green.

## 2026-06-16 — apply saved rules on transaction entry

- Wired the persisted rules into the add-transaction form: built `autoRules` = user rules (priority)
  ++ implicit category-name rules, and switched `onDesc` from `rules.Category` to `rules.FirstMatch`
  so a single matching rule can fill both the category and the tags. Guarded so it never overrides a
  category or tags the user already entered.
- Scope: manual entry only this commit. Import paths (CSV paste + vision review→import) match
  categories by name today; routing those through `autoRules` is the remaining §2.4 "apply on import"
  piece — kept separate to stay feature-granular. wasm green.

## 2026-06-16 — Rules management screen

- Added the `/rules` screen (Rules view) mirroring the Categories management pattern: an add form
  (match phrase, category select, optional comma-separated tags), a list of rules, inline edit via a
  per-row `RuleRow` component (hooks declared unconditionally so the edit toggle never reorders
  them), and delete. Registered the route in `screens.All()` and a nav entry in the shell's System
  group next to Categories.
- Validation: client-side `validateRuleInput` shows localized messages (match + category required)
  so the raw `appstate:` error never reaches the UI — same approach as Categories' `nameRequired`.
- Reused the existing `parseTags` helper from transactions.go (caught a redeclare at build — they
  were identical) rather than duplicating it. Added `rules.*` + `nav.rules` i18n keys.
- This is the management-UI half of §2.4. Remaining: apply rules automatically on entry/import (the
  engine + store are ready; just needs to be invoked at those write paths).

## 2026-06-16 — store/state: persist auto-categorization rules

- Added persistence for the existing `rules` engine (which had no storage): a `rules` table, store
  CRUD, dataset export/import inclusion, and validated `appstate` accessors. This is the bottom-up
  foundation for §2.4 — the management UI and apply-on-entry come next, on top of this.
- Architecture decision: persisted `rules.Rule` directly rather than introducing a `domain.Rule` +
  engine refactor. Precedent already exists — the store persists `customfields.Def` (also a
  non-domain type) the same way — so this stays consistent and fully additive (no churn to the
  engine or its one caller in transactions.go).
- Schema decision: added `Rules []rules.Rule json:"rules,omitempty"` to the Dataset without bumping
  SchemaVersion, matching how `CustomFields` was added — an additive optional field is backward
  compatible (old exports with no `rules` key import as empty), and the migrate() guard still rejects
  newer-than-supported data.
- Validation: `appstate.PutRule` requires id + non-empty match + category (done inline rather than in
  `validate`, which is oriented around domain types). Tests: store CRUD + upsert + delete, dataset
  round-trip now carries a rule, and appstate validation + round-trip. Full native `go test` + wasm
  green.

## 2026-06-16 — dashboard: top Spending highlight widget

- Added `topHighlightWidget` to the bento: shows the #1 spending anomaly this month as a one-line
  highlight (green/red ↑/↓ + plain-English sentence), or a calm "no big changes" message. Placed at
  a fresh grid row (1 / span 4, row 9) so its new widget ID can't collide with users' persisted
  layouts — and since the dashboard is reconfigurable, they can drag it wherever they like.
- Refactored to keep it DRY: extracted `detectSpendingAnomalies` (build series → Detect) and
  `highlightText/highlightTone/highlightArrow` as shared `screens`-package helpers, so the dashboard
  widget and the Insights card render identical wording/colors from one source. The Insights card now
  just calls these.
- New i18n keys `dashboard.highlight`/`dashboard.noHighlights`. Catalog test, full native `go test`,
  and wasm build all green. Closes the §2.3 "show top insight on dashboard" piece.

## 2026-06-16 — insights UI: offline Spending highlights card

- Wired the anomaly engine into the Insights screen: `spendingHighlights` builds the last four
  monthly boundaries, runs `ledger.CategorySpendSeries` → maps category IDs to names (uncategorized
  grouped under a localized label) → `insights.Detect` with `DefaultOptions`, and renders each
  anomaly as a plain-English row with a green/red ↑/↓ marker. Rendered first in the screen and
  needing no API key, so it's useful even with AI off; returns an empty `Fragment()` when nothing is
  notable so the card just doesn't show.
- Kept the row rendering loop hook-free (display-only spans, no `On*`), per the framework gotcha. New
  i18n keys (`insights.highlightsTitle/Hint/highlightUp/highlightDown/uncategorized`) and matching
  CSS (`.insight-list/.insight-row/.insight-dot`) reusing the existing `text-up`/`text-down` Tailwind
  colors. Catalog-quality test + wasm build green.
- This completes the §2.3 "trend/anomaly highlights" line end-to-end (engine + feeder + UI). The
  remaining §2.3 items (pin/save insights, top insight on dashboard, advice from AI) stay open.

## 2026-06-16 — ledger: per-category spend-series feeder for anomalies

- Added `ledger.CategorySpendSeries(all, bounds, rates)`: buckets non-transfer expense into the
  half-open periods `[bounds[i], bounds[i+1])` and returns `map[categoryID][]int64` of base-currency
  spend per period (oldest first, positive magnitudes, zeros where idle). This is the bridge between
  raw transactions and `insights.Detect` — the UI maps the result to `[]insights.CategorySeries` with
  display names.
- Decoupling decision: the feeder lives in `ledger` (which already owns FX-aware aggregation) and
  returns a plain map rather than `insights.CategorySeries`, so `insights` stays a pure math leaf
  with zero domain/currency dependencies. The trivial map→CategorySeries mapping happens at the UI
  edge that already imports both.
- Table-tested: multi-period bucketing, same-period accumulation, FX conversion, and exclusion of
  income/transfers/out-of-window/uncategorized-key handling; plus the <2-bounds empty case.

## 2026-06-16 — insights: pure spending anomaly/trend engine

- New `internal/insights` package: `Detect(series, opts)` takes per-category spend histories
  (oldest→newest, positive minor-unit magnitudes), computes a baseline = mean of the prior periods,
  and flags categories whose current period deviates from that baseline by ≥ a percent threshold.
  Each `Anomaly` carries Current/Baseline/Delta/PctChange/Direction so the UI can explain *why* it
  surfaced — no black box, per the determinism rule.
- Design choices: baseline excludes the current period (compares "this month" to "how it normally
  behaves"); a `MinBaseline` noise floor avoids the $0.50→$1.00 = "+100%" trap; zero/sub-floor
  baselines are skipped so the percentage is always meaningful (a from-zero "new category" highlight
  is a deliberate future addition, not folded in here). Results sort by absolute delta, then name.
  `DefaultOptions` = 2 baseline periods, 1000-minor-unit floor, 50% threshold.
- Built bottom-up and pure-first (no `syscall/js`), mirroring `rules`/`payoff`/`allocate`: 7-case
  table test + option-normalization + helper tests, all green on native. UI wiring (Insights cards /
  dashboard top-insight) is a separate later feature.

## 2026-06-16 — README: badges, live-demo link, License section

- The README already covered what/stack/build/architecture/docs; this fills the §0 gaps: a badge row
  (MIT / Go 1.26+ / WebAssembly / live demo via shields.io), a **Live demo** callout to the GitHub
  Pages URL with the "starts empty, Load sample, local-storage" caveat, and a **License** section
  linking `LICENSE`. That also closes the MIT item's "note the license in README" follow-up.
- Skipped screenshots/GIF for now — capturing them needs a browser session and image assets; the live
  demo link is the better first-impression substitute until those are produced deliberately.

## 2026-06-16 — MIT licensing: LICENSE file + SPDX convention

- Added a top-level `LICENSE` (standard MIT text, 2026, copyright holder `monstercameron` — the
  verifiable repo-owner identity, chosen over guessing a legal name). Established the per-file
  convention as a single `// SPDX-License-Identifier: MIT` line rather than full headers, matching
  idiomatic Go and the TODOS §0 note.
- Placement decision: in `main.go` the SPDX line goes *above* the `//go:build js && wasm` constraint
  (build constraints may be preceded by line comments). Verified the wasm build still succeeds and
  the native toolchain still excludes the file — so the constraint survived the insertion.
- Scope decision: did **not** sweep SPDX across all ~150 files in this commit. Many carry build tags
  where header placement is fragile, and a tree-wide sweep is a mechanical change better done
  deliberately; the entrypoint marker plus the LICENSE file is the substantive licensing act. The
  README "License" section + badge ships with the README feature (LICENSE has to exist first, which
  it now does). TODOS §0 MIT item left open, narrowed to those follow-ups.

## 2026-06-16 — i18n: CI catalog-quality guard

- Added `TestDefaultCatalogQuality` to `internal/i18n`: asserts the seeded English catalog is
  non-empty, every key is dot-namespaced with no embedded/surrounding whitespace, and every key
  maps to a non-empty string. Since `lookup` treats an empty value as missing, a blank English entry
  would silently render the raw key in the UI — this catches that at `go test` time (ci.yml runs
  `go test ./...`).
- Scoped deliberately: values are NOT checked for trimming or format-verb validity. Several catalog
  strings legitimately carry leading/trailing spaces (suffix fragments like " · by %s") or a literal
  `%` ("APR %", "...cut spending 10%?"), and format strings are only Sprintf'd when args are passed —
  so a blanket fmt-validity assertion would false-fail. Key shape + non-empty value are the
  invariants that are always true.
- This is the optional "CI completeness test" noted as the last i18n item; the central language
  store (catalog + T + export/import + selector) is now fully done and guarded.

## 2026-06-16 — Backlog: MIT licensing setup (§0)

- Logged a Phase 0 project-setup item to put CashFlux under the MIT license: a top-level `LICENSE`
  file (standard MIT text, current year + copyright holder), license references where the repo
  convention calls for them, and a README "License" section + badge.
- Decided on light SPDX references (`// SPDX-License-Identifier: MIT`) over a full per-file header —
  idiomatic for Go and keeps source files clean. Placed alongside the README item in §0 since it's
  foundational repo setup. Docs-only change; no code/tests affected.

## 2026-06-16 — i18n: language selector — central language store COMPLETE

- Added a **Display language** `<select>` to Settings → Languages, listing `uistate.Languages()`
  (every language the bundle carries) with the active one preselected. Picking one calls
  `uistate.SetActiveLanguage`, which persists the code to `localStorage` (`cashflux:active-lang`) and
  reloads the page.
- Reworked `uistate/i18n.go`: dropped the unused `UseLang` atom (T is non-reactive by design, so a
  reload is the clean way to re-resolve every rendered string at once) and added boot-loaded
  `activeLang` (`loadActiveLang`), `ActiveLanguage`, `SetActiveLanguage`, and `Languages`. `T` now
  resolves against `activeLang`, falling back to English then the key for anything untranslated.
- Decision: reload-to-apply over a reactive re-render. `uistate.T` is deliberately hook-free so it's
  safe inside loops/row components, which means it can't observe a live language atom; a reload is
  simpler and guaranteed-correct vs. threading the active language through every render edge.
- Helper `langDisplay` labels languages (English by name, other codes uppercased) until localized
  names ship. New key `settings.language`. `go test ./internal/i18n` + wasm build green.
- **Milestone:** the central-language-store loop is closed — pick, export, import. What's left is
  optional polish: a CI catalog-completeness test (via `MissingKeys`) and localized language names.

## 2026-06-16 — i18n: TransactionRow — verbiage migration COMPLETE

- Migrated `TransactionRow` (inline edit form, category/transfer/uncategorized labels, cleared
  status + meta, and all row action buttons/titles) and the transfer-description default. Reused the
  transactions.* keys already in the catalog + action.*.
- **Milestone:** every screen and shared component now renders user-facing text through `uistate.T`
  against the English catalog — the "hook up all the verbiage" request is fulfilled. Intentional
  literals remain: `humanizeType` account-type names, currency/AI-model display names, date-format
  example option text, and the OpenAI prompt instructions (sent to the model, not the user).
- **What's left for full localization** (not English-text changes): a language **selector** in
  Settings so imported languages actually display (TODOS §1.19), then optionally a CI catalog-
  completeness test. `i18n` tests + wasm green.

## 2026-06-16 — i18n: Transactions screen (main function)

- Migrated the `Transactions()` function: add form (kind options reuse category.expense/income +
  transactions.transfer; account/category/transfer pickers + placeholders), all filter controls
  (search, account/category/member, date range, cleared status, sort) + their option labels, the
  bulk-action bar, validation + paired-transfer/bulk error notices, empty states, and the shown-count
  summary — all via `uistate.T`.
- Kept the `kind`/`f.Sort`/`f.Cleared` *values* literal (they're internal identifiers compared in
  code); only the option *labels* are localized. The `label = "Transfer"` stored-desc default stays
  literal (persisted data).
- `i18n` tests + wasm green. **Last chunk: `TransactionRow`** (inline edit + per-row buttons), then the
  entire UI verbiage is migrated.

## 2026-06-16 — i18n: AccountRow (Accounts screen complete)

- Migrated the `AccountRow` component: inline edit form (reusing the accounts.* field keys + common.name/
  owner + action.save/cancel), the update-balance prompt (`%s (%s)` via T args), the stale badge, the
  cleared-balance suffix, and all row action buttons + titles (view→nav.transactions, update balance,
  mark updated, edit, archive/restore, delete). Accounts is now fully localized.
- Left `humanizeType` (a generic enum title-caser) as-is — localizing account-type display names is a
  separate domain-enum task. `i18n` tests + wasm green.
- **Last remaining UI verbiage: the Transactions screen** (the other giant).

## 2026-06-16 — i18n: Accounts screen (main function)

- Migrated the `Accounts()` function (the big add form with all asset/liability sub-fields, the welcome
  card, net-worth/assets/liabilities stats, mark-all-updated notice + button, section headers + empty
  states, the balance-adjustment txn desc, and the invalid-balance error) onto `uistate.T`. Reused
  `common.name`, `owner.group`, `dashboard.netWorth/liabilities`.
- Account *type* labels still come from the `humanizeType` helper (shared, used elsewhere) — left for a
  helper-level pass. The `AccountRow` component (inline edit, per-row actions) is the next chunk.
- `i18n` tests + wasm green. Split Accounts across two cycles to bound the diff.

## 2026-06-16 — i18n: settings panel right column (panel complete)

- Migrated the right column of `app/settings.go`'s global panel: AI section (title/enable/key
  placeholder), Appearance (theme seg Dark/Light/System, Accent, Compact), Preferences (week-start
  seg Sunday/Monday), Date format title, Data action buttons, and the Languages buttons — all via
  `uistate.T`. Currency/model display names and the date-format example option text stay literal.
- The global settings panel is now fully localized. `i18n` tests + wasm green.
- Remaining UI verbiage: the two large screens **Accounts** and **Transactions**.

## 2026-06-16 — i18n: settings panel (left column + chrome)

- Migrated the first half of `app/settings.go`'s global panel: SettingsHost panel title, the
  widgetSettingsForm no-settings placeholder, and the left column (household members, base currency,
  exchange rates, screens + hint, freshness + hint). Converted the `freshnessTypes` table from
  hardcoded `Label` to an i18n `Key` (like cfEntities), resolved at render.
- Kept the currency option display names ("USD — US Dollar") literal (registry territory). Split the
  panel across two cycles to keep edits bounded — the right column (AI/appearance/prefs/data/languages)
  is next.
- `i18n` tests + wasm green.

## 2026-06-16 — i18n: custom-fields components migrated

- Migrated the shared custom-fields UI onto i18n: `customfields.go` (CustomFieldsManager) and
  `customfieldform.go` (CustomFieldInput). Converted the package-level `cfEntities`/`cfTypes` tables
  from hardcoded `Label` to an i18n `Key` resolved at render (entities reuse `nav.*`; types get
  `cf.type*`), so `cfTypeLabel` and the section headers localize too. Added the form/list strings,
  the required suffix/label, and Yes/No. Added uistate imports to both files.
- `i18n` tests + wasm green. Remaining UI verbiage: the big `app/settings.go` global panel, and the
  two large screens Accounts + Transactions.

## 2026-06-16 — i18n: Settings screen migrated

- Twelfth screen onto i18n: `screens/settings.go` — household summary (reusing `nav.members/accounts/
  categories` for the row labels) + the debug-log viewer (title, refresh, empty state). Added uistate
  import; `fmt` stays for the count values.
- `i18n` tests + wasm green. Remaining UI verbiage: the big `app/settings.go` global panel, the
  Accounts + Transactions screens, and the CustomFieldsManager/CustomFieldInput components.

## 2026-06-16 — i18n: Dashboard chrome migrated

- Eleventh screen onto i18n: `dashboard.go` — every widget title (reusing `nav.*` for Accounts/
  Budgets/Goals/To-do), the header cell (title/hint/Reset), the freshness widget (all-fresh, stale
  count via T args, Remind), the savings sub-line + "this period", and the KPI assets/accounts
  sublines. The nudge task title is localized too.
- Left a couple of dynamic KPI sublines (`periodLabel + plural(...)`) literal — they concat a
  date-label with the English `plural()` helper, so cleanly localizing them is its own task (plural
  rules). The cashflow bar heights and the freshness "· %dd" chip stay `fmt` (numeric). `fmt` remains.
- `i18n` tests + wasm green. 11 screens + chrome. Remaining: Accounts, Transactions (the giants),
  Settings, and the CustomFieldsManager/CustomFieldInput components.

## 2026-06-16 — i18n: Allocate screen migrated

- Tenth screen onto i18n: `allocate.go` — profile picker + amount/reserve inputs, ranked rows (the
  breakdown via `T` args, exclude/restore), candidate name prefixes ("Pay down %s", "Goal · %s"),
  empty states, kept-back note, and the AI-explanation card. Kept the numeric `%.0f%%` score
  formatting and the AI prompt builder (`fmt.Fprintf`) literal, so `fmt` stays.
- `i18n` tests + wasm green. 10 screens + chrome. Remaining: Accounts, Transactions, Dashboard,
  Settings (+ CustomFieldsManager/CustomFieldInput).

## 2026-06-16 — i18n: Documents screen migrated

- Ninth screen onto i18n: `documents.go` — vision-import + CSV-import cards, the review/edit draft
  list, and all status/error messages (several via `T` args with `plural(...)`). Kept the vision
  *system prompt* (model instruction) and the CSV-format example placeholder literal.
- All `fmt.Sprintf`s became `T(key, args…)`, so the `fmt` import was dropped. `i18n` tests + wasm green.
- 9 screens + chrome. Remaining: Accounts, Transactions, Dashboard, Allocate, Settings (+ the
  CustomFieldsManager/CustomFieldInput shared components).

## 2026-06-16 — i18n: Customize screen migrated

- Eighth screen onto i18n: `customize.go` — formula-calculator title/desc, expression placeholder, the
  example chips ("Savings rate %", etc. — `%` is literal since T only Sprintf's with args), and the
  Result/Available-variables sections. The formula example *expressions* set on click stay literal
  (they're code), as do the `true`/`false` result values. Added uistate import; `fmt` stays for the
  `%v` value fallback.
- `CustomFieldsManager` (rendered here, defined elsewhere) still has its own strings — a later screen.
- `i18n` tests + wasm green. 8 screens + chrome. Remaining: Accounts, Transactions, Dashboard,
  Documents, Allocate, Settings (+ CustomFieldsManager + CustomFieldInput components).

## 2026-06-16 — i18n: Planning screen migrated

- Seventh screen onto i18n: `planning.go` — debt-payoff calculator (inputs, stat labels, hint/invalid/
  too-low messages, the extra-payment note via T args), and the 12-month forecast card (title, the
  cash-flow projection hint, trim-spending placeholder + note) all via `uistate.T`. Added uistate import.
- The `%d` months stat value stays as `fmt.Sprintf` (a number, not text), so `fmt` remains. `i18n`
  tests + wasm green.
- 7 screens + chrome migrated. Remaining: Accounts, Transactions (the giants), Dashboard, Documents,
  Allocate, Customize, Settings.

## 2026-06-16 — i18n: Insights screen migrated

- Sixth screen onto i18n: `insights.go` UI strings (explain/ask cards, the key hint, placeholders,
  answer + save-as-task, status messages). Added the missing `uistate` import.
- **Decision:** left the OpenAI system/user prompt strings in English — they're instructions sent to
  the model, not user-facing text, so they shouldn't be translated (and the `fmt.Sprintf` prompt
  builders stay). Only the visible chrome is localized.
- Sequencing: did Insights (moderate) rather than the 330-line Accounts to keep the cycle's token cost
  reasonable; Accounts/Transactions (the two giants) remain, plus Dashboard/Documents/Allocate/
  Planning/Customize/Settings. `i18n` tests + wasm green.

## 2026-06-16 — i18n: Budgets screen migrated (+ shared owner picker)

- Fifth screen onto i18n: `budgets.go` — add form (incl. `Limit (%s)` via T args), period picker, month
  stepper titles, spent/budgeted/left stats, the over/near summary, and budget rows (on-track/near/over
  labels, the `%s · %s · %d%% · %s left` sub via T args, edit/delete).
- Also localized the **shared** `ownerSelectOptions` helper ("Group (shared)" → `owner.group`), which
  the budgets add-form now uses (replaced its inline duplicate) and which goals' edit row also calls —
  so two screens' owner pickers are covered at once. Added shared `common.owner`.
- Period option labels still come from `domain.Period.Label()` (enum-level), left as-is. `fmt` stays
  for the CSS bar width. `i18n` tests + wasm green.
- **Next:** Accounts (the largest) or Transactions screen.

## 2026-06-16 — i18n: Goals screen migrated

- Fourth screen onto i18n: `goals.go` — add form (incl. `Target (%s)` placeholder via T args), owner +
  linked-account pickers, combined-progress stats, and the progress sub-line assembled from T
  fragments (`progressFmt`, `complete`, `bySuffix`, `saveSuffix`, `linkedSuffix`) plus the contribute
  prompt and row actions. Reused `common.name`, `owner.group`, `nav.goals`, `action.*`.
- The CSS bar-width `fmt.Sprintf` and the `%d%%` stat value stay as fmt (not user-facing text), so the
  fmt import remains. `i18n` tests + wasm green.
- **Next:** Budgets or Accounts screen.

## 2026-06-16 — i18n: Categories screen migrated

- Third screen onto i18n: `categories.go` fully migrated — add form, kind + parent pickers (incl. the
  inline-edit ones), reassign-before-delete panel (templated description via `T` args), income/expense
  list cards + empty states, and row edit/delete + the kind meta (replaced the `humanizeType` call with
  `category.income`/`category.expense`).
- Introduced shared keys to curb sprawl: `common.name`, `common.reassignTitle`, `common.moveAndDelete`,
  and `category.expense`/`category.income`. (Members still has its own reassign/name keys — a tiny
  later convergence; not worth churn now.)
- Dropped the unused `fmt` import; added the missing `uistate` import. `i18n` tests + wasm green.
- **Next:** Goals or Budgets screen. 5 screens migrated after this (shell, todo, members, categories;
  + chrome).

## 2026-06-16 — i18n: Members screen migrated

- Second screen onto i18n: `members.go` fully migrated — add form, the reassign-before-delete panel
  (incl. the `%q owns %d…` description via `T(key, args…)`), member rows (make-default, view
  transactions, edit, delete, default badge, role meta), net-worth-by-member, and validation messages.
- Reused shared keys: `common.notReady`, `action.save/cancel/edit`, `nav.transactions` (row button),
  and added `owner.group` ("Group (shared)") which the owner pickers elsewhere can adopt next.
- Dropped the now-unused `fmt` import (the only `fmt.Sprintf` became a `T(...)` call).
- `i18n` tests + wasm green. **Next:** Categories or Goals screen.

## 2026-06-16 — i18n: To-do screen migrated (first full screen)

- First full screen onto the language store: every user-facing string in `todo.go` now resolves via
  `uistate.T` — add-form title/placeholders, priority options (both the add form and inline edit),
  empty/all-done states, the hide-done toggle, the validation message, and the row actions
  (toggle/edit/delete titles, due prefix). `priorityMeta` returns translated labels.
- Added shared keys (`priority.high/medium/low`, `common.notReady`) so the other screens reuse them
  rather than re-adding. Catalog grew accordingly; English values match the old literals → no visible
  change.
- `i18n` tests + wasm green. **Next:** migrate the next screen (Members or Categories), reusing the
  shared keys; track coverage as the catalog grows.

## 2026-06-16 — Extract credit-utilization into ledger

- Moved the inline `owed*100/limit` from `accountMeta` into pure `ledger.Utilization(balance, limit)`
  (balance magnitude, ok=false for non-positive limit). Accounts liability rows delegate to it.
- Table-tested: no/negative limit not-ok, negative & positive balance magnitudes, zero owed, over-limit.
  `internal/ledger` + wasm green.
- Continues the "no math in view code" + trust theme; the ledger package now owns SavingsRate,
  PercentChange, and Utilization alongside the balance/net-worth functions.

## 2026-06-16 — Top-bar verbiage migrated to i18n (chrome complete)

- Migrated the top bar's chrome onto `uistate.T`: menu-toggle tooltip (`topbar.menu`), "+ Add" label
  (`topbar.addLabel`) and its tooltip (`topbar.add`). With the sidebar already done, the app shell's
  verbiage is fully on the language store.
- The page title itself still comes from `screens.All()` data (per-route Title/Subtitle); i18n-ing
  those needs per-route title keys — left for the screen-verbiage migration pass.
- `i18n` tests + wasm green. **Next:** begin per-screen verbiage migration, or pivot to another area.

## 2026-06-16 — Language bundle export/import in Settings

- Made the language store's round-trip user-accessible (the user's "easy to export/import all langs"
  ask): `uistate.ExportLanguages()`/`ImportLanguages()` wrap the bundle's JSON codec; import merges and
  persists to localStorage (`cashflux:languages`), and `loadBundle()` seeds those on boot. Added
  Settings → Languages "Export languages" / "Import languages" buttons, wired through the toast.
- Note: with T still non-reactive (English-only display), an imported language is stored/ready but not
  shown until the language selector lands — the notice says "reload to apply" and the selector is the
  remaining §1.19 piece. Export gives translators the English source + any existing langs.
- wasm build green; pure `i18n` codec already tested. **Next:** the language selector (then imported
  languages actually display), or continue migrating screen verbiage.

## 2026-06-16 — i18n live wiring + sidebar verbiage migrated

- Wired the language store into the UI: `uistate` holds a shared `i18n.DefaultBundle()`, a `UseLang`
  atom (default English), and a hook-free `T(key, args…)` helper (resolves against the bundle default).
  T deliberately takes no hook so it's safe in loops / row components (the nav maps over items).
- Migrated the first screen's chrome onto it: shell brand, primary + System nav labels, the
  "My pages"/"System"/"New page" headers, and the household card now call `uistate.T(...)`. All keys
  already existed in the English catalog and match the old strings, so zero visible change.
- **Design note:** kept T non-reactive for now (English-only). When a language selector lands (TODOS
  §1.19), the active lang gets threaded at render edges (read `UseLang` at a component top) rather than
  inside T — preserving the no-hooks-in-loops rule.
- wasm build green. **Next:** migrate more screens' verbiage onto T incrementally, or the next feature.

## 2026-06-16 — Extract bill next-due date into dateutil

- Moved `nextDue` out of the js-only dashboard into pure `dateutil.NextMonthlyDue(now, day)` — the
  next occurrence of a monthly due-day on/after today, day clamped to 1–28 so it's valid every month
  (incl. February). The upcoming-bills widget calls it.
- Table-tested the fiddly cases: later-this-month, on-the-day, already-passed→next-month, >28 clamp,
  February clamp, non-positive→1. `internal/dateutil` + wasm green.
- (Fixed a wrong expected value while writing the test — day=0 clamps to the 1st, which is already
  past the 10th, so it rolls to next month.)

## 2026-06-16 — Extract savings-rate calc into ledger

- Moved the dashboard's inline `(income-expense)*100/income` into pure `ledger.SavingsRate` (0 when
  income ≤ 0, negative when overspent, truncates toward zero). Same "no math in view code" + trust
  pattern as the earlier `PercentChange` extraction; the savings widget now calls it.
- Table-tested (no-income, negative-income, saved/overspent, spent-nothing, truncation). `internal/ledger`
  + wasm green.
- (Minor: the in-place edit briefly scrambled the SavingsRate/PercentChange doc comments; fixed.)

## 2026-06-16 — Default category scheme (§1.10, pure)

- Added `internal/catscheme.Default()` — a starter set of income/expense categories plus a few
  sub-categories (Housing → Rent/Utilities, Transportation → Fuel/Public transit). Returns ID-less
  `Item`s with `Parent` named so the persistence layer assigns IDs and resolves parents.
- Table-tested: both kinds present, unique names, parents resolve to a same-kind top-level item, hex
  colors. `internal/catscheme` green.
- Bottom-up piece of §1.10 "Default scheme + reset": the pure scheme now exists; the "reset
  categories" action (apply it via appstate, replacing/merging) and methodology presets (envelope/
  zero-based) remain.

## 2026-06-16 — Backlog: Electron desktop app as a post-core item (§5.1)

- Added a new `TODOS.md` §5 "Future / nice-to-have (post-core)" tier and logged **standalone
  desktop app via Electron** as its first item. Placed at the very bottom of the priority-ordered
  file, after Phase 3 / sync and the continuous Cross-cutting section, to mark it explicitly as
  lower-priority and post-core — not part of the spec.
- Scoped it as a thin wrapper that reuses the *exact* production `web/` build (wasm bundle +
  `wasm_exec.js` + `sw.js` + manifest) as the renderer, so there's no second UI codebase and the
  wasm stays the single source of truth. Sub-tasks cover wrapper choice (Electron vs. Tauri/Wails —
  to be decided), scaffold, window chrome, per-OS packaging, a CI artifact job, and verification.
- Docs-only change; no code or tests affected.

## 2026-06-16 — Currency display helper (§1.2)

- Closed the §1.2 "format a Money in a target/base currency" checklist item: added
  `Rates.FormatAccounting(m, toCurrency)` and `Rates.FormatInBase(m)` to `internal/currency` —
  convert via the rate table, then `money.FormatAccounting` with the target's symbol/decimals.
  Lives in currency (which already imports money) so the money package stays registry-free.
- Table-tested: same-currency, negative-parenthesized, cross-currency conversion (€→$ and $→€),
  missing-rate error. `internal/currency` green.
- Small decision-free increment between the bigger parked items; screens can adopt it for multi-
  currency display.

## 2026-06-16 — B10 foundation: pure period presets

- Shifted off the widget-settings sweep to the B10 resolution-control redesign, starting with the
  decision-free pure layer: `period.Previous`, `period.YearToDate`, plus `Window.Shift` (move the
  whole window as a unit — distinct from the edge-only StepFrom/StepTo) and `Window.IsCurrent` (flag
  when the view has paged off "now"). All take an explicit `now` so they're pure; table-tested.
- Dropped `LastNDays` from the plan: the Window model is unit-based (week/month/quarter), so an
  arbitrary N-day range doesn't fit it cleanly — noted for the UI step (those would need a different
  representation). ThisPeriod stays `NewWindow`.
- `internal/period` green + wasm build green. The UI redesign (single stepper + presets dropdown)
  still carries the keep-range-vs-drop decision, so it stays parked until the user calls it; these
  constructors are ready for whichever way it goes.

## 2026-06-16 — B12: Spending breakdown top-N setting

- Fourth widget on the settings API: "breakdown" schema (`topN`, 2–6, default 3); the widget reads it
  and groups the rest as "Other" (generalized the hardcoded top-3). Tones cycle so >4 segments are fine.
- widgetcfg tests + wasm green. Dashboard widgets with persisted settings now: savings, recent, trend,
  breakdown. **Next:** likely wrap up the widget-settings sweep and pick another backlog item.

## 2026-06-16 — B12: Net worth trend months setting

- Third widget on the settings API: "trend" schema (`months`, 3–12, default 6); `netWorthTrendWidget`
  reads it and generalizes the cutoff window (offset `i-(months-2)`, preserving the prior 6-month
  shape that ends one month ahead). Persisted via `WidgetConfigs`.
- widgetcfg tests + wasm green. **Next:** spending-breakdown top-N, or move on to another backlog item.

## 2026-06-16 — B12: Recent transactions row-count setting

- Second widget on the settings API: registered a "recent" schema (`count`, number 3–20, default 6)
  and had `recentWidget` read it (`widgetCfgs.For("recent")`) to size the list instead of the
  hardcoded 6. Same pattern as savings — schema in `widgetcfg`, consumption in the screen, persisted
  via the `WidgetConfigs` atom.
- widgetcfg tests + wasm build green.
- **Next:** keep extending — net-worth-trend range (months), spending-breakdown top-N, etc.

## 2026-06-16 — B12: savings widget consumes its settings

- Closed the loop on the per-widget settings example: `savingsRateWidget` now reads its persisted
  config (target rate %, show-bar). Dashboard reads `uistate.UseWidgetConfigs()` at the top (stable
  hook position) and passes `widgetCfgs.For("savings")` down — cleaner than a hook inside the helper.
- Behavior: tone now reflects performance vs target — at/above target green, positive-but-short amber
  (`text-warn`), negative red; the subline shows the target; the bar hides when `showBar` is off.
- End-to-end demoable: gear → set target/hide bar → persists across reload → widget reflects it.
- wasm build green. **Next:** register feasible schemas for more widgets (recent count, trend range,
  budgets scope) so "any feasible settings exposed and persisted" extends across the dashboard.

## 2026-06-16 — B12 wiring: schema-driven, persisted widget settings panel

- Loop back in implement mode; resumed B12 (the widget-settings wiring paused earlier). Picked it as
  the highest-value self-contained item with no blocking decision and no external deps.
- Re-added `uistate.WidgetConfigs` (localStorage-backed atom; `For` + copy-on-write `WithField`), and
  rewrote `app.widgetSettingsForm` to be schema-driven: it looks up `widgetcfg.SchemaFor(id)` (ID now
  threaded from `SettingsHost`) and renders each field via a dedicated `widgetFieldRow` component
  (toggle→ToggleRow, number→numeric input, select→Select) bound to the persisted config; placeholder
  for widgets without a schema. The old fake title/behavior toggles are gone.
- `widgetFieldRow` is its own component so each input's hook stays at a stable position (On*-in-loops
  rule); the row's branch is fixed per field type, so hook order is stable.
- Savings rate now shows real, persisted settings (target rate %, show-bar). **Next (next cooldown):**
  the savings widget *consumes* those values (compare actual vs target, optional bar), then register
  schemas for more widgets.
- wasm build green; native suite clean; `widgetcfg` unit tests already cover the accessors.

## 2026-06-16 — Pages deploy workflow (built); E2E test stories logged (B16)

- User: build a CI workflow that redeploys the build on every push so they can review from anywhere,
  and (separately) log an extensive E2E testing program.
- **Built `deploy-pages.yml`:** on push to main, set up Go, build `web/bin/main.wasm` (GOOS=js), copy
  `wasm_exec.js` from GOROOT (`lib/wasm` with `misc/wasm` fallback), generate `404.html` from
  index.html for deep-link routing, upload `web/` as a Pages artifact, deploy via `deploy-pages@v4`.
  - **Chose Actions-deploy over committing a `/docs` folder.** Same live URL
    (monstercameron.github.io/CashFlux), but no build artifacts committed and no push/commit loop.
    Updated the §0 hosting item to reflect this; the only manual step is setting Pages Source =
    "GitHub Actions" once (will try via `gh api`).
- **B16 — E2E stories:** logged a trustworthy-app testing program — dozens of scripted user-journey
  stories (canonical: add a transaction) asserting both UX (smooth standard path) and correctness
  (data/state/derived figures), covering every feature + cross-cutting (reload/offline/routing/a11y).
  Needs the Playwright/Chromium browser lane (§0, not installed) to run; authored/queued until then.
- Planning for B16; the deploy workflow is the one build action this turn (explicitly requested).

## 2026-06-16 — App-wide accessibility spike + program (B15)

- User: think deeply about app-wide accessibility and log it as a spike (it's extensive). Added B15.
- Framed it as a **spike first** (axe/keyboard/SR audit → catalogue framework a11y primitives → decide
  reusable patterns → output a prioritized plan), then a deep area checklist that becomes the tasks:
  semantics/landmarks, keyboard (flagging the pointer-only bento drag/resize as a real gap needing a
  keyboard alternative), dialog focus-trap for FlipPanel, correct ARIA for the custom controls
  (Segmented/ToggleRow/icon-buttons that rely on `title` today), focus-visible, live regions, color-
  not-the-only-cue, AA contrast (text-faint/accent suspect), reduced-motion, 200% zoom/reflow (ties to
  B6's px-heavy concern), forms, SPA route focus/title (ties B3/B9), chart alt-text (ties B14), touch
  targets, i18n'd labels, and CI axe once Playwright is in (§0).
- Pointed the §1.20 one-line a11y item at B15 (subsumed). Planning-only.

## 2026-06-16 — Designed Lucide (B13) + D3 (B14) integrations (planning)

- User: integrate Lucide (glyphs) and D3 (charts) with strong Go interfaces. Per planning mode, logged
  the designs as B13/B14 rather than building.
- **B13 Lucide:** the existing `ui.Icon(name string)` already emits 24×24 stroked currentColor SVGs —
  Lucide's exact format — so it's a clean swap. Proposed a type-safe `internal/icon` (`Name` constants
  → embedded Lucide path data, compile-checked), with a build-time generator to pull the curated set.
  Flagged embed-at-build (recommended, offline/vdom-safe) vs. CDN createIcons (fragile).
- **B14 D3:** proposed a pure, typed `chartspec.Spec` (Kind/Series/Axis + Validate + extent helpers)
  with `ui.Chart(spec)` driving D3 through a ref/portal + UseEffect (since D3 mutates the DOM and the
  framework owns a vdom), D3 pinned via CDN and SW-cached for offline. Flagged the real decision: adopt
  the heavy D3 dep vs. keep growing the working pure-Go SVG charts — needs the user's call before build.
- Both interfaces sketched in TODOS so the "strong Go interfaces" ask is answered at design level.

## 2026-06-16 — Switch pushes to gh; pause widget-settings wiring → B12

- User: use the `gh` CLI for pushing (more reliable), and go back to TODO-planning mode.
- **Push fix:** background `git push` was failing with "could not read Username for github.com" — the
  Windows/bare-git credential prompt pops a GUI dialog that can't complete in a non-interactive shell.
  Ran `gh auth setup-git` (gh is logged in as monstercameron, repo scope) so git uses gh's token
  non-interactively; pushes now succeed. Updated the CLAUDE.md push rule accordingly.
- **Back to planning:** the per-widget settings API foundation (`internal/widgetcfg`) is committed and
  tested, but I'd started the UI wiring (a `uistate.WidgetConfigs` atom). Per the user's "just todo
  planning", removed that uncommitted file and logged the remaining wiring as **B12** (persisted atom +
  schema-driven `widgetSettingsForm` + savings consumption + more widget schemas). The committed pure
  package stays — it's a standalone tested foundation with no consumers yet, which is fine.

## 2026-06-16 — Per-widget settings API (widgetcfg) — step 1

- User wants the per-widget flip panel wired to each widget's own settings (savings rate → savings
  settings), persisted. Building it bottom-up.
- Step 1: pure `internal/widgetcfg` — `Field` (toggle/number/select + default/bounds/options),
  `Schema` per widget, `Config` (key→string values) with typed accessors (`Str`/`Bool`/`Int` with
  default fallback + clamp + select validation), and a registry (`SchemaFor`/`Has`/`IDs`). Savings
  rate registers the first schema (target rate %, show-bar toggle). Table-tested.
- Next: a persisted `uistate` widget-configs atom + a schema-driven `widgetSettingsForm`, then the
  savings widget consuming its target setting.

## 2026-06-16 — README + Pages-hosting TODOs; background pushes

- Logged two §0 items: a proper **README.md**, and **hosting the app on GitHub Pages from `/docs`**
  (production build committed to `docs/`, relative paths for the `/CashFlux/` subpath, a `404.html`
  shell for deep-link routing — the static-host side of B1 — and a build script so `/docs` is
  regenerated, not hand-copied). Ticked the now-stale "create repo + push" §0 item.
- Per user: run `git push` in the **background** (fire-and-forget) so a credential/elevation prompt or
  UI button can't hang the non-interactive shell. Updated the CLAUDE.md push rule accordingly.

## 2026-06-16 — Logged "+ Add" flip-panel of add actions (B11)

- User wants "+ Add" to open a centered flip panel (settings-style) with add options: transaction,
  bills to scan, docs to scan, custom workflows, etc. Logged B11.
- Reuse note: the flip animation already exists as `ui.FlipPanel` via the `UseSettings` atom +
  `SettingsHost`; add an "add" target kind and an `addMenu` back face rather than a parallel overlay.
- Flagged the one scope question: what "custom workflows" maps to (existing Customize/formula vs. a
  new concept). Analysis/TODO only.

## 2026-06-16 — Deep-analyzed the time-resolution control (B10)

- User asked for a deep analysis to drastically improve the resolution-control UX. Logged B10.
- Core finding: the dual From/To stepper makes a *range* the default when ~90% of users want a single
  period — and it reads "Jun 2026 – Jun 2026" (looks broken) in that common case. Also no presets, no
  "back to now" reset, no off-current indicator, and it's wide (will crowd +Add and the B9 breadcrumb).
- Proposed: a single period stepper as the primary, a presets dropdown (This/Last month, This quarter,
  YTD, Last 30 days, Custom range…), a "this period"/Today reset, and the existing From/To range tucked
  behind "Custom range". Bottom-up: add pure `period` preset constructors (`ThisPeriod`/`Previous`/
  `YearToDate`/`LastNDays`/`IsCurrent`) + a single-period helper, table-tested, before the UI rebuild.
- One decision teed up: keep range power behind Custom range (recommended) vs. drop ranges entirely.
- Analysis/TODO only.

## 2026-06-16 — Logged top-bar breadcrumb (B9)

- User wants a clickable breadcrumb on the right of the top-level panel for stepping backwards.
  Logged as B9. Flagged the real design decision: routing is flat (no nesting), so the trail needs a
  defined meaning — home-rooted `Dashboard / {page}` (recommended, no history needed), a visited-
  history trail, or a logical hierarchy once drill-downs carry context. Build once the trail behavior
  is confirmed.
- Analysis/TODO only (consistent with the user's recent review-and-queue mode).

## 2026-06-16 — Logged sidebar menu management (B8)

- User (analysis/TODO-only): menu items should Shift+drag-reorder, the "My pages" segment should be
  removed (custom pages integrate into the page), and there should be settings for which menu items
  are visible. Logged as B8.
- Noted overlaps: the Shift-gating mirrors the dashboard resize-handle pattern (B-section) and the
  reorder can reuse the `dashlayout` ordered-sequence/`Move` model; the visibility piece already has a
  base in `internal/modules` + `hideableScreens` + Settings toggles, so B8 is mostly extending that to
  cover all nav items (incl. B7's additions) and exposing it as menu management.
- No code this turn (per the user's instruction).

## 2026-06-16 — Polished boot loader + app settle-in animation

- Replaced the plain "Loading CashFlux…" boot text with an on-brand animated loader: a spinning
  accent stroke-arc ring around the Fraunces "C" mark (gentle breathing), with the wordmark + subtle
  subline fading up. Pure HTML/CSS in `web/index.html` (no Go change), so it paints instantly before
  the wasm finishes.
- Added an app settle-in: the MutationObserver that reveals `#app` now adds an `app-enter` class that
  runs a calm fade + slight lift/scale (cubic-bezier ease-out); `#boot` fades and scales away. Both
  gated behind `prefers-reduced-motion`.
- **Next.** (User has queued more menu requests — logging those separately.)

## 2026-06-16 — Logged UI-scale (B6) and missing-menu-items (B7); reverted a premature impl

- User asked for a font/UI-scale setting and to update the menu with all main-line features, then
  clarified they want these **logged as TODOs, not implemented** right now. I had started building the
  scale feature (prefs field + ApplyPrefs + CSS + settings control) — reverted those uncommitted
  changes back to HEAD and captured both as backlog items instead.
- **B6 (UI scale):** design is px-heavy, so a rem scale won't catch buttons — logged a `--ui-scale`
  zoom on `#app` driven by a `prefs.Scale` percent (70–130). Analysis kept for when it's picked up.
- **B7 (menu gaps):** confirmed via `screens.All()` that Planning, Allocate, Insights, Documents, and
  Customize are routed but absent from the rail (URL-only). Logged adding them as a nav group + module
  toggles, and deriving nav from the routed set so screens can't silently miss the menu again.
- **Process note:** when the user is rapidly reporting issues/requests, default to logging them as
  TODOs unless they explicitly say implement.

## 2026-06-16 — Localization foundation: central language store (i18n)

- User wants all page verbiage localizable via a central store with easy export/import of all langs
  (English-only for now). Confirmed the one consequential fork first (asked): **dot-namespaced keys**
  (`nav.accounts`) over English-source-as-key — stable keys, copy edits don't orphan translations,
  cleaner export files.
- Built the bottom-up foundation: pure `internal/i18n` — `Bundle`/`Catalog`, `T(lang, key, args…)`
  with an en→key fallback chain and `fmt.Sprintf` formatting, `Set`, `Languages` (default first),
  `MissingKeys` (coverage gap), and `ExportJSON`/`ImportJSON` of the whole multi-language bundle
  (the translator round-trip the user asked for). `DefaultBundle()` seeds the English source catalog,
  starting with the shell/nav. Table-tested (fallback chain, empty-as-missing, arg formatting,
  missing-keys, export/import round-trip, merge/overwrite, bad-JSON, languages ordering).
- This is the store only — no UI wiring yet. "Hook up all verbiage" is a large per-screen migration,
  so it's tracked as a multi-commit task (see TODOS §1.19): active-language atom + localStorage of
  imported langs + a language selector (English-only now) + converting each screen's strings to `T`.
- Verified `internal/i18n` green.
- **Next.** Wire the live bundle + active-language atom and a `t()` helper, then migrate verbiage
  screen by screen (start with the shell/nav already seeded).

## 2026-06-16 — Logged settings-duplication (B4) and collapsed-rail-hover (B5)

- User reported two more items (analysis-only, add to TODOS):
- **B4 settings duplication:** confirmed two settings surfaces. The `/settings` nav screen
  (`screens.Settings()`) is just a read-only household summary + the debug log; all real editing lives
  in the household-card global flip panel (`globalSettingsForm`). So the menu "Settings" item is the
  emptier duplicate. Logged fix: make the household-card panel the single primary surface — move the
  debug log viewer into it, then remove the `/settings` route/screen (or repoint the item to open the
  panel), updating the locked-screens/module-visibility references.
- **B5 collapsed-rail hover:** the rail already collapses to icon-only (`.collapsed`); the missing
  piece is a hover/focus flyout revealing each item's label. Logged: add `title` attrs + a CSS flyout
  pill in collapsed mode (overlay, don't widen the rail), reduced-motion + keyboard-focus aware.
- Both analysis-only per the user's instruction; no code changes for them this turn.

## 2026-06-16 — B2 step 1: pure dashboard packing engine

- Started the B2 dashboard-grid rewrite bottom-up: added `internal/dashlayout/pack.go` — an
  ordered-sequence model (`Item{ID,ColSpan,RowSpan}`) plus `Pack(items, cols)` (first-fit, row-major,
  span-aware, no overlap, deterministic, 1-based output), `Move(items, id, toIndex)` (the reorder a
  drag produces), and `ResizeItem`. `DefaultItems()` reproduces the current bento when packed at 4
  cols (verified by test, modulo the +1 header row offset the view will apply).
- Kept the legacy `Placement`/`Swap`/`Resize` API intact — this commit is the engine only, fully
  table-tested (default reproduction, mixed-span no-overlap + clamping, first-fit gap backfill, Move
  reorder/clamp/unknown-noop, ResizeItem clamp, no-mutation). UI migration + FLIP animation are
  separate follow-up commits so each stays green.
- **Design note:** chose first-fit *with* gap backfill (CSS auto-flow "dense" semantics) — deterministic
  and space-efficient; order still drives placement. Can revisit if strict order-preservation feels
  better once it's on screen.
- **Next.** Migrate dashboard state/UI onto Items+Pack (replace Swap-on-drop with Move+re-pack, persist
  the order, offset rows for the header), then layer the FLIP reorder/resize animations.

## 2026-06-16 — Dashboard: Shift-to-reveal resize handles (+ animation reqs → B2)

- User asked for three dashboard-editing refinements: resize handles only while holding Shift,
  animated size scaling, and smooth animated reorder.
- Shipped the standalone one now: `.rz` handles are hidden by default and revealed only while Shift
  is held. A global keydown/keyup listener (`internal/app/resizereveal.go`, wired once in `Run`)
  toggles `data-resize` on the document root; CSS fades `.rz` in/out off that attribute. A window
  `blur` handler clears it so the handles can't get stuck visible if focus is lost mid-hold. The
  callbacks live for the app's lifetime, so they're intentionally not released.
- The two animation requirements are entangled with the B2 reflow-engine rewrite (CSS-grid placement
  changes don't transition natively — they need a FLIP technique over the packed layout), so I folded
  them into B2 rather than bolt on a half-animation that the rewrite would throw away. B2 now lists
  "animate reorder" and "animate resize" explicitly.
- Verified the wasm build.
- **Next.** B2 proper (ordered-sequence packing model + FLIP animation) when picked up.

## 2026-06-16 — Diagnosed the page-duplication-on-route bug (B3)

- User reported the page sometimes duplicating on navigation and asked me to scan the live DOM with
  gwc. Couldn't: `gwc probe` reports `playwright unavailable: install the driver` and the gwc MCP
  server isn't connected this session. Diagnosed from the framework router source instead — which is
  definitive for this bug.
- **Cause:** GoWebComponents' router is a *nested-layout* router. `expandPathPrefixes("/accounts")` →
  `["/", "/accounts"]`, so `resolveRouteStack` produces a route stack `[exact "/", exact "/accounts"]`
  and renders `/` as the parent layout wrapping `/accounts` via `router.GetOutlet()`. But `app.go`
  registers every route — including `/` — as a full `Shell`, and no Shell calls `GetOutlet()`. Result:
  non-root navigation renders the `/` Dashboard Shell **and** the target Shell = duplicated page. The
  `*` route is innocent — `Register("*", …)` is the router's not-found factory, not a stacking pattern.
- **Fix (logged as B3):** adopt the framework's layout+outlet structure — `/` becomes a chrome-only
  layout with `GetOutlet()`, screens become child routes rendering just their content, Dashboard
  becomes an index child. Real refactor of `app.go` + `Shell` + screen registration; deferred.
- This also interacts with B1: once `/` is a proper layout, the deep-link refresh fix still needs the
  SW/server history fallback, but the in-app routing will at least be structurally correct.

## 2026-06-16 — Logged two bugs (deep-link 404, dashboard drag) to the backlog

- User reported two bugs; analyzed both and added a high-priority **§B Bug fixes** section to TODOS.
- **B1 — deep-link 404:** not a router bug. `NewHistoryRouter` gives clean pushState URLs and its `*`
  fallback only runs after the wasm boots; a hard refresh at `/accounts` hits the server first, which
  404s before `index.html` loads. `sw.js` only cache-falls-back on a thrown error (not a non-ok
  response) and doesn't cache `/accounts`. Fix is layered: SW navigation fallback to the cached shell
  + a server SPA history rewrite (ties into the existing `gwc dev -html` item). Keep clean paths — no
  hash router.
- **B2 — dashboard drag:** `ui.Widget`'s drop calls `dashlayout.Swap`, a pairwise exchange of absolute
  Col/Row + spans, so nothing reflows, there's no live displacement, and span-swaps overlap neighbors.
  Logged the real fix: re-model `dashlayout` as an ordered sequence + pure size-aware `Pack`
  (bin-packing), `Move(id, toIndex)` + re-pack for drag, persisted/migrated, with pointer-based live
  reflow in the UI — iOS-home-screen behavior that respects multi-cell tiles.
- Analysis only this turn (user asked to add to TODOS, not implement). Both are bottom-up: B2 starts
  with the pure packing model + table tests before any UI.
- **Next.** Implement when picked up — B1 is the smaller/safer (SW + serve config); B2 is a real
  layout-engine rewrite (pure model + tests first).

## 2026-06-16 — Extract to-do ordering into a tested package

- Knocked off part of TODOS §1.14 "Tests: ordering, status transitions": the list ordering was a
  `sort.Slice` inline in the js-only `todo.go`, untestable. Moved it to pure `internal/tasksort`
  (`Order` returns a sorted copy; `Visible` applies the hide-done filter), both non-mutating, with
  table tests covering open-before-done, dated-before-undated, due ascending, title tie-break, the
  no-mutation guarantee, and the hide-done filter.
- Mirrors the earlier `txnfilter` extraction — same pattern of pulling a core behavior out from
  behind the wasm build tag so it gets native table tests. The screen lost its `sort` import.
- Verified `internal/tasksort` green + wasm build.
- **Next.** More inline-logic extraction / small polish (parked items await user input).

## 2026-06-16 — Extract (and fix) the net-worth percent-change calc

- The dashboard KPI computed `(net - prev) * 100 / prev` inline — both a "no computation in view
  code" violation and a latent bug: dividing by the *signed* baseline flips the arrow when net worth
  is negative (−1000 → −500 is a +50% improvement but rendered as ▼50%). Liability-heavy households
  hit this.
- Added pure, table-tested `ledger.PercentChange(curr, prev) (pct, ok)`: `ok=false` for a zero
  baseline, and division by `|prev|` so the sign tracks the real direction (cases cover increase,
  decrease, negative-baseline improving/worsening, crossing zero, and toward-zero truncation). The
  dashboard now calls it and only renders the delta when `ok`.
- **Why ledger and not a UI helper?** It's a money-derivation, same family as `NetWorth`/`PeriodTotals`
  — keeping it there means it's covered by the package's table tests and reusable by other KPIs.
- Verified `internal/ledger` green and the wasm build.
- **Next.** Keep extracting inline view computations into tested helpers, or other small polish.

## 2026-06-16 — Toast confirmations for data actions

- Made the toast dual-purpose (it already supported a non-error/info style via `Notice.Err=false`):
  the Settings data actions — Export JSON/CSV, Import, Load sample, Wipe — now post a success message,
  and the errors they previously swallowed (`if err != nil { return }`) now surface as error toasts.
  "Mark all updated" on Accounts reports the count it refreshed via `plural(n, "balance")`.
- **Plumbing note:** these are package-level funcs (`exportJSON`, …), not components, so they can't
  call the `UseNotice` hook. The `state` package exposes no non-hook atom setter (only `UseAtom`), so I
  threaded a `notify func(string, bool)` closure captured in `globalSettingsForm` (where the hook is
  valid) down into each. Clean and keeps the hook rules intact.
- Scope check: this is the §1.4 error-surface item, not feature-inference — export/import/wipe
  failing silently was a real gap; the success confirmations are the natural complement.
- Verified the wasm build; touched files are js-only (no native target).
- **Next.** Small polish; comprehensive feature set otherwise (layered config + sync remain
  deliberately out of scope pending spec agreement / backend).

## 2026-06-16 — Route remaining swallowed writes through the toast

- Followed up the toast surface by sweeping the last `_ = app.Put…` sites in the screens
  (`grep _ = app\.(Put|Delete|…)`): Accounts' "Mark all updated" bulk balance refresh, and the
  dashboard freshness nudge's "Remind me" task creation. Both now surface a friendly toast on failure.
- The nudge additionally **gates navigation on success** — previously it jumped to /todo regardless,
  so a failed `PutTask` left the user staring at a list missing the task they just "created." Now it
  stays put and explains.
- Screens-wide grep is clean of swallowed entity writes after this. Verified the wasm build; the
  touched files are js-only so there's no native target (native suite remains green from before).
- **Next.** Small polish; the local-first feature set is comprehensive (sync stays out of scope).

## 2026-06-16 — App-wide toast surface for silent failures

- Picked up TODOS §1.4 "Error/toast surface for failed persistence." Several bulk paths in the
  ledger screen swallowed errors with `_ = app.Put…` (bulk recategorize, bulk mark cleared/uncleared,
  and the paired-transfer delete) — so a failed write left the UI looking successful.
- Added `uistate.Notice` (a tiny `{Seq, Text, Err}` atom; `Seq` bumps per post so identical text
  still re-fires, `With`/`Cleared` helpers) and `app.Toast`, a single bottom-center toast mounted in
  the Shell. It auto-dismisses via `UseEffect` keyed on `Seq` (a `setTimeout` whose cleanup clears the
  timer and releases the `js.Func` exactly once — `Cleared()` preserves `Seq` so the fire doesn't
  re-trigger the effect). Wired the three swallowed sites to post friendly errors.
- **Why an atom + global component rather than per-screen error state?** Bulk/delete actions often
  have no visible error slot (unlike the add form's `errMsg`), and the surface should be reusable by
  any screen. One shared atom keeps it DRY and lets future call sites opt in with a one-liner.
- **Trade-off:** kept the add form's inline `errMsg` (validation feedback belongs next to the form);
  the toast is for incidental/background failures. Auto-dismiss timeout is a fixed 4.5s for now.
- Verified the wasm build and native suite (toast/notice are js-only, so no native target — expected).
- **Next.** Route more swallowed `_ = app.Put…` sites (other screens' bulk/delete) through the toast,
  or further small polish.

## 2026-06-16 — Dashboard week boundaries honor the week-start preference

- Closed the nit flagged in the previous entry: `defaultWindow` hardcoded `time.Monday`, so the
  dashboard's Week resolution ignored a Sunday week-start preference. Now it seeds from
  `loadPrefs().WeekStartWeekday()`.
- Added a pure, tested `period.Window.WithWeekStart(weekday)` mutator (re-snaps both anchors under
  the new convention; no-op when unchanged; leaves Month/Quarter anchors alone). Settings' `savePrefs`
  reconciles the period atom through it whenever the week-start changes, so the dashboard updates live
  — consistent with how the date-format pref already updates lists live.
- **Why a mutator in the pure package rather than rebuilding the window in view code?** Date/anchor
  math belongs in `internal/period`, not the shell; this keeps the screen a thin caller and gives the
  behavior table tests (week re-snap Mon→Sun, no-op, month-untouched).
- Verified: `internal/period` green, wasm build green, full native suite clean.
- **Next.** Further small polish; the local-first feature set is comprehensive (sync remains the only
  out-of-scope major item — needs a hosted backend).

## 2026-06-16 — Dashboard resolution persists across reloads

- The top-bar Week/Month/Quarter toggle now survives reloads. `uistate.PersistResolution` stores the
  chosen `period.Resolution` in localStorage; `defaultWindow()` seeds `UsePeriod` from it via
  `loadResolution()` and re-anchors to `time.Now()`. The `ResolutionControl`'s `OnSelect` persists
  before setting the atom.
- **Why persist only the resolution, not the whole window?** The From/To anchors are transient
  navigation — restoring last session's anchored week/month would dump the user on a stale period.
  Remembering just the granularity keeps their preference (e.g. "I think in quarters") while always
  landing on the current period. Stepping the pills stays in-memory by design.
- Pre-existing nit noted for later: `defaultWindow` still hardcodes `time.Monday` for week-start
  rather than reading the prefs week-start atom, so the dashboard week resolution may not match the
  user's configured first-day-of-week. Left as a separate, orthogonal fix (one feature per commit).
- Verified the wasm build and the native suite (`internal/period` green); `uistate` is js-only so it
  has no native target, as expected.
- **Next.** Reconcile the dashboard window's week-start with the prefs atom, or further small polish.

## 2026-06-16 — Refactor: transaction filtering → pure tested package

- Moved the ledger's filter+sort out of the js-only `transactions.go` (untestable) into pure
  `internal/txnfilter`: `Criteria` (the persisted shape), `Apply` (filter + sort, non-mutating), and
  `AbsAmount`. `uistate.TxFilter` is now a type alias for `txnfilter.Criteria`, so the localStorage
  atom and JSON are unchanged; the screen calls `txnfilter.Apply`.
- **Why:** filtering is core behavior (account/category/member/text/date/cleared + three sorts) that
  had zero tests because it lived behind the wasm build tag. Now it's table-tested (8 cases incl.
  tag-text match, date range, each sort, and a no-mutation check) per the standards.
- Kept the alias so nothing downstream changed type-wise; verified the full native suite plus the
  wasm build. The explicit `go test ./internal/uistate` "setup failed" is just that js-only package
  having no native build target — `./...` skips it cleanly.
- **Next.** Genuine small polish or further testability extraction; the feature set is comprehensive.

## 2026-06-16 — Budgets: period summary header

- Added a stat-grid above the budgets list: total spent, total budgeted (sum of each status's
  spent+remaining), and amount left for the viewed period. Folded the totals into the existing
  over/near counting loop so it's a single pass. "Left" tone follows its sign via `accentFor`.
- Parallels the goals summary; both give a one-line "where do I stand" without scanning every row.
- **Next.** Genuine small polish; the local feature set is comprehensive (sync needs a backend).

## 2026-06-16 — Goals: combined progress header

- Added a stat-grid above the goals list (when there are goals): total saved, total target, and
  overall progress % — the at-a-glance "how am I doing across everything" the per-goal bars don't
  give. Amounts sum directly since goals are stored in the base currency; percent clamps at 100.
- Reused the shared `stat` cell used on the accounts net-worth header for visual consistency.
- **Next.** Genuine small polish; the local feature set is comprehensive (sync needs a backend).

## 2026-06-16 — Transactions: filtered summary line

- Added a "N shown · net $X" line above the ledger list: the count of the filtered set plus its net
  total, each transaction converted to the base currency via the FX rates (skipping any that fail to
  convert). Recomputes from `shown` each render, so it tracks the filter (account, category, member,
  date, cleared) live.
- **Net, not income/expense.** It's the raw sum of shown amounts — for an account or category filter
  that's the meaningful figure; transfers (rare in a filtered view) net out within an account anyway.
- Pairs naturally with the existing "Export CSV" of the same filtered set — see the total, then
  export it.
- **Next.** Genuine small polish only; the local feature set is comprehensive (sync needs a backend).

## 2026-06-16 — Docs: status refresh

- The CLAUDE.md "Status" bullet had drifted badly — it still listed custom-field defs, document
  vision AI, and reload-persistent prefs as *remaining*, all of which shipped many commits ago.
  Rewrote it to reflect reality: Phases 1–2 essentially complete, every entity add/edit/delete, the
  full pure-package roster (now ~21 packages), and the headline features per screen. Multi-device
  sync is called out as the only major remaining item (needs a hosted backend, out of scope here).
- Keeps the new-session quick-reference accurate so a future session doesn't re-implement done work
  or mis-scope what's left. TODOS.md remains the granular checklist.
- **Next.** Genuine small polish only; the local feature set is comprehensive.

## 2026-06-16 — Members → ledger drill-down

- Added a "Transactions" button to each member row that sets the persisted `TxFilter.Member` and
  navigates to `/transactions` — same parent-owned-closure + `OnView` pattern as the account
  drill-down (router + filter atom in `Members`, `MemberRow` stays import-light).
- Drill-down now exists from both accounts and members into the filtered ledger; consistent
  navigation across the app.
- **Next.** Genuine small polish; sync is the only large remaining item and needs a backend.

## 2026-06-16 — Accounts: update-balance reconcile

- Added "Update balance" to account rows: `promptText` asks for the real balance, then `setBalance`
  posts a cleared "Balance adjustment" transaction for `target − currentBalance` and stamps
  `BalanceAsOf`. So the computed balance matches a statement in one step, with the gap recorded as a
  real (cleared) transaction rather than silently overwritten — keeps the ledger honest.
- Marked the adjustment `Cleared` since it's a reconciliation entry. Zero-delta is a no-op (just
  marks checked). Reused the existing balance prop for the current figure and `ledger.Balance`
  semantics implicitly via the displayed balance.
- Satisfies the §1.9 "Update balance → adjustment txn + set BalanceAsOf" backlog item.
- **Next.** Genuine small polish; sync is the only large remaining item and needs a backend.

## 2026-06-16 — Freshness nudge → to-do (create-from-nudge)

- The dashboard freshness widget's stale state now has a "Remind me" button that creates a
  Nudge-sourced task ("Update stale account balances") and navigates to /todo for confirmation. The
  handler is a `ui.UseEvent` created at the top of `Dashboard` (stable hook) and threaded into the
  widget as a `ui.Handler` param — keeping the widget a plain function while keeping the hook order
  safe.
- **Gotcha:** `ui.UseEvent` returns `ui.Handler`, not `func()`; the widget param had to be typed
  `ui.Handler` for `OnClick` to accept it.
- Both create-from hooks are now done: AI insight → task (Insights) and freshness nudge → task
  (dashboard). The `SourceNudge`/`SourceAI` task sources both have real producers.
- **Next.** Genuine small polish; multi-device sync is the only large remaining item and needs a
  backend.

## 2026-06-16 — Insights → to-do (create-from-insight)

- The Answer card now has "Save as task": it creates a `domain.Task` from the AI result (rune-safe
  80-char title, full text in notes, `Source: SourceAI`, medium priority, open) via `PutTask`, with
  an inline "Saved to your to-do list." confirmation that clears when a new answer is requested.
- Wires the §1.14 "create-from-insight" backlog hook — AI advice becomes an actionable, tracked
  to-do instead of disappearing when you leave the screen. The AI `Source` tag (long defined) now has
  a real producer.
- **Next.** Genuine small polish; the only large remaining item (multi-device sync) needs a backend.

## 2026-06-16 — Accounts → ledger drill-down

- Each account row gets a "Transactions" button that sets the persisted `TxFilter` to that account
  (normalized, other filters cleared) and navigates to `/transactions` — a quick way to see one
  account's activity. The transactions screen already renders from the filter atom, so it just works
  on arrival.
- Did it with a parent-owned `viewTransactions` closure (router + filter atom live in `Accounts`),
  passed as `OnView`, keeping `AccountRow` from importing router/uistate directly.
- A near-equivalent of the "per-account ledger view" backlog item, reusing the existing filtered
  ledger instead of a separate screen (running-balance series could still be a future dedicated view).
- **Next.** Genuine small polish; sync is the only large item left and needs a backend.

## 2026-06-16 — Accounts: Mark all updated

- Added a bulk "Mark all updated (N stale)" button (shown only when something's stale) that stamps
  `BalanceAsOf = now` on every stale, non-archived account via `PutAccount` — clearing all stale
  badges and the dashboard freshness nudge in one click, instead of per-row "Mark updated".
- Reused `freshness.IsStale` + `FreshnessWindows` for both the visible count and the action, so the
  button's label and effect agree.
- **Next.** Genuine small polish as it arises; sync remains out of scope without a backend.

## 2026-06-16 — Accounts: edit lock-until inline (lock-until complete)

- Added the "Locked until" date to the account inline editor's asset branch (seeded from
  `LockUntil`; a blank value clears it, unlocking the account). `saveEdit` parses it into `cp`.
- Lock-until is now fully manageable: set on add, change/clear on edit, and it gates allocation
  suggestions. Existing accounts can be locked (e.g. when you open a CD) or unlocked when it matures.
- **Next.** Genuine small polish; major remaining backlog (sync) needs a backend.

## 2026-06-16 — Allocation: honor account lock-until

- Put the long-unused `Account.LockUntil` to work. The Allocate screen now skips an asset account
  whose `LockUntil` is in the future when building candidates — you can't put new money into a locked
  account (a CD, a vesting lot), so it shouldn't be suggested. Added a "Locked until" date to the
  account add form's asset section.
- Clear semantics (locked = no new money before the date), so no spec ambiguity — unlike a member
  view filter, which I'm deliberately not inferring.
- **Next.** Add the lock-until field to the account *inline editor* too (so an existing account can
  be locked/unlocked), then it's fully manageable.

## 2026-06-16 — Spending breakdown: roll up to parent categories

- The dashboard breakdown now attributes each expense to its top-level ancestor category, so
  sub-category spend aggregates under the parent (Food, not Food/Restaurants + Food/Groceries
  separately). A small cycle/orphan-safe `rootOf` walks `ParentID` up to the root; uncategorized
  ("") stays its own bucket.
- Reused the existing top-3-plus-Other rendering — only the bucketing key changed (root ancestor
  instead of the literal category), so the chart and legend are unaffected.
- Puts the category hierarchy to work in reporting, completing sub-categories beyond just display.
- **Next.** Genuine small polish as it arises; the major remaining backlog item (sync) needs a
  backend.

## 2026-06-16 — Sub-categories: re-parent on the inline editor

- The inline category editor gains a parent `Select`, so an existing category can be nested under
  another, moved, or promoted to top level. `saveCat` now takes a parent and sets `ParentID`;
  `CategoryRow` receives `AllCategories` and builds same-kind, self-excluded, indented options.
- **Self excluded; deeper cycles tolerated.** The picker drops the category itself (the obvious
  self-parent); picking a *descendant* could form a cycle, but `categorytree.Build`'s visited-set
  guard drops cyclic nodes from display rather than looping — so the worst case is a temporarily
  hidden branch the user can fix, not a hang. Changing kind clears the parent (kinds must match).
- Sub-categories are now fully usable: create nested, re-parent, indented display. Breakdown
  rollup-to-parent remains an optional future enhancement.
- **Next.** Optional: roll the dashboard spending breakdown up to parent categories; otherwise other
  small polish.

## 2026-06-16 — Sub-categories: add-form picker + indented lists

- Wired the tree engine into the UI. The add form gains a parent `Select` populated from
  `categorytree.Flatten(kindCats)` (indented with `indentLabel`), filtered to the chosen kind; the
  category lists now render flattened-by-depth so children sit under their parent. `CategoryRow`
  takes a `Depth` and prefixes its label.
- **Kind/parent consistency.** Changing the kind clears the parent choice (`onKind` resets
  `parentID`), since a parent must share the child's kind — avoids creating a cross-kind nesting.
- Deferred parent editing on the inline editor to keep this commit focused; the engine is cycle-safe
  so even an odd edit can't break the display.
- **Next.** Parent selector on the category inline editor (excluding self), then optionally rolling
  spending-breakdown up to parent categories.

## 2026-06-16 — Sub-categories: the tree engine

- `Category.ParentID` has existed in the schema but was unused. Started sub-categories bottom-up with
  a pure `internal/categorytree`: `Build` → forest of `Node`s (siblings name-sorted), `Flatten` →
  depth-tagged list for an indented picker/list.
- **Defensive by construction.** Bad parent data shouldn't break the UI: an orphan (parent missing)
  or self-reference becomes a root, and a mutual cycle (a↔b) yields no roots rather than recursing
  forever (a shared `visited` set stops re-emission). Tested all three: nesting+sort, orphan-as-root,
  and cycle/self-reference safety, plus flatten depth.
- **Next.** A parent selector on the category add/edit forms (using `Flatten` for the indented
  options, excluding self/descendants to keep it acyclic), then indented display on the Categories
  screen. Spending breakdown rollup-to-parent could follow.

## 2026-06-16 — Ledger: cleared balance

- Added pure `ledger.ClearedBalance` (opening balance + only `Cleared` transactions) — the
  reconciliation figure — mirroring `Balance` but skipping uncleared rows. Tested against a mix of
  cleared/uncleared/other-account transactions.
- Surfaced on the account row: when the cleared balance differs from the live balance, the meta line
  shows "· cleared $X", so the gap (uncleared activity) is visible at a glance and the cleared figure
  can be matched to a statement. Computed per row in the accounts screen.
- Reconciliation is now complete: per-row + bulk cleared toggles, a cleared filter, and the cleared
  balance to check against.
- **Next.** Genuine small polish as it arises; sync remains out of scope without a backend.

## 2026-06-16 — Transactions: bulk mark cleared

- Added "Mark cleared"/"Mark uncleared" to the selection bar. One `bulkSetCleared(val)` closure sets
  the flag on each selected transaction (skipping ones already in the target state) and clears the
  selection; two thin event hooks bind the two buttons.
- Reconciliation is now fully ergonomic: filter to "not cleared", select a run of statement-matched
  rows, "Mark cleared" — repeat. Per-row toggle remains for one-offs.
- **Next.** Continue with genuine small polish where it helps; the major remaining backlog (sync)
  needs a backend.

## 2026-06-16 — Transactions: cleared-status filter

- Completed the reconciliation loop: a tri-state cleared filter (both / not cleared / cleared) added
  to `TxFilter` (so it persists with the rest of the filter), honored in the shared `applyTxFilter`,
  and surfaced as a dropdown. "Not cleared" gives a clean reconcile worklist; the toggle then clears
  items off it.
- Reused the existing persisted-filter + `applyTxFilter` plumbing — the new field flows through
  display, export, and persistence with no extra wiring.
- **Next.** Reconciliation is now usable end-to-end (toggle + filter). Will continue with genuine
  small polish; the major remaining item (sync) is out of scope without a backend.

## 2026-06-16 — Transactions: cleared/reconciled toggle

- Surfaced the long-defined-but-unused `Transaction.Cleared` flag. Each row gets a toggle
  ("Mark cleared" ↔ "Cleared ✓") that flips it via `PutTransaction`, and the meta line shows
  "· cleared" — the start of statement reconciliation.
- Used existing schema (no migration) and the per-row component hook pattern (`clr` event +
  `OnToggleCleared` prop taking the whole txn so the parent flips and persists).
- **Next.** A "cleared only / uncleared only" filter would round this out, but I'll keep increments
  genuine; otherwise polish or the out-of-scope sync.

## 2026-06-16 — To-do: inline edit (CRUD-edit complete across all entities)

- Added inline edit to `TaskRow` (title, priority, due, notes), the last entity without it.
  `saveTask` guards the priority with `TaskPriority.Valid()`, clears the due date when blank, and
  persists via `PutTask` — mirroring the goal date-clearing behavior.
- Now genuinely every entity — accounts, transactions, budgets, goals, members, categories, tasks —
  has inline edit, alongside add, delete, reassign-on-delete (categories/members), and bulk ops
  (transactions). The local CRUD surface is fully complete.
- **Next.** No edit gaps remain. Further work is optional UX polish (e.g. a member view filter,
  sub-categories) or the out-of-scope sync; I'll only add what's genuinely useful.

## 2026-06-16 — Categories: inline edit (CRUD-edit fully complete)

- The last edit gap: `CategoryRow` now edits inline (name + kind). `saveCat` guards the kind with
  `CategoryKind.Valid()` and persists via `PutCategory`.
- **Every entity is now fully editable inline** — accounts, transactions, budgets, goals, members,
  categories — each with the same unconditional-hooks + editing-toggle shape. Combined with the
  reassign-on-delete flows and bulk transaction ops, the CRUD surface is complete.
- **Next.** No CRUD gaps remain; further work is small UX polish or the out-of-scope sync. Will keep
  additions genuine and avoid churn.

## 2026-06-16 — Members: inline edit (name + color)

- Closed a real CRUD gap — members had add / delete / set-default but no edit. `MemberRow` now has
  an inline editor for the name and color via the same unconditional-hooks + editing-toggle pattern;
  `saveMember` persists through `PutMember`.
- **Next.** Category edit (name/kind) is the last remaining CRUD-edit gap; after that every entity
  is fully editable.

## 2026-06-16 — Transactions: export the filtered view to CSV

- Added an "Export CSV" button to the ledger filter bar that downloads exactly what's shown. To
  guarantee export==view, I extracted the inline filter+sort into a pure `applyTxFilter(txns, f)`
  used by both the render and the export handler — no duplicated predicate logic to drift.
- `appstate.TransactionsCSV(txns)` wraps `store.TransactionsToCSV` for an arbitrary subset (the
  existing `ExportCSV` does all), and a screens-local `downloadBytes` mirrors the app-package one
  (Blob + transient anchor) so the screens layer can trigger egress without reaching into app.
- **Caught my own refactor fallout:** removing the inline `fa/fc/fm` locals left the filter-option
  builders referencing them; pointed those at `f.Account/.Category/.Member`. Also re-ran the native
  suite in a clean shell — a stray `GOOS=js` from the combined build command had made the first
  `go test` falsely FAIL (the known lingering-env gotcha), green once isolated.
- **Next.** The feature set is comprehensive; I'll keep making small, genuinely-useful additions
  (export ergonomics, empty states) and avoid inventing churn — the major remaining item (sync)
  needs a backend.

## 2026-06-16 — Accounts: editable owner (ownership editing uniform)

- Added the owner picker to the account inline editor. Since `AccountRow` already builds a `cp` copy
  on save, this just sets `cp.OwnerID`/`cp.Scope` from the new `ownerS` and adds the select (reusing
  `ownerSelectOptions`).
- Ownership is now editable inline everywhere it can be owned — accounts, budgets, goals — closing
  the "ownership assignment UI" gap beyond the create-time selectors and the member-delete reassign.
- **Next.** The local, single-device feature set is now effectively complete (every entity: create /
  edit / delete / reassign; full budgeting/goals/planning/allocation/AI/documents/customization/
  preferences/PWA). The one remaining large backlog item, Phase-3 multi-device sync, needs a hosted
  backend and per-entity version metadata — out of scope for this local-first build to implement
  meaningfully. Will keep doing contained polish where it adds real value.

## 2026-06-16 — Goals: editable owner

- Added the owner picker to the goal inline editor too, reusing `ownerSelectOptions`. `saveGoal`
  gained an owner param and sets `OwnerID`/`Scope` the same way as budgets. Budgets and goals now
  both allow post-creation ownership changes inline.
- **Next.** Account owner edit to finish uniform ownership editing, then the local feature set is
  effectively complete.

## 2026-06-16 — Budgets: editable owner

- You could set a budget's owner at creation and reassign it only by deleting a member; now the
  inline editor has an owner picker. `saveBudget` gained an owner param and sets `OwnerID` + `Scope`
  (shared for the group, individual otherwise) consistently with the add path and the reassign flow.
- Added a reusable `ownerSelectOptions(members, selected)` helper (group + members) — the first step
  toward sharing owner editing across goals and accounts too.
- **Next.** Same owner picker on goal and account inline edits, then ownership editing is uniform.

## 2026-06-16 — Budget periods: the UI (feature complete)

- Wired periods into the budgets screen. A period `Select` (shared `periodOptions` over
  `domain.AllPeriods`) on both the add form and `BudgetRow`'s inline editor; `saveBudget` grew a
  period param (guarded by `Period.Valid()`).
- **Per-budget evaluation.** Replaced the single `EvaluateAll(start,end)` over one shared month with
  a loop that calls `budgeting.PeriodRange(b.Period, viewMonth, weekStart)` per budget and
  `Evaluate`s each in its own window. `weekStart` comes from the prefs atom, so weekly budgets
  respect the user's Sunday/Monday choice. Each row shows its period label.
- **Note on the month stepper.** It still navigates a reference date by month; a weekly/quarterly
  budget shows the period *containing* that reference. Stepping by the budget's own unit would be
  nicer but means a per-period stepper — deferred; the current behavior is correct and clear with the
  period label visible.
- Budget periods are now end-to-end: enum → `PeriodRange` engine (tested) → selector + per-budget
  evaluation.
- **Next.** The local feature set is essentially complete; the remaining large item (Phase-3 sync)
  needs a backend. Will continue with small polish or note completion.

## 2026-06-16 — Budget periods: enum + range engine

- Lifting budgets beyond monthly, bottom-up. `domain.Period` gains `PeriodWeekly`/`PeriodQuarterly`
  (with a `Label()` for the UI) and `Valid()`/`AllPeriods` updated. `budgeting.PeriodRange(p, ref,
  weekStart)` returns the half-open window of the period containing `ref`: weekly via
  `dateutil.WeekStart` (honoring the week-start pref) +7d, quarterly snapped to the calendar quarter
  (Apr–Jun → Apr 1..Jul 1), monthly via the existing `MonthRange`; unknown falls back to monthly.
- **Caught a brittle test.** `validate`'s budget test used `Period: "weekly"` as its *invalid*
  example (from when only monthly existed). Widening the enum made "weekly" valid and flipped the
  test; updated it to `"yearly"` (a genuinely unknown period). Good reminder that "invalid" fixtures
  age when an enum grows.
- Tested PeriodRange for monthly/weekly(Sun & Mon start)/quarterly.
- **Next.** Wire it into the budgets screen: a period selector on the add/edit form and per-budget
  evaluation using `PeriodRange(b.Period, ref, weekStart)` instead of one shared month range.

## 2026-06-16 — Goals: linked account

- Put the long-defined-but-unused `Goal.AccountID` to work. Added an optional "linked account"
  select to both the add form and the inline editor (shared `goalAccountOptions` helper with a
  leading "no link" choice), threaded the account id through `add` and `saveGoal` (its signature
  grew a param), and the row now shows "· linked to <name>" via an `accountName` lookup.
- **Scope — record the link, don't auto-sync the balance (yet).** Linking captures *which* account
  funds the goal; it doesn't override `CurrentAmount` from the account balance. That keeps the
  contribute flow and progress semantics intact while making the relationship explicit and editable.
  Auto-funding from the linked account could be a later opt-in.
- **Next.** Local feature set is essentially complete; remaining substantial item is Phase-3 sync
  (needs a backend). Will continue with small polish (e.g. owner editing on existing entities) or
  note completion.

## 2026-06-16 — Goals: pace guidance (save $X/mo)

- Added `goals.MonthlyNeeded(goal, from)`: remaining ÷ whole months until the target date, partial
  final month rounded up, ceil division so a goal is never under-funded. Returns ok=false for
  no-target-date, already-complete, or past-due goals. Pure, table-tested (incl. the rounding case).
- `GoalRow` shows "· save $X/mo" alongside the "by <date>" when the goal has a future target and
  isn't complete — turning a static deadline into an actionable pace. Reused `fmtMoney` and the
  prefs date format.
- **Decision — round up, minimum one month.** Better to suggest slightly too much than to land short
  of the goal by the date; a partial month (target day-of-month past today's) counts as a whole
  contribution month, and same-month targets floor to one month rather than dividing by zero.
- **Next.** Remaining backlog is dominated by Phase-3 sync (needs a backend); the local feature set
  is essentially complete. Will continue with small polish or flag completion.

## 2026-06-16 — Accounts: inline edit (CRUD-edit fully complete)

- The last CRUD-edit gap: `AccountRow` now edits inline. It mirrors the add form — name, opening
  balance, and the `If(isLiab,…)`/`If(!isLiab,…)` split for liability vs asset attributes. `OnSave`
  takes a fully-built `domain.Account`, so the row does the parsing (it has the currency) and the
  parent just `PutAccount`s it through validation.
- **Many hooks, all unconditional.** A dozen field states + their event hooks + the three action
  hooks are declared at the top; only the *return* branches on `editing`. Added small
  `moneyMajorOrEmpty`/`floatOrEmpty`/`intOrEmpty` seeders and `parseMoneyOrZero`/`parseFloatOrZero`/
  `parseIntOrZero` so blank optional fields round-trip as zero cleanly.
- **Currency intentionally not editable.** Changing an account's currency reinterprets every stored
  amount, so it stays fixed; everything else is editable. Opening balance edits flow through the
  balance calc immediately.
- Inline edit now exists for accounts, transactions, budgets, goals (+ categories/members via their
  forms). Every primary entity supports add / edit / delete.
- **Next.** The remaining substantial backlog item is Phase-3 sync (server + client), which needs a
  backend; otherwise the feature set is essentially complete. Will continue with contained polish or
  note completion.

## 2026-06-16 — Transactions: bulk recategorize (bulk actions complete)

- Added a category picker + "Apply category" to the selection bar. `bulkRecategorize` walks the
  ledger, and for each selected non-transfer transaction sets `CategoryID` and saves via
  `PutTransaction`, then clears the selection.
- **Transfers skipped.** Transfers aren't categorized (they show "Transfer"), so bulk recategorize
  ignores selected transfer legs rather than stamping a meaningless category on them — mirrors how
  the per-row edit/duplicate already exclude transfers.
- The empty option is "No category", so Apply can also *clear* categories — a legitimate bulk
  action — not just set one.
- Bulk actions are now complete: select → recategorize and/or delete → clear. Closes the §
  transactions bulk-ops TODO.
- **Next.** Remaining backlog is mostly Phase-3 sync (large) and smaller polish; will pick a
  contained item or note that the major feature set is essentially complete.

## 2026-06-16 — Transactions: bulk select + delete

- Added multi-select to the ledger. A `selected` set state in `Transactions`; each `TransactionRow`
  gets a `☐/☑` toggle button (reusing the to-do `check` style and the per-row-component hook rule),
  and when the set is non-empty a bar shows "N transactions selected" with Delete selected / Clear.
- **Bulk delete reuses `deleteTxn`.** Rather than a separate path, bulk delete calls the existing
  per-row `deleteTxn` for each selected id — so transfer pairs are still removed together, and a
  leg whose partner was already deleted is a harmless no-op. Selection clears afterward.
- Used a glyph toggle button instead of a real `<input type=checkbox>` to dodge the checked-attribute
  binding question and match the existing to-do check control — consistent and simple.
- **Next.** Bulk *recategorize* is the natural follow-up (reuse the category picker + a set update),
  or move to remaining polish / Phase-3.

## 2026-06-16 — Document import: dedupe vs existing (review TODO closed)

- Added duplicate detection so re-importing the same receipt doesn't double-enter rows. Pure side:
  `Row.Signature()` (date + normalized amount; description deliberately excluded) and `FilterNew`,
  with a `normalizeAmount` that strips `$`/commas/leading-`+` and formats to two decimals so "-4.5",
  "-4.50", and "$4.50" compare correctly.
- **Same Signature for both sides.** The screen builds the seen-set by rendering each existing
  transaction in the chosen account as a `Row{Date: FormatDate, Amount: FormatMinor}` and taking its
  `Signature()` — so the row-side and txn-side normalize identically. That sidesteps the earlier
  worry about "-4.5" vs "-4.50" mismatches: both go through the one normalizer.
- Scoped to the chosen account and to date+amount (not description), which is the right
  conservativeness — it won't suppress two genuinely-different same-day same-amount entries across
  accounts, and the user still sees and can re-add anything via the review list. Skips are reported.
- Closes the §2.2 review TODO (list, edit, remove, import, dedupe all done). Tests: signature
  normalization (incl. sign + description-excluded) and FilterNew.
- **Next.** Remaining large item is Phase-3 sync; otherwise smaller polish (bulk transaction ops,
  empty-state/a11y).

## 2026-06-16 — Document review: per-row edit (review complete)

- `DraftRow` now also edits inline: an Edit button reveals date/description/amount/category fields;
  Save calls `OnUpdate(index, Row)` which rebuilds the draft slice with the corrected row. Same
  unconditional-hooks-then-branch-on-editing shape as the budget/goal/transaction rows.
- Vision misreads (a smudged amount, a wrong date) can now be fixed in place rather than removed and
  re-entered, so the import is trustworthy without leaving the review step.
- The review screen is now full-featured: list → edit any row → remove any row → pick account →
  import. Only dedupe-vs-existing remains from the original review TODO.
- **Next.** Dedupe on import (skip rows already in the account by date+amount), or shift to Phase-3
  sync groundwork / other polish.

## 2026-06-16 — Document review: per-row remove

- Small polish on the just-shipped import: the review list rows are now `DraftRow` components with a
  ✕ that removes that row from the draft slice (`removeDraft(i)` rebuilds the slice without index i).
  So a misread line can be dropped before importing instead of importing everything or starting over.
- The row owns only an event hook (no state), so reusing instances across a removal is harmless; a
  plain index loop building `CreateElement` nodes is enough — no MapKeyed needed.
- That covers the "reject" half of the review TODO; per-row *editing* and dedupe-vs-existing remain
  as future polish.
- **Next.** Likely dedupe extracted rows against existing transactions, or shift to Phase-3 sync
  groundwork.

## 2026-06-16 — Document vision AI: the Documents UI (feature complete)

- Wired the three pieces into a working flow: Choose image → `pickImageDataURL` (a small js helper
  that creates a hidden file input, reads the chosen file via `FileReader.readAsDataURL`, and calls
  back with the data URL) → "Read with AI" → `SendVisionChat` with a strict JSON system prompt →
  `extract.ParseRows` → a review list → pick account → `importDraft` maps rows to transactions and
  saves through `PutTransaction`.
- **Forces a vision model.** Settings often holds `gpt-4o-mini` (fine for text, no vision), so the
  screen upgrades the model to `gpt-4o` for image reads rather than failing cryptically.
- **Mapping decisions at import:** amounts parse to minor units with the chosen account's currency
  and keep the model's sign (negative = expense); categories match by name (blank if unknown); an
  unparseable date falls back to today. Invalid/zero-amount rows are skipped. Review list is
  read-only for v1 — per-row editing can come later, but the user already controls the account and
  can decline to import.
- **js gotcha handled:** the framework's `OnChange` event doesn't expose the picked `File`, so the
  data-URL read is done with a direct `js.FuncOf` FileReader chain (funcs released on completion),
  the same pattern as the ai transport.
- Document vision import is now end-to-end (codec → transport → parser → UI). The CSV paste path is
  untouched and still key-free.
- **Next.** Larger remaining work is Phase-3 sync; smaller polish includes per-row editing of draft
  rows or empty-state/a11y passes.

## 2026-06-16 — Document vision AI: the extraction parser

- `internal/extract.ParseRows` bridges the model's reply to the import flow. Models are unreliable
  about output shape, so the parser is forgiving by design: bare array *or* object wrapper (tries
  transactions/rows/items/data/results), amounts as JSON numbers *or* strings, a spread of field-name
  synonyms (description/desc/merchant/payee/name), and it strips a ```json code fence. Rows with
  neither description nor amount are dropped.
- **Decision — strings out, not domain.Transaction.** `Row` is all strings and the package has no
  domain dependency. The user reviews/edits before import, and the screen maps rows → real
  transactions against a chosen account/currency at that point. Keeps extraction decoupled and the
  values exactly as the model gave them (editable).
- Fixed a first-draft bug where `amountString` returned early on a missing key instead of trying the
  next synonym — caught by the string-amount test. Six table tests cover array, wrapper, fence,
  skip-empty, and two error cases.
- **Next.** The Documents-screen flow: pick an image → base64 data URL → `SendVisionChat` with a
  strict "return JSON" prompt → `ParseRows` → editable draft rows → import against a chosen account.

## 2026-06-16 — Document vision AI: the transport

- Added `SendVisionChat` and, while there, factored the fetch promise chain out of `SendChat` into a
  shared `postCompletions(apiKey, baseURL, body, onResult, onError)`. Both senders now just build
  their body (text or vision) and hand it to the same network code — no duplicated js.Func juggling
  or release logic.
- Same contract preserved: exactly one of `onResult`/`onError` fires, errors are plain English, and
  the js.Funcs are released on completion. The vision reply parses through `ParseResponse` like any
  chat.
- **Next.** The Documents-screen flow: pick an image, base64 it into a data URL, call
  `SendVisionChat` with a "return JSON transactions" prompt, parse the JSON into draft rows, and let
  the user review and import. The JSON→transactions mapping should be a pure, tested helper.

## 2026-06-16 — Document vision AI: the request codec

- Started the document image-import feature (SPEC document vision) bottom-up with the pure codec.
  OpenAI vision differs from text chat in one way: the user message's `content` is an array of parts
  (`{type:"text"}` + `{type:"image_url",image_url:{url}}`) rather than a string.
- **Decision — a separate `visionRequest` shape, not a looser `Message`.** Rather than change
  `Message.Content` from `string` to `any` (which would ripple through every existing text call and
  weaken the type), vision gets its own small request/message/part structs in `ai/vision.go`. The
  *response* is identical to a text chat, so `ParseResponse` is reused as-is — no new parse path.
- Images travel as data: URLs, so the bytes go only to OpenAI (same BYO-key, client-side stance as
  the rest of the ai package). Tested the built JSON's structure (string system content, two-part
  user content, image url preserved) and that `ParseResponse` reads a vision reply.
- **Next.** A js/wasm `SendVisionChat` transport (read the picked file → base64 data URL → fetch),
  then a Documents-screen flow: pick an image, parse the model's JSON into draft transactions to
  review and import.

## 2026-06-16 — Transactions: inline edit (non-transfers)

- Completed CRUD-edit parity: income/expense rows now edit inline (description, amount, category,
  date). `TransactionRow` gained the editing toggle + four field states; the category picker needs
  the category list, so the row now takes a `Categories` prop. `OnSave(orig, desc, amount, cat,
  date)` hands the original txn back so the parent keeps the account and re-applies the sign.
- **Sign + account preserved, not re-entered.** The amount field shows the absolute value
  (`absAmount` + `FormatMinor`); on save the parent negates it iff the original was negative, so an
  expense stays an expense without a kind selector in the row. The account isn't editable inline
  (changing it is rare and affects currency) — that stays a delete-and-re-add.
- **Transfers excluded.** Editing one leg of a paired transfer can't keep the pair consistent, so —
  like Duplicate — Edit is hidden on transfer rows.
- Add / edit / delete now exist for accounts(+archive), transactions, budgets, goals, categories,
  members, tasks.
- **Next.** The big remaining items are document vision-AI parsing and Phase-3 sync. Likely start the
  sync groundwork with a pure, tested merge primitive, or do a smaller empty-state/a11y polish pass.

## 2026-06-16 — Goals: inline edit

- Same edit pattern as budgets, one entity over: `GoalRow` gets an `editing` toggle and name/target/
  date field states, with `saveEdit` calling a parent `OnSave(id, name, target, date)` that parses
  the target to minor units and the date via `dateutil.ParseDate`, saving through `PutGoal`.
- **Empty date clears the deadline.** A blank date field sets `TargetDate` to the zero time (no
  deadline) rather than erroring — matching the add form's optional-date behavior. The date input is
  seeded in ISO (`dateutil.FormatDate`) since `<input type=date>` needs ISO regardless of the user's
  display format preference.
- All ~12 hooks declared unconditionally; only the return branches on `editing`. Budgets and goals
  now both support add / edit / delete (goals also Contribute).
- **Next.** CRUD-edit parity is close; remaining big items are document vision-AI and Phase-3 sync.
  Could also do transaction edit, or start the sync groundwork with a pure merge primitive.

## 2026-06-16 — Budgets: inline edit

- Budgets were add/delete only; added inline editing of the name and monthly limit. `BudgetRow`
  gained an `editing` toggle plus name/limit field states and a `saveEdit` that calls a parent
  `OnSave(id, name, limit)`; the parent finds the budget, parses the limit to minor units, and saves
  through `PutBudget`.
- **Hook discipline.** All the row's hooks (`del`, `editing`, two field states, five event hooks)
  are declared unconditionally at the top; only the *return* branches on `editing`. So toggling edit
  mode never reorders hooks — the trap that bites when you wrap hooks in an `if`.
- Seeds the edit fields from the budget on each `startEdit` (not just initial mount), so reopening
  the editor always reflects the current values; the limit is shown in major units via
  `money.FormatMinor`.
- **Next.** Goal edit (same pattern) would round out CRUD-edit parity, or move to a larger item
  (document vision AI / Phase-3 sync groundwork).

## 2026-06-16 — Members: reassign-before-delete

- Mirrored the category reassign flow for members. `appstate.ReassignOwner(old, new)` moves owned
  accounts, budgets, and goals — and re-attributes the member's transactions — to the new owner,
  setting scope to match (shared for the group owner, individual otherwise) and clearing the
  transaction member when moving to the group. Tested with an account + goal reassigned to the group.
- The Members screen's delete now opens a reassign panel (default target: the shared group) instead
  of blocking; "Move and delete" reassigns then deletes. Same stable-hook discipline: panel hooks
  declared at the top, panel conditionally rendered, reusing the `Fragment()`-default pattern.
- **Decision — reuse the existing per-screen reassign shape rather than abstracting it.** Categories
  and members now have near-identical panels, but the entity types and the "what counts as owned/used"
  differ enough that a shared component would need awkward generics; two ~30-line panels read more
  clearly than one parameterized one. Noted in case a third reassign target appears.
- Both delete-guards (members §1.13, categories) are now reassign flows, not dead ends.
- **Next.** A larger remaining area — document vision-AI parsing, or a Phase-3 sync primitive — or an
  empty-state/accessibility polish pass.

## 2026-06-16 — Categories: reassign-before-delete

- Replaced the hard block on deleting an in-use category with a reassignment flow. Logic first:
  `appstate.ReassignCategory(old, new)` repoints every referencing transaction and budget (via the
  store directly — the records are already valid, just re-categorized) and reports how many moved;
  tested with a transaction and a budget.
- UI: `deleteCat` now opens a reassign panel (sets `reassignID`) when the category is in use, else
  deletes immediately. The panel lists the other categories in a select; "Move and delete" runs
  `ReassignCategory` then `DeleteCategory`, "Cancel" closes it. All the panel's hooks
  (`onReassignTo`, `confirmReassign`, `cancelReassign`) are declared at the component top and the
  panel itself is conditionally rendered, so hook order stays stable.
- **Decision — reassign to any category, not just same-kind.** Simpler and occasionally useful
  (recategorizing an expense as income-adjacent); validation already guarantees the target exists.
  Guard still prevents picking the same category or none.
- **Next.** Pick the next backlog item — likely the document vision-AI parse path, or a Phase-3 sync
  primitive, or smaller polish (empty-state/accessibility pass).

## 2026-06-16 — Freshness overrides: the editor (feature complete)

- Added a "Freshness reminders" section to the global settings left column: one number input per
  account type (a curated six), seeded from `app.FreshnessWindows()` so each shows its *effective*
  window (override or default). `setFreshness(typeKey, days)` writes `Settings.FreshnessOverrides`
  and bumps the data revision; the Accounts badges and dashboard widget re-read immediately.
- **Per-row component again.** Each input is a `freshnessRow` (CreateElement) so its `OnInput` hook
  is at a stable position — rendering them in a plain loop would break hook order.
- **0 means never.** Kept freshness's existing semantics (`window <= 0` → never stale) rather than
  inventing a separate "off" control; the helper text says "0 = never". To restore a default a user
  re-types it — acceptable, and avoids a tri-state.
- Freshness overrides are now end-to-end: stored field → `Merge` in the engine → `FreshnessWindows`
  application → Settings editor.
- **Next.** The category-delete reassign flow (move referencing transactions/budgets to another
  category before deleting), which currently just blocks with an error.

## 2026-06-16 — Freshness overrides: apply them

- `Settings.FreshnessOverrides` (a `map[string]int` of account-type → days) has existed and round-
  trips through export/import, but nothing read it — the screens always used `DefaultWindows()`.
  Wired it in via `appstate.FreshnessWindows()`, which converts the string-keyed overrides to a
  `freshness.Windows` and layers them over the defaults with the package's existing `Merge`.
- Both stale surfaces now use it: the Accounts list's stale badges and the dashboard Freshness
  widget (gave `freshnessWidget` a `windows` parameter rather than reaching for `app` inside it).
- **Bottom-up first.** No editor UI yet, but the feature is already functional — overrides set via
  imported JSON now change staleness — and the logic (`Merge`) was already tested. The Settings
  editor is the next, purely additive commit.
- **Next.** A Settings "Freshness" section: per-type day inputs that write `Settings.FreshnessOverrides`,
  so users can tune windows without editing JSON.

## 2026-06-16 — Transactions: persist the last filter

- The seven filter/sort fields were independent `UseState`s that reset on reload. Consolidated them
  into one `uistate.TxFilter` struct behind a localStorage-backed atom (`UseTxFilter`), the fourth
  durable atom alongside layout, prefs, and hidden-modules.
- **Refactor shape — one atom, a `setFilter(mutator)` helper.** Rather than seven persisted states,
  the component reads `f := atom.Get()` for rendering and every change calls
  `setFilter(func(x *TxFilter){ x.Field = v })`, which gets-mutates-sets-persists in one place. That
  keeps each handler a one-liner and guarantees the whole filter is saved atomically on any change.
  Clear writes a normalized empty filter.
- All read sites (`ft/fa/fc/fm`, the date parses, the sort switch, and every input's `Value`/
  `SelectedIf`) now derive from `f`, so the screen has a single source of truth.
- **Why an atom and not just persisted `UseState`s:** reading-then-persisting the full struct avoids
  the trap of `Set` followed by a stale `Get` in the same handler — the mutator operates on a fresh
  `Get()` and persists exactly what it sets.
- **Next.** Another contained backlog item — the category-delete reassign flow, or the
  freshness-window overrides editor (the `Settings.FreshnessOverrides` field already exists).

## 2026-06-16 — Allocation: amount-split UI

- Added two number inputs to the Allocate profile card — amount to allocate and emergency buffer —
  parsed to minor units via the base currency's decimals. When an amount is present, `Distribute`
  runs over the current ranking and a `planByID` map feeds each `AllocRow` its suggested dollar
  figure (shown beside the score). A "Kept back" line surfaces the returned remainder.
- **Reactive for free.** Everything recomputes from the input states each render, so changing the
  amount, buffer, profile, or excluding a destination instantly re-splits — no explicit wiring,
  just the atom/state re-render.
- **Money discipline held.** All arithmetic is int64 minor units (`money.ParseMinor` in,
  `money.New`+`fmtMoney` out); the engine owns the only float. Amount column is blank until an
  amount is entered, so the screen stays clean for users who just want the ranking.
- Allocation constraints are now meaningfully complete: rank → exclude/restore → split an amount
  with buffer and (engine-level) per-destination caps.
- **Next.** A different backlog item — persist-last-transaction-filter, the category-delete reassign
  flow, or the freshness-window overrides editor.

## 2026-06-16 — Allocation: amount-split engine

- The ranking told you the *order*; `Distribute` now turns it into *amounts*. Pure function:
  proportional-to-score split of a total (minor units), after a `Reserve` (emergency buffer) is held
  back and with an optional `MaxPer` cap per destination. Returns `[]Plan` + the unallocated
  remainder.
- **Decision — don't redistribute the remainder, return it.** Capped overflow and integer-rounding
  leftovers, plus the reserve, all flow into the returned remainder rather than being re-spread.
  That keeps the function simple and deterministic, and the remainder is meaningful to show ("kept
  back: $X"). A redistribution pass can come later if users want every cent placed.
- Money stays int64 minor units throughout (code-rule #6); the only float is the transient score
  proportion. Even-split fallback when all scores are zero avoids a divide-by-zero and still does
  something sensible.
- Tested proportional split, reserve hold-back, per-destination cap, even split, and the empty /
  over-reserve edges.
- **Next.** Wire it into the Allocate screen: an amount input (+ optional buffer) that runs
  `Distribute` over the current ranking and shows each destination's suggested dollar amount.

## 2026-06-16 — Allocation: exclusion UI

- Wired the engine constraint into the Allocate screen. An `excluded` map state feeds
  `allocate.Constraints{Exclude: …}` into `RankWith`; excluded destinations drop out of the ranked
  list and surface in a new "Excluded" card with Restore.
- **Component split for hooks.** The ranked list was a plain `for`-loop of `Div`s; adding an Exclude
  button there would put an `OnClick` hook in a loop — the cardinal sin. So the row became its own
  `AllocRow` component (rendered via `MapKeyed`), and excluded entries are `ExcludedChip`
  components, each owning its action hook.
- **One toggle for both directions.** `toggleExclude(id)` adds or removes the id (cloning the map so
  the atom gets a fresh value), so the same handler powers Exclude and Restore. Added an empty-state
  for "everything excluded" so the list never looks mysteriously blank.
- **Next.** Either the remaining allocation constraints (emergency buffer / max-per-destination,
  starting again at the engine) or a different backlog item like persist-last-filter.

## 2026-06-16 — Allocation: exclusion constraint (engine)

- New backlog area (allocation constraints), started bottom-up with the pure engine. Added a
  `Constraints` struct to `internal/allocate` rather than a bare `exclude map` parameter, so the
  obvious follow-ups (max-per-destination, required/emergency buffer, min-balance) slot in as more
  fields without breaking call sites. First field: `Exclude` (candidate-ID set) with an `Eligible`
  predicate.
- `RankWith(candidates, weights, constraints)` filters ineligible candidates, then delegates to the
  existing `Rank`. Kept `Rank` untouched and proved `RankWith(_, _, Constraints{})` is identical to
  it, so existing callers and tests are unaffected.
- Tests cover exclusion (excluded id absent, survivors correctly ordered), the zero-constraint
  equivalence, and the `Eligible` predicate including the zero-value-accepts-all case.
- **Next.** Wire it into the Allocate screen: per-candidate exclude toggles that build the `Exclude`
  set and call `RankWith`, so the user can park destinations they don't want recommended.

## 2026-06-16 — Transactions: per-row Duplicate

- Small, self-contained quality-of-life feature: a Duplicate button on each transaction row. The
  handler copies the struct, swaps in a fresh `id.New()` and today's date, deep-copies the tags
  slice (so the copy doesn't alias the original's backing array), and saves through
  `app.PutTransaction` — so it re-validates and honors custom fields like any new entry.
- **Decision — clear the transfer link on duplicate and only offer it for non-transfers.** A
  transfer is a matched pair; cloning one leg can't recreate the pairing, so a duplicate
  deliberately becomes a plain standalone entry. Rather than silently produce a half-transfer, the
  button is hidden on transfer rows (`If(!IsTransfer, …)`).
- Reused the established per-row component pattern: `OnDuplicate` prop + a `dup` hook owned by
  `TransactionRow`, so the action button's hook stays at a stable position.
- **Next.** Another contained item — persist-last-transaction-filter, or an allocation constraint
  (e.g. exclude destinations / emergency buffer) which would start with a pure engine change.

## 2026-06-16 — Appearance prefs: the light theme (feature complete)

- Authored the `[data-theme="light"]` skin deferred last commit. Three layers needed overriding,
  because the candidate-C styling mixes three coloring strategies: (1) the legacy `:root` CSS vars
  (one block flip covers all the screen components — cards, stats, fields, buttons, bars, badges);
  (2) the shell's Tailwind utility classes (`.bg-base`, `.text-fg`, `.border-line`, the
  arbitrary-value `.bg-[#1c1c1e]` active-nav surface — escaped as `.bg-\[\#1c1c1e\]`); (3) the
  widgets' hardcoded hexes (`.w`, `.seg`, `.rpill`, flip panel, `.set-input`, `.switch`, scrollbars).
- **Verified the one risky override.** `text-base` is also a Tailwind font-size utility, so blindly
  coloring it could turn body text invisible — grep showed it's used in exactly one place (the brand
  badge, alongside `bg-fg`), so inverting both there is correct and contained.
- The accent var is deliberately left out of the theme block: it is user-chosen and reads fine on
  both backgrounds, so it stays applied on top of whichever theme is active.
- Appearance preferences are now complete end-to-end: engine (week-start/date/theme/accent/density)
  → localStorage atom → Settings UI → `ApplyPrefs` to the DOM → working light/dark skins, all
  reload-persistent. Only fiscal-month start remains from the original preferences line.
- **Next.** A fresh backlog item — likely persist-last-transaction-filter or a per-row transaction
  duplicate action (both small, contained), or an allocation constraint.

## 2026-06-16 — Appearance prefs: apply to the DOM

- `uistate.ApplyPrefs(p)` reflects prefs onto `document.documentElement`: `data-theme` (with
  `resolveTheme` consulting `matchMedia` for "system"), `data-density`, and `--accent` via
  `style.setProperty`. Added `LoadPrefs()` so boot can apply the saved prefs without a hook (the
  atom can't be read outside a component). `app.Run` calls it right after `appstate.Init`, before
  mounting, so the first paint is already correct — no flash of defaults. `savePrefs` calls it too,
  so changes are instant.
- **Why accent works immediately:** the legacy `:root --accent` var is wired through the
  design-system CSS (buttons, `.bar-fill`, `.field:focus`, active nav), so overriding it on the root
  cascades everywhere at once. Density got a new `[data-density="compact"]` block tightening cards,
  rows, and fields.
- **Honest scope note — theme is half-applied on purpose.** The candidate-C surfaces are authored in
  fixed dark hexes (Tailwind config + hardcoded values), so a real light skin is a sizable CSS pass.
  This commit lands the mechanism (the `data-theme` attribute is set, system-resolved) and the two
  pieces that work cleanly today (accent, density). Picking "Light" sets the attribute but the skin
  is deferred to its own feature, so I don't ship a broken half-light look.
- **Next.** Either the light-theme stylesheet (a `[data-theme="light"]` palette pass) or move on to
  another backlog item (persist-last-filter, transaction duplicate, allocation constraints).

## 2026-06-16 — Appearance prefs: wire the controls

- Replaced the three local `UseState`s (theme/accent/compact) in `globalSettingsForm` with reads off
  the normalized `pr` and writes through the existing `savePrefs` (normalize → atom set →
  `PersistPrefs`). Dropping three hooks is safe — they were removed wholesale, so hook order stays
  consistent across renders.
- The Segmented/SwatchPicker/ToggleRow now reflect the persisted values and remember them; closing
  and reopening the panel keeps the selection, and it survives reload.
- **Note.** This makes the *preference* real and durable, but it does not yet *apply* visually — the
  page still renders dark with the green accent regardless. That is the next step: on change and on
  boot, set a `data-theme`/`data-density` attribute and the accent CSS variable on the document root
  so the choice actually changes the look.

## 2026-06-16 — Appearance prefs: extend the engine

- The settings panel's theme / accent / density controls have been local-only React-style state all
  along (they reset on close). Making them real reuses the prefs pipeline, so step one is extending
  `internal/prefs`: added `Theme` (dark/light/system), `Accent` (hex string), and `Compact` (bool).
- **Decision — validate the accent in `Normalize` with a tiny `isHexColor`.** Accent comes from a
  color `<input>` but persisted data could be anything; rather than trust it, normalize rejects
  non-`#rgb`/`#rrggbb` strings back to the default green. Keeps the "always-usable persisted data"
  invariant the rest of prefs already holds.
- `Default()` now seeds dark + green; `Compact` defaults to false (zero value), so no special case.
  Existing week-start/date tests unchanged; added theme/accent/hex-color tests.
- **Next.** Wire the settings appearance controls to these fields (atom + PersistPrefs), then apply
  them to the DOM (a `data-theme`/`data-density` attribute + accent CSS var on the document root),
  and seed that application on boot so it survives reload.

## 2026-06-16 — Module visibility: Settings toggles (feature complete)

- Final step: a "Screens" section in the global settings left column. A package-level
  `hideableScreens` list (label + path, excluding the locked dashboard/settings) drives a
  `ui.ToggleRow` per screen. Because `ToggleRow` is a `CreateElement` component owning its own hook,
  the per-row toggles render safely in a plain loop — no parent hook-ordering worry.
- Each toggle's `OnChange` calls `toggleModule(path)` → `Toggle` (immutable) → atom `Set` →
  `PersistHiddenModules`. Both the form (subscribed via `UseHiddenModules`) and the sidebar
  re-render, so a hidden screen vanishes from the rail the instant you flip it, and the choice
  survives reload.
- Module visibility is now end-to-end: pure engine (locked + toggle) → localStorage atom → sidebar
  filter → Settings toggles. Closes §1.18's show/hide-screens item.
- **Next.** Other §1.18 items remain (theme/density, fiscal-month start, budgeting methodology
  selector), or move to a contained Phase-3 sync primitive. Will pick the next granular increment.

## 2026-06-16 — Module visibility: sidebar filtering

- `Sidebar` now reads `uistate.UseHiddenModules().Get()` and filters: the primary nav is built into a
  `visibleNav` slice (skipping hidden paths) before the `MapKeyed`, and the two hideable System items
  (Members, Categories) are wrapped in `If(!hidden.IsHidden(path), …)`. Settings and Dashboard are
  locked in `internal/modules`, so they are never filtered — no special-casing needed here.
- Reading the atom subscribes the Sidebar, so flipping a toggle re-renders the rail at once.
- **Scope note.** Hiding is a *navigation* concern: the routes stay registered, so a hidden screen
  reached directly by URL (or the unknown-path fallback) still renders. That is deliberate — we are
  decluttering the rail, not building access control.
- **Next.** The Settings panel show/hide toggles, which write `Toggle` + `PersistHiddenModules` for
  each hideable screen — the last step to close this feature.

## 2026-06-16 — Module visibility: the persistence atom

- `uistate/modules.go` — the third localStorage-backed atom (after layout and prefs), same shape:
  `UseHiddenModules` seeds from `loadHiddenModules()` (key `cashflux:hidden-modules`, normalized,
  empty set on miss/parse error), `PersistHiddenModules` marshals the normalized set back. Empty set
  = everything visible, which is the right default.
- Thin plumbing again — the `Normalize`/locked-path logic all lives in the tested `internal/modules`
  package; this file is JSON ↔ localStorage only.
- **Next.** Filter the sidebar nav by the hidden set (shell.go), then add per-screen show/hide
  toggles to the global Settings panel.

## 2026-06-16 — Module visibility: the pure engine

- New backlog item (§1.18 module-visibility toggles). Same reload-persistent shape as preferences,
  so same approach: pure logic first, then a localStorage atom, then sidebar filtering + settings
  toggles. Pure package `internal/modules`.
- **Decision — lock the home and settings screens.** Hiding the dashboard or the settings screen
  (which is where you'd un-hide things) would be a footgun, so `IsLocked` makes them permanently
  visible and `Toggle`/`Normalize`/`IsHidden` all respect that. Cheap guard, big safety win.
- **Decision — immutable Toggle returning a minimal set.** `Toggle` clones rather than mutating
  (the atom value should be replaced, not edited in place) and the set only ever stores `true`
  entries, so it serializes compactly and `Normalize` can clean stale/false/locked keys on load.
- **Next.** `uistate` localStorage atom for the hidden set, then filter the sidebar nav by it and
  add per-screen toggles to Settings. (Routes themselves stay registered; hiding is a nav concern,
  and a hidden screen reached by URL still works.)

## 2026-06-16 — Reload-persistent preferences: wiring dates through (feature complete)

- Final step: the three user-facing date displays (TransactionRow, GoalRow, TaskRow) now format via
  `prefs.FormatDate` instead of `dateutil.FormatDate`. Each row reads `uistate.UsePrefs().Get()` at
  the top of its component — unconditionally, because GoalRow's date sits inside an
  `if !TargetDate.IsZero()` and a hook there would be conditional. Reading the atom also subscribes
  the row, so flipping the format in Settings re-renders every list immediately.
- Left `dateutil.FormatDate` in place for machine/edit contexts (date `<input>` values, parsing)
  where ISO is required — preferences only change *display*, not the canonical storage/parse format.
- §1.18 week-start + date-format preference is now end-to-end: pure engine → localStorage atom →
  Settings UI → live rendering, all surviving reload. Theme/density and fiscal-month start remain as
  separate future prefs.
- **Next.** Move to the next backlog area — likely module-visibility toggles (show/hide screens,
  also a reload-persistent preference) or a contained Phase-3 sync primitive.

## 2026-06-16 — Reload-persistent preferences: the Settings UI

- Step 5 (UI): a "Preferences" block in the global settings back-face. Week start is a `Segmented`
  (its OnSelect is a plain prop, so no parent hook needed); date format is a `Select`, whose
  `OnChange` *does* register a parent hook — so `onDateStyle` is declared unconditionally at the top
  with the other event hooks, keeping hook order stable.
- Both controls funnel through one `savePrefs` closure that normalizes, sets the atom, and calls
  `PersistPrefs`. Reading uses `prefsAtom.Get().Normalize()` so the rendered selection always
  reflects a valid value. Date options show a live example (2026-06-05, 06/05/2026, …) so the choice
  is self-explanatory — plain-English-UI rule.
- **Next.** The preference is captured and persists, but the screens still render dates via
  `dateutil.FormatDate`. Final step: route user-facing date rendering through `prefs.FormatDate` so
  the choice actually shows up in Transactions, Goals, etc.

## 2026-06-16 — Reload-persistent preferences: the persistence atom

- Step 4 (state): `uistate/prefs.go`, a near-mirror of `layout.go`. `UsePrefs` is a `state.Atom`
  seeded from `loadPrefs()` (localStorage key `cashflux:prefs`, normalized, defaults on miss/parse
  error); `PersistPrefs` marshals the normalized prefs back. No store involvement — by design,
  preferences live outside the dataset because the store is wiped on every boot.
- This keeps the wasm/persistence layer thin: all the meaning (formatting, week math, normalization)
  is in the tested `internal/prefs` package; this file is just JSON ↔ localStorage plumbing.
- **Next.** A Settings form (global panel) to choose week start and date style, calling
  `atom.Set` + `PersistPrefs`; then route the screens' date rendering through `prefs.FormatDate`.

## 2026-06-16 — Reload-persistent preferences: the pure engine

- New backlog area: preferences that survive a reload (week start, date format). Established first
  that store-backed `Settings` do *not* survive reload — `app.Run` calls `appstate.Init(nil, true)`,
  which re-seeds the in-memory SQLite store on every boot. The only durable channel is localStorage
  (that is how the dashboard layout persists). So preferences will follow the layout pattern: a
  localStorage-backed atom, seeded on boot, written on change — separate from the dataset.
- Per the SDLC rule, started with the pure logic: `internal/prefs`. `Prefs{WeekStart, DateStyle}`
  plus `FormatDate`, `WeekStartWeekday`, `WeekStartOf`, and `Normalize`. Keeping the display logic in
  a platform-free package means it is unit-tested on native Go and the wasm layer stays a thin
  localStorage + form shell.
- **Decision — `Normalize` everywhere a value is read.** Persisted prefs may be partial or from an
  older build, so every accessor normalizes first; blanks/unknowns fall back to defaults rather than
  producing an empty layout string. Same forward-compatibility stance as the custom-field defs.
- **Next.** Wrap `prefs` in a `uistate` localStorage atom (`UsePrefs`/`PersistPrefs`), then a
  Settings form to edit it, then route date rendering in the screens through it.

## 2026-06-16 — Custom fields: Goals, Budgets, Members (rollout complete)

- Applied the now-proven pattern to the last three entity forms in one pass: each gets a
  `customVals` value-map state, a `<entity>Defs := app.CustomFieldDefsFor(...)`, the `onCustom`
  push-up closure, a `MapKeyed` of `CustomFieldInput`s in the form, `Custom:
  customValuesToMap(...)` on the built entity, and a reset on success. The matching appstate write
  paths (`PutGoal`/`PutBudget`/`PutMember`) now call `validateCustom`.
- **Grouped as one feature deliberately.** The three integrations are byte-for-byte the same shape
  the Accounts/Transactions commits already established; splitting them into three near-identical
  commits would be noise, not granularity. The unit of work here is "finish the rollout", and it
  maps to one checklist line in §1.16.
- §1.16 form rendering is now closed for all five entity types (accounts, transactions, budgets,
  goals, members). The whole custom-fields feature — model, validate, persist, manage UI, render on
  forms, export/import — is complete.
- **Next.** Pick up the next backlog area: module-visibility toggles / reload-persistent
  preferences, or a contained Phase-3 sync primitive.

## 2026-06-16 — Custom fields: Transactions form

- Second entity wired up, and the reusable pieces paid off: the Transactions add-form now renders
  transaction custom fields via the same `CustomFieldInput` + `customValuesToMap` + parent value-map
  pattern, and `appstate.PutTransaction` gained the `validateCustom("transaction", …)` guard.
- **Decision — custom fields apply to income/expense, not transfer legs.** A transfer is two paired
  rows; hanging user fields off one leg is ambiguous, so when the kind is Transfer the form passes an
  empty def slice (`formTxnDefs = nil`) and nothing renders. Keeps the model honest without inventing
  transfer-pair custom semantics.
- Confirmed the empty-slice-flattens trick again: `MapKeyed(nil, …)` renders nothing, so no `If`
  guard is needed around the custom inputs.
- **Next.** Budgets, Goals, Members forms — same mechanical integration — then §1.16 is fully closed.

## 2026-06-16 — Custom fields: rendering on entity forms (Accounts first)

- The defs now drive real inputs. `CustomFieldInput` is a reusable component that picks the control
  for a field's type and reports `(key, value)` up to the parent form, which owns a
  `map[string]string` value state. Both event hooks (`onText` for inputs, `onSel` for selects) are
  declared unconditionally at the top so hook order is stable whatever the field type — the
  component is then safe to render from a `MapKeyed` list.
- **Decision — push values up, don't pull them down.** Each input is controlled and emits changes to
  a single parent map rather than holding its own state, so the submit handler can read every value
  at once and build the typed `custom{}` map (`customValuesToMap`: numbers→float64, yes/no→bool,
  else string; empties omitted so optional fields stay unset).
- **Validation lives in `appstate.PutAccount`, not the view.** Added `validateCustom`, which loads
  the account defs and runs `customfields.Validate`, returning `validate.Issues` — so any save path
  (not just this form) enforces required/typed custom fields. A defs *read* error never blocks a
  save (logged and ignored); only real value problems reject.
- **Framework gotcha hit:** `If(cond, MapKeyed(...))` doesn't compile — `MapKeyed` returns
  `[]ui.Node` and `If` wants a single `ui.Node`. The fix is to drop the `If`: an empty def list
  yields an empty slice that flattens to nothing, same as `Div(..., MapKeyed(...))`.
- **Next.** Repeat the integration for Transactions (and other entities) so custom fields are
  available everywhere they're defined.

## 2026-06-16 — Custom fields: management UI

- Step 5 (UI last) for §1.16: `CustomFieldsManager`, a thin shell over the now-tested persistence.
  Add-field form (entity type, key, label, type, options, required) + grouped list with per-row
  delete. Per-row delete is its own component (`CustomFieldRow`) so its `OnClick` hook sits at a
  stable render position — the cardinal framework rule.
- **Decision — host it on the existing Customize screen, not a new route.** That screen is already
  subtitled "Custom fields and formulas" but only did formulas; dropping the manager above the
  calculator fulfils the promise and keeps the nav uncluttered. No routing changes.
- **UI choices.** The choice-field options input only appears when the type is "Choice"
  (`If(isChoice, …)`); required is a plain Optional/Required select rather than a checkbox to match
  the other dropdown-driven forms. Validation errors from `Def.Validate()` surface inline via the
  shared `validate.Issues` error string. Entity list is curated (the five entities users actually
  annotate) rather than reflected, so the labels read in plain English.
- **Next.** The defs exist and persist; the remaining step is rendering these fields as inputs on
  the actual entity forms (accounts/transactions/…) and validating `custom{}` on save — a per-form
  integration I'll do entity by entity.

## 2026-06-16 — Custom fields: persistence layer

- Step 3 of the SDLC for §1.16: persist `CustomFieldDef`s. Added a `customfielddefs` table to the
  SQLite store (same id+JSON-document shape as every other entity), full CRUD, a
  `CustomFieldDefsByEntity` query (via `json_extract` on `$.entityType`, mirroring the
  transactions-by-account pattern), and wired the new entity through `Load`/`Snapshot`, `Wipe`'s
  `allTables`, and the `Dataset` aggregate so export/import round-trips it.
- **Decision — keep `Def` in `internal/customfields`, not `internal/domain`.** The type and its
  validation are inseparable, so the package that validates owns the type. `store` and `appstate`
  importing `customfields` is a clean one-way dependency (it only pulls in `dateutil`). Added JSON
  tags to `Def` so the persisted shape is stable and lowercase like the rest of the dataset.
- **No schema-version bump.** The new `customFieldDefs` array is additive and `omitempty`; old
  exports decode fine (nil slice), so `SchemaVersion` stays at 1 — the migration guard is reserved
  for shape changes that actually break old data.
- `appstate.PutCustomFieldDef` runs `Def.Validate()` and adapts the plain-English messages into the
  existing `validate.Issues` error type, so the write path behaves like every other entity.
- **Next.** State seam is thin here (defs are read directly), so the remaining work is UI: a
  management screen to add/edit/remove defs per entity type, then rendering the inputs on entity
  forms and validating `custom{}` on save.

## 2026-06-16 — Custom fields: the validation core first

- Started SPEC §1.16 (user-defined custom fields) bottom-up, per the SDLC rule: model + validate
  before any store or UI. New pure package `internal/customfields`.
- **Design.** `Def` is a strongly-typed field definition (id, entity type, map key, label, one of
  five `FieldType`s, optional select `Options`, `Required`). This honours code-rule #7: the core
  schema stays strongly typed; extensibility comes from *validated* custom fields, not from
  loosening entities into untyped maps. `Validate(defs, values)` collects *all* issues (not
  first-fail) so a form can show every problem at once, and returns plain-English messages.
- **Trade-offs.** Custom values arrive from JSON, so numbers are `float64` — `isNumber` accepts the
  float and int kinds rather than insisting on one. Dates are validated through the existing
  `dateutil.ParseDate` (single source of truth for the YYYY-MM-DD format) instead of a second
  parser. Unknown keys in a value map are ignored rather than flagged, so data written before a def
  existed (or after one is removed) never hard-fails — forward/backward compatible by default.
- **Next.** Persist `CustomFieldDef`s (store + export/import round-trip), expose them via appstate,
  then a thin Settings UI to manage defs and render the inputs on entity forms — strictly in that
  order.

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

- Added the `AreaChart` ui helper (feeds `chart` paths into an `<svg>` with a gradient fill, built
  from generic `Tag("defs"/"linearGradient"/"stop")` SVG nodes) and the **Net worth trend** widget
  (1×2): current figure + a six-month end-of-month area chart from `ledger.NetWorthSeries`. Cutoffs
  are first-of-month from M-5 to M (AddMonths(start, i-4)).

- Added the **Cash flow** widget (2×1): income/expense bars for the last four months (div bars,
  height % scaled to the largest bar across all months) plus the current month's net, from
  `ledger.PeriodTotals` (confirmed it returns expense as a positive magnitude). Used a tiny div-bar
  approach rather than SVG since the mockup does.

- Added **Savings rate** (period income saved %, big figure + bar) and **Spending breakdown**
  (segmented bar of period expenses by category — top three + "Other" — with a color-keyed legend,
  all converted to base currency, sorted desc). Both reuse the `Widget` shell; breakdown reuses the
  window range already computed in `Dashboard`.

- Added the **Upcoming bills** widget (2×1), completing the 12-widget catalog: next due date + min
  payment per liability account (clamped due-day, soonest first, within-a-week dates toned amber),
  via a small `nextDue` helper. The whole candidate-C bento is now live data on the reusable shells.

- Wired **per-widget settings**: new `settings:target` atom in `uistate` (`SettingsTarget{Kind,ID,
  Title}` — closed/widget/global). The `Widget` gear defaults to opening its own panel (computes the
  open-closure during render so the `UseSettings` hook stays at a stable position, not in the click
  handler), overridable via `OnGear`. A `SettingsHost` component mounted at the shell root renders the
  `FlipPanel` for the active target and nothing (`Fragment()`) when closed — so each open is a fresh
  mount and the flip animation replays. The widget settings back face (editable title + behavior
  toggles via `ToggleRow`) holds local state for now; persisting visibility/layout to the store
  arrives with the layout model. Confirmed `internal/ui` can depend on `uistate` without a cycle.

- Built the **global settings** panel body: a two-column form inside the `FlipPanel` (760×560) with
  live household member chips, base currency, and sorted editable FX rate rows on the left; AI
  (BYO-key toggle + key + model), Appearance (theme `Segmented` + accent `SwatchPicker` + compact),
  and Data action buttons on the right. Reuses every shared control primitive. Members/base/FX are
  real reads from `appstate`; appearance is local state for now; the Data buttons are present but
  wired in the next feature (export/import/wipe need js download + store mutation + refresh).

- Wired the **Export JSON** data action: a tiny `downloadBytes` helper (the one DOM-touching spot for
  file egress — Blob + transient anchor via `syscall/js`) downloads `appstate.ExportJSON()` as
  `cashflux.json`. Generalized `dataBtn` into its own `dataButton` component taking an `OnClick` so
  the remaining actions slot in cleanly.

- Added the data-action seams to `appstate`: `ExportCSV` (via `store.TransactionsToCSV`), `LoadSample`
  (replace with `store.SampleDataset` — `store.Load` replaces, as the import path proves), and `Wipe`.
  Native test loads sample → asserts populated → wipes → asserts empty, plus a CSV smoke test.

- Wired all global-settings **Data actions**: Export CSV (download), Import (`.json` file picker →
  `ImportJSON`), Load sample, and Wipe (guarded by a native confirm). Added `pickFile` (file input +
  `FileReader` → bytes, releasing the js callbacks after read) and `confirmAction`. Refresh uses a
  shared `data:revision` atom: bulk actions bump it and `Dashboard` reads it so it re-renders behind
  the still-open panel. Other screens refresh on their own navigation/rev atoms.

- Started bento drag/resize **bottom-up**: new pure `internal/dashlayout` holds the grid model —
  `Placement` (col/row + spans, with `GridColumn`/`GridRow` CSS string helpers), `Layout` with the
  candidate-C `Default()` (14 widgets), and immutable `Swap`/`Resize`. Table-driven tests cover the
  CSS strings, swap symmetry + immutability, unknown-id no-ops, and span clamping. The UI will hold a
  `Layout` in an atom, source each widget's placement from it, and write back swaps/resizes.

- Wired placement through state: new `uistate.UseLayout()` atom (default `dashlayout.Default()`); the
  `Widget` shell looks up its own `Placement` by ID and uses its CSS grid strings when present, else
  the caller's `GridColumn`/`GridRow`. No visual change (default == the hardcoded positions) but now a
  single `layout.Swap`/`Resize` written to the atom re-places every widget. Widgets already subscribe
  to the atom via the hook, so reorder/resize will re-render the whole grid.

- Wired **drag-to-swap**: the framework's `Prevent` wrapper calls `PreventDefault` before the handler,
  so `OnDragOver(Prevent(func(){}))` enables the drop. `OnDragStart` stashes the widget id in a shared
  `drag-source` atom; `OnDrop` swaps via `dashlayout.Swap` written to the layout atom (re-placing both
  widgets) and clears the source; `OnDragEnd` clears it if dropped outside. The dragged cell dims via
  `.drag`. No `DataTransfer` needed — the atom carries the source id.

- Made the **resize handles functional**: the right/bottom edge handles cycle the widget's col/row
  span via `dashlayout.Resize` (clamped to the 4×3 grid), re-placing it live through the layout atom.
  Chose click-to-cycle over pointer-drag for now — it's reliable without browser testing and the math
  stays in the tested `dashlayout`; smooth pointer-drag resize is a later polish. Only `kpi-networth`
  currently carries handles (as in the mockup); enabling them across all widgets is the next commit.

- Enabled drag+resize on **all** widgets (normalized the net-worth widget then `Resizable`-stamped
  every call), and added **layout persistence to `localStorage`**: `PersistLayout` marshals the layout
  after each drag/resize and `loadLayout` seeds `UseLayout`'s initial value (falling back to
  `Default()` when absent/invalid). Chose `localStorage` over the store because the SQLite store is
  in-memory and re-seeded on boot — only browser storage actually survives a reload. Missing widgets
  fall back to their default placement, so adding widgets later degrades gracefully.

- Restyled all non-dashboard screens in one move by **retargeting the legacy CSS variables** to the
  candidate-C palette (base `#0e0e0f`, tile `#121214`, border `#2a2a2c`, up/down, radius 4px). Since
  the old screen components (cards, stats, rows, forms, bars) are all driven by these vars, they now
  match the flat neutral-dark shell without rewriting any Go — and they already inherit Inter from the
  shell root. Per-screen bento-style polish can follow, but the jarring blue theme is gone.

- Added a **Reset layout** action in the dashboard header cell: restores `dashlayout.Default()` to the
  atom and persists it, undoing any drag/resize. (Persisting the default overwrites the saved layout,
  which is the intended "clear customization" behavior.)

- Synced `TODOS.md` (§1.7c mostly done) and implemented **account transfers** (§1.11 ★). The model is
  paired entries: Balance only counts a transaction against its own `AccountID`, so a transfer needs a
  debit on the source and a credit on the destination, both carrying `TransferAccountID` (so
  `IsTransfer` excludes them from income/expense). The Transactions form gains a "Transfer" kind that
  swaps the category picker for a "To account" picker; submit validates distinct accounts + matching
  currency (cross-currency deferred) and writes both legs. Known gap: deleting one leg orphans the
  other — a paired delete is the follow-up.

- **Paired transfer delete**: deleting a transfer leg now finds and removes its reciprocal (accounts
  swapped, amount negated, same date) so balances don't drift. Heuristic match (no schema change /
  migration); a shared transfer-group id would be more robust if duplicate transfers collide — noted.

- Added **transaction filters**: a filter bar (case-insensitive description search + account picker,
  Clear button) narrows the in-memory list before render, with a separate "No matching transactions"
  empty state distinct from "No transactions yet". Date-range/category/member filters + persistence
  can follow.

- Added **account archive/restore**: each account row gets an Archive/Restore toggle (`AccountRow`
  grows a second action hook); archived accounts move to a dedicated "Archived" card and leave the
  assets/liabilities lists and net-worth totals (`ledger` already excludes them). Toggling just flips
  `Archived` and re-puts through the validated `appstate` path.

- Extended the transaction filter bar with a **category** picker (combines with search + account).

- Added the **Categories screen** (add name + income/expense kind, grouped lists, per-row delete via
  `CategoryRow`), registered `/categories`, and surfaced it in the rail's System group with a new
  `tag` icon. Category edit + color + delete-reassignment are follow-ups.

- Added the **Members screen**: add (name + color picker), list with a color swatch + Default badge,
  Make-default (flips `IsDefault` across members through the validated put path), and per-row delete
  (`MemberRow`). Registered `/members` with a new `users` rail icon under System. Delete-guard for
  members with owned entities is a follow-up.

- Added a **Freshness nudge** widget (full-width row 8 — grew the bento to 8 rows + a `dashlayout`
  placement, test count 14→15): friendly "N balances could use a refresh" with per-account days-since
  via `freshness.StaleAccounts`/`DaysSinceUpdate`. One-tap update + dismissal are follow-ups.

- Added a one-tap **"Mark updated"** action per active account (`AccountRow` grows a third hook,
  rendered only when not archived): sets `BalanceAsOf` to now via the validated put, clearing the
  staleness the freshness nudge reports. A full "update balance" (enter a new figure → adjustment txn)
  is the richer follow-up.

- Added a **member delete-guard**: deletion is blocked (with a plain-English count) when the member
  still owns any account, budget, or goal, so those references can't be orphaned. A reassign flow is
  the richer follow-up.

- Added a **category delete-guard** mirroring the member one: blocks deletion (with a count) when any
  transaction or budget still references the category. A pick-a-replacement reassign flow is the
  richer follow-up.

- Replaced the **Settings** stub with a real page: a household summary (base currency + member/
  account/category counts) and an in-app **debug log viewer** (the slog `Ring`, newest-first, Refresh
  button). Heavy editing stays in the global panel + dedicated screens to avoid duplication.

- Added **transaction tags**: a comma-separated tags field on income/expense entries (`parseTags`
  trims/drops empties), stored on `Transaction.Tags` and shown on the row as `#tag`. Tag-based
  filtering is a follow-up.

- Added **contribute-to-goal**: a per-goal Contribute button prompts for an amount (`promptText`
  wraps `window.prompt`), parses it in the goal's currency, and adds it to `CurrentAmount` via the
  validated put — advancing the progress bar. Auto-progress from a linked account is a follow-up.

- Wired the top-bar **"+ Add"** button to navigate to Transactions (was a no-op).

- Added a **budget month stepper**: a `monthOffset` state + ‹/› pills drive `dateutil.AddMonths` so
  you can review any month's budget spend, not just the current one.

- Added an optional **notes** field to tasks (form input + row display), stored on `Task.Notes`.

- Extended the transaction search to match **tags** as well as descriptions (`matchesText`).

- Added an **onboarding** welcome card on the Accounts screen (shown when there are no accounts) with
  a "Load sample data" button wired to `appstate.LoadSample` + the screen's revision bump.

- Added **transaction sort** (newest first / largest amount via `absAmount` / payee A–Z), applied to
  the filtered list before render.

- Replaced the Net worth KPI's static "Assets X" subline with a real **month-over-month delta** (▲/▼
  integer %) computed from `ledger.NetWorthSeries` at this month's start; falls back to the assets
  line when there's no prior figure. Removes the last fabricated "2.4%" placeholder from the mockup.

- Added a **Hide done / Show all** toggle to the To-do list (filters completed tasks; distinct
  "All done 🎉" state when everything's hidden).

- Sorted the **goals list incomplete-first** (then alphabetical) via a stable sort using
  `goals.IsComplete`, so active goals stay on top.

- Income/Spending KPI sublines now show the period plus the real deposit/transaction **count** for it
  (a `plural` helper), replacing the bare period label and matching the mockup's "June · 1 deposit".

- Hygiene pass after ~67 features: `go vet ./...` (js/wasm) is clean; `gofmt -w` tidied alignment in
  seven files (mostly table-comment columns + one entities.go gap). Native tests still green.

- Added a **per-account "Stale" badge** (amber) on the Accounts screen via `freshness.IsStale`,
  closing the loop with the dashboard freshness nudge and the per-row "Mark updated" action.

- Added a **budget health summary** line ("N over budget · M near the limit") from the evaluated
  statuses, shown above the budget list when any are over/near.

- Added **credit utilization** to liability account rows (an `accountMeta` helper appends "N% of limit
  used" when a liability has a credit limit), using the row's already-computed balance.

## 2026-06-15 — Phase 2 begins (bottom-up): debt payoff

- Phase-1 core is broadly built out (all candidate-C UI + accounts/transactions/budgets/goals/todo/
  members/categories/settings with filters, transfers, archive, freshness, tags, etc.), so I started
  **Phase 2 bottom-up** with a pure logic package: `internal/payoff`. `Project(balance, aprPercent,
  payment)` simulates monthly APR compounding + a fixed payment, returning months-to-zero, total
  interest, and total paid; `ok=false` when the payment can't cover the interest (so it would never
  clear) and a 1200-month cap as a backstop. Table-driven tests: 0% APR exact (10 months), an
  interest-bearing case (~11 months, interest > 0), payment-too-small, already-paid, zero-payment.

- Surfaced payoff in the **Planning screen** (replaced the stub): a live debt-payoff calculator
  (balance / APR / monthly payment → months, total interest, total paid) wired to `payoff.Project`,
  recomputing on each keystroke (no submit) with a plain-English non-viable message.

- Built the **allocation engine core** `internal/allocate` (pure, tested): `Candidate` criteria
  normalized to 0..1 (returns capped at 15% APR, stability/liquidity /100, debt-reduction boolean),
  combined by a `Weights` profile into a weight-normalized `Score` with a per-criterion `Breakdown`
  (explainable, no black box), and `Rank` sorting highest-first (stable). Tests cover normalization +
  capping, equal-weight averaging, zero-weight safety, returns ordering, and debt-priority weighting.

- Built the **Allocate screen** (replaced the stub): assembles candidates from non-archived asset
  accounts (return/stability/liquidity) and interest-bearing liabilities ("Pay down …", guaranteed
  return), ranks them with one of four preset profiles, and renders a score bar + per-criterion
  breakdown per suggestion. Amount input + constraints (emergency buffer, max-per-destination) later.

- Started the **formula engine** `internal/formula` with the **tokenizer**: numbers (including
  leading-dot), identifiers, double-quoted strings, `+ - * / %` and `== != <= >= < >` operators,
  parens, commas; EOF sentinel; errors on unterminated strings, a lone `=`/`!` (no assignment), and
  unexpected characters. Table-driven tests cover arithmetic, calls, comparisons+strings, and errors.

- Added the formula **parser** (`Parse` → AST): recursive descent with a precedence ladder
  (comparison < additive < multiplicative < unary < primary), left-associative binaries, parens, and
  function calls (incl. empty/nested args). AST nodes are NumberLit/StringLit/Ident/Unary/Binary/Call.
  Tested via a canonical s-expr renderer covering precedence, calls, and malformed-input errors.

- Completed the formula engine with the **evaluator** (`Eval`): `Value` is only float64/string/bool
  (no host references); arithmetic + comparisons (numeric, plus string equality), variables resolved
  from `Env.Vars`, and the allow-list functions `sum/avg/min/max/count/abs/round/if` (variadic where
  apt, arity-checked otherwise; `if` truthiness over bool/number/string). Errors on unknown
  var/function, division/modulo by zero, and type mismatch. Tests: arithmetic, comparisons, every
  function, variable formulas (a savings-rate expression), and the error cases.

- Surfaced the engine in the **Customize screen** (replaced the stub): a live formula calculator over
  real figures — net worth/assets/liabilities + current-month income/expense (converted to major
  units by base-currency decimals) and account/transaction/member counts — with the result (or the
  engine's error message) updating per keystroke and an available-variables reference table.

- Added the **forecast engine** `internal/forecast` (pure, tested): `Project(start, recurring,
  oneTimes, months)` walks the horizon applying the recurring monthly net (`MonthlyNet`) plus any
  one-time events scheduled in each month, returning the end-of-month balance series; empty for a
  non-positive horizon. Tests cover recurring-only, a mid-horizon one-time, flat, and zero-horizon.

- Surfaced the forecast on **Planning**: a 12-month net-worth projection chart seeded from current
  `ledger.NetWorth` and this month's net cash flow (income − expense) as the recurring monthly figure,
  fed through `forecast.Project` into the `AreaChart` (toned red when the monthly net is negative),
  with a plain-English caption of the projected end value.

- Built the **Documents** screen (replaced the stub): paste-and-import transactions from CSV via a new
  `appstate.ImportTransactionsCSV` (wraps `store.TransactionsFromCSV`, best-effort: stores each valid
  row through the validated path, skips invalid, returns the count). Header-name column matching means
  any spreadsheet export works; AI PDF/receipt parsing is flagged as arriving with the OpenAI client.
  (Reminder logged: don't run native `go test` with `GOOS=js` still set — it silently "fails" fast.)

- Built the **AI codec** `internal/ai` (pure, tested): OpenAI chat request/response shapes plus
  `BuildRequest` (marshal a chat body), `ParseResponse` (assistant content, surfacing API errors and
  empty responses), and `ParseUsage` (token counts). Round-trip tests stand in for a mock transport;
  the browser `fetch` layer (sending with the user's key) is a thin js/wasm file added next.

- Wired AI end to end: a js `fetch` **transport** (`ai.SendChat`) that POSTs a chat request with the
  user's key and resolves the promise chain (`then(text) → ParseResponse`), releasing its `js.Func`s
  on both success and catch paths; and an **Insights** screen with "Explain my month" that builds a
  system+user prompt from live figures (`fmtMoney(net/income/expense)`), shows a Thinking…/error/
  result state, and prompts to add a key in Settings when absent. `Settings.OpenAIKey/OpenAIModel`
  already exist in the store; the codec stays pure/native-tested, the transport is js-only.

- Wired the **AI key/model to persist**: the global-settings key input (seeded from `Settings.OpenAIKey`)
  saves on each input and the model select (GPT-4o mini / GPT-4o) saves on change, both via
  `app.PutSettings`. Insights now has a real key to use. (In-memory store, so it survives the session;
  reload-persistence rides on the broader settings/storage work.)

- Added the pure **auto-categorization engine** `internal/rules`: a `Rule{Match, SetCategoryID,
  SetTags}` matched case-insensitively as a substring of payee+description, first-match-wins, with
  `Category`/`Tags`/`FirstMatch` helpers; empty matches never fire. Table-driven tested (case folding,
  ordering, tags, empty-match). The transaction entry/import flows can apply it to auto-fill category.

- Applied auto-categorization on **transaction entry** with zero new storage: built implicit
  `rules.Rule`s from the existing categories (Match = name → SetCategoryID = id) and, on each
  description keystroke, suggest a category via `rules.Category` only when none is chosen — so it
  helps without fighting the user. A real `Rule` store + management UI (custom patterns/tags) is later.

- Added **natural-language Q&A** to Insights ("Ask about your money"): a question box that sends the
  user's question plus a figures context (net worth / income / spending / active accounts) to the
  same `ai.SendChat`, sharing the loading/result/error states with "Explain my month". Shown only
  when a key is set. Richer data context (per-category, history) is a follow-up.

- Started **Phase 3 (PWA)** with the web manifest: `manifest.webmanifest` (name, standalone display,
  dark background/theme colors, scope/start_url, categories) linked from the host page along with
  `theme-color` and the apple-mobile meta tags, so CashFlux can be installed as a standalone app.
  Icons + a caching service worker (offline shell) are the follow-ups.

- Added a **service worker** (`sw.js`, registered best-effort on load): **network-first** so it never
  breaks the gwc live-reload (always fetches fresh, caches the response) yet serves the last good copy
  when offline; pre-caches the core shell (index/wasm_exec/main.wasm/manifest) on install, evicts old
  caches on activate, and only touches same-origin GETs so cross-origin OpenAI calls pass through.

- Added **GitHub Actions CI** (`.github/workflows/ci.yml`): on push/PR it sets up Go from `go.mod`,
  runs `go vet ./...`, `go test ./...`, and a `GOOS=js GOARCH=wasm` build. Verified locally that
  native `vet`/`test ./...` don't choke on the js-only view packages (Go skips build-constraint-
  excluded packages silently), so the workflow is green-by-construction. Activates once the repo is
  pushed to GitHub (the create-repo step is still pending — needs the owner's `gh` auth).

- Completed the transaction **filter set** with a member picker (combines with search/account/
  category/sort and the shared Clear). Date-range filter + persisting the last filter remain.

- Added a **From/To date-range** to the transaction filters (parsed via `dateutil.ParseDate`,
  inclusive bounds, ignored when blank/invalid), combining with the other filters and Clear.

- Added the **PWA install prompt**: capture `beforeinstallprompt`, reveal an "Install CashFlux"
  button (dark-themed, fixed bottom-right), call `prompt()` on click, and hide it after the choice or
  on `appinstalled`. With the manifest + service worker, the PWA install path is complete.

- Added a **"Repeat last"** transaction helper: finds the newest transaction and pre-fills the form
  (description, account, category, kind from the amount sign / transfer, abs amount formatted in the
  account currency), so logging a recurring purchase is one click + Add.

- Added a **"Net worth by member"** rollup to the Members screen via `ledger.NetByOwner` (each member
  plus a Group (shared) row, base currency, green/red toned), defaulting an absent owner to zero base.

- Added an **extra-payment scenario** to the debt-payoff calculator: an optional extra-monthly input
  runs `payoff.Project` a second time at `payment + extra` and reports the months saved and interest
  saved in plain English — the engine's first what-if surfaced.

- Added a **trim-spending what-if** to the net-worth forecast: an input re-runs `forecast.Project`
  with `monthlyNet + trim` and reports the improved 12-month figure and the difference. (Declared the
  `trimStr` state at the component top so the hook stays unconditional even though the forecast card
  is built inside `if app != nil`.)

- Added one-click **example chips** to the formula Customize screen (savings rate, spending ratio,
  gross assets, over-budget bool) that populate the input. Rendered as four explicit buttons (not a
  loop) so the inline `OnClick` hooks stay at stable positions.

- Added the **liability sub-form** to the Accounts add form: when the selected type is a liability
  (`AccountType.Class() == ClassLiability`), it reveals credit-limit / APR / min-payment / due-day /
  lender inputs (each a conditional `If(isLiab, …)` so hooks stay stable) and the add handler parses
  them onto the `Account`. This finally gives the Upcoming-bills widget and credit-utilization a real
  data source.

- Added **unfinished goals as allocation candidates** (stability 80 / liquidity 60, no return), so the
  Allocate ranking can suggest funding a goal alongside accounts and debts. ~100 features in this
  session: the candidate-C design, all Phase-1 core, deep Phase 2, and Phase-3 PWA all shipped.

- Added an **AI narrative to Allocate** ("Explain with AI"): builds a short prompt from the top-5
  ranked candidates + profile and runs `ai.SendChat`, reusing the loading/result/error pattern. Hooks
  (states + handler) are placed after the unconditional `ranked` so their order stays stable.

- Refreshed the **`CLAUDE.md` status** line (was stuck at "Phase 0 … Phase 1 not yet started") to
  reflect reality for future sessions: Phase 1 complete, Phase 2 engines+screens live, Phase 3 PWA +
  CI in, with the remaining work (sync, custom fields, vision AI, reload-persistent prefs) noted.

- Wired the global-settings **"+ Add member"** button (was a no-op) to close the flip panel (clear the
  settings target) and navigate to `/members`, via `router.UseNavigate` + the settings atom.

- Added the **allocation-attributes sub-form** for asset accounts (expected return APR, liquidity,
  stability), mirroring the liability sub-form (conditional `If(!isLiab, …)` inputs, parsed in the
  add handler's else branch). Now the Allocate engine scores asset candidates on real data, not zeros.

- Wrote a project **`README.md`** (the repo had none): tagline, feature highlights, stack, build/run
  commands, the pure-logic-vs-thin-wasm-UI architecture, and links to the other docs — the GitHub
  landing page for when the repo is pushed.

- Expanded the formula calculator's variable set with budget/goal/task counts, broadening what
  user formulas can reference.

**Next:** per-row duplicate, persist-last-filter, then more Phase 2 polish — as the loop continues.

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

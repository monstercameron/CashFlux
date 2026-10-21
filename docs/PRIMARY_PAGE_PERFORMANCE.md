# Primary page performance benchmark

`e2e/perf/primary-pages-benchmark.mjs` records warm SPA navigation and render
signals for Dashboard, Transactions, Accounts, Budgets, Goals, Todo, Notifications,
Assistant, and Reports. It complements `e2e/perf/perf-audit.mjs`: the existing audit
builds and serves the app, scores all routes, and updates versioned history; this
benchmark does not build, start, score, or rewrite that history.

## Run a capture

Start a local server that serves the already-built app you want to measure. The
benchmark requires a loopback HTTP URL and blocks requests to every non-loopback
host. It does not assume that port 8080 is the app, and it does not launch a server.
The browser context is new for each capture, so the app's sample seed and IndexedDB
are disposable and cannot reuse the normal browser profile. The run stops unless the
sample-data banner appears after app readiness. Before app startup, the isolated
context selects the all-dates transaction filter, so the Transactions route processes
the full sample ledger instead of only the current month.

From the repository root, in PowerShell:

```powershell
node e2e/perf/primary-pages-benchmark.mjs `
  --base-url http://127.0.0.1:8099 `
  --label before `
  --wasm-file C:\path\to\before-main.wasm `
  --build-id ACTUAL_WASM_SHA256 `
  --warmups 1 `
  --runs 7 `
  --out e2e/perf/results/primary-before.json
```

After changing/building the app served at that local URL, save the second capture
under a different file:

```powershell
node e2e/perf/primary-pages-benchmark.mjs `
  --base-url http://127.0.0.1:8099 `
  --label after `
  --wasm-file C:\path\to\after-main.wasm `
  --build-id ACTUAL_WASM_SHA256 `
  --warmups 1 `
  --runs 7 `
  --out e2e/perf/results/primary-after.json
```

Use the same machine, browser, server mode, build flags, viewport, sample-data
version, and run counts for both captures. Close other CPU-heavy work where practical.
Use `Get-FileHash PATH -Algorithm SHA256` on each frozen artifact and pass that path
as `--wasm-file` together with its hash as `--build-id`. The benchmark verifies the
WASM header and SHA256 before launching the browser. The isolated page receives
the artifact through the app's gzip fallback; the served file is not replaced.
This avoids Playwright's 100 MiB identity response limit. The report records the
delivery path and rejects unexpected fallback attempts. Startup measurements from
this path are not normal server-negotiated startup measurements.
Without these paired flags, it measures the server's artifact and records no
verified artifact identity. It does not infer source provenance from Git HEAD. Record dirty source
changes alongside it when the build predates their commits.
Both benchmark scripts block loopback WebSocket connections inside their own
contexts, so the live dev server cannot reload a frozen artifact mid-capture.
The user's browser and the dev server retain their normal live reload behavior.
The warm-up sweep is discarded; each measured sweep visits all nine pages in order,
ending at Reports before the next sweep returns to Dashboard. Raw per-sweep samples
are preserved beside route medians in the JSON. `--help` prints the options. Without
`--out`, JSON goes to stdout; the run summary goes to stderr-style console output.

## Captured signals

Assistant navigation waits for the populated sample Insights rail on both
artifacts. An earlier fixed delay could sample the deferred rail before it was
mounted, making incomplete Assistant content appear faster. Captures retain
normalized main text, its SHA256 and raw samples; inconsistent visible content
within a build fails the capture. Inspect content differences between builds
before comparing timings. Route failures and missing samples also return a
nonzero exit status.
Startup failures retain bounded page errors, failed requests, HTTP error responses
and loader-state diagnostics. A failed startup produces no route latency result.

For representative interaction checks, use
`e2e/perf/primary-interactions-benchmark.mjs` with the same artifact, hash, URL,
warmup, run and output flags. Each sweep has its own isolated sample context.
Required actions cover period stepping, transaction search and date filters,
account detail expansion, budget period and income-basis dialog, goal tabs/sort/
expansion, To-do views/search, populated notification triage, local Assistant
controls/typing and Reports views/export dialog/scope. No Assistant prompt is
submitted. Missing or failed actions return a nonzero exit status. It retains
per-action timings and diagnostics; a one-sweep pass is functional evidence,
not a latency comparison.

Optional `--profile-dir DIR --profile-route /` records a Chromium CPU profile
for each measured visit to that route. Warmups are unprofiled. Profiling captures
are diagnostic and excluded from reported performance medians; compare timings
with separate unprofiled runs.

The file includes the app-ready boot time; per-route median mount and settled times;
long-task count, total blocking time (time above 50 ms), and longest task; layout
shift; DOM element and rendered-row counts; and raw samples. It also records the
browser/runtime, OS, CPU count, viewport, device scale, fixed clock, locale, timezone,
sample-workspace confirmation, and `conditions.backgroundWork` when the caller sets
`PRIMARY_BENCH_BACKGROUND_WORK`. Record active builds or other heavy work there so
the two captures' machine load is visible alongside the timing data.

Newer builds expose a fixed-schema `window.__cashfluxBootTimings` record, captured
as `boot.goPhases` when present (otherwise `null`). Schema version 2 adds fixed
duration-only `details` for app-state initialization, workspace preparation,
artifact storage, hydration, legacy unlock waiting, undo baseline and audit-feed
restoration. Details subdivide their enclosing phase and must not be added to the
phase totals. No workspace, account or dataset values are recorded.
Its total starts at Go `Run`,
so it excludes WASM fetch/decode/compile. Phase durations separate protected
storage, hydration, sign-in bootstrap, recurring covers, mounting and boot wiring.
`postMountNotificationScheduleMs` measures scheduling only; notification generation
continues asynchronously. The record contains durations only, with no financial
content or identity fields. Readiness still follows the existing boot contract.

The `conditions.dataSize.transactions` value is the all-dates filtered ledger count
shown by the transaction table. In the baseline capture, that was 2,808 transactions;
the page rendered 25 row-like nodes because the table pages and virtualizes its rows.
Account, budget, goal, task, and notification counts are visible route entities, so
pagination or collapsed sections can hide records. General element and row-like
counts are render-weight proxies for all nine routes, including Assistant and
Reports where a stable entity selector is not available. If a route fails, the JSON
keeps the error and continues measuring the remaining routes; a startup failure also
writes a failure report when `--out` is supplied.

## Compare captures

Compare matching routes and signals in the two JSON files. A useful summary is the
change in `mountMsMedian`, `settledMsMedian`, and `longTaskTotalMsMedian`, with the
raw samples checked for spread and `mainElementCountMedian` / rendered-row counts
checked to confirm comparable content. A percentage change can be calculated as
`(after - before) / before * 100`; negative values mean less time or fewer elements.

These are descriptive local measurements, not latency pass/fail gates. Do not treat
one capture or small differences as evidence of a regression or improvement; repeat
captures if results are noisy. The browser uses reduced motion and blocks external
requests, so these numbers are intended for before/after comparisons under this same
setup, not as production-user latency estimates.

## Capture comparison: primary page refactor

The baseline WASM was built from `48e8f7ea90c76013a852a4c866b634ceb0fb2e8f`. The
current artifact was built with first-pass source changes present in the working
tree and subsequently committed through `e3e508a5`; its SHA256 is
`f6677c310d5513744f257a0cc4e0c43b43d8eefd89d0ae32d512d3f916a595e7`
(108,549,380 bytes). The original capture label includes `dde098e`, which was the
HEAD when the dirty build was made; that commit alone cannot reproduce it. These
captures predate the optional `--build-id` metadata field. Both used Go 1.26.9,
Node 26.2.0, Playwright 1.61.0, Chromium 149.0.7827.55, Windows ARM64, a 1440×900
viewport, and the same isolated sample. Each capture had one warm-up sweep and seven
measured sweeps. The paired repeat ran without Go builds or tests detected. Counts
matched at 2,808 transactions, 15 accounts, 10 budgets, 5 goals, 20 tasks, and no
individual items visible in Notifications’ default “Needs you” view (not the total
feed across triage tabs).

| Route | Settled before | Settled after | Change |
| --- | ---: | ---: | ---: |
| Dashboard | 703 ms | 717 ms | +2.0% |
| Transactions | 748 ms | 796 ms | +6.4% |
| Accounts | 1,020 ms | 672 ms | −34.1% |
| Budgets | 461 ms | 318 ms | −31.0% |
| Goals | 216 ms | 125 ms | −42.1% |
| Todo | 203 ms | 168 ms | −17.2% |
| Notifications | 113 ms | 82 ms | −27.4% |
| Assistant | 131 ms | 121 ms | −7.6% |
| Reports | 350 ms | 354 ms | +1.1% |

Accounts and Reports also had 54% and 21% lower median long-task blocking time in
the repeat pair. Transactions was 4% higher. The first before/after capture pair
gave different directions for Dashboard, Transactions, Budgets, Goals, and Reports;
the raw repeat samples also include wide spreads, including one 5.3-second Accounts
settle in the baseline. Treat the measurements as evidence of likely gains on several
routes, with Dashboard, Transactions, and Reports still inconclusive. The Assistant
DOM count fell from 354 to 298 elements, so that route rendered less content in the
current capture. Full JSON samples are in the task's `work/` directory:
`primary-baseline.json`, `primary-current-1.json`, `primary-baseline-2.json`, and
`primary-current-2.json`.

These captures measure route navigation only. They do not establish typing,
filtering, expanding a populated notification feed, or CRUD latency improvements.
Cold app readiness was approximately 7.6 seconds in the repeat current capture;
it remains a separate performance concern. Subsequent source changes need a fresh
build and capture before the table above can describe them.

## Capture comparison: final pass-two artifact

After the second-pass fixes, the baseline artifact from `48e8f7ea` and the final
second-pass working-tree artifact were measured on the same fixed servers (`:8142`
and `:8143`). The current WASM includes the timer-origin correction and the
`RunwayMonths` integer-width fix. The baseline SHA256 is
`e134f4cf64ae3fc1dd0aa77ca0967c01a4ec9df52cd1bf655bc8d11b86344186`; the current
SHA256 is `37d895c4a8093a13a2397ee0bfde0ed047da89b956c75ad4bb666791bf21e02b`.
The hashes identify the measured artifacts; the current source state was a working
tree build and is not identified by a Git commit alone.

Both captures used Go 1.26.9, Node 26.2.0, Playwright 1.61.0, Chromium
149.0.7827.55, Windows ARM64, a 1440×900 viewport, the fixed July 1, 2026 clock,
one warm-up sweep, and seven measured sweeps. Each confirmed a fresh isolated sample
context. Counts and median DOM sizes matched on every route; the sample had 2,808
transactions, 15 accounts, 10 budgets, 5 goals, 20 tasks, and zero individual items
in the default Notifications view. Neither capture had a route failure.

| Route | Settled before | Settled after | Change |
| --- | ---: | ---: | ---: |
| Dashboard | 682 ms | 607 ms | −11.0% |
| Transactions | 813 ms | 635 ms | −21.9% |
| Accounts | 746 ms | 653 ms | −12.5% |
| Budgets | 446 ms | 301 ms | −32.5% |
| Goals | 155 ms | 139 ms | −10.3% |
| Todo | 159 ms | 154 ms | −3.1% |
| Notifications | 78 ms | 81 ms | +3.8% |
| Assistant | 132 ms | 92 ms | −30.3% |
| Reports | 306 ms | 287 ms | −6.2% |

The app-ready time was 10,018 ms in the baseline and 8,213 ms in the current
capture. The current artifact reported `boot.goPhases.totalMs` of 6,466 ms, including
4,060 ms for workspace state and hydration, 848 ms pre-mount preparation, 684 ms
entry-to-styles, 362 ms protected storage, 240 ms post-mount wiring, and 35 ms router
mount. The baseline artifact predates this phase instrumentation, so its `goPhases`
field is null and phase-by-phase comparison is unavailable.

These measurements overlap an unrelated single-worker Playwright run
(`sv0_artifact_sync.spec.mjs`) in another task. Processor samples before the pair
ranged from 46% to 91%; during the baseline capture they were 24–26%, and during the
current capture they were 10–22%. Two source-edit-only tasks were also active. The
JSON records this background work. The lower medians on eight routes are useful
directional evidence, but changing machine load means they do not establish clean,
causal percentage improvements; Notifications was 3.8% slower. No extra pair was run.

The Notifications entity selector measures rows in the active default “Needs you”
view, not every item across all triage tabs. A separate sample-only inspection at the
same clock confirmed that this view had no notification rows or group cards and
showed “You're all caught up — nothing needs you right now.” It also displayed a
separate “Recent wins” section. Thus the recorded zero describes an empty default
attention queue, not necessarily an empty notification archive or the Watching tab.

The exact captures and console logs are in the task `work/` directory:
`primary-baseline-pass2-final.json`, `primary-current-pass2-final.json`, and the
matching `.log` files. Commands, run from the repository root in PowerShell (with
`PRIMARY_BENCH_BACKGROUND_WORK` set to the contention note recorded in each JSON):

```powershell
node e2e/perf/primary-pages-benchmark.mjs --base-url http://127.0.0.1:8142 --label before-48e8f7e-pass2-final --build-id e134f4cf64ae3fc1dd0aa77ca0967c01a4ec9df52cd1bf655bc8d11b86344186 --warmups 1 --runs 7 --out C:\Users\mreca\Documents\Codex\2026-10-19\fin\work\primary-baseline-pass2-final.json
node e2e/perf/primary-pages-benchmark.mjs --base-url http://127.0.0.1:8143 --label after-37d895c-pass2-final --build-id 37d895c4a8093a13a2397ee0bfde0ed047da89b956c75ad4bb666791bf21e02b --warmups 1 --runs 7 --out C:\Users\mreca\Documents\Codex\2026-10-19\fin\work\primary-current-pass2-final.json
```

## Pass-three verification

The canonical goal models and lazy Reports sections were verified with the full
native suite and 817 WASM screen tests. One existing asynchronous Import test
needed a bounded wait for the visible Review panel rather than assuming a 10 ms
transport delay also completed the render; the complete rerun passed.

The rebuilt artifact had SHA256
`b39389a35392aacce2408a1f5d2dcc424e0cbf5e1c984042347971ee1a260f34`.
Its production source was subsequently committed through `239d2669`; the later
sample payoff-baseline optimization is not in this artifact. Five focused browser
cases passed with one Chromium worker, no retries, fixed sample data and external
DNS blocked: Dashboard Focus presets, an unusual-charge notification deep link,
Goals alphabetical sorting, Reports Summary/Full and saved-view actions, and the
all-routes smoke sweep. The log is `work/primary-pass3-focused.log` in the task
workspace. This was a functional verification run, not another latency capture.

## Pass-five verification

The next integrated batch passed all native packages and 835 WASM screen tests.
The screen rerun replaced another immediate fake-fetch assertion with a bounded
wait for the visible Import Review panel; the initial run failed only that
parent test and its PDF/image/receipt subtests. A subsequent six-case focused
history/card run also verified the added FX-adjusted deadline-plan fixture.
Seventeen journal/recovery WASM tests passed after the prepared-import and undo
record-key changes, and six contribution app-host tests covered atomic retry,
Undo/Redo, workflow effects and conflict retention.

Six sample-only browser cases passed on the existing port 8080 live server with
one Chromium worker, no retries and external DNS blocked: Accounts details and
notes, Goals alphabetical order and allocation validation, Dashboard goal counts,
Reports Summary/Full and saved views, and substantive smoke checks for 47 routes.
The local and served artifact hashes before and after the browser run matched:
`9bf0df25055acc89e3985c0821abf07013cee853d01b1a50a8a12c9fdb736130`.
This identifies the frozen working-tree artifact used for that functional run;
subsequent snapshot, net-worth and notification work is not verified by it.

Evidence in the task workspace: `work/primary-native-pass5-final.log`,
`work/primary-screens-pass5-fixed.log`,
`work/primary-goal-history-canonical-final.log`,
`work/primary-prepared-recovery-pass5.log` and
`work/primary-pass5-focused.log`. These are correctness checks, not a fresh
latency comparison or proof that the entire nine-page objective is complete.

## Native transaction-filter preparation benchmark

The prepared matcher was compared with the frozen prior matcher retained in the
parity tests. This isolates predicate preparation/matching and widened preset
counts; it does not measure WASM, sorting, DOM updates or typing-to-paint latency.
The synthetic rows and criteria are defined in
`internal/txnfilter/compiled_bench_test.go`. The command used Go 1.26.9 targeting
Windows AMD64 on an ARM64 host, with one benchmark CPU, 25 iterations and three
samples per case (the target is recorded in the raw benchmark header):

```powershell
go test -work ./internal/txnfilter -run '^$' -bench '^BenchmarkCompiledCriteriaPreparation$' -benchtime=25x -count=3 -cpu=1 -benchmem
```

For 10,000 rows, median elapsed time and allocations per iteration were:

| Case | Prior parsing | Prepared | Allocations, prior → prepared |
| --- | ---: | ---: | ---: |
| Plain search | 3.377 ms | 0.839 ms | 30,016 → 10,018 |
| Numeric search | 3.005 ms | 2.250 ms | 20,014 → 20,014 |
| Multiple facets | 7.760 ms | 1.741 ms | 85,016 → 10,036 |
| Widened preset counts | 25.170 ms | 6.044 ms | 230,066 → 30,096 |

The same run includes 1,000-row cases. Raw results are in the task workspace at
`work/primary-txnfilter-preparation-bench.log`. Other development work was active
on the machine, so elapsed values are local observations rather than browser
performance guarantees. The allocation reduction is direct evidence that the
repeated parsing work was removed; numeric parsing primarily reduces CPU work.

## Native undo snapshot key preparation

The typed snapshot converter now resolves direct row-key fields once per
collection instead of decoding every newly encoded row just to find its key.
Unknown custom encoders, embedded fields, ambiguous tags and case variants keep
the wire-JSON fallback. Exact snapshot bytes, device-key redaction, receipts,
nil/empty collections and duplicate-key errors remain covered by parity tests.
Six actual app-host snapshot cache tests also passed.

A three-sample, one-CPU benchmark of 3,000 transactions compared the legacy
export/parse path, the prior typed converter and the optimized typed converter:

| Path | Median time | Allocations per operation | Bytes per operation, approximately |
| --- | ---: | ---: | ---: |
| Legacy export and parse | 34.33 ms | 39,116 | 8.58 MB |
| Prior typed converter | 23.03 ms | 45,187 | 6.90 MB |
| Typed converter with prepared keys | 13.74 ms | 24,229 | 6.14 MB |

This ran with Go 1.26.9 targeting Windows AMD64 on an ARM64 host while other
functional development work was active. It excludes the SQLite snapshot read
and does not measure startup or browser navigation. The reduction in allocations
is direct evidence of removed row decoding; elapsed times are local observations.
Raw output is in the task workspace at `work/primary-snapshot-key-bench.log`.
The reproducible benchmark is `BenchmarkSnapshot3000Transactions` in
`internal/undosnap/dataset_snapshot_bench_test.go` (`-count=3 -cpu=1 -benchmem`).

## Native account history preparation

Net-worth and explicit portfolio series prepare account-to-row positions once,
then visit each account's source-order rows for every cutoff. They no longer
copy the complete transaction structs for every cutoff or scan every unrelated
account's rows. They preserve account iteration order, source addition order,
strict cutoff comparisons, checked intermediate overflow, error order, duplicate
account records and per-account FX rounding. Adversarial cases and 400 seeded
scenarios compare both selection policies with the frozen prior implementation.

Three samples per case with one benchmark CPU gave these local medians:

| Fixture | Prior series | Indexed series | Allocated bytes per operation, prior → indexed |
| --- | ---: | ---: | ---: |
| 24 accounts, 24,000 rows, 8 cutoffs | 80.46 ms | 3.715 ms | 87,556,416 → 531,136 |
| 80 accounts, 60,000 rows, 12 cutoffs | 709.6 ms | 11.624 ms | 328,338,528 → 1,041,376 |

Small allocation counts increase (17 → 264 and 61 → 811) because index slices
grow per account, while total allocated bytes fall substantially. Candidate row
visits calculated from the fixtures fall from 4,072,800 to 192,000 and from
47,709,000 to 681,000. This isolates native series computation, not browser
navigation. Go 1.26.9 targeted Windows AMD64 on an ARM64 host; other development
work was active. Raw results and the contention note are in the task workspace
at `work/primary-account-value-series-bench.log`; the benchmark is
`BenchmarkAccountValuePreparation` (`-count=3 -cpu=1 -benchmem`).

## Pass-seven integration and timing capture

The full native suite and 845 WASM screen tests passed after account eligibility,
source-order series preparation, snapshot keys, and lazy layout/widget seeds.
Three sample-only browser cases passed on the existing 8080 live server:
notification keyboard tabs, To-do List/Board/Calendar parity, and substantive
smoke checks for all 47 routes. Chromium used one worker, no retries, a fixed
sample clock and blocked external DNS; the temporary runner config was removed.

One warm-up and seven measured sweeps across all nine pages completed without
failures. The local and served artifact hashes before and after were identical:
`3271503a3043bb419487db0e910c08f1b61b1e667338127b14026a28b8316a6b`.
This is the Go source through `6e76fc3d`; subsequent documentation commits do not
change that artifact. Entity counts match both pass-two captures: 2,808
transactions, 15 accounts, 10 budgets, 5 goals and 20 tasks. The default
notification-view metric remains zero with the caveat above. Descendant element
counts match on eight routes; Assistant now has 354 elements rather than 298.

| Page | Pass-two current settled median | Pass-seven settled median |
| --- | ---: | ---: |
| Dashboard | 607 ms | 811 ms |
| Transactions | 635 ms | 882 ms |
| Accounts | 653 ms | 664 ms |
| Budgets | 301 ms | 464 ms |
| Goals | 139 ms | 189 ms |
| To-do | 154 ms | 220 ms |
| Notifications | 81 ms | 133 ms |
| Assistant | 92 ms | 181 ms |
| Reports | 287 ms | 375 ms |

These observed values are slower. The captures were not taken under the same
load, and Assistant content also changed, so they establish neither a causal
code regression nor a latency improvement. During pass seven there were no
root/agent Go jobs, but the live dev poller and uncontrolled desktop/backend work
remained active. No additional runs were made to seek better numbers. The goal
of demonstrating lower latency on every page remains unverified; native work and
allocation reductions above have narrower, independently measured evidence.

App-ready was 8,779 ms. The nested startup record has schema version 2 and
7,314 ms inside Go, including 4,659 ms for app state/workspace/hydration.
Detail durations include app state initialization 2,565 ms, dataset hydration
1,983 ms and undo baseline 528 ms. Details are subdivisions of enclosing phases,
not extra time to add to their totals.

Task workspace evidence: `work/primary-native-pass7.log`,
`work/primary-screens-pass7.log`, `work/primary-pass7-focused.log`,
`work/primary-current-pass7.json` and `work/primary-current-pass7-run.log`.

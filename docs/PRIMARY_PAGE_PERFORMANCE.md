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
  --build-id ACTUAL_WASM_SHA256 `
  --warmups 1 `
  --runs 7 `
  --out e2e/perf/results/primary-after.json
```

Use the same machine, browser, server mode, build flags, viewport, sample-data
version, and run counts for both captures. Close other CPU-heavy work where practical.
Use `Get-FileHash web/bin/main.wasm -Algorithm SHA256` on the artifact being served
and pass its hash as `--build-id`. The benchmark records this caller-supplied ID;
it does not infer source provenance from the current Git HEAD. Record dirty source
changes alongside it when the build predates their commits.
The warm-up sweep is discarded; each measured sweep visits all nine pages in order,
ending at Reports before the next sweep returns to Dashboard. Raw per-sweep samples
are preserved beside route medians in the JSON. `--help` prints the options. Without
`--out`, JSON goes to stdout; the run summary goes to stderr-style console output.

## Captured signals

The file includes the app-ready boot time; per-route median mount and settled times;
long-task count, total blocking time (time above 50 ms), and longest task; layout
shift; DOM element and rendered-row counts; and raw samples. It also records the
browser/runtime, OS, CPU count, viewport, device scale, fixed clock, locale, timezone,
sample-workspace confirmation, and `conditions.backgroundWork` when the caller sets
`PRIMARY_BENCH_BACKGROUND_WORK`. Record active builds or other heavy work there so
the two captures' machine load is visible alongside the timing data.

Newer builds expose a fixed-schema `window.__cashfluxBootTimings` record, captured
as `boot.goPhases` when present (otherwise `null`). Its total starts at Go `Run`,
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

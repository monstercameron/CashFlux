# Primary-page browser evidence — 2026-10-21

These are descriptive local captures, not production latency guarantees. The
shared machine ran other Codex work; this task kept Go builds/tests idle during
the measured sweeps. Raw observations, content and environment are retained.

## Artifacts and method

| Capture | Production Go source | Main WASM SHA256 |
| --- | --- | --- |
| Original baseline | `48e8f7ea90c76013a852a4c866b634ceb0fb2e8f` | `e134f4cf64ae3fc1dd0aa77ca0967c01a4ec9df52cd1bf655bc8d11b86344186` |
| Immediately before shared SMART results | `eb553074` | `3129b60648c8f3259ceedc2da91c5e2363ac7996b61ec9f031dec50c58413510` |
| After shared SMART results | through `22dca667` | `2965dc389a2b18b20774ca4f9b84598fb10f969bae1272d2fcbf55cc7fa51091` |

The after artifact was copied from a successful dev build before its commit;
the listed commit identifies its production Go sources, not its embedded VCS
metadata. Copies were hashed before, during and after copying. The benchmark
verifies the frozen WASM header and SHA256, then injects it into a fresh isolated
sample context through the existing gzip fallback. It does not replace the
server's artifact. The same served services module is used; sample calculations
run in main. External traffic and service workers are blocked.

All navigation captures use 1440×900, reduced motion, July 1, 2026 at noon UTC,
America/New_York, en-US, one discarded warmup and seven measured sweeps. The
Assistant completion contract requires its populated sample rail, including in
the old baseline. The paired before/repeat-after captures block dev reload
WebSockets. The earlier successful after capture had no source changes during
its run. The scripts and full method are documented in
[Primary page performance](../../PRIMARY_PAGE_PERFORMANCE.md).

## Warm route navigation

The directly paired captures are [before](smart-before.json) and
[repeat after](smart-after-repeat.json). A separate earlier successful
[after capture](smart-after-first.json) is retained, including its slower and
noisier observations. [Comparison JSON](comparison.json) keeps the raw settle
values alongside the medians below.

| Route | Before median | After median | Change |
| --- | ---: | ---: | ---: |
| Dashboard | 727 ms | 508 ms | −30.1% |
| Transactions | 847 ms | 528 ms | −37.7% |
| Accounts | 692 ms | 541 ms | −21.8% |
| Budgets | 400 ms | 335 ms | −16.2% |
| Goals | 153 ms | 126 ms | −17.6% |
| To-do | 205 ms | 160 ms | −22.0% |
| Notifications | 94 ms | 90 ms | −4.3% |
| Assistant | 717 ms | 690 ms | −3.8% |
| Reports | 371 ms | 428 ms | +15.4% |

All nine routes have exact normalized visible-text parity between the immediate
before and both successful after captures. Counts also match. The earlier after
capture had Reports at 438 ms and Notifications at 99 ms; the repeat does not
erase that variance. Reports remains a measured performance concern. Small
changes on Notifications and Assistant are not strong causal evidence. These
results do not establish that every latency on all nine pages improved.

The [original baseline](original-baseline.json) retains the earlier seven-sample
reference. Its visible text matches the new artifact on seven routes. The Budget
headline changed because selected-period spending now excludes an annual-budget
charge from another period; its annual card still retains that charge. Assistant's
context estimate changed because its supplied context expanded. Do not claim
exact text parity for those two original-baseline routes.

## Startup and failures

Forced-fallback app-ready observations were 6,586 ms before, 11,879 ms in the
first successful after capture and 7,806 ms in the repeat after. Each is a single
startup observation. They do not establish a cold-start improvement, and this
delivery path is not normal server-negotiated startup.

The [initial after capture](startup-failure.json) timed out before requesting
main WASM and has no route results. Its cause was not established. It is retained
as failed evidence; the subsequent successful runs are distinct captures.

## Correctness evidence for the optimization

A separate Dashboard CPU profile found strip and peek each calling the same
deterministic SMART engine. The shared result cache retains exact prepared time,
app/data identity, week start, page, full settings and language, checks store
readiness before reuse and returns owned copies. Native ownership coverage and
four focused WASM tests passed. The mounted test proves one engine pass for both
surfaces, reuse when opened, one fresh shared pass after a real revision event,
and a distinct page-scoped pass. These tests establish removed duplicate work;
the browser numbers above establish only the observed timing effects.

Broader correctness residuals and the requirement checklist remain in
[App componentization](../../APP_COMPONENTIZATION.md). This evidence does not
mark that work complete.

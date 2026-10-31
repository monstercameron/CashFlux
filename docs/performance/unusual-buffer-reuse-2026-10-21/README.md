# Unusual history buffer reuse

Fixture: `jobFixture(3300)`, completing each detector job with `Step(32)`.
Both captures used Go 1.26.9, windows/amd64 on an ARM64 Windows host, a private
build cache, and the same benchmark command recorded in each raw log. Five
samples were captured per version. Other work on this machine caused substantial
elapsed-time variation; these are native detector measurements, not browser
latency measurements.

| Median | Before | After |
| --- | ---: | ---: |
| Allocated bytes | 456,729,800 | 61,586,952 |
| Allocations | 1,051,743 | 1,028,231 |
| Elapsed | 998.102 ms | 336.375 ms |

Allocation volume fell about 86.5%. The after timing samples ranged from 247.7
to 1,185.6 ms, so the elapsed medians are weaker evidence.

The change retains history-buffer capacity between candidates. The price job
finishes using the merchant history before the next candidate resets its length.
No scoring or evidence policy changes. Native reference parity across work
budgets and focused price-policy tests pass. The unchanged 3,300-row timing gate
passed with 18,568 slices, a longest slice of 38.4098 ms, and 183 findings.

Earlier Smart/detector timing failures overlapped a separate coverage-heavy Go
job. Quiet isolated repetitions passed 10/10 Smart and 5/5 detector runs, without
changing their 50 ms thresholds.

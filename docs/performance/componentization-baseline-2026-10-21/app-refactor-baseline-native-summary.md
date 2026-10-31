# Native pure-model benchmark capture summary

Source and environment details are in `app-refactor-baseline-native-metadata.txt`; full command output is retained in the three adjacent `.log` files. Captures ran sequentially with no browser/server jobs. All three `go test` commands exited 0 with `-work`; Go retained the temporary build work directories shown in each log.

HEAD: `d79e1b90061948e4af2c1cc94c359a97e4682da6`. Working tree had one untracked file: `docs/ASSISTANT_VALIDATION.md`; no source edits were made for this capture. Go 1.26.9; Windows host reports ARM64, but `GOARCH=amd64` and benchmark binaries reported `Virtual CPU @ 4.45GHz`. Comparisons should be repeated on the same target for useful before/after evidence; these numbers are not physical CPU throughput claims.

## Median of five samples

| Benchmark | Fixture | Median | Median allocations |
|---|---:|---:|---:|
| `BenchmarkTxnFilterSort3300/derive` | 3,300 transactions | 13.29 ms/op | 1,535,812 B/op; 3,326 allocs/op |
| `BenchmarkTxnFilterSort3300/memo-hit` | 3,300 transactions | 95.82 ns/op | 0 B/op; 0 allocs/op |
| `BenchmarkFundingSuggestions/repeated-ledger-scan` | model fixture | 3.397 s/op | 0 B/op; 0 allocs/op |
| `BenchmarkFundingSuggestions/shared-index` | same fixture | 1.455 ms/op | 279,571 B/op; 144 allocs/op |
| `BenchmarkSortClosest/scan-comparator` | goal sorting fixture | 837.08 ms/op | 133,980,104 B/op; 16,424 allocs/op |
| `BenchmarkSortClosest/indexed` | same fixture | 2.06 ms/op | 412,977 B/op; 630 allocs/op |
| `BenchmarkSnapshotEntityLookup/prepared` | snapshot fixture | 0.971 ms/op | 51,016 B/op; 127 allocs/op |
| `BenchmarkSnapshotEntityLookup/previous-per-row-search` | same fixture | 20.32 ms/op | 0 B/op; 0 allocs/op |
| `BenchmarkSelect/100/single-pass` | 100 tasks | 71.46 µs/op | 28,576 B/op; 201 allocs/op |
| `BenchmarkSelect/100/previous-multiple-passes` | 100 tasks | 245.76 µs/op | 248,851 B/op; 226 allocs/op |
| `BenchmarkSelect/1000/single-pass` | 1,000 tasks | 595.88 µs/op | 277,570 B/op; 2,001 allocs/op |
| `BenchmarkSelect/1000/previous-multiple-passes` | 1,000 tasks | 3.139 ms/op | 2,873,542 B/op; 2,038 allocs/op |
| `BenchmarkSelect/10000/single-pass` | 10,000 tasks | 5.471 ms/op | 2,726,533 B/op; 20,001 allocs/op |
| `BenchmarkSelect/10000/previous-multiple-passes` | 10,000 tasks | 28.06 ms/op | 40,072,972 B/op; 20,062 allocs/op |

The paired `scan`/`shared-index`, `scan-comparator`/`indexed`, and `previous-*` cases are internal algorithm comparisons in existing benchmarks, not changes made in this task and not user-visible app before/after measurements. The transaction `derive`/`memo-hit` split is also model-level; browser route and interaction timings remain unmeasured.

## Commands

```powershell
go test -work -run '^$' -bench '^BenchmarkTxnFilterSort3300$' -benchmem -count=5 ./internal/screens
go test -work -run '^$' -bench '^(BenchmarkSortClosest|BenchmarkFundingSuggestions)$' -benchmem -count=5 ./internal/goalview
go test -work -run '^$' -bench '^(BenchmarkSelect|BenchmarkSnapshotEntityLookup)$' -benchmem -count=5 ./internal/taskview
```

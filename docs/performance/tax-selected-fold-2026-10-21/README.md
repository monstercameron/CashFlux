# Tax net-income native performance evidence

Cold native preparation only. Five samples per primary-row count, the same synthetic FX/split/refund fixture and command before and after. Refund counterparts are additional rows. Fixture construction is outside the timer.

Environment: Go 1.26.9 launcher reports windows/arm64; the benchmark target is windows/amd64 and reports Virtual CPU @ 4.45GHz, 18-way default concurrency. These are emulated/virtualized native measurements, not browser/WASM measurements. Both runs used the same private GOCACHE. Timing was highly variable despite serialized benchmark lanes; background host contention was not eliminated.

Command: `go test -work ./internal/taxgather -run ^$ -bench ^BenchmarkNetIncomeWithLinks$ -benchmem -benchtime=300ms -count=5`

| Primary rows | Before median ms | After median ms | Time change | Before median B/op | After median B/op | Bytes change | Before median allocs/op | After median allocs/op | Allocations change |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1,000 | 6.116 | 5.022 | -17.9% | 2,169,534 | 1,657,881 | -23.6% | 6,263 | 1,657 | -73.5% |
| 10,000 | 37.296 | 19.979 | -46.4% | 23,204,505 | 17,070,811 | -26.4% | 62,195 | 16,181 | -74.0% |
| 100,000 | 410.844 | 780.437 | +90.0% | 238,501,160 | 170,036,808 | -28.7% | 621,367 | 161,345 | -74.0% |

All separate captures are retained, including the slower 100k after result. The supported performance conclusion is approximately 74% fewer allocations and 24–29% fewer allocated bytes on these fixtures. A reliable wall-time speedup is not established.

## Bounded timing diagnostic

The 100k regression prompted one additional same-process comparison. The old full-gather composition remains a test-only parity reference. Each sample runs six operation pairs over the identical fixture, alternating which implementation executes first. Three samples per size. `gather-ns/op` and `selected-ns/op` are per-preparation time metrics; standard ns/op and allocation metrics on these lines cover the entire pair.

Command: `go test -work ./internal/taxgather -run ^$ -bench ^BenchmarkNetIncomeMatched$ -benchmem -benchtime=6x -count=3`

| Primary rows | Reference median ms | Selected median ms | Observed change |
| ---: | ---: | ---: | ---: |
| 10,000 | 82.292 | 59.779 | -27.4% |
| 100,000 | 810.962 | 608.253 | -25.0% |

The diagnostic favors the selected implementation in five of six paired samples, but one 100k sample is slower and absolute times vary substantially. It does not erase the separate-run regression or establish an end-to-end app improvement.

Raw logs: [before](before.txt), [after](after.txt), [matched diagnostic](matched.txt). All commands exited 0. No browser build, server, paid call or real household data was used.

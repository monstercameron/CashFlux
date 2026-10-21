# Repeated Settings reads

Controllers now share a successful Settings snapshot for the current store
revision. Every read still checks store readiness and returns independently owned
maps, slices and pointer values. Direct store writes invalidate the snapshot;
failed loads are retried, and a closed store remains an error after a warm hit.

The benchmark compares nine prior SQL/JSON reads with nine checked snapshot reads
per operation. Both paths return owned settings, including eight FX rates and
timestamps, three freshness overrides, twelve AI usage buckets and the two
optional pointer records. Initialization and the first read are outside timing.

| Metric, median of five samples | SQL/JSON | Checked snapshot |
| --- | ---: | ---: |
| Time per nine reads | 348,313 ns | 47,421 ns |
| Bytes per nine reads | 73,800 | 21,816 |
| Allocations per nine reads | 855 | 198 |

This is about 70% fewer allocated bytes and 77% fewer allocations. The local
elapsed median fell about 86%; it is not a browser route latency measurement.
Other development jobs were active, so elapsed timings may contain contention.

Environment: Go 1.26.9, Windows AMD64 target on a Windows ARM64 host, one benchmark
CPU. Reproduce from the repository with:

```powershell
go test -work -run '^$' -bench '^BenchmarkSettingsSnapshot$' -benchmem -benchtime=200ms -count=5 -cpu=1 ./internal/appstate
```

Raw samples are retained in [benchmark.txt](benchmark.txt). All appstate tests and
vet passed in a separate run; a production WASM build also passed. Regression
tests cover nested caller mutation, direct store writes, retry, warm hits,
revision changes, nil/empty shape, and warm closed-store access.

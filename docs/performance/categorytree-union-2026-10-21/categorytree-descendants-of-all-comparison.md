# categorytree.DescendantsOfAll benchmark comparison

Commands ran sequentially with Go 1.26.9, `GOOS=windows GOARCH=amd64`, `-work`, and `-benchmem -count=5`. Before and after logs are saved in the adjacent `categorytree-descendants-of-all-{before,after}.log` files. The exact same benchmark fixture and command were used both times.

Fixture: 2,047 categories arranged as a balanced binary tree; 512 selected roots (every fourth node) create heavily overlapping subtrees. Fixture construction is outside the benchmark timer. The operation returns the union set from `DescendantsOfAll`.

| Measure | Before median | After median |
|---|---:|---:|
| Time | 484,233,033 ns/op (484.23 ms) | 776,313 ns/op (0.776 ms) |
| Bytes | 126,765,608 B/op | 464,111 B/op |
| Allocations | 1,053,689 allocs/op | 2,086 allocs/op |

Median time improved about 624x; allocated bytes fell about 273x and allocations about 505x on this synthetic case. The machine reports an ARM64 Windows host but Go benchmark binaries use `windows/amd64` and label the CPU `Virtual CPU @ 4.45GHz`; the ratios are local microbenchmark evidence, not a browser/app latency claim.

After-change validation: `go test -work ./internal/categorytree` passed. The new parity test covers duplicate and blank roots, overlapping cycles, self-parent, unknown root with a dangling child, exact expected union, and input nonmutation. The public `Descendants` and `DescendantsOfAll` now share the same adjacency builder and cycle-guarded traversal, and the union call builds adjacency once.

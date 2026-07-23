// SPDX-License-Identifier: MIT

package loadgen

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Recorder accumulates per-operation latencies, errors, and named counters
// from many concurrent virtual clients. All methods are safe for concurrent
// use.
type Recorder struct {
	mu        sync.Mutex
	started   time.Time
	latencies map[OpKind][]time.Duration
	errors    map[OpKind]int
	counters  map[string]int64
	bytesUp   int64
	bytesDown int64
}

// NewRecorder returns an empty Recorder stamped with the run start time.
func NewRecorder(start time.Time) *Recorder {
	return &Recorder{
		started:   start,
		latencies: map[OpKind][]time.Duration{},
		errors:    map[OpKind]int{},
		counters:  map[string]int64{},
	}
}

// Record logs one completed operation: its latency, and whether it errored.
func (r *Recorder) Record(op OpKind, d time.Duration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.latencies[op] = append(r.latencies[op], d)
	if err != nil {
		r.errors[op]++
	}
}

// Count bumps a named counter (e.g. "push_accepted", "push_rejected",
// "watch_events") by delta.
func (r *Recorder) Count(name string, delta int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[name] += delta
}

// AddBytes tracks payload volume moved up to / down from the server.
func (r *Recorder) AddBytes(up, down int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bytesUp += up
	r.bytesDown += down
}

// OpStats summarizes one operation's outcomes across the run.
type OpStats struct {
	Op     OpKind        `json:"op"`
	Count  int           `json:"count"`
	Errors int           `json:"errors"`
	P50    time.Duration `json:"p50"`
	P95    time.Duration `json:"p95"`
	P99    time.Duration `json:"p99"`
	Max    time.Duration `json:"max"`
	PerSec float64       `json:"perSec"`
}

// Report is the immutable end-of-run summary.
type Report struct {
	Wall      time.Duration    `json:"wall"`
	Ops       []OpStats        `json:"ops"`
	Counters  map[string]int64 `json:"counters"`
	BytesUp   int64            `json:"bytesUp"`
	BytesDown int64            `json:"bytesDown"`
}

// TotalOps sums completed operations across every op kind.
func (rep Report) TotalOps() int {
	n := 0
	for _, o := range rep.Ops {
		n += o.Count
	}
	return n
}

// TotalErrors sums errors across every op kind.
func (rep Report) TotalErrors() int {
	n := 0
	for _, o := range rep.Ops {
		n += o.Errors
	}
	return n
}

// Snapshot freezes the recorder into a Report as of now.
func (r *Recorder) Snapshot(now time.Time) Report {
	r.mu.Lock()
	defer r.mu.Unlock()
	wall := now.Sub(r.started)
	if wall <= 0 {
		wall = time.Nanosecond
	}
	rep := Report{Wall: wall, Counters: map[string]int64{}, BytesUp: r.bytesUp, BytesDown: r.bytesDown}
	for k, v := range r.counters {
		rep.Counters[k] = v
	}
	ops := make([]OpKind, 0, len(r.latencies))
	for op := range r.latencies {
		ops = append(ops, op)
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i] < ops[j] })
	for _, op := range ops {
		samples := append([]time.Duration(nil), r.latencies[op]...)
		sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
		st := OpStats{
			Op:     op,
			Count:  len(samples),
			Errors: r.errors[op],
			P50:    Percentile(samples, 50),
			P95:    Percentile(samples, 95),
			P99:    Percentile(samples, 99),
			PerSec: float64(len(samples)) / wall.Seconds(),
		}
		if len(samples) > 0 {
			st.Max = samples[len(samples)-1]
		}
		rep.Ops = append(rep.Ops, st)
	}
	return rep
}

// Percentile returns the pth percentile (nearest-rank) of an ascending-sorted
// sample set; zero when empty.
func Percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(p / 100 * float64(len(sorted)))
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

// String renders the report as an aligned plain-text table for the terminal.
func (rep Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "wall %s · ops %d · errors %d · up %s · down %s\n",
		rep.Wall.Round(time.Millisecond), rep.TotalOps(), rep.TotalErrors(),
		humanBytes(rep.BytesUp), humanBytes(rep.BytesDown))
	fmt.Fprintf(&b, "%-10s %8s %7s %10s %10s %10s %10s %9s\n",
		"op", "count", "errors", "p50", "p95", "p99", "max", "ops/sec")
	for _, o := range rep.Ops {
		fmt.Fprintf(&b, "%-10s %8d %7d %10s %10s %10s %10s %9.1f\n",
			o.Op, o.Count, o.Errors,
			o.P50.Round(time.Microsecond), o.P95.Round(time.Microsecond),
			o.P99.Round(time.Microsecond), o.Max.Round(time.Microsecond), o.PerSec)
	}
	if len(rep.Counters) > 0 {
		names := make([]string, 0, len(rep.Counters))
		for k := range rep.Counters {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			fmt.Fprintf(&b, "counter %-24s %d\n", k, rep.Counters[k])
		}
	}
	return b.String()
}

// humanBytes formats a byte count with a binary-unit suffix.
func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fGiB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fKiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%dB", n)
}

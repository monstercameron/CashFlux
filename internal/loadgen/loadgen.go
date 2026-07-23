// SPDX-License-Identifier: MIT

// Package loadgen models benchmark/stress-test workloads for the CashFlux
// backend: named scenarios, per-client operation schedules with Poisson
// arrivals, and deterministic seeding so any run can be reproduced exactly.
//
// The package is pure logic (no networking, no syscall/js) so it unit-tests on
// native Go. The network driver that executes a schedule against a live server
// lives in driver.go; the CLI wrapper is cmd/cashflux-loadgen.
package loadgen

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
)

// OpKind names one client→server operation the harness can perform.
type OpKind string

// The operation vocabulary. These map 1:1 onto real protocol calls in the
// driver: sync RPCs over the gRPC-over-WS bridge and blob HTTP.
const (
	OpPush      OpKind = "push"      // SyncService/PutWorkspace
	OpPull      OpKind = "pull"      // SyncService/GetWorkspace
	OpList      OpKind = "list"      // SyncService/ListWorkspaces
	OpBlobPut   OpKind = "blob_put"  // PUT /v1/blobs/{sha256}
	OpBlobGet   OpKind = "blob_get"  // GET /v1/blobs/{sha256}
	OpReconnect OpKind = "reconnect" // close the bridge conn and re-dial (backoff+jitter)
)

// Scenario names a workload shape. Each scenario answers a different scaling
// question; see Describe for the intent of each.
type Scenario string

// The built-in scenarios.
const (
	// ScenarioSteady spreads ops uniformly (Poisson) across the run: the
	// baseline "how many quietly-active clients fit" question.
	ScenarioSteady Scenario = "steady"
	// ScenarioStorm front-loads every client's ops into an opening burst
	// window: the 9 a.m. sync-storm question.
	ScenarioStorm Scenario = "storm"
	// ScenarioStampede schedules a mid-run reconnect for every client inside
	// a short window: the restart thundering-herd question.
	ScenarioStampede Scenario = "stampede"
	// ScenarioConflict pairs clients onto shared workspace IDs so competing
	// pushes exercise the last-write-wins guard: the two-device question.
	ScenarioConflict Scenario = "conflict"
	// ScenarioBlobHeavy shifts the mix toward blob upload/download: the
	// receipt-heavy-user question.
	ScenarioBlobHeavy Scenario = "blob"
	// ScenarioMixed is a realistic blend of everything above except the
	// stampede (steady ops + light blobs + occasional reconnects).
	ScenarioMixed Scenario = "mixed"
)

// Scenarios lists every built-in scenario in presentation order.
func Scenarios() []Scenario {
	return []Scenario{ScenarioSteady, ScenarioStorm, ScenarioStampede, ScenarioConflict, ScenarioBlobHeavy, ScenarioMixed}
}

// Describe returns the one-line intent of a scenario ("" for unknown).
func Describe(s Scenario) string {
	switch s {
	case ScenarioSteady:
		return "uniform Poisson ops — baseline concurrent-client capacity"
	case ScenarioStorm:
		return "everyone syncs in an opening burst — peak throughput ceiling"
	case ScenarioStampede:
		return "mid-run reconnect for every client — restart thundering herd"
	case ScenarioConflict:
		return "paired clients share workspaces — LWW guard under contention"
	case ScenarioBlobHeavy:
		return "blob-dominated mix — receipt-heavy users"
	case ScenarioMixed:
		return "realistic blend of sync, blobs, and reconnects"
	}
	return ""
}

// Profile sets the average per-client rates (events per minute of run time)
// for each operation, and payload sizes. Zero rates drop the op from the mix.
type Profile struct {
	PushPerMin      float64
	PullPerMin      float64
	ListPerMin      float64
	BlobPutPerMin   float64
	BlobGetPerMin   float64
	ReconnectPerMin float64
	// DatasetBytes is the target encoded size of a pushed workspace dataset.
	DatasetBytes int
	// BlobBytes is the size of each uploaded blob.
	BlobBytes int
}

// DefaultProfile returns the strawman per-client mix for a scenario. These
// rates are deliberately aggressive (a stress harness compresses a day of real
// behavior into minutes); the honest per-user-day profile comes later from
// client instrumentation and can be dialed in via flags.
func DefaultProfile(s Scenario) Profile {
	base := Profile{
		PushPerMin:   6,
		PullPerMin:   6,
		ListPerMin:   2,
		DatasetBytes: 32 << 10, // 32 KiB encoded dataset
		BlobBytes:    64 << 10, // 64 KiB receipt-sized blob
	}
	switch s {
	case ScenarioBlobHeavy:
		base.BlobPutPerMin = 6
		base.BlobGetPerMin = 6
		base.PushPerMin = 2
		base.PullPerMin = 2
	case ScenarioMixed:
		base.BlobPutPerMin = 1
		base.BlobGetPerMin = 1
		base.ReconnectPerMin = 0.5
	}
	return base
}

// Event is one scheduled operation for one virtual client.
type Event struct {
	// At is the offset from run start at which the op should fire.
	At time.Duration
	// Op is the operation to perform.
	Op OpKind
}

// Plan is the fully-materialized, deterministic workload for a run.
type Plan struct {
	Scenario Scenario
	Clients  int
	Duration time.Duration
	Seed     int64
	Profile  Profile
	// Schedules holds one time-ordered event list per client.
	Schedules [][]Event
	// WorkspaceIDs holds the workspace each client writes to. Under
	// ScenarioConflict, clients are paired (0,1 share; 2,3 share; …), so
	// competing pushes hit the same row.
	WorkspaceIDs []string
}

// stormWindow is the fraction of the run into which ScenarioStorm compresses
// each client's whole schedule.
const stormWindow = 0.10

// stampedeWindow is the fraction of the run (centered at the midpoint) inside
// which ScenarioStampede fires every client's reconnect.
const stampedeWindow = 0.05

// BuildPlan materializes the deterministic workload for (scenario, clients,
// duration, seed, profile). The same inputs always yield the identical plan,
// so a run can be reproduced or diffed exactly.
func BuildPlan(s Scenario, clients int, duration time.Duration, seed int64, p Profile) (*Plan, error) {
	if clients <= 0 {
		return nil, fmt.Errorf("loadgen: clients must be positive, got %d", clients)
	}
	if duration <= 0 {
		return nil, fmt.Errorf("loadgen: duration must be positive, got %v", duration)
	}
	if Describe(s) == "" {
		return nil, fmt.Errorf("loadgen: unknown scenario %q", s)
	}
	plan := &Plan{
		Scenario:     s,
		Clients:      clients,
		Duration:     duration,
		Seed:         seed,
		Profile:      p,
		Schedules:    make([][]Event, clients),
		WorkspaceIDs: make([]string, clients),
	}
	for c := 0; c < clients; c++ {
		// One independent, seeded stream per client keeps schedules stable
		// even if the client count changes between runs.
		rng := rand.New(rand.NewSource(seed + int64(c)*7919))
		events := poissonEvents(rng, duration, opRates(p))
		switch s {
		case ScenarioStorm:
			compress(events, duration, stormWindow)
		case ScenarioStampede:
			mid := time.Duration(float64(duration) * (0.5 + (rng.Float64()-0.5)*stampedeWindow))
			events = append(events, Event{At: mid, Op: OpReconnect})
		}
		sort.Slice(events, func(i, j int) bool { return events[i].At < events[j].At })
		plan.Schedules[c] = events
		plan.WorkspaceIDs[c] = workspaceID(s, c)
	}
	return plan, nil
}

// TotalEvents counts every scheduled event across all clients.
func (p *Plan) TotalEvents() int {
	n := 0
	for _, s := range p.Schedules {
		n += len(s)
	}
	return n
}

// opRates flattens a Profile into (op, events-per-minute) pairs, dropping
// zero-rate ops.
func opRates(p Profile) map[OpKind]float64 {
	rates := map[OpKind]float64{}
	set := func(op OpKind, r float64) {
		if r > 0 {
			rates[op] = r
		}
	}
	set(OpPush, p.PushPerMin)
	set(OpPull, p.PullPerMin)
	set(OpList, p.ListPerMin)
	set(OpBlobPut, p.BlobPutPerMin)
	set(OpBlobGet, p.BlobGetPerMin)
	set(OpReconnect, p.ReconnectPerMin)
	return rates
}

// poissonEvents draws each op's arrivals as an independent Poisson process
// (exponential inter-arrival times) over the run duration.
func poissonEvents(rng *rand.Rand, duration time.Duration, rates map[OpKind]float64) []Event {
	var events []Event
	// Iterate ops in a fixed order so the rng draw sequence is deterministic.
	for _, op := range []OpKind{OpPush, OpPull, OpList, OpBlobPut, OpBlobGet, OpReconnect} {
		perMin, ok := rates[op]
		if !ok {
			continue
		}
		mean := time.Duration(float64(time.Minute) / perMin)
		at := time.Duration(0)
		for {
			// Exponential inter-arrival with the op's mean gap.
			gap := time.Duration(-math.Log(1-rng.Float64()) * float64(mean))
			at += gap
			if at >= duration {
				break
			}
			events = append(events, Event{At: at, Op: op})
		}
	}
	return events
}

// compress rescales every event into the opening fraction of the run,
// preserving relative order — the storm shape.
func compress(events []Event, duration time.Duration, fraction float64) {
	for i := range events {
		events[i].At = time.Duration(float64(events[i].At) * fraction)
		if max := time.Duration(float64(duration) * fraction); events[i].At > max {
			events[i].At = max
		}
	}
}

// workspaceID assigns the workspace a client writes to. Conflict pairs
// clients two-by-two onto one ID; every other scenario isolates clients.
func workspaceID(s Scenario, client int) string {
	if s == ScenarioConflict {
		return fmt.Sprintf("w-load-pair-%03d", client/2)
	}
	return fmt.Sprintf("w-load-%03d", client)
}

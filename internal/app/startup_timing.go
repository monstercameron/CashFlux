// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"math"
	"syscall/js"
	"time"
)

// Startup timing is intentionally a small, fixed-schema record. It contains only
// elapsed durations; it never captures route, account, workspace, or dataset data.
// Run starts this clock after WASM has initialized, so the record does not include
// fetching, decoding, or compiling the WASM binary.
const startupTimingSchemaVersion = 1

var startupTimingPhaseNames = [...]string{
	"entryToStylesMs",
	"protectedStorageMs",
	"appStateWorkspaceAndHydrationMs",
	"sessionBootstrapMs",
	"preMountPreparationMs",
	"recurringCoversMs",
	"preMountAppearanceAndRoutesMs",
	"routerMountMs",
	"postMountWiringMs",
	"postMountRecurringMs",
	"postMountScheduledWorkflowsMs",
	"postMountBillDueTriggerMs",
	"postMountNotificationScheduleMs",
	"postMountTailWiringMs",
	"readySignalMs",
}

type startupTimer struct {
	target   js.Value
	started  float64
	active   map[string]float64
	phases   map[string]float64
	clock    func() float64
	finished bool
}

func newStartupTimer() *startupTimer {
	win := js.Global().Get("window")
	perf := js.Global().Get("performance")
	if !perf.Truthy() && win.Truthy() {
		perf = win.Get("performance")
	}
	return newStartupTimerWith(perf, win, time.Now)
}

// newStartupTimerWith keeps clock and target injection local to this helper so
// the wasm unit tests can exercise publication without running app boot.
func newStartupTimerWith(perf, target js.Value, wallNow func() time.Time) *startupTimer {
	clock := func() float64 {
		if perf.Truthy() {
			now := perf.Get("now")
			if now.Type() == js.TypeFunction {
				elapsed := perf.Call("now").Float()
				if !math.IsNaN(elapsed) && !math.IsInf(elapsed, 0) {
					return elapsed
				}
			}
		}
		return float64(wallNow().UnixNano()) / float64(time.Millisecond)
	}
	started := clock()
	return &startupTimer{
		target:  target,
		started: started,
		active:  make(map[string]float64, len(startupTimingPhaseNames)),
		phases:  make(map[string]float64, len(startupTimingPhaseNames)),
		clock:   clock,
	}
}

func (t *startupTimer) BeginPhase(name string) {
	if t == nil || t.finished || !knownStartupPhase(name) {
		return
	}
	t.active[name] = t.clock()
}

func (t *startupTimer) EndPhase(name string) {
	if t == nil || t.finished || !knownStartupPhase(name) {
		return
	}
	started, ok := t.active[name]
	if !ok {
		return
	}
	duration := t.clock() - started
	if duration < 0 {
		duration = 0
	}
	t.phases[name] += duration
	delete(t.active, name)
}

func (t *startupTimer) Finish() {
	if t == nil || t.finished {
		return
	}
	t.finished = true
	if !t.target.Truthy() {
		return
	}
	end := t.clock()
	total := end - t.started
	if total < 0 {
		total = 0
	}
	record := js.Global().Get("Object").New()
	record.Set("schemaVersion", startupTimingSchemaVersion)
	record.Set("totalMs", total)
	phases := js.Global().Get("Object").New()
	for _, name := range startupTimingPhaseNames {
		if duration, ok := t.phases[name]; ok {
			phases.Set(name, duration)
		}
	}
	record.Set("phases", phases)
	t.target.Set("__cashfluxBootTimings", record)
}

func knownStartupPhase(name string) bool {
	for _, known := range startupTimingPhaseNames {
		if name == known {
			return true
		}
	}
	return false
}

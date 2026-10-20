// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"math"
	"syscall/js"
	"testing"
	"time"
)

func TestStartupTimerPublishesBoundedPhaseRecord(t *testing.T) {
	nowMS := 10.0
	nowFn := js.FuncOf(func(js.Value, []js.Value) any { return nowMS })
	defer nowFn.Release()
	perf := js.Global().Get("Object").New()
	perf.Set("now", nowFn)
	target := js.Global().Get("Object").New()

	timer := newStartupTimerWith(perf, target, time.Now)
	timer.BeginPhase("routerMountMs")
	nowMS = 18.25
	timer.EndPhase("routerMountMs")
	nowMS = 20
	timer.BeginPhase("routerMountMs")
	nowMS = 22.5
	timer.EndPhase("routerMountMs")
	timer.BeginPhase("notAStartupPhase") // ignored; the record schema stays fixed
	timer.Finish()
	nowMS = 50
	timer.Finish() // publication is once per boot record

	record := target.Get("__cashfluxBootTimings")
	if !record.Truthy() {
		t.Fatal("startup timing record was not published")
	}
	if got := record.Get("schemaVersion").Int(); got != startupTimingSchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", got, startupTimingSchemaVersion)
	}
	if got := record.Get("totalMs").Float(); got != 12.5 {
		t.Fatalf("totalMs = %v, want 12.5", got)
	}
	phases := record.Get("phases")
	if got := phases.Get("routerMountMs").Float(); got != 10.75 {
		t.Fatalf("routerMountMs = %v, want 10.75 (sum of both intervals)", got)
	}
	if got := phases.Get("notAStartupPhase"); !got.IsUndefined() {
		t.Fatalf("unexpected phase value = %v", got)
	}
}

func TestStartupTimerFallsBackWithoutPerformanceAPI(t *testing.T) {
	wall := time.Unix(0, 0)
	target := js.Global().Get("Object").New()
	timer := newStartupTimerWith(js.Undefined(), target, func() time.Time { return wall })
	timer.BeginPhase("protectedStorageMs")
	wall = wall.Add(6 * time.Millisecond)
	timer.EndPhase("protectedStorageMs")
	wall = wall.Add(4 * time.Millisecond)
	timer.Finish()

	record := target.Get("__cashfluxBootTimings")
	if got := record.Get("totalMs").Float(); got != 10 {
		t.Fatalf("fallback totalMs = %v, want 10", got)
	}
	if got := record.Get("phases").Get("protectedStorageMs").Float(); got != 6 {
		t.Fatalf("fallback protectedStorageMs = %v, want 6", got)
	}
}

func TestStartupTimerFallsBackFromNonFinitePerformanceValues(t *testing.T) {
	nowFn := js.FuncOf(func(js.Value, []js.Value) any { return math.NaN() })
	defer nowFn.Release()
	perf := js.Global().Get("Object").New()
	perf.Set("now", nowFn)
	wall := time.Unix(0, 0)
	target := js.Global().Get("Object").New()
	timer := newStartupTimerWith(perf, target, func() time.Time { return wall })
	timer.BeginPhase("routerMountMs")
	wall = wall.Add(4 * time.Millisecond)
	timer.EndPhase("routerMountMs")
	wall = wall.Add(2 * time.Millisecond)
	timer.Finish()

	record := target.Get("__cashfluxBootTimings")
	if got := record.Get("totalMs").Float(); math.IsNaN(got) || math.IsInf(got, 0) || got != 6 {
		t.Fatalf("totalMs = %v, want finite 6", got)
	}
	if got := record.Get("phases").Get("routerMountMs").Float(); math.IsNaN(got) || math.IsInf(got, 0) || got != 4 {
		t.Fatalf("routerMountMs = %v, want finite 4", got)
	}
}

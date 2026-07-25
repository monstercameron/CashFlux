// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"sync"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
)

// phaseBudget is roughly two frames. Below this a phase is invisible; above it,
// the user sees a hitch — so that is the line worth reporting at.
const phaseBudget = 32 * time.Millisecond

// timePhase reports how long a unit of sync work held the main thread, for any
// unit that held it long enough to cost a frame.
//
// This exists because wasm's single thread makes "how long does this take" and
// "how long is the page frozen" the same question, while the browser's own
// profiler cannot answer it: Chrome attributes the cost to whichever JS callback
// happened to resume the Go scheduler, so a slow sync phase shows up as a slow
// 'click' or 'scroll' handler and the real culprit is never named. Timing the
// phases from inside Go is the only way to see which one is actually expensive.
//
// Usage: defer timePhase("prepare")().
func timePhase(name string) func() {
	t0 := time.Now()
	return func() {
		d := time.Since(t0)
		if d < phaseBudget {
			return
		}
		if app := appstate.Default; app != nil {
			app.Log().Info("sync phase held the main thread", "phase", name, "ms", d.Milliseconds())
		}
	}
}

// lastYieldAt tracks when sync work last handed the thread back, so maybeYield can
// bound how long it runs between breaths without a caller having to thread state.
// Guarded because pushes, pulls and blob transfers all call maybeYield from their
// own goroutines: wasm runs them on one thread, but "one thread" is not the same
// promise as "no interleaving", and the race detector is right to insist.
var (
	yieldMu     sync.Mutex
	lastYieldAt time.Time
)

// yieldSlice is the longest sync work may hold the thread before giving it back
// even with nothing waiting. Roughly three frames: long enough that yielding is not
// the dominant cost, short enough that a click arriving right after a check still
// gets served well inside the ~100ms that reads as instant.
const yieldSlice = 50 * time.Millisecond

// maybeYield gives the main thread back when the user needs it, and is nearly free
// when they don't.
//
// It exists because deferring sync to an idle callback fixed WHEN work starts but
// not how long it runs once started. Go on wasm has one thread and no preemption
// across the JS boundary: once the scheduler is resumed it runs every runnable
// goroutine until they all block. So a sync that has been carved into individually
// cheap phases can still hold the thread for a second in aggregate — and because
// the browser charges that to whichever callback resumed Go, it surfaces as a
// 'click' handler that took 1977ms. The user did not click something slow. They
// clicked, and their click paid for the sync.
//
// navigator.scheduling.isInputPending() is the direct question: does the browser
// have a click, tap, scroll or keystroke queued that it cannot deliver because we
// are holding the thread? When the answer is yes, nothing sync is doing matters
// more, so yield immediately. The elapsed-time check is the fallback for browsers
// without it and for the case where no input is pending but a frame is due anyway.
//
// Call it inside loops and between steps — it is a cheap property read on the fast
// path, and only the yield itself costs a frame.
func maybeYield() {
	yieldMu.Lock()
	first := lastYieldAt.IsZero()
	elapsed := time.Since(lastYieldAt)
	if first {
		lastYieldAt = time.Now()
	}
	yieldMu.Unlock()
	if first {
		return
	}
	if !inputPending() && elapsed < yieldSlice {
		return
	}
	yieldToBrowser() // never held across the lock: it blocks for a whole frame
	yieldMu.Lock()
	lastYieldAt = time.Now()
	yieldMu.Unlock()
}

// inputPending reports whether the browser is holding user input it cannot deliver
// because Go has the thread. Returns false where the API is unavailable, which
// leaves maybeYield on its time-based fallback rather than yielding blindly.
func inputPending() bool {
	sched := js.Global().Get("navigator").Get("scheduling")
	if !sched.Truthy() {
		return false
	}
	fn := sched.Get("isInputPending")
	if fn.IsUndefined() || fn.Type() != js.TypeFunction {
		return false
	}
	// includeContinuous surfaces scroll and pointermove, not just discrete clicks —
	// a scroll that stutters is exactly the symptom being fixed here.
	return sched.Call("isInputPending", map[string]any{"includeContinuous": true}).Bool()
}

// SPDX-License-Identifier: MIT

// Package appstate — sandbox_ops.go is the what-if sandbox (AG2): a real, working
// copy of the household's data that the assistant can change freely, because
// nothing in it is theirs.
//
// The question "what if I moved to a £1,800 flat?" has no good answer from
// arithmetic alone — the interesting effects are second-order (the budget that
// stops fitting, the goal whose date slips, the runway that shortens), and they
// come out of the same engines that compute the real figures. So rather than model
// a hypothetical, the sandbox COPIES the dataset and lets the ordinary machinery
// run over the copy. The diff between the two is the answer.
//
// Two guarantees make this safe enough to hand an agent:
//
//   - The copy is a separate App over its own in-memory store, built from a JSON
//     export. There is no shared pointer, no shared store handle, and therefore no
//     way for a sandbox write to reach the real dataset by accident.
//   - The sandbox is never persisted. It has no persist hook, and it is discarded
//     when the conversation ends. Applying a scenario for real goes back through
//     the ordinary changeset review, where each step is approved individually.
//
// The second guarantee is why the sandbox can be generous about what it allows: a
// change that cannot escape does not need to be argued about.
package appstate

import (
	"fmt"
	"io"

	"github.com/monstercameron/CashFlux/internal/whatif"
)

// Sandbox is a disposable copy of the household's data.
type Sandbox struct {
	// App is the copy. It is a fully working App — every read and write method
	// behaves normally, over data that belongs to nobody.
	App *App
	// baseline is the engine's view of the REAL dataset at the moment the sandbox
	// was taken, so the diff compares like with like even if the real data changes
	// while the scenario is being explored.
	baseline map[string]float64
}

// NewSandbox copies the app's current data into a disposable App.
//
// The copy goes through the ordinary JSON export/import rather than a struct copy,
// because that is the one path already guaranteed to be lossless — it is what the
// backup feature relies on — and because a hand-written deep copy would silently
// stop copying any field added afterwards.
func (a *App) NewSandbox() (*Sandbox, error) {
	if a == nil {
		return nil, fmt.Errorf("appstate: no app to sandbox")
	}
	data, err := a.ExportJSON()
	if err != nil {
		return nil, fmt.Errorf("appstate: snapshot for sandbox: %w", err)
	}
	// io.Discard for the log: a sandbox's warnings are about data that does not
	// exist, and mixing them into the real log would make the real ones harder to
	// find.
	copyApp, err := New(io.Discard, false)
	if err != nil {
		return nil, fmt.Errorf("appstate: build sandbox: %w", err)
	}
	if err := copyApp.ImportJSON(data); err != nil {
		return nil, fmt.Errorf("appstate: fill sandbox: %w", err)
	}
	// A sandbox must never reach the household's screen either. Clearing the
	// notifier is belt and braces on top of it having its own store: if a
	// hypothetical workflow fires inside the sandbox, its notice does not surface
	// as though it were about real money.
	copyApp.Notifier = nil
	// The copy shares the original's clock. Without this the two sides of the diff
	// are evaluated at different instants, and every time-dependent figure —
	// no-spend days, days until a bill, anything counting from "today" — drifts on
	// its own. An untouched sandbox would then report changes nobody made, which is
	// exactly the noise that makes a what-if tool untrustworthy.
	copyApp.now = a.clock
	return &Sandbox{App: copyApp, baseline: copySafeVars(a)}, nil
}

// Diff returns what changed between the real household and the sandbox, largest
// movement first, ignoring differences below epsilon.
//
// The baseline is the one captured when the sandbox was created, not the app's
// current state. Recomputing it would mean an ordinary edit made in another tab
// while somebody explored a scenario would show up as an effect OF that scenario,
// which is the most misleading thing this feature could do.
func (s *Sandbox) Diff(epsilon float64) []whatif.Change {
	if s == nil || s.App == nil {
		return nil
	}
	return whatif.Diff(s.baseline, copySafeVars(s.App), epsilon)
}

// copySafeVars takes an independent copy of an app's engine variable surface — the
// vocabulary the diff speaks: net worth, runway, per-budget and per-goal figures.
//
// The copy matters. The baseline is held for the life of the sandbox, and handing
// out the engine's own map would let a later recomputation mutate the thing the
// diff is measured against, so a scenario's effects would quietly shrink as the
// real data moved underneath it.
func copySafeVars(a *App) map[string]float64 {
	if a == nil {
		return map[string]float64{}
	}
	vars := a.engineVars()
	out := make(map[string]float64, len(vars))
	for k, v := range vars {
		out[k] = v
	}
	return out
}

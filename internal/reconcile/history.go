// SPDX-License-Identifier: MIT

package reconcile

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// MaxHistory caps how many reconciliation events an account keeps (three years
// of monthly statements). Recording past the cap drops the oldest entries.
const MaxHistory = 36

// Record adds a reconciliation event to an account's history, keeping the list
// ordered oldest-first by reconciled-through date and capped at MaxHistory. An
// event with the same through date as existing ones goes after them, so a
// statement dated earlier than the newest one is inserted before it rather than
// appended. It returns a new slice; the input is never mutated.
func Record(history []domain.Reconciliation, ev domain.Reconciliation) []domain.Reconciliation {
	at := sort.Search(len(history), func(i int) bool { return history[i].Through().After(ev.Through()) })
	out := make([]domain.Reconciliation, 0, len(history)+1)
	out = append(out, history[:at]...)
	out = append(out, ev)
	out = append(out, history[at:]...)
	if len(out) > MaxHistory {
		out = out[len(out)-MaxHistory:]
	}
	return out
}

// Undo removes the NEWEST reconciliation event (the last in history order) from the history (the
// reopen-last action, QA R3 CF-02): it returns the shortened history, the
// removed event (so the caller can re-seed the reconcile form with its
// statement balance/date), and false when there is nothing to undo. The input
// slice is never mutated.
func Undo(history []domain.Reconciliation) ([]domain.Reconciliation, domain.Reconciliation, bool) {
	if len(history) == 0 {
		return nil, domain.Reconciliation{}, false
	}
	out := make([]domain.Reconciliation, len(history)-1)
	copy(out, history[:len(history)-1])
	return out, history[len(history)-1], true
}

// Through returns the latest "reconciled through" date across the history —
// the newest entry's statement date (or recording time) — and false when the
// account has never been reconciled.
func Through(history []domain.Reconciliation) (time.Time, bool) {
	var best time.Time
	for _, r := range history {
		if t := r.Through(); t.After(best) {
			best = t
		}
	}
	return best, !best.IsZero()
}

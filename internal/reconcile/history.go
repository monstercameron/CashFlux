// SPDX-License-Identifier: MIT

package reconcile

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// MaxHistory caps how many reconciliation events an account keeps (three years
// of monthly statements). Recording past the cap drops the oldest entries.
const MaxHistory = 36

// Record appends a reconciliation event to an account's history, keeping the
// list oldest-first and capped at MaxHistory. It returns a new slice; the input
// is never mutated.
func Record(history []domain.Reconciliation, ev domain.Reconciliation) []domain.Reconciliation {
	out := make([]domain.Reconciliation, 0, len(history)+1)
	out = append(out, history...)
	out = append(out, ev)
	if len(out) > MaxHistory {
		out = out[len(out)-MaxHistory:]
	}
	return out
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

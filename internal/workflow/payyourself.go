// SPDX-License-Identifier: MIT

package workflow

import "strings"

// IsPayYourselfFirst reports whether w uses the persisted marker shared by the
// pay-yourself-first transfer creators: one transfer action with a pyf: dedupe
// key. Ordinary transfer workflows use a different dedupe namespace.
func IsPayYourselfFirst(w Workflow) bool {
	if len(w.Actions) != 1 {
		return false
	}
	a := w.Actions[0]
	return a.Kind == ActionTransfer && strings.HasPrefix(a.DedupeKey, "pyf:")
}

// SamePayYourselfFirstIdentity reports whether two PYF workflows describe the
// same transfer route and cadence. Amount, name, enabled state, next run, and
// workflow ID do not distinguish a second scheduled transfer on that route.
func SamePayYourselfFirstIdentity(a, b Workflow) bool {
	if !IsPayYourselfFirst(a) || !IsPayYourselfFirst(b) {
		return false
	}
	actionA, actionB := a.Actions[0], b.Actions[0]
	return actionA.TransferFromAccountID == actionB.TransferFromAccountID &&
		actionA.TransferToAccountID == actionB.TransferToAccountID &&
		a.Trigger.Cadence == b.Trigger.Cadence
}

// FindPayYourselfFirstDuplicate returns the first existing workflow with the
// same PYF identity as candidate. Disabled workflows count because reenabling
// one would otherwise duplicate the transfer. The candidate's ID is excluded
// so callers can validate updates without matching a workflow against itself.
func FindPayYourselfFirstDuplicate(candidate Workflow, existing []Workflow) (Workflow, bool) {
	if !IsPayYourselfFirst(candidate) {
		return Workflow{}, false
	}
	for _, other := range existing {
		if candidate.ID != "" && other.ID == candidate.ID {
			continue
		}
		if SamePayYourselfFirstIdentity(candidate, other) {
			return other, true
		}
	}
	return Workflow{}, false
}

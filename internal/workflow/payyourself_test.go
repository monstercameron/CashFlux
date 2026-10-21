// SPDX-License-Identifier: MIT

package workflow

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func pyfWorkflow(id, from, to string, cadence domain.RecurringCadence, enabled bool) Workflow {
	return Workflow{
		ID:      id,
		Name:    "PYF " + id,
		Enabled: enabled,
		Trigger: Trigger{Kind: TriggerScheduled, Cadence: cadence},
		Actions: []Action{{
			Kind:                  ActionTransfer,
			TransferFromAccountID: from,
			TransferToAccountID:   to,
			TransferAmount:        100,
			DedupeKey:             "pyf:" + id + ":{period}",
		}},
	}
}

func TestIsPayYourselfFirstUsesMarkerAndSingleTransfer(t *testing.T) {
	cases := []struct {
		name string
		wf   Workflow
		want bool
	}{
		{"monthly generated transfer", pyfWorkflow("pyf-monthly", "checking", "savings", domain.CadenceMonthly, true), true},
		{"weekly generated transfer", pyfWorkflow("pyf-weekly", "checking", "savings", domain.CadenceWeekly, true), true},
		{"ordinary transfer namespace", func() Workflow {
			wf := pyfWorkflow("ordinary", "checking", "savings", domain.CadenceMonthly, true)
			wf.Actions[0].DedupeKey = "wf:ordinary:{period}"
			return wf
		}(), false},
		{"prefix without colon", func() Workflow {
			wf := pyfWorkflow("plain", "checking", "savings", domain.CadenceMonthly, true)
			wf.Actions[0].DedupeKey = "pyf"
			return wf
		}(), false},
		{"not a transfer", func() Workflow {
			wf := pyfWorkflow("notify", "checking", "savings", domain.CadenceMonthly, true)
			wf.Actions[0].Kind = ActionNotify
			return wf
		}(), false},
		{"multiple actions", func() Workflow {
			wf := pyfWorkflow("multiple", "checking", "savings", domain.CadenceMonthly, true)
			wf.Actions = append(wf.Actions, Action{Kind: ActionNotify})
			return wf
		}(), false},
		{"missing marker", func() Workflow {
			wf := pyfWorkflow("missing", "checking", "savings", domain.CadenceMonthly, true)
			wf.Actions[0].DedupeKey = ""
			return wf
		}(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsPayYourselfFirst(tc.wf); got != tc.want {
				t.Fatalf("IsPayYourselfFirst() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSamePayYourselfFirstIdentityUsesRouteAndCadence(t *testing.T) {
	a := pyfWorkflow("first", "checking", "savings", domain.CadenceMonthly, true)
	b := pyfWorkflow("second", "checking", "savings", domain.CadenceMonthly, false)
	b.Name = "Different name"
	b.Actions[0].TransferAmount = 500
	if !SamePayYourselfFirstIdentity(a, b) {
		t.Fatal("same route and cadence should identify a duplicate despite amount, name, enabled, and ID differences")
	}

	for _, tc := range []struct {
		name   string
		change func(*Workflow)
	}{
		{"source", func(w *Workflow) { w.Actions[0].TransferFromAccountID = "other" }},
		{"destination", func(w *Workflow) { w.Actions[0].TransferToAccountID = "other" }},
		{"cadence", func(w *Workflow) { w.Trigger.Cadence = domain.CadenceWeekly }},
		{"ordinary dedupe namespace", func(w *Workflow) { w.Actions[0].DedupeKey = "wf:second:{period}" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := b
			changed.Actions = append([]Action(nil), b.Actions...)
			tc.change(&changed)
			if SamePayYourselfFirstIdentity(a, changed) {
				t.Fatal("changed identity should not be a duplicate")
			}
		})
	}
}

func TestFindPayYourselfFirstDuplicateIncludesDisabledAndExcludesCandidateID(t *testing.T) {
	candidate := pyfWorkflow("edited", "checking", "savings", domain.CadenceMonthly, true)
	self := pyfWorkflow("edited", "checking", "savings", domain.CadenceMonthly, true)
	if _, ok := FindPayYourselfFirstDuplicate(candidate, []Workflow{self}); ok {
		t.Fatal("candidate must not match its own ID during an update")
	}

	disabled := pyfWorkflow("off", "checking", "savings", domain.CadenceMonthly, false)
	got, ok := FindPayYourselfFirstDuplicate(candidate, []Workflow{self, disabled})
	if !ok {
		t.Fatal("disabled duplicate should still be found")
	}
	if got.ID != disabled.ID || got.Enabled {
		t.Fatalf("duplicate = %#v, want disabled workflow %q", got, disabled.ID)
	}
}

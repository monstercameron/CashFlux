// SPDX-License-Identifier: MIT

package nlauthor

import (
	"fmt"
	"testing"

	"github.com/monstercameron/CashFlux/internal/workflow"
)

func TestCompileWorkflowRestrictsReservedVariablesToTransactionTrigger(t *testing.T) {
	for _, trigger := range []workflow.TriggerKind{workflow.TriggerTxnAdded, workflow.TriggerManual, workflow.TriggerScheduled,
		workflow.TriggerBudgetExceeded, workflow.TriggerGoalReached, workflow.TriggerBillDue} {
		for _, name := range workflow.TransactionVariableNames() {
			t.Run(string(trigger)+"/"+name, func(t *testing.T) {
				input := WorkflowInput{ID: "wf", Name: "Review", Enabled: true, Trigger: workflow.Trigger{Kind: trigger},
					Condition: fmt.Sprintf("%s == 0", name), Actions: []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "Review"}}}
				for _, known := range []func(string) bool{nil, func(string) bool { return true }} {
					_, errs := CompileWorkflow(input, known)
					if (len(errs) == 0) != (trigger == workflow.TriggerTxnAdded) {
						t.Fatalf("reserved variable gating for %s: errors=%v", trigger, errs)
					}
				}
			})
		}
	}
}

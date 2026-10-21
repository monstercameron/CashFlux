// SPDX-License-Identifier: MIT

package appstate

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// WorkflowVariableNamesFor lists the sorted, declared condition variables for a
// trigger. Transaction triggers include their own numeric and string fields;
// other triggers retain the aggregate engine surface, including numeric custom
// field sums, without advertising unavailable per-transaction values.
func (a *App) WorkflowVariableNamesFor(trigger workflow.TriggerKind) []string {
	names := map[string]bool{}
	for name := range a.engineVars() {
		if trigger == workflow.TriggerTxnAdded || !workflow.IsTransactionVariable(name) {
			names[name] = true
		}
	}
	// An unavailable spending observation must retain its established handle for
	// validation/discovery, without publishing an invented runtime value.
	for _, base := range engineenv.BudgetVarBases(a.Budgets()) {
		for _, field := range engineenv.BudgetVarFields {
			names[base.Prefix+field] = true
		}
	}
	if trigger == workflow.TriggerTxnAdded {
		for _, name := range workflow.TransactionVariableNames() {
			names[name] = true
		}
		// txnContext exposes each actual transaction custom definition, using
		// numeric values for number/bool and string values for text/date/choice.
		for _, def := range a.CustomFieldDefsFor("transaction") {
			names["cf_txn_"+def.Key] = true
		}
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/rules"
)

func TestRuleFlowConditionIsSelectableInAddAndEditForms(t *testing.T) {
	foundFlow := false
	for _, option := range condSlotFieldOpts() {
		if option.Value == string(rules.ConditionFieldFlow) {
			foundFlow = true
		}
	}
	if !foundFlow {
		t.Fatal("flow is missing from the shared add/edit condition field picker")
	}
	if got := condSlotOpOpts(string(rules.ConditionFieldFlow)); len(got) != 1 || got[0].Value != string(rules.ConditionOpEquals) {
		t.Fatalf("flow operators = %+v, want equals", got)
	}
}

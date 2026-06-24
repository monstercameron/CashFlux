// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestCustomFieldValidationEnforcedOnWrites verifies that a required custom-field
// definition is enforced on every entity write that supports custom fields — each
// otherwise-valid entity that omits the required value is rejected (the
// validateCustom-rejection branch of PutMember/PutTransaction/PutBudget/PutGoal;
// PutAccount is covered by TestPutAccountValidatesCustomFields).
func TestCustomFieldValidationEnforcedOnWrites(t *testing.T) {
	a := newApp(t, false)
	for _, et := range []string{"member", "transaction", "budget", "goal"} {
		if err := a.PutCustomFieldDef(customfields.Def{
			ID: "cf_" + et, EntityType: et, Key: "ref", Label: "Reference",
			Type: customfields.TypeText, Required: true,
		}); err != nil {
			t.Fatalf("PutCustomFieldDef(%s): %v", et, err)
		}
	}

	if err := a.PutMember(domain.Member{ID: "m1", Name: "Alice"}); err == nil {
		t.Error("PutMember missing a required custom field should be rejected")
	}
	if err := a.PutTransaction(domain.Transaction{ID: "t1", AccountID: "a1", Desc: "Coffee", Amount: money.New(-100, "USD"), Date: time.Now()}); err == nil {
		t.Error("PutTransaction missing a required custom field should be rejected")
	}
	if err := a.PutBudget(domain.Budget{ID: "b1", Name: "Food", CategoryID: "c1", Period: domain.PeriodMonthly, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: money.New(1000, "USD")}); err == nil {
		t.Error("PutBudget missing a required custom field should be rejected")
	}
	if err := a.PutGoal(domain.Goal{ID: "g1", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(1000, "USD")}); err == nil {
		t.Error("PutGoal missing a required custom field should be rejected")
	}
}

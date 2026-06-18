package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestAppErrorsOnClosedStore drives appstate's store-error branches by closing the
// underlying store (no production seam — Store()/Close() are public). Valid
// entities pass validation and reach the failing store call; accessors swallow the
// error via logErr but exercise its error arm.
func TestAppErrorsOnClosedStore(t *testing.T) {
	// A second, open app gives us a valid dataset JSON so ImportJSON's parse
	// succeeds and the *Load* fails (not the parse).
	src := newApp(t, false)
	validJSON, err := src.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON(src): %v", err)
	}

	a := newApp(t, false)
	a.Store().Close()

	validAcc := domain.Account{ID: "a1", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
	checks := []struct {
		name string
		err  error
	}{
		{"PutAccount", a.PutAccount(validAcc)},
		{"PutMember", a.PutMember(domain.Member{ID: "m1", Name: "Alice"})},
		{"PutCategory", a.PutCategory(domain.Category{ID: "c1", Name: "Food", Kind: domain.KindExpense})},
		{"PutTransaction", a.PutTransaction(domain.Transaction{ID: "t1", AccountID: "a1", Desc: "Coffee", Amount: money.New(-100, "USD"), Date: time.Now()})},
		{"PutBudget", a.PutBudget(domain.Budget{ID: "b1", Name: "Food", CategoryID: "c1", Period: domain.PeriodMonthly, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: money.New(1000, "USD")})},
		{"PutGoal", a.PutGoal(domain.Goal{ID: "g1", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(1000, "USD")})},
		{"PutTask", a.PutTask(domain.Task{ID: "k1", Title: "Pay rent", Status: domain.StatusOpen, Priority: domain.PriorityLow})},
		{"PutCustomFieldDef", a.PutCustomFieldDef(customfields.Def{ID: "d1", EntityType: "account", Key: "branch", Label: "Branch", Type: customfields.TypeText})},
		{"PutSettings", a.PutSettings(a.Settings())},
		{"DeleteMember", a.DeleteMember("m1")},
		{"ImportJSON", a.ImportJSON(validJSON)},
		{"LoadSample", a.LoadSample()},
		{"Wipe", a.Wipe()},
	}
	for _, c := range checks {
		if c.err == nil {
			t.Errorf("%s on a closed store should error", c.name)
		}
	}
	if _, err := a.ExportJSON(); err == nil {
		t.Error("ExportJSON on a closed store should error")
	}

	// Accessors return nil on error (logged via logErr) — exercise the error arm.
	_ = a.Categories()
	_ = a.Tasks()
	_ = a.Accounts()
	_ = a.CustomFieldDefsFor("account")
}

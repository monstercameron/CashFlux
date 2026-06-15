package validate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(n int64) money.Money { return money.New(n, "USD") }

func hasField(is Issues, field string) bool {
	for _, i := range is {
		if i.Field == field {
			return true
		}
	}
	return false
}

func TestValidateMember(t *testing.T) {
	if !ValidateMember(domain.Member{Name: "Alice"}).OK() {
		t.Error("valid member should pass")
	}
	if ValidateMember(domain.Member{}).OK() {
		t.Error("member without name should fail")
	}
}

func TestValidateAccountValid(t *testing.T) {
	a := domain.Account{
		Name: "Savings", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD",
		OpeningBalance: usd(1000), LiquidityScore: 50, StabilityScore: 80,
	}
	if is := ValidateAccount(a); !is.OK() {
		t.Errorf("valid account failed: %v", is)
	}
}

func TestValidateAccountProblems(t *testing.T) {
	a := domain.Account{
		Name: "", OwnerID: "", Scope: "bogus",
		Class: domain.ClassAsset, Type: domain.TypeCreditCard, // class mismatch (cc is liability)
		Currency: "US", OpeningBalance: money.New(10, "EUR"),
		LiquidityScore: 200, DueDayOfMonth: 31,
	}
	is := ValidateAccount(a)
	for _, f := range []string{"name", "ownerId", "scope", "class", "currency", "openingBalance", "liquidityScore", "dueDayOfMonth"} {
		if !hasField(is, f) {
			t.Errorf("expected issue for %q; got %v", f, is)
		}
	}
}

func TestValidateCategory(t *testing.T) {
	if !ValidateCategory(domain.Category{Name: "Food", Kind: domain.KindExpense}).OK() {
		t.Error("valid category should pass")
	}
	is := ValidateCategory(domain.Category{Kind: "bogus"})
	if !hasField(is, "name") || !hasField(is, "kind") {
		t.Errorf("expected name + kind issues, got %v", is)
	}
}

func TestValidateTransaction(t *testing.T) {
	good := domain.Transaction{AccountID: "a1", Desc: "Coffee", Amount: usd(-500), Date: time.Now()}
	if is := ValidateTransaction(good); !is.OK() {
		t.Errorf("valid transaction failed: %v", is)
	}
	bad := domain.Transaction{AccountID: "a1", TransferAccountID: "a1", Amount: money.Money{}}
	is := ValidateTransaction(bad)
	for _, f := range []string{"desc", "amount", "date", "transferAccountId"} {
		if !hasField(is, f) {
			t.Errorf("expected issue for %q, got %v", f, is)
		}
	}
}

func TestValidateBudget(t *testing.T) {
	good := domain.Budget{Name: "Food", OwnerID: "g", CategoryID: "c", Scope: domain.ScopeShared, Period: domain.PeriodMonthly, Limit: usd(50000)}
	if is := ValidateBudget(good); !is.OK() {
		t.Errorf("valid budget failed: %v", is)
	}
	is := ValidateBudget(domain.Budget{Period: "weekly", Limit: usd(0)})
	for _, f := range []string{"name", "ownerId", "categoryId", "scope", "period", "limit"} {
		if !hasField(is, f) {
			t.Errorf("expected issue for %q, got %v", f, is)
		}
	}
}

func TestValidateGoal(t *testing.T) {
	good := domain.Goal{Name: "Trip", OwnerID: "m1", Scope: domain.ScopeIndividual, TargetAmount: usd(100000), CurrentAmount: usd(0)}
	if is := ValidateGoal(good); !is.OK() {
		t.Errorf("valid goal failed: %v", is)
	}
	bad := domain.Goal{TargetAmount: usd(0), CurrentAmount: money.New(10, "EUR")}
	is := ValidateGoal(bad)
	for _, f := range []string{"name", "ownerId", "scope", "targetAmount", "currentAmount"} {
		if !hasField(is, f) {
			t.Errorf("expected issue for %q, got %v", f, is)
		}
	}
}

func TestValidateTask(t *testing.T) {
	good := domain.Task{Title: "Pay loan", Status: domain.StatusOpen, Priority: domain.PriorityHigh, RelatedType: domain.RelatedAccount, RelatedID: "a1"}
	if is := ValidateTask(good); !is.OK() {
		t.Errorf("valid task failed: %v", is)
	}
	bad := domain.Task{Status: "bogus", Priority: "bogus", RelatedType: domain.RelatedGoal}
	is := ValidateTask(bad)
	for _, f := range []string{"title", "status", "priority", "relatedId"} {
		if !hasField(is, f) {
			t.Errorf("expected issue for %q, got %v", f, is)
		}
	}
}

func TestIssuesError(t *testing.T) {
	is := Issues{{Field: "name", Message: "is required"}}
	if is.Error() == "" || is.OK() {
		t.Error("non-empty issues should report an error and not be OK")
	}
	if !(Issues{}).OK() {
		t.Error("empty issues should be OK")
	}
}

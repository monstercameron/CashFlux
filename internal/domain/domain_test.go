package domain

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestEnumValidAndString(t *testing.T) {
	check := func(name string, valid bool, s, want string) {
		if !valid {
			t.Errorf("%s: Valid()=false, want true", name)
		}
		if s != want {
			t.Errorf("%s: String()=%q, want %q", name, s, want)
		}
	}
	check("ClassAsset", ClassAsset.Valid(), ClassAsset.String(), "asset")
	check("TypeCreditCard", TypeCreditCard.Valid(), TypeCreditCard.String(), "credit_card")
	check("KindIncome", KindIncome.Valid(), KindIncome.String(), "income")
	check("ScopeShared", ScopeShared.Valid(), ScopeShared.String(), "shared")
	check("PeriodMonthly", PeriodMonthly.Valid(), PeriodMonthly.String(), "monthly")
	check("StatusDone", StatusDone.Valid(), StatusDone.String(), "done")
	check("PriorityHigh", PriorityHigh.Valid(), PriorityHigh.String(), "high")
	check("RelatedGoal", RelatedGoal.Valid(), RelatedGoal.String(), "goal")
	check("SourceNudge", SourceNudge.Valid(), SourceNudge.String(), "nudge")
}

func TestEnumInvalid(t *testing.T) {
	if AccountClass("nope").Valid() {
		t.Error("invalid AccountClass should be invalid")
	}
	if AccountType("nope").Valid() {
		t.Error("invalid AccountType should be invalid")
	}
	if CategoryKind("nope").Valid() {
		t.Error("invalid CategoryKind should be invalid")
	}
	if TaskPriority("urgent").Valid() {
		t.Error("invalid TaskPriority should be invalid")
	}
}

func TestAllSlicesAreValid(t *testing.T) {
	for _, c := range AllAccountClasses {
		if !c.Valid() {
			t.Errorf("AllAccountClasses has invalid %q", c)
		}
	}
	for _, ty := range AllAccountTypes {
		if !ty.Valid() {
			t.Errorf("AllAccountTypes has invalid %q", ty)
		}
	}
	for _, r := range AllRelatedTypes {
		if !r.Valid() {
			t.Errorf("AllRelatedTypes has invalid %q", r)
		}
	}
	if len(AllAccountTypes) != 11 {
		t.Errorf("AllAccountTypes len = %d, want 11", len(AllAccountTypes))
	}
}

func TestAccountTypeClass(t *testing.T) {
	liabilities := []AccountType{TypeCreditCard, TypeLineOfCredit, TypeLoan, TypePersonalLoan, TypeMortgage}
	for _, ty := range liabilities {
		if ty.Class() != ClassLiability || !ty.IsLiability() {
			t.Errorf("%s should be a liability", ty)
		}
	}
	assets := []AccountType{TypeChecking, TypeDebit, TypeSavings, TypeCash, TypeInvestment, TypeOther}
	for _, ty := range assets {
		if ty.Class() != ClassAsset || ty.IsLiability() {
			t.Errorf("%s should be an asset", ty)
		}
	}
}

func TestTransactionClassification(t *testing.T) {
	income := Transaction{Amount: money.New(100, "USD")}
	expense := Transaction{Amount: money.New(-100, "USD")}
	transfer := Transaction{Amount: money.New(-100, "USD"), TransferAccountID: "acc2"}

	if !income.IsIncome() || income.IsExpense() || income.IsTransfer() {
		t.Error("income misclassified")
	}
	if !expense.IsExpense() || expense.IsIncome() || expense.IsTransfer() {
		t.Error("expense misclassified")
	}
	if !transfer.IsTransfer() || transfer.IsIncome() || transfer.IsExpense() {
		t.Error("transfer should not count as income or expense")
	}
}

func TestEntitiesCarryCustomFields(t *testing.T) {
	// Smoke check that entities compile with the shared shapes we rely on.
	a := Account{ID: "a1", Type: TypeSavings, Class: ClassAsset, Currency: "USD", BalanceAsOf: time.Now(), Custom: map[string]any{"nickname": "rainy day"}}
	if a.Custom["nickname"] != "rainy day" {
		t.Error("custom field not stored")
	}
}

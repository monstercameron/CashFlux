package tasklink_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/tasklink"
)

func TestRoute(t *testing.T) {
	cases := []struct {
		rt   domain.RelatedType
		want string
	}{
		{domain.RelatedAccount, "/accounts"},
		{domain.RelatedBudget, "/budgets"},
		{domain.RelatedGoal, "/goals"},
		{domain.RelatedTransaction, "/transactions"},
		{domain.RelatedNone, ""},
		{domain.RelatedDocument, ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := tasklink.Route(tc.rt); got != tc.want {
			t.Errorf("Route(%q) = %q, want %q", tc.rt, got, tc.want)
		}
	}
}

func TestTypeLabel(t *testing.T) {
	if got := tasklink.TypeLabel(domain.RelatedAccount); got != "Account" {
		t.Errorf("TypeLabel(account) = %q", got)
	}
	if got := tasklink.TypeLabel(domain.RelatedNone); got != "None" {
		t.Errorf("TypeLabel(none) = %q", got)
	}
}

func TestEntityName(t *testing.T) {
	accounts := []domain.Account{{ID: "a1", Name: "Checking"}}
	budgets := []domain.Budget{{ID: "b1", Name: "Groceries"}}
	goals := []domain.Goal{{ID: "g1", Name: "Emergency Fund"}}
	transactions := []domain.Transaction{{ID: "tx1", Payee: "Amazon", Desc: "Online order"}}
	txNoPayee := []domain.Transaction{{ID: "tx2", Payee: "", Desc: "Salary"}}

	cases := []struct {
		rt       domain.RelatedType
		id       string
		wantName string
		wantOK   bool
	}{
		{domain.RelatedAccount, "a1", "Checking", true},
		{domain.RelatedAccount, "missing", "", false},
		{domain.RelatedBudget, "b1", "Groceries", true},
		{domain.RelatedGoal, "g1", "Emergency Fund", true},
		{domain.RelatedTransaction, "tx1", "Amazon", true},
		{domain.RelatedNone, "a1", "", false},
		{domain.RelatedAccount, "", "", false},
	}
	for _, tc := range cases {
		name, ok := tasklink.EntityName(tc.rt, tc.id, accounts, budgets, goals, transactions)
		if ok != tc.wantOK || name != tc.wantName {
			t.Errorf("EntityName(%q, %q) = (%q, %v), want (%q, %v)",
				tc.rt, tc.id, name, ok, tc.wantName, tc.wantOK)
		}
	}

	// Transaction with no Payee falls back to Desc.
	name, ok := tasklink.EntityName(domain.RelatedTransaction, "tx2", nil, nil, nil, txNoPayee)
	if !ok || name != "Salary" {
		t.Errorf("transaction fallback: got (%q, %v), want (\"Salary\", true)", name, ok)
	}
}

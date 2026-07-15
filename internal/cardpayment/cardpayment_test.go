// SPDX-License-Identifier: MIT

package cardpayment

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestStatementPeriodCalendarMonth(t *testing.T) {
	acct := domain.Account{ID: "c1"} // no due day -> calendar month
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	p := StatementPeriod(acct, now)
	if !p.Start.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("start = %v", p.Start)
	}
	if !p.End.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("end = %v", p.End)
	}
}

func TestStatementPeriodDueDay(t *testing.T) {
	acct := domain.Account{ID: "c1", DueDayOfMonth: 20}
	// now before the 20th -> window is [prev 20th, this 20th)
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	p := StatementPeriod(acct, now)
	if !p.Start.Equal(time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("before-day start = %v", p.Start)
	}
	if !p.End.Equal(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("before-day end = %v", p.End)
	}
	// now on/after the 20th -> window is [this 20th, next 20th)
	now2 := time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC)
	p2 := StatementPeriod(acct, now2)
	if !p2.Start.Equal(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("after-day start = %v", p2.Start)
	}
}

func TestCompute(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	card := domain.Account{ID: "c1", Name: "Visa", Class: domain.ClassLiability, Type: domain.TypeCreditCard}
	other := domain.Account{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking}

	cats := []domain.Category{
		{ID: "groceries", Name: "Groceries", Kind: domain.KindExpense},
		{ID: "dining", Name: "Dining", Kind: domain.KindExpense},
	}
	// Only groceries is budgeted.
	budgets := []domain.Budget{{ID: "b1", CategoryID: "groceries", Limit: money.New(50000, "USD")}}

	txns := []domain.Transaction{
		{ID: "1", AccountID: "c1", Date: mid, CategoryID: "groceries", Amount: money.New(-30000, "USD")},  // funded
		{ID: "2", AccountID: "c1", Date: mid, CategoryID: "dining", Amount: money.New(-12000, "USD")},     // unfunded (no budget)
		{ID: "3", AccountID: "chk", Date: mid, CategoryID: "groceries", Amount: money.New(-5000, "USD")},  // wrong account
		{ID: "4", AccountID: "c1", Date: mid, Amount: money.New(-40000, "USD"), TransferAccountID: "chk"}, // payment, excluded
	}

	got, err := Compute([]domain.Account{card, other}, txns, budgets, cats, rates, now, budgeting.MethodEnvelope)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 card, got %d", len(got))
	}
	f := got[0]
	if f.StatementMinor != 42000 {
		t.Errorf("statement = %d, want 42000", f.StatementMinor)
	}
	if f.FundedMinor != 30000 {
		t.Errorf("funded = %d, want 30000", f.FundedMinor)
	}
	if f.FullyFunded() {
		t.Error("should not be fully funded")
	}
	if f.UnfundedMinor() != 12000 {
		t.Errorf("unfunded = %d, want 12000", f.UnfundedMinor())
	}
}

func TestComputeScopedToEnvelopeFlex(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	card := domain.Account{ID: "c1", Name: "Visa", Class: domain.ClassLiability, Type: domain.TypeCreditCard}
	txns := []domain.Transaction{
		{ID: "1", AccountID: "c1", Date: now, CategoryID: "g", Amount: money.New(-100, "USD")},
	}
	for _, m := range []budgeting.Methodology{budgeting.MethodSimple, budgeting.MethodZeroBased} {
		got, err := Compute([]domain.Account{card}, txns, nil, nil, rates, now, m)
		if err != nil {
			t.Fatalf("compute %s: %v", m, err)
		}
		if got != nil {
			t.Errorf("method %s should yield no funding, got %+v", m, got)
		}
	}
	got, err := Compute([]domain.Account{card}, txns, nil, nil, rates, now, budgeting.MethodFlex)
	if err != nil {
		t.Fatalf("compute flex: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("flex should compute, got %+v", got)
	}
}

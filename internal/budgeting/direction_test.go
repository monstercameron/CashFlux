// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// A saving budget is the same arithmetic as a spending budget read the opposite
// way: the number is a floor to reach rather than a ceiling not to breach. These
// pin that inversion, and the one scope difference it implies — a transfer is
// not spending, but it IS saving.

func dirBudget(dir domain.BudgetDirection, limitMinor int64) domain.Budget {
	return domain.Budget{
		ID: "b1", Name: "Investments", CategoryID: "invest",
		Period: domain.PeriodMonthly, Limit: money.New(limitMinor, "USD"),
		Direction: dir,
	}
}

func dirTxn(id string, minor int64, transfer bool) domain.Transaction {
	t := domain.Transaction{
		ID: id, Desc: id, CategoryID: "invest",
		Date:   time.Date(2024, time.January, 10, 0, 0, 0, 0, time.UTC),
		Amount: money.New(minor, "USD"),
	}
	if transfer {
		t.TransferAccountID = "acct-brokerage"
	}
	return t
}

func dirRange() (time.Time, time.Time) {
	return time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC)
}

// TestSavingBudgetInvertsTheHealthyState is the core of it: under a spending cap
// passing the number is the failure; under a saving target it is the goal.
func TestSavingBudgetInvertsTheHealthyState(t *testing.T) {
	start, end := dirRange()
	rates := currency.Rates{Base: "USD"}
	cases := []struct {
		name      string
		dir       domain.BudgetDirection
		moved     int64 // negative = money out
		wantState State
	}{
		{"spending, well under", domain.DirectionSpend, -10000, StateOK},
		{"spending, near the cap", domain.DirectionSpend, -45000, StateNear},
		{"spending, past the cap", domain.DirectionSpend, -60000, StateOver},
		// Same numbers, opposite readings.
		{"saving, barely started", domain.DirectionSave, -10000, StateOver},
		{"saving, nearly there", domain.DirectionSave, -45000, StateNear},
		{"saving, funded", domain.DirectionSave, -50000, StateOK},
		{"saving, more than asked", domain.DirectionSave, -60000, StateOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, err := Evaluate(dirBudget(tc.dir, 50000), []domain.Transaction{dirTxn("t1", tc.moved, false)},
				start, end, rates, DefaultNearThreshold)
			if err != nil {
				t.Fatal(err)
			}
			if st.State != tc.wantState {
				t.Errorf("State = %q, want %q (moved %d of 50000)", st.State, tc.wantState, tc.moved)
			}
		})
	}
}

// TestSavingBudgetCountsTransfers is C539. The ordinary way to put money into
// investments is a transfer from checking, and a spending budget rightly ignores
// transfers — so without this a savings budget would read $0 forever.
func TestSavingBudgetCountsTransfers(t *testing.T) {
	start, end := dirRange()
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{dirTxn("t1", -50000, true)}

	saving, err := Evaluate(dirBudget(domain.DirectionSave, 50000), txns, start, end, rates, DefaultNearThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if saving.Spent.Amount != 50000 {
		t.Errorf("a saving budget must count the transfer: got %d, want 50000", saving.Spent.Amount)
	}
	if saving.State != StateOK {
		t.Errorf("State = %q, want ok — the target was met", saving.State)
	}

	// The same transfer must NOT count against a spending budget: money moved
	// between your own accounts was not spent, and counting it would
	// double-report it against the category.
	spending, err := Evaluate(dirBudget(domain.DirectionSpend, 50000), txns, start, end, rates, DefaultNearThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if spending.Spent.Amount != 0 {
		t.Errorf("a spending budget must ignore the transfer: got %d, want 0", spending.Spent.Amount)
	}
}

// TestSavingBudgetIgnoresMoneyComingBack: only the leg that LEAVES counts, so a
// matching pair of transfer legs cannot be counted twice, and a withdrawal from
// the investment account is not a contribution.
func TestSavingBudgetIgnoresMoneyComingBack(t *testing.T) {
	start, end := dirRange()
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{
		dirTxn("out", -50000, true), // leaves checking
		dirTxn("in", 50000, true),   // arrives in the brokerage — the other leg
	}
	st, err := Evaluate(dirBudget(domain.DirectionSave, 50000), txns, start, end, rates, DefaultNearThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if st.Spent.Amount != 50000 {
		t.Errorf("both legs of one move must count once: got %d, want 50000", st.Spent.Amount)
	}
}

// TestSavingBudgetWithNoTargetIsNotFailing guards the zero-limit edge: a budget
// asking for nothing cannot be short of it.
func TestSavingBudgetWithNoTargetIsNotFailing(t *testing.T) {
	start, end := dirRange()
	st, err := Evaluate(dirBudget(domain.DirectionSave, 0), nil, start, end, currency.Rates{Base: "USD"}, DefaultNearThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateOK {
		t.Errorf("State = %q, want ok", st.State)
	}
}

// TestExistingBudgetsAreUnchanged: Direction's zero value is the historical
// spending cap, so every budget in every existing dataset keeps its meaning with
// no migration.
func TestExistingBudgetsAreUnchanged(t *testing.T) {
	start, end := dirRange()
	rates := currency.Rates{Base: "USD"}
	var b domain.Budget
	b = dirBudget("", 50000)
	if b.IsSaving() {
		t.Fatal("the zero value must be a spending cap")
	}
	st, err := Evaluate(b, []domain.Transaction{dirTxn("t1", -60000, false)}, start, end, rates, DefaultNearThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateOver {
		t.Errorf("State = %q, want over — an unmarked budget is still a cap", st.State)
	}
}

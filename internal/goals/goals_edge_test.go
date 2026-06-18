package goals

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// mismatchGoal has its target and current in different currencies, so the money
// Sub/Cmp operations error — exercising the error paths.
func mismatchGoal() domain.Goal {
	return domain.Goal{TargetAmount: money.New(100000, "USD"), CurrentAmount: money.New(30000, "EUR")}
}

func TestRemainingCurrencyMismatch(t *testing.T) {
	if _, err := Remaining(mismatchGoal()); err == nil {
		t.Error("Remaining should error when target and current currencies differ")
	}
}

func TestIsCompleteCurrencyMismatch(t *testing.T) {
	if _, err := IsComplete(mismatchGoal()); err == nil {
		t.Error("IsComplete should error when target and current currencies differ")
	}
}

func TestProjectRemainingError(t *testing.T) {
	from := mustDate("2026-06-15")
	// The internal Remaining call errors on mismatched goal currencies, so Project
	// surfaces it before reaching the monthly-currency check.
	if _, _, err := Project(mismatchGoal(), usd(20000), from); err == nil {
		t.Error("Project should propagate the Remaining currency-mismatch error")
	}
}

func TestEvaluateErrorPaths(t *testing.T) {
	from := mustDate("2026-06-15")

	// Remaining (the first call) errors → Evaluate errors.
	if _, err := Evaluate(mismatchGoal(), usd(20000), from); err == nil {
		t.Error("Evaluate should surface the Remaining error for a mismatched goal")
	}

	// A valid goal but a monthly contribution in another currency makes the
	// internal Project call error — Evaluate must surface that too.
	if _, err := Evaluate(goal(100000, 40000), money.New(20000, "EUR"), from); err == nil {
		t.Error("Evaluate should surface the Project currency-mismatch error")
	}
}

func TestMonthlyNeededPartialFinalMonth(t *testing.T) {
	// from day-of-month (15) < target day-of-month (20): the partial final month
	// still needs a contribution, so the month count rounds up from 3 to 4.
	from := mustDate("2026-01-15")
	g := domain.Goal{TargetAmount: usd(40000), CurrentAmount: usd(0), TargetDate: mustDate("2026-04-20")}
	per, ok, err := MonthlyNeeded(g, from)
	if err != nil || !ok {
		t.Fatalf("expected a projection, ok=%v err=%v", ok, err)
	}
	// $400 over 4 months (3 full + 1 partial) → $100/mo.
	if per.Amount != 10000 {
		t.Errorf("per month = %d, want 10000 ($100 over 4 months incl. the partial)", per.Amount)
	}
}

func TestMonthlyNeededCurrencyMismatch(t *testing.T) {
	from := mustDate("2026-01-15")
	g := domain.Goal{
		TargetAmount:  money.New(100000, "USD"),
		CurrentAmount: money.New(30000, "EUR"),
		TargetDate:    mustDate("2027-01-15"),
	}
	if _, _, err := MonthlyNeeded(g, from); err == nil {
		t.Error("MonthlyNeeded should surface the Remaining currency-mismatch error")
	}
}

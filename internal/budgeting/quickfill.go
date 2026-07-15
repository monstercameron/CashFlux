package budgeting

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// QuickFill keys — stable identifiers the UI maps to localized chip labels (BG4).
const (
	QuickFillLastMonth   = "last-month"  // last full calendar month's spending
	QuickFillAvg3        = "avg-3"       // average spend over the last 3 months
	QuickFillAvg6        = "avg-6"       // average spend over the last 6 months
	QuickFillLastPeriod  = "last-period" // last period's effective budget (limit + boost)
	QuickFillUnderfunded = "underfunded" // amount still needed to fund the target (BG1)
)

// QuickFill is one computed one-tap fill suggestion for a budget's amount (BG4).
// The UI renders it as a chip showing a localized label plus fmtMoney(Amount);
// Key identifies which suggestion it is so the label copy lives in i18n, not here.
type QuickFill struct {
	// Key is the stable suggestion identifier (one of the QuickFill* constants).
	Key string
	// Amount is the computed fill value in the budget's limit currency.
	Amount money.Money
}

// QuickFillInput carries everything QuickFills needs beyond the transaction
// history: the reference "now", the household week-start (for period math), the
// FX rates, and the target underfunding from BG1 (Needed().Needed). Pass
// HasUnderfunded=false to omit the underfunded chip (e.g. the budget has no target).
type QuickFillInput struct {
	Now            time.Time
	WeekStart      time.Weekday
	Rates          currency.Rates
	Underfunded    money.Money
	HasUnderfunded bool
}

// QuickFills computes the one-tap fill figures for a budget (BG4): last full
// month's spending, the trailing 3- and 6-month averages, last period's effective
// budget, and — when in.HasUnderfunded — the amount still needed to fund the
// budget's target (from BG1's Needed). Spending figures reuse budgeting.Spent so a
// multi-category or rollup budget's own tracked categories are honoured. Suggestions
// whose value cannot be computed (an FX gap in a month) are simply omitted.
func QuickFills(budget domain.Budget, all []domain.Transaction, in QuickFillInput) []QuickFill {
	cur := normalizedLimit(budget, in.Rates).Currency
	var out []QuickFill

	spends, ok := monthlySpends(budget, all, in.Now, in.Rates, 6)
	if ok {
		out = append(out, QuickFill{Key: QuickFillLastMonth, Amount: spends[0]})
		out = append(out, QuickFill{Key: QuickFillAvg3, Amount: averageMonths(spends[:3], cur)})
		out = append(out, QuickFill{Key: QuickFillAvg6, Amount: averageMonths(spends[:6], cur)})
	}

	// Last period's effective budget: the base limit plus any one-time boost that
	// applied to the period immediately before the current one.
	start, _ := PeriodRange(budget.Period, in.Now, in.WeekStart)
	prevStart, _ := PeriodRange(budget.Period, start.AddDate(0, 0, -1), in.WeekStart)
	lastPeriod := normalizedLimit(budget, in.Rates).Amount + budget.PeriodBoost(prevStart)
	out = append(out, QuickFill{Key: QuickFillLastPeriod, Amount: money.New(lastPeriod, cur)})

	if in.HasUnderfunded {
		out = append(out, QuickFill{Key: QuickFillUnderfunded, Amount: money.New(in.Underfunded.Amount, cur)})
	}
	return out
}

// monthlySpends returns the budget's spend over each of the last n full calendar
// months (index 0 = the month immediately before Now's month, most recent first),
// in the budget's limit currency. ok is false if any month fails to evaluate.
func monthlySpends(budget domain.Budget, all []domain.Transaction, now time.Time, rates currency.Rates, n int) ([]money.Money, bool) {
	out := make([]money.Money, n)
	for i := 0; i < n; i++ {
		start, end := dateutil.MonthRange(dateutil.AddMonths(now, -(i + 1)))
		spent, err := Spent(budget, all, start, end, rates)
		if err != nil {
			return nil, false
		}
		out[i] = spent
	}
	return out, true
}

// averageMonths returns the mean of the given monthly spends (integer division,
// floored), in currency cur. An empty slice yields zero.
func averageMonths(months []money.Money, cur string) money.Money {
	if len(months) == 0 {
		return money.Zero(cur)
	}
	var sum int64
	for _, m := range months {
		sum += m.Amount
	}
	return money.New(sum/int64(len(months)), cur)
}

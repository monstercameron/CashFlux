package reports

// Runway estimates how long a cash balance would last at a steady monthly burn —
// the "how many months of buffer do I have?" metric.
type Runway struct {
	// Months is the whole months the balance covers at the given burn.
	Months int
	// Days is the additional whole days after Months (0–29), prorating the
	// leftover against a 30-day month.
	Days int
	// Sustainable is true when the burn is non-positive (income covers spending),
	// so the balance never runs down — Months/Days are then meaningless.
	Sustainable bool
}

// EstimateRunway computes how long balanceMinor lasts at monthlyBurnMinor, both
// in base-currency minor units. A non-positive burn is Sustainable (the balance
// never depletes). A non-positive balance with a positive burn is zero runway.
func EstimateRunway(balanceMinor, monthlyBurnMinor int64) Runway {
	if monthlyBurnMinor <= 0 {
		return Runway{Sustainable: true}
	}
	if balanceMinor <= 0 {
		return Runway{}
	}
	months := balanceMinor / monthlyBurnMinor
	rem := balanceMinor % monthlyBurnMinor
	// Prorate the leftover into days against a 30-day month. rem < burn, so this
	// is always in [0, 29].
	days := int(rem * 30 / monthlyBurnMinor)
	return Runway{Months: int(months), Days: days}
}

// AverageMonthlyExpense averages the Expense across monthly period flows,
// skipping fully-inactive buckets (no income AND no expense) so empty months
// with no data don't drag the average toward zero. It returns 0 when no bucket
// had any activity. Pass monthly buckets (e.g. from IncomeExpenseSeries over
// monthly bounds) for the figure to mean "per month".
func AverageMonthlyExpense(flows []PeriodFlow) int64 {
	var sum, n int64
	for _, f := range flows {
		if f.Income == 0 && f.Expense == 0 {
			continue
		}
		sum += f.Expense
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

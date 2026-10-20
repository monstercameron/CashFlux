// SPDX-License-Identifier: MIT

package reports

import "github.com/monstercameron/CashFlux/internal/money"

// Runway estimates how long a cash balance would last at a steady monthly burn —
// the "how many months of buffer do I have?" metric.
type Runway struct {
	// Months is the whole months the balance covers at the given burn. Keep the
	// quotient wide on 32-bit WebAssembly even when a balance spans many months.
	Months int64
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
	// The prorated remainder always fits; its multiplication by 30 need not.
	dayRatio, _ := money.MulDiv(rem, 30, monthlyBurnMinor, money.RoundTowardZero)
	days := int(dayRatio)
	return Runway{Months: months, Days: days}
}

// AverageMonthlyExpense averages the Expense across monthly period flows,
// skipping fully-inactive buckets (no income AND no expense) so empty months
// with no data don't drag the average toward zero. It returns 0 when no bucket
// had any activity. Pass monthly buckets (e.g. from IncomeExpenseSeries over
// monthly bounds) for the figure to mean "per month".
func AverageMonthlyExpense(flows []PeriodFlow) int64 {
	_, expense, _, _ := MonthlyAveragesChecked(flows)
	return expense
}

// AverageMonthlyIncome is AverageMonthlyExpense's income twin: the trailing
// monthly income average over the active buckets, 0 when none had activity.
func AverageMonthlyIncome(flows []PeriodFlow) int64 {
	income, _, _, _ := MonthlyAveragesChecked(flows)
	return income
}

// MonthlyAveragesChecked derives both monthly averages over active buckets in
// one pass. Unrepresentable sums return an error and no partial averages.
func MonthlyAveragesChecked(flows []PeriodFlow) (income, expense int64, active int, err error) {
	for _, f := range flows {
		if f.Income == 0 && f.Expense == 0 {
			continue
		}
		income, err = money.Add(income, f.Income)
		if err != nil {
			return 0, 0, 0, err
		}
		expense, err = money.Add(expense, f.Expense)
		if err != nil {
			return 0, 0, 0, err
		}
		active++
	}
	if active > 0 {
		income /= int64(active)
		expense /= int64(active)
	}
	return income, expense, active, nil
}

// ActiveMonths counts the buckets with any activity — the honest "averaged over
// N months" caption for the two averages above.
func ActiveMonths(flows []PeriodFlow) int {
	n := 0
	for _, f := range flows {
		if f.Income != 0 || f.Expense != 0 {
			n++
		}
	}
	return n
}

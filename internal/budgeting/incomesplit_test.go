// SPDX-License-Identifier: MIT

package budgeting

import "testing"

// The month from the screenshot that prompted this (Jul 2026), in minor units.
// Four budgets: Misc·Bars 4,108.70 of 1,000 · House Keeper 200 of 100 ·
// Tesla Loan 750 of 750 · Bills 1,777.53 of 2,500. Untracked 3,723.62.
// Savings targets 2,500. Income 7,021.19.
func julyRollup() RollupSummary {
	return RollupSummary{
		SpentMinor: 683623, // 410870 + 20000 + 75000 + 177753
		LimitMinor: 435000,
		OverMinor:  320870, // 310870 over on bars + 10000 on the housekeeper
	}
}

func TestTheOverageIsThePerBudgetSumNotSpendMinusLimits(t *testing.T) {
	// The defect this file exists for. The hero showed 683623-435000 = 248623 and
	// called it "over budget", which is the real overage NETTED against the bills
	// budget's 72247 of headroom — a number that appears in none of the rows and
	// that nobody can act on, because money in one budget is not available to
	// another unless somebody moves it.
	s := SplitFromRollup(julyRollup(), 702119, 372362, 250000)
	if s.OverLimits != 320870 {
		t.Errorf("OverLimits = %d, want 320870 (310870 + 10000 from the rows)", s.OverLimits)
	}
	if s.OverLimits == 248623 {
		t.Error("OverLimits is spend-minus-limits again — the netted figure is back")
	}
}

func TestWithinLimitsExcludesTheOverspend(t *testing.T) {
	s := SplitFromRollup(julyRollup(), 702119, 372362, 250000)
	if s.WithinLimits != 362753 {
		t.Errorf("WithinLimits = %d, want 362753", s.WithinLimits)
	}
	// And the two together are the tracked spend, with nothing counted twice.
	if s.WithinLimits+s.OverLimits != julyRollup().SpentMinor {
		t.Errorf("within+over = %d, want %d", s.WithinLimits+s.OverLimits, julyRollup().SpentMinor)
	}
}

func TestTheSegmentsReconcileToIncome(t *testing.T) {
	// The property that makes the bar drawable as one track: every part plus what
	// is left IS the income. A bar whose segments do not sum to its own total is
	// worse than no bar.
	s := SplitFromRollup(julyRollup(), 702119, 372362, 250000)
	total := s.WithinLimits + s.OverLimits + s.Untracked + s.Savings + s.Left
	if total != s.Income {
		t.Errorf("segments sum to %d, income is %d (drift %d)", total, s.Income, total-s.Income)
	}
}

func TestAMonthCanBeOverIncomeAndSaySo(t *testing.T) {
	// The fact the old hero could not state at all. Spend 683623 + untracked
	// 372362 + savings 250000 = 1305985 against 702119 of income.
	s := SplitFromRollup(julyRollup(), 702119, 372362, 250000)
	if !s.Overspent() {
		t.Errorf("Left = %d; this month is well past its income and should say so", s.Left)
	}
	if s.Spent() != 1055985 {
		t.Errorf("Spent = %d, want 1055985 (tracked 683623 + untracked 372362)", s.Spent())
	}
}

func TestUntrackedIsCountedAtAll(t *testing.T) {
	// It was a footnote under the list. In this month it is larger than every
	// budget except one, and leaving it out is how a page says "97% budgeted"
	// while thousands leave unaccounted.
	with := SplitFromRollup(julyRollup(), 702119, 372362, 250000)
	without := SplitFromRollup(julyRollup(), 702119, 0, 250000)
	if with.Left == without.Left {
		t.Error("untracked spending does not move the bottom line")
	}
	if with.Untracked != 372362 {
		t.Errorf("Untracked = %d, want 372362", with.Untracked)
	}
}

func TestSavingsIsNotSpending(t *testing.T) {
	// Money that stayed. Counting it as an outflow makes prudence look like a
	// problem, and the two must not be summed into one bar segment.
	s := SplitFromRollup(julyRollup(), 702119, 372362, 250000)
	if s.Spent() != s.WithinLimits+s.OverLimits+s.Untracked {
		t.Error("Savings leaked into Spent")
	}
}

func TestAHealthyMonthLeavesSomethingAndIsNotOverspent(t *testing.T) {
	r := RollupSummary{SpentMinor: 200000, LimitMinor: 300000, OverMinor: 0}
	s := SplitFromRollup(r, 500000, 50000, 100000)
	if s.Overspent() {
		t.Errorf("Left = %d; this month is inside its income", s.Left)
	}
	if s.Left != 150000 {
		t.Errorf("Left = %d, want 150000", s.Left)
	}
	if s.OverLimits != 0 {
		t.Errorf("OverLimits = %d, want 0 — nothing exceeded a limit", s.OverLimits)
	}
}

func TestPctNeverDividesByAnAbsentIncome(t *testing.T) {
	// A month with no recorded income is not a month where everything is 100% of
	// nothing — the caller draws an empty state instead.
	s := SplitFromRollup(julyRollup(), 0, 372362, 250000)
	if got := s.Pct(s.Untracked); got != 0 {
		t.Errorf("Pct with no income = %d, want 0", got)
	}
	neg := SplitFromRollup(julyRollup(), -100, 0, 0)
	if got := neg.Pct(1000); got != 0 {
		t.Errorf("Pct with negative income = %d, want 0", got)
	}
}

func TestPctClampsAndIgnoresNonPositiveParts(t *testing.T) {
	s := SplitFromRollup(julyRollup(), 100000, 0, 0)
	if got := s.Pct(500000); got != 100 {
		t.Errorf("Pct(500%%) = %d, want 100", got)
	}
	if got := s.Pct(0); got != 0 {
		t.Errorf("Pct(0) = %d, want 0", got)
	}
	if got := s.Pct(-5); got != 0 {
		t.Errorf("Pct(negative) = %d, want 0", got)
	}
}

func TestWithinLimitsNeverGoesNegative(t *testing.T) {
	// A rollup where over exceeds spent should not produce a negative segment that
	// would render as a bar growing backwards.
	r := RollupSummary{SpentMinor: 100, OverMinor: 500}
	s := SplitFromRollup(r, 10000, 0, 0)
	if s.WithinLimits < 0 {
		t.Errorf("WithinLimits = %d", s.WithinLimits)
	}
}

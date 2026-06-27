// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(n int64) money.Money { return money.New(n, "USD") }

func mustDate(s string) time.Time {
	t, err := dateutil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

var june = func() (start, end time.Time) { return dateutil.MonthRange(mustDate("2026-06-15")) }

func TestPeriodRange(t *testing.T) {
	ref := mustDate("2026-06-15") // a Monday

	// Monthly: the whole of June.
	s, e := PeriodRange(domain.PeriodMonthly, ref, time.Sunday)
	if s != mustDate("2026-06-01") || e != mustDate("2026-07-01") {
		t.Errorf("monthly = %v..%v", s.Format("2006-01-02"), e.Format("2006-01-02"))
	}

	// Weekly (Sunday start): 2026-06-14 .. 2026-06-21.
	s, e = PeriodRange(domain.PeriodWeekly, ref, time.Sunday)
	if s != mustDate("2026-06-14") || e != mustDate("2026-06-21") {
		t.Errorf("weekly(Sun) = %v..%v", s.Format("2006-01-02"), e.Format("2006-01-02"))
	}
	// Weekly (Monday start): 2026-06-15 .. 2026-06-22.
	s, e = PeriodRange(domain.PeriodWeekly, ref, time.Monday)
	if s != mustDate("2026-06-15") || e != mustDate("2026-06-22") {
		t.Errorf("weekly(Mon) = %v..%v", s.Format("2006-01-02"), e.Format("2006-01-02"))
	}

	// Quarterly: Q2 is Apr 1 .. Jul 1.
	s, e = PeriodRange(domain.PeriodQuarterly, ref, time.Sunday)
	if s != mustDate("2026-04-01") || e != mustDate("2026-07-01") {
		t.Errorf("quarterly = %v..%v", s.Format("2006-01-02"), e.Format("2026-07-01"))
	}

	// Yearly: Jan 1, 2026 .. Jan 1, 2027.
	s, e = PeriodRange(domain.PeriodYearly, ref, time.Sunday)
	if s != mustDate("2026-01-01") || e != mustDate("2027-01-01") {
		t.Errorf("yearly = %v..%v", s.Format("2006-01-02"), e.Format("2006-01-02"))
	}
}

// TestPeriodRangeBiweekly verifies that biweekly windows are exactly 14 days,
// contiguous (no gaps or overlaps between consecutive fortnight windows), and
// that a date mid-window falls inside the correct window.
func TestPeriodRangeBiweekly(t *testing.T) {
	tests := []struct {
		name      string
		ref       string
		weekStart time.Weekday
		wantStart string
		wantEnd   string
	}{
		// Epoch (2006-01-02) is itself a Monday; first fortnight begins there.
		{"epoch Monday itself", "2006-01-02", time.Monday, "2006-01-02", "2006-01-16"},
		{"epoch+1 (Tuesday mid-window)", "2006-01-03", time.Monday, "2006-01-02", "2006-01-16"},
		{"epoch+13 (Sunday, last day of window)", "2006-01-15", time.Monday, "2006-01-02", "2006-01-16"},
		{"epoch+14 (Monday, start of next window)", "2006-01-16", time.Monday, "2006-01-16", "2006-01-30"},
		// 2026-06-15 is a Monday; verify it lands in a real fortnight.
		{"2026-06-15 Monday", "2026-06-15", time.Monday, "2026-06-08", "2026-06-22"},
		{"2026-06-14 Sunday (mid window, Mon-start)", "2026-06-14", time.Monday, "2026-06-08", "2026-06-22"},
		// Sunday-start grid: 2026-06-14 is itself a Sunday, so it is exactly on a
		// Sun-anchored fortnight boundary — it starts a new 14-day window.
		{"epoch anchor shifts for Sun-start", "2026-06-14", time.Sunday, "2026-06-14", "2026-06-28"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := mustDate(tt.ref)
			s, e := PeriodRange(domain.PeriodBiweekly, ref, tt.weekStart)
			if s != mustDate(tt.wantStart) || e != mustDate(tt.wantEnd) {
				t.Errorf("biweekly(%s, ws=%v) = %s..%s, want %s..%s",
					tt.ref, tt.weekStart,
					s.Format("2006-01-02"), e.Format("2006-01-02"),
					tt.wantStart, tt.wantEnd)
			}
			// Window must be exactly 14 days.
			if int(e.Sub(s).Hours()) != 14*24 {
				t.Errorf("biweekly window is %v, want 336h", e.Sub(s))
			}
			// ref must be inside [start, end).
			if ref.Before(s) || !ref.Before(e) {
				t.Errorf("ref %s not in [%s, %s)", tt.ref, tt.wantStart, tt.wantEnd)
			}
		})
	}

	// Contiguity: consecutive biweekly windows must be gap-free.
	base := mustDate("2026-01-01")
	prev := mustDate("2026-01-01")
	for i := 0; i < 26; i++ {
		probe := base.AddDate(0, 0, i*14)
		s, e := PeriodRange(domain.PeriodBiweekly, probe, time.Monday)
		if i > 0 && s != prev {
			t.Errorf("gap between window %d and %d: prev end %s, next start %s",
				i-1, i, prev.Format("2006-01-02"), s.Format("2006-01-02"))
		}
		prev = e
	}
}

// TestPeriodRangeSemimonthly verifies first-half (1st–15th) and second-half
// (16th–end-of-month) windows across varying month lengths.
func TestPeriodRangeSemimonthly(t *testing.T) {
	tests := []struct {
		name      string
		ref       string
		wantStart string
		wantEnd   string
	}{
		// First half
		{"1st of month → first half", "2026-06-01", "2026-06-01", "2026-06-16"},
		{"mid first half (10th)", "2026-06-10", "2026-06-01", "2026-06-16"},
		{"15th (last day of first half)", "2026-06-15", "2026-06-01", "2026-06-16"},
		// Second half
		{"16th → second half", "2026-06-16", "2026-06-16", "2026-07-01"},
		{"mid second half (20th)", "2026-06-20", "2026-06-16", "2026-07-01"},
		{"30th (last day of 30-day month)", "2026-06-30", "2026-06-16", "2026-07-01"},
		// 31-day month
		{"31-day month: 16th→end", "2026-01-31", "2026-01-16", "2026-02-01"},
		// February (28 days in non-leap year)
		{"Feb non-leap: first half", "2026-02-10", "2026-02-01", "2026-02-16"},
		{"Feb non-leap: second half (17th)", "2026-02-17", "2026-02-16", "2026-03-01"},
		{"Feb non-leap: 28th", "2026-02-28", "2026-02-16", "2026-03-01"},
		// February in a leap year
		{"Feb leap: second half (16th)", "2024-02-16", "2024-02-16", "2024-03-01"},
		{"Feb leap: 29th", "2024-02-29", "2024-02-16", "2024-03-01"},
		// December year-boundary
		{"Dec: second half → Jan 1", "2026-12-25", "2026-12-16", "2027-01-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := mustDate(tt.ref)
			s, e := PeriodRange(domain.PeriodSemimonthly, ref, time.Sunday)
			if s != mustDate(tt.wantStart) || e != mustDate(tt.wantEnd) {
				t.Errorf("semimonthly(%s) = %s..%s, want %s..%s",
					tt.ref,
					s.Format("2006-01-02"), e.Format("2006-01-02"),
					tt.wantStart, tt.wantEnd)
			}
			// ref must be inside [start, end).
			if ref.Before(s) || !ref.Before(e) {
				t.Errorf("ref %s not in [%s, %s)", tt.ref, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func expense(amount int64, cur, cat, member, day string) domain.Transaction {
	return domain.Transaction{
		Amount:     money.New(-amount, cur),
		CategoryID: cat,
		MemberID:   member,
		Date:       mustDate(day),
	}
}

// TestPeriodRangeAnchored verifies that PeriodRangeAnchored snaps biweekly grids
// to the user-supplied payday anchor date.
func TestPeriodRangeAnchored(t *testing.T) {
	// anchor = 2026-06-05 (a Friday — simulates a user whose payday is every
	// other Friday).
	anchor := mustDate("2026-06-05")

	t.Run("ref within anchor fortnight returns [anchor, anchor+14)", func(t *testing.T) {
		// 2026-06-10 is 5 days after the anchor — inside the first fortnight.
		ref := mustDate("2026-06-10")
		s, e := PeriodRangeAnchored(domain.PeriodBiweekly, ref, time.Monday, anchor)
		if s != mustDate("2026-06-05") || e != mustDate("2026-06-19") {
			t.Errorf("got %s..%s, want 2026-06-05..2026-06-19", s.Format("2006-01-02"), e.Format("2006-01-02"))
		}
		if int(e.Sub(s).Hours()) != 14*24 {
			t.Errorf("window %v, want 336h", e.Sub(s))
		}
	})

	t.Run("ref one fortnight later returns next block", func(t *testing.T) {
		// 2026-06-19 is exactly anchor + 14 days — starts the second fortnight.
		ref := mustDate("2026-06-19")
		s, e := PeriodRangeAnchored(domain.PeriodBiweekly, ref, time.Monday, anchor)
		if s != mustDate("2026-06-19") || e != mustDate("2026-07-03") {
			t.Errorf("got %s..%s, want 2026-06-19..2026-07-03", s.Format("2006-01-02"), e.Format("2006-01-02"))
		}
	})

	t.Run("ref before anchor falls in previous fortnight", func(t *testing.T) {
		// 2026-05-25 is 11 days before the anchor — sits in the prior window.
		ref := mustDate("2026-05-25")
		s, e := PeriodRangeAnchored(domain.PeriodBiweekly, ref, time.Monday, anchor)
		if s != mustDate("2026-05-22") || e != mustDate("2026-06-05") {
			t.Errorf("got %s..%s, want 2026-05-22..2026-06-05", s.Format("2006-01-02"), e.Format("2006-01-02"))
		}
		if int(e.Sub(s).Hours()) != 14*24 {
			t.Errorf("window %v, want 336h", e.Sub(s))
		}
		// ref must be inside [start, end).
		if ref.Before(s) || !ref.Before(e) {
			t.Errorf("ref %s not in [%s, %s)", ref.Format("2006-01-02"), s.Format("2006-01-02"), e.Format("2006-01-02"))
		}
	})

	t.Run("zero anchor falls back to PeriodRange", func(t *testing.T) {
		ref := mustDate("2026-06-15")
		got, gote := PeriodRangeAnchored(domain.PeriodBiweekly, ref, time.Monday, time.Time{})
		want, wante := PeriodRange(domain.PeriodBiweekly, ref, time.Monday)
		if got != want || gote != wante {
			t.Errorf("zero anchor: got %s..%s, want %s..%s",
				got.Format("2006-01-02"), gote.Format("2006-01-02"),
				want.Format("2006-01-02"), wante.Format("2006-01-02"))
		}
	})

	t.Run("non-biweekly period delegates to PeriodRange regardless of anchor", func(t *testing.T) {
		ref := mustDate("2026-06-15")
		got, gote := PeriodRangeAnchored(domain.PeriodMonthly, ref, time.Monday, anchor)
		want, wante := PeriodRange(domain.PeriodMonthly, ref, time.Monday)
		if got != want || gote != wante {
			t.Errorf("monthly: got %s..%s, want %s..%s",
				got.Format("2006-01-02"), gote.Format("2006-01-02"),
				want.Format("2006-01-02"), wante.Format("2006-01-02"))
		}
	})
}

func TestSpentIndividualScope(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "m1", Limit: usd(50000)}
	all := []domain.Transaction{
		expense(10000, "USD", "food", "m1", "2026-06-03"),                                     // counts
		expense(5000, "USD", "food", "m2", "2026-06-04"),                                      // other member, excluded
		expense(3000, "USD", "rent", "m1", "2026-06-05"),                                      // other category, excluded
		expense(2000, "USD", "food", "m1", "2026-07-02"),                                      // out of period, excluded
		{Amount: usd(9999), CategoryID: "food", MemberID: "m1", Date: mustDate("2026-06-06")}, // income, excluded
	}
	spent, err := Spent(budget, all, start, end, rates)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !spent.Equal(usd(10000)) {
		t.Errorf("spent = %v, want 10000 USD", spent)
	}
}

func TestSpentSharedScope(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: usd(50000)}
	all := []domain.Transaction{
		expense(10000, "USD", "food", "m1", "2026-06-03"),
		expense(5000, "USD", "food", "m2", "2026-06-04"),
	}
	spent, err := Spent(budget, all, start, end, rates)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !spent.Equal(usd(15000)) {
		t.Errorf("spent = %v, want 15000 USD (all members)", spent)
	}
}

func TestSpentIgnoresTransfers(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: usd(50000)}
	all := []domain.Transaction{
		expense(2000, "USD", "food", "", "2026-06-03"),
		{
			AccountID: "checking", TransferAccountID: "savings", CategoryID: "food",
			Amount: usd(-9000), Date: mustDate("2026-06-04"),
		},
	}

	spent, err := Spent(budget, all, start, end, rates)
	if err != nil {
		t.Fatalf("Spent: %v", err)
	}
	if !spent.Equal(usd(2000)) {
		t.Errorf("spent with transfer = %v, want 2000 USD", spent)
	}
}

// C58: a split transaction counts only the split lines whose category the budget
// covers, attributed per line — never the whole transaction — so a grocery
// receipt split into food/household lands the food portion in the food budget.
func TestSpentSplitTransactionAttributesPerCategory(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	foodBudget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: usd(50000)}
	householdBudget := domain.Budget{CategoryID: "household", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: usd(50000)}
	// $100 charge, whole-transaction category empty, split $70 food / $30 household.
	split := domain.Transaction{
		CategoryID: "", Amount: usd(-10000), Date: mustDate("2026-06-05"),
		Splits: []domain.CategorySplit{
			{CategoryID: "food", Amount: usd(-7000)},
			{CategoryID: "household", Amount: usd(-3000)},
		},
	}
	all := []domain.Transaction{split, expense(2000, "USD", "food", "", "2026-06-06")}

	foodSpent, err := Spent(foodBudget, all, start, end, rates)
	if err != nil {
		t.Fatalf("Spent(food): %v", err)
	}
	if !foodSpent.Equal(usd(9000)) { // 7000 split line + 2000 plain food
		t.Errorf("food spent = %v, want 9000 USD", foodSpent)
	}
	hhSpent, err := Spent(householdBudget, all, start, end, rates)
	if err != nil {
		t.Fatalf("Spent(household): %v", err)
	}
	if !hhSpent.Equal(usd(3000)) {
		t.Errorf("household spent = %v, want 3000 USD", hhSpent)
	}
}

func TestSpentScopeAggregationMixedMembers(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{
		expense(10000, "USD", "food", "m1", "2026-06-03"),
		expense(5000, "USD", "food", "m2", "2026-06-04"),
		expense(3000, "USD", "rent", "m1", "2026-06-05"),
	}
	individual := domain.Budget{CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "m1", Limit: usd(50000)}
	group := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: usd(50000)}

	indivSpent, err := Spent(individual, txns, start, end, rates)
	if err != nil {
		t.Fatalf("individual Spent error: %v", err)
	}
	if !indivSpent.Equal(usd(10000)) {
		t.Errorf("individual spent = %v, want 10000 USD", indivSpent)
	}

	groupSpent, err := Spent(group, txns, start, end, rates)
	if err != nil {
		t.Fatalf("group Spent error: %v", err)
	}
	if !groupSpent.Equal(usd(15000)) {
		t.Errorf("group spent = %v, want 15000 USD", groupSpent)
	}
}

func TestSpentMultiCurrency(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10}}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(50000)}
	all := []domain.Transaction{
		expense(10000, "USD", "food", "", "2026-06-03"), // 100 USD
		expense(10000, "EUR", "food", "", "2026-06-04"), // 100 EUR -> 110 USD
	}
	spent, err := Spent(budget, all, start, end, rates)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !spent.Equal(usd(21000)) { // 210.00
		t.Errorf("spent = %v, want 21000 USD", spent)
	}
}

func TestEvaluateStates(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	mk := func(spentMinor int64) Status {
		budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(10000)}
		all := []domain.Transaction{expense(spentMinor, "USD", "food", "", "2026-06-03")}
		s, err := Evaluate(budget, all, start, end, rates, DefaultNearThreshold)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		return s
	}

	ok := mk(5000) // 50% of 100.00
	if ok.State != StateOK || ok.Percent != 50 || !ok.Remaining.Equal(usd(5000)) {
		t.Errorf("ok: state=%s pct=%d rem=%v", ok.State, ok.Percent, ok.Remaining)
	}
	near := mk(9000) // 90%
	if near.State != StateNear || near.Percent != 90 {
		t.Errorf("near: state=%s pct=%d", near.State, near.Percent)
	}
	over := mk(12000) // 120%
	if over.State != StateOver || over.Percent != 120 || !over.Remaining.Equal(usd(-2000)) {
		t.Errorf("over: state=%s pct=%d rem=%v", over.State, over.Percent, over.Remaining)
	}
}

func TestEvaluateZeroLimit(t *testing.T) {
	start, end := june()
	rates := currency.Rates{Base: "USD"}
	budget := domain.Budget{CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(0)}
	all := []domain.Transaction{expense(1000, "USD", "food", "", "2026-06-03")}
	s, err := Evaluate(budget, all, start, end, rates, DefaultNearThreshold)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if s.State != StateOver || s.Percent != 100 {
		t.Errorf("zero limit with spend: state=%s pct=%d, want over/100", s.State, s.Percent)
	}
}

func TestIsDuplicateBudget(t *testing.T) {
	existing := []domain.Budget{
		{ID: "b1", CategoryID: "food", Period: domain.PeriodMonthly, OwnerID: "grp"},
		{ID: "b2", CategoryID: "rent", Period: domain.PeriodMonthly, OwnerID: "grp"},
		{ID: "b3", CategoryID: "food", Period: domain.PeriodWeekly, OwnerID: "grp"},
	}

	tests := []struct {
		name      string
		catID     string
		period    string
		ownerID   string
		excludeID string
		want      bool
	}{
		{"exact match → duplicate", "food", "monthly", "grp", "", true},
		{"different category → ok", "transport", "monthly", "grp", "", false},
		{"different period → ok", "food", "quarterly", "grp", "", false},
		{"different owner → ok", "food", "monthly", "alice", "", false},
		{"exclude own ID → ok (edit self)", "food", "monthly", "grp", "b1", false},
		{"weekly food already exists", "food", "weekly", "grp", "", true},
		{"no existing budgets", "food", "monthly", "grp", "", true}, // still finds b1
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDuplicateBudget(existing, tt.catID, tt.period, tt.ownerID, tt.excludeID)
			if got != tt.want {
				t.Errorf("IsDuplicateBudget(%q,%q,%q,excl=%q) = %v, want %v",
					tt.catID, tt.period, tt.ownerID, tt.excludeID, got, tt.want)
			}
		})
	}

	// Empty slice → never a duplicate.
	if IsDuplicateBudget(nil, "food", "monthly", "grp", "") {
		t.Error("empty existing slice should never be a duplicate")
	}
}

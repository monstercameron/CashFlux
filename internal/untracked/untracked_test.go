// SPDX-License-Identifier: MIT

package untracked

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

const base = "USD"

func rates() currency.Rates { return currency.Rates{Base: base} }

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

func expense(catID string, minor int64, at time.Time) domain.Transaction {
	return domain.Transaction{
		ID: catID + at.Format("0102"), CategoryID: catID,
		Amount: money.New(-minor, base), Date: at,
	}
}

func income(catID string, minor int64, at time.Time) domain.Transaction {
	return domain.Transaction{
		ID: "inc" + catID, CategoryID: catID,
		Amount: money.New(minor, base), Date: at,
	}
}

func scanForTest(txns []domain.Transaction, categories []domain.Category, budgets []domain.Budget, from, to time.Time, rates currency.Rates, base string, hint Hint) []Candidate {
	got, err := Scan(txns, categories, budgets, from, to, rates, base, 12, hint)
	if err != nil {
		panic(err)
	}
	return got
}

func cats(pairs ...string) []domain.Category {
	var out []domain.Category
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, domain.Category{ID: pairs[i], Name: pairs[i+1]})
	}
	return out
}

// --- Scan -------------------------------------------------------------------

func TestScanFindsOnlyUntrackedExpenses(t *testing.T) {
	from, to := day(2026, 8, 1), day(2026, 9, 1)
	txns := []domain.Transaction{
		expense("auto", 62000, day(2026, 8, 15)),
		expense("auto", 48000, day(2026, 8, 17)),
		expense("hoa", 38000, day(2026, 8, 1)),
		expense("groceries", 24250, day(2026, 8, 9)), // tracked — must not appear
		income("salary", 470000, day(2026, 8, 1)),    // income is never a candidate
	}
	budgets := []domain.Budget{{ID: "b1", CategoryID: "groceries"}}

	got := scanForTest(txns, cats("auto", "Auto loans", "hoa", "HOA dues", "groceries", "Groceries"),
		budgets, from, to, rates(), base, nil)

	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2 (auto, hoa): %+v", len(got), got)
	}
	// Largest first.
	if got[0].CategoryID != "auto" || got[0].SpentMinor != 110000 {
		t.Errorf("first = %s/%d, want auto/110000 (620 + 480)", got[0].CategoryID, got[0].SpentMinor)
	}
	if got[1].CategoryID != "hoa" || got[1].SpentMinor != 38000 {
		t.Errorf("second = %s/%d, want hoa/38000", got[1].CategoryID, got[1].SpentMinor)
	}
	if got[0].Name != "Auto loans" {
		t.Errorf("name = %q, want %q", got[0].Name, "Auto loans")
	}
	// LastSeen is the LATEST hit, not the first one seen while scanning.
	if !got[0].LastSeen.Equal(day(2026, 8, 17)) {
		t.Errorf("auto LastSeen = %s, want 2026-08-17", got[0].LastSeen.Format("2006-01-02"))
	}
}

// A budget that watches several categories covers ALL of them. Testing only the
// primary CategoryID would let an "extra" category show up as untracked while its
// spending was already counted — double-counting the same money in the sheet.
func TestScanRespectsExtraTrackedCategories(t *testing.T) {
	from, to := day(2026, 8, 1), day(2026, 9, 1)
	txns := []domain.Transaction{
		expense("fuel", 9000, day(2026, 8, 3)),
		expense("parking", 1500, day(2026, 8, 4)),
	}
	// TrackedCategoryIDs is either/or: a non-empty CategoryIDs REPLACES CategoryID
	// rather than extending it, so a multi-category budget has to list every
	// category it watches — including the one that would otherwise be primary.
	multi := domain.Budget{ID: "b1", CategoryIDs: []string{"fuel", "parking"}}

	got := scanForTest(txns, cats("fuel", "Fuel", "parking", "Parking"), []domain.Budget{multi},
		from, to, rates(), base, nil)

	if len(got) != 0 {
		t.Fatalf("got %+v, want none — both categories are tracked by one budget", got)
	}
}

// The either/or rule above has a sharp edge worth pinning: a budget carrying BOTH
// a primary CategoryID and a CategoryIDs list tracks only the list. Spend on the
// primary is then genuinely untracked, and the sheet must say so rather than
// assuming the primary is always covered.
func TestScanPrimaryCategoryIsNotCoveredWhenCategoryIDsIsSet(t *testing.T) {
	from, to := day(2026, 8, 1), day(2026, 9, 1)
	txns := []domain.Transaction{expense("fuel", 9000, day(2026, 8, 3))}
	odd := domain.Budget{ID: "b1", CategoryID: "fuel", CategoryIDs: []string{"parking"}}

	got := scanForTest(txns, cats("fuel", "Fuel", "parking", "Parking"), []domain.Budget{odd},
		from, to, rates(), base, nil)

	if len(got) != 1 || got[0].CategoryID != "fuel" {
		t.Fatalf("got %+v, want fuel — CategoryIDs replaces CategoryID, it does not extend it", got)
	}
}

func TestScanWindowExcludesOutsideDates(t *testing.T) {
	from, to := day(2026, 8, 1), day(2026, 9, 1)
	txns := []domain.Transaction{
		expense("tax", 150000, day(2026, 7, 31)), // before
		expense("tax", 150000, day(2026, 9, 1)),  // on the exclusive end
		expense("hoa", 38000, day(2026, 8, 1)),   // on the inclusive start
	}
	got := scanForTest(txns, cats("tax", "Property tax", "hoa", "HOA dues"), nil, from, to, rates(), base, nil)

	if len(got) != 1 || got[0].CategoryID != "hoa" {
		t.Fatalf("got %+v, want only hoa — the range is half-open [from, to)", got)
	}
}

// The 12-month scan is the whole reason the sheet can see yearly obligations. A
// viewed-period scan finds nothing for them in eleven months out of twelve.
func TestScanOverAYearFindsNonMonthlyObligations(t *testing.T) {
	txns := []domain.Transaction{
		expense("ptax", 150000, day(2025, 11, 12)),
		expense("ins", 140000, day(2025, 10, 7)),
		expense("hoa", 38000, day(2026, 8, 1)),
	}
	all := cats("ptax", "Property tax", "ins", "Home insurance", "hoa", "HOA dues")

	month := scanForTest(txns, all, nil, day(2026, 8, 1), day(2026, 9, 1), rates(), base, nil)
	if len(month) != 1 {
		t.Fatalf("viewed-period scan got %d, want 1 — yearly items are invisible here", len(month))
	}
	year := scanForTest(txns, all, nil, day(2025, 9, 1), day(2026, 9, 1), rates(), base, nil)
	if len(year) != 3 {
		t.Fatalf("12-month scan got %d, want 3 — this is what makes yearly items visible", len(year))
	}
	if year[0].CategoryID != "ptax" {
		t.Errorf("largest = %s, want ptax", year[0].CategoryID)
	}
}

// Where a recurring schedule is known, its amount and cadence beat window spend:
// a yearly bill seen once in the window would otherwise seed a monthly budget with
// the whole annual figure.
func TestScanPrefersHintAmountAndPeriod(t *testing.T) {
	txns := []domain.Transaction{expense("ptax", 150000, day(2026, 8, 12))}
	hint := func(id string) (int64, domain.Period, bool) {
		if id == "ptax" {
			return 150000, domain.PeriodYearly, true
		}
		return 0, "", false
	}
	got := scanForTest(txns, cats("ptax", "Property tax"), nil,
		day(2026, 8, 1), day(2026, 9, 1), rates(), base, hint)

	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	if got[0].Period != domain.PeriodYearly {
		t.Errorf("period = %q, want yearly — a yearly bill in a monthly budget reads as a catastrophic overspend every twelfth month", got[0].Period)
	}
	if !got[0].FromHint {
		t.Error("FromHint = false; the sheet must be able to say the amount came from a schedule, not a guess")
	}
}

func TestScanFallsBackWhenHintDeclines(t *testing.T) {
	txns := []domain.Transaction{expense("misc", 4200, day(2026, 8, 12))}
	hint := func(string) (int64, domain.Period, bool) { return 0, "", false }
	got := scanForTest(txns, cats("misc", "Misc"), nil, day(2026, 8, 1), day(2026, 9, 1), rates(), base, hint)

	if len(got) != 1 || got[0].SuggestMinor != 350 {
		t.Fatalf("got %+v, want 350 monthly from 4200 over 12 months", got)
	}
	if got[0].FromHint {
		t.Error("FromHint = true without a hint")
	}
	if got[0].Period != domain.PeriodMonthly {
		t.Errorf("period = %q, want monthly by default", got[0].Period)
	}
}

func TestScanReportsOverflowInsteadOfWrappingSpent(t *testing.T) {
	_, err := Scan([]domain.Transaction{
		{ID: "first", Date: day(2026, 8, 2), CategoryID: "misc", Amount: money.New(-int64(^uint64(0)>>1), base)},
		{ID: "second", Date: day(2026, 8, 3), CategoryID: "misc", Amount: money.New(-1, base)},
	}, cats("misc", "Misc"), nil, day(2026, 8, 1), day(2026, 9, 1), rates(), base, 12, nil)
	if err == nil {
		t.Fatal("Scan returned no error for an overflowing category total")
	}
}

func TestScanRejectsInvalidFallbackPeriod(t *testing.T) {
	for _, months := range []int{0, -1} {
		if _, err := Scan(nil, nil, nil, day(2026, 8, 1), day(2026, 9, 1), rates(), base, months, nil); err == nil {
			t.Errorf("Scan fallback period %d returned no error", months)
		}
	}
}

func TestScanReturnsErrorForUnrepresentableExpenseMagnitude(t *testing.T) {
	_, err := Scan([]domain.Transaction{{
		ID: "min", Date: day(2026, 8, 2), CategoryID: "misc", Amount: money.New(-int64(^uint64(0)>>1)-1, base),
	}}, cats("misc", "Misc"), nil, day(2026, 8, 1), day(2026, 9, 1), rates(), base, 12, nil)
	if err == nil {
		t.Fatal("Scan returned no error for MinInt64 expense magnitude")
	}
}

// --- Impact -----------------------------------------------------------------

func impactForTest(t *testing.T, choices []Choice, pool, assigned int64) Effect {
	t.Helper()
	effect, err := Impact(choices, pool, assigned)
	if err != nil {
		t.Fatalf("Impact: %v", err)
	}
	return effect
}

// The zero-based consequence, stated before the write: tracking spending makes
// To Assign WORSE, because the money was always going out and is only now counted.
func TestImpactNewBudgetsDriveToAssignNegative(t *testing.T) {
	pool, assigned := int64(590000), int64(913020) // $5,900 income, $9,130.20 assigned
	choices := []Choice{
		{CategoryID: "auto", AmountMinor: 110000},
		{CategoryID: "hoa", AmountMinor: 38000},
		{CategoryID: "edu", AmountMinor: 32000},
	}
	e := impactForTest(t, choices, pool, assigned)

	if e.Categories != 3 || e.NewBudgets != 3 || e.Raises != 0 {
		t.Fatalf("counts = %+v, want 3 categories / 3 new / 0 raises", e)
	}
	if e.TrackedMinor != 180000 {
		t.Errorf("tracked = %d, want 180000", e.TrackedMinor)
	}
	if e.AssignedDeltaMinor != 180000 {
		t.Errorf("assigned delta = %d, want 180000", e.AssignedDeltaMinor)
	}
	if e.ToAssignBeforeMinor != -323020 {
		t.Errorf("before = %d, want -323020", e.ToAssignBeforeMinor)
	}
	if e.ToAssignAfterMinor != -503020 {
		t.Errorf("after = %d, want -503020 — the honest plan costs more", e.ToAssignAfterMinor)
	}
}

// Aiming a category at an existing budget without raising it adds NOTHING to the
// assigned total. That is the trap: the plan looks unchanged while the budget it
// landed on silently starts counting spend it has no limit for.
func TestImpactExistingWithoutRaiseDoesNotMovePlan(t *testing.T) {
	pool, assigned := int64(590000), int64(913020)
	e := impactForTest(t, []Choice{{CategoryID: "auto", AmountMinor: 110000, BudgetID: "b-transport"}}, pool, assigned)

	if e.AssignedDeltaMinor != 0 {
		t.Errorf("assigned delta = %d, want 0 — no raise means no new assignment", e.AssignedDeltaMinor)
	}
	if e.ToAssignAfterMinor != e.ToAssignBeforeMinor {
		t.Errorf("To Assign moved (%d → %d) with nothing assigned", e.ToAssignBeforeMinor, e.ToAssignAfterMinor)
	}
	if e.Raises != 0 || e.NewBudgets != 0 {
		t.Errorf("counts = %+v, want neither a new budget nor a raise", e)
	}
	if e.TrackedMinor != 110000 {
		t.Errorf("tracked = %d — the honesty gain is real even when the plan does not move", e.TrackedMinor)
	}
}

func TestImpactRaiseAssignsLikeANewBudget(t *testing.T) {
	pool, assigned := int64(590000), int64(913020)
	e := impactForTest(t, []Choice{{CategoryID: "auto", AmountMinor: 110000, BudgetID: "b-transport", Raise: true}}, pool, assigned)

	if e.Raises != 1 || e.NewBudgets != 0 {
		t.Fatalf("counts = %+v, want 1 raise", e)
	}
	if e.AssignedDeltaMinor != 110000 {
		t.Errorf("assigned delta = %d, want 110000", e.AssignedDeltaMinor)
	}
	if e.ToAssignAfterMinor != -433020 {
		t.Errorf("after = %d, want -433020", e.ToAssignAfterMinor)
	}
}

func TestImpactEmptyIsANoOp(t *testing.T) {
	e := impactForTest(t, nil, 590000, 913020)
	if e.Categories != 0 || e.AssignedDeltaMinor != 0 {
		t.Fatalf("empty selection changed something: %+v", e)
	}
	if e.ToAssignAfterMinor != e.ToAssignBeforeMinor {
		t.Errorf("To Assign moved on an empty selection")
	}
}

// --- OverspendRisk ----------------------------------------------------------

// The live case that motivated this: Transportation sits at $1,100 of $1,300, and
// Auto loans is $1,100 of untracked spend. Pointing one at the other without a
// raise turns a healthy budget into a $900 overspend in one click.
func TestOverspendRiskFlagsTheUnraisedDestination(t *testing.T) {
	spent := map[string]int64{"b-transport": 110000}
	limit := map[string]int64{"b-transport": 130000}
	spentOf := func(id string) int64 { return spent[id] }
	limitOf := func(id string) int64 { return limit[id] }

	risky, err := OverspendRisk([]Choice{{CategoryID: "auto", AmountMinor: 110000, BudgetID: "b-transport"}}, spentOf, limitOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(risky) != 1 || risky[0] != "b-transport" {
		t.Fatalf("risky = %v, want [b-transport]: 110000 + 110000 > 130000", risky)
	}

	// With the raise on, the limit grows with the spend and nothing is flagged.
	safe, err := OverspendRisk([]Choice{{CategoryID: "auto", AmountMinor: 110000, BudgetID: "b-transport", Raise: true}}, spentOf, limitOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(safe) != 0 {
		t.Errorf("risky = %v with the raise on, want none", safe)
	}
}

func TestOverspendRiskIgnoresNewBudgetsAndFittingAdds(t *testing.T) {
	spentOf := func(string) int64 { return 1000 }
	limitOf := func(string) int64 { return 100000 }
	got, err := OverspendRisk([]Choice{
		{CategoryID: "a", AmountMinor: 500},                    // new budget — created at its own amount
		{CategoryID: "b", AmountMinor: 500, BudgetID: "roomy"}, // fits under the limit
	}, spentOf, limitOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("risky = %v, want none", got)
	}
}

// Several categories aimed at ONE destination have to be judged together: each
// fits alone, the pair does not.
func TestOverspendRiskAccumulatesPerDestination(t *testing.T) {
	spentOf := func(string) int64 { return 0 }
	limitOf := func(string) int64 { return 10000 }
	got, err := OverspendRisk([]Choice{
		{CategoryID: "a", AmountMinor: 6000, BudgetID: "b1"},
		{CategoryID: "b", AmountMinor: 6000, BudgetID: "b1"},
	}, spentOf, limitOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "b1" {
		t.Fatalf("risky = %v, want [b1] — 6000 + 6000 > 10000 together", got)
	}
}

func TestTotalSpentSumsTheWindow(t *testing.T) {
	// The hero and the unbudgeted strip both need this number, and the whole point
	// of it living here is that they cannot disagree about it.
	got, err := TotalSpent([]Candidate{{SpentMinor: 310276}, {SpentMinor: 54348}, {SpentMinor: 4015}, {SpentMinor: 3723}})
	if err != nil {
		t.Fatal(err)
	}
	if got != 372362 {
		t.Errorf("TotalSpent = %d, want 372362", got)
	}
	if got, err := TotalSpent(nil); err != nil || got != 0 {
		t.Error("TotalSpent(nil) should be zero, not a panic or a stale sum")
	}
}

func TestUntrackedCalculationsReportOverflow(t *testing.T) {
	max := int64(^uint64(0) >> 1)
	min := -max - 1
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "impact tracked sum", run: func() error {
			_, err := Impact([]Choice{{AmountMinor: max}, {AmountMinor: 1}}, 0, 0)
			return err
		}},
		{name: "impact To Assign subtraction", run: func() error {
			_, err := Impact(nil, max, min)
			return err
		}},
		{name: "impact assigned total", run: func() error {
			_, err := Impact([]Choice{{AmountMinor: 1}}, 0, max)
			return err
		}},
		{name: "overspend accumulation", run: func() error {
			_, err := OverspendRisk([]Choice{{BudgetID: "b", AmountMinor: max}, {BudgetID: "b", AmountMinor: 1}}, func(string) int64 { return 0 }, func(string) int64 { return max })
			return err
		}},
		{name: "overspend projection", run: func() error {
			_, err := OverspendRisk([]Choice{{BudgetID: "b", AmountMinor: 1}}, func(string) int64 { return max }, func(string) int64 { return max })
			return err
		}},
		{name: "spent total", run: func() error {
			_, err := TotalSpent([]Candidate{{SpentMinor: max}, {SpentMinor: 1}})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err == nil {
				t.Fatal("calculation returned no overflow error")
			}
		})
	}
}

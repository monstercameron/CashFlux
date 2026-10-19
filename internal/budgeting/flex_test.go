// SPDX-License-Identifier: MIT

package budgeting

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func flexDate(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func flexCat(id, name string, class domain.CategoryClass) domain.Category {
	return domain.Category{ID: id, Name: name, Kind: domain.KindExpense, CategoryClass: class}
}

func flexTxn(catID string, minor int64, date time.Time) domain.Transaction {
	return domain.Transaction{ID: "t" + catID + date.Format("0102"), AccountID: "a1", CategoryID: catID, Amount: money.New(minor, "USD"), Date: date}
}

func evaluateFlexForTest(t *testing.T, cats []domain.Category, txns []domain.Transaction, recs []domain.Recurring, target int64, base string, start, end time.Time) FlexView {
	t.Helper()
	view, err := EvaluateFlex(cats, NewNettedLedger(txns), recs, target, currency.Rates{Base: base}, start, end)
	if err != nil {
		t.Fatalf("EvaluateFlex: %v", err)
	}
	return view
}

func TestDefaultCategoryClass(t *testing.T) {
	recs := []domain.Recurring{
		{ID: "r1", Label: "Rent", CategoryID: "rent", Amount: money.New(-150000, "USD"), Cadence: domain.CadenceMonthly},
		{ID: "r2", Label: "Insurance", CategoryID: "ins", Amount: money.New(-120000, "USD"), Cadence: domain.CadenceYearly, SmoothIntoBudgets: true},
	}
	tests := []struct {
		name string
		cat  domain.Category
		want domain.CategoryClass
	}{
		{"recurring monthly bill seeds fixed", domain.Category{ID: "rent", Kind: domain.KindExpense}, domain.ClassFixed},
		{"smoothed annual seeds non-monthly", domain.Category{ID: "ins", Kind: domain.KindExpense}, domain.ClassNonMonthly},
		{"unmapped seeds flex", domain.Category{ID: "dining", Kind: domain.KindExpense}, domain.ClassFlex},
		{"income always flex", domain.Category{ID: "rent", Kind: domain.KindIncome}, domain.ClassFlex},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultCategoryClass(tt.cat, recs); got != tt.want {
				t.Errorf("DefaultCategoryClass = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassOfDefaultsFlex(t *testing.T) {
	if got := (domain.Category{ID: "x"}).ClassOf(); got != domain.ClassFlex {
		t.Errorf("empty class = %q, want flex", got)
	}
	if got := (domain.Category{ID: "x", CategoryClass: "bogus"}).ClassOf(); got != domain.ClassFlex {
		t.Errorf("unknown class = %q, want flex", got)
	}
	if got := (domain.Category{ID: "x", CategoryClass: domain.ClassFixed}).ClassOf(); got != domain.ClassFixed {
		t.Errorf("stored class = %q, want fixed", got)
	}
}

func TestEvaluateFlex(t *testing.T) {
	start := flexDate(2026, 6, 1)
	end := flexDate(2026, 7, 1)
	cats := []domain.Category{
		flexCat("dining", "Dining", domain.ClassFlex),
		flexCat("fun", "Fun", domain.ClassFlex),
		flexCat("rent", "Rent", domain.ClassFixed),
		flexCat("gifts", "Gifts", domain.ClassNonMonthly),
		{ID: "salary", Name: "Salary", Kind: domain.KindIncome},
	}
	recs := []domain.Recurring{
		{ID: "r1", Label: "Rent", CategoryID: "rent", Amount: money.New(-150000, "USD"), Cadence: domain.CadenceMonthly, NextDue: flexDate(2026, 6, 1)},
		{ID: "r2", Label: "Gift fund", CategoryID: "gifts", Amount: money.New(-120000, "USD"), Cadence: domain.CadenceYearly, NextDue: flexDate(2026, 12, 1), SmoothIntoBudgets: true},
	}
	txns := []domain.Transaction{
		flexTxn("dining", -4000, flexDate(2026, 6, 5)),
		flexTxn("fun", -2500, flexDate(2026, 6, 10)),
		flexTxn("rent", -150000, flexDate(2026, 6, 2)),
		flexTxn("gifts", -3000, flexDate(2026, 6, 15)),
		flexTxn("dining", -1000, flexDate(2026, 5, 30)), // out of window
		{ID: "inc", AccountID: "a1", CategoryID: "salary", Amount: money.New(500000, "USD"), Date: flexDate(2026, 6, 1)},
	}

	view := evaluateFlexForTest(t, cats, txns, recs, 10000, "USD", start, end)

	if view.Spent.Amount != 6500 {
		t.Errorf("flex spent = %d, want 6500", view.Spent.Amount)
	}
	if view.Target.Amount != 10000 {
		t.Errorf("target = %d, want 10000", view.Target.Amount)
	}
	if view.Remaining.Amount != 3500 {
		t.Errorf("remaining = %d, want 3500", view.Remaining.Amount)
	}
	if view.Over {
		t.Error("should not be over")
	}
	if len(view.Fixed) != 1 {
		t.Fatalf("fixed rows = %d, want 1", len(view.Fixed))
	}
	fr := view.Fixed[0]
	if fr.CategoryID != "rent" || fr.Expected.Amount != 150000 || fr.Actual.Amount != 150000 || !fr.Paid {
		t.Errorf("rent row = %+v, want expected/actual 150000 paid", fr)
	}
	if len(view.NonMonthly) != 1 {
		t.Fatalf("non-monthly rows = %d, want 1", len(view.NonMonthly))
	}
	nm := view.NonMonthly[0]
	if nm.CategoryID != "gifts" || nm.Accrual.Amount != 10000 || nm.Spent.Amount != 3000 {
		t.Errorf("gifts row = %+v, want accrual 10000 spent 3000", nm)
	}
}

func TestEvaluateFlexOverspent(t *testing.T) {
	start := flexDate(2026, 6, 1)
	end := flexDate(2026, 7, 1)
	cats := []domain.Category{flexCat("dining", "Dining", domain.ClassFlex)}
	txns := []domain.Transaction{flexTxn("dining", -12000, flexDate(2026, 6, 5))}
	view := evaluateFlexForTest(t, cats, txns, nil, 10000, "USD", start, end)
	if !view.Over {
		t.Error("expected over")
	}
	if view.Remaining.Amount != -2000 {
		t.Errorf("remaining = %d, want -2000", view.Remaining.Amount)
	}
}

func TestEvaluateFlexSplits(t *testing.T) {
	start := flexDate(2026, 6, 1)
	end := flexDate(2026, 7, 1)
	cats := []domain.Category{
		flexCat("dining", "Dining", domain.ClassFlex),
		flexCat("rent", "Rent", domain.ClassFixed),
	}
	txns := []domain.Transaction{{
		ID: "sp", AccountID: "a1", Amount: money.New(-10000, "USD"), Date: flexDate(2026, 6, 5),
		Splits: []domain.CategorySplit{
			{CategoryID: "dining", Amount: money.New(-6000, "USD")},
			{CategoryID: "rent", Amount: money.New(-4000, "USD")},
		},
	}}
	view := evaluateFlexForTest(t, cats, txns, nil, 10000, "USD", start, end)
	if view.Spent.Amount != 6000 {
		t.Errorf("flex spent from split = %d, want 6000", view.Spent.Amount)
	}
	if len(view.Fixed) != 1 || view.Fixed[0].Actual.Amount != 4000 {
		t.Errorf("fixed actual from split = %+v, want 4000", view.Fixed)
	}
}

func TestEvaluateFlexUsesContributionRulesFXAndRefundNetting(t *testing.T) {
	start, end := flexDate(2026, 6, 1), flexDate(2026, 7, 1)
	cats := []domain.Category{
		flexCat("parent", "Flex parent", domain.ClassFlex),
		{ID: "child", Name: "Child", ParentID: "parent", Kind: domain.KindExpense, CategoryClass: domain.ClassFlex},
		flexCat("fixed", "Fixed", domain.ClassFlex),
	}
	txns := []domain.Transaction{
		{ID: "yen", Date: flexDate(2026, 6, 2), CategoryID: "child", Amount: money.New(-10000, "JPY")},
		{ID: "split", Date: flexDate(2026, 6, 3), Amount: money.New(-10000, "USD"), Splits: []domain.CategorySplit{
			{CategoryID: "parent", Amount: money.New(-12000, "USD")},
			{CategoryID: "fixed", Amount: money.New(2000, "USD")},
		}},
		{ID: "excluded", Date: flexDate(2026, 6, 4), CategoryID: "parent", Amount: money.New(-9000, "USD"), ExcludeFromReports: true},
		{ID: "purchase", Date: flexDate(2026, 6, 5), CategoryID: "parent", Payee: "Shop", Amount: money.New(-5000, "USD")},
		{ID: "refund", Date: flexDate(2026, 6, 6), CategoryID: "parent", Payee: "Shop", Amount: money.New(2000, "USD")},
	}
	links := []domain.TxnLink{{ID: "pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"purchase", "refund"}, Amount: money.New(2000, "USD")}}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.01}}
	view, err := EvaluateFlex(cats, NewNettedLedger(txns, links), nil, 50000, rates, start, end)
	if err != nil {
		t.Fatal(err)
	}
	// ¥10,000 -> $100, the signed split nets to $100 of flex spend, and the
	// refund pair nets $30. Excluded reimbursable spend and the fixed split line
	// stay out. Together: $230.
	if view.Spent.Amount != 23000 {
		t.Fatalf("flex spent = %d, want 23000 USD cents", view.Spent.Amount)
	}
}

func TestEvaluateFlexAttributesNestedCategoriesWithoutDoubleCounting(t *testing.T) {
	start, end := flexDate(2026, 6, 1), flexDate(2026, 7, 1)
	cats := []domain.Category{
		flexCat("housing", "Housing", domain.ClassFixed),
		{ID: "repairs", Name: "Repairs", ParentID: "housing", Kind: domain.KindExpense, CategoryClass: domain.ClassFlex},
		{ID: "unclassified", Name: "Unclassified", ParentID: "housing", Kind: domain.KindExpense},
	}
	recs := []domain.Recurring{{ID: "housing-bill", CategoryID: "housing", Amount: money.New(-2500, "USD"), Cadence: domain.CadenceMonthly, NextDue: start}}
	txns := []domain.Transaction{
		flexTxn("housing", -10000, flexDate(2026, 6, 2)),
		flexTxn("repairs", -5000, flexDate(2026, 6, 3)),
		flexTxn("unclassified", -2000, flexDate(2026, 6, 4)),
	}

	view := evaluateFlexForTest(t, cats, txns, recs, 20000, "USD", start, end)
	if view.Spent.Amount != 7000 {
		t.Fatalf("flex spend = %d, want flex child and default-flex child (7000)", view.Spent.Amount)
	}
	if len(view.Fixed) != 1 {
		t.Fatalf("fixed rows = %d, want one owner row", len(view.Fixed))
	}
	if got := view.Fixed[0]; got.CategoryID != "housing" || got.Actual.Amount != 10000 || got.Expected.Amount != 2500 {
		t.Fatalf("fixed row = %+v, want housing actual 10000 and expected 2500", got)
	}
}

func TestEvaluateFlexReturnsErrorsForMissingFXAndOverflow(t *testing.T) {
	start, end := flexDate(2026, 6, 1), flexDate(2026, 7, 1)
	tests := []struct {
		name  string
		txns  []domain.Transaction
		rates currency.Rates
	}{
		{name: "missing rate", txns: []domain.Transaction{{ID: "eur", Date: start, CategoryID: "flex", Amount: money.New(-100, "EUR")}}, rates: currency.Rates{Base: "USD"}},
		{name: "sum overflow", txns: []domain.Transaction{
			{ID: "large1", Date: start, CategoryID: "flex", Amount: money.New(-math.MaxInt64, "USD")},
			{ID: "large2", Date: start, CategoryID: "flex", Amount: money.New(-1, "USD")},
		}, rates: currency.Rates{Base: "USD"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EvaluateFlex([]domain.Category{flexCat("flex", "Flex", domain.ClassFlex)}, NewNettedLedger(tt.txns), nil, 100, tt.rates, start, end)
			if err == nil {
				t.Fatal("EvaluateFlex returned no error")
			}
		})
	}
}

func TestFixedPaidDoesNotOverflowNearMaxInt64(t *testing.T) {
	if !fixedPaid(math.MaxInt64, math.MaxInt64) {
		t.Fatal("a payment equal to its expected amount must be settled")
	}
	if !fixedPaid(math.MaxInt64, math.MaxInt64-amountTolerance(math.MaxInt64)) {
		t.Fatal("a payment within tolerance must be settled without overflowing")
	}
}

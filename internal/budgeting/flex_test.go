// SPDX-License-Identifier: MIT

package budgeting

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
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

func TestEvaluateFlexEmptyAndTargetLimits(t *testing.T) {
	for _, target := range []int64{0, -1, 1, math.MinInt64, math.MaxInt64} {
		t.Run(money.FormatMinor(target, 0), func(t *testing.T) {
			got := evaluateFlexForTest(t, nil, nil, nil, target, "USD", time.Time{}, time.Time{})
			want := FlexView{Target: money.New(target, "USD"), Spent: money.Zero("USD"), Remaining: money.New(target, "USD"), Over: target < 0}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("empty evaluation = %+v, want %+v", got, want)
			}
		})
	}
}

func TestEvaluateFlexWindowBoundaries(t *testing.T) {
	start, end := flexDate(2028, 2, 1), flexDate(2028, 3, 1)
	west := time.FixedZone("west", -5*60*60)
	tests := []struct {
		name       string
		start, end time.Time
		date       time.Time
		want       int64
	}{
		{"start included", start, end, start, 100},
		{"before start excluded", start, end, start.Add(-time.Nanosecond), 0},
		{"end excluded", start, end, end, 0},
		{"before end included", start, end, end.Add(-time.Nanosecond), 100},
		{"leap month end", start, end, flexDate(2028, 2, 29), 100},
		{"same start instant in another zone", start, end, start.In(west), 100},
		{"same end instant in another zone", start, end, end.In(west), 0},
		{"empty window", start, start, start, 0},
		{"reversed window", end, start, start, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cats := []domain.Category{flexCat("f", "Flex", domain.ClassFlex), flexCat("b", "Bill", domain.ClassFixed)}
			txns := []domain.Transaction{flexTxn("f", -100, tt.date), flexTxn("b", -100, tt.date)}
			got := evaluateFlexForTest(t, cats, txns, nil, 1000, "USD", tt.start, tt.end)
			if got.Spent.Amount != tt.want || got.Fixed[0].Actual.Amount != tt.want {
				t.Fatalf("window evaluation = %+v, want flex and fixed spending %d", got, tt.want)
			}
		})
	}
}

func TestEvaluateFlexRecurringAndRows(t *testing.T) {
	start, end := flexDate(2028, 2, 1), flexDate(2028, 3, 1)
	cats := []domain.Category{
		flexCat("z", "Z bill", domain.ClassFixed), flexCat("a", "A bill", domain.ClassFixed),
		flexCat("q", "Quarterly", domain.ClassNonMonthly), flexCat("y", "Annual", domain.ClassNonMonthly),
		flexCat("f", "Flex", domain.ClassFlex), {ID: "income", Kind: domain.KindIncome},
	}
	rec := func(id, cat string, amount int64, cadence domain.RecurringCadence, smooth bool) domain.Recurring {
		return domain.Recurring{ID: id, CategoryID: cat, Amount: money.New(amount, "USD"), Cadence: cadence, NextDue: flexDate(2028, 2, 29), SmoothIntoBudgets: smooth}
	}
	recs := []domain.Recurring{
		rec("monthly", "z", -100, domain.CadenceMonthly, false), rec("positive", "z", 200, domain.CadenceMonthly, false),
		rec("quarterly", "q", -300, domain.CadenceQuarterly, true), rec("yearly", "y", 1200, domain.CadenceYearly, true),
		rec("not smoothed", "q", -999, domain.CadenceMonthly, true), rec("zero", "z", 0, domain.CadenceMonthly, false),
		rec("flex", "f", -100, domain.CadenceMonthly, false), rec("income", "income", math.MinInt64, domain.CadenceMonthly, false),
		rec("unknown", "missing", math.MinInt64, domain.CadenceMonthly, false), rec("unassigned", "", math.MinInt64, domain.CadenceMonthly, false),
	}
	recs = append(recs, domain.Recurring{ID: "no due date", CategoryID: "a", Amount: money.New(-100, "USD")})
	got := evaluateFlexForTest(t, cats, []domain.Transaction{flexTxn("z", -300, start)}, recs, 1000, "USD", start, end)
	wantFixed := []FixedRow{
		{CategoryID: "a", CategoryName: "A bill", Expected: money.Zero("USD"), Actual: money.Zero("USD")},
		{CategoryID: "z", CategoryName: "Z bill", Expected: money.New(300, "USD"), Actual: money.New(300, "USD"), Paid: true},
	}
	wantNonMonthly := []NonMonthlyRow{
		{CategoryID: "y", CategoryName: "Annual", Accrual: money.New(100, "USD"), Spent: money.Zero("USD")},
		{CategoryID: "q", CategoryName: "Quarterly", Accrual: money.New(100, "USD"), Spent: money.Zero("USD")},
	}
	if !reflect.DeepEqual(got.Fixed, wantFixed) || !reflect.DeepEqual(got.NonMonthly, wantNonMonthly) {
		t.Fatalf("rows = %+v / %+v, want %+v / %+v", got.Fixed, got.NonMonthly, wantFixed, wantNonMonthly)
	}
}

func TestEvaluateFlexRecurringFX(t *testing.T) {
	start, end := flexDate(2026, 6, 1), flexDate(2026, 7, 1)
	for _, class := range []domain.CategoryClass{domain.ClassFixed, domain.ClassNonMonthly} {
		t.Run(string(class), func(t *testing.T) {
			cats := []domain.Category{flexCat("c", "Category", class)}
			recs := []domain.Recurring{{ID: "foreign", CategoryID: "c", Amount: money.New(-12000, "EUR"), Cadence: domain.CadenceYearly, NextDue: start, SmoothIntoBudgets: true}}
			rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.5}}
			got, err := EvaluateFlex(cats, NewNettedLedger([]domain.Transaction{flexTxn("c", -100, start)}), recs, 0, rates, start, end)
			if err != nil {
				t.Fatal(err)
			}
			if class == domain.ClassFixed && got.Fixed[0].Expected.Amount != 18000 {
				t.Fatalf("expected = %+v", got.Fixed)
			}
			if class == domain.ClassNonMonthly && got.NonMonthly[0].Accrual.Amount != 1500 {
				t.Fatalf("accrual = %+v", got.NonMonthly)
			}
		})
	}
}

func TestEvaluateFlexFailureContexts(t *testing.T) {
	start, end := flexDate(2026, 6, 1), flexDate(2026, 7, 1)
	badSplit := flexTxn("c", -1, start)
	badSplit.Splits = []domain.CategorySplit{{CategoryID: "c", Amount: money.New(-1, "USD")}, {CategoryID: "c", Amount: money.New(-1, "EUR")}}
	for _, class := range []domain.CategoryClass{domain.ClassFlex, domain.ClassFixed, domain.ClassNonMonthly} {
		t.Run(string(class), func(t *testing.T) {
			prefix := "evaluate flex transaction"
			convertPrefix, sumPrefix := "convert flex transaction", "sum flex spending"
			if class != domain.ClassFlex {
				prefix = "evaluate flex category"
				convertPrefix, sumPrefix = "convert flex category", "sum flex category"
			}
			tests := []struct {
				name  string
				txns  []domain.Transaction
				want  string
				cause error
			}{
				{"negation overflow", []domain.Transaction{flexTxn("c", math.MinInt64, start)}, prefix, money.ErrOverflow},
				{"mixed split currencies", []domain.Transaction{badSplit}, prefix, money.ErrCurrencyMismatch},
				{"missing FX", []domain.Transaction{{ID: "eur", Date: start, CategoryID: "c", Amount: money.New(-100, "EUR")}}, convertPrefix, currency.ErrUnknownRate},
				{"sum overflow", []domain.Transaction{flexTxn("c", -math.MaxInt64, start), flexTxn("c", -1, start)}, sumPrefix, money.ErrOverflow},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := EvaluateFlex([]domain.Category{flexCat("c", "Category", class)}, NewNettedLedger(tt.txns), nil, 0, currency.Rates{Base: "USD"}, start, end)
					assertFlexError(t, got, err, tt.want, tt.cause)
				})
			}
		})
	}
}

func assertFlexError(t *testing.T, got FlexView, err error, prefix string, cause error) {
	t.Helper()
	if err == nil || !strings.HasPrefix(err.Error(), prefix) || cause != nil && !errors.Is(err, cause) {
		t.Fatalf("error = %v, want prefix %q and cause %v", err, prefix, cause)
	}
	if !reflect.DeepEqual(got, FlexView{}) {
		t.Fatalf("failure returned partial view: %+v", got)
	}
}

func TestEvaluateFlexRecurringFailures(t *testing.T) {
	start, end := flexDate(2026, 6, 1), flexDate(2026, 7, 1)
	rec := func(amount int64, code string, cadence domain.RecurringCadence) domain.Recurring {
		return domain.Recurring{ID: "r", CategoryID: "c", Amount: money.New(amount, code), Cadence: cadence, NextDue: start, SmoothIntoBudgets: true}
	}
	large := rec(-math.MaxInt64, "USD", domain.CadenceQuarterly)
	tests := []struct {
		name  string
		class domain.CategoryClass
		recs  []domain.Recurring
		want  string
		cause error
	}{
		{"minimum amount", domain.ClassFixed, []domain.Recurring{rec(math.MinInt64, "USD", domain.CadenceMonthly)}, "recurring \"r\" amount:", money.ErrOverflow},
		{"missing rate", domain.ClassFlex, []domain.Recurring{rec(-1, "EUR", domain.CadenceMonthly)}, "convert recurring \"r\":", currency.ErrUnknownRate},
		{"zero still needs rate", domain.ClassNonMonthly, []domain.Recurring{rec(0, "EUR", domain.CadenceYearly)}, "convert recurring \"r\":", currency.ErrUnknownRate},
		{"occurrence multiplication", domain.ClassFixed, []domain.Recurring{rec(-math.MaxInt64, "USD", domain.CadenceDaily)}, "calculate recurring \"r\" occurrences:", money.ErrOverflow},
		{"expected sum", domain.ClassFixed, []domain.Recurring{rec(-math.MaxInt64, "USD", domain.CadenceMonthly), rec(-1, "USD", domain.CadenceMonthly)}, "sum expected recurring for \"c\":", money.ErrOverflow},
		{"accrual sum", domain.ClassNonMonthly, []domain.Recurring{large, large, large, large}, "sum recurring accrual for \"c\":", money.ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvaluateFlex([]domain.Category{flexCat("c", "Category", tt.class)}, nil, tt.recs, 0, currency.Rates{Base: "USD"}, start, end)
			assertFlexError(t, got, err, tt.want, tt.cause)
		})
	}
}

func TestEvaluateFlexBaseAndRemainingFailures(t *testing.T) {
	start, end := flexDate(2026, 6, 1), flexDate(2026, 7, 1)
	got, err := EvaluateFlex(nil, nil, nil, 0, currency.Rates{}, start, end)
	assertFlexError(t, got, err, "evaluate flex: base currency is empty", nil)
	got, err = EvaluateFlex([]domain.Category{flexCat("c", "Category", domain.ClassFlex)}, NewNettedLedger([]domain.Transaction{flexTxn("c", -1, start)}), nil, math.MinInt64, currency.Rates{Base: "USD"}, start, end)
	assertFlexError(t, got, err, "calculate flex remaining:", money.ErrOverflow)
}

func TestEvaluateFlexSignedDiscountsAndReportExclusions(t *testing.T) {
	start, end := flexDate(2026, 6, 1), flexDate(2026, 7, 1)
	cats := []domain.Category{flexCat("f", "Flex", domain.ClassFlex), flexCat("b", "Bill", domain.ClassFixed)}
	split := flexTxn("", -100, start)
	split.Splits = []domain.CategorySplit{
		{CategoryID: "f", Amount: money.New(-150, "USD")},
		{CategoryID: "b", Amount: money.New(50, "USD")},
	}
	transfer := flexTxn("f", -999, start)
	transfer.TransferAccountID = "other"
	excluded := flexTxn("f", -999, start)
	excluded.ExcludeFromReports = true
	txns := []domain.Transaction{split, transfer, excluded, flexTxn("f", 999, start), flexTxn("missing", -999, start), flexTxn("f", 0, start)}
	got := evaluateFlexForTest(t, cats, txns, nil, 100, "USD", start, end)
	if got.Spent.Amount != 150 || got.Remaining.Amount != -50 || !got.Over || got.Fixed[0].Actual.Amount != -50 || got.Fixed[0].Paid {
		t.Fatalf("signed split evaluation = %+v", got)
	}
	// A discount selected into flex can make its contribution negative, even
	// while the overall transaction is an expense. Preserve checked subtraction
	// on this side of the integer range too.
	split.Splits[0].Amount.Amount = 50
	split.Splits[1].Amount.Amount = -150
	got, err := EvaluateFlex(cats, NewNettedLedger([]domain.Transaction{split}), nil, math.MaxInt64, currency.Rates{Base: "USD"}, start, end)
	assertFlexError(t, got, err, "calculate flex remaining:", money.ErrOverflow)
}

func TestFixedPaidBoundaries(t *testing.T) {
	tests := []struct {
		expected, actual int64
		want             bool
	}{
		{0, 0, false}, {0, 1, true}, {-1, 1, true}, {-1, -1, false},
		{100, 98, false}, {100, 99, true}, {100, 101, true}, {1, 0, true},
		{math.MaxInt64, math.MaxInt64, true}, {math.MaxInt64, math.MinInt64, false},
	}
	for _, tt := range tests {
		if got := fixedPaid(tt.expected, tt.actual); got != tt.want {
			t.Errorf("fixedPaid(%d, %d) = %v, want %v", tt.expected, tt.actual, got, tt.want)
		}
	}
}

func TestDefaultCategoryClassMixedSchedules(t *testing.T) {
	cat := flexCat("c", "Category", domain.ClassFlex)
	monthly := domain.Recurring{CategoryID: "c", Cadence: domain.CadenceMonthly}
	annual := domain.Recurring{CategoryID: "c", Cadence: domain.CadenceYearly, SmoothIntoBudgets: true}
	tests := []struct {
		recs []domain.Recurring
		want domain.CategoryClass
	}{
		{nil, domain.ClassFlex},
		{[]domain.Recurring{monthly, annual}, domain.ClassNonMonthly},
		{[]domain.Recurring{annual, monthly}, domain.ClassNonMonthly},
		{[]domain.Recurring{{CategoryID: "other"}}, domain.ClassFlex},
		{[]domain.Recurring{monthly}, domain.ClassFixed},
	}
	for _, tt := range tests {
		if got := DefaultCategoryClass(cat, tt.recs); got != tt.want {
			t.Errorf("DefaultCategoryClass(%+v) = %s, want %s", tt.recs, got, tt.want)
		}
	}
}

func FuzzEvaluateFlexReconcilesAndPreservesInputs(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5, 6}, int64(1000))
	f.Add([]byte{0, 255, 127}, int64(math.MinInt64))
	f.Add([]byte{}, int64(-1))
	f.Add([]byte("2"), int64(math.MinInt64+48))
	f.Fuzz(func(t *testing.T, data []byte, target int64) {
		if len(data) > 1024 {
			data = data[:1024]
		}
		start, end := flexDate(2026, 6, 1), flexDate(2026, 7, 1)
		cats := []domain.Category{flexCat("f", "Flex", domain.ClassFlex), flexCat("b", "Bill", domain.ClassFixed), flexCat("n", "Annual", domain.ClassNonMonthly)}
		recs := []domain.Recurring{
			{ID: "fixed", CategoryID: "b", Amount: money.New(-1000, "USD"), Cadence: domain.CadenceMonthly, NextDue: start},
			{ID: "annual", CategoryID: "n", Amount: money.New(-1200, "EUR"), Cadence: domain.CadenceYearly, NextDue: start, SmoothIntoBudgets: true},
		}
		rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.5}}
		var txns []domain.Transaction
		var totals [3]int64
		for i, b := range data {
			cat := i % len(cats)
			amount := int64(b)
			txn := flexTxn(cats[cat].ID, -amount, start)
			txn.ExcludeFromReports = b%7 == 0
			if !txn.ExcludeFromReports {
				totals[cat] += amount
			}
			txns = append(txns, txn)
		}
		before, err := json.Marshal([]any{cats, txns, recs, rates})
		if err != nil {
			t.Fatal(err)
		}
		got, err := EvaluateFlex(cats, NewNettedLedger(txns), recs, target, rates, start, end)
		remaining, arithmeticErr := money.Sub(target, totals[0])
		if arithmeticErr != nil {
			assertFlexError(t, got, err, "calculate flex remaining:", money.ErrOverflow)
		} else {
			if err != nil {
				t.Fatal(err)
			}
			if got.Spent.Amount != totals[0] || got.Remaining.Amount != remaining || got.Target.Amount != target || got.Over != (totals[0] > target) || got.Fixed[0].Actual.Amount != totals[1] || got.NonMonthly[0].Spent.Amount != totals[2] {
				t.Fatalf("view = %+v, totals = %v, target = %d", got, totals, target)
			}
			if got.Fixed[0].Expected.Amount != 1000 || got.NonMonthly[0].Accrual.Amount != 150 {
				t.Fatalf("recurring totals = %+v / %+v", got.Fixed, got.NonMonthly)
			}
		}
		after, marshalErr := json.Marshal([]any{cats, txns, recs, rates})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if string(before) != string(after) {
			t.Fatal("evaluation mutated its inputs")
		}
		for i, j := 0, len(txns)-1; i < j; i, j = i+1, j-1 {
			txns[i], txns[j] = txns[j], txns[i]
		}
		reversed, reverseErr := EvaluateFlex(cats, NewNettedLedger(txns), recs, target, rates, start, end)
		if !reflect.DeepEqual(got, reversed) || (err == nil) != (reverseErr == nil) {
			t.Fatal("ledger order changed evaluation")
		}
	})
}

func TestEvaluateFlexLargeLedger(t *testing.T) {
	start, end := flexDate(2026, 6, 1), flexDate(2026, 7, 1)
	txns := make([]domain.Transaction, 10000)
	for i := range txns {
		txns[i] = flexTxn("c", -1, start)
	}
	got := evaluateFlexForTest(t, []domain.Category{flexCat("c", "Flex", domain.ClassFlex)}, txns, nil, 10000, "USD", start, end)
	if got.Spent.Amount != 10000 || got.Remaining.Amount != 0 || got.Over {
		t.Fatalf("large ledger = %+v", got)
	}
}

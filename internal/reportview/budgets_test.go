// SPDX-License-Identifier: MIT

package reportview

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBudgetAdherenceConvertsNativeSpendBeforeComparing(t *testing.T) {
	b := domain.Budget{ID: "food", CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(10000, "EUR")}
	bounds := []time.Time{day(time.January, 1), day(time.February, 1), day(time.March, 1), day(time.April, 1), day(time.May, 1)}
	txns := budgeting.NettedLedger{
		{Date: day(time.February, 2), CategoryID: "food", Amount: money.New(-10000, "EUR")},
		{Date: day(time.March, 2), CategoryID: "food", Amount: money.New(-11000, "EUR")},
		{Date: day(time.April, 2), CategoryID: "food", Amount: money.New(-15000, "EUR")},
		{Date: day(time.May, 1), CategoryID: "food", Amount: money.New(-999999, "EUR")},
	}
	got, err := BuildBudgetAdherence([]domain.Budget{b}, txns, bounds, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Counted != 1 || got.Clean != 0 || len(got.Rows) != 1 {
		t.Fatalf("summary = %+v", got)
	}
	row := got.Rows[0]
	if row.Limit != 20000 || row.MonthsOver != 1 || len(row.Months) != 4 {
		t.Fatalf("row = %+v", row)
	}
	for i, state := range []AdherenceState{AdherenceQuiet, AdherenceWithin, AdherenceNear, AdherenceOver} {
		if row.Months[i].State != state {
			t.Fatalf("month %d state = %q, want %q", i, row.Months[i].State, state)
		}
	}
	if row.Months[3].Spent != 30000 {
		t.Fatalf("native amount labelled as base: %+v", row.Months[3])
	}
}

func TestBudgetAdherenceAnnualCapsUseSharedCadenceRules(t *testing.T) {
	for _, tc := range []struct {
		period domain.Period
		count  int64
	}{{domain.PeriodWeekly, 52}, {domain.PeriodBiweekly, 26}, {domain.PeriodSemimonthly, 24}, {domain.PeriodQuarterly, 4}, {domain.PeriodYearly, 1}} {
		t.Run(string(tc.period), func(t *testing.T) {
			b := domain.Budget{ID: "b", CategoryID: "food", Period: tc.period, Limit: money.New(10000, "EUR")}
			// Deliberately a short window: the report labels these as full-year caps.
			bounds := []time.Time{day(time.January, 1), day(time.February, 1)}
			cap := 20000 * tc.count
			txns := budgeting.NettedLedger{{Date: bounds[0], CategoryID: "food", Amount: money.New(-cap-1, "USD")}}
			got, err := BuildBudgetAdherence([]domain.Budget{b}, txns, bounds, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
			if err != nil {
				t.Fatal(err)
			}
			row := got.Rows[0]
			// Native EUR rounds the odd USD cent before display conversion.
			if row.AnnualLimit != cap || row.AnnualSpent != cap+2 || row.AnnualOver != 2 || row.AnnualPercent != 100 || row.AnnualState != AdherenceNear || got.Clean != 0 {
				t.Fatalf("annual = %+v / %+v", row, got)
			}
		})
	}
}

func TestBudgetAdherenceUsesNettedLedgerAndSplitOwners(t *testing.T) {
	bounds := []time.Time{day(time.March, 1), day(time.April, 1), day(time.May, 1)}
	budgets := []domain.Budget{
		{ID: "household", CategoryID: "food", Limit: money.New(5000, "USD")},
		{ID: "mine", CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "me", Limit: money.New(2000, "USD")},
	}
	txns := []domain.Transaction{
		{ID: "purchase", Date: day(time.March, 2), CategoryID: "food", Amount: money.New(-8000, "USD")},
		{ID: "refund", Date: day(time.April, 2), CategoryID: "food", Amount: money.New(4000, "USD")},
		{ID: "split", Date: day(time.March, 3), MemberID: "them", Amount: money.New(-5000, "USD"), Splits: []domain.CategorySplit{
			{CategoryID: "food", MemberID: "me", Amount: money.New(-1000, "USD")},
			{CategoryID: "other", MemberID: "them", Amount: money.New(-4000, "USD")},
		}},
		{Date: day(time.March, 3), CategoryID: "food", Amount: money.New(-999999, "USD"), TransferAccountID: "savings"},
		{Date: day(time.March, 3), CategoryID: "food", Amount: money.New(-999999, "USD"), ExcludeFromReports: true},
	}
	links := []domain.TxnLink{{ID: "pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"purchase", "refund"}, Amount: money.New(4000, "USD")}}
	got, err := BuildBudgetAdherence(budgets, budgeting.NewNettedLedger(txns, links), bounds, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Clean != 2 || got.Counted != 2 || got.Rows[0].Months[0].Spent != 5000 || got.Rows[0].Months[1].Spent != 0 || got.Rows[1].Months[0].Spent != 1000 {
		t.Fatalf("refund/split attribution = %+v", got)
	}
	if txns[0].Amount.Amount != -8000 || txns[2].Splits[0].Amount.Amount != -1000 {
		t.Fatal("report mutated the source ledger")
	}
}

func TestBudgetAdherenceFailuresNeverCountAsClean(t *testing.T) {
	bounds := []time.Time{day(time.June, 1), day(time.July, 1)}
	budgets := []domain.Budget{
		{ID: "valid", CategoryID: "safe", Limit: money.New(100, "USD")},
		{ID: "missing-limit-rate", Limit: money.New(100, "GBP")},
		{ID: "missing-spend-rate", CategoryID: "bad", Limit: money.New(100, "USD")},
		{ID: "zero", Limit: money.New(0, "USD")},
	}
	txns := budgeting.NettedLedger{{Date: bounds[0], CategoryID: "bad", Amount: money.New(-100, "EUR")}}
	got, err := BuildBudgetAdherence(budgets, txns, bounds, currency.Rates{Base: "USD"})
	if err == nil || got.Counted != 1 || got.Clean != 1 || len(got.Rows) != 1 || got.Rows[0].Budget.ID != "valid" {
		t.Fatalf("failed rows inflated summary: %+v, %v", got, err)
	}
	if got, err := BuildBudgetAdherence(budgets, txns, nil, currency.Rates{Base: "USD"}); err != nil || len(got.Rows) != 0 {
		t.Fatalf("no window = %+v, %v", got, err)
	}
}

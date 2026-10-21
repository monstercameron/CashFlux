// SPDX-License-Identifier: MIT

package notificationview

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Frozen pre-projection loop. Keep independent of the new helper so parity and
// paired benchmarks detect changes to eligibility, order or error isolation.
func frozenMonthlyBudgetWins(budgets []domain.Budget, all budgeting.NettedLedger, start, end time.Time, rates currency.Rates) int {
	kept := 0
	for _, budget := range budgets {
		if budget.Period != domain.PeriodMonthly {
			continue
		}
		spent, err := budgeting.Spent(budget, all, start, end, rates)
		if err == nil && spent.Amount > 0 && spent.Amount <= budget.Limit.Amount {
			kept++
		}
	}
	return kept
}

func TestBudgetWinMonthPreservesBoundsNettedTypeAndSourceOrder(t *testing.T) {
	start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	all := budgeting.NettedLedger{
		{ID: "end", Date: end, Amount: money.New(math.MinInt64, "USD")},
		{ID: "later", Date: start.AddDate(0, 0, 20), Tags: []string{"trip"}, Splits: []domain.CategorySplit{{CategoryID: "food", MemberID: "A", Amount: money.New(-10, "USD")}}},
		{ID: "start", Date: start},
		{ID: "before", Date: start.Add(-time.Nanosecond)},
		{ID: "earlier", Date: start.AddDate(0, 0, 3)},
		{ID: "last", Date: end.Add(-time.Nanosecond)},
	}
	before, _ := json.Marshal(all)
	got := budgetWinMonth(all, start, end)
	if reflect.TypeOf(got) != reflect.TypeOf(budgeting.NettedLedger{}) {
		t.Fatal("projection lost netted type")
	}
	want := budgeting.NettedLedger{all[1], all[2], all[4], all[5]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projection reordered or changed rows: %+v", got)
	}
	after, _ := json.Marshal(all)
	if !bytes.Equal(before, after) {
		t.Fatal("projection mutated nested source records")
	}
}

func TestMonthlyBudgetWinProjectionMatchesCanonicalSpent(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.FixedZone("west", -5*3600))
	start, end := budgeting.PeriodRange(domain.PeriodMonthly, now.AddDate(0, 0, -now.Day()), time.Monday)
	inside := start.AddDate(0, 0, 10)
	budget := func(id, cat string, limit int64) domain.Budget {
		return domain.Budget{ID: id, CategoryID: cat, Period: domain.PeriodMonthly, Limit: money.New(limit, "USD"), Scope: domain.ScopeShared}
	}
	txn := func(id, cat string, amount int64) domain.Transaction {
		return domain.Transaction{ID: id, Date: inside, CategoryID: cat, Amount: money.New(amount, "USD")}
	}
	split := txn("split", "elsewhere", -100)
	split.MemberID = "A"
	split.Splits = []domain.CategorySplit{{CategoryID: "food", MemberID: "A", Amount: money.New(-60, "USD")}, {CategoryID: "food", MemberID: "B", Amount: money.New(-40, "USD")}}
	budgetA, budgetB := budget("A", "food", 60), budget("B", "food", 40)
	budgetA.Scope, budgetA.OwnerID = domain.ScopeIndividual, "A"
	budgetB.Scope, budgetB.OwnerID = domain.ScopeIndividual, "B"
	saving := budget("saving", "saving", 100)
	saving.Direction = domain.DirectionSave
	transfer := txn("transfer", "saving", -100)
	transfer.TransferAccountID = "brokerage"
	transferIn := transfer
	transferIn.ID = "transfer-in"
	transferIn.Amount = money.New(100, "USD")
	tagBudget := budget("tag", "none", 100)
	tagBudget.TrackedTags = []string{" Trip ", "trip"}
	tagged := split
	tagged.Tags = []string{"TRIP"}
	duplicateCategories := budget("duplicate", "food", 100)
	duplicateCategories.CategoryIDs = []string{"food", "food", "food"}
	unknown := txn("unknown", "bad", -50)
	unknown.Amount = money.New(-50, "GBP")
	overflow := txn("overflow", "bad", math.MinInt64)
	maxAmount := txn("maximum", "bad", -math.MaxInt64)
	maxAmount.Date = inside.AddDate(0, 0, 2)
	plusOne := txn("plus-one", "bad", -1)
	plusOne.Date = inside
	creditLine := txn("credit-line", "other", -1)
	creditLine.Date = inside.AddDate(0, 0, 1)
	creditLine.Splits = []domain.CategorySplit{{CategoryID: "bad", Amount: money.New(1, "USD")}}
	excluded := txn("excluded", "food", -999)
	excluded.ExcludeFromReports = true
	before := txn("before", "food", math.MinInt64)
	before.Date = start.Add(-time.Nanosecond)
	atStart := txn("start", "food", -20)
	atStart.Date = start
	atEnd := txn("end", "food", math.MinInt64)
	atEnd.Date = end
	last := txn("last", "food", -30)
	last.Date = end.Add(-time.Nanosecond)
	foreignOne := txn("tiny-a", "tiny", -1)
	foreignOne.Amount = money.New(-1, "EUR")
	foreignTwo := foreignOne
	foreignTwo.ID = "tiny-b"
	blankCurrency := budget("blank", "food", 100)
	blankCurrency.Limit.Currency = ""
	weekly := budget("weekly", "food", 100)
	weekly.Period = domain.PeriodWeekly
	tests := []struct {
		name    string
		budgets []domain.Budget
		ledger  budgeting.NettedLedger
		rates   currency.Rates
		want    int
	}{
		{"half-open boundaries", []domain.Budget{budget("b", "food", 50)}, budgeting.NettedLedger{before, last, atEnd, atStart}, currency.Rates{Base: "USD"}, 1},
		{"owned split lines", []domain.Budget{budgetA, budgetB, budget("shared", "food", 100)}, budgeting.NettedLedger{split}, currency.Rates{Base: "USD"}, 3},
		{"tag precedence and duplicate categories", []domain.Budget{tagBudget, duplicateCategories}, budgeting.NettedLedger{tagged}, currency.Rates{Base: "USD"}, 2},
		{"duplicate budget IDs still counted", []domain.Budget{budget("same", "food", 100), budget("same", "food", 100)}, budgeting.NettedLedger{txn("expense", "food", -50)}, currency.Rates{Base: "USD"}, 2},
		{"savings transfer direction", []domain.Budget{saving, budget("spend", "saving", 100)}, budgeting.NettedLedger{transfer, transferIn}, currency.Rates{Base: "USD"}, 1},
		{"report exclusions and income", []domain.Budget{budget("b", "food", 50)}, budgeting.NettedLedger{excluded, txn("income", "food", 500), txn("expense", "food", -50)}, currency.Rates{Base: "USD"}, 1},
		{"per-row FX rounding retained", []domain.Budget{budget("b", "tiny", 1)}, budgeting.NettedLedger{foreignOne, foreignTwo}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 0.4}}, 0},
		{"unknown FX isolated", []domain.Budget{budget("bad", "bad", 100), budget("good", "food", 100)}, budgeting.NettedLedger{unknown, txn("good", "food", -50)}, currency.Rates{Base: "USD"}, 1},
		{"negative sign overflow isolated", []domain.Budget{budget("bad", "bad", 100), budget("good", "food", 100)}, budgeting.NettedLedger{overflow, txn("good", "food", -50)}, currency.Rates{Base: "USD"}, 1},
		{"addition order retains checked overflow", []domain.Budget{budget("bad", "bad", math.MaxInt64), budget("good", "food", 100)}, budgeting.NettedLedger{maxAmount, plusOne, creditLine, txn("good", "food", -50)}, currency.Rates{Base: "USD"}, 1},
		{"base currency fallback and monthly eligibility", []domain.Budget{blankCurrency, weekly}, budgeting.NettedLedger{txn("good", "food", -50)}, currency.Rates{Base: "USD"}, 1},
		{"empty monthly set", []domain.Budget{weekly}, budgeting.NettedLedger{overflow}, currency.Rates{Base: "USD"}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before, _ := json.Marshal(struct {
				Budgets []domain.Budget
				Ledger  budgeting.NettedLedger
			}{tc.budgets, tc.ledger})
			ref := frozenMonthlyBudgetWins(tc.budgets, tc.ledger, start, end, tc.rates)
			got := keptMonthlyBudgetWins(tc.budgets, tc.ledger, start, end, tc.rates)
			if ref != tc.want || got != ref {
				t.Fatalf("new=%d reference=%d want=%d", got, ref, tc.want)
			}
			prepared := PrepareWins(WinsInput{Now: now, WeekStart: time.Monday, Budgets: tc.budgets, BudgetLedger: tc.ledger, Rates: tc.rates})
			if prepared.KeptBudgets != got || prepared.KeptPeriodKey != "2026-02" {
				t.Fatalf("PrepareWins did not use previous-month projection: %+v", prepared)
			}
			after, _ := json.Marshal(struct {
				Budgets []domain.Budget
				Ledger  budgeting.NettedLedger
			}{tc.budgets, tc.ledger})
			if !bytes.Equal(before, after) {
				t.Fatal("wins projection mutated its source")
			}
		})
	}
}

func TestMonthlyBudgetWinProjectionKeepsAlreadyNettedRefundAcrossMonth(t *testing.T) {
	start := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	raw := []domain.Transaction{{ID: "buy", Date: start.AddDate(0, 0, 5), CategoryID: "food", Amount: money.New(-10000, "USD")}, {ID: "refund", Date: end.AddDate(0, 0, 2), CategoryID: "food", Amount: money.New(4000, "USD")}}
	links := []domain.TxnLink{{ID: "refund-pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}, Amount: money.New(4000, "USD")}}
	all := budgeting.NewNettedLedger(raw, links)
	month := budgetWinMonth(all, start, end)
	if len(month) != 1 || month[0].Amount.Amount != -6000 {
		t.Fatalf("net refund changed during projection: %+v", month)
	}
	idempotent := budgeting.NewNettedLedger(month, links)
	if &idempotent[0] != &month[0] || idempotent[0].Amount.Amount != -6000 {
		t.Fatal("projection lost netted identity")
	}
	budgets := []domain.Budget{{CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(6000, "USD")}}
	if got := keptMonthlyBudgetWins(budgets, all, start, end, currency.Rates{Base: "USD"}); got != 1 {
		t.Fatalf("refund-adjusted budget not kept: %d", got)
	}
	if raw[0].Amount.Amount != -10000 {
		t.Fatal("raw refund source changed")
	}
}

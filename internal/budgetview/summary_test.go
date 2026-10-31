// SPDX-License-Identifier: MIT

package budgetview

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func summarySpendingFixture() SummarySpendingInput {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	return SummarySpendingInput{Start: start, End: start.AddDate(0, 1, 0), Now: start.AddDate(0, 0, 10), Base: "USD", Rates: currency.Rates{Base: "USD"}, Statuses: []budgeting.Status{{Budget: domain.Budget{ID: "yearly", CategoryID: "food", Period: domain.PeriodYearly}}}}
}

func summarySpendingRow(id string, amount int64, at time.Time) domain.Transaction {
	return domain.Transaction{ID: id, CategoryID: "food", Amount: money.New(amount, "USD"), Date: at}
}

func TestSummarySpendingUsesSelectedWindowAndNettedLedger(t *testing.T) {
	in := summarySpendingFixture()
	in.Ledger = budgeting.NettedLedger{
		summarySpendingRow("outside-own-cadence", -10000, in.Start.AddDate(0, -1, 3)),
		summarySpendingRow("inclusive", -100, in.Start),
		summarySpendingRow("exclusive", -99999, in.End),
	}
	buy := summarySpendingRow("purchase", -1000, in.Start.AddDate(0, 0, 3))
	refund := summarySpendingRow("refund", 400, in.End.AddDate(0, 0, 3))
	in.Ledger = budgeting.NewNettedLedger(append([]domain.Transaction(in.Ledger), buy, refund), []domain.TxnLink{{ID: "refund-pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"purchase", "refund"}, Amount: money.New(400, "USD")}})
	in.Statuses = append(in.Statuses, in.Statuses[0]) // overlap never doubles spend
	before := append([]domain.Transaction(nil), in.Ledger...)
	for range 2 {
		got := PrepareSummarySpending(in)
		if got.Amount != money.New(700, "USD") || !got.Known || !got.Complete || len(got.Issues) != 0 {
			t.Fatalf("selected range/refund union = %+v, want 700", got)
		}
		if !reflect.DeepEqual(before, []domain.Transaction(in.Ledger)) {
			t.Fatal("model mutated the shared netted ledger")
		}
	}
}

func TestSummarySpendingConvertsSelectedSplitUnionOnce(t *testing.T) {
	in := summarySpendingFixture()
	in.Rates.Rates = map[string]float64{"EUR": 1.5}
	in.Statuses = []budgeting.Status{{Budget: domain.Budget{CategoryIDs: []string{"a", "b"}}}}
	in.Ledger = budgeting.NettedLedger{{ID: "rounding", Date: in.Start, Amount: money.New(-2, "EUR"), Splits: []domain.CategorySplit{{CategoryID: "a", Amount: money.New(-1, "EUR")}, {CategoryID: "b", Amount: money.New(-1, "EUR")}}}}
	got := PrepareSummarySpending(in)
	if got.Amount.Amount != 3 || !got.Known || !got.Complete {
		t.Fatalf("each line was converted separately (would round to 4): %+v", got)
	}
}

func TestSummarySpendingPartialAndFatalAvailability(t *testing.T) {
	for _, tc := range []struct {
		name     string
		change   func(*SummarySpendingInput)
		want     int64
		known    bool
		complete bool
		err      error
	}{
		{"missing FX retains independent amount", func(in *SummarySpendingInput) {
			r := summarySpendingRow("foreign", -100, in.Start)
			r.Amount.Currency = "EUR"
			in.Ledger = append(in.Ledger, r)
		}, 25, true, false, currency.ErrUnknownRate},
		{"minimum contribution unavailable", func(in *SummarySpendingInput) {
			in.Ledger = append(in.Ledger, summarySpendingRow("invalid", math.MinInt64, in.Start))
		}, 25, true, false, money.ErrOverflow},
		{"aggregate overflow is not healthy zero", func(in *SummarySpendingInput) {
			in.Ledger = append(in.Ledger, summarySpendingRow("huge", -math.MaxInt64, in.Start))
		}, 0, false, false, money.ErrOverflow},
		{"failed budget is explicitly incomplete", func(in *SummarySpendingInput) {
			in.Statuses = append(in.Statuses, budgeting.Status{Budget: domain.Budget{ID: "invalid"}, Error: &budgeting.BudgetEvaluationError{BudgetID: "invalid", Err: money.ErrOverflow}})
		}, 25, true, false, money.ErrOverflow},
		{"saving budget excluded", func(in *SummarySpendingInput) { in.Statuses[0].Budget.Direction = domain.DirectionSave }, 0, true, true, nil},
		{"empty scopes", func(in *SummarySpendingInput) { in.Statuses = nil }, 0, true, true, nil},
		{"future includes no posted spend", func(in *SummarySpendingInput) { in.Now = in.Start.Add(-time.Nanosecond) }, 0, true, true, nil},
		{"invalid interval", func(in *SummarySpendingInput) { in.End = in.Start }, 0, false, false, nil},
		{"unknown reference time", func(in *SummarySpendingInput) { in.Now = time.Time{} }, 0, false, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := summarySpendingFixture()
			in.Ledger = budgeting.NettedLedger{summarySpendingRow("good", -25, in.Start)}
			tc.change(&in)
			got := PrepareSummarySpending(in)
			if got.Amount.Amount != tc.want || got.Known != tc.known || got.Complete != tc.complete {
				t.Fatalf("got %+v; want %d known=%v complete=%v", got, tc.want, tc.known, tc.complete)
			}
			if !tc.complete && len(got.Issues) == 0 {
				t.Fatal("unavailable model has no reason")
			}
			if tc.err != nil {
				found := false
				for _, issue := range got.Issues {
					found = found || errors.Is(issue.Err, tc.err)
				}
				if !found {
					t.Fatalf("lost error %v: %+v", tc.err, got.Issues)
				}
			}
		})
	}
}

func TestSummarySpendingUsesExactLocalWindowBoundaries(t *testing.T) {
	in := summarySpendingFixture()
	zone := time.FixedZone("west", -5*60*60)
	in.Start = time.Date(2026, 3, 1, 0, 0, 0, 0, zone)
	in.End, in.Now = in.Start.AddDate(0, 1, 0), in.Start.AddDate(0, 0, 3)
	in.Ledger = budgeting.NettedLedger{summarySpendingRow("before-local-midnight", -999, time.Date(2026, 3, 1, 4, 59, 59, 0, time.UTC)), summarySpendingRow("local-midnight", -10, in.Start.UTC())}
	if got := PrepareSummarySpending(in); got.Amount.Amount != 10 || !got.Complete {
		t.Fatal(got)
	}
}

func TestHeadlinePercentLargeAmountsNeverWrap(t *testing.T) {
	for _, tc := range []struct {
		spent, plan int64
		want        int
	}{{0, 1, 0}, {-1, 1, 0}, {1, 0, 0}, {1, 3, 33}, {math.MaxInt64 / 2, math.MaxInt64, 49}, {math.MaxInt64 - 1, math.MaxInt64, 99}, {math.MaxInt64, math.MaxInt64, 100}, {math.MaxInt64, 1, 100}} {
		if got := HeadlinePercent(tc.spent, tc.plan); got != tc.want {
			t.Fatalf("percent(%d,%d)=%d want%d", tc.spent, tc.plan, got, tc.want)
		}
	}
}

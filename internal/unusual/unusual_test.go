// SPDX-License-Identifier: MIT

package unusual

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var testNow = time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)

func txn(id, merchant, category string, minor int64, daysAgo int) domain.Transaction {
	return domain.Transaction{ID: id, Payee: merchant, CategoryID: category, Date: testNow.AddDate(0, 0, -daysAgo), Amount: money.New(-minor, "USD")}
}

func hasFinding(findings []Finding, id string, signal Signal) bool {
	for _, f := range findings {
		if f.TxnID == id && f.Signal == signal {
			return true
		}
	}
	return false
}

func TestRobustSpikeRules(t *testing.T) {
	tests := []struct {
		name   string
		input  Input
		want   bool
		source BaselineSource
	}{
		{name: "merchant baseline excludes candidate by ID", input: Input{Transactions: []domain.Transaction{txn("c", "Cafe", "food", 20000, 1), txn("h1", "Cafe", "food", 2000, 31), txn("h2", "Cafe", "food", 2000, 61), txn("h3", "Cafe", "food", 2000, 91), txn("h4", "Cafe", "food", 2000, 121)}, Now: testNow}, want: true, source: BaselineMerchant},
		{name: "merchant needs four other charges", input: Input{Transactions: []domain.Transaction{txn("c", "Cafe", "food", 20000, 1), txn("h1", "Cafe", "food", 2000, 31), txn("h2", "Cafe", "food", 2000, 61), txn("h3", "Cafe", "food", 2000, 91)}, Now: testNow}, want: false},
		{name: "category fallback needs eight other charges", input: categorySpikeInput(8), want: true, source: BaselineCategory},
		{name: "category fallback rejects seven", input: categorySpikeInput(7), want: false},
		{name: "MAD tolerates high variance", input: Input{Transactions: []domain.Transaction{txn("c", "Market", "food", 30000, 1), txn("h1", "Market", "food", 10000, 31), txn("h2", "Market", "food", 10000, 61), txn("h3", "Market", "food", 11000, 91), txn("h4", "Market", "food", 20000, 121)}, Now: testNow}, want: false},
		{name: "material amount floor", input: Input{Transactions: []domain.Transaction{txn("c", "Cafe", "food", 4000, 1), txn("h1", "Cafe", "food", 500, 31), txn("h2", "Cafe", "food", 500, 61), txn("h3", "Cafe", "food", 500, 91), txn("h4", "Cafe", "food", 500, 121)}, Now: testNow}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(tt.input)
			found := hasFinding(got, "c", Spike)
			if found != tt.want {
				t.Fatalf("Spike = %t, want %t; findings=%+v", found, tt.want, got)
			}
			if found && tt.source != "" {
				for _, f := range got {
					if f.TxnID == "c" && f.Signal == Spike && f.Evidence.BaselineSource != tt.source {
						t.Errorf("source=%s want %s", f.Evidence.BaselineSource, tt.source)
					}
				}
			}
		})
	}
}

func categorySpikeInput(n int) Input {
	rows := []domain.Transaction{txn("c", "Outlier", "food", 25000, 1)}
	for i := 0; i < n; i++ {
		rows = append(rows, txn("h"+string(rune('a'+i)), "Other "+string(rune('a'+i)), "food", 1000, 10+i*20))
	}
	return Input{Transactions: rows, Now: testNow}
}

func TestDoubleChargeBoundariesAndDeterministicOrder(t *testing.T) {
	for _, tt := range []struct {
		name string
		days int
		want bool
	}{{"same day", 0, false}, {"one day", 1, true}, {"three days", 3, true}, {"four days", 4, false}} {
		t.Run(tt.name, func(t *testing.T) {
			input := Input{Now: testNow, Transactions: []domain.Transaction{txn("a", "Coffee Shop", "food", 1200, tt.days), txn("b", "coffee shop", "food", 1200, 0)}}
			got := Detect(input)
			actual := hasFinding(got, "b", PossibleDoubleCharge)
			if actual != tt.want {
				t.Fatalf("double finding=%t want %t: %+v", actual, tt.want, got)
			}
		})
	}
	input := Input{Now: testNow, Transactions: []domain.Transaction{txn("z", "Cafe", "food", 1000, 0), txn("a", "Cafe", "food", 1000, 1)}}
	got := Detect(input)
	if len(got) != 2 || got[0].TxnID != "a" || got[1].TxnID != "z" {
		t.Fatalf("order=%+v", got)
	}
}

func TestExclusionsAndExpectationEvidence(t *testing.T) {
	base := []domain.Transaction{txn("candidate", "New Shop", "food", 40000, 1)}
	cases := []struct {
		name   string
		mutate func(*Input)
		want   Signal
	}{
		{"large first charge", func(*Input) {}, FirstTimeMerchantLarge},
		{"custom floor", func(in *Input) { in.Settings.FirstTimeMerchantFloorMinor = 50000 }, ""},
		{"goal linked", func(in *Input) { in.Transactions[0].GoalID = "g"; in.Goals = []domain.Goal{{ID: "g", Name: "Trip"}} }, ""},
		{"explicit expected", func(in *Input) { in.ExpectedTxnIDs = map[string][]string{"candidate": {"expected:planned"}} }, ""},
		{"transfer", func(in *Input) { in.Transactions[0].TransferAccountID = "other" }, ""},
		{"income refund", func(in *Input) { in.Transactions[0].Amount = money.New(40000, "USD") }, ""},
		{"split parent", func(in *Input) {
			in.Transactions[0].Splits = []domain.CategorySplit{{CategoryID: "food", Amount: money.New(-40000, "USD")}}
		}, ""},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			in := Input{Now: testNow, Transactions: append([]domain.Transaction(nil), base...)}
			tt.mutate(&in)
			findings := Detect(in)
			got := hasFinding(findings, "candidate", FirstTimeMerchantLarge)
			want := tt.want == FirstTimeMerchantLarge
			if got != want {
				t.Fatalf("first time finding=%t want %t: %+v", got, want, findings)
			}
		})
	}
}

func TestRecurringIncreaseNeedsTwoConsecutiveCycles(t *testing.T) {
	makeInput := func(lastPrevious int64) Input {
		return Input{Now: testNow, Recurring: []domain.Recurring{{ID: "stream", Label: "Video", Amount: money.New(-1599, "USD"), Cadence: domain.CadenceMonthly}}, Transactions: []domain.Transaction{
			txn("candidate", "Video", "bills", 1999, 1), txn("prev", "Video", "bills", lastPrevious, 31), txn("old1", "Video", "bills", 1599, 61), txn("old2", "Video", "bills", 1599, 92), txn("old3", "Video", "bills", 1599, 123),
		}}
	}
	if got := Detect(makeInput(1599)); hasFinding(got, "candidate", PriceIncrease) {
		t.Fatalf("single increase flagged: %+v", got)
	}
	if got := Detect(makeInput(1999)); !hasFinding(got, "candidate", PriceIncrease) {
		t.Fatalf("two increases missed: %+v", got)
	}
}

func TestSeasonalSuppressionNeedsTwoPriorCycles(t *testing.T) {
	candidate := txn("c", "Holiday Shop", "gifts", 10000, 1)
	for _, tt := range []struct {
		name  string
		years []int
		want  bool
	}{{"one prior year", []int{2025}, false}, {"two prior years", []int{2024, 2025}, true}} {
		t.Run(tt.name, func(t *testing.T) {
			rows := []domain.Transaction{candidate}
			for i, y := range tt.years {
				rows = append(rows, domain.Transaction{ID: string(rune('a' + i)), Payee: "Holiday Shop", CategoryID: "gifts", Date: time.Date(y, candidate.Date.Month(), 2, 0, 0, 0, 0, time.UTC), Amount: money.New(-1000, "USD")})
			}
			in := Input{Now: testNow, Transactions: rows}
			seasonal := map[string]map[time.Month]map[int]bool{"holiday shop": {candidate.Date.Month(): {candidate.Date.Year(): true}}}
			for _, year := range tt.years {
				seasonal["holiday shop"][candidate.Date.Month()][year] = true
			}
			got := len(expectationEvidence(charge{txn: candidate, key: "holiday shop", date: candidate.Date}, in, nil, seasonal)) > 0
			if got != tt.want {
				t.Fatalf("seasonal expectation evidence=%t want %t", got, tt.want)
			}
		})
	}
}

func TestPermutedInputHasStableFindingOutput(t *testing.T) {
	input := categorySpikeInput(8)
	a := Detect(input)
	for left, right := 0, len(input.Transactions)-1; left < right; left, right = left+1, right-1 {
		input.Transactions[left], input.Transactions[right] = input.Transactions[right], input.Transactions[left]
	}
	b := Detect(input)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("permutation changed findings: %+v != %+v", a, b)
	}
}

func TestFXFirstTimeFloorUsesCapturedBaseRates(t *testing.T) {
	in := Input{Now: testNow, Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"CAD": .75}}, Transactions: []domain.Transaction{
		{ID: "small", Payee: "First Store", Date: testNow.AddDate(0, 0, -1), Amount: money.New(-33000, "CAD")},
		{ID: "large", Payee: "Second Store", Date: testNow.AddDate(0, 0, -2), Amount: money.New(-40000, "CAD")},
	}}
	got := Detect(in)
	if hasFinding(got, "small", FirstTimeMerchantLarge) {
		t.Fatal("CAD 330.00 unexpectedly crossed the USD 250 floor")
	}
	if !hasFinding(got, "large", FirstTimeMerchantLarge) {
		t.Fatalf("CAD 400.00 should cross the USD 250 floor: %+v", got)
	}
	for _, finding := range got {
		if finding.TxnID == "large" && finding.Signal == FirstTimeMerchantLarge && finding.Evidence.CurrencyCode != "USD" {
			t.Fatalf("evidence currency=%q want USD", finding.Evidence.CurrencyCode)
		}
	}
}

func TestRecurringLinkIsExpectationEvidence(t *testing.T) {
	tx := txn("candidate", "Card descriptor 1234", "bills", 1599, 1)
	tx.SubscriptionName = "Video plan"
	in := Input{Now: testNow, Transactions: []domain.Transaction{tx}, Recurring: []domain.Recurring{{ID: "video", Label: "Video plan", Amount: money.New(-1599, "USD"), Cadence: domain.CadenceMonthly}}}
	if got := Detect(in); len(got) != 0 {
		t.Fatalf("matching recurring subscription should be suppressed: %+v", got)
	}
}

func TestCandidateDateBoundsAndFutureRows(t *testing.T) {
	candidate := txn("c", "Cafe", "food", 20000, 1)
	boundary := domain.Transaction{ID: "boundary", Payee: "Cafe", CategoryID: "food", Date: candidate.Date.AddDate(-2, 0, 0), Amount: money.New(-2000, "USD")}
	rows := []domain.Transaction{candidate, boundary, txn("h1", "Cafe", "food", 2000, 30), txn("h2", "Cafe", "food", 2000, 60), txn("h3", "Cafe", "food", 2000, 90)}
	in := Input{Now: testNow, Transactions: rows}
	if !hasFinding(Detect(in), "c", Spike) {
		t.Fatal("history exactly 24 months old should count")
	}
	in.Transactions[1].Date = in.Transactions[1].Date.Add(-24 * time.Hour)
	if hasFinding(Detect(in), "c", Spike) {
		t.Fatal("history older than 24 months should not count")
	}
	future := candidate
	future.ID = "future"
	future.Date = testNow.AddDate(0, 0, 1)
	in.Transactions = append(in.Transactions, future)
	if hasFinding(Detect(in), "future", FirstTimeMerchantLarge) {
		t.Fatal("future transaction unexpectedly produced a finding")
	}
}

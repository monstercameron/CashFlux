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

func TestEstablishedSeriesFlagsFirstIncreaseAndConfirmsNextCycle(t *testing.T) {
	makeInput := func(amounts ...int64) Input {
		rows := make([]domain.Transaction, 0, len(amounts))
		for i, amount := range amounts {
			rows = append(rows, txn("cycle"+string(rune('a'+i)), "Video", "bills", amount, (len(amounts)-i)*31))
		}
		return Input{Now: testNow, Transactions: rows}
	}
	first := Detect(makeInput(1599, 1599, 1599, 2099))
	var raised Finding
	for _, finding := range first {
		if finding.TxnID == "cycled" && finding.Signal == PriceIncrease {
			raised = finding
		}
	}
	if raised.TxnID == "" || raised.Confidence != ConfidenceMedium {
		t.Fatalf("first raised cycle = %+v; want medium PriceIncrease", first)
	}

	confirmed := Detect(makeInput(1599, 1599, 1599, 2099, 2099))
	var firstEvidence, confirming Finding
	for _, finding := range confirmed {
		if finding.Signal != PriceIncrease {
			continue
		}
		if finding.TxnID == "cycled" {
			firstEvidence = finding
		}
		if finding.TxnID == "cyclee" {
			confirming = finding
		}
	}
	if firstEvidence.Confidence != ConfidenceMedium || firstEvidence.Evidence.Confirmation != "confirmed by the next charge" {
		t.Fatalf("first cycle evidence = %+v; want medium with next-charge confirmation", firstEvidence)
	}
	if confirming.Confidence != ConfidenceHigh {
		t.Fatalf("confirming cycle = %+v; want high confidence", confirming)
	}

	// A one-off rise is reported once; the return to the prior amount is not a rise.
	oneOff := Detect(makeInput(1599, 1599, 1599, 2099, 1599))
	if hasFinding(oneOff, "cyclee", PriceIncrease) {
		t.Fatalf("return to baseline generated another price finding: %+v", oneOff)
	}
}

func TestPriceIncreaseThresholdAndEstablishedHistory(t *testing.T) {
	for _, tc := range []struct {
		name    string
		amounts []int64
		want    bool
	}{
		{name: "9 percent is below threshold", amounts: []int64{10000, 10000, 10000, 10900}},
		{name: "less than two dollars is below threshold", amounts: []int64{1000, 1000, 1000, 1199}},
		{name: "three prior stable cycles establish series", amounts: []int64{1599, 1599, 1599, 2099}, want: true},
		{name: "two prior charges do not establish series", amounts: []int64{1599, 1599, 2099}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := make([]domain.Transaction, 0, len(tc.amounts))
			for i, amount := range tc.amounts {
				rows = append(rows, txn("cycle"+string(rune('a'+i)), "Video", "bills", amount, (len(tc.amounts)-i)*31))
			}
			findings := Detect(Input{Now: testNow, Transactions: rows})
			got := hasFinding(findings, "cycle"+string(rune('a'+len(tc.amounts)-1)), PriceIncrease)
			if got != tc.want {
				t.Fatalf("PriceIncrease=%t want %t; findings=%+v", got, tc.want, findings)
			}
		})
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

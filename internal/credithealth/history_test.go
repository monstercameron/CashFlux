// SPDX-License-Identifier: MIT

package credithealth_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func historySnapshot(id string, amount int64, code string, day int) domain.BalanceSnapshot {
	return domain.BalanceSnapshot{ID: id, AccountID: "card", BalanceMinor: amount, Currency: code, AsOf: baseNow.AddDate(0, 0, day)}
}
func TestUtilizationHistoryCanonicalRoundingAndOverLimit(t *testing.T) {
	account := checkedCreditCard("card", "USD", 10000)
	snapshots := []domain.BalanceSnapshot{historySnapshot("a", -1783, "USD", 0), historySnapshot("b", 0, "USD", 1), historySnapshot("c", -15000, "USD", 2)}
	got := credithealth.BuildUtilizationHistory(account, snapshots, currency.Rates{Base: "USD"})
	if got.Err != nil || !got.Ready || got.Known != 3 || got.Total != 3 {
		t.Fatalf("history unavailable: %+v", got)
	}
	for i, want := range []int64{18, 0, 150} {
		if got.Points[i].UtilPct != want {
			t.Errorf("point%d=%d want%d", i, got.Points[i].UtilPct, want)
		}
	}
	if got.Points[2].FillPercent != 100 || got.Points[2].Band != credithealth.BandUtilWorst {
		t.Fatalf("over-limit reading lost: %+v", got.Points[2])
	}
	for _, snapshot := range snapshots {
		canonical := credithealth.EvaluateWithRates(credithealth.Inputs{Accounts: []domain.Account{account}, Balances: map[string]int64{"card": snapshot.BalanceMinor}, Now: baseNow}, currency.Rates{Base: "USD"})
		pct, err := credithealth.UtilizationChecked(money.New(snapshot.BalanceMinor, "USD"), account.CreditLimit, currency.Rates{Base: "USD"})
		if err != nil || pct != canonical.Cards[0].UtilPct {
			t.Fatalf("ratio drift %d %v", pct, err)
		}
	}
}
func TestUtilizationHistoryOrdersCopiesAndSelectsBeforeChecking(t *testing.T) {
	account := checkedCreditCard("card", "USD", 10000)
	var snapshots []domain.BalanceSnapshot
	for i := 9; i >= 0; i-- {
		snapshots = append(snapshots, historySnapshot(string(rune('a'+i)), -100, "USD", i))
	}
	snapshots[0].BalanceMinor = math.MinInt64
	unrelated := historySnapshot("other", math.MinInt64, "USD", 20)
	unrelated.AccountID = "other"
	snapshots = append(snapshots, unrelated)
	before := append([]domain.BalanceSnapshot(nil), snapshots...)
	got := credithealth.BuildUtilizationHistory(account, snapshots, currency.Rates{Base: "USD"})
	if !reflect.DeepEqual(before, snapshots) || got.Total != 10 || len(got.Points) != 8 || got.Points[0].ID != "c" || got.Points[7].ID != "j" || got.Known != 7 || !errors.Is(got.Err, money.ErrOverflow) {
		t.Fatalf("ordering/window/immutability: %+v", got)
	}
	equal := []domain.BalanceSnapshot{historySnapshot("z", -100, "USD", 0), historySnapshot("a", -100, "USD", 0)}
	got = credithealth.BuildUtilizationHistory(account, equal, currency.Rates{Base: "USD"})
	if got.Points[0].ID != "a" || got.Points[1].ID != "z" {
		t.Fatalf("equal date order: %+v", got)
	}
}
func TestUtilizationHistoryDifferentCurrenciesAndLegacyAssumption(t *testing.T) {
	account := checkedCreditCard("card", "USD", 10000)
	snapshots := []domain.BalanceSnapshot{historySnapshot("eur", -1000, "EUR", 0), historySnapshot("jpy", -1000, "JPY", 1), historySnapshot("legacy", -1000, "", 2)}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2, "JPY": 0.01}}
	got := credithealth.BuildUtilizationHistory(account, snapshots, rates)
	if got.Err != nil || !got.AssumedAccountCurrency || !got.Points[2].AssumedAccountCurrency {
		t.Fatalf("currencies/legacy: %+v", got)
	}
	for i, want := range []int64{20, 10, 10} {
		if got.Points[i].UtilPct != want {
			t.Errorf("point%d %d want%d", i, got.Points[i].UtilPct, want)
		}
	}
	account.Currency = "EUR"
	account.CreditLimit = money.New(10000, "USD")
	got = credithealth.BuildUtilizationHistory(account, []domain.BalanceSnapshot{historySnapshot("eur", -8000, "EUR", 0)}, rates)
	if got.Err != nil || got.Points[0].UtilPct != 160 {
		t.Fatalf("foreign limit mishandled: %+v", got)
	}
}
func TestUtilizationHistoryErrorsAreExplicitGaps(t *testing.T) {
	account := checkedCreditCard("card", "USD", 10000)
	for _, tc := range []struct {
		name, code string
		amount     int64
		rates      currency.Rates
		want       error
	}{
		{"minimum", "USD", math.MinInt64, currency.Rates{Base: "USD"}, money.ErrOverflow},
		{"unknown FX", "EUR", -1, currency.Rates{Base: "USD"}, currency.ErrUnknownRate},
		{"invalid FX", "EUR", -1, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": math.NaN()}}, currency.ErrInvalidRate},
		{"unsupported", "XYZ", -1, currency.Rates{Base: "USD", Rates: map[string]float64{"XYZ": 1}}, credithealth.ErrUnsupportedCurrency},
		{"FX overflow", "EUR", -math.MaxInt64, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, money.ErrOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := credithealth.BuildUtilizationHistory(account, []domain.BalanceSnapshot{historySnapshot("good", 0, "USD", 0), historySnapshot("bad", tc.amount, tc.code, 1)}, tc.rates)
			if got.Ready || got.Known != 1 || len(got.Points) != 2 || got.Points[1].Err == nil || !errors.Is(got.Err, tc.want) || got.Points[1].UtilPct != -1 {
				t.Fatalf("error became zero/history omission: %+v", got)
			}
		})
	}
}
func TestUtilizationHistoryLimitAndEmptyStates(t *testing.T) {
	account := checkedCreditCard("card", "USD", 10000)
	rates := currency.Rates{Base: "USD"}
	for _, n := range []int{0, 1, 2, 8, 9} {
		var snapshots []domain.BalanceSnapshot
		for i := range n {
			snapshots = append(snapshots, historySnapshot(string(rune('a'+i)), -100, "USD", i))
		}
		got := credithealth.BuildUtilizationHistory(account, snapshots, rates)
		if got.Err != nil || got.Ready != (n >= 2) || len(got.Points) != min(n, 8) || got.Total != n {
			t.Fatalf("count%d %+v", n, got)
		}
	}
	account.CreditLimit = money.Money{}
	got := credithealth.BuildUtilizationHistory(account, nil, rates)
	if !got.MissingLimit || got.Err != nil || got.Ready {
		t.Fatalf("no limit: %+v", got)
	}
	account.CreditLimit = money.New(-1, "USD")
	got = credithealth.BuildUtilizationHistory(account, nil, rates)
	if got.MissingLimit || got.Err == nil {
		t.Fatalf("negative limit became missing: %+v", got)
	}
	account.CreditLimit = money.New(1, "JPY")
	got = credithealth.BuildUtilizationHistory(account, nil, currency.Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.00001}})
	if got.Err == nil {
		t.Fatal("rounded-zero denominator accepted")
	}
}
func TestUtilizationHistoryCheckedWideRatios(t *testing.T) {
	got, err := credithealth.UtilizationChecked(money.New(-math.MaxInt64, "USD"), money.New(math.MaxInt64, "USD"), currency.Rates{Base: "USD"})
	if err != nil || got != 100 {
		t.Fatalf("safe wide ratio: %d %v", got, err)
	}
	account := checkedCreditCard("card", "USD", 1)
	result := credithealth.BuildUtilizationHistory(account, []domain.BalanceSnapshot{historySnapshot("wide", -100000000, "USD", 0)}, currency.Rates{Base: "USD"})
	if result.Err != nil || result.Points[0].UtilPct != 10000000000 || result.Points[0].FillPercent != 100 {
		t.Fatalf("wasm int narrowing: %+v", result)
	}
	_, err = credithealth.UtilizationChecked(money.New(-math.MaxInt64, "USD"), money.New(1, "USD"), currency.Rates{Base: "USD"})
	if !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("unrepresentable ratio accepted: %v", err)
	}
}

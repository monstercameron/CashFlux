// SPDX-License-Identifier: MIT

package audit

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
)

func TestAuditCheckedConvertsCurrencyAndSeparatesImpactPeriods(t *testing.T) {
	in := []smart.Insight{
		{Key: "context", Amount: money.New(999999, "USD"), HasAmount: true, Severity: smart.SeverityAlert},
		{Key: "once", Amount: money.New(-3000, "USD"), HasAmount: true, AmountCadence: smart.AmountOneTime},
		{Key: "annual", Amount: money.New(1100, "USD"), HasAmount: true, AmountCadence: smart.AmountAnnual},
		{Key: "monthly", Amount: money.New(100, "EUR"), HasAmount: true, AmountCadence: smart.AmountMonthly},
	}
	before := append([]smart.Insight(nil), in...)
	r, err := AuditChecked(in, "USD", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	if err != nil {
		t.Fatal(err)
	}
	if r.RecurringAnnualMinor != 3500 || r.OneTimeMinor != 3000 || r.UnclassifiedCount != 1 {
		t.Fatalf("period totals: %+v", r)
	}
	for i, want := range []string{"monthly", "annual", "once", "context"} {
		if r.Findings[i].Insight.Key != want {
			t.Fatalf("group ordering: %+v", r.Findings)
		}
	}
	if r.Findings[0].ImpactMinor != 200 || r.Findings[0].ComparisonMinor != 2400 || r.Findings[0].Insight.Amount.Currency != "EUR" {
		t.Fatalf("currency/evidence lost: %+v", r.Findings[0])
	}
	if !reflect.DeepEqual(before, in) {
		t.Fatal("input mutated")
	}
}

func TestRunCheckedCannotTreatUnavailableInputFXAsNoFindings(t *testing.T) {
	in := smartengine.Input{Now: time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC), Base: "USD", Rates: currency.Rates{Base: "USD"}, Accounts: []domain.Account{{ID: "foreign", OpeningBalance: money.New(1000, "EUR")}}}
	r, err := RunChecked(in, smart.Settings{})
	if !errors.Is(err, currency.ErrUnknownRate) || len(r.Findings) != 0 {
		t.Fatalf("missing input FX looked clean: %+v %v", r, err)
	}
	in.Accounts = nil
	in.Transactions = []domain.Transaction{{ID: "minimum", Amount: money.New(math.MinInt64, "USD")}}
	if _, err := RunChecked(in, smart.Settings{}); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("input minimum magnitude: %v", err)
	}
	in.Transactions = nil
	in.Now = time.Time{}
	if _, err := RunChecked(in, smart.Settings{}); err == nil {
		t.Fatal("missing explicit audit clock accepted")
	}
}

func TestAuditCheckedJPYPrecisionAndSameCurrencyWithoutFX(t *testing.T) {
	i := smart.Insight{Key: "yen"}.WithAnnualAmount(money.New(1200, "JPY"))
	r, err := AuditChecked([]smart.Insight{i}, "USD", currency.Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.01}})
	if err != nil || r.RecurringAnnualMinor != 1200 {
		t.Fatalf("JPY precision: %+v %v", r, err)
	}
	r, err = AuditChecked([]smart.Insight{i}, "JPY", currency.Rates{})
	if err != nil || r.RecurringAnnualMinor != 1200 {
		t.Fatalf("same currency needed unrelated rate: %+v %v", r, err)
	}
}

func TestAuditCheckedDoesNotReturnPartialTotalsOnFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		ins  []smart.Insight
		want error
	}{
		{"missing FX", []smart.Insight{{Key: "foreign", Amount: money.New(1, "EUR"), HasAmount: true}}, currency.ErrUnknownRate},
		{"magnitude", []smart.Insight{{Key: "min", Amount: money.New(math.MinInt64, "USD"), HasAmount: true}}, money.ErrOverflow},
		{"annualization", []smart.Insight{{Key: "monthly", Amount: money.New(math.MaxInt64, "USD"), HasAmount: true, AmountCadence: smart.AmountMonthly}}, money.ErrOverflow},
		{"recurring sum", []smart.Insight{{Key: "a", Amount: money.New(math.MaxInt64, "USD"), HasAmount: true, AmountCadence: smart.AmountAnnual}, {Key: "b", Amount: money.New(1, "USD"), HasAmount: true, AmountCadence: smart.AmountMonthly}}, money.ErrOverflow},
		{"one-time sum", []smart.Insight{{Key: "a", Amount: money.New(math.MaxInt64, "USD"), HasAmount: true, AmountCadence: smart.AmountOneTime}, {Key: "b", Amount: money.New(1, "USD"), HasAmount: true, AmountCadence: smart.AmountOneTime}}, money.ErrOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := AuditChecked(tc.ins, "USD", currency.Rates{Base: "USD"})
			if !errors.Is(err, tc.want) || len(r.Findings) != 0 || r.RecurringAnnualMinor != 0 || r.OneTimeMinor != 0 {
				t.Fatalf("partial/false total: %+v / %v", r, err)
			}
		})
	}
	legacy := Audit([]smart.Insight{{Key: "foreign", HasAmount: true, Amount: money.New(1, "EUR")}}, "USD")
	if legacy.Err == nil || len(legacy.Findings) != 0 {
		t.Fatalf("compatibility masked failure: %+v", legacy)
	}
}

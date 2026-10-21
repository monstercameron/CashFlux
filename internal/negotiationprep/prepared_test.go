// SPDX-License-Identifier: MIT

package negotiationprep

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func provider(cadence domain.RecurringCadence, amount int64) domain.Recurring {
	return domain.Recurring{ID: "provider", Label: "Streamly", Cadence: cadence, Amount: money.New(amount, "EUR")}
}
func payment(id string, date time.Time, amount int64, code string) domain.Transaction {
	return domain.Transaction{ID: id, Payee: "Streamly", Date: date, Amount: money.New(amount, code)}
}

func TestPrepareExactAnnualAndEligibleHistory(t *testing.T) {
	in := Input{Now: callNow, Recurring: []domain.Recurring{provider(domain.CadenceYearly, -12001)}}
	regular := payment("paid", callNow.AddDate(-2, 0, 0), -10000, "EUR")
	future := payment("future", callNow.Add(time.Nanosecond), -1, "EUR")
	excluded := regular
	excluded.ID = "excluded"
	excluded.ExcludeFromReports = true
	excluded.Amount = money.New(-1, "EUR")
	transfer := regular
	transfer.TransferAccountID = "other"
	transfer.Amount = money.New(-1, "EUR")
	in.Transactions = []domain.Transaction{regular, future, excluded, transfer, payment("refund", callNow, 100, "EUR"), payment("now", callNow, -12001, "EUR")}
	p, err := Prepare(in, Selection{Name: "stream", CompetitorNote: "A supplied comparison"})
	if err != nil {
		t.Fatal(err)
	}
	if p.ChargeCount != 2 || p.Currency != "EUR" || p.Subscription.AnnualMinor != 12001 || p.Subscription.MonthlyMinor != 1000 {
		t.Fatalf("prepared: %+v", p)
	}
	sheet, err := BuildChecked(p.Subscription, callNow, usd)
	if err != nil || sheet.AnnualMinor != 12001 || sheet.RiseMinor != 0 || !strings.Contains(sheet.TaskNotes(), "monthly equivalent") {
		t.Fatalf("sheet: %+v / %v", sheet, err)
	}
	if !strings.Contains(p.Basis(callNow), "Refunds are not netted") {
		t.Fatal("gross evidence basis missing")
	}
}

func TestPrepareNoScheduleDoesNotInventMonthlyCost(t *testing.T) {
	p, err := Prepare(Input{Now: callNow, Transactions: []domain.Transaction{payment("p", callNow.AddDate(-2, 0, 0), -25000, "JPY")}}, Selection{Name: "Streamly"})
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := BuildChecked(p.Subscription, callNow, usd)
	if err != nil || sheet.AnnualKnown || sheet.AnnualMinor != 0 || sheet.TenureMonths != 24 {
		t.Fatalf("history-only sheet: %+v / %v", sheet, err)
	}
	if !strings.Contains(sheet.TaskNotes(), "Monthly and annual costs are unknown") || strings.Contains(sheet.TaskNotes(), "a month, which is") {
		t.Fatal(sheet.TaskNotes())
	}
}

func TestPrepareCurrentActiveSelectionAndAmbiguity(t *testing.T) {
	paused := provider(domain.CadenceMonthly, -900)
	paused.ID = "paused"
	paused.Paused = true
	income := provider(domain.CadenceMonthly, 1000)
	income.ID = "income"
	active := provider(domain.CadenceMonthly, -1000)
	in := Input{Now: callNow, Recurring: []domain.Recurring{paused, income, active}}
	p, err := Prepare(in, Selection{Name: "Streamly"})
	if err != nil || p.RecurringID != "provider" {
		t.Fatalf("active selection: %+v %v", p, err)
	}
	second := active
	second.ID = "second"
	in.Recurring = append(in.Recurring, second)
	if _, err := Prepare(in, Selection{Name: "Streamly"}); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := Prepare(in, Selection{RecurringID: "second"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare(in, Selection{RecurringID: "paused"}); err == nil {
		t.Fatal("paused treated as active")
	}
	in.Recurring = nil
	a := payment("a", callNow, -100, "USD")
	b := a
	b.Payee = "Streamly Shop"
	in.Transactions = []domain.Transaction{a, b}
	if _, err := Prepare(in, Selection{Name: "Streamly"}); err == nil || !strings.Contains(err.Error(), "ambiguous historical") {
		t.Fatalf("merchant ambiguity: %v", err)
	}
}

func TestPrepareMixedCurrencyDoesNotClaimNominalRise(t *testing.T) {
	in := Input{Now: callNow, Recurring: []domain.Recurring{provider(domain.CadenceMonthly, -2000)}, Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, Transactions: []domain.Transaction{
		payment("one", callNow.AddDate(-1, 0, 0), -2000, "USD"), payment("two", callNow.AddDate(0, -2, 0), -3000, "USD"), payment("three", callNow, -4000, "USD"),
	}}
	p, err := Prepare(in, Selection{Name: "Streamly"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.ConvertedHistory || p.Subscription.Charges[0].AmountMinor != 1000 {
		t.Fatalf("FX preparation: %+v", p)
	}
	sheet, err := BuildChecked(p.Subscription, callNow, usd)
	if err != nil || sheet.RiseMinor != 0 || !strings.Contains(sheet.TaskNotes(), "nominal provider price rise") {
		t.Fatalf("FX sheet: %+v %v", sheet, err)
	}
	in.Rates.Rates = nil
	if _, err := Prepare(in, Selection{Name: "Streamly"}); !errors.Is(err, currency.ErrUnknownRate) {
		t.Fatalf("missing rate: %v", err)
	}
}

func TestBuildCheckedMoneyBoundariesAndCompatibility(t *testing.T) {
	for _, s := range []Subscription{
		{MonthlyMinor: math.MaxInt64},
		{Charges: []Charge{{At: callNow, AmountMinor: math.MinInt64}}},
		{MonthlyMinor: math.MaxInt64, AnnualKnown: true, AnnualMinor: 1, Charges: []Charge{{At: callNow, AmountMinor: 1}, {At: callNow, AmountMinor: 1}, {At: callNow, AmountMinor: 1}}},
	} {
		// The final case is deliberately scattered, so no price rise is claimed
		// only when historical evidence itself varies. Here history is identical.
		if _, err := BuildChecked(s, callNow, usd); !errors.Is(err, money.ErrOverflow) {
			t.Fatalf("expected checked overflow: %+v: %v", s, err)
		}
		legacy := Build(s, callNow, usd)
		if legacy.Err == nil || legacy.Worthwhile() || !strings.Contains(legacy.TaskNotes(), "unavailable") {
			t.Fatalf("compatibility masked failure: %+v", legacy)
		}
	}
	// Large positive spread must not wrap the guard's low*3 calculation.
	s := Subscription{MonthlyMinor: math.MaxInt64, AnnualKnown: true, AnnualMinor: math.MaxInt64, Charges: []Charge{{At: callNow, AmountMinor: math.MaxInt64 / 2}, {At: callNow, AmountMinor: math.MaxInt64 / 2}, {At: callNow, AmountMinor: math.MaxInt64}}}
	p, err := BuildChecked(s, callNow, usd)
	if err != nil || p.RisePercent != 100 {
		t.Fatalf("wide spread/percentage: %+v %v", p, err)
	}
}

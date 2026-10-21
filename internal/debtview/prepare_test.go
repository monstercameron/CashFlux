package debtview

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
)

func debtAccount(id, name, cur string, amount int64) domain.Account {
	return domain.Account{ID: id, Name: name, Currency: cur, Class: domain.ClassLiability, Type: domain.TypeCreditCard, OpeningBalance: money.New(amount, cur), InterestRateAPR: floatPtr(20)}
}
func floatPtr(v float64) *float64 { return &v }

func TestPrepareCheckedDebtFacts(t *testing.T) {
	t.Run("preserves mortgage exclusion and strategy rank", func(t *testing.T) {
		a := debtAccount("a", "A", "USD", -10000)
		a.MinPayment = money.New(500, "USD")
		b := debtAccount("b", "B", "USD", -20000)
		b.InterestRateAPR = floatPtr(5)
		b.MinPayment = money.New(500, "USD")
		m := debtAccount("m", "Mortgage", "USD", -90000)
		m.Type = domain.TypeMortgage
		r := Prepare([]domain.Account{a, b, m}, nil, currency.Rates{Base: "USD"}, "USD", Config{Strategy: payoff.Avalanche, ExtraMinor: 200}, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		if !r.TotalKnown || r.TotalOwed.Amount != 120000 {
			t.Fatalf("total=%+v known=%v", r.TotalOwed, r.TotalKnown)
		}
		if !r.PlanKnown || r.PlanOwed.Amount != 30000 || len(r.Debts) != 2 {
			t.Fatalf("plan facts: %+v", r)
		}
		if len(r.Accounts) != 3 || r.Accounts[0].Account.ID != "a" || r.Accounts[0].Rank != 1 {
			t.Fatalf("rank/order=%+v", r.Accounts)
		}
		if !r.PlanValid || r.DebtFree.IsZero() {
			t.Fatalf("plan/date missing: %+v", r)
		}
	})
	t.Run("does not turn unavailable balance or fx into zero", func(t *testing.T) {
		a := debtAccount("bad", "Bad", "EUR", -500)
		r := Prepare([]domain.Account{a}, nil, currency.Rates{Base: "USD"}, "USD", Config{}, time.Now())
		if r.TotalKnown || r.PlanKnown || r.Accounts[0].BaseKnown || r.TotalErr == nil {
			t.Fatalf("missing FX was hidden: %+v", r)
		}
		a.OpeningBalance = money.New(math.MinInt64, "USD")
		r = Prepare([]domain.Account{a}, nil, currency.Rates{Base: "USD"}, "USD", Config{}, time.Now())
		if r.TotalKnown || r.Accounts[0].BalanceErr == nil {
			t.Fatalf("abs overflow was hidden: %+v", r)
		}
	})
	t.Run("checked total overflow", func(t *testing.T) {
		a := debtAccount("a", "A", "USD", math.MaxInt64)
		b := debtAccount("b", "B", "USD", math.MaxInt64)
		r := Prepare([]domain.Account{a, b}, nil, currency.Rates{Base: "USD"}, "USD", Config{}, time.Now())
		if r.TotalKnown || !errors.Is(r.TotalErr, money.ErrOverflow) {
			t.Fatalf("overflow not surfaced: %+v", r)
		}
	})
}

func TestPreparePreservesInputsAndAttachesLedgerSourceError(t *testing.T) {
	a := debtAccount("card", "Card", "USD", -10000)
	accounts := []domain.Account{a}
	txns := []domain.Transaction{{ID: "wrong-currency", AccountID: "card", Amount: money.New(-25, "EUR")}}
	rateTable := currency.Rates{Base: "USD", Rates: map[string]float64{}}
	accountsBefore, txnsBefore, ratesBefore := append([]domain.Account(nil), accounts...), append([]domain.Transaction(nil), txns...), currency.Rates{Base: rateTable.Base, Rates: map[string]float64{}}
	for k, v := range rateTable.Rates {
		ratesBefore.Rates[k] = v
	}
	r := Prepare(accounts, txns, rateTable, "USD", Config{Strategy: payoff.Avalanche}, time.Time{})
	if len(r.Accounts) != 1 || r.Accounts[0].BalanceErr == nil || r.TotalKnown || r.PlanKnown {
		t.Fatalf("ledger source error was hidden: %+v", r)
	}
	if !reflect.DeepEqual(accounts, accountsBefore) || !reflect.DeepEqual(txns, txnsBefore) || !reflect.DeepEqual(rateTable, ratesBefore) {
		t.Fatal("preparation mutated caller-owned inputs")
	}
}

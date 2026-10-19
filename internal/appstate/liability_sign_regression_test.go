// SPDX-License-Identifier: MIT

package appstate

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestTransferLegsAcrossAccountConventions(t *testing.T) {
	a := newApp(t, false)
	accounts := []domain.Account{
		{ID: "cash", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "positive", Class: domain.ClassLiability, LiabilitySign: domain.LiabilityPositiveOwed, Currency: "USD"},
		{ID: "negative", Class: domain.ClassLiability, LiabilitySign: domain.LiabilityNegativeOwed, Currency: "USD"},
	}
	for _, from := range accounts {
		for _, to := range accounts {
			t.Run(from.ID+" to "+to.ID, func(t *testing.T) {
				out, in, err := a.transferLegs(from, to, 1000, 9999)
				if err != nil || out.Amount != -from.InflowSign()*1000 || in.Amount != to.InflowSign()*1000 {
					t.Fatalf("legs = %+v / %+v, err = %v", out, in, err)
				}
			})
		}
	}
}

func TestMirrorRoundedZeroFXLegUsesConventionAndDefaultBase(t *testing.T) {
	a := newApp(t, false)
	checking, card := seedLiability(t, a, 0)
	card.Currency, card.OpeningBalance.Currency = "EUR", "EUR"
	if err := a.PutAccount(card); err != nil {
		t.Fatal(err)
	}
	// Exercise a legacy settings record whose base was omitted, bypassing the
	// app's settings form defaults. Transfer creation and editing agree on USD.
	if err := a.store.PutSettings(store.Settings{FXRates: map[string]float64{"EUR": 2}}); err != nil {
		t.Fatal(err)
	}
	edited := domain.Transaction{AccountID: checking.ID, Amount: money.New(-10000, "USD")}
	pair := domain.Transaction{AccountID: card.ID, Amount: money.New(0, "EUR")}
	got, err := a.mirroredLegAmount(edited, pair, 0)
	if err != nil || got != money.New(-5000, "EUR") {
		t.Fatalf("rounded partner = %+v, err = %v", got, err)
	}
	_, in, err := a.transferLegs(checking, card, 10000, 0)
	if err != nil || in != got {
		t.Fatalf("creation = %+v, edit = %+v, err = %v", in, got, err)
	}
	edited.Amount.Amount = math.MinInt64
	if _, err := a.mirroredLegAmount(edited, pair, 0); err == nil {
		t.Fatal("mirror silently overflowed")
	}
}

func TestZeroOpeningLiabilityPaymentEditAndOverpayment(t *testing.T) {
	for _, convention := range []domain.LiabilitySign{"", domain.LiabilityNegativeOwed} {
		t.Run(string(convention), func(t *testing.T) {
			a := newApp(t, false)
			checking, card := seedLiability(t, a, 0)
			card.LiabilitySign = convention
			if err := a.PutAccount(card); err != nil {
				t.Fatal(err)
			}
			sign := card.InflowSign()
			if err := a.PutTransaction(domain.Transaction{ID: "charge", AccountID: card.ID, Amount: money.New(-sign*10000, "USD"), Date: time.Now(), Desc: "Card charge"}); err != nil {
				t.Fatal(err)
			}
			outID, inID, err := a.CreateTransferPair(TransferParams{FromAccountID: checking.ID, ToAccountID: card.ID, AmountMinor: 10000})
			if err != nil {
				t.Fatal(err)
			}
			if got := balanceOf(t, a, card); got != 0 {
				t.Fatalf("paid balance = %d", got)
			}
			prior, _ := legByID(a, outID)
			next := prior
			next.Amount.Amount = -12000
			if err := a.PutTransactionWithTransferPair(prior, next); err != nil {
				t.Fatal(err)
			}
			partner, _ := legByID(a, inID)
			if partner.Amount.Amount != sign*12000 {
				t.Fatalf("edited partner = %d, want %d", partner.Amount.Amount, sign*12000)
			}
			if _, _, err := a.CreateTransferPair(TransferParams{FromAccountID: checking.ID, ToAccountID: card.ID, AmountMinor: 1000}); err != nil {
				t.Fatal(err)
			}
			if err := a.RecordBillPayment(card.ID, "Loan", money.New(1000, "USD")); err != nil {
				t.Fatal(err)
			}
			if got := balanceOf(t, a, card); got != sign*4000 {
				t.Fatalf("credit balance = %d, want %d", got, sign*4000)
			}
			// Editing from the liability side retains the asset partner's direction.
			prior, _ = legByID(a, inID)
			next = prior
			next.Amount.Amount = sign * 13000
			if err := a.PutTransactionWithTransferPair(prior, next); err != nil {
				t.Fatal(err)
			}
			partner, _ = legByID(a, outID)
			if partner.Amount.Amount != -13000 {
				t.Fatalf("asset partner = %d, want -13000", partner.Amount.Amount)
			}
		})
	}
}

func TestCashAdvanceIncreasesDebtAndBooksFee(t *testing.T) {
	for _, opening := range []int64{-50000, 0, 50000} {
		t.Run(money.New(opening, "USD").String(), func(t *testing.T) {
			a := newApp(t, false)
			checking, card := seedLiability(t, a, opening)
			if _, _, err := a.CreateTransferPair(TransferParams{FromAccountID: card.ID, ToAccountID: checking.ID, AmountMinor: 10000, FeeMinor: 500}); err != nil {
				t.Fatal(err)
			}
			want := opening - card.InflowSign()*10500
			if got := balanceOf(t, a, card); got != want {
				t.Fatalf("debt = %d, want %d", got, want)
			}
			if got := balanceOf(t, a, checking); got != 110000 {
				t.Fatalf("cash balance = %d, want 110000", got)
			}
		})
	}
}

func TestCrossCurrencyLiabilityPaymentEditsKeepPartnerSigns(t *testing.T) {
	for _, opening := range []int64{-10000, 10000} {
		t.Run(money.New(opening, "EUR").String(), func(t *testing.T) {
			a := newApp(t, false)
			checking, card := seedLiability(t, a, opening)
			card.Currency = "EUR"
			card.OpeningBalance.Currency = "EUR"
			if err := a.PutAccount(card); err != nil {
				t.Fatal(err)
			}
			settings := a.Settings()
			settings.BaseCurrency, settings.FXRates = "USD", map[string]float64{"EUR": 2}
			if err := a.PutSettings(settings); err != nil {
				t.Fatal(err)
			}
			outID, inID, err := a.CreateTransferPair(TransferParams{FromAccountID: checking.ID, ToAccountID: card.ID, AmountMinor: 20000})
			if err != nil {
				t.Fatal(err)
			}
			if got := balanceOf(t, a, card); got != 0 {
				t.Fatalf("paid balance = %d", got)
			}
			prior, _ := legByID(a, outID)
			next := prior
			next.Amount.Amount = -24000
			if err := a.PutTransactionWithTransferPair(prior, next); err != nil {
				t.Fatal(err)
			}
			partner, _ := legByID(a, inID)
			if partner.Amount != money.New(card.InflowSign()*12000, "EUR") {
				t.Fatalf("FX partner = %+v", partner.Amount)
			}
			prior = partner
			next = partner
			next.Amount.Amount = card.InflowSign() * 13000
			if err := a.PutTransactionWithTransferPairReceived(prior, next, 27000); err != nil {
				t.Fatal(err)
			}
			partner, _ = legByID(a, outID)
			if partner.Amount != money.New(-27000, "USD") {
				t.Fatalf("explicit partner = %+v", partner.Amount)
			}
		})
	}
}

func TestLegacyBothNegativeLiabilityPairEdit(t *testing.T) {
	a := newApp(t, false)
	checking, card := seedLiability(t, a, 10000)
	when := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	out := domain.Transaction{ID: "legacy-out", AccountID: checking.ID, TransferAccountID: card.ID, Amount: money.New(-10000, "USD"), Date: when, Desc: "Legacy payment"}
	in := domain.Transaction{ID: "legacy-in", AccountID: card.ID, TransferAccountID: checking.ID, Amount: money.New(-10000, "USD"), Date: when, Desc: "Legacy payment"}
	for _, txn := range []domain.Transaction{out, in} {
		if err := a.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	next := out
	next.Amount.Amount = -12000
	if err := a.PutTransactionWithTransferPair(out, next); err != nil {
		t.Fatal(err)
	}
	partner, _ := legByID(a, in.ID)
	if partner.Amount.Amount != -12000 {
		t.Fatalf("legacy partner = %d, want -12000", partner.Amount.Amount)
	}
}

func TestRecurringCatchUpKeepsPaymentDirectionAcrossZero(t *testing.T) {
	for _, opening := range []int64{-10000, 10000} {
		t.Run(money.New(opening, "USD").String(), func(t *testing.T) {
			a := newApp(t, false)
			checking, card := seedLiability(t, a, opening)
			card.Name = "Mortgage"
			card.Type = domain.TypeMortgage
			card.MinPayment = money.New(10000, "USD")
			if err := a.PutAccount(card); err != nil {
				t.Fatal(err)
			}
			due := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			if err := a.PutRecurring(domain.Recurring{ID: "mortgage-pay", Label: "Mortgage payment", AccountID: checking.ID, Amount: money.New(-10000, "USD"), Cadence: domain.CadenceMonthly, NextDue: due, Autopost: true}); err != nil {
				t.Fatal(err)
			}
			posted, err := a.PostDueRecurring(due.AddDate(0, 2, 0))
			if err != nil {
				t.Fatal(err)
			}
			if posted != 3 {
				t.Fatalf("posted = %d, want 3", posted)
			}
			if got := balanceOf(t, a, card); got != opening+card.InflowSign()*30000 {
				t.Fatalf("catch-up balance = %d", got)
			}
			legs := 0
			for _, txn := range a.Transactions() {
				if txn.AccountID == card.ID {
					legs++
					if txn.Amount.Amount != card.InflowSign()*10000 {
						t.Fatalf("catch-up leg = %d", txn.Amount.Amount)
					}
				}
			}
			if legs != 3 {
				t.Fatalf("liability legs = %d, want 3", legs)
			}
		})
	}
}

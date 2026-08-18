// SPDX-License-Identifier: MIT

package transferpair

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func liability(id string, openingMinor int64) domain.Account {
	return domain.Account{ID: id, Name: id, Class: domain.ClassLiability,
		Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(openingMinor, "USD")}
}

func asset(id string, openingMinor int64) domain.Account {
	return domain.Account{ID: id, Name: id, Class: domain.ClassAsset,
		Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(openingMinor, "USD")}
}

// The case that made the old check fire on healthy data: a payment into a card
// that stores its debt POSITIVE-owed is negative on BOTH legs. Summing the stored
// amounts reads as a thousand dollars evaporating; summing the stated ones reads
// as zero, which is the truth.
func TestPositiveOwedCardPaymentBalancesWithTwoNegativeLegs(t *testing.T) {
	accs := []domain.Account{asset("chk", 100000), liability("card", 50000)}
	out := tx("o", "chk", "Card payment", -1000, day(5))
	in := tx("i", "card", "Card payment", -1000, day(5))

	net, balanced, known := NetStated(out, in, accs)
	if !known {
		t.Fatalf("known = false; both conventions are readable from their opening balances")
	}
	if !balanced {
		t.Errorf("net = %d, want the legs to cancel — this is a correct payment", net)
	}
}

// The same two negative legs against a NEGATIVE-owed card really are corruption:
// money left checking and the debt grew.
func TestNegativeOwedCardWithTwoNegativeLegsIsCorruption(t *testing.T) {
	accs := []domain.Account{asset("chk", 100000), liability("card", -50000)}
	out := tx("o", "chk", "Card payment", -1000, day(5))
	in := tx("i", "card", "Card payment", -1000, day(5))

	net, balanced, known := NetStated(out, in, accs)
	if !known {
		t.Fatalf("known = false")
	}
	if balanced {
		t.Errorf("the legs cancelled, but $10 left checking and the debt grew by $10")
	}
	if net != -2000 {
		t.Errorf("net = %d, want -2000", net)
	}
}

// A liability with no opening balance has no convention to read, only one to
// guess — and a health warning resting on a coin flip fires on correct data.
func TestUnknownConventionRefusesToJudge(t *testing.T) {
	accs := []domain.Account{asset("chk", 100000), liability("card", 0)}
	out := tx("o", "chk", "Card payment", -1000, day(5))
	in := tx("i", "card", "Card payment", -1000, day(5))

	if _, _, known := NetStated(out, in, accs); known {
		t.Errorf("known = true for a liability with nothing to infer from")
	}
}

func TestTwoAssetsNeedNoInference(t *testing.T) {
	accs := []domain.Account{asset("chk", 0), asset("sav", 0)}
	out := tx("o", "chk", "Sweep", -5000, day(5))
	in := tx("i", "sav", "Sweep", 5000, day(5))

	net, balanced, known := NetStated(out, in, accs)
	if !known || !balanced || net != 0 {
		t.Errorf("net=%d balanced=%v known=%v, want 0/true/true", net, balanced, known)
	}
}

func TestTwoAssetsBothNegativeIsAlwaysCorruption(t *testing.T) {
	accs := []domain.Account{asset("chk", 0), asset("sav", 0)}
	out := tx("o", "chk", "Sweep", -5000, day(5))
	in := tx("i", "sav", "Sweep", -5000, day(5))

	net, balanced, known := NetStated(out, in, accs)
	if !known || balanced {
		t.Errorf("balanced=%v known=%v, want a reported disagreement", balanced, known)
	}
	if net != -10000 {
		t.Errorf("net = %d, want -10000", net)
	}
}

// Summing across currencies would manufacture agreement out of an exchange rate.
func TestCrossCurrencyLegsAreNotJudged(t *testing.T) {
	accs := []domain.Account{asset("chk", 0), {ID: "eur", Class: domain.ClassAsset, Currency: "EUR"}}
	out := tx("o", "chk", "Fund EU", -10000, day(5))
	in := domain.Transaction{ID: "i", AccountID: "eur", Desc: "Fund EU",
		Amount: money.New(9200, "EUR"), Date: day(5)}

	if _, _, known := NetStated(out, in, accs); known {
		t.Errorf("known = true across currencies")
	}
}

// Declared honours an explicit mutual link regardless of amount — that is what
// keeps an edited-apart pair from being reported as a vanished leg.
func TestDeclaredIgnoresTheAmount(t *testing.T) {
	out := asTransfer(tx("o", "chk", "Transfer", -50000, day(5)), "sav")
	in := asTransfer(tx("i", "sav", "Transfer", 47000, day(5)), "chk")

	got, ok := Declared(out, []domain.Transaction{out, in})
	if !ok || got.ID != "i" {
		t.Errorf("Declared = %+v ok=%v, want the mutually-pointing leg", got, ok)
	}
}

func TestDeclaredNeedsAMutualPointer(t *testing.T) {
	out := asTransfer(tx("o", "chk", "Transfer", -50000, day(5)), "sav")
	// Points at a third account, so it is not this row's far side.
	other := asTransfer(tx("i", "sav", "Transfer", 50000, day(5)), "cns")

	if _, ok := Declared(out, []domain.Transaction{out, other}); ok {
		t.Errorf("Declared matched a row pointing somewhere else")
	}
	if _, ok := Declared(tx("plain", "chk", "Groceries", -1000, day(5)), nil); ok {
		t.Errorf("Declared matched for a row that is not a transfer at all")
	}
}

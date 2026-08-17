// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
)

var utilNow = time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)

// card builds a credit card whose balance a month ago was `thenMinor` and is
// `nowMinor` today, against a $1,000 limit.
func cardInput(thenMinor, nowMinor int64) Input {
	acct := domain.Account{
		ID: "card", Name: "Rewards Card", Class: domain.ClassLiability,
		Type: domain.TypeCreditCard, Currency: "USD",
		CreditLimit: money.New(100_000, "USD"),
	}
	txns := []domain.Transaction{
		{ID: "old", AccountID: "card", Date: utilNow.AddDate(0, 0, -60), Amount: money.New(-thenMinor, "USD")},
		{ID: "new", AccountID: "card", Date: utilNow.AddDate(0, 0, -2), Amount: money.New(-(nowMinor - thenMinor), "USD")},
	}
	return Input{
		Now: utilNow, Base: "USD", Rates: currency.Rates{Base: "USD"},
		Accounts: []domain.Account{acct}, Transactions: txns,
	}
}

func TestA14ReportsACrossingAndWhatWouldUndoIt(t *testing.T) {
	// 25% → 45%: Good into Fair.
	got := a14UtilCrossing(cardInput(25_000, 45_000))
	if len(got) != 1 {
		t.Fatalf("insights = %d, want 1: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Title, "moderate") {
		t.Errorf("title did not name the band in plain English: %q", got[0].Title)
	}
	// The figure is what it would take to get BACK, not the balance the card
	// already shows: 45% → 30% of a $1,000 limit is $150.
	if !strings.Contains(got[0].Detail, "$150") {
		t.Errorf("detail did not offer the way back: %q", got[0].Detail)
	}
	if got[0].AmountCadence != smart.AmountOneTime {
		t.Errorf("amount cadence = %v, want one-time", got[0].AmountCadence)
	}
}

// Repeating the current band would be telling somebody what they can already
// see, every time they look.
func TestA14StaysQuietWithinABand(t *testing.T) {
	if got := a14UtilCrossing(cardInput(12_000, 28_000)); len(got) != 0 {
		t.Errorf("movement inside one band was reported: %+v", got)
	}
}

// A watch that only ever reports bad news is one people learn to dread.
func TestA14SaysWhenACardComesBackDown(t *testing.T) {
	got := a14UtilCrossing(cardInput(60_000, 25_000)) // Poor → Good
	if len(got) != 1 {
		t.Fatalf("insights = %d, want 1", len(got))
	}
	if !strings.Contains(got[0].Title, "come back down") {
		t.Errorf("an improvement was not reported as one: %q", got[0].Title)
	}
	if got[0].Severity != smart.SeverityInfo {
		t.Errorf("severity = %v, want info for good news", got[0].Severity)
	}
	// And it offers no payment, because there is nothing to undo.
	if got[0].HasAmount {
		t.Errorf("an improvement carried a payment figure: %+v", got[0])
	}
}

func TestA14EscalatesOnlyAtTheWorstBand(t *testing.T) {
	warn := a14UtilCrossing(cardInput(60_000, 95_000)) // Poor → Worst
	if len(warn) != 1 || warn[0].Severity != smart.SeverityWarn {
		t.Errorf("crossing into the worst band was not a warning: %+v", warn)
	}
	nudge := a14UtilCrossing(cardInput(25_000, 45_000)) // Good → Fair
	if len(nudge) != 1 || nudge[0].Severity != smart.SeverityNudge {
		t.Errorf("an ordinary crossing was escalated: %+v", nudge)
	}
}

// A card with no limit recorded is not a card at 100%.
func TestA14NeedsALimit(t *testing.T) {
	in := cardInput(25_000, 45_000)
	in.Accounts[0].CreditLimit = money.Money{}
	if got := a14UtilCrossing(in); len(got) != 0 {
		t.Errorf("a card with no limit produced a crossing: %+v", got)
	}
}

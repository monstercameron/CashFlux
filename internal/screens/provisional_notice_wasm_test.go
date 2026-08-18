// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/provisional"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

// C693: a month still being written renders in the same type as a finished one,
// which invites the comparison that means least — a month-to-date total always
// looks like a collapse in spending, because it is three weeks short.

func provDate(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func provApp(t *testing.T, through time.Time, txns ...domain.Transaction) *appstate.App {
	t.Helper()
	a, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	acc := domain.Account{
		ID: "chk", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
	}
	if !through.IsZero() {
		acc.Reconciliations = []domain.Reconciliation{{
			At: through, StatementDate: through, StatementBalance: money.New(1000, "USD"),
		}}
	}
	if err := a.PutAccount(acc); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	for _, x := range txns {
		if err := a.PutTransaction(x); err != nil {
			t.Fatalf("PutTransaction: %v", err)
		}
	}
	return a
}

func TestAProvisionalPeriodSaysItIsNotFinal(t *testing.T) {
	app := provApp(t, provDate(2026, time.July, 31))
	f := render.New(t)
	f.Render(provisionalNotice(app, provDate(2026, time.August, 1), provDate(2026, time.September, 1)))

	got := f.Text()
	if !strings.Contains(got, "not final") {
		t.Fatalf("August was not captioned as provisional:\n%s", got)
	}
	if !strings.Contains(got, "July 31, 2026") {
		t.Errorf("the caption does not say how far the statements reach:\n%s", got)
	}
}

// A finished month resting on nothing gets no banner. One that appears on every
// report is one nobody reads.
func TestAClosedPeriodIsNotCaptionedAtAll(t *testing.T) {
	app := provApp(t, provDate(2026, time.July, 31))
	f := render.New(t)
	f.Render(provisionalNotice(app, provDate(2026, time.July, 1), provDate(2026, time.August, 1)))

	if got := strings.TrimSpace(f.Text()); got != "" {
		t.Errorf("a closed period was captioned:\n%s", got)
	}
}

// The case worth saying out loud: part of the period's balance is a guess.
func TestAPeriodRestingOnACheckpointSaysHowMuch(t *testing.T) {
	cp, ok := provisional.New("cp1", "chk", "Balance checkpoint", 0, 25000, "USD",
		provDate(2026, time.August, 15), provDate(2026, time.August, 15))
	if !ok {
		t.Fatal("fixture produced no checkpoint")
	}
	app := provApp(t, provDate(2026, time.July, 31), cp)
	f := render.New(t)
	f.Render(provisionalNotice(app, provDate(2026, time.August, 1), provDate(2026, time.September, 1)))

	got := f.Text()
	if !strings.Contains(got, "250.00") {
		t.Errorf("the caption does not name what rests on a guess:\n%s", got)
	}
	if !strings.Contains(got, "left out of the figures above") {
		t.Errorf("the caption does not say the checkpoint is excluded:\n%s", got)
	}
}

// With nothing reconciled at all, the caption must not name a date it does not
// have.
func TestNoReconciliationSaysSoWithoutInventingADate(t *testing.T) {
	app := provApp(t, time.Time{})
	f := render.New(t)
	f.Render(provisionalNotice(app, provDate(2026, time.August, 1), provDate(2026, time.September, 1)))

	got := f.Text()
	if !strings.Contains(got, "No account has been reconciled") {
		t.Errorf("caption = %q, want it to say nothing has been confirmed", got)
	}
	if strings.Contains(got, "reconciled through") {
		t.Errorf("the caption named a through-date it does not have:\n%s", got)
	}
}

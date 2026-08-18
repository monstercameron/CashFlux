// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// C685: an account card carries the balance and, beside it, the cleared-only
// figure. That second number is a reconciliation aid, and on an account nobody
// reconciles it reads as a second claim about what is owed.
//
// The reported case: a utility shell sits at $0 because every bill was posted and
// every bill paid, but one side of that had not been ticked cleared — so the card
// read "$0.00 · cleared ($120.00)" and looked like a debt of $120.

func clearedMeta(t *testing.T, f *render.Fixture, acc domain.Account, balance, cleared money.Money) string {
	t.Helper()
	// AccountRow owns hooks, so it must be MOUNTED as a component; calling the
	// function directly panics in the reconciler with no current fiber.
	f.Render(ui.CreateElement(AccountRow, accountRowProps{Account: acc, Balance: balance, Cleared: cleared}))
	return f.Text()
}

func utilityShell() domain.Account {
	return domain.Account{
		ID: "fpl", Name: "FPL", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassLiability, Type: domain.TypeUtilities, Currency: "USD",
	}
}

func TestUtilityShellDoesNotShowAClearedFigure(t *testing.T) {
	f := render.New(t)
	got := clearedMeta(t, f, utilityShell(), money.New(0, "USD"), money.New(12000, "USD"))
	if strings.Contains(strings.ToLower(got), "cleared") {
		t.Errorf("a utility shell showed a cleared figure:\n%s", got)
	}
	if strings.Contains(got, "120.00") {
		t.Errorf("the card still states $120.00 on an account that owes nothing:\n%s", got)
	}
}

// The suppression is about the account having no statement, not about the balance
// happening to be zero — a utility shell mid-cycle has the same problem.
func TestUtilityShellWithABalanceStillShowsNoClearedFigure(t *testing.T) {
	f := render.New(t)
	got := clearedMeta(t, f, utilityShell(), money.New(9000, "USD"), money.New(12000, "USD"))
	if strings.Contains(strings.ToLower(got), "cleared") {
		t.Errorf("a utility shell with a balance showed a cleared figure:\n%s", got)
	}
}

// An account that IS reconciled keeps the figure — it is how you check a
// statement, and removing it everywhere would be a worse bug than the one fixed.
func TestCheckingStillShowsItsClearedFigure(t *testing.T) {
	chk := domain.Account{
		ID: "chk", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
	}
	f := render.New(t)
	got := clearedMeta(t, f, chk, money.New(500000, "USD"), money.New(449395, "USD"))
	if !strings.Contains(strings.ToLower(got), "cleared") {
		t.Errorf("a checking account lost its cleared figure:\n%s", got)
	}
	if !strings.Contains(got, "4,493.95") {
		t.Errorf("the cleared figure is missing or misformatted:\n%s", got)
	}
}

// C683's display half: an overpaid card is money the lender owes YOU. Taking
// -Abs, which the card used to do, turned that credit into more debt.
func TestOverpaidCardReadsAsACreditNotMoreDebt(t *testing.T) {
	f := render.New(t)
	for _, acc := range []domain.Account{
		{ // stores debt positive; overpaid, so the balance went negative
			ID: "c1", Name: "Card", OwnerID: "m1", Scope: domain.ScopeIndividual,
			Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD",
			OpeningBalance: money.New(50000, "USD"),
		},
		{ // stores debt negative; overpaid, so the balance went positive
			ID: "c2", Name: "Card", OwnerID: "m1", Scope: domain.ScopeIndividual,
			Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD",
			OpeningBalance: money.New(-50000, "USD"),
		},
	} {
		overpaid := money.New(-2500, "USD")
		if acc.OpeningBalance.Amount < 0 {
			overpaid = money.New(2500, "USD")
		}
		f.Render(ui.CreateElement(AccountRow, accountRowProps{Account: acc, Balance: overpaid, Cleared: overpaid}))
		got := f.Text()
		if strings.Contains(got, "($25.00)") || strings.Contains(got, "-$25.00") || strings.Contains(got, "−$25.00") {
			t.Errorf("an overpaid card (opening %d) reads as debt:\n%s", acc.OpeningBalance.Amount, got)
		}
		if !strings.Contains(got, "25.00") {
			t.Errorf("the credit balance is missing entirely (opening %d):\n%s", acc.OpeningBalance.Amount, got)
		}
	}
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

// setBalanceNote is the one line under the Update balance field. Figures are in the
// stated convention: a debt is negative, and the line says which way it moved.

func setBalNoteText(t *testing.T, a domain.Account, curStated, targetStated int64) string {
	t.Helper()
	f := render.New(t)
	f.Render(setBalanceNote(a, curStated, targetStated))
	return f.Text()
}

func TestSetBalanceNoteExplainsDebtMoves(t *testing.T) {
	loan := domain.Account{ID: "loan", Name: "Loan", Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD"}
	tests := []struct {
		name            string
		current, target int64
		want            string
	}{
		{"debt goes down", -800000, -700000, "Debt down $1,000.00"},
		{"debt goes up", -800000, -850000, "Debt up $500.00"},
		{"debt unchanged", -800000, -800000, "No change."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setBalNoteText(t, loan, tt.current, tt.target)
			if !strings.Contains(got, tt.want) {
				t.Errorf("note = %q, want it to contain %q", got, tt.want)
			}
		})
	}
}

func TestSetBalanceNoteWarnsOnDebtTypedWithoutMinus(t *testing.T) {
	loan := domain.Account{ID: "loan", Name: "Loan", Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD"}
	got := setBalNoteText(t, loan, -800000, 750000)
	if !strings.Contains(got, "Did you mean ($7,500.00)?") {
		t.Errorf("note = %q, want the minus-sign warning", got)
	}
}

func TestSetBalanceNoteSignsAssetChanges(t *testing.T) {
	checking := domain.Account{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}
	savings := domain.Account{ID: "sav", Name: "Savings", Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "USD"}
	other := domain.Account{ID: "oth", Name: "Other", Class: domain.ClassAsset, Type: domain.TypeOther, Currency: "USD"}
	tests := []struct {
		name            string
		account         domain.Account
		current, target int64
		want            string
	}{
		{"positive delta carries a plus", checking, 100000, 150000, "Change: +$500.00"},
		{"negative delta carries a minus", checking, 150000, 100000, "Change: −$500.00"},
		{"zero delta needs no adjustment", checking, 100000, 100000, "No change."},
		{"zero delta on another asset type", savings, 25000, 25000, "No change."},
		{"zero delta on a zero balance", other, 0, 0, "No change."},
		{"going deeper into overdraft is a signed change, not debt wording", checking, -150000, -200000, "Change: −$500.00"},
		{"crossing zero upward is a signed change", checking, -50000, 50000, "Change: +$1,000.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setBalNoteText(t, tt.account, tt.current, tt.target)
			if !strings.Contains(got, tt.want) {
				t.Errorf("note = %q, want it to contain %q", got, tt.want)
			}
			for _, banned := range []string{"Did you mean", "Debt up", "Debt down"} {
				if strings.Contains(got, banned) {
					t.Errorf("an asset note must never use debt wording %q: %q", banned, got)
				}
			}
		})
	}
}

func TestSetBalanceNoteOverdraftGoingPositiveIsNotAWarning(t *testing.T) {
	checking := domain.Account{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}
	got := setBalNoteText(t, checking, -150000, 50000)
	if strings.Contains(got, "Did you mean") {
		t.Errorf("an overdrawn asset typed positive is ordinary, got %q", got)
	}
}

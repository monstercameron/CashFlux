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
	got := setBalNoteText(t, checking, 100000, 150000)
	if !strings.Contains(got, "Change: +$500.00") {
		t.Errorf("note = %q, want a signed change", got)
	}
	if strings.Contains(got, "Did you mean") {
		t.Errorf("an asset must never get the debt warning: %q", got)
	}
}

func TestSetBalanceNoteOverdraftGoingPositiveIsNotAWarning(t *testing.T) {
	checking := domain.Account{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}
	got := setBalNoteText(t, checking, -150000, 50000)
	if strings.Contains(got, "Did you mean") {
		t.Errorf("an overdrawn asset typed positive is ordinary, got %q", got)
	}
}

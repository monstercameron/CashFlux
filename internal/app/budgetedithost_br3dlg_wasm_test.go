// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestBudgetEditDialogTitlesNameTheBudget(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	if err := app.PutBudget(domain.Budget{ID: "groceries", Name: "Groceries", Limit: money.New(45000, "USD")}); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous })

	for _, test := range []struct {
		mode string
		want string
	}{
		{mode: uistate.BudgetEditModeTopup, want: "Raise Groceries limit"},
		{mode: uistate.BudgetEditModeCover, want: "Cover Groceries from another budget"},
	} {
		if got := budgetEditDialogTitle(test.mode, "groceries"); got != test.want {
			t.Errorf("dialog title for %q = %q, want %q", test.mode, got, test.want)
		}
	}
}

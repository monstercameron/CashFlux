// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func transactionRowRegressionProps() transactionRowProps {
	return transactionRowProps{
		Txn: domain.Transaction{
			ID: "tx-row-hook", Desc: "Lunch",
			Amount: money.New(-1250, "USD"),
		},
		OnSave: func(domain.Transaction, string, string, string, string, string, string, string) bool {
			return true
		},
	}
}

func findRowEditButton(f *render.Fixture) *render.QueryNode {
	for _, button := range f.AllByTag("button") {
		if button.Attr("aria-label") == uistate.T("transactions.editTitle") {
			return button
		}
	}
	return nil
}

// TestTransactionRowCanOpenAndCloseEditTwice keeps the same keyed row mounted
// while edit mode changes, exercising the hook ordering on both transitions.
func TestTransactionRowCanOpenAndCloseEditTwice(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(TransactionRow, transactionRowRegressionProps()))

	for cycle := 0; cycle < 2; cycle++ {
		edit := findRowEditButton(f)
		if edit == nil {
			t.Fatalf("cycle %d: edit button missing while row is closed", cycle+1)
		}
		edit.Click()
		if f.ByID("txn-edit-tx-row-hook") == nil {
			t.Fatalf("cycle %d: editor did not open", cycle+1)
		}

		forms := f.AllByTag("form")
		if len(forms) == 0 {
			t.Fatalf("cycle %d: edit form missing", cycle+1)
		}
		forms[0].Dispatch("onkeydown", render.Event{Key: "Escape"})
		if f.ByID("txn-edit-tx-row-hook") != nil {
			t.Fatalf("cycle %d: Escape did not close the editor", cycle+1)
		}
	}
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestTransactionLedgerExpenseUsesSignedNegativeTone(t *testing.T) {
	text, tone := fmtLedgerAmount(money.New(-48000, "USD"), false)
	f := render.New(t)
	f.Render(Table(Tbody(ui.CreateElement(txnFrameRow, &txnFrameRowProps{
		ID: "money-red-expense", Date: "Oct 19", Desc: "Groceries",
		Amount: text, AmtTone: tone, AmountMoney: money.New(-48000, "USD"),
		Vis: uistate.TxnCols{Amount: true}, OnOpen: func(string) {}, OnToggleSelect: func(string, bool) {},
	}))))
	row := findByTestID(f, "tr", "txn-row-money-red-expense")
	if row == nil {
		t.Fatal("transaction ledger expense row was not rendered")
	}
	var amount *render.QueryNode
	for _, cell := range row.Children() {
		if strings.Contains(cell.Attr("class"), "td-amount") {
			amount = cell
			break
		}
	}
	if amount == nil || amount.Text() != "−$480.00" || !strings.Contains(amount.Attr("class"), "text-down") {
		t.Fatalf("ledger expense amount = %v, want −$480.00 with negative tone", amount)
	}
}

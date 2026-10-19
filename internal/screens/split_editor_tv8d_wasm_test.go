// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTV8DSplitModalHasEditableRowAndReceiptActionWithoutInlineTitle(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(SplitEditor, splitEditorProps{
		Txn:            domain.Transaction{ID: "receipt", Amount: money.New(-10000, "USD")},
		Categories:     []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}},
		OnReceiptSplit: func(string) {},
		FooterFormID:   SplitModalFormID,
	}))

	if f.ByID(SplitModalFormID) == nil {
		t.Fatal("split modal form did not render")
	}
	if byTestID(f, "div", "split-row") == nil {
		t.Fatal("split modal did not render its editable row")
	}
	if byTestID(f, "button", "txn-receipt-split-open") == nil {
		t.Fatal("receipt split action did not render")
	}
	if byTestID(f, "p", "split-inline-title") != nil {
		t.Fatal("modal rendered the inline split title in addition to its shared title")
	}
}

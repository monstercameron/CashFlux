// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

type txnPageCategoryPickerProps struct {
	App   *appstate.App
	ID    string
	Close func()
}

// The ledger owns a single picker. Rows emit an ID and never subscribe to the
// picker's data revision or construct category options while the picker is closed.
func txnPageCategoryPicker(props txnPageCategoryPickerProps) ui.Node {
	if props.ID == "" || props.App == nil {
		return Fragment()
	}
	txn, ok := props.App.TransactionByID(props.ID)
	if !ok {
		return Fragment()
	}
	return ui.Portal(ui.PortalProps{Target: ui.PortalTarget{Selector: "body"},
		Child: ui.CreateElement(CategoryPicker, CategoryPickerProps{
			Kind: txnCategoryKind(txn.Amount), Selected: txn.CategoryID,
			OnClose: props.Close,
			OnPick:  func(categoryID string) { applyTxnManualCategory(props.ID, categoryID); props.Close() },
		}),
	})
}

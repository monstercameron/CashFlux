// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestR4KRowMenuConsolidatesWithoutDroppingActions(t *testing.T) {
	var linkedID, linkedMode string
	props := txnFrameRowProps{
		ID: "r4k-row", AutoCategory: true, IsIncome: true, IsRefunded: true, IsBillMatched: true, GroupSize: 2, HasSplits: true,
		OnConfirmCategory: func(string) {}, OnSuggestCategory: func(string) {}, OnOpenSplit: func(string) {},
		OnPairRefund: func(string) {}, OnUnpair: func(string) {}, OnUnlinkBill: func(string) {}, OnUngroup: func(string) {},
		OnOpenLink: func(id, mode string) { linkedID, linkedMode = id, mode }, OnToggleExclude: func(string) {}, OnDelete: func(string) {},
	}
	items := txnRowMenuItems(props)
	if len(items) > 8 {
		t.Fatalf("menu contains %d top-level actions, want at most 8", len(items))
	}
	for _, id := range []string{"txn-confirm-category", "txn-categorize-open", "txn-split-open", "txn-link-open", "txn-toggle-exclude", "txn-delete"} {
		if !menuHasID(items, id) {
			t.Errorf("row menu is missing %q", id)
		}
	}
	var linkItem *uiw.OverflowMenuItem
	for i := range items {
		if items[i].TestID == "txn-link-open" {
			linkItem = &items[i]
		}
	}
	if linkItem == nil {
		t.Fatal("Link to… submenu is missing")
	}
	for _, id := range []string{"txn-markbill-open", "txn-marksub-open", "txn-pair-refund", "txn-unpair", "txn-unlink-bill", "txn-ungroup"} {
		if !menuHasID(linkItem.Submenu, id) {
			t.Errorf("Link submenu is missing %q", id)
		}
	}
	for _, child := range linkItem.Submenu {
		if child.TestID == "txn-markbill-open" {
			child.OnSelect()
		}
	}
	if linkedID != props.ID || linkedMode != uistate.TxnLinkModeBill {
		t.Fatalf("bill submenu selected (%q, %q)", linkedID, linkedMode)
	}

	menu := render.New(t)
	menu.Render(uiw.OverflowMenu(uiw.OverflowMenuProps{Items: items, TriggerTestID: "r10w-menu-trigger"}))
	findByTestID(menu, "button", "r10w-menu-trigger").Click()
	linkButton := findByTestID(menu, "button", "txn-link-open")
	if linkButton == nil {
		t.Fatal("Link to… menu item did not render")
	}
	linkButton.Click()
	if findByTestID(menu, "button", "txn-markbill-open") == nil {
		t.Fatal("Link submenu did not open on activation")
	}
	menu.Cleanup()

	app := newR3EEditApp(t)
	txn := domain.Transaction{ID: "r4k-expense", AccountID: "acct-1", Date: time.Now(), Payee: "Market", Desc: "Market", Amount: money.New(-4200, "USD")}
	if err := app.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	edit := render.New(t)
	edit.Render(ui.CreateElement(TransactionEditForm, TransactionEditFormProps{TxnID: txn.ID}))
	moreActions := findByTestID(edit, "button", "txn-edit-more-actions")
	if moreActions == nil {
		t.Fatal("Edit form More actions menu is missing")
	}
	moreActions.Click()
	for _, id := range []string{"txn-create-rule", "txn-categorize-open", "txn-cleanname-open", "txn-followup-task", "txn-history-open"} {
		if findByTestID(edit, "button", id) == nil {
			t.Errorf("Edit form cannot reach %q", id)
		}
	}
	edit.Cleanup()
	split := render.New(t)
	split.Render(ui.CreateElement(SplitEditor, splitEditorProps{Txn: txn, OnReceiptSplit: func(string) {}}))
	if findByTestID(split, "button", "txn-receipt-split-open") == nil {
		t.Fatal("Split editor no longer exposes Split from receipt")
	}
	split.Cleanup()
}
func menuHasID(items []uiw.OverflowMenuItem, id string) bool {
	for _, item := range items {
		if item.TestID == id {
			return true
		}
	}
	return false
}

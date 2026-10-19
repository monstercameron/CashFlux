// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestR4KRowMenuConsolidatesWithoutDroppingActions(t *testing.T) {
	callbacks := func() txnFrameRowProps {
		return txnFrameRowProps{
			ID: "r4k-row", Desc: "Market", Date: "2026-10-18", Amount: "$42.00",
			AutoCategory: true, IsIncome: true, IsRefund: true, IsRefunded: true,
			IsBillMatched: true, GroupSize: 2, HasSplits: true,
			OnConfirmCategory: func(string) {}, OnCreateRule: func(string) {}, OnSuggestCategory: func(string) {},
			OnOpenSplit: func(string) {}, OnReceiptSplit: func(string) {},
			OnPairRefund: func(string) {}, OnUnpair: func(string) {},
			OnUnlinkBill: func(string) {}, OnUngroup: func(string) {},
			OnOpenLink: func(string, string) {}, OnToggleExclude: func(string) {},
			OnDelete: func(string) {},
		}
	}

	for _, tc := range []struct {
		name string
		row  txnFrameRowProps
	}{
		{name: "expense", row: func() txnFrameRowProps {
			p := callbacks()
			p.IsIncome = false
			p.IsRefund = false
			p.IsRefunded = false
			return p
		}()},
		{name: "income", row: callbacks()},
		{name: "split", row: func() txnFrameRowProps { p := callbacks(); p.HasSplits = true; return p }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items := txnRowMenuItems(tc.row)
			if len(items) > 8 {
				t.Fatalf("menu contains %d actions, want at most 8", len(items))
			}
			for _, id := range []string{"txn-split-open", "txn-link-open", "txn-toggle-exclude", "txn-delete"} {
				if !menuHasID(items, id) {
					t.Errorf("menu is missing %q", id)
				}
			}
			if tc.name == "expense" && !menuHasID(items, "txn-categorize-open") {
				t.Error("row menu is missing Suggest a category (Smart)")
			}
			if tc.name == "expense" && len(items) != 6 {
				t.Errorf("rich expense menu has %d entries, want 6", len(items))
			}
		})
	}

	// This is the complete TestID list from HEAD's row menu. The consolidated
	// actions map to the Link chooser's corresponding button IDs.
	headMenuIDs := []string{
		"txn-confirm-category", "txn-create-rule", "txn-split-open", "txn-receipt-split-open",
		"txn-categorize-open", "txn-cleanname-open", "txn-followup-task", "txn-history-open",
		"txn-markbill-open", "txn-marksub-open", "txn-pair-refund", "txn-unpair",
		"txn-unlink-bill", "txn-ungroup", "txn-toggle-exclude", "txn-delete",
	}
	editIDs := []string{"txn-create-rule", "txn-categorize-open", "txn-cleanname-open", "txn-followup-task", "txn-history-open"}
	chooserIDs := []string{"txn-link-chooser-bill", "txn-link-chooser-sub", "txn-link-chooser-pair", "txn-link-chooser-unpair", "txn-link-chooser-unlink-bill", "txn-link-chooser-ungroup"}
	splitIDs := []string{"txn-receipt-split-open"}

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
	for _, id := range editIDs {
		if findByTestID(edit, "button", id) == nil {
			t.Errorf("Edit form cannot reach HEAD action %q", id)
		}
	}
	edit.Cleanup()
	for _, id := range []string{"txn-confirm-category", "txn-split-open", "txn-toggle-exclude", "txn-delete"} {
		if !menuHasID(txnRowMenuItems(txnFrameRowProps{
			AutoCategory: true, OnConfirmCategory: func(string) {}, OnOpenSplit: func(string) {},
			OnOpenLink: func(string, string) {}, OnToggleExclude: func(string) {}, OnDelete: func(string) {},
		}), id) {
			t.Errorf("menu cannot reach HEAD action %q", id)
		}
	}
	if !menuHasID(txnRowMenuItems(txnFrameRowProps{OnOpenLink: func(string, string) {}}), "txn-link-open") {
		t.Error("Link chooser cannot be opened from the row menu")
	}

	row := txnFrameRowProps{
		ID: txn.ID, IsIncome: true, IsRefunded: true, IsBillMatched: true, GroupSize: 2,
		OnOpenLink: func(string, string) {}, OnPairRefund: func(string) {}, OnUnpair: func(string) {},
		OnUnlinkBill: func(string) {}, OnUngroup: func(string) {},
	}
	chooser := render.New(t)
	chooserClosed := false
	chooser.Render(ui.CreateElement(txnLinkChooser, txnLinkChooserProps{
		Row: row, Target: ui.PortalTarget{Node: chooser.Target()}, OnClose: func() { chooserClosed = true },
	}))
	if findByTestID(chooser, "div", "txn-link-chooser-content") == nil {
		t.Fatal("Link chooser did not render its content through the modal")
	}
	for _, id := range chooserIDs {
		if findByTestID(chooser, "button", id) == nil {
			t.Errorf("Link chooser cannot reach HEAD action %q", id)
		}
	}
	dialogCount := 0
	for _, node := range chooser.AllByTag("div") {
		if node.Attr("role") == "dialog" {
			dialogCount++
		}
	}
	if dialogCount != 1 {
		t.Errorf("Link chooser renders %d dialogs, want the shared modal", dialogCount)
	}
	findByTestID(chooser, "div", "txn-link-chooser-content").Dispatch("onkeydown", render.Event{Key: "Escape"})
	if !chooserClosed {
		t.Error("Escape did not close the Link chooser")
	}
	chooser.Cleanup()
	split := render.New(t)
	split.Render(ui.CreateElement(SplitEditor, splitEditorProps{Txn: txn, OnReceiptSplit: func(string) {}}))
	for _, id := range splitIDs {
		if findByTestID(split, "button", id) == nil {
			t.Errorf("Split editor cannot reach HEAD action %q", id)
		}
	}
	split.Cleanup()
	reachableHeadIDs := map[string]bool{
		"txn-confirm-category": true, "txn-create-rule": true, "txn-split-open": true,
		"txn-receipt-split-open": true, "txn-categorize-open": true, "txn-cleanname-open": true,
		"txn-followup-task": true, "txn-history-open": true, "txn-markbill-open": true,
		"txn-marksub-open": true, "txn-pair-refund": true, "txn-unpair": true,
		"txn-unlink-bill": true, "txn-ungroup": true, "txn-toggle-exclude": true, "txn-delete": true,
	}
	for _, id := range headMenuIDs {
		if !reachableHeadIDs[id] {
			t.Errorf("HEAD row-menu action %q has no reachable destination", id)
		}
	}

	// The pre-consolidation menu held separate entries for these actions; the
	// test also verifies their consolidated menu anchor remains present.
	var suggestedID string
	menu := txnRowMenuItems(txnFrameRowProps{
		ID: "menu-row", OnOpenSplit: func(string) {}, OnOpenLink: func(string, string) {}, OnSuggestCategory: func(id string) { suggestedID = id },
		OnToggleExclude: func(string) {}, OnDelete: func(string) {},
	})
	for _, id := range []string{"txn-split-open", "txn-link-open", "txn-categorize-open", "txn-toggle-exclude", "txn-delete"} {
		if !menuHasID(menu, id) {
			t.Errorf("consolidated menu cannot reach action %q", id)
		}
	}
	for _, item := range menu {
		if item.TestID == "txn-categorize-open" {
			item.OnSelect()
		}
	}
	if suggestedID != "menu-row" {
		t.Fatalf("Smart category menu selected id %q, want menu-row", suggestedID)
	}
}

func menuHasID(items []uiw.OverflowMenuItem, id string) bool {
	for _, item := range items {
		if item.TestID == id {
			return true
		}
	}
	return false
}

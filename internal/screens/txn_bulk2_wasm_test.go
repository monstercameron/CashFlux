// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestBULK2BulkBarKeepsPrimaryControlsInlineAndNamesActionsOnce(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnBulkBarSelectionHarness, app))

	var bar *render.QueryNode
	for _, node := range f.AllByTag("div") {
		if strings.Contains(node.Attr("class"), "txn-bulkbar-overlay") {
			bar = node
			break
		}
	}
	if bar == nil {
		t.Fatal("bulk bar floating surface did not render")
	}
	if !strings.Contains(bar.Attr("class"), "bulk-bar-refined") {
		t.Errorf("bulk bar classes = %q, want refined bar class", bar.Attr("class"))
	}
	if !strings.Contains(bar.Text(), "Owners") {
		t.Error("Owners picker label is missing from the bulk action row")
	}
	for _, id := range []string{"bulk-category-picker", "bulk-owner-picker", "bulk-mark-reviewed", "bulk-delete", "bulk-more-actions", "bulk-clear-selection"} {
		if bulk2ByTestID(f, "button", id) == nil {
			t.Errorf("primary bulk control %q is missing", id)
		}
	}
	if bulk2ByTestID(f, "button", "bulk-apply-owners") != nil {
		t.Error("owner choices apply when the picker closes; a separate Apply control must not render")
	}
	for _, id := range []string{"bulk-mark-reviewed", "bulk-delete"} {
		button := bulk2ByTestID(f, "button", id)
		if button == nil {
			continue
		}
		if got, want := button.Attr("aria-label"), button.Text(); got != want {
			t.Errorf("%s accessible name = %q, visible text = %q", id, got, want)
		}
		if !strings.Contains(button.Attr("class"), "txn-bulk-inline-") {
			t.Errorf("%s is not marked as an inline desktop primary action: %q", id, button.Attr("class"))
		}
	}
	clear := bulk2ByTestID(f, "button", "bulk-clear-selection")
	if clear == nil || clear.Text() != "×" || clear.Attr("aria-label") != "Clear selection" {
		t.Errorf("clear selection control = %v, want × with accessible name Clear selection", clear)
	}

	bulk2ByTestID(f, "button", "bulk-more-actions").Click()
	f.Stabilize()
	for _, id := range []string{"bulk-add-tag-menu", "bulk-exclude", "bulk-include", "bulk-mark-cleared", "bulk-mark-uncleared", "bulk-export-selected", "bulk-mark-reviewed-menu", "bulk-delete-menu", "bulk-clear-category", "bulk-group-order"} {
		if bulk2ByTestID(f, "button", id) == nil {
			t.Errorf("overflow action %q is not reachable", id)
		}
	}
}

func TestBULK2OwnersPickerAppliesMemberSelection(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.PutMember(domain.Member{ID: "m3", Name: "Morgan"}); err != nil {
		t.Fatalf("add member: %v", err)
	}
	options := txnBulkOwnerPickerOptions(app.Members())
	if len(options) != 2 || options[0].Value != txnBulkUnassignedOwner || options[0].Label != "Unassigned" || options[1].Value != "m3" || options[1].Label != "Morgan" {
		t.Fatalf("Owners picker options = %#v, want Unassigned and Morgan", options)
	}
	original := domain.Transaction{ID: "bulk-ui", AccountID: "account", Desc: "Shared purchase", Amount: money.New(-1000, "USD"), Date: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC), MemberIDs: []string{"m1", "m2"}}
	if err := app.PutTransaction(original); err != nil {
		t.Fatal(err)
	}
	selected := toggleTxnBulkOwner(nil, "m3")
	owners := txnBulkAppliedOwners(selected)
	if len(owners) != 1 || owners[0] != "m3" {
		t.Fatalf("committed member selection = %#v, want [m3]", owners)
	}
	if _, err := assignSelectedTxnOwners(app, map[string]bool{original.ID: true}, owners); err != nil {
		t.Fatalf("apply selected owner: %v", err)
	}
	updated, ok := findTransactionByID(app.Transactions(), original.ID)
	if !ok || len(updated.Owners()) != 1 || updated.Owners()[0] != "m3" {
		t.Fatalf("applied owners = %#v, exists=%t; want [m3]", updated.Owners(), ok)
	}
}

func bulk2ByTestID(f *render.Fixture, tag, id string) *render.QueryNode {
	for _, node := range f.AllByTag(tag) {
		if node.Attr("data-testid") == id {
			return node
		}
	}
	return nil
}

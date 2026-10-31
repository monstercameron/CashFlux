// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestEditTV7DShowsAmountOnlyInItsField(t *testing.T) {
	app, txn := reviewTestApp(t)
	f := render.New(t)
	pickerFixtureTarget(t, f)
	defer f.Cleanup()
	f.Render(ui.CreateElement(TransactionEditForm, TransactionEditFormProps{App: app, TxnID: txn.ID}))

	if findByTestID(f, "input", "txn-edit-amount") == nil {
		t.Fatal("Edit amount field is missing")
	}
	for _, span := range f.AllByTag("span") {
		if strings.Contains(span.Attr("class"), "txn-form-record-amount") {
			t.Fatalf("the transaction amount is duplicated in the header: %q", span.Text())
		}
	}
	category := findByTestID(f, "select", "txn-edit-category")
	if category == nil {
		t.Fatal("Edit category selector is missing")
	}
	var hasNewCategoryOption bool
	for _, option := range f.AllByTag("option") {
		if option.Property("value") == txnEditNewCategoryOption && option.Text() == "New category…" {
			hasNewCategoryOption = true
		}
	}
	if !hasNewCategoryOption {
		t.Fatal("New category action should appear as a named category option")
	}
	category.Change(txnEditNewCategoryOption)
	f.Stabilize()
	if findByTestID(f, "div", "catpick") == nil {
		t.Fatal("choosing New category should open the category picker")
	}
	if findByTestID(f, "button", "txn-edit-newcat-toggle") != nil {
		t.Fatal("Edit should not render a separate bare plus button")
	}
}

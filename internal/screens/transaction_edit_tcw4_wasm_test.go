// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestTCW4EditCategoryUsesItsApp(t *testing.T) {
	owner := newR3EEditApp(t)
	other := newR3EEditApp(t) // Becomes the shell default, with the same revision.
	if owner.Rev() != other.Rev() {
		t.Fatal("fixture revisions differ")
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnEditCategory, txnEditCategoryProps{App: owner, Open: true, Kind: domain.KindExpense,
		OnOpen: func() {}, OnClose: func() {}, OnSelect: func(string) {}, OnPick: func(string) {}}))
	findByTestID(f, "input", "catpick-search").Input("Owner category")
	f.Stabilize()
	findByTestID(f, "button", "catpick-create").Click()
	f.Stabilize()
	if len(owner.Categories()) != 2 || len(other.Categories()) != 1 {
		t.Fatal("category was created in a different app's store")
	}
}

func TestTCW4EditAccountPreviewSeparatesAppIdentity(t *testing.T) {
	first, second := newR3EEditApp(t), newR3EEditApp(t)
	for _, pair := range []struct {
		appIndex int
		name     string
	}{{0, "First savings"}, {1, "Second savings"}} {
		app := first
		if pair.appIndex == 1 {
			app = second
		}
		account, _ := domain.AccountByID(app.Accounts(), "acct-2")
		account.Name = pair.name
		if err := app.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	if first.Rev() != second.Rev() {
		t.Fatal("fixture revisions differ")
	}
	props := txnEditAccountProps{App: first, Txn: domain.Transaction{ID: "same-id", AccountID: "acct-1", Amount: money.New(-100, "USD")},
		Selected: "acct-2", OnSelect: func(string) {}}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnEditAccount, props))
	want := uistate.T("transactions.accountMoveEffect", "Checking", "First savings")
	if findByTestID(f, "p", "txn-edit-account-note").Text() != want {
		t.Fatal("first preview used the wrong owner")
	}
	props.App = second
	f.Rerender(ui.CreateElement(txnEditAccount, props))
	want = uistate.T("transactions.accountMoveEffect", "Checking", "Second savings")
	if findByTestID(f, "p", "txn-edit-account-note").Text() != want {
		t.Fatal("memo reused another app's preview at an equal revision")
	}
}

func TestTCW4EditTypingReusesPreviews(t *testing.T) {
	app := newR3EEditApp(t)
	row := domain.Transaction{ID: "tcw4-memo", AccountID: "acct-1", Payee: "TCW4 Market", Desc: "TCW4 Market", Date: time.Now(), Amount: money.New(-100, "USD")}
	if err := app.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	if err := app.PutRule(rules.Rule{ID: "tcw4-rule", Match: row.Payee, SetCategoryID: "cat-food"}); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	previous := txnPerfObserve
	txnPerfObserve = func(stage string, _ time.Duration) { counts[stage]++ }
	t.Cleanup(func() { txnPerfObserve = previous })
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(TransactionEditForm, TransactionEditFormProps{App: app, TxnID: row.ID}))
	f.Stabilize()
	for _, id := range []string{"txn-edit-payee", "txn-edit-description", "txn-edit-tags", "txn-edit-note"} {
		for _, text := range []string{"a", "ab", "abc"} {
			tag := "input"
			if id == "txn-edit-note" {
				tag = "textarea"
			}
			findByTestID(f, tag, id).Input(text)
			f.Stabilize()
		}
	}
	for _, stage := range []string{"edit-category-suggestion", "edit-account-preview", "edit-movement-preview"} {
		if counts[stage] != 1 {
			t.Errorf("%s computed %d times during unrelated typing; want once", stage, counts[stage])
		}
	}
	if t.Failed() {
		return
	}
	findByTestID(f, "input", "txn-edit-amount").Input("2")
	f.Stabilize()
	if counts["edit-category-suggestion"] != 2 || counts["edit-account-preview"] != 1 || counts["edit-movement-preview"] != 1 {
		t.Fatalf("amount invalidation = %v", counts)
	}
	findByTestID(f, "select", "txn-edit-account").Change("acct-2")
	f.Stabilize()
	if counts["edit-account-preview"] != 2 {
		t.Fatalf("account selection did not refresh preview: %v", counts)
	}
	findByTestID(f, "select", "txn-edit-classify").Change("acct-2")
	f.Stabilize()
	if counts["edit-movement-preview"] != 2 {
		t.Fatalf("movement selection did not refresh preview: %v", counts)
	}
	if err := app.PutCategory(domain.Category{ID: "tcw4-added", Name: "Added", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if counts["edit-category-suggestion"] != 3 || counts["edit-account-preview"] != 3 || counts["edit-movement-preview"] != 3 {
		t.Fatalf("data revision did not refresh all previews: %v", counts)
	}
}

func TestTCW4EditCategoryDismissAndCreate(t *testing.T) {
	for _, categoryID := range []string{"", "cat-food"} {
		t.Run(categoryID, func(t *testing.T) {
			app := newR3EEditApp(t)
			row := domain.Transaction{ID: "tcw4-picker", AccountID: "acct-1", CategoryID: categoryID,
				Payee: "Market", Desc: "Market", Date: time.Now(), Amount: money.New(-100, "USD")}
			if err := app.PutTransaction(row); err != nil {
				t.Fatal(err)
			}
			f := render.New(t)
			defer f.Cleanup()
			f.Render(ui.CreateElement(TransactionEditForm, TransactionEditFormProps{App: app, TxnID: row.ID}))
			for attempt := 0; attempt < 2; attempt++ {
				findByTestID(f, "select", "txn-edit-category").Change(txnEditNewCategoryOption)
				f.Stabilize()
				if findByTestID(f, "div", "catpick") == nil {
					t.Fatal("New category did not open the picker")
				}
				findByTestID(f, "button", "catpick-close").Click()
				f.Stabilize()
				if value := findByTestID(f, "select", "txn-edit-category").Property("value"); value != categoryID {
					t.Fatalf("after dismissal select value = %q, want %q", value, categoryID)
				}
			}
			findByTestID(f, "select", "txn-edit-category").Change(txnEditNewCategoryOption)
			f.Stabilize()
			findByTestID(f, "input", "catpick-search").Input("TCW4 new category")
			f.Stabilize()
			findByTestID(f, "button", "catpick-create").Click()
			f.Stabilize()
			var created string
			for _, cat := range app.Categories() {
				if cat.Name == "TCW4 new category" {
					created = cat.ID
				}
			}
			if created == "" || findByTestID(f, "div", "catpick") != nil {
				t.Fatal("new category was not created and selected")
			}
			if value := findByTestID(f, "select", "txn-edit-category").Property("value"); value != created {
				t.Fatalf("after creation select value = %q, want %q", value, created)
			}
			findByTestID(f, "input", "txn-edit-payee").Input("Updated merchant")
			f.Stabilize()
			if value := findByTestID(f, "select", "txn-edit-category").Property("value"); value != created {
				t.Fatalf("typing reset the new selection to %q", value)
			}
			// Picking an ordinary category never opens the create overlay.
			findByTestID(f, "select", "txn-edit-category").Change("cat-food")
			f.Stabilize()
			if findByTestID(f, "div", "catpick") != nil {
				t.Fatal("ordinary selection opened the picker")
			}
		})
	}
}

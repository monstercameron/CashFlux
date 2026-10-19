// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestQuickAddForm3UsesSharedGridAndFieldOrder(t *testing.T) {
	appState, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range []domain.Account{
		{ID: "form3-checking", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
		{ID: "form3-savings", Name: "Savings", Currency: "USD", Type: domain.TypeSavings, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
	} {
		if err := appState.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	previous := appstate.Default
	appstate.Default = appState
	t.Cleanup(func() { appstate.Default = previous; uistate.SetQuickAdd(false) })

	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ID("form3-add"), ui.CreateElement(QuickAddHost)))
	uistate.SetQuickAdd(true)
	f.Render(Body(ID("form3-add"), ui.CreateElement(QuickAddHost)))
	text := f.Text()
	ordered := []string{
		uistate.T("txu3.sentenceLabel"), uistate.T("quickAdd.amount"), uistate.T("quickAdd.kind"),
		uistate.T("quickAdd.payee"), uistate.T("quickAdd.date"), uistate.T("quickAdd.category"), uistate.T("quickAdd.account"),
	}
	last := -1
	for _, label := range ordered {
		at := strings.Index(text[last+1:], label)
		if at < 0 {
			t.Fatalf("field %q is out of Add form order in %q", label, text)
		}
		last += at + 1
	}
	var form *render.QueryNode
	for _, node := range f.AllByTag("div") {
		if strings.Contains(node.Attr("class"), "txn-form-grid") {
			form = node
			break
		}
	}
	if form == nil || !strings.Contains(form.Attr("class"), "txn-form-add") {
		t.Fatalf("Add form is missing shared grid classes: %#v", form)
	}
	if strings.Contains(text, "Use Type to choose money in or money out.") {
		t.Fatalf("redundant Amount helper remains in Add form: %q", text)
	}
	if !strings.Contains(text, "No category") || strings.Contains(text, "— No category —") {
		t.Fatalf("Add category option should read No category without decoration: %q", text)
	}
}

func TestQuickAddRequiredFieldErrors(t *testing.T) {
	accounts := []domain.Account{{ID: "checking", Currency: "USD"}}
	tests := []struct {
		name, accountID, amount, kind, payee, description string
		wantField, wantKey                                string
	}{
		{name: "missing account", amount: "12.50", kind: "Expense", description: "Lunch", wantField: "txn-add-account", wantKey: "txu3.needAccount"},
		{name: "empty amount", accountID: "checking", kind: "Expense", description: "Lunch", wantField: "txn-add-amount", wantKey: "txu3.needAmount"},
		{name: "invalid amount", accountID: "checking", amount: "coffee", kind: "Expense", description: "Lunch", wantField: "txn-add-amount", wantKey: "txu3.needAmount"},
		{name: "both payee and description missing", accountID: "checking", amount: "12.50", kind: "Expense", wantField: "txn-add-payee", wantKey: "r4f.addPayeeOrDescription"},
		{name: "payee only is valid", accountID: "checking", amount: "12.50", kind: "Expense", payee: "Corner Market"},
		{name: "description only is valid", accountID: "checking", amount: "12.50", kind: "Expense", description: "Lunch"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			field, key := quickAddInvalidFieldR4F(accounts, tc.accountID, tc.amount, tc.kind, tc.payee, tc.description)
			if field != tc.wantField || key != tc.wantKey {
				t.Fatalf("quickAddInvalidFieldR4F() = (%q, %q), want (%q, %q)", field, key, tc.wantField, tc.wantKey)
			}
		})
	}
}

func TestQuickAddDefaultsToReviewedAndCurrentMember(t *testing.T) {
	if !quickAddReviewedValue(false) {
		t.Fatal("an unchecked Needs review later box should save as reviewed")
	}
	if quickAddReviewedValue(true) {
		t.Fatal("a checked Needs review later box should save as not reviewed")
	}
	if got := quickAddMemberValue("member-current", "", false); got != "member-current" {
		t.Fatalf("default member = %q, want current member", got)
	}
	if got := quickAddMemberValue("member-current", "member-other", true); got != "member-other" {
		t.Fatalf("explicit member = %q, want selected member", got)
	}
	if got := quickAddMemberValue("member-current", "", true); got != "" {
		t.Fatalf("explicit automatic assignment = %q, want empty", got)
	}
}

func TestQuickAddNextFocusAfterSentenceFill(t *testing.T) {
	accounts := []domain.Account{{ID: "checking", Currency: "USD"}}
	tests := []struct {
		name, accountID, amount, payee, description string
		fill                                        screens.TxnDraftFill
		want                                        string
	}{
		{name: "missing account", amount: "40", payee: "Whole Foods", fill: screens.TxnDraftFill{AmountMajor: "40", Payee: "Whole Foods"}, want: "txn-add-account"},
		{name: "missing amount", accountID: "checking", fill: screens.TxnDraftFill{Payee: "Whole Foods"}, want: "txn-add-amount"},
		{name: "missing payee and description", accountID: "checking", fill: screens.TxnDraftFill{AmountMajor: "40"}, want: "txn-add-payee"},
		{name: "all required values filled", accountID: "checking", fill: screens.TxnDraftFill{AmountMajor: "40", Payee: "Whole Foods"}, want: "txn-add-save"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := quickAddNextFocusR4F(accounts, tc.accountID, tc.amount, tc.payee, tc.description, tc.fill); got != tc.want {
				t.Fatalf("quickAddNextFocusR4F() = %q, want %q", got, tc.want)
			}
		})
	}
}

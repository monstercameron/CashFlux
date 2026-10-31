// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestCSEDITInputChildren(t *testing.T) {
	for _, tc := range []struct {
		name, id, kind string
		component      func(txnEditInputProps) ui.Node
	}{
		{"amount", "txn-edit-amount", "text", txnEditAmount},
		{"payee", "txn-edit-payee", "text", txnEditPayee},
		{"date", "txn-edit-date", "date", txnEditDate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			defer f.Cleanup()
			got := ""
			f.Render(ui.CreateElement(tc.component, txnEditInputProps{Value: "initial", Error: "Check this field", OnInput: func(v string) { got = v }}))
			field := f.ByID(tc.id)
			if field == nil || field.Attr("type") != tc.kind || field.Attr("data-cf-value") != "initial" {
				t.Fatalf("wrong controlled input: %#v", field)
			}
			if tc.name != "payee" && field.Attr("aria-invalid") != "true" {
				t.Fatal("missing inline validation")
			}
			field.Input("changed")
			if got != "changed" {
				t.Fatal(got)
			}
		})
	}
}

func TestCSEDITDirectionChild(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	got := ""
	f.Render(ui.CreateElement(txnEditDirection, txnEditDirectionProps{Value: txnDirOut, OnSelect: func(v string) { got = v }}))
	findByTestID(f, "button", "txn-edit-direction-income").Click()
	if got != txnDirIn {
		t.Fatal(got)
	}
	got = ""
	findByTestID(f, "button", "txn-edit-direction-transfer").Click()
	if got != "" {
		t.Fatal("transfer must explain without changing the direction")
	}
}

func TestCSEDITCategoryChild(t *testing.T) {
	newR3EEditApp(t)
	f := render.New(t)
	pickerFixtureTarget(t, f)
	defer f.Cleanup()
	opened := 0
	selected := ""
	props := txnEditCategoryProps{Kind: domain.KindExpense, Options: []uiw.SelectOption{{Value: "food", Label: "Food"}, {Value: txnEditNewCategoryOption, Label: "New category…"}},
		OnOpen: func() { opened++ }, OnSelect: func(v string) { selected = v }, OnPick: func(string) {}, OnClose: func() {}}
	f.Render(ui.CreateElement(txnEditCategory, props))
	findByTestID(f, "select", "txn-edit-category").Change("food")
	findByTestID(f, "select", "txn-edit-category").Change(txnEditNewCategoryOption)
	if selected != "food" || opened != 1 {
		t.Fatal(selected, opened)
	}
	props.Open = true
	f.Rerender(ui.CreateElement(txnEditCategory, props))
	if findByTestID(f, "div", "catpick") == nil {
		t.Fatal("picker did not open")
	}
	props.Open = false
	f.Rerender(ui.CreateElement(txnEditCategory, props))
	findByTestID(f, "select", "txn-edit-category").Change("food")
	if selected != "food" {
		t.Fatal("category hook moved after picker close")
	}
}

func TestCSEDITDetailsChild(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	values := map[string]string{}
	checks := map[string]bool{}
	props := txnEditDetailsProps{Desc: "bank", Tags: "tag", Note: "memo", HasMembers: true, MemberOptions: []uiw.PickerOption{{Value: "m", Label: "Cam"}},
		OnDesc: func(v string) { values["desc"] = v }, OnTags: func(v string) { values["tags"] = v }, OnNote: func(v string) { values["note"] = v }, OnOwner: func(v string) { values["owner"] = v },
		OnCleared: func(v bool) { checks["cleared"] = v }, OnExclude: func(v bool) { checks["exclude"] = v }}
	f.Render(ui.CreateElement(txnEditDetails, props))
	for _, tc := range []struct{ tag, id, key string }{{"input", "txn-edit-description", "desc"}, {"input", "txn-edit-tags", "tags"}, {"textarea", "txn-edit-note", "note"}} {
		findByTestID(f, tc.tag, tc.id).Input("new " + tc.key)
		if values[tc.key] != "new "+tc.key {
			t.Fatal(tc, values)
		}
	}
	findByTestID(f, "input", "txn-edit-cleared").Dispatch("onchange", render.Event{Checked: true})
	findByTestID(f, "input", "txn-edit-exclude").Dispatch("onchange", render.Event{Checked: true})
	if !checks["cleared"] || !checks["exclude"] {
		t.Fatal(checks)
	}
	props.HasMembers = false
	f.Rerender(ui.CreateElement(txnEditDetails, props))
	findByTestID(f, "input", "txn-edit-tags").Input("after")
	if values["tags"] != "after" {
		t.Fatal("details handlers moved when members vanished")
	}
}

func TestCSEDITToolsChild(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	selected := ""
	callback := func(id string) func() { return func() { selected = id } }
	props := txnEditToolsProps{ShowHistory: callback("history"), ShowCategorize: callback("categorize"), ShowPayeeClean: callback("clean"),
		CreateRuleAction: callback("rule"), FollowUpAction: callback("task"), ToggleSplit: callback("split"), Attach: callback("attach")}
	for _, tc := range []struct{ id, want string }{{"txn-history-open", "history"}, {"txn-categorize-open", "categorize"}, {"txn-cleanname-open", "clean"}, {"txn-create-rule", "rule"}, {"txn-followup-task", "task"}, {"txn-edit-split-toggle", "split"}, {"txn-edit-attach", "attach"}} {
		f.Render(ui.CreateElement(txnEditTools, props))
		findByTestID(f, "button", "txn-edit-more-actions").Click()
		action := findByTestID(f, "button", tc.id)
		if action == nil {
			t.Fatalf("missing %s", tc.id)
		}
		action.Click()
		if selected != tc.want {
			t.Fatal(tc, selected)
		}
	}
	props.Transfer = true
	f.Rerender(ui.CreateElement(txnEditTools, props))
	findByTestID(f, "button", "txn-edit-more-actions").Click()
	if findByTestID(f, "button", "txn-categorize-open") != nil {
		t.Fatal("transfer exposes categorization")
	}
}

func TestCSEDITAttachmentsChild(t *testing.T) {
	app := newR3EEditApp(t)
	f := render.New(t)
	defer f.Cleanup()
	row := domain.Transaction{Attachments: []domain.AttachmentRef{{ArtifactID: "missing", Name: "receipt.jpg"}}}
	f.Render(ui.CreateElement(txnEditAttachments, txnEditAttachmentsProps{App: app, Txn: row}))
	thumb := findByTestID(f, "button", "txn-receipt-thumb")
	if thumb == nil || !strings.Contains(f.Text(), uistate.T("transactions.receiptAttached", 1)) {
		t.Fatal(f.Text())
	}
	thumb.Click()
	row.Attachments = append(row.Attachments, domain.AttachmentRef{ArtifactID: "another"})
	f.Rerender(ui.CreateElement(txnEditAttachments, txnEditAttachmentsProps{App: app, Txn: row}))
	count := 0
	for _, button := range f.AllByTag("button") {
		if button.Attr("data-testid") == "txn-receipt-thumb" {
			count++
		}
	}
	if count != 2 {
		t.Fatal(count)
	}
	row.Attachments = nil
	f.Rerender(ui.CreateElement(txnEditAttachments, txnEditAttachmentsProps{App: app, Txn: row}))
	if findByTestID(f, "button", "txn-receipt-thumb") != nil {
		t.Fatal("stale receipt")
	}
}

func TestCSEDITSplitsChild(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	props := txnEditSplitsProps{Txn: domain.Transaction{ID: "row", Amount: money.New(-100, "USD"), Splits: []domain.CategorySplit{{CategoryID: "food", MemberID: "m", Amount: money.New(-100, "USD")}}},
		Categories: []domain.Category{{ID: "food", Name: "Food"}}, Members: []domain.Member{{ID: "m", Name: "Cam"}}, OnSave: func(domain.Transaction) {}}
	f.Render(ui.CreateElement(txnEditSplits, props))
	if !strings.Contains(f.Text(), "Food") || !strings.Contains(f.Text(), "Cam") {
		t.Fatal(f.Text())
	}
	props.Open = true
	f.Rerender(ui.CreateElement(txnEditSplits, props))
	if findByTestID(f, "div", "split-editor") == nil {
		t.Fatal("split editor did not open")
	}
	props.Open = false
	props.Txn.Splits = nil
	f.Rerender(ui.CreateElement(txnEditSplits, props))
	if strings.Contains(f.Text(), "Food") {
		t.Fatal("stale breakdown")
	}
}

func TestCSEDITAccountChild(t *testing.T) {
	app := newR3EEditApp(t)
	f := render.New(t)
	defer f.Cleanup()
	got := ""
	props := txnEditAccountProps{App: app, Txn: domain.Transaction{AccountID: "acct-1", Amount: money.New(-100, "USD")}, Selected: "acct-1", OnSelect: func(v string) { got = v }}
	f.Render(ui.CreateElement(txnEditAccount, props))
	findByTestID(f, "select", "txn-edit-account").Change("acct-2")
	if got != "acct-2" {
		t.Fatal(got)
	}
	props.Selected = got
	f.Rerender(ui.CreateElement(txnEditAccount, props))
	if findByTestID(f, "p", "txn-edit-account-note") == nil {
		t.Fatal("missing move preview")
	}
}

func TestCSEDITMovementChild(t *testing.T) {
	app := classifyTestApp(t)
	f := render.New(t)
	defer f.Cleanup()
	got := ""
	checked := false
	props := txnEditMovementProps{App: app, Txn: classifyTestTxn(), Selected: "card", OnSelect: func(v string) { got = v }, OnDebtPay: func(v bool) { checked = v }}
	f.Render(ui.CreateElement(txnEditMovement, props))
	findByTestID(f, "input", "txn-edit-classify-debt").Dispatch("onchange", render.Event{Checked: true})
	if !checked {
		t.Fatal("debt claim not reported")
	}
	findByTestID(f, "select", "txn-edit-classify").Change("sav")
	if got != "sav" {
		t.Fatal(got)
	}
	props.Selected = "sav"
	f.Rerender(ui.CreateElement(txnEditMovement, props))
	if findByTestID(f, "input", "txn-edit-classify-debt") != nil {
		t.Fatal("asset counterparty exposes debt claim")
	}
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnform"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

func csquickApp(t *testing.T) *appstate.App {
	t.Helper()
	a, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Store().Close() })
	for _, account := range []domain.Account{
		{ID: "cq-checking", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
		{ID: "cq-savings", Name: "Savings", Currency: "JPY", Type: domain.TypeSavings, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
	} {
		if err := a.PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []domain.Category{{ID: "cq-food", Name: "Food", Kind: domain.KindExpense}, {ID: "cq-pay", Name: "Pay", Kind: domain.KindIncome}} {
		if err := a.PutCategory(c); err != nil {
			t.Fatal(err)
		}
	}
	previous := appstate.Default
	appstate.Default = a
	t.Cleanup(func() { appstate.Default = previous; uistate.SetQuickAdd(false) })
	return a
}

func csquickSource(a *appstate.App) txnform.QuickSource {
	return txnform.QuickSource{Accounts: a.Accounts(), Categories: a.Categories(), Members: []domain.Member{{ID: "cq-owner", Name: "Owner", Color: "red"}},
		DefaultAccount: "cq-checking", Now: time.Date(2024, 2, 29, 23, 0, 0, 0, time.FixedZone("west", -8*3600))}
}

// Every new field component is mounted independently to prove it owns its bindings.
func TestCSQUICKFieldChildren(t *testing.T) {
	for _, tc := range []struct {
		name           string
		component      func(quickAddFieldsProps) uic.Node
		id, tag, input string
		change         bool
		check          func(*testing.T, txnform.QuickDraft)
	}{
		{"amount", quickAddAmount, "txn-add-amount", "input", "-12.50", false, func(t *testing.T, d txnform.QuickDraft) {
			if d.Amount != "-12.50" || d.Direction != "Expense" {
				t.Fatal(d)
			}
		}},
		{"payee", quickAddPayee, "txn-add-payee", "input", "Market", false, func(t *testing.T, d txnform.QuickDraft) {
			if d.Payee != "Market" {
				t.Fatal(d)
			}
		}},
		{"date", quickAddDate, "txn-add-date", "input", "2024-02-29", false, func(t *testing.T, d txnform.QuickDraft) {
			if d.Date != "2024-02-29" {
				t.Fatal(d)
			}
		}},
		{"account", quickAddAccount, "txn-add-account", "select", "cq-savings", true, func(t *testing.T, d txnform.QuickDraft) {
			if d.AccountID != "cq-savings" {
				t.Fatal(d)
			}
		}},
		{"category", quickAddCategory, "txn-add-category", "select", "cq-food", true, func(t *testing.T, d txnform.QuickDraft) {
			if d.CategoryID != "cq-food" {
				t.Fatal(d)
			}
		}},
		{"description", quickAddDescription, "txn-add-desc", "input", "Bank text", false, func(t *testing.T, d txnform.QuickDraft) {
			if d.Desc != "Bank text" {
				t.Fatal(d)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := csquickApp(t)
			source := csquickSource(a)
			var state uic.State[txnform.QuickDraft]
			f := render.New(t)
			defer f.Cleanup()
			f.Render(uic.CreateElement(func() uic.Node {
				state = uic.UseState(txnform.NewQuickDraft(""))
				return uic.CreateElement(tc.component, quickAddFieldsProps{App: a, Source: source, Draft: state.Get(), View: txnform.QuickPreview(state.Get(), source, true), Change: state.Update, SetCategoryOpen: func(bool) {}})
			}))
			node := findAppTestID(f, tc.tag, tc.id)
			if node == nil {
				t.Fatalf("missing %s", tc.id)
			}
			if tc.change {
				node.Change(tc.input)
			} else {
				node.Input(tc.input)
			}
			tc.check(t, state.Get())
			if tc.name == "amount" {
				if findAppTestID(f, "div", "txn-add-amount-note") == nil {
					t.Fatal("missing signed intent note")
				}
				f.ByID("txn-add-amount").Dispatch("onblur", render.Event{Value: "-12.50"})
				if state.Get().Amount != "12.50" {
					t.Fatal("blur did not normalize the display")
				}
			}
		})
	}
}

func TestCSQUICKDirectionChildAndTransfer(t *testing.T) {
	a := csquickApp(t)
	source := csquickSource(a)
	var state uic.State[txnform.QuickDraft]
	closed, transferred := 0, 0
	f := render.New(t)
	defer f.Cleanup()
	f.Render(uic.CreateElement(func() uic.Node {
		state = uic.UseState(txnform.NewQuickDraft(""))
		return uic.CreateElement(quickAddDirection, quickAddFieldsProps{Source: source, Draft: state.Get(), Change: state.Update, Close: func() { closed++ }, Transfer: func() { transferred++ }})
	}))
	findAppTestID(f, "button", "txn-add-direction-income").Click()
	if state.Get().Direction != "Income" {
		t.Fatal(state.Get())
	}
	findAppTestID(f, "button", "txn-add-direction-transfer").Click()
	if closed != 1 || transferred != 1 {
		t.Fatalf("close=%d transfer=%d", closed, transferred)
	}
}

func TestCSQUICKReviewAndDetailsChildren(t *testing.T) {
	for _, component := range []struct {
		name   string
		render func(quickAddFieldsProps) uic.Node
	}{{"review", quickAddReview}, {"details", quickAddDetails}} {
		t.Run(component.name, func(t *testing.T) {
			a := csquickApp(t)
			source := csquickSource(a)
			var state uic.State[txnform.QuickDraft]
			f := render.New(t)
			defer f.Cleanup()
			f.Render(uic.CreateElement(func() uic.Node {
				state = uic.UseState(txnform.NewQuickDraft(""))
				return Body(ID("csquick-details-body"), uic.CreateElement(component.render, quickAddFieldsProps{Source: source, Draft: state.Get(), View: txnform.QuickPreview(state.Get(), source, false), Change: state.Update}))
			}))
			if component.name == "review" {
				findAppTestID(f, "input", "txn-add-needs-review").Dispatch("onchange", render.Event{Checked: true})
				if !state.Get().NeedsReview {
					t.Fatal("checkbox did not update")
				}
				return
			}
			findAppTestID(f, "input", "txn-add-tags").Input("food, home")
			findAppTestID(f, "input", "txn-add-note").Input("note")
			findAppTestID(f, "input", "txn-add-cleared").Dispatch("onchange", render.Event{Checked: true})
			findAppTestID(f, "input", "txn-add-exclude").Dispatch("onchange", render.Event{Checked: true})
			d := state.Get()
			if d.Tags != "food, home" || d.Note != "note" || !d.Cleared || !d.Exclude {
				t.Fatal(d)
			}
			f.ByID("txn-add-who").Click()
			owner := f.ByRole("option", "Owner")
			if owner == nil {
				t.Fatal("owner picker missing")
			}
			owner.Click()
			if !reflect.DeepEqual(state.Get().Owners, []string{"cq-owner"}) || !state.Get().OwnersTouched {
				t.Fatal("owner binding did not update")
			}
		})
	}
}

func TestCSQUICKSentenceChildAppliesWithoutClearingOtherFields(t *testing.T) {
	a := csquickApp(t)
	source := csquickSource(a)
	var state uic.State[txnform.QuickDraft]
	focus := ""
	f := render.New(t)
	defer f.Cleanup()
	f.Render(uic.CreateElement(func() uic.Node {
		d := txnform.NewQuickDraft("")
		d.Note = "keep note"
		state = uic.UseState(d)
		return uic.CreateElement(quickAddSentence, quickAddFieldsProps{Source: source, Draft: state.Get(), View: txnform.QuickPreview(state.Get(), source, false), Change: state.Update, Focus: func(s string) { focus = s }})
	}))
	f.ByID("sm15-input").Input("12.50 coffee")
	f.ByID("sm15-input").Dispatch("onkeydown", render.Event{Key: "Enter"})
	d := state.Get()
	if d.Amount != "12.50" || d.Payee == "" || d.Desc != d.Payee || d.Note != "keep note" || focus != "txn-add-save" {
		t.Fatalf("draft=%+v focus=%q", d, focus)
	}
}

func TestCSQUICKDatalistChildPreservesOrderAndUpdatesLength(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	for _, values := range [][]string{{"Newest", "Older"}, nil, {"Only"}} {
		f.Render(uic.CreateElement(quickAddPayeeDatalist, values))
		if f.ByID("qa-payees") == nil {
			t.Fatal("datalist missing")
		}
		var got []string
		for _, option := range f.AllByTag("option") {
			value, _ := option.Property("value").(string)
			got = append(got, value)
		}
		if !reflect.DeepEqual(got, values) {
			t.Fatalf("options=%v want %v", got, values)
		}
	}
}

func TestCSQUICKTemplateChildAppliesSavedTemplate(t *testing.T) {
	a := csquickApp(t)
	source := csquickSource(a)
	source.TemplateCount = 1
	template := domain.TxnTemplate{ID: "csquick-template", Name: "Coffee", AccountID: "cq-checking", AmountMinor: 1250, Currency: "USD", Direction: domain.DirectionExpense, Payee: "Cafe", CategoryID: "cq-food", Note: "Bank"}
	uistate.SaveTxnTemplate(template)
	t.Cleanup(func() { uistate.DeleteTxnTemplate(template.ID) })
	var state uic.State[txnform.QuickDraft]
	f := render.New(t)
	defer f.Cleanup()
	f.Render(uic.CreateElement(func() uic.Node {
		state = uic.UseState(txnform.NewQuickDraft(""))
		return uic.CreateElement(quickAddTemplates, quickAddFieldsProps{Source: source, Draft: state.Get(), View: txnform.QuickPreview(state.Get(), source, false), Change: state.Update})
	}))
	if findAppTestID(f, "button", "txn-tmpl-save") == nil {
		t.Fatal("save template action missing")
	}
	findAppTestID(f, "button", "txn-tmpl-chip-"+template.ID).Click()
	d := state.Get()
	if d.Amount != "12.50" || d.Payee != "Cafe" || d.Desc != "Bank" || d.CategoryID != "cq-food" {
		t.Fatal(d)
	}
}

func TestCSQUICKFooterChildBindingsAndPendingState(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	close, another, save := 0, 0, 0
	p := quickAddFooterProps{Close: func() { close++ }, Another: func() { another++ }, Save: func() { save++ }}
	f.Render(uic.CreateElement(quickAddFooter, p))
	findAppTestID(f, "button", "txn-add-cancel").Click()
	findAppTestID(f, "button", "txn-add-another").Click()
	f.ByID("txn-add-save").Click()
	if close != 1 || another != 1 || save != 1 {
		t.Fatalf("callbacks %d %d %d", close, another, save)
	}
	p.Saving = true
	f.Render(uic.CreateElement(quickAddFooter, p))
	if f.ByID("txn-add-save").Attr("aria-busy") != "true" || f.ByID("txn-add-save").Property("disabled") != true {
		t.Fatal("save pending attributes changed")
	}
}

func TestCSQUICKBodyChildrenRemainMountedAcrossChangingOptions(t *testing.T) {
	a := csquickApp(t)
	source := csquickSource(a)
	f := render.New(t)
	defer f.Cleanup()
	var state uic.State[txnform.QuickDraft]
	host := func() uic.Node {
		state = uic.UseState(txnform.NewQuickDraft(""))
		return uic.CreateElement(quickAddBody, quickAddFieldsProps{App: a, Source: source, Draft: state.Get(), View: txnform.QuickPreview(state.Get(), source, false), Change: state.Update,
			Focus: func(string) {}, SetCategoryOpen: func(bool) {}, Close: func() {}, Transfer: func() {}})
	}
	f.Render(uic.CreateElement(host))
	f.ByID("txn-add-payee").Input("Keep me")
	for _, count := range []int{0, 4, 1, 2} {
		source.Categories = nil
		for i := 0; i < count; i++ {
			source.Categories = append(source.Categories, domain.Category{ID: strings.Repeat("c", i+1), Name: "Category", Kind: domain.KindExpense})
		}
		f.Render(uic.CreateElement(host))
		if f.ByID("txn-add-payee").Attr("data-cf-value") != "Keep me" {
			t.Fatal("child state lost when option count changed")
		}
		f.ByID("txn-add-amount").Input("5")
		if state.Get().Amount != "5" {
			t.Fatal("handler moved as options changed")
		}
	}
	for _, signal := range f.BuildDiagnostics() {
		if strings.Contains(strings.ToLower(signal.Code), "hook") {
			t.Fatalf("hook diagnostic: %+v", signal)
		}
	}
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestSplitLineNewCategoryCreatesIntoOpeningLine(t *testing.T) {
	app := pickerTestApp(t)
	txn := domain.Transaction{
		ID: "split-cat-new", Amount: money.New(-10000, "USD"),
		Splits: []domain.CategorySplit{
			{CategoryID: "c-auto", Amount: money.New(-6000, "USD")},
			{CategoryID: "c-home", Amount: money.New(-4000, "USD")},
		},
	}
	var saved domain.Transaction
	f := render.New(t)
	pickerFixtureTarget(t, f)
	f.Render(ui.CreateElement(SplitEditor, splitEditorProps{
		Txn: txn, Categories: app.Categories(), OnSave: func(updated domain.Transaction) { saved = updated },
	}))
	byTestID(f, "select", "split-cat-1").Change(splitNewCategoryOption)
	f.Stabilize()
	if byTestID(f, "div", "catpick") == nil {
		t.Fatal("split line New category did not open CategoryPicker")
	}
	byTestID(f, "input", "catpick-search").Input("New split category")
	byTestID(f, "button", "catpick-create").Click()
	f.Stabilize()
	if byTestID(f, "div", "catpick") != nil {
		t.Fatal("CategoryPicker stayed open after creating a category")
	}
	if !hasCategoryNamed(app.Categories(), "New split category") {
		t.Fatal("the split line picker did not create the requested category")
	}
	byTestID(f, "button", "split-save").Click()
	createdID := categoryIDNamed(app.Categories(), "New split category")
	if len(saved.Splits) != 2 || saved.Splits[1].CategoryID != createdID || saved.Splits[0].CategoryID != "c-auto" {
		t.Fatalf("new category was not assigned to the line that opened the picker: %+v", saved.Splits)
	}
}

func TestCatSuggestNewCategorySelectsCreatedCategory(t *testing.T) {
	app := pickerTestApp(t)
	if err := app.PutAccount(domain.Account{
		ID: "cat-suggest-account", Name: "Checking", Currency: "USD", Type: domain.TypeChecking,
		Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	txn := domain.Transaction{
		ID: "cat-suggest-new", AccountID: "cat-suggest-account", Desc: "Unknown merchant",
		Date: time.Date(2026, time.October, 18, 0, 0, 0, 0, time.UTC), Amount: money.New(-1200, "USD"),
	}
	if err := app.PutTransaction(txn); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	f := render.New(t)
	pickerFixtureTarget(t, f)
	f.Render(ui.CreateElement(CatSuggestBody, struct{}{}))
	uistate.SetCatSuggest(txn.ID)
	f.Stabilize()
	byTestID(f, "button", "catsuggest-new-category").Click()
	f.Stabilize()
	if byTestID(f, "div", "catpick") == nil {
		t.Fatal("Smart suggestion New category did not open CategoryPicker")
	}
	byTestID(f, "input", "catpick-search").Input("Smart category")
	byTestID(f, "button", "catpick-create").Click()
	f.Stabilize()
	if byTestID(f, "div", "catpick") != nil {
		t.Fatal("CategoryPicker stayed open after creating a category")
	}
	if !hasCategoryNamed(app.Categories(), "Smart category") {
		t.Fatal("Smart suggestion picker did not create the requested category")
	}
	f.ByID(CatSuggestFormID).Dispatch("onsubmit", render.Event{})
	for _, updated := range app.Transactions() {
		if updated.ID == txn.ID {
			if updated.CategoryID != categoryIDNamed(app.Categories(), "Smart category") {
				t.Fatal("the created category was not selected in the Smart suggestion control")
			}
			return
		}
	}
	t.Fatal("Smart suggestion transaction was not persisted")
}

func TestRuleFormNewCategoryAndKindFilteredPaths(t *testing.T) {
	app := pickerTestApp(t)
	f := render.New(t)
	pickerFixtureTarget(t, f)
	f.Render(ui.CreateElement(RuleAddForm, RuleAddFormProps{}))
	byTestID(f, "button", "rule-add-new-category").Click()
	f.Stabilize()
	if byTestID(f, "div", "catpick") == nil {
		t.Fatal("rule form New category did not open CategoryPicker")
	}
	byTestID(f, "input", "catpick-search").Input("Rule category")
	byTestID(f, "button", "catpick-create").Click()
	f.Stabilize()
	if !hasCategoryNamed(app.Categories(), "Rule category") {
		t.Fatal("rule form picker did not create the requested category")
	}
	var match *render.QueryNode
	for _, input := range f.AllByTag("input") {
		if input.Attr("placeholder") == uistate.T("rules.matchPlaceholder") {
			match = input
			break
		}
	}
	if match == nil {
		t.Fatal("rule match field was not rendered")
	}
	match.Input("Market")
	f.ByID("rule-add-form").Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	foundRule := false
	for _, rule := range app.Rules() {
		if rule.Match == "Market" {
			foundRule = true
			if rule.SetCategoryID != categoryIDNamed(app.Categories(), "Rule category") {
				t.Fatal("created category was not selected in the rule category control")
			}
		}
	}
	if !foundRule {
		t.Fatal("rule form did not persist the category assignment")
	}
	for _, opt := range categorySelectOptions(app.Categories(), "") {
		if opt.Value == "c-salary" {
			t.Fatal("an income category appeared in the default expense category list")
		}
	}
	income := categorySelectOptions(app.Categories(), "c-salary")
	foundIncome := false
	for _, opt := range income {
		if opt.Value == "c-salary" {
			foundIncome = true
			if opt.Label != "Salary" {
				t.Fatalf("income category label = %q, want full-path label Salary", opt.Label)
			}
		}
	}
	if !foundIncome {
		t.Fatal("selecting an income category did not retain its kind-filtered option")
	}
}

func hasCategoryNamed(cats []domain.Category, name string) bool {
	for _, c := range cats {
		if c.Name == name {
			return true
		}
	}
	return false
}

func categoryIDNamed(cats []domain.Category, name string) string {
	for _, c := range cats {
		if c.Name == name {
			return c.ID
		}
	}
	return ""
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestR4SplitOpensWithoutErrorsAndShowsLiveRemainder(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(SplitEditor, splitEditorProps{
		Txn:          domain.Transaction{ID: "expense", CategoryID: "food", Amount: money.New(-6400, "USD")},
		Categories:   []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}, {ID: "home", Name: "Home", Kind: domain.KindExpense}},
		FooterFormID: SplitModalFormID,
	}))

	if strings.Contains(f.Text(), "Add at least two rows") || strings.Contains(f.Text(), "Finish or remove this split row") {
		t.Fatalf("validation errors appeared before the user acted: %q", f.Text())
	}
	if got := byTestID(f, "span", "split-remainder"); got == nil || got.Text() != "Left to assign: $64.00" {
		t.Fatalf("initial remainder = %v, want Left to assign: $64.00", got)
	}
	byTestID(f, "input", "split-amt-0").Input("32.00")
	byTestID(f, "select", "split-cat-1").Change("home")
	byTestID(f, "input", "split-amt-1").Input("32.00")
	f.Stabilize()
	if got := byTestID(f, "span", "split-remainder"); got == nil || got.Text() != "Balanced" {
		t.Fatalf("balanced remainder = %v, want Balanced", got)
	}
}

func TestR4SplitSaveValidatesAndLabelsColumns(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(SplitEditor, splitEditorProps{
		Txn:     domain.Transaction{ID: "expense", Amount: money.New(-6400, "USD")},
		Members: []domain.Member{{ID: "member-1", Name: "Cam"}},
	}))
	for id, want := range map[string]string{
		"split-label-category": "Category",
		"split-label-who":      "Who",
		"split-label-amount":   "Amount",
	} {
		if got := byTestID(f, "p", id); got == nil || got.Text() != want {
			t.Errorf("column label %q = %v, want %q", id, got, want)
		}
	}
	newCategoryOption := false
	for _, option := range f.AllByTag("option") {
		if option.Property("value") == splitNewCategoryOption && option.Text() == "New category…" {
			newCategoryOption = true
		}
	}
	if !newCategoryOption {
		t.Error("category select is missing the New category… option")
	}
	byTestID(f, "button", "split-save").Click()
	f.Stabilize()
	if got := f.Text(); !strings.Contains(got, "Add at least two categories") {
		t.Fatalf("saving an empty split did not validate: %q", got)
	}
}

func TestR4SplitCategoryOptionsMatchTransactionDirection(t *testing.T) {
	categories := []domain.Category{
		{ID: "expense", Name: "Groceries", Kind: domain.KindExpense},
		{ID: "income", Name: "Paycheck", Kind: domain.KindIncome},
	}
	for _, tc := range []struct {
		name, selected, excluded string
		amount                   int64
	}{
		{name: "expense and refund row", selected: "expense", excluded: "income", amount: -6400},
		{name: "income", selected: "income", excluded: "expense", amount: 6400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			f.Render(ui.CreateElement(SplitEditor, splitEditorProps{
				Txn: domain.Transaction{Amount: money.New(tc.amount, "USD")}, Categories: categories,
			}))
			selectNode := byTestID(f, "select", "split-cat-0")
			if selectNode == nil {
				t.Fatal("category selector did not render")
			}
			options := selectNode.Text()
			if !strings.Contains(options, map[string]string{"expense": "Groceries", "income": "Paycheck"}[tc.selected]) {
				t.Errorf("matching category missing from options: %q", options)
			}
			if strings.Contains(options, map[string]string{"expense": "Groceries", "income": "Paycheck"}[tc.excluded]) {
				t.Errorf("wrong-kind category included in options: %q", options)
			}
		})
	}
}

func TestR4SplitCellLabelIncludesCategoryCountAndAmounts(t *testing.T) {
	tx := domain.Transaction{Amount: money.New(-6400, "USD"), Splits: []domain.CategorySplit{
		{CategoryID: "food", Amount: money.New(-2400, "USD")},
		{CategoryID: "home", Amount: money.New(-4000, "USD")},
	}}
	label, tooltip := splitCellLabel(tx)
	wantTooltip := fmtMoney(money.New(-2400, "USD")) + ", " + fmtMoney(money.New(-4000, "USD"))
	if label != "2 categories" || tooltip != wantTooltip {
		t.Fatalf("split cell = (%q, %q), want compact count and both amounts", label, tooltip)
	}
}

func TestR4SplitSaveNoticeSaysSplitSaved(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(r4SplitNotice, struct{}{}))
	postTransactionSplitNotice(true)
	f.Stabilize()
	if got := byTestID(f, "div", "r4split-notice"); got == nil || got.Text() != "Split saved" {
		t.Fatalf("successful split notice = %v, want Split saved", got)
	}
}

func r4SplitNotice(_ struct{}) ui.Node {
	notice := uistate.UseNotice()
	return Div(Attr("data-testid", "r4split-notice"), notice.Get().Text)
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestEditUsesSharedFieldOrderAndBankText(t *testing.T) {
	props := transactionRowRegressionProps()
	props.Account = "Household checking"
	props.Txn.Desc = "Card purchase"
	props.Txn.Splits = []domain.CategorySplit{{CategoryID: "food", Amount: money.New(-500, "USD")}}
	props.Txn.Attachments = []domain.AttachmentRef{{ArtifactID: "receipt-1", Name: "receipt.jpg"}}
	props.Categories = []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}}
	props.OnSave = func(domain.Transaction, string, string, string, string, string, string, string) bool { return true }
	f := render.New(t)
	f.Render(ui.CreateElement(TransactionRow, props))
	findRowEditButton(f).Click()
	text := f.Text()
	ordered := []string{
		uistate.T("transactions.amountPlaceholder"), uistate.T("quickAdd.expense"),
		uistate.T("transactions.payeeLabel"), uistate.T("transactions.categoryLabel"),
		uistate.T("transactions.dateLabel"), uistate.T("quickAdd.account"),
		uistate.T("r1e.moreDetails"), uistate.T("transactions.descPlaceholder"),
		uistate.T("transactions.tagsLabel"), uistate.T("quickAdd.note"),
		uistate.T("quickAdd.cleared"), uistate.T("quickAdd.exclude"),
	}
	previous := -1
	for _, label := range ordered {
		position := strings.Index(text, label)
		if position <= previous {
			t.Fatalf("edit form does not place %q in the shared field order: %q", label, text)
		}
		previous = position
	}
	if !strings.Contains(text, uistate.T("r2e.bankText", "Card purchase")) {
		t.Fatalf("read-only bank text missing: %q", text)
	}
	noteFound := false
	for _, input := range f.AllByTag("input") {
		if input.Attr("data-testid") == "txn-edit-note" {
			noteFound = true
			break
		}
	}
	if !noteFound {
		t.Fatal("note field missing from More details")
	}
	splitCount, receiptCount := 0, 0
	for _, div := range f.AllByTag("div") {
		switch div.Attr("data-testid") {
		case "txn-edit-split-entry":
			splitCount++
		case "txn-edit-receipt-entry":
			receiptCount++
		}
	}
	if splitCount != 1 || receiptCount != 1 {
		t.Fatal("edit form should list existing split and receipt entries")
	}
}

func TestCategorySuggestionRespectsEditedDirection(t *testing.T) {
	categories := []domain.Category{
		{ID: "food", Name: "Dining", Kind: domain.KindExpense},
		{ID: "pay", Name: "Paycheck", Kind: domain.KindIncome},
	}
	cases := []struct {
		name       string
		amount     int64
		categoryID string
		want       bool
	}{
		{name: "expense suggestion on expense", amount: -100, categoryID: "food", want: true},
		{name: "income suggestion on income", amount: 100, categoryID: "pay", want: true},
		{name: "expense suggestion is hidden after changing to income", amount: 100, categoryID: "food", want: false},
		{name: "income suggestion is hidden after changing to expense", amount: -100, categoryID: "pay", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := categorySuggestionMatchesDirection(domain.Transaction{Amount: money.New(tc.amount, "USD")}, tc.categoryID, categories)
			if got != tc.want {
				t.Fatalf("categorySuggestionMatchesDirection() = %v, want %v", got, tc.want)
			}
		})
	}
}

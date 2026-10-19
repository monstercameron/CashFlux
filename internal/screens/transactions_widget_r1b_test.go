// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestR1BStatusOnlyCallsForReview(t *testing.T) {
	tests := []struct {
		name string
		row  txnFrameRowProps
		word string
		tone string
	}{
		{name: "queued expense", row: txnFrameRowProps{Queued: true}, word: "Needs review", tone: "txn-status-attention"},
		{name: "ordinary unreviewed expense", row: txnFrameRowProps{}, word: "", tone: ""},
		{name: "cleared expense", row: txnFrameRowProps{Cleared: true, Reviewed: true}, word: "", tone: ""},
		{name: "transfer", row: txnFrameRowProps{Queued: true, IsTransfer: true}, word: "", tone: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rowStatusWord(tt.row); got != tt.word {
				t.Errorf("rowStatusWord() = %q, want %q", got, tt.word)
			}
			if got := statusToneClass(tt.row); got != tt.tone {
				t.Errorf("statusToneClass() = %q, want %q", got, tt.tone)
			}
		})
	}
}

func TestR1BRowMenuLabelsOrderAndExcludeTone(t *testing.T) {
	props := txnFrameRowProps{
		ID: "txn-1", Desc: "Coffee shop", Date: "Oct 18, 2026", Amount: "$4.50",
		AutoCategory: true, OnConfirmCategory: func(string) {}, OnCreateRule: func(string) {}, OnSuggestCategory: func(string) {},
		OnOpenSplit: func(string) {}, OnOpenLink: func(string, string) {},
		OnToggleExclude: func(string) {}, OnDelete: func(string) {},
	}
	items := txnRowMenuItems(props)
	labels := make([]string, len(items))
	byTestID := make(map[string]int, len(items))
	for i, item := range items {
		labels[i] = item.Label
		byTestID[item.TestID] = i
	}
	if len(items) == 0 {
		t.Fatal("expected row menu items")
	}
	if got := items[byTestID["txn-confirm-category"]].Label; got != uistate.T("transactions.confirmCategory") {
		t.Errorf("confirm category action = %q", got)
	}
	if got := items[byTestID["txn-categorize-open"]].Label; got != uistate.T("r5w.suggestCategorySmart") {
		t.Errorf("Smart category action = %q", got)
	}
	if got := items[byTestID["txn-split-open"]].Label; got != uistate.T("r4k.split") {
		t.Errorf("split action = %q", got)
	}
	if got := items[byTestID["txn-link-open"]].Label; got != "Link to…" {
		t.Errorf("link action = %q", got)
	}
	if items[byTestID["txn-toggle-exclude"]].Danger {
		t.Error("reversible report exclusion is styled as destructive")
	}
	if !items[byTestID["txn-delete"]].Danger {
		t.Error("delete should remain visually destructive")
	}
	if byTestID["txn-confirm-category"] >= byTestID["txn-categorize-open"] || byTestID["txn-categorize-open"] >= byTestID["txn-split-open"] || byTestID["txn-split-open"] >= byTestID["txn-link-open"] || byTestID["txn-link-open"] >= byTestID["txn-toggle-exclude"] || byTestID["txn-toggle-exclude"] >= byTestID["txn-delete"] {
		t.Errorf("menu order is not confirm, suggest, split, link, report, delete: %v", labels)
	}
	name := txnRowActionName(uistate.T("transactions.rowActions"), props.Desc, props.Date, props.Amount)
	if !strings.Contains(name, props.Desc) || !strings.Contains(name, props.Date) || !strings.Contains(name, props.Amount) {
		t.Errorf("row action name lacks transaction context: %q", name)
	}
}

func TestR1BMismatchedRuleCategoryIsNotSuggested(t *testing.T) {
	expense := domain.Category{ID: "dining", Kind: domain.KindExpense}
	income := domain.Category{ID: "pay", Kind: domain.KindIncome}
	categories := []domain.Category{expense, income}
	tests := []struct {
		name     string
		amount   int64
		category string
		want     bool
	}{
		{name: "income cannot suggest expense", amount: 5000, category: expense.ID},
		{name: "income accepts income", amount: 5000, category: income.ID, want: true},
		{name: "expense cannot suggest income", amount: -5000, category: income.ID},
		{name: "expense accepts expense", amount: -5000, category: expense.ID, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txn := domain.Transaction{Amount: money.New(tt.amount, "USD")}
			if got := categorySuggestionMatchesDirection(txn, tt.category, categories); got != tt.want {
				t.Errorf("categorySuggestionMatchesDirection() = %v, want %v", got, tt.want)
			}
		})
	}
}

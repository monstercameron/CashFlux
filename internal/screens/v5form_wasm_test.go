// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSplitRemainderWarnsOnlyAfterSaveAttempt(t *testing.T) {
	if got := splitRemainderTone(false); got != "split-remainder-neutral" {
		t.Fatalf("untouched remainder tone = %q, want neutral", got)
	}
	if got := splitRemainderTone(true); got != "neg" {
		t.Fatalf("remainder tone after save attempt = %q, want neg", got)
	}
}

func TestSplitGridKeepsCategoryAndOwnerReadable(t *testing.T) {
	if got, want := splitColumns(true), "minmax(180px, 1fr) 160px 120px 32px"; got != want {
		t.Fatalf("split grid with owners = %q, want %q", got, want)
	}
	if got, want := splitColumns(false), "minmax(180px, 1fr) 120px 32px"; got != want {
		t.Fatalf("split grid without owners = %q, want %q", got, want)
	}
}

func TestTransactionEditCategoryUsesLedgerPath(t *testing.T) {
	categories := []domain.Category{
		{ID: "housing", Name: "Housing", Kind: domain.KindExpense},
		{ID: "hoa", Name: "HOA dues", ParentID: "housing", Kind: domain.KindExpense},
	}
	for _, option := range CategoryPickOptions(categories) {
		if option.Value == "hoa" {
			if option.Label != "Housing > HOA dues" {
				t.Fatalf("edit category label = %q, want the shared category path %q", option.Label, "Housing > HOA dues")
			}
			return
		}
	}
	t.Fatal("edit category options do not contain HOA dues")
}

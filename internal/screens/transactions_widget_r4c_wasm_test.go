// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func r4cRow(id string) txnFrameRowProps {
	return txnFrameRowProps{
		ID: id, Desc: "Sample transaction", Date: "Oct 17", Amount: "$20.00",
		Category: "Uncategorized", Account: "Joint Checking", Vis: uistate.TxnCols{Category: true, Account: true, User: true, Status: true},
		OnOpen: func(string) {}, OnToggleSelect: func(string, bool) {},
	}
}

func TestR4CReviewPillStaysBesideCategory(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	props := r4cRow("review")
	props.Queued, props.Unfiled = true, true
	f.Render(ui.CreateElement(txnFrameRow, &props))

	pill := byTestID(f, "span", "txn-review-pill")
	if pill == nil || pill.Text() != "Review" {
		t.Fatalf("review pill = %v, want visible Review label", pill)
	}
	if pill.Attr("title") != statusDetail(props) || pill.Attr("aria-label") != statusDetail(props) {
		t.Fatalf("review pill lost accessible name or tooltip: title=%q aria-label=%q", pill.Attr("title"), pill.Attr("aria-label"))
	}
	if byTestID(f, "td", "txn-status-cell") != nil {
		t.Fatal("row still rendered the dropped Status column cell")
	}
}

func TestR4CTransferReadsInCategoryAndHasNoBadge(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	props := r4cRow("transfer")
	props.IsTransfer = true
	props.TransferWith = "Mortgage"
	props.AmountMoney = money.New(-148000, "USD")
	f.Render(ui.CreateElement(txnFrameRow, &props))

	category := byTestID(f, "span", "txn-transfer-category")
	if category == nil || category.Text() != "Transfer → Mortgage" {
		t.Fatalf("transfer category = %v, want Transfer → Mortgage", category)
	}
	if category.Attr("title") != "Transfer with Mortgage" {
		t.Errorf("counterparty tooltip = %q", category.Attr("title"))
	}
	if byTestID(f, "span", "txn-transfer-badge") != nil || strings.Contains(strings.ToUpper(f.Text()), "⇄ TRANSFER") {
		t.Fatal("transfer badge remains in the description")
	}
}

func TestR4CWhoHeaderAndUnassignedCell(t *testing.T) {
	f := render.New(t)
	defer func() {
		if f != nil {
			f.Cleanup()
		}
	}()
	f.Render(txnWhoHeader())
	if f.Text() != "Who" || strings.Contains(f.Text(), "♙") {
		t.Fatalf("member header = %q, want Who", f.Text())
	}

	f.Cleanup()
	f = render.New(t)
	props := r4cRow("unassigned")
	f.Render(ui.CreateElement(txnFrameRow, &props))
	var cell *render.QueryNode
	for _, candidate := range f.AllByTag("td") {
		if strings.Contains(candidate.Attr("class"), "td-user") {
			cell = candidate
			break
		}
	}
	if cell == nil {
		t.Fatal("Who cell was not rendered")
	}
	if cell.Attr("aria-label") != "No one assigned" || strings.TrimSpace(cell.Text()) != "" {
		t.Fatalf("unassigned member cell label/text = %q/%q", cell.Attr("aria-label"), cell.Text())
	}

	f.Cleanup()
	f = render.New(t)
	props = r4cRow("assigned")
	props.Member, props.MemberColor = "Jordan Lee", "#7357C8"
	f.Render(ui.CreateElement(txnFrameRow, &props))
	avatars := f.AllByTag("span")
	found := false
	for _, avatar := range avatars {
		if strings.Contains(avatar.Attr("class"), "txn-member-avatar") {
			found = avatar.Text() == "JL" && strings.Contains(avatar.Attr("style"), "#7357C8")
		}
	}
	if !found {
		var rendered []string
		for _, avatar := range avatars {
			if strings.Contains(avatar.Attr("class"), "txn-member-avatar") {
				rendered = append(rendered, avatar.Text()+" style="+avatar.Attr("style"))
			}
		}
		t.Fatalf("assigned member did not render colored JL avatar: %v (row %q)", rendered, f.Text())
	}
}

func TestR4CLedgerDateDropsYearOnlyForCurrentYearPeriod(t *testing.T) {
	year := time.Now().Year()
	date := time.Date(year, time.October, 17, 0, 0, 0, 0, time.UTC)
	if got := ledgerDate(date, time.Date(year, 10, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), time.Date(year, 10, 31, 0, 0, 0, 0, time.UTC).Format("2006-01-02")); got != "Oct 17" {
		t.Errorf("current-year date = %q, want Oct 17", got)
	}
	if got := ledgerDate(date, "2025-10-01", "2025-10-31"); got != "Oct 17, 2026" {
		t.Errorf("other-year period date = %q, want Oct 17, 2026", got)
	}
	if got := ledgerDate(date, "", ""); got != "Oct 17, 2026" {
		t.Errorf("all-dates date = %q, want Oct 17, 2026", got)
	}
}

func TestR4CTooltipsAreReservedForLongNames(t *testing.T) {
	if txnAccountNeedsTooltip("Joint Checking") || !txnAccountNeedsTooltip("A very long household checking account") {
		t.Fatal("account title tooltip threshold does not distinguish fitting and long names")
	}
	if txnCategoryNeedsTooltip("Dining") || !txnCategoryNeedsTooltip("Household repairs and improvements") {
		t.Fatal("category title tooltip threshold does not distinguish fitting and long names")
	}
}

func TestR10WUncategorizedRemainsBesideSuggestionAndReview(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	props := r4cRow("suggested-review")
	props.Category = "Uncategorized"
	props.SuggestedCategory = "Dining"
	props.Unfiled = true
	props.Queued = true
	props.ShowSuggestions = true
	f.Render(ui.CreateElement(txnFrameRow, &props))

	if !strings.Contains(f.Text(), "Uncategorized") {
		t.Fatalf("uncategorized label is missing beside suggestion: %q", f.Text())
	}
	if !strings.Contains(f.Text(), "Review") {
		t.Fatalf("review pill missing from suggested row: %q", f.Text())
	}
	if strings.Contains(f.Text(), "·") {
		t.Fatalf("stray suggestion separator remains: %q", f.Text())
	}
	var cell *render.QueryNode
	for _, candidate := range f.AllByTag("td") {
		if strings.Contains(candidate.Attr("class"), "td-cat") {
			cell = candidate
			break
		}
	}
	want := "Uncategorized, suggested Dining"
	if cell == nil || cell.Attr("aria-label") != want || cell.Attr("title") != want {
		t.Fatalf("category accessibility/tooltip = %v, want %q", cell, want)
	}
	if byTestID(f, "span", "txn-unconfirmed-badge") != nil {
		t.Fatal("unconfirmed state dot remains in the description cell")
	}
}

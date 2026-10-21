// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"os"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestUXT5DescriptionUsesOneHighestPriorityBadgeAndInfoSlot(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	p := txnFrameRowProps{
		ID: "uxt5", Desc: "Coffee Shop", Date: "Oct 19", Amount: "$8.00",
		Category: "Dining", Vis: uistate.TxnCols{Category: true},
		Duplicate: true, Queued: true, SpikeWhy: "About $8 more than usual",
		InfoText: "2 open follow-ups. 1 receipt. Has a note. Category added automatically — needs review",
		Receipts: 1, OnOpen: func(string) {}, OnToggleSelect: func(string, bool) {},
	}
	f.Render(Body(ui.CreateElement(txnFrameRow, &p)))
	if got := uxt5TestIDCount(f, "txn-flag-dupe"); got != 1 {
		t.Fatalf("duplicate badges = %d, want one highest-priority badge", got)
	}
	if uxt5TestIDCount(f, "txn-review-pill") != 0 || uxt5TestIDCount(f, "txn-flag-spike") != 0 {
		t.Fatal("lower-priority Review or unusual-spend badge also rendered")
	}
	info := byTestID(f, "button", "txn-row-info")
	if info == nil {
		t.Fatal("info slot is missing")
	}
	for _, phrase := range []string{"2 open follow-ups", "1 receipt", "Has a note", "needs review"} {
		if !strings.Contains(info.Attr("aria-label"), phrase) || !strings.Contains(info.Attr("title"), phrase) {
			t.Errorf("info slot does not expose %q in both tooltip and accessible name", phrase)
		}
	}
}

func uxt5TestIDCount(f *render.Fixture, id string) int {
	count := 0
	for _, tag := range []string{"button", "span", "div"} {
		for _, node := range f.AllByTag(tag) {
			if node.Attr("data-testid") == id {
				count++
			}
		}
	}
	return count
}

func TestUXT5ReviewOutranksUnusualSpend(t *testing.T) {
	if got := txnDescriptionBadge(false, true, true); got != txnBadgeReview {
		t.Fatalf("badge priority = %v, want Review", got)
	}
	if got := txnDescriptionBadge(false, false, true); got != txnBadgeSpike {
		t.Fatalf("badge priority = %v, want unusual spend", got)
	}
}

func TestUXT5InfoSlotKeepsReceiptAndFollowUpActions(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	p := txnFrameRowProps{ID: "info", Desc: "Coffee Shop", Date: "Oct 19", Amount: "$8.00",
		InfoText: "1 receipt. 2 open follow-ups", Receipts: 1, FollowUpOpen: 2, FollowUpTotal: 2,
		Vis: uistate.TxnCols{Category: true}, OnOpen: func(string) {}, OnToggleSelect: func(string, bool) {}}
	f.Render(Body(ui.CreateElement(txnFrameRow, &p)))
	info := byTestID(f, "button", "txn-row-info")
	if info == nil || info.Attr("aria-haspopup") != "true" {
		t.Fatal("combined info slot does not announce its action popover")
	}
	info.Click()
	f.Stabilize()
	if got := len(f.BuildOverlaySurfaces()); got != 1 {
		t.Fatal("combined info slot does not offer both receipt and follow-up actions")
	}
	if f.ByRole("button", "View receipt") == nil || f.ByRole("button", "Open follow-ups") == nil {
		t.Fatal("combined info popover is missing a receipt or follow-up action")
	}
}

func TestUXT5SplitCellUsesSharedLabelAndAmountsTooltip(t *testing.T) {
	tx := domain.Transaction{Amount: money.New(-6400, "USD"), Splits: []domain.CategorySplit{
		{CategoryID: "food", Amount: money.New(-4000, "USD")},
		{CategoryID: "home", Amount: money.New(-2400, "USD")},
	}}
	label, tooltip := splitCellLabel(tx)
	if label != "2 categories" || !strings.Contains(tooltip, "$40.00") || !strings.Contains(tooltip, "$24.00") {
		t.Fatalf("split label/tooltip = %q / %q", label, tooltip)
	}
	f := render.New(t)
	defer f.Cleanup()
	p := txnFrameRowProps{ID: "split", Desc: "Household purchase", Date: "Oct 19", Amount: "$64.00",
		Category: label, CategoryTooltip: tooltip, HasSplits: true, Vis: uistate.TxnCols{Category: true},
		OnOpen: func(string) {}, OnToggleSelect: func(string, bool) {}}
	f.Render(ui.CreateElement(txnFrameRow, &p))
	for _, cell := range f.AllByTag("td") {
		if strings.Contains(cell.Attr("class"), "td-cat") {
			if cell.Text() != label || cell.Attr("title") != tooltip {
				t.Fatalf("split category cell = %q title %q", cell.Text(), cell.Attr("title"))
			}
			return
		}
	}
	t.Fatal("split category cell is missing")
}

func TestUXT5UpcomingDisclosureAnchorsPopoverToTrigger(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(txnUpcomingDisclosure(true, "1 bill still to come", ui.Handler{}, ui.Text("Scheduled bill")))
	trigger := byTestID(f, "button", "txn-upcoming-toggle")
	popover := byTestID(f, "div", "txn-upcoming-popover")
	if trigger == nil || popover == nil || trigger.Attr("aria-controls") != "txn-upcoming-popover" || popover.Attr("role") != "dialog" {
		t.Fatal("upcoming disclosure does not expose its anchored popover")
	}
}

func TestUXT5BulkUndoHasNoInlineBanner(t *testing.T) {
	widget, err := os.ReadFile("transactions_widget.go")
	if err != nil {
		t.Fatal(err)
	}
	tiles, err := os.ReadFile("transactions_tiles.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(widget), `txnNativeSpec("txn-undobar")`) || strings.Contains(string(tiles), "bulkUndoBanner") {
		t.Fatal("bulk undo still renders an inline banner above the ledger")
	}
}

func TestUXT5ChangedRowGetsFlashClass(t *testing.T) {
	source, err := os.ReadFile("transactions_widget.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), `Call("add", "txn-row-flash")`) || !strings.Contains(string(source), "150*time.Millisecond") {
		t.Fatal("changed transaction row does not add the 150 ms flash class")
	}
}

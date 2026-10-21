// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestTxnSearchChipUsesDirectSearchLabel(t *testing.T) {
	f := render.New(t)
	f.Render(Span(txnSearchChipLabel("coffee")))
	if got := f.Text(); got != "Search: coffee" {
		t.Fatalf("search chip = %q, want %q", got, "Search: coffee")
	}
}

func TestTxnDateChipUsesHouseholdFormatter(t *testing.T) {
	formatted := formatTxnChipDateWith("2026-10-01", func(date time.Time) string { return date.Format("01/02/2006") })
	if formatted != "10/01/2026" {
		t.Fatalf("date chip = %q, want household-style %q", formatted, "10/01/2026")
	}
}

func TestTxnBulkBarActionLabelsUseCurrentTerms(t *testing.T) {
	f := render.New(t)
	f.Render(Div(
		txnBulkActionGroup(uistate.T("transactions.categoryToApply"), Button(uistate.T("r1c.category"))),
		txnBulkActionGroup(uistate.T("bulk2.owners"), Button(uistate.T("bulk2.apply"))),
		txnBulkActionGroup("Primary actions", Button(uistate.T("r1c.markReviewed"))),
		txnBulkActionGroup(uistate.T("r1c.moreActions"), Button(uistate.T("txp3.bulkMarkCleared"))),
		Button(Attr("data-testid", "bulk-delete"), uistate.T("txp3.bulkDeleteSelected")),
	))
	labels := []string{"Category", "Owners", "Apply", "Mark reviewed", uistate.T("r1c.moreActions"), "Mark as cleared", "Delete selected"}
	text := f.Text()
	for _, label := range labels {
		if !strings.Contains(text, label) {
			t.Errorf("bulk bar text %q does not contain %q", text, label)
		}
	}
}

func TestTxnToolbarDestinationsAreRouterLinks(t *testing.T) {
	tests := []struct{ id, href string }{
		{"txn-rules-btn", uistate.RoutePath("/rules")},
		{"txn-undobar-activity", uistate.RoutePath("/activity")},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			f := render.New(t)
			f.Render(toolbarIconLink(tt.id, icon.Workflow, "Destination", tt.href))
			links := f.AllByTag("a")
			if len(links) != 1 || links[0].Attr("data-testid") != tt.id || links[0].Attr("href") != tt.href {
				t.Fatalf("router link = %#v, want id %q href %q", links, tt.id, tt.href)
			}
		})
	}
}

func TestTxnToolbarSummaryComputesOncePerRevisionAndFilter(t *testing.T) {
	cache := &txnToolbarSummaryCache{}
	filter := uistate.TxFilter{Text: "coffee"}
	calls := 0
	compute := func() txnToolbarSummary { calls++; return txnToolbarSummary{review: calls} }
	cache.get(1, filter, compute)
	cache.get(1, filter, compute)
	if calls != 1 {
		t.Fatalf("summary computed %d times for one input, want 1", calls)
	}
	cache.get(1, uistate.TxFilter{Text: "tea"}, compute)
	if calls != 2 {
		t.Fatalf("filter change computed %d times total, want 2", calls)
	}
	cache.get(2, uistate.TxFilter{Text: "tea"}, compute)
	if calls != 3 {
		t.Fatalf("revision change computed %d times total, want 3", calls)
	}
}

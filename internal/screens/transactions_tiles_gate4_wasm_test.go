// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestGATE4StatusLegendTracksRenderedSettlementGlyphs(t *testing.T) {
	if txnHasStatusGlyph([]domain.Transaction{{Reviewed: false}, {Tags: []string{"needs-review"}}}) {
		t.Fatal("rows without cleared settlement state should not show the glyph legend")
	}
	if !txnHasStatusGlyph([]domain.Transaction{{Cleared: true}}) {
		t.Fatal("a cleared row renders a settlement glyph and should show the legend")
	}

	f := render.New(t)
	f.Render(txnStatusLegend())
	got := f.Text()
	if !strings.Contains(got, "Cleared") || !strings.Contains(got, "Reconciled") {
		t.Fatalf("settlement legend is missing an indicator actually rendered by rows: %q", got)
	}
	for _, node := range f.AllByTag("span") {
		if id := node.Attr("data-testid"); id == "txn-legend-review" || id == "txn-legend-unconfirmed" {
			t.Fatalf("legend still lists non-glyph row states: %s", id)
		}
	}
}

func TestGATE4ColumnsDoNotOfferRemovedStatusColumn(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(TxnColumnsBody, struct{}{}))
	for _, input := range f.AllByTag("input") {
		if input.Attr("data-testid") == "col-toggle-status" {
			t.Fatal("Columns still offers a Status toggle for the removed column")
		}
	}
}

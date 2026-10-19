// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestR10TRemovesStatusLegendAboveLedger(t *testing.T) {
	f := render.New(t)
	f.Render(txnToolbarFoot(P("October 2026 · 14 transactions")))
	if got := f.Text(); got != "October 2026 · 14 transactions" {
		t.Fatalf("toolbar foot text = %q, want only the count line", got)
	}
	for _, node := range f.AllByTag("div") {
		if node.Attr("data-testid") == "txn-status-legend" {
			t.Fatal("status legend still renders above the ledger")
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

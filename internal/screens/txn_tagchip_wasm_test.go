// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// C651: "Filter to #coffee" on a row left the search text, the chip row, the
// count and the ledger unchanged.
//
// The state half of that action is pinned in internal/txnfilter (DrillToTag).
// This is the click half — the part no pure test can reach: the chip has to hand
// the ledger the tag it is showing, unprefixed, since that is what Criteria.Tag
// matches against. A chip that renders "#coffee" and reports "#coffee" would
// filter to a tag nothing carries, which looks exactly like a no-op.
func TestTagChipReportsTheTagItShows(t *testing.T) {
	got, calls := "", 0
	f := render.New(t)
	f.Render(ui.CreateElement(txnTagChip, txnTagChipProps{
		Tag:     "coffee",
		OnClick: func(tag string) { got, calls = tag, calls+1 },
	}))
	var btn *render.QueryNode
	for _, n := range f.AllByTag("button") {
		if n.Attr("data-testid") == "txn-tag-coffee" {
			btn = n
		}
	}
	if btn == nil {
		t.Fatal("tag chip did not render")
	}
	btn.Click()
	if calls != 1 {
		t.Fatalf("OnClick fired %d times, want 1 — the chip promises a tag drill-down (C651)", calls)
	}
	if got != "coffee" {
		t.Errorf("OnClick received %q, want %q", got, "coffee")
	}
}

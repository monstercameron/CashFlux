// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestSpikeBadgeClickShowsEvidence(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ui.CreateElement(txnFlagChip, txnFlagChipProps{
		TxnID:    "spike-charge",
		SpikeWhy: "Babylist on Oct 4, 2026 — confidence high; typical amount $139; above typical by $1,711; category baseline from 33 charges",
	})))
	badge := byTestID(f, "button", "txn-flag-spike")
	if badge == nil || badge.Attr("title") != "" {
		t.Fatal("spike badge should be an interactive control without a native evidence tooltip")
	}
	badge.Click()
	f.Stabilize()
	if got := len(f.BuildOverlaySurfaces()); got != 1 {
		t.Fatalf("evidence surfaces = %d, want one", got)
	}
	for _, fact := range []string{"typical amount $139", "above typical by $1,711", "category baseline from 33 charges"} {
		if !strings.Contains(f.Text(), fact) {
			t.Errorf("evidence panel is missing %q: %q", fact, f.Text())
		}
	}
	if f.ByRole("button", "Open transaction") == nil {
		t.Fatal("evidence panel should retain the transaction action")
	}
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestReviewOpeningFallsBackToEverythingWhenViewHasNoQueuedItems(t *testing.T) {
	app := newReviewApp(t)
	putCharge(t, app, "outside-view", "SHELL 4471", 6)
	ledger := txnfilter.Criteria{Text: "no matching transaction"}

	idx := buildReviewIndexForOpen(app, reviewscope.ScopeCurrentView, false, ledger)
	if idx.Scope != reviewscope.ScopeEntireQueue {
		t.Fatalf("opening scope = %q, want everything when the current view is empty", idx.Scope)
	}
	if idx.Total != 1 || idx.QueueTotal != 1 || idx.ViewTotal != 0 {
		t.Fatalf("opening counts = total %d, queue %d, view %d; want 1, 1, 0",
			idx.Total, idx.QueueTotal, idx.ViewTotal)
	}
	if got := idx.Rows[0].Group.Items[0].ID; got != "outside-view" {
		t.Fatalf("first opening row = %q, want the queued charge outside the filter", got)
	}
}

func TestReviewOpeningKeepsAnExplicitEmptyCurrentView(t *testing.T) {
	app := newReviewApp(t)
	putCharge(t, app, "outside-view", "SHELL 4471", 6)
	ledger := txnfilter.Criteria{Text: "no matching transaction"}

	idx := buildReviewIndexForOpen(app, reviewscope.ScopeCurrentView, true, ledger)
	if idx.Scope != reviewscope.ScopeCurrentView || idx.Total != 0 {
		t.Fatalf("explicit current view = scope %q with %d items, want view with 0 items", idx.Scope, idx.Total)
	}
}

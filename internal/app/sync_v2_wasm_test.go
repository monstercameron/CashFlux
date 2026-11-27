// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/conflictcheckpoint"
	uic "github.com/monstercameron/GoWebComponents/v8/ui"
)

func TestSyncV2StatusLabelsAndFacesHandleLockedAndProtection(t *testing.T) {
	for _, state := range []string{"locked", "needs-data-protection"} {
		key, cls, ok := syncChipFace(state)
		if !ok || key == "" || cls != "sync-chip sync-warn" {
			t.Fatalf("syncChipFace(%q) = (%q, %q, %v), want warn chip", state, key, cls, ok)
		}
		title, hint := cloudFace(cloudFacts{SignedIn: true, Protected: true, State: state}, cloudDone)
		if title != "cloud1.attention" || hint != "cloud1.protectHint" {
			t.Fatalf("cloudFace(%q) = (%q, %q), want attention/protectHint", state, title, hint)
		}
	}
}

func TestCloudConflictReviewRendersV2DiffAndMergeAction(t *testing.T) {
	merged := 0
	cmp := conflictcheckpoint.Comparison{
		Available: true,
		Counts: map[conflictcheckpoint.Side]map[string]int{
			conflictcheckpoint.SideLocal:  {"transactions": 12},
			conflictcheckpoint.SideServer: {"transactions": 11},
		},
		Changes: []conflictcheckpoint.RecordChange{
			{Kind: conflictcheckpoint.ChangeChanged, Entity: "transaction", ID: "tx-1", BeforeLabel: "Coffee", AfterLabel: "Cafe"},
		},
	}
	f := cloudTestFixture(t, uic.CreateElement(cloudConflictReview, cloudConflictReviewProps{
		Open:        true,
		HasDiff:     true,
		Comparison:  cmp,
		OnClose:     func() {},
		OnMerge:     func() { merged++ },
		OnKeepLocal: func() {},
		OnUseServer: func() {},
	}))
	if cloudTestID(f, "div", "sync-conflict-diff") == nil {
		t.Fatal("expected sync-conflict-diff summary card")
	}
	btn := cloudTestID(f, "button", "sync-conflict-merge")
	if btn == nil {
		t.Fatal("expected sync-conflict-merge button")
	}
	btn.Click()
	f.Stabilize()
	if merged != 1 {
		t.Fatalf("OnMerge calls = %d, want 1", merged)
	}
}

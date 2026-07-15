// SPDX-License-Identifier: MIT

package artifactref

import (
	"sort"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestReferencedCountsTxnAttachments(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "t1", Attachments: []domain.AttachmentRef{{ArtifactID: "a1"}, {ArtifactID: "a2"}}},
		{ID: "t2"},
		{ID: "t3", Attachments: []domain.AttachmentRef{{ArtifactID: ""}}}, // blank ignored
	}
	refs := Referenced(txns, nil)
	if !refs["a1"] || !refs["a2"] {
		t.Errorf("txn attachments should be referenced: %+v", refs)
	}
	if len(refs) != 2 {
		t.Errorf("want 2 referenced, got %d: %+v", len(refs), refs)
	}
}

func TestReferencedCountsWidgetBindings(t *testing.T) {
	pages := []domain.CustomPage{
		{ID: "p1", Widgets: []domain.PageWidget{
			{ID: "w1", Binding: domain.WidgetBinding{ArtifactID: "img1"}},
			{ID: "w2"},
		}},
	}
	refs := Referenced(nil, pages)
	if !refs["img1"] || len(refs) != 1 {
		t.Errorf("widget binding should be referenced: %+v", refs)
	}
}

func TestOrphansExcludesReferenced(t *testing.T) {
	arts := []domain.Artifact{{ID: "a1"}, {ID: "a2"}, {ID: "a3"}}
	txns := []domain.Transaction{{ID: "t1", Attachments: []domain.AttachmentRef{{ArtifactID: "a2"}}}}
	got := Orphans(arts, txns, nil)
	sort.Strings(got)
	want := []string{"a1", "a3"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("orphans = %v, want %v (a2 is referenced by a txn receipt)", got, want)
	}
}

// SPDX-License-Identifier: MIT

package domain

import (
	"testing"
	"time"
)

func TestReassignAccountsOnInstitutionDelete(t *testing.T) {
	accts := []Account{
		{ID: "a1", InstitutionID: "chase"},
		{ID: "a2", InstitutionID: "fidelity"},
		{ID: "a3", InstitutionID: "chase"},
		{ID: "a4"},
	}
	changed := ReassignAccountsOnInstitutionDelete(accts, "chase")
	if len(changed) != 2 {
		t.Fatalf("want 2 changed accounts, got %d", len(changed))
	}
	for _, c := range changed {
		if c.InstitutionID != "" {
			t.Errorf("account %s should have InstitutionID cleared, got %q", c.ID, c.InstitutionID)
		}
	}
	if got := ReassignAccountsOnInstitutionDelete(accts, ""); got != nil {
		t.Errorf("blank delID should change nothing, got %v", got)
	}
}

func TestInstitutionByIDAndTrimmedName(t *testing.T) {
	insts := []Institution{
		{ID: "i1", Name: "  Chase  "},
		{ID: "", Name: "skip"},
		{ID: "i2", Name: ""},
	}
	m := InstitutionByID(insts)
	if len(m) != 2 {
		t.Fatalf("want 2 indexed, got %d", len(m))
	}
	if m["i1"].TrimmedName() != "Chase" {
		t.Errorf("TrimmedName = %q", m["i1"].TrimmedName())
	}
	if m["i2"].TrimmedName() != "Untitled institution" {
		t.Errorf("blank name fallback = %q", m["i2"].TrimmedName())
	}
}

func TestSortDocRefsByDate(t *testing.T) {
	d := func(day int) time.Time { return time.Date(2026, 3, day, 0, 0, 0, 0, time.UTC) }
	refs := []AccountDocRef{
		{ArtifactID: "old", AttachedAt: d(1)},
		{ArtifactID: "new", AttachedAt: d(10)},
		{ArtifactID: "mid", AttachedAt: d(5)},
	}
	got := SortDocRefsByDate(refs)
	want := []string{"new", "mid", "old"}
	for i, w := range want {
		if got[i].ArtifactID != w {
			t.Errorf("pos %d = %q, want %q", i, got[i].ArtifactID, w)
		}
	}
}

func TestAccountDocArtifactIDsAndLabelKey(t *testing.T) {
	accts := []Account{
		{ID: "a1", DocRefs: []AccountDocRef{{ArtifactID: "x", Label: "  Auto Policy "}, {ArtifactID: ""}}},
	}
	ids := AccountDocArtifactIDs(accts)
	if !ids["x"] || len(ids) != 1 {
		t.Errorf("ids = %v", ids)
	}
	if k := accts[0].DocRefs[0].LabelKey(); k != "auto policy" {
		t.Errorf("LabelKey = %q", k)
	}
	if l := (AccountDocRef{}).DisplayLabel("Statement.pdf"); l != "Statement.pdf" {
		t.Errorf("DisplayLabel fallback = %q", l)
	}
}

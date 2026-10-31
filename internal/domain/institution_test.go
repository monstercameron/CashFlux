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

func TestInstitutionDisplayNameSuppliesTheViewsPlaceholder(t *testing.T) {
	cases := []struct {
		name        string
		institution Institution
		placeholder string
		wantClean   string
		wantDisplay string
		wantTrimmed string
	}{
		{"named", Institution{Name: "  Chase "}, "Sans titre", "Chase", "Chase", "Chase"},
		{"blank", Institution{Name: "   "}, "Sans titre", "", "Sans titre", UntitledInstitution},
		{"empty", Institution{}, "", "", "", UntitledInstitution},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.institution.CleanName(); got != tc.wantClean {
				t.Errorf("CleanName = %q, want %q", got, tc.wantClean)
			}
			if got := tc.institution.DisplayName(tc.placeholder); got != tc.wantDisplay {
				t.Errorf("DisplayName = %q, want %q", got, tc.wantDisplay)
			}
			if got := tc.institution.TrimmedName(); got != tc.wantTrimmed {
				t.Errorf("TrimmedName = %q, want %q", got, tc.wantTrimmed)
			}
		})
	}
}

func TestReassignAccountsOnInstitutionDeleteOwnsItsReferenceFields(t *testing.T) {
	original := []Account{
		{
			ID: "a1", InstitutionID: "i1",
			DocRefs:         []AccountDocRef{{ArtifactID: "doc"}},
			OwnershipShares: map[string]int{"m1": 60, "m2": 40},
			Reconciliations: []Reconciliation{{DifferenceMinor: 5}},
			Custom:          map[string]any{"k": "v"},
		},
		{ID: "a2", InstitutionID: "other"},
	}
	changed := ReassignAccountsOnInstitutionDelete(original, "i1")
	if len(changed) != 1 || changed[0].ID != "a1" || changed[0].InstitutionID != "" {
		t.Fatalf("changed = %+v, want only a1 with its institution cleared", changed)
	}
	if original[0].InstitutionID != "i1" {
		t.Error("the input account was modified")
	}

	changed[0].DocRefs[0].ArtifactID = "mutated"
	changed[0].OwnershipShares["m1"] = 1
	changed[0].Reconciliations[0].DifferenceMinor = 99
	changed[0].Custom["k"] = "mutated"

	if original[0].DocRefs[0].ArtifactID != "doc" || original[0].OwnershipShares["m1"] != 60 ||
		original[0].Reconciliations[0].DifferenceMinor != 5 || original[0].Custom["k"] != "v" {
		t.Errorf("mutating the result leaked into the original: %+v", original[0])
	}

	if got := ReassignAccountsOnInstitutionDelete(original, ""); got != nil {
		t.Errorf("blank id changed %v accounts", got)
	}
}

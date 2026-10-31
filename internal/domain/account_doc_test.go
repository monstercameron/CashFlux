// SPDX-License-Identifier: MIT

package domain_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestAccountDocRefDisplayLabel(t *testing.T) {
	tests := []struct {
		name         string
		label        string
		artifactName string
		want         string
	}{
		{"explicit label wins over artifact name", "March 2026 statement", "scan_001.pdf", "March 2026 statement"},
		{"label is trimmed", "  Auto policy  ", "scan.pdf", "Auto policy"},
		{"blank label falls back to artifact name", "", "scan_001.pdf", "scan_001.pdf"},
		{"whitespace-only label falls back to artifact name", "   \t", "scan_001.pdf", "scan_001.pdf"},
		{"artifact name is trimmed", "", "  scan.pdf ", "scan.pdf"},
		{"no label and no artifact name gives the placeholder", "", "", "Document"},
		{"whitespace-only label and name give the placeholder", " ", "\n", "Document"},
		{"label is used even when the artifact name is blank", "Title", "", "Title"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := domain.AccountDocRef{Label: tt.label}
			if got := r.DisplayLabel(tt.artifactName); got != tt.want {
				t.Errorf("DisplayLabel(%q) = %q, want %q", tt.artifactName, got, tt.want)
			}
		})
	}
}

func TestAccountDocRefLabelKey(t *testing.T) {
	tests := []struct {
		label string
		want  string
	}{
		{"Auto Insurance", "auto insurance"},
		{"  Auto Insurance  ", "auto insurance"},
		{"AUTO INSURANCE", "auto insurance"},
		{"", ""},
		{"   ", ""},
	}
	for _, tt := range tests {
		r := domain.AccountDocRef{Label: tt.label}
		if got := r.LabelKey(); got != tt.want {
			t.Errorf("LabelKey() for %q = %q, want %q", tt.label, got, tt.want)
		}
	}
}

func TestSortDocRefsByDate(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, time.March, d, 0, 0, 0, 0, time.UTC) }
	ref := func(id, label string, at time.Time) domain.AccountDocRef {
		return domain.AccountDocRef{ArtifactID: id, Label: label, AttachedAt: at}
	}
	ids := func(refs []domain.AccountDocRef) []string {
		out := make([]string, len(refs))
		for i, r := range refs {
			out[i] = r.ArtifactID
		}
		return out
	}

	tests := []struct {
		name string
		in   []domain.AccountDocRef
		want []string
	}{
		{"nil input gives an empty result", nil, []string{}},
		{"empty input gives an empty result", []domain.AccountDocRef{}, []string{}},
		{"single ref is returned as is", []domain.AccountDocRef{ref("x", "A", day(1))}, []string{"x"}},
		{
			"newest attached comes first",
			[]domain.AccountDocRef{ref("old", "A", day(1)), ref("new", "A", day(20)), ref("mid", "A", day(10))},
			[]string{"new", "mid", "old"},
		},
		{
			"same date breaks ties by label ascending",
			[]domain.AccountDocRef{ref("z", "Zeta", day(5)), ref("a", "Alpha", day(5)), ref("m", "Mid", day(5))},
			[]string{"a", "m", "z"},
		},
		{
			"same date and label breaks ties by artifact id ascending",
			[]domain.AccountDocRef{ref("c", "Same", day(5)), ref("a", "Same", day(5)), ref("b", "Same", day(5))},
			[]string{"a", "b", "c"},
		},
		{
			"undated refs sort after dated ones",
			[]domain.AccountDocRef{ref("undated", "A", time.Time{}), ref("dated", "A", day(1))},
			[]string{"dated", "undated"},
		},
		{
			"date outranks label",
			[]domain.AccountDocRef{ref("a-old", "Alpha", day(1)), ref("z-new", "Zeta", day(9))},
			[]string{"z-new", "a-old"},
		},
		{
			"same instant in different time zones counts as a tie",
			[]domain.AccountDocRef{
				ref("b", "L", day(5).In(time.FixedZone("east", 5*3600))),
				ref("a", "L", day(5)),
			},
			[]string{"a", "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.SortDocRefsByDate(tt.in)
			if !reflect.DeepEqual(ids(got), tt.want) {
				t.Errorf("order = %v, want %v", ids(got), tt.want)
			}
		})
	}
}

func TestSortDocRefsByDateDoesNotMutateInput(t *testing.T) {
	in := []domain.AccountDocRef{
		{ArtifactID: "old", AttachedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ArtifactID: "new", AttachedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
	}
	out := domain.SortDocRefsByDate(in)
	if in[0].ArtifactID != "old" || in[1].ArtifactID != "new" {
		t.Errorf("input was reordered: %v", in)
	}
	out[0].ArtifactID = "changed"
	if in[1].ArtifactID != "new" {
		t.Error("result shares storage with the input")
	}
}

func TestAccountDocArtifactIDs(t *testing.T) {
	tests := []struct {
		name     string
		accounts []domain.Account
		want     map[string]bool
	}{
		{"nil accounts give an empty set", nil, map[string]bool{}},
		{"accounts without documents give an empty set", []domain.Account{{ID: "a1"}}, map[string]bool{}},
		{
			"blank artifact ids are ignored",
			[]domain.Account{{ID: "a1", DocRefs: []domain.AccountDocRef{{ArtifactID: ""}, {ArtifactID: "art1"}}}},
			map[string]bool{"art1": true},
		},
		{
			"ids from several accounts are unioned and de-duplicated",
			[]domain.Account{
				{ID: "a1", DocRefs: []domain.AccountDocRef{{ArtifactID: "art1"}, {ArtifactID: "art2"}}},
				{ID: "a2", DocRefs: []domain.AccountDocRef{{ArtifactID: "art2"}, {ArtifactID: "art3"}}},
			},
			map[string]bool{"art1": true, "art2": true, "art3": true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.AccountDocArtifactIDs(tt.accounts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AccountDocArtifactIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccountDocRefJSONOmitsZeroTimesAndRoundTrips(t *testing.T) {
	attached := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	expires := time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name         string
		ref          domain.AccountDocRef
		wantAttached bool
		wantExpires  bool
	}{
		{"both zero", domain.AccountDocRef{ArtifactID: "a"}, false, false},
		{"attached only", domain.AccountDocRef{ArtifactID: "a", AttachedAt: attached}, true, false},
		{"both set", domain.AccountDocRef{ArtifactID: "a", Label: "Policy", AttachedAt: attached, ExpiresAt: expires}, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.ref)
			if err != nil {
				t.Fatal(err)
			}
			text := string(raw)
			if got := strings.Contains(text, "attachedAt"); got != tc.wantAttached {
				t.Errorf("attachedAt present = %v, want %v in %s", got, tc.wantAttached, text)
			}
			if got := strings.Contains(text, "expiresAt"); got != tc.wantExpires {
				t.Errorf("expiresAt present = %v, want %v in %s", got, tc.wantExpires, text)
			}
			var back domain.AccountDocRef
			if err := json.Unmarshal(raw, &back); err != nil {
				t.Fatal(err)
			}
			if back.ArtifactID != tc.ref.ArtifactID || back.Label != tc.ref.Label || !back.AttachedAt.Equal(tc.ref.AttachedAt) || !back.ExpiresAt.Equal(tc.ref.ExpiresAt) {
				t.Errorf("round trip = %+v, want %+v", back, tc.ref)
			}
		})
	}
}

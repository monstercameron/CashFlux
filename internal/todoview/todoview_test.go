// SPDX-License-Identifier: MIT

package todoview

import (
	"strings"
	"testing"
)

func TestSaveOverwritesByFoldedName(t *testing.T) {
	var s Store
	if err := s.Save(View{Name: "Monday triage", Quick: "overdue"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Save(View{Name: "  monday TRIAGE ", Quick: "today"}); err != nil {
		t.Fatalf("Save overwrite: %v", err)
	}
	if len(s.Views) != 1 {
		t.Fatalf("got %d views, want 1 — a re-save must not create a near-duplicate", len(s.Views))
	}
	if s.Views[0].Quick != "today" {
		t.Errorf("Quick = %q, want the newer value", s.Views[0].Quick)
	}
	// The caller's capitalization wins, trimmed.
	if s.Views[0].Name != "monday TRIAGE" {
		t.Errorf("Name = %q", s.Views[0].Name)
	}
}

// An overwritten view must keep its slot: a picker that reshuffles every time
// you tweak a view is unusable.
func TestOverwriteKeepsPosition(t *testing.T) {
	var s Store
	for _, n := range []string{"A", "B", "C"} {
		if err := s.Save(View{Name: n}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Save(View{Name: "B", HideDone: true}); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(s.Names(), ",")
	if got != "A,B,C" {
		t.Errorf("order = %q, want A,B,C", got)
	}
	if !s.Views[1].HideDone {
		t.Error("the overwrite did not land on B")
	}
}

func TestSaveRejectsAnEmptyName(t *testing.T) {
	var s Store
	if err := s.Save(View{Name: "   "}); err != ErrNameRequired {
		t.Errorf("err = %v, want ErrNameRequired", err)
	}
	if len(s.Views) != 0 {
		t.Error("a rejected save still stored something")
	}
}

func TestSaveCapsTheList(t *testing.T) {
	var s Store
	for i := range MaxViews {
		if err := s.Save(View{Name: string(rune('a' + i))}); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}
	if err := s.Save(View{Name: "one too many"}); err != ErrTooMany {
		t.Errorf("err = %v, want ErrTooMany", err)
	}
	// …but overwriting an existing one is still allowed when full: the cap is on
	// how many views exist, not on editing the ones you have.
	if err := s.Save(View{Name: "a", HideDone: true}); err != nil {
		t.Errorf("overwrite while full: %v", err)
	}
}

func TestSaveTruncatesALongName(t *testing.T) {
	var s Store
	long := strings.Repeat("é", MaxNameLen+10)
	if err := s.Save(View{Name: long}); err != nil {
		t.Fatal(err)
	}
	// Runes, not bytes — truncating a multi-byte name by bytes produces mojibake.
	if n := len([]rune(s.Views[0].Name)); n != MaxNameLen {
		t.Errorf("name length = %d runes, want %d", n, MaxNameLen)
	}
}

func TestDeleteAndFind(t *testing.T) {
	var s Store
	_ = s.Save(View{Name: "Overdue", Quick: "overdue"})
	if v, ok := s.Find("OVERDUE"); !ok || v.Quick != "overdue" {
		t.Errorf("Find = %+v,%v", v, ok)
	}
	if !s.Delete(" overdue ") {
		t.Error("Delete missed a folded name")
	}
	if s.Delete("overdue") {
		t.Error("Delete reported success twice")
	}
	if len(s.Views) != 0 {
		t.Errorf("%d views survived", len(s.Views))
	}
}

// The picker highlights a view only while the screen still IS that view.
func TestActiveNameGoesQuietAsSoonAsAnythingChanges(t *testing.T) {
	var s Store
	saved := View{Name: "Monday triage", Display: "list", Quick: "overdue", Sort: "due", HideDone: true}
	_ = s.Save(saved)

	current := saved
	current.Name = "" // the live screen has no name
	if got := s.ActiveName(current); got != "Monday triage" {
		t.Errorf("ActiveName = %q, want the saved view", got)
	}
	current.HideDone = false
	if got := s.ActiveName(current); got != "" {
		t.Errorf("ActiveName = %q after a control changed, want empty", got)
	}
}

func TestRoundTrip(t *testing.T) {
	var s Store
	_ = s.Save(View{Name: "Mine", Display: "board", Quick: "today", Sort: "due",
		Priority: "high", Link: "bill", BoardGroup: "priority", HideDone: true, Search: "insurance"})
	got := Unmarshal(s.Marshal())
	if len(got.Views) != 1 || got.Views[0] != s.Views[0] {
		t.Errorf("round trip = %+v, want %+v", got.Views, s.Views)
	}
}

func TestUnmarshalIsForgivingAndBounded(t *testing.T) {
	if len(Unmarshal("").Views) != 0 {
		t.Error("empty input produced views")
	}
	if len(Unmarshal("{not json").Views) != 0 {
		t.Error("garbage input produced views")
	}
	// A nameless entry can never be picked, deleted or overwritten — it would be
	// a permanent dead row.
	if len(Unmarshal(`{"views":[{"name":"  "},{"name":"Real"}]}`).Views) != 1 {
		t.Error("a nameless entry survived")
	}
	// A duplicate name would give the picker two rows that fight over one slot.
	if len(Unmarshal(`{"views":[{"name":"A"},{"name":"a"}]}`).Views) != 1 {
		t.Error("a folded duplicate survived")
	}
	// A store written by a version with a higher cap must not make this one
	// unbounded.
	var big strings.Builder
	big.WriteString(`{"views":[`)
	for i := range MaxViews + 5 {
		if i > 0 {
			big.WriteString(",")
		}
		big.WriteString(`{"name":"v`)
		big.WriteString(string(rune('a' + i)))
		big.WriteString(`"}`)
	}
	big.WriteString("]}")
	if n := len(Unmarshal(big.String()).Views); n != MaxViews {
		t.Errorf("got %d views from an oversized store, want %d", n, MaxViews)
	}
}

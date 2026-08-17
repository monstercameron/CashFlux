// SPDX-License-Identifier: MIT

package catname

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// labelSet is a household with one unambiguous leaf ("Groceries"), a leaf name
// used under two different parents ("Gas"), and a parent whose own name is
// unique — the three cases every surface has to render.
func labelSet() []domain.Category {
	return []domain.Category{
		{ID: "c-auto", Name: "Auto", Kind: domain.KindExpense},
		{ID: "c-home", Name: "Home", Kind: domain.KindExpense},
		{ID: "c-groceries", Name: "Groceries", Kind: domain.KindExpense},
		{ID: "c-auto-gas", Name: "Gas", Kind: domain.KindExpense, ParentID: "c-auto"},
		{ID: "c-home-gas", Name: "Gas", Kind: domain.KindExpense, ParentID: "c-home"},
	}
}

func catByID(cats []domain.Category, id string) domain.Category {
	for _, c := range cats {
		if c.ID == id {
			return c
		}
	}
	return domain.Category{}
}

func TestLabelQualifiesOnlyWhenAmbiguous(t *testing.T) {
	cats := labelSet()
	tests := []struct {
		id   string
		want string
	}{
		// A unique leaf reads as the leaf. Qualifying it would make the ledger
		// shout a parent nobody needed.
		{"c-groceries", "Groceries"},
		{"c-auto", "Auto"},
		// A shared leaf must carry its parent, or the two are indistinguishable.
		{"c-auto-gas", "Auto > Gas"},
		{"c-home-gas", "Home > Gas"},
	}
	for _, tt := range tests {
		if got := Label(cats, catByID(cats, tt.id)); got != tt.want {
			t.Errorf("Label(%s) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

// TestLabelIsStableAcrossSurfaces is the property C619 actually asks for: the
// label depends only on the category set, never on the caller. Calling it twice
// for the same data must give the same answer — that is what stops Review and
// the ledger disagreeing.
func TestLabelIsStableAcrossSurfaces(t *testing.T) {
	cats := labelSet()
	for _, c := range cats {
		a := Label(cats, c)
		b := LabelByID(cats, c.ID)
		if a != b {
			t.Errorf("Label(%s) = %q but LabelByID = %q — the same category must read the same everywhere", c.ID, a, b)
		}
	}
}

// TestLabelAmbiguityFoldsCaseAndSpace: two names that differ only by case or
// padding are indistinguishable on screen, so both count as a collision.
//
// The assertion is that the two labels DIFFER, not that both gain a parent — a
// top-level category has no parent to add, so its qualified path is its own name.
// "Travel" beside "Work > travel" is already unambiguous, which is the property
// that matters; demanding a prefix it cannot have would be testing the
// implementation rather than the outcome.
func TestLabelAmbiguityFoldsCaseAndSpace(t *testing.T) {
	cats := []domain.Category{
		{ID: "c-a", Name: "Travel", Kind: domain.KindExpense},
		{ID: "c-p", Name: "Work", Kind: domain.KindExpense},
		{ID: "c-b", Name: " travel ", Kind: domain.KindExpense, ParentID: "c-p"},
	}
	if !Ambiguous(cats, catByID(cats, "c-a")) || !Ambiguous(cats, catByID(cats, "c-b")) {
		t.Fatal("a name that collides only by case/whitespace is still a collision — EqualNames folds both")
	}
	a, b := Label(cats, catByID(cats, "c-a")), Label(cats, catByID(cats, "c-b"))
	if a == b {
		t.Errorf("both categories label as %q — colliding names must end up distinguishable", a)
	}
	if b != "Work > travel" {
		t.Errorf("Label(c-b) = %q, want %q", b, "Work > travel")
	}
}

func TestAmbiguousIgnoresSelf(t *testing.T) {
	cats := []domain.Category{{ID: "c-only", Name: "Rent", Kind: domain.KindExpense}}
	if Ambiguous(cats, cats[0]) {
		t.Error("a lone category must not be ambiguous with itself")
	}
	if got := Label(cats, cats[0]); got != "Rent" {
		t.Errorf("Label = %q, want %q", got, "Rent")
	}
}

func TestLabelByIDHandlesMisses(t *testing.T) {
	cats := labelSet()
	if got := LabelByID(cats, ""); got != "" {
		t.Errorf("LabelByID(\"\") = %q, want empty", got)
	}
	if got := LabelByID(cats, "c-does-not-exist"); got != "" {
		t.Errorf("LabelByID(missing) = %q, want empty — a dangling id must not render as a name", got)
	}
}

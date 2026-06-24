// SPDX-License-Identifier: MIT

package pages

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"My Spending", "my-spending"},
		{"  Trimmed  ", "trimmed"},
		{"Cash/Flow 2026!", "cash-flow-2026"},
		{"--Already--Hyphened--", "already-hyphened"},
		{"   ", "page"},
		{"$$$", "page"},
		{"Über Café", "ber-caf"}, // non-ASCII dropped; remainder slugified
		{"A  B   C", "a-b-c"},
	}
	for _, c := range cases {
		if got := Slug(c.in); got != c.want {
			t.Errorf("Slug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUniqueSlug(t *testing.T) {
	existing := []domain.CustomPage{
		{ID: "1", Slug: "budget"},
		{ID: "2", Slug: "budget-2"},
		{ID: "3", Slug: "spending"},
	}
	if got := UniqueSlug("Budget", existing, ""); got != "budget-3" {
		t.Errorf("collision: got %q, want budget-3", got)
	}
	if got := UniqueSlug("Goals", existing, ""); got != "goals" {
		t.Errorf("free slug: got %q, want goals", got)
	}
	// Renaming page 1 back to its own slug should be allowed (exceptID skips it).
	if got := UniqueSlug("Budget", existing, "1"); got != "budget" {
		t.Errorf("own slug: got %q, want budget", got)
	}
}

func TestBySlugByID(t *testing.T) {
	ps := []domain.CustomPage{{ID: "a", Slug: "alpha"}, {ID: "b", Slug: "beta"}}
	if p, ok := BySlug(ps, "beta"); !ok || p.ID != "b" {
		t.Errorf("BySlug beta = %+v, %v", p, ok)
	}
	if _, ok := BySlug(ps, "nope"); ok {
		t.Error("BySlug nope should be false")
	}
	if p, ok := ByID(ps, "a"); !ok || p.Slug != "alpha" {
		t.Errorf("ByID a = %+v, %v", p, ok)
	}
	if _, ok := ByID(ps, "nope"); ok {
		t.Error("ByID nope should be false")
	}
}

func TestOrderedAndVisible(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ps := []domain.CustomPage{
		{ID: "c", Order: 2, CreatedAt: t0},
		{ID: "a", Order: 0, CreatedAt: t0},
		{ID: "b", Order: 1, CreatedAt: t0, Hidden: true},
	}
	gotOrder := ids(Ordered(ps))
	if !reflect.DeepEqual(gotOrder, []string{"a", "b", "c"}) {
		t.Errorf("Ordered = %v, want [a b c]", gotOrder)
	}
	gotVisible := ids(Visible(ps))
	if !reflect.DeepEqual(gotVisible, []string{"a", "c"}) {
		t.Errorf("Visible = %v, want [a c]", gotVisible)
	}
	// Tie on Order falls back to CreatedAt then ID.
	tie := []domain.CustomPage{
		{ID: "z", Order: 0, CreatedAt: t0.Add(time.Hour)},
		{ID: "y", Order: 0, CreatedAt: t0},
	}
	if got := ids(Ordered(tie)); !reflect.DeepEqual(got, []string{"y", "z"}) {
		t.Errorf("tie order = %v, want [y z]", got)
	}
}

func TestNextOrder(t *testing.T) {
	if got := NextOrder(nil); got != 0 {
		t.Errorf("NextOrder(nil) = %d, want 0", got)
	}
	ps := []domain.CustomPage{{Order: 0}, {Order: 5}, {Order: 3}}
	if got := NextOrder(ps); got != 6 {
		t.Errorf("NextOrder = %d, want 6", got)
	}
}

func TestReorder(t *testing.T) {
	ps := []domain.CustomPage{
		{ID: "a", Order: 0}, {ID: "b", Order: 1}, {ID: "c", Order: 2},
	}
	// Move c to the front.
	got := Reorder(ps, "c", 0)
	if order := ids(got); !reflect.DeepEqual(order, []string{"c", "a", "b"}) {
		t.Fatalf("Reorder c->0 = %v, want [c a b]", order)
	}
	for i, p := range got {
		if p.Order != i {
			t.Errorf("renumber: %s Order = %d, want %d", p.ID, p.Order, i)
		}
	}
	// Out-of-range index clamps; unknown id is a no-op move but still renumbers.
	if order := ids(Reorder(ps, "b", 99)); !reflect.DeepEqual(order, []string{"a", "c", "b"}) {
		t.Errorf("clamp = %v, want [a c b]", order)
	}
	if order := ids(Reorder(ps, "zzz", 0)); !reflect.DeepEqual(order, []string{"a", "b", "c"}) {
		t.Errorf("unknown id = %v, want [a b c]", order)
	}
	// Input not mutated.
	if ps[0].ID != "a" || ps[2].ID != "c" {
		t.Error("Reorder mutated its input")
	}
}

func TestValidate(t *testing.T) {
	if errs := Validate(domain.CustomPage{Name: "X", Slug: "x"}); errs != nil {
		t.Errorf("valid page reported errors: %v", errs)
	}
	if errs := Validate(domain.CustomPage{Name: "  ", Slug: ""}); len(errs) != 2 {
		t.Errorf("empty page: got %d errors, want 2: %v", len(errs), errs)
	}
}

func ids(ps []domain.CustomPage) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.ID
	}
	return out
}

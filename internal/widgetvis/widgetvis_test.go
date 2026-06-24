// SPDX-License-Identifier: MIT

package widgetvis

import (
	"reflect"
	"testing"
)

func TestToggleAndIsHidden(t *testing.T) {
	s := Set{}
	if s.IsHidden("trend") {
		t.Fatal("empty set should hide nothing")
	}
	s = s.Toggle("trend")
	if !s.IsHidden("trend") {
		t.Fatal("toggle should hide trend")
	}
	s = s.Toggle("trend")
	if s.IsHidden("trend") {
		t.Fatal("toggling again should show trend")
	}
}

func TestWithIsExplicit(t *testing.T) {
	s := Set{}.With("a", true).With("b", true).With("a", false)
	if s.IsHidden("a") || !s.IsHidden("b") {
		t.Fatalf("With produced wrong set: %v", s)
	}
}

func TestToggleIsImmutable(t *testing.T) {
	orig := Set{"x": true}
	next := orig.Toggle("y")
	if orig.IsHidden("y") {
		t.Fatal("Toggle mutated the original set")
	}
	if !next.IsHidden("x") || !next.IsHidden("y") {
		t.Fatalf("Toggle dropped state: %v", next)
	}
}

func TestFilterPreservesOrderDroppingHidden(t *testing.T) {
	s := Set{"b": true, "d": true}
	got := s.Filter([]string{"a", "b", "c", "d", "e"})
	want := []string{"a", "c", "e"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Filter = %v, want %v", got, want)
	}
}

func TestNormalizeDropsFalse(t *testing.T) {
	s := Set{"a": true, "b": false}
	n := s.Normalize()
	if _, ok := n["b"]; ok {
		t.Fatalf("Normalize kept a false entry: %v", n)
	}
	if !n["a"] {
		t.Fatalf("Normalize dropped a true entry: %v", n)
	}
}

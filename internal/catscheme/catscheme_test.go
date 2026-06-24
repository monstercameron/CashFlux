// SPDX-License-Identifier: MIT

package catscheme

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestDefaultNonEmptyAndKinds(t *testing.T) {
	s := Default()
	if len(s) == 0 {
		t.Fatal("Default scheme is empty")
	}
	var income, expense int
	for _, it := range s {
		switch it.Kind {
		case domain.KindIncome:
			income++
		case domain.KindExpense:
			expense++
		default:
			t.Errorf("category %q has invalid kind %q", it.Name, it.Kind)
		}
	}
	if income == 0 || expense == 0 {
		t.Errorf("want both income and expense categories, got income=%d expense=%d", income, expense)
	}
}

func TestDefaultNamesUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, it := range Default() {
		if it.Name == "" {
			t.Error("empty category name")
		}
		if seen[it.Name] {
			t.Errorf("duplicate category name %q", it.Name)
		}
		seen[it.Name] = true
	}
}

func TestDefaultParentsResolve(t *testing.T) {
	s := Default()
	top := map[string]domain.CategoryKind{}
	for _, it := range s {
		if it.Parent == "" {
			top[it.Name] = it.Kind
		}
	}
	for _, it := range s {
		if it.Parent == "" {
			continue
		}
		pk, ok := top[it.Parent]
		if !ok {
			t.Errorf("category %q references missing parent %q", it.Name, it.Parent)
			continue
		}
		if pk != it.Kind {
			t.Errorf("category %q (%s) under parent %q (%s): kind mismatch", it.Name, it.Kind, it.Parent, pk)
		}
	}
}

func TestDefaultColorsAreHex(t *testing.T) {
	for _, it := range Default() {
		c := it.Color
		if len(c) != 7 || c[0] != '#' {
			t.Errorf("category %q color %q is not #rrggbb", it.Name, c)
		}
	}
}

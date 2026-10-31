// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"reflect"
	"testing"
)

func TestAccountsFilterToggleTypeChoosesSeveralTypes(t *testing.T) {
	f := AccountsFilter{}.ToggleType("checking")
	f = f.ToggleType("savings")
	if want := []string{"checking", "savings"}; !reflect.DeepEqual(f.Types, want) {
		t.Fatalf("Types = %v, want %v", f.Types, want)
	}
	f = f.ToggleType("checking")
	if want := []string{"savings"}; !reflect.DeepEqual(f.Types, want) {
		t.Fatalf("after untoggling checking, Types = %v, want %v", f.Types, want)
	}
}

func TestAccountsFilterToggleTypeDoesNotShareBackingArray(t *testing.T) {
	base := AccountsFilter{}.ToggleType("checking")
	a := base.ToggleType("savings")
	b := base.ToggleType("credit")
	if reflect.DeepEqual(a.Types, b.Types) {
		t.Fatalf("branches share one backing list: %v and %v", a.Types, b.Types)
	}
}

func TestAccountsFilterMatchesAnyChosenType(t *testing.T) {
	f := AccountsFilter{Types: []string{"checking", "savings"}}
	cases := []struct {
		accType string
		want    bool
	}{
		{"checking", true},
		{"savings", true},
		{"credit", false},
	}
	for _, tc := range cases {
		if got := f.Matches("Main", tc.accType); got != tc.want {
			t.Errorf("Matches(%q) = %v, want %v", tc.accType, got, tc.want)
		}
	}
	if !(AccountsFilter{}).Matches("Main", "credit") {
		t.Error("an empty type set must match every type")
	}
}

func TestAccountsFilterWithoutRemovesOneOrAllTypes(t *testing.T) {
	f := AccountsFilter{Types: []string{"checking", "savings"}}
	if got := f.Without("type:checking").Types; !reflect.DeepEqual(got, []string{"savings"}) {
		t.Errorf("removing one type left %v", got)
	}
	if got := f.Without("type").Types; got != nil {
		t.Errorf("clearing all types left %v", got)
	}
	if len(f.Types) != 2 {
		t.Errorf("Without changed the original filter: %v", f.Types)
	}
}

func TestAccountsFilterHasNarrowingWithTypes(t *testing.T) {
	if (AccountsFilter{}).HasNarrowing() {
		t.Error("an empty filter reports narrowing")
	}
	if !(AccountsFilter{Types: []string{"checking"}}).HasNarrowing() {
		t.Error("a chosen type does not report narrowing")
	}
}

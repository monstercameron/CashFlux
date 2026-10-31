// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestInstitutionChoicesAreSortedUniqueNames(t *testing.T) {
	insts := map[string]domain.Institution{
		"a": {ID: "a", Name: "Chase"},
		"b": {ID: "b", Name: "  Comcast  "},
		"c": {ID: "c", Name: "Chase"},
		"d": {ID: "d", Name: ""},
	}
	got := institutionChoices(insts)
	want := []string{"Chase", "Comcast"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("institutionChoices = %v, want %v", got, want)
	}
}

func TestInstitutionChoicesEmptyDirectory(t *testing.T) {
	if got := institutionChoices(nil); len(got) != 0 {
		t.Fatalf("institutionChoices(nil) = %v, want none", got)
	}
}

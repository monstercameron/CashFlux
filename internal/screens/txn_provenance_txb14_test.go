// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestProvenanceKeyDistinguishesInteriorRows(t *testing.T) {
	views := [][]domain.Transaction{
		{{ID: "first"}, {ID: "automatic-a"}, {ID: "last"}},
		{{ID: "first"}, {ID: "automatic-b"}, {ID: "last"}},
	}
	if got, want := provenanceKey(views[0], 9), provenanceKey(views[1], 9); got == want {
		t.Fatalf("different visible row sets share provenance cache key %q", got)
	}
}

func TestProvenanceKeyIncludesOrderAndUnambiguousIDs(t *testing.T) {
	tests := []struct {
		name string
		a, b []domain.Transaction
	}{
		{name: "order", a: []domain.Transaction{{ID: "a"}, {ID: "b"}}, b: []domain.Transaction{{ID: "b"}, {ID: "a"}}},
		{name: "separator characters", a: []domain.Transaction{{ID: "a:b"}, {ID: "c"}}, b: []domain.Transaction{{ID: "a"}, {ID: "b:c"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if provenanceKey(tt.a, 3) == provenanceKey(tt.b, 3) {
				t.Fatal("different row sequences share a provenance key")
			}
		})
	}
}

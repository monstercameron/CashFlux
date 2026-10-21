// SPDX-License-Identifier: MIT

package splitview

import (
	"errors"
	"math"
	"testing"
)

var fixtureMembers = []Member{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}, {ID: "c", Name: "C"}}

func TestPrepareEqualAndTransfers(t *testing.T) {
	got, err := Prepare(Input{AmountText: "10.00", Decimals: 2, Members: fixtureMembers, Selected: []string{"a", "b", "c"}, PayerID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 1000 || !got.HasAmount || len(got.Shares) != 3 {
		t.Fatalf("prepared amount/shares = %d/%+v", got.Amount, got.Shares)
	}
	if got.ShareByID["a"] != 334 || got.ShareByID["b"] != 333 || got.ShareByID["c"] != 333 {
		t.Fatalf("shares = %v", got.ShareByID)
	}
	if len(got.Transfers) != 2 || got.Transfers[0].From != "b" || got.Transfers[0].To != "a" || got.Transfers[0].Amount != 333 {
		t.Fatalf("transfers = %+v", got.Transfers)
	}
}

func TestPrepareWeightsDefaultsZeroAndHamilton(t *testing.T) {
	got, err := Prepare(Input{AmountText: "0.07", Decimals: 2, Members: fixtureMembers, Selected: []string{"a", "b", "c"}, Weighted: true, Weights: map[string]string{"b": "0", "c": "2"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.ShareByID["a"] != 2 || got.ShareByID["b"] != 0 || got.ShareByID["c"] != 5 {
		t.Fatalf("shares = %v, want default weight 1, zero excluded, weight 2", got.ShareByID)
	}
}

func TestPrepareBlankAmountHasNoPreview(t *testing.T) {
	got, err := Prepare(Input{Members: fixtureMembers, Selected: []string{"a"}, Weighted: true, Weights: map[string]string{"a": "bad"}})
	if err != nil || got.HasAmount || len(got.Shares) != 0 {
		t.Fatalf("blank form = %+v, %v", got, err)
	}
}

func TestPrepareRejectsInvalidAmountWithoutPreview(t *testing.T) {
	for _, input := range []string{"wat", "92233720368547758.08", "-1.00", "0.00"} {
		t.Run(input, func(t *testing.T) {
			got, err := Prepare(Input{AmountText: input, Decimals: 2, Members: fixtureMembers, Selected: []string{"a"}})
			if err == nil || !errors.Is(err, ErrInvalidAmount) || len(got.Shares) != 0 {
				t.Fatalf("Prepare(%q) = %+v, %v; want invalid amount and empty preview", input, got, err)
			}
		})
	}
}

func TestPrepareRejectsInvalidWeightsWithoutPreview(t *testing.T) {
	for _, tc := range []struct {
		name    string
		weights map[string]string
		want    error
	}{
		{"malformed", map[string]string{"a": "1.5"}, ErrInvalidWeight},
		{"negative", map[string]string{"a": "-1"}, ErrInvalidWeight},
		{"all zero", map[string]string{"a": "0", "b": "0"}, ErrNoPositiveWeight},
		{"sum overflow", map[string]string{"a": "9223372036854775807", "b": "1"}, ErrWeightOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Prepare(Input{AmountText: "1.00", Decimals: 2, Members: fixtureMembers, Selected: []string{"a", "b"}, Weighted: true, Weights: tc.weights})
			if !errors.Is(err, tc.want) || len(got.Shares) != 0 {
				t.Fatalf("Prepare() = %+v, %v; want %v and empty preview", got, err, tc.want)
			}
		})
	}
}

func TestPrepareRejectsUnknownAndDuplicateMembers(t *testing.T) {
	for _, tc := range []struct {
		name     string
		selected []string
		payer    string
		want     error
	}{
		{"unknown selected", []string{"missing"}, "", ErrUnknownMember},
		{"unknown payer", []string{"a"}, "missing", ErrUnknownMember},
		{"duplicate selected", []string{"a", "a"}, "", ErrDuplicateMember},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Prepare(Input{AmountText: "1", Decimals: 0, Members: fixtureMembers, Selected: tc.selected, PayerID: tc.payer})
			if !errors.Is(err, tc.want) || len(got.Shares) != 0 {
				t.Fatalf("Prepare() = %+v, %v", got, err)
			}
		})
	}
}

func TestPrepareWeightedMaxAmountDoesNotOverflow(t *testing.T) {
	got, err := Prepare(Input{AmountText: "9223372036854775807", Decimals: 0, Members: fixtureMembers, Selected: []string{"a", "b"}, Weighted: true, Weights: map[string]string{"a": "2", "b": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.ShareByID["a"] <= 0 || got.ShareByID["b"] <= 0 || got.ShareByID["a"]+got.ShareByID["b"] != math.MaxInt64 {
		t.Fatalf("shares do not safely sum to max int64: %v", got.ShareByID)
	}
}

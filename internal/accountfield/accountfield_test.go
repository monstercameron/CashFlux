// SPDX-License-Identifier: MIT

package accountfield

import (
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestRateText(t *testing.T) {
	tests := []struct {
		name string
		acct domain.Account
		want string
	}{
		{"no rate recorded", domain.Account{}, ""},
		{"recorded zero", domain.Account{}.WithRateAPR(0), "0"},
		{"whole rate", domain.Account{}.WithRateAPR(5), "5"},
		{"fractional rate", domain.Account{}.WithRateAPR(19.99), "19.99"},
		{"NaN rate", domain.Account{}.WithRateAPR(math.NaN()), ""},
		{"positive infinity", domain.Account{}.WithRateAPR(math.Inf(1)), ""},
		{"negative infinity", domain.Account{}.WithRateAPR(math.Inf(-1)), ""},
		{"cleared rate", domain.Account{}.WithRateAPR(4).WithoutRateAPR(), ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := RateText(tc.acct); got != tc.want {
				t.Fatalf("RateText = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCloneShares(t *testing.T) {
	if got := CloneShares(nil); got != nil {
		t.Fatalf("CloneShares(nil) = %v, want nil", got)
	}
	if got := CloneShares(map[string]int{}); got != nil {
		t.Fatalf("CloneShares(empty) = %v, want nil", got)
	}
	src := map[string]int{"a": 60, "b": 40}
	got := CloneShares(src)
	if !reflect.DeepEqual(got, src) {
		t.Fatalf("CloneShares = %v, want %v", got, src)
	}
	got["a"] = 1
	if src["a"] != 60 {
		t.Fatal("CloneShares aliased the source map")
	}
}

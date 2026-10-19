// SPDX-License-Identifier: MIT

package domain

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestBCS2BoostProvenanceAndCheckedArithmetic(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	key := start.Format(time.DateOnly)
	for _, tc := range []struct {
		name                 string
		boost, topup, delta  int64
		move, fail           bool
		wantBoost, wantTopup int64
	}{
		{"legacy stays opaque", 500, 0, 100, true, false, 600, 0},
		{"new top-up beside legacy", 500, 0, 100, false, false, 600, 100},
		{"cover beside top-up", 100, 100, 200, true, false, 300, 100},
		{"net zero total retains provenance", 100, 100, -100, true, false, 0, 100},
		{"release", 100, 100, -100, false, false, 0, 0},
		{"total overflow", math.MaxInt64, 0, 1, true, true, 0, 0},
		{"provenance overflow", 0, math.MaxInt64, 1, false, true, 0, 0},
		{"total underflow", math.MinInt64, 0, -1, true, true, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := Budget{ID: "b", PeriodBoosts: map[string]int64{key: tc.boost}, PeriodTopups: map[string]int64{key: tc.topup}}
			before, _ := json.Marshal(b)
			var got Budget
			var err error
			if tc.move {
				got, err = b.WithPeriodMoveChecked(start, tc.delta)
			} else {
				got, err = b.WithPeriodBoostChecked(start, tc.delta)
			}
			if (err != nil) != tc.fail {
				t.Fatalf("err=%v want failure=%v", err, tc.fail)
			}
			after, _ := json.Marshal(b)
			if string(before) != string(after) {
				t.Fatal("input maps mutated")
			}
			if tc.fail {
				return
			}
			if got.PeriodBoost(start) != tc.wantBoost || got.PeriodTopups[key] != tc.wantTopup {
				t.Fatalf("got %+v", got)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var roundTrip Budget
			if err := json.Unmarshal(encoded, &roundTrip); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(roundTrip, got) {
				t.Fatal("provenance did not round-trip")
			}
		})
	}
}

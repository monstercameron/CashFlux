// SPDX-License-Identifier: MIT

package split

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestBalanced(t *testing.T) {
	for _, tc := range []struct {
		name   string
		target int64
		values []int64
		want   bool
	}{
		{"exact", 3, []int64{1, 2}, true},
		{"under", 3, []int64{1}, false},
		{"over", 3, []int64{1, 3}, false},
		{"overflow", math.MaxInt64, []int64{math.MaxInt64, 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Balanced(tc.target, tc.values); got != tc.want {
				t.Fatalf("Balanced(%d, %v) = %v, want %v", tc.target, tc.values, got, tc.want)
			}
		})
	}
}

func TestReconcile(t *testing.T) {
	for _, tc := range []struct {
		name    string
		total   int64
		values  []int64
		percent bool
		want    []int64
		wantErr error
	}{
		{"amounts", 100, []int64{25, 75}, false, []int64{25, 75}, nil},
		{"amount mismatch", 100, []int64{25, 74}, false, nil, ErrUnbalanced},
		{"percentages", 100, []int64{3333, 3333, 3334}, true, []int64{33, 33, 34}, nil},
		{"tiny percentage", 1, []int64{5000, 5000}, true, nil, ErrTooSmall},
		{"wrong percent total", 100, []int64{5000, 4000}, true, nil, ErrUnbalanced},
		{"zero amount", 100, []int64{100, 0}, false, nil, ErrNonPositive},
		{"negative total", -1, []int64{100}, false, nil, ErrUnbalanced},
		{"wide safe allocation", math.MaxInt64, []int64{5000, 5000}, true, []int64{4611686018427387904, 4611686018427387903}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := append([]int64(nil), tc.values...)
			got, err := Reconcile(tc.total, tc.values, tc.percent)
			if !errors.Is(err, tc.wantErr) || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Reconcile() = %v, %v; want %v, %v", got, err, tc.want, tc.wantErr)
			}
			if !reflect.DeepEqual(tc.values, input) {
				t.Fatal("Reconcile mutated its input")
			}
		})
	}
}

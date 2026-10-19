// SPDX-License-Identifier: MIT

package money

import (
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"math/rand/v2"
	"reflect"
	"slices"
	"sort"
	"testing"
)

func TestAllocate(t *testing.T) {
	tests := []struct {
		name          string
		total         int64
		weights, want []int64
		err           error
	}{
		{"equal ties", 10, []int64{1, 1, 1}, []int64{4, 3, 3}, nil},
		{"negative ties", -10, []int64{1, 1, 1}, []int64{-4, -3, -3}, nil},
		{"largest fraction first", 2, []int64{1, 3, 3}, []int64{0, 1, 1}, nil},
		{"zero weight", 5, []int64{0, 1, 1}, []int64{0, 3, 2}, nil},
		{"zero total", 0, []int64{1, 2}, []int64{0, 0}, nil},
		{"one cent", 1, []int64{1, 1, 1}, []int64{1, 0, 0}, nil},
		{"minus one cent", -1, []int64{1, 1, 1}, []int64{-1, 0, 0}, nil},
		{"review product overflow", 1000000000000, []int64{10000000, 10000000}, []int64{500000000000, 500000000000}, nil},
		{"review goal loop", 10000000000, []int64{10000000000, 10000000000}, []int64{5000000000, 5000000000}, nil},
		{"minimum", math.MinInt64, []int64{1}, []int64{math.MinInt64}, nil},
		{"minimum wide product", math.MinInt64, []int64{math.MaxInt64}, []int64{math.MinInt64}, nil},
		{"minimum split", math.MinInt64, []int64{1, 1}, []int64{-4611686018427387904, -4611686018427387904}, nil},
		{"maximum split", math.MaxInt64, []int64{1, 1}, []int64{4611686018427387904, 4611686018427387903}, nil},
		{"large weights", math.MaxInt64, []int64{math.MaxInt64 - 1, 1}, []int64{math.MaxInt64 - 1, 1}, nil},
		{"empty", 1, nil, nil, ErrInvalidWeights},
		{"empty zero", 0, nil, nil, ErrInvalidWeights},
		{"all zero", 5, []int64{0, 0}, nil, ErrInvalidWeights},
		{"all zero total", 0, []int64{0, 0}, nil, ErrInvalidWeights},
		{"negative weight", 5, []int64{-1, 2}, nil, ErrInvalidWeights},
		{"negative minimum weight", 5, []int64{math.MinInt64}, nil, ErrInvalidWeights},
		{"weight sum overflow", 5, []int64{math.MaxInt64, 1}, nil, ErrOverflow},
		{"weight sum overflow zero total", 0, []int64{math.MaxInt64, 1}, nil, ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := slices.Clone(tt.weights)
			got, err := Allocate(tt.total, tt.weights)
			if !reflect.DeepEqual(got, tt.want) || !errors.Is(err, tt.err) {
				t.Fatalf("got (%v,%v), want (%v,%v)", got, err, tt.want, tt.err)
			}
			if !slices.Equal(original, tt.weights) {
				t.Fatal("mutated weights")
			}
		})
	}
}

// referenceAllocate deliberately uses big integers and a comparison sort,
// independently of production's 128-bit products and stable radix ordering.
func referenceAllocate(total int64, weights []int64) ([]int64, bool) {
	sum := new(big.Int)
	for _, w := range weights {
		if w < 0 {
			return nil, false
		}
		sum.Add(sum, big.NewInt(w))
	}
	if sum.Sign() == 0 || !sum.IsInt64() {
		return nil, false
	}
	magnitude := new(big.Int).Abs(big.NewInt(total))
	remaining := new(big.Int).Set(magnitude)
	quotients := make([]*big.Int, len(weights))
	remainders := make([]*big.Int, len(weights))
	order := make([]int, len(weights))
	for i, w := range weights {
		quotients[i], remainders[i] = new(big.Int), new(big.Int)
		quotients[i].QuoRem(new(big.Int).Mul(magnitude, big.NewInt(w)), sum, remainders[i])
		remaining.Sub(remaining, quotients[i])
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return remainders[order[i]].Cmp(remainders[order[j]]) > 0 })
	for i := int64(0); i < remaining.Int64(); i++ {
		quotients[order[i]].Add(quotients[order[i]], big.NewInt(1))
	}
	result := make([]int64, len(weights))
	for i, q := range quotients {
		if total < 0 {
			q.Neg(q)
		}
		result[i] = q.Int64()
	}
	return result, true
}

func checkAllocationReference(t *testing.T, total int64, weights []int64) {
	t.Helper()
	original := slices.Clone(weights)
	got, err := Allocate(total, weights)
	want, valid := referenceAllocate(total, weights)
	if !slices.Equal(original, weights) {
		t.Fatal("mutated weights")
	}
	if !valid {
		if err == nil || got != nil {
			t.Fatalf("invalid weights %v returned (%v,%v)", weights, got, err)
		}
		return
	}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("Allocate(%d,%v)=(%v,%v), want %v", total, weights, got, err, want)
	}
	sum := new(big.Int)
	for _, v := range got {
		sum.Add(sum, big.NewInt(v))
		if (total < 0 && v > 0) || (total >= 0 && v < 0) {
			t.Fatal("wrong sign")
		}
	}
	if sum.Cmp(big.NewInt(total)) != 0 {
		t.Fatal("sum not conserved")
	}
	again, err := Allocate(total, weights)
	if err != nil || !slices.Equal(got, again) {
		t.Fatal("not deterministic")
	}
}

func TestAllocationBigReference(t *testing.T) {
	extrema := []int64{math.MinInt64, math.MinInt64 + 1, -9007199254740993, -1, 0, 1, 9007199254740993, math.MaxInt64 - 1, math.MaxInt64}
	for _, total := range extrema {
		for _, weights := range [][]int64{{1}, {0, 1, 0}, {1, 1, 1}, {math.MaxInt64 / 2, math.MaxInt64 / 2, 1}, {math.MaxInt64 - 2, 1, 1}} {
			checkAllocationReference(t, total, weights)
		}
	}
	random := rand.New(rand.NewPCG(31, 79))
	for range 5000 {
		weights := make([]int64, 1+random.IntN(32))
		for i := range weights {
			weights[i] = int64(random.Uint64() >> 6)
		}
		checkAllocationReference(t, int64(random.Uint64()), weights)
	}
	// Large recipient count plus extreme total protects against per-cent awards.
	weights := make([]int64, 100000)
	for i := range weights {
		weights[i] = 1
	}
	checkAllocationReference(t, math.MaxInt64, weights)
	checkAllocationReference(t, math.MinInt64, weights)
}

func FuzzAllocate(f *testing.F) {
	for _, total := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
		for _, weights := range [][]int64{nil, {0}, {1, 1, 1}, {10000000, 10000000}, {math.MaxInt64, 1}, {-1, 2}, {math.MaxInt64 - 2, 1, 1}} {
			data := make([]byte, len(weights)*8)
			for i, w := range weights {
				binary.LittleEndian.PutUint64(data[i*8:], uint64(w))
			}
			f.Add(total, data)
		}
	}
	f.Fuzz(func(t *testing.T, total int64, data []byte) {
		// Bound oracle work; production takes an int64 slice, not untrusted bytes.
		if len(data) > 2048 {
			return
		}
		weights := make([]int64, len(data)/8)
		for i := range weights {
			weights[i] = int64(binary.LittleEndian.Uint64(data[i*8:]))
		}
		checkAllocationReference(t, total, weights)
	})
}

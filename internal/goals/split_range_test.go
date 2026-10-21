// SPDX-License-Identifier: MIT

package goals

import (
	"math"
	"math/big"
	"math/rand/v2"
	"reflect"
	"sort"
	"testing"
)

func TestSplitEarmarkWideAmounts(t *testing.T) {
	for _, tc := range []struct {
		total int64
		cap   []int64
		want  []int64
	}{
		{10_000_000_000, []int64{10_000_000_000, 10_000_000_000}, []int64{5_000_000_000, 5_000_000_000}},
		{math.MaxInt64, []int64{math.MaxInt64, math.MaxInt64}, []int64{math.MaxInt64/2 + 1, math.MaxInt64 / 2}},
		{math.MaxInt64, []int64{math.MaxInt64, math.MaxInt64, math.MaxInt64}, []int64{math.MaxInt64/3 + 1, math.MaxInt64 / 3, math.MaxInt64 / 3}},
	} {
		for _, mode := range []SplitMode{SplitEven, SplitProportional} {
			got := SplitEarmark(tc.total, tc.cap, mode)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("%s total %d: %v want %v", mode, tc.total, got, tc.want)
			}
		}
	}
}

func TestSplitProportionalMatchesExactReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(71, 99))
	for range 300 {
		caps := make([]int64, 1+rng.IntN(20))
		for i := range caps {
			caps[i] = rng.Int64N(math.MaxInt64)
			if i%7 == 0 {
				caps[i] = 0
			}
		}
		total := rng.Int64N(math.MaxInt64)
		got := SplitEarmark(total, caps, SplitProportional)
		want := exactSplitReference(total, caps)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("total %d caps %v: %v want %v", total, caps, got, want)
		}
		var sum int64
		for i, v := range got {
			if v < 0 || v > caps[i] {
				t.Fatalf("invalid share %d: %d", i, v)
			}
			sum += v
		}
		if sum > total {
			t.Fatalf("distributed %d exceeds %d", sum, total)
		}
	}
}

func exactSplitReference(total int64, caps []int64) []int64 {
	out := make([]int64, len(caps))
	capacity := new(big.Int)
	for _, c := range caps {
		if c > 0 {
			capacity.Add(capacity, big.NewInt(c))
		}
	}
	if capacity.Sign() == 0 {
		return out
	}
	if capacity.IsInt64() && total > capacity.Int64() {
		total = capacity.Int64()
	}
	type remainder struct {
		index int
		value *big.Int
	}
	var remainders []remainder
	left := total
	for i, c := range caps {
		if c <= 0 {
			continue
		}
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(new(big.Int).Mul(big.NewInt(total), big.NewInt(c)), capacity, r)
		out[i] = q.Int64()
		left -= out[i]
		remainders = append(remainders, remainder{i, r})
	}
	sort.SliceStable(remainders, func(i, j int) bool { return remainders[i].value.Cmp(remainders[j].value) > 0 })
	for _, r := range remainders {
		if left == 0 {
			break
		}
		out[r.index]++
		left--
	}
	return out
}

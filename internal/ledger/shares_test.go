// SPDX-License-Identifier: MIT

package ledger

import (
	"testing"
)

func TestSplitByShares(t *testing.T) {
	type tc struct {
		name   string
		amount int64
		shares map[string]int
		want   map[string]int64
	}
	cases := []tc{
		{
			name:   "empty shares returns empty map",
			amount: 1000,
			shares: map[string]int{},
			want:   map[string]int64{},
		},
		{
			name:   "nil shares returns empty map",
			amount: 500,
			shares: nil,
			want:   map[string]int64{},
		},
		{
			name:   "single owner 100 pct exact",
			amount: 999,
			shares: map[string]int{"alice": 100},
			want:   map[string]int64{"alice": 999},
		},
		{
			name:   "60/40 split exact",
			amount: 1000,
			shares: map[string]int{"marcus": 60, "priya": 40},
			want:   map[string]int64{"marcus": 600, "priya": 400},
		},
		{
			name:   "indivisible 100 across 3 ways — remainder to highest",
			amount: 100,
			// 34+33+33 = 100: "anna" gets 34 since she has the highest share
			shares: map[string]int{"anna": 34, "bob": 33, "carl": 33},
			// anna: 100*34/100=34 rem=0; bob: 100*33/100=33 rem=0; carl: 33 rem=0
			// allocated=100, remaining=0 → no extra
			want: map[string]int64{"anna": 34, "bob": 33, "carl": 33},
		},
		{
			name:   "indivisible 10 across three 33/33/34 shares — Hamilton remainder",
			amount: 10,
			// shares sum to 100: 33+33+34
			shares: map[string]int{"alice": 33, "bob": 33, "carol": 34},
			// alice: 10*33/100=3 rem=30; bob: 3 rem=30; carol: 10*34/100=3 rem=40
			// allocated=9, remaining=1; carol has highest rem (40) → carol gets +1
			want: map[string]int64{"alice": 3, "bob": 3, "carol": 4},
		},
		{
			name:   "zero amount produces all zeros",
			amount: 0,
			shares: map[string]int{"x": 60, "y": 40},
			want:   map[string]int64{"x": 0, "y": 0},
		},
		{
			name:   "negative amount 60/40 exact",
			amount: -1000,
			shares: map[string]int{"marcus": 60, "priya": 40},
			want:   map[string]int64{"marcus": -600, "priya": -400},
		},
		{
			name:   "negative indivisible — magnitude treated correctly",
			amount: -10,
			shares: map[string]int{"alice": 33, "bob": 33, "carol": 34},
			// abs=10 → same split as positive then negate
			want: map[string]int64{"alice": -3, "bob": -3, "carol": -4},
		},
		{
			name:   "deterministic tie-break by member ID: a before b for equal remainder",
			amount: 1,
			// Both 50/50: each gets floor(1*50/100)=0, rem=50. Remaining=1.
			// Tie: "alice" < "zelda" lexically → "alice" gets the +1.
			shares: map[string]int{"alice": 50, "zelda": 50},
			want:   map[string]int64{"alice": 1, "zelda": 0},
		},
		{
			name:   "parts sum to amount for large indivisible case",
			amount: 7,
			shares: map[string]int{"a": 33, "b": 33, "c": 34},
			// a: 7*33/100=2 rem=31; b: 2 rem=31; c: 7*34/100=2 rem=38
			// allocated=6, remaining=1; c has highest rem (38) → c gets +1
			want: map[string]int64{"a": 2, "b": 2, "c": 3},
		},
		{
			name:   "two members equal remainder tie-break: earlier alpha wins",
			amount: 3,
			shares: map[string]int{"m1": 50, "m2": 50},
			// each: 3*50/100=1 rem=50. allocated=2, remaining=1.
			// tie: "m1" < "m2" → m1 gets +1
			want: map[string]int64{"m1": 2, "m2": 1},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SplitByShares(c.amount, c.shares)
			// Verify the map keys match.
			if len(got) != len(c.want) {
				t.Fatalf("len=%d want %d: got=%v want=%v", len(got), len(c.want), got, c.want)
			}
			for id, wv := range c.want {
				gv, ok := got[id]
				if !ok {
					t.Errorf("missing member %q in result", id)
					continue
				}
				if gv != wv {
					t.Errorf("member %q = %d, want %d", id, gv, wv)
				}
			}
			// Verify the parts sum to the original amount — only meaningful when
			// there are shares to distribute (empty/nil shares return an empty map).
			if len(c.shares) > 0 {
				var sum int64
				for _, v := range got {
					sum += v
				}
				if sum != c.amount {
					t.Errorf("parts sum = %d, want %d (parts=%v)", sum, c.amount, got)
				}
			}
		})
	}
}

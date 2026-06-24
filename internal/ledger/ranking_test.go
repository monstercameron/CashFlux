// SPDX-License-Identifier: MIT

package ledger

import "testing"

func TestRankSpending(t *testing.T) {
	// n+1 or fewer categories → all returned sorted, no "other".
	totals := map[string]int64{"food": 300, "rent": 1000, "fun": 100}
	top, other := RankSpending(totals, 2)
	if other != 0 {
		t.Errorf("other = %d, want 0 (3 categories, n=2 → n+1, keep all)", other)
	}
	if len(top) != 3 || top[0].CategoryID != "rent" || top[1].CategoryID != "food" || top[2].CategoryID != "fun" {
		t.Errorf("top = %+v, want rent,food,fun by amount desc", top)
	}

	// More than n+1 → top n plus a summed remainder.
	totals2 := map[string]int64{"rent": 1000, "food": 300, "fun": 100, "gas": 50, "misc": 25}
	top2, other2 := RankSpending(totals2, 2)
	if len(top2) != 2 || top2[0].CategoryID != "rent" || top2[1].CategoryID != "food" {
		t.Errorf("top2 = %+v, want [rent food]", top2)
	}
	if other2 != 100+50+25 {
		t.Errorf("other2 = %d, want 175 (fun+gas+misc)", other2)
	}

	// n <= 0 returns all sorted, no collapsing.
	allTop, allOther := RankSpending(totals2, 0)
	if allOther != 0 || len(allTop) != 5 {
		t.Errorf("n=0 → %d entries, other=%d; want 5 entries, other 0", len(allTop), allOther)
	}

	// Empty input.
	if tp, o := RankSpending(nil, 3); len(tp) != 0 || o != 0 {
		t.Errorf("empty → %+v, other=%d; want none", tp, o)
	}
}

func TestRankSpendingTieIsDeterministic(t *testing.T) {
	// Equal amounts must order by CategoryID, regardless of map iteration order.
	totals := map[string]int64{"c": 100, "a": 100, "b": 100}
	for i := 0; i < 20; i++ {
		top, _ := RankSpending(totals, 0)
		if len(top) != 3 || top[0].CategoryID != "a" || top[1].CategoryID != "b" || top[2].CategoryID != "c" {
			t.Fatalf("iteration %d: top = %+v, want a,b,c by id on ties", i, top)
		}
	}
}

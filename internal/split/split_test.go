package split

import (
	"sort"
	"testing"
)

func TestEqual(t *testing.T) {
	cases := []struct {
		total   int64
		members []string
		want    []int64
	}{
		{1000, []string{"a", "b"}, []int64{500, 500}},
		{1000, []string{"a", "b", "c"}, []int64{334, 333, 333}},
		{100, []string{"a", "b", "c"}, []int64{34, 33, 33}},
		{0, []string{"a", "b"}, []int64{0, 0}},
		{7, []string{"a", "b", "c"}, []int64{3, 2, 2}},
	}
	for _, tc := range cases {
		got := Equal(tc.total, tc.members)
		var sum int64
		for i, s := range got {
			if s.Amount != tc.want[i] {
				t.Errorf("Equal(%d, %v)[%d] = %d, want %d", tc.total, tc.members, i, s.Amount, tc.want[i])
			}
			sum += s.Amount
		}
		if sum != tc.total {
			t.Errorf("Equal(%d, %v) shares sum to %d, want %d", tc.total, tc.members, sum, tc.total)
		}
	}
	if Equal(100, nil) != nil {
		t.Error("Equal with no members should be nil")
	}
}

func TestNetBalances(t *testing.T) {
	// a paid 90, split evenly 3 ways (30 each).
	exp := []Expense{{
		PayerID: "a", Total: 90,
		Shares: []Share{{"a", 30}, {"b", 30}, {"c", 30}},
	}}
	net := NetBalances(exp)
	if net["a"] != 60 || net["b"] != -30 || net["c"] != -30 {
		t.Fatalf("net = %v, want a:60 b:-30 c:-30", net)
	}
	var sum int64
	for _, v := range net {
		sum += v
	}
	if sum != 0 {
		t.Errorf("net balances sum to %d, want 0", sum)
	}
}

func TestSettleUp(t *testing.T) {
	balances := map[string]int64{"a": 60, "b": -30, "c": -30}
	got := SettleUp(balances)

	// Apply the transfers and confirm everyone lands at zero.
	settled := map[string]int64{}
	for k, v := range balances {
		settled[k] = v
	}
	for _, tr := range got {
		if tr.Amount <= 0 {
			t.Errorf("non-positive transfer: %+v", tr)
		}
		settled[tr.From] += tr.Amount // debtor's negative balance moves toward zero
		settled[tr.To] -= tr.Amount   // creditor's positive balance moves toward zero
	}
	for id, v := range settled {
		if v != 0 {
			t.Errorf("%s not settled: %d remaining", id, v)
		}
	}
	// Two debtors, one creditor → two transfers, both into "a".
	if len(got) != 2 {
		t.Fatalf("got %d transfers, want 2: %+v", len(got), got)
	}
	froms := []string{got[0].From, got[1].From}
	sort.Strings(froms)
	if got[0].To != "a" || got[1].To != "a" || froms[0] != "b" || froms[1] != "c" {
		t.Errorf("transfers = %+v, want b→a and c→a", got)
	}
}

func TestSettleUpChain(t *testing.T) {
	// a owes 50, b is owed 50; single transfer a→b 50.
	got := SettleUp(map[string]int64{"a": -50, "b": 50})
	if len(got) != 1 || got[0].From != "a" || got[0].To != "b" || got[0].Amount != 50 {
		t.Errorf("got %+v, want a→b 50", got)
	}
}

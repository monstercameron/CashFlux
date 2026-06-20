package settle

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(c int64) money.Money { return money.New(c, "USD") }

func TestSplitEqually(t *testing.T) {
	tests := []struct {
		name    string
		total   int64
		members []string
		want    map[string]int64
	}{
		{
			name:    "even split",
			total:   9000,
			members: []string{"a", "b", "c"},
			want:    map[string]int64{"a": 3000, "b": 3000, "c": 3000},
		},
		{
			name:    "remainder cents go to the first members in order",
			total:   1000, // $10.00 / 3 = 334 + 333 + 333
			members: []string{"sam", "lee", "priya"},
			want:    map[string]int64{"lee": 334, "priya": 333, "sam": 333}, // sorted: lee, priya, sam
		},
		{
			name:    "single member takes the whole amount",
			total:   500,
			members: []string{"only"},
			want:    map[string]int64{"only": 500},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitEqually(usd(tc.total), tc.members)
			var sum int64
			for m, want := range tc.want {
				if got[m].Amount != want {
					t.Errorf("share[%s] = %d, want %d", m, got[m].Amount, want)
				}
			}
			for _, v := range got {
				sum += v.Amount
			}
			if sum != tc.total {
				t.Errorf("shares sum to %d, want %d (no lost/created cents)", sum, tc.total)
			}
		})
	}
}

func TestNet(t *testing.T) {
	// Priya pays $90 for a meal split three ways ($30 each).
	expenses := []Expense{
		{Payer: "priya", Shares: map[string]money.Money{"priya": usd(3000), "sam": usd(3000), "lee": usd(3000)}},
	}
	net := Net(expenses, nil, "USD")
	if net["priya"].Amount != 6000 {
		t.Errorf("priya net = %d, want 6000 (fronted sam + lee)", net["priya"].Amount)
	}
	if net["sam"].Amount != -3000 || net["lee"].Amount != -3000 {
		t.Errorf("sam/lee net = %d/%d, want -3000/-3000", net["sam"].Amount, net["lee"].Amount)
	}
	var sum int64
	for _, v := range net {
		sum += v.Amount
	}
	if sum != 0 {
		t.Errorf("net balances sum to %d, want 0", sum)
	}
}

func TestNetWithSettlement(t *testing.T) {
	expenses := []Expense{
		{Payer: "priya", Shares: map[string]money.Money{"priya": usd(3000), "sam": usd(3000), "lee": usd(3000)}},
	}
	// Sam already paid Priya $30 — Sam is square, Priya is owed only Lee's $30.
	settlements := []Settlement{{From: "sam", To: "priya", Amount: usd(3000)}}
	net := Net(expenses, settlements, "USD")
	if net["sam"].Amount != 0 {
		t.Errorf("sam net = %d, want 0 after settling", net["sam"].Amount)
	}
	if net["priya"].Amount != 3000 {
		t.Errorf("priya net = %d, want 3000 (only lee left)", net["priya"].Amount)
	}
}

func TestMinimize(t *testing.T) {
	// priya +6000, sam -3000, lee -3000 → two transfers of 3000 to priya.
	net := map[string]money.Money{"priya": usd(6000), "sam": usd(-3000), "lee": usd(-3000)}
	got := Minimize(net)
	if len(got) != 2 {
		t.Fatalf("got %d transfers, want 2: %+v", len(got), got)
	}
	var toPriya int64
	for _, tr := range got {
		if tr.To != "priya" {
			t.Errorf("transfer to %s, want priya", tr.To)
		}
		if tr.From != "sam" && tr.From != "lee" {
			t.Errorf("transfer from %s, want sam or lee", tr.From)
		}
		toPriya += tr.Amount.Amount
	}
	if toPriya != 6000 {
		t.Errorf("total paid to priya = %d, want 6000", toPriya)
	}
}

func TestMinimizeIsMinimalAcrossExpenses(t *testing.T) {
	// A cycle that simplifies: a owes b, b owes c, c owes a-ish. Three expenses,
	// each paid by a different member and split equally three ways.
	mk := func(payer string) Expense {
		return Expense{Payer: payer, Shares: map[string]money.Money{"a": usd(1000), "b": usd(1000), "c": usd(1000)}}
	}
	// Everyone pays one $30 expense → everyone is square → no transfers.
	got := Simplify([]Expense{mk("a"), mk("b"), mk("c")}, nil, "USD")
	if len(got) != 0 {
		t.Errorf("balanced group should need 0 transfers, got %+v", got)
	}
}

func TestMinimizeDeterministicAndZeroes(t *testing.T) {
	net := map[string]money.Money{"a": usd(-5000), "b": usd(2000), "c": usd(3000)}
	got := Minimize(net)
	// At most n-1 = 2 transfers; a (debtor 5000) pays c (3000) then b (2000) — c
	// first since it's the larger creditor.
	if len(got) != 2 {
		t.Fatalf("want 2 transfers, got %+v", got)
	}
	if got[0].To != "c" || got[0].Amount.Amount != 3000 {
		t.Errorf("first transfer = %+v, want a->c 3000", got[0])
	}
	if got[1].To != "b" || got[1].Amount.Amount != 2000 {
		t.Errorf("second transfer = %+v, want a->b 2000", got[1])
	}
	// Applying the transfers settles everyone (re-net is empty of nonzeros).
	settlements := make([]Settlement, len(got))
	for i, tr := range got {
		settlements[i] = Settlement{From: tr.From, To: tr.To, Amount: tr.Amount}
	}
	after := Net(nil, settlements, "USD")
	// Net of just settlements should be the inverse of the original net; combined
	// with the original it zeroes. Check the settlements move the right totals.
	if after["a"].Amount != 5000 {
		t.Errorf("settlements credit a by %d, want 5000", after["a"].Amount)
	}
}

func TestAlreadySettledIsEmpty(t *testing.T) {
	if got := Minimize(map[string]money.Money{"a": usd(0), "b": usd(0)}); len(got) != 0 {
		t.Errorf("all-zero net should yield no transfers, got %+v", got)
	}
}

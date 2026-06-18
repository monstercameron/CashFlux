package ledger

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestRecent(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "old", Date: mustDate("2026-01-01")},
		{ID: "new", Date: mustDate("2026-06-01")},
		{ID: "mid", Date: mustDate("2026-03-01")},
	}

	// Newest first, limited to n.
	top2 := Recent(txns, 2)
	if len(top2) != 2 || top2[0].ID != "new" || top2[1].ID != "mid" {
		t.Errorf("Recent(.,2) = %v, want [new mid]", ids(top2))
	}

	// n beyond the length returns all, still newest first.
	all := Recent(txns, 10)
	if len(all) != 3 || all[0].ID != "new" || all[2].ID != "old" {
		t.Errorf("Recent(.,10) = %v, want [new mid old]", ids(all))
	}

	// n <= 0 yields empty (and no panic on negative).
	if got := Recent(txns, 0); len(got) != 0 {
		t.Errorf("Recent(.,0) = %v, want empty", ids(got))
	}
	if got := Recent(txns, -1); len(got) != 0 {
		t.Errorf("Recent(.,-1) = %v, want empty", ids(got))
	}

	// The input is not mutated.
	if txns[0].ID != "old" {
		t.Error("Recent mutated the caller's slice order")
	}
}

func ids(txns []domain.Transaction) []string {
	out := make([]string, len(txns))
	for i, t := range txns {
		out[i] = t.ID
	}
	return out
}

// SPDX-License-Identifier: MIT

package similartxns

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/payeealias"
	"github.com/monstercameron/CashFlux/internal/rules"
)

func tx(id, payee, cat string) domain.Transaction {
	return domain.Transaction{ID: id, Payee: payee, CategoryID: cat}
}

func TestFindByAliasAndRulePack(t *testing.T) {
	resolver := payeealias.NewResolver([]domain.PayeeAlias{
		{ID: "a", RawPayee: "AMZN Mktp US*2K4RT0", Display: "Amazon"},
	})
	target := tx("t1", "AMZN Mktp US*2K4RT0", "shopping")
	all := []domain.Transaction{
		target,
		tx("t2", "AMAZON PRIME*11", "food"),     // rule pack → "Amazon Prime" — different key, NOT similar
		tx("t3", "amzn mktp us*2k4rt0", ""),     // learned alias → Amazon, uncategorized: candidate
		tx("t4", "AMZN Mktp US*ZZ", "shopping"), // rule pack → Amazon, already target category: skipped
		tx("t5", "AMZN Mktp US*QQ", "travel"),   // rule pack → Amazon, different category: candidate
		tx("t6", "STARBUCKS", "food"),           // unrelated
	}
	got := Find(target, all, "shopping", resolver, nil)
	if len(got) != 2 {
		t.Fatalf("want 2 candidates, got %d: %+v", len(got), ids(got))
	}
	// t3 (uncategorized) and t5 (different category).
	if got[0].Txn.ID != "t3" || got[0].AlreadyCategorized {
		t.Errorf("first candidate = %+v, want t3 uncategorized", got[0])
	}
	if got[1].Txn.ID != "t5" || !got[1].AlreadyCategorized {
		t.Errorf("second candidate = %+v, want t5 already-categorized", got[1])
	}
}

func TestFindRulesFallback(t *testing.T) {
	// Transactions with no payee, only a description; grouped by rule match.
	rs := []rules.Rule{{ID: "r1", Match: "uber", SetCategoryID: "transport"}}
	target := domain.Transaction{ID: "t1", Desc: "UBER TRIP 123", CategoryID: "transport"}
	all := []domain.Transaction{
		target,
		{ID: "t2", Desc: "UBER EATS ORDER", CategoryID: "food"}, // matches rule r1 → candidate
		{ID: "t3", Desc: "LYFT RIDE", CategoryID: "food"},       // no rule match
	}
	got := Find(target, all, "transport", nil, rs)
	if len(got) != 1 || got[0].Txn.ID != "t2" {
		t.Fatalf("rules fallback = %+v", ids(got))
	}
}

func TestFindNoKey(t *testing.T) {
	target := domain.Transaction{ID: "t1"} // no payee, no desc, no rules
	got := Find(target, []domain.Transaction{{ID: "t2"}}, "x", nil, nil)
	if got != nil {
		t.Fatalf("expected nil for keyless target, got %+v", got)
	}
}

func ids(cs []Candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Txn.ID
	}
	return out
}

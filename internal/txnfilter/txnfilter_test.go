package txnfilter

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func d(s string) time.Time {
	t, err := dateutil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

func sample() []domain.Transaction {
	return []domain.Transaction{
		{ID: "a", AccountID: "acc1", CategoryID: "food", MemberID: "m1", Desc: "Coffee shop", Amount: money.New(-450, "USD"), Date: d("2026-06-01"), Cleared: true, Tags: []string{"treat"}},
		{ID: "b", AccountID: "acc2", CategoryID: "rent", Desc: "Rent", Amount: money.New(-120000, "USD"), Date: d("2026-06-03")},
		{ID: "c", AccountID: "acc1", CategoryID: "pay", Desc: "Payday", Amount: money.New(250000, "USD"), Date: d("2026-06-02")},
	}
}

func ids(ts []domain.Transaction) string {
	s := ""
	for _, t := range ts {
		s += t.ID
	}
	return s
}

func TestApplyDefaultSortNewestFirst(t *testing.T) {
	got := Apply(sample(), Criteria{})
	if ids(got) != "bca" { // 06-03, 06-02, 06-01
		t.Errorf("default order = %q, want bca", ids(got))
	}
}

func TestApplyFilters(t *testing.T) {
	all := sample()
	if got := Apply(all, Criteria{Account: "acc1"}); ids(got) != "ca" {
		t.Errorf("account filter = %q, want ca", ids(got))
	}
	if got := Apply(all, Criteria{Category: "rent"}); ids(got) != "b" {
		t.Errorf("category filter = %q, want b", ids(got))
	}
	if got := Apply(all, Criteria{Member: "m1"}); ids(got) != "a" {
		t.Errorf("member filter = %q, want a", ids(got))
	}
	if got := Apply(all, Criteria{Text: "coffee"}); ids(got) != "a" {
		t.Errorf("text filter (desc) = %q, want a", ids(got))
	}
	if got := Apply(all, Criteria{Text: "treat"}); ids(got) != "a" {
		t.Errorf("text filter (tag) = %q, want a", ids(got))
	}
	if got := Apply(all, Criteria{Cleared: "yes"}); ids(got) != "a" {
		t.Errorf("cleared filter = %q, want a", ids(got))
	}
	if got := Apply(all, Criteria{Cleared: "no"}); ids(got) != "bc" {
		t.Errorf("not-cleared filter = %q, want bc", ids(got))
	}
}

func TestApplyDateRange(t *testing.T) {
	got := Apply(sample(), Criteria{From: "2026-06-02", To: "2026-06-03"})
	if ids(got) != "bc" {
		t.Errorf("date range = %q, want bc", ids(got))
	}
}

func TestApplySort(t *testing.T) {
	// Largest absolute amount first: rent 1200, pay 2500, coffee 4.5 → c,b,a.
	if got := Apply(sample(), Criteria{Sort: "amount"}); ids(got) != "cba" {
		t.Errorf("amount sort = %q, want cba", ids(got))
	}
	// Payee A–Z: Coffee shop, Payday, Rent → a,c,b.
	if got := Apply(sample(), Criteria{Sort: "payee"}); ids(got) != "acb" {
		t.Errorf("payee sort = %q, want acb", ids(got))
	}
}

func TestApplyDoesNotMutate(t *testing.T) {
	all := sample()
	_ = Apply(all, Criteria{Sort: "amount"})
	if all[0].ID != "a" {
		t.Error("Apply mutated the input slice order")
	}
}

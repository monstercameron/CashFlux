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

func TestApplySortByKeyAndDirection(t *testing.T) {
	// sample(): a=06-01/food/acc1/"Coffee shop"/450, b=06-03/rent/acc2/"Rent"/120000,
	// c=06-02/pay/acc1/"Payday"/250000. Category/account sort by raw ID here.
	cases := []struct {
		key, dir, want string
	}{
		{"date", Asc, "acb"},
		{"date", Desc, "bca"},
		{"amount", Asc, "abc"},
		{"amount", Desc, "cba"},
		{"payee", Asc, "acb"},
		{"payee", Desc, "bca"},
		{"category", Asc, "acb"}, // food < pay < rent
		{"category", Desc, "bca"},
		{"account", Asc, "acb"},  // acc1(a,c tie ID) then acc2(b)
		{"account", Desc, "bac"}, // acc2(b) then acc1(a,c tie ID)
	}
	for _, tc := range cases {
		got := ids(Apply(sample(), Criteria{Sort: tc.key, Dir: tc.dir}))
		if got != tc.want {
			t.Errorf("sort %s/%s = %q, want %q", tc.key, tc.dir, got, tc.want)
		}
	}
}

func TestApplyWithLabelsSortsByName(t *testing.T) {
	labels := Labels{
		Category: map[string]string{"food": "Food", "rent": "Rent", "pay": "Salary"},
		Account:  map[string]string{"acc1": "Zebra", "acc2": "Apple"},
	}
	// Category by name asc: Food(a) < Rent(b) < Salary(c).
	if got := ids(ApplyWithLabels(sample(), Criteria{Sort: "category", Dir: Asc}, labels)); got != "abc" {
		t.Errorf("category-by-name asc = %q, want abc", got)
	}
	// Account by name asc: Apple(b) < Zebra(a,c tie on ID).
	if got := ids(ApplyWithLabels(sample(), Criteria{Sort: "account", Dir: Asc}, labels)); got != "bac" {
		t.Errorf("account-by-name asc = %q, want bac", got)
	}
}

func TestNormalizeSortAndDir(t *testing.T) {
	if n := (Criteria{}).Normalize(); n.Sort != "date" || n.Dir != Desc {
		t.Errorf("empty normalize = %s/%s, want date/desc", n.Sort, n.Dir)
	}
	if n := (Criteria{Sort: "bogus"}).Normalize(); n.Sort != "date" {
		t.Errorf("invalid sort key not reset: %s", n.Sort)
	}
	if n := (Criteria{Sort: "payee"}).Normalize(); n.Dir != Asc {
		t.Errorf("payee default dir = %s, want asc", n.Dir)
	}
	if n := (Criteria{Sort: "amount", Dir: "sideways"}).Normalize(); n.Dir != Desc {
		t.Errorf("invalid dir not reset to amount default: %s", n.Dir)
	}
	if n := (Criteria{Sort: "date", Dir: Asc}).Normalize(); n.Dir != Asc {
		t.Errorf("explicit dir overwritten: %s", n.Dir)
	}
}

func TestNormalizePagination(t *testing.T) {
	if n := (Criteria{}).Normalize(); n.Page != 1 || n.PageSize != DefaultPageSize {
		t.Errorf("empty page normalize = page %d size %d, want 1/%d", n.Page, n.PageSize, DefaultPageSize)
	}
	if n := (Criteria{Page: 0}).Normalize(); n.Page != 1 {
		t.Errorf("page 0 -> %d, want 1", n.Page)
	}
	if n := (Criteria{Page: 4, PageSize: 25}).Normalize(); n.Page != 4 || n.PageSize != 25 {
		t.Errorf("explicit page/size changed: %d/%d", n.Page, n.PageSize)
	}
	if n := (Criteria{PageSize: PageSizeAll}).Normalize(); n.PageSize != PageSizeAll {
		t.Errorf("All page size not preserved: %d", n.PageSize)
	}
}

func TestScopeChangedAndPageReset(t *testing.T) {
	base := Criteria{Account: "acc1", Sort: "date", Page: 3}
	// Only the page differs -> same scope.
	if ScopeChanged(base, Criteria{Account: "acc1", Sort: "date", Page: 7}) {
		t.Error("changing only the page should not be a scope change")
	}
	// Different filter / sort / direction -> scope changed.
	if !ScopeChanged(base, Criteria{Account: "acc2", Sort: "date", Page: 3}) {
		t.Error("changing the account filter should be a scope change")
	}
	if !ScopeChanged(base, Criteria{Account: "acc1", Sort: "amount", Page: 3}) {
		t.Error("changing the sort key should be a scope change")
	}
	if !ScopeChanged(base, Criteria{Account: "acc1", Sort: "date", Dir: Asc, Page: 3}) {
		t.Error("flipping the sort direction should be a scope change")
	}
	// ResetPageIfScopeChanged keeps the page on a same-scope change, resets otherwise.
	if got := (Criteria{Account: "acc1", Sort: "date", Page: 5}).ResetPageIfScopeChanged(base); got.Page != 5 {
		t.Errorf("same-scope page reset to %d, want kept 5", got.Page)
	}
	if got := (Criteria{Account: "acc2", Sort: "date", Page: 5}).ResetPageIfScopeChanged(base); got.Page != 1 {
		t.Errorf("scope-changed page = %d, want reset to 1", got.Page)
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

func TestApplyTieOrderingIsDeterministic(t *testing.T) {
	// Same date and same amount across rows: order must fall back to ID, stable
	// across input arrangements.
	mk := func(order []string) []domain.Transaction {
		out := make([]domain.Transaction, len(order))
		for i, id := range order {
			out[i] = domain.Transaction{ID: id, Desc: "Same", Amount: money.New(-100, "USD"), Date: d("2026-06-01")}
		}
		return out
	}
	for _, sortKey := range []string{"date", "amount", "payee"} {
		a := Apply(mk([]string{"c", "a", "b"}), Criteria{Sort: sortKey})
		b := Apply(mk([]string{"b", "c", "a"}), Criteria{Sort: sortKey})
		if ids(a) != "abc" || ids(b) != "abc" {
			t.Errorf("sort %q: got %q and %q, want both abc", sortKey, ids(a), ids(b))
		}
	}
}

func TestApplyDoesNotMutate(t *testing.T) {
	all := sample()
	_ = Apply(all, Criteria{Sort: "amount"})
	if all[0].ID != "a" {
		t.Error("Apply mutated the input slice order")
	}
}

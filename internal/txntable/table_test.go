package txntable

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestPaginate(t *testing.T) {
	for _, tt := range []struct {
		page, size, total int
		want              Page
	}{
		{0, 0, 0, Page{Number: 1, Size: 25}},
		{-1, 25, 26, Page{Number: 1, Size: 25, End: 25, SelectionEnd: 25}},
		{99, 25, 26, Page{Number: 2, Size: 25, Start: 25, End: 26, SelectionStart: 25, SelectionEnd: 26}},
		{2, -1, 101, Page{Number: 1, Size: -1, End: 101, SelectionEnd: 101, Virtual: true}},
		{2, -1, 100, Page{Number: 1, Size: -1, End: 100, SelectionEnd: 100}},
		{1, 25, 25, Page{Number: 1, Size: 25, End: 25, SelectionEnd: 25, OnePage: true}},
		{math.MaxInt, math.MaxInt, math.MaxInt, Page{Number: 1, Size: math.MaxInt, End: math.MaxInt, SelectionEnd: math.MaxInt}},
		{math.MaxInt, 2, math.MaxInt, Page{Number: math.MaxInt/2 + 1, Size: 2, Start: math.MaxInt - 1, End: math.MaxInt, SelectionStart: math.MaxInt - 1, SelectionEnd: math.MaxInt}},
		{1, 25, -1, Page{Number: 1, Size: 25}},
	} {
		t.Run(fmt.Sprint(tt.page, "/", tt.size, "/", tt.total), func(t *testing.T) {
			if got := Paginate(tt.page, tt.size, tt.total); got != tt.want {
				t.Fatalf("got %+v want %+v", got, tt.want)
			}
		})
	}
}

func TestCriteriaDecisions(t *testing.T) {
	f := txnfilter.Criteria{Text: "coffee", Page: 4, PageSize: 50, Sort: "amount", Dir: txnfilter.Asc}
	if got := SelectionScope(f); got.Text != "coffee" || got.Page != 0 || got.PageSize != 0 || got.Sort != "" || got.Dir != "" {
		t.Fatal(got)
	}
	for _, tt := range []struct {
		register  bool
		hidden    map[string]bool
		sort, dir string
	}{
		{false, nil, "amount", txnfilter.Asc}, {true, map[string]bool{"amount": true}, "date", txnfilter.Asc}, {false, map[string]bool{"amount": true}, "date", txnfilter.Desc},
	} {
		got := EffectiveCriteria(f, tt.register, tt.hidden)
		if got.Sort != tt.sort || got.Dir != tt.dir {
			t.Fatal(got)
		}
	}
	for _, tt := range []struct {
		key       string
		register  bool
		sort, dir string
	}{
		{"amount", false, "amount", txnfilter.Desc}, {"date", false, "date", txnfilter.Desc}, {"payee", false, "payee", txnfilter.Asc}, {"date", true, "amount", txnfilter.Asc},
	} {
		got := SortBy(f, tt.key, tt.register)
		if got.Sort != tt.sort || got.Dir != tt.dir {
			t.Fatal(got)
		}
	}
	if SortBy(txnfilter.Criteria{Sort: "date", Dir: txnfilter.Desc}, "date", false).Dir != txnfilter.Asc {
		t.Fatal("toggle")
	}
	if SortKey(true, "date") != "" || SortKey(false, "date") != "date" {
		t.Fatal("sort key")
	}
	cat := CategoryCriteria(txnfilter.Criteria{Text: "x", Flow: "out", Category: "c", Uncategorized: true, Sort: "amount", Dir: "asc", Page: 2, PageSize: 25})
	if cat != (txnfilter.Criteria{Text: "x"}) {
		t.Fatal(cat)
	}
}

func TestToggleSelection(t *testing.T) {
	order := []string{"a", "b", "c", "d"}
	for _, tt := range []struct {
		name, anchor, id string
		shift            bool
		selected, want   map[string]bool
	}{
		{"nil", "", "b", false, nil, map[string]bool{"b": true}},
		{"remove", "", "b", false, map[string]bool{"b": true, "a": false}, map[string]bool{}},
		{"forward", "a", "c", true, map[string]bool{"d": true}, map[string]bool{"a": true, "b": true, "c": true, "d": true}},
		{"reverse", "d", "b", true, nil, map[string]bool{"b": true, "c": true, "d": true}},
		{"missing", "z", "c", true, nil, map[string]bool{"c": true}},
		{"same", "c", "c", true, map[string]bool{"c": true}, map[string]bool{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := map[string]bool{}
			for k, v := range tt.selected {
				before[k] = v
			}
			got := ToggleSelection(tt.selected, tt.anchor, tt.id, tt.shift, order)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatal(got)
			}
			for k, v := range tt.selected {
				if before[k] != v {
					t.Fatal("mutated input")
				}
			}
		})
	}
}

func TestRangesAndIndexes(t *testing.T) {
	field := domain.Field{Values: []any{"a", "b", "c"}}
	for _, tt := range []struct {
		a, b int
		want []string
	}{{0, 2, []string{"a", "b"}}, {-1, 99, []string{"a", "b", "c"}}, {3, 2, []string{}}, {99, 100, []string{}}} {
		if got := IDs(field, tt.a, tt.b); !reflect.DeepEqual(got, tt.want) {
			t.Fatal(got)
		}
	}
	if got := Indices(-1, 3); !slices.Equal(got, []int{0, 1, 2}) {
		t.Fatal(got)
	}
	if len(Indices(3, 1)) != 0 {
		t.Fatal("reverse")
	}
	rows := []domain.Transaction{{ID: "a", Desc: "first"}, {ID: "b"}, {ID: "a", Desc: "last"}}
	groups := map[string]domain.TxnLink{"a": {TxnIDs: []string{"a", "missing"}}}
	if got := Transactions(rows); len(got) != 2 || got["a"].Desc != "last" {
		t.Fatal(got)
	}
	if got := GroupMembers(groups, rows); len(got) != 1 || got["a"].Desc != "last" {
		t.Fatal(got)
	}
	if len(GroupMembers(nil, rows)) != 0 || len(Transactions(nil)) != 0 {
		t.Fatal("nil")
	}
}

func TestIndexAndRegister(t *testing.T) {
	day := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{{ID: "a", Name: "Bank", Currency: "USD", OpeningBalance: money.New(100, "USD"), Reconciliations: []domain.Reconciliation{{At: day}}}, {ID: "b", Name: "Savings"}}
	links := []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"purchase", "refund"}}, {Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"invalid"}}, {Kind: domain.TxnLinkBillMatch, TxnIDs: []string{"bill"}}, {Kind: domain.TxnLinkEventTxn, TxnIDs: []string{"a"}, EventID: "e"}, {Kind: domain.TxnLinkEventTxn, TxnIDs: []string{"a"}, EventID: "later"}, {Kind: domain.TxnLinkEventTxn, TxnIDs: []string{"missing"}, EventID: "missing"}}
	x := BuildIndex(nil, accounts, []domain.Category{{ID: "c", Name: "Food"}}, []domain.Member{{ID: "m", Name: "Cam", Color: "blue"}}, links, []domain.Event{{ID: "e", Name: "Trip"}, {ID: "later", Name: "Wrong"}})
	if x.AccountNames["a"] != "Bank" || x.CategoryNames["c"] != "Food" || x.MemberNames["m"] != "Cam" || x.MemberColors["m"] != "blue" || !x.ReconciledThrough["a"].Equal(day) || !x.Refund["refund"] || !x.Refunded["purchase"] || !x.BillMatched["bill"] || x.EventNames["a"] != "Trip" {
		t.Fatalf("%+v", x)
	}
	if len(BuildIndex(nil, nil, nil, nil, nil, nil).Transactions) != 0 {
		t.Fatal("nil index")
	}
	rows := []domain.Transaction{{ID: "t", AccountID: "a", Date: day, Amount: money.New(-25, "USD")}}
	if got := RegisterBalances(true, "a", accounts, rows); got["t"].Amount != 75 {
		t.Fatal(got)
	}
	for _, tt := range []struct {
		active bool
		id     string
		rows   []domain.Transaction
	}{{false, "a", rows}, {true, "missing", rows}, {true, "a", []domain.Transaction{{ID: "bad", AccountID: "a", Amount: money.New(1, "EUR")}}}, {true, "a", []domain.Transaction{{ID: "bad", AccountID: "a", Amount: money.New(math.MaxInt64, "USD")}}}} {
		if RegisterBalances(tt.active, tt.id, accounts, tt.rows) != nil {
			t.Fatal(tt)
		}
	}
}

func TestGroupSum(t *testing.T) {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	for _, tt := range []struct {
		name    string
		amounts []money.Money
		want    money.Money
		err     bool
	}{
		{"empty", nil, money.Money{}, false}, {"zero", []money.Money{money.New(0, "USD")}, money.New(0, "USD"), false},
		{"signed", []money.Money{money.New(-120, "USD"), money.New(20, "USD")}, money.New(-100, "USD"), false},
		{"overflow", []money.Money{money.New(math.MaxInt64, "USD"), money.New(1, "USD")}, money.Money{}, true},
		{"underflow", []money.Money{money.New(math.MinInt64, "USD"), money.New(-1, "USD")}, money.Money{}, true},
		{"mixed", []money.Money{money.New(100, "USD"), money.New(100, "EUR")}, money.New(300, "USD"), false},
		{"missing fx", []money.Money{money.New(100, "USD"), money.New(100, "JPY")}, money.Money{}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := make([]domain.Transaction, len(tt.amounts))
			for i, a := range tt.amounts {
				rows[i].Amount = a
			}
			got, err := GroupSum(rows, rates)
			if got != tt.want || (err != nil) != tt.err {
				t.Fatalf("%+v %v", got, err)
			}
			if strings.Contains(tt.name, "flow") && !errors.Is(err, money.ErrOverflow) {
				t.Fatal(err)
			}
		})
	}
}

func TestDeriveRowsAndDescription(t *testing.T) {
	resolve := func(s string) string { return strings.ToUpper(s) }
	learned := func(s string) bool { return s == "alias" }
	for _, tt := range []struct{ desc, payee, want string }{{"statement", "alias", "ALIAS"}, {"statement", "raw", "statement"}, {" ", "raw", "RAW"}} {
		if got := Description(tt.desc, tt.payee, learned, resolve); got != tt.want {
			t.Fatal(got)
		}
	}
	if Description("", "raw", nil, nil) != "raw" {
		t.Fatal("nil resolver")
	}
	rows := []domain.Transaction{{ID: "z", Desc: "same", Amount: money.New(-10, "USD")}, {ID: "a", Desc: "SAME", Amount: money.New(-20, "USD")}, {ID: "b", Desc: "alpha", Amount: money.New(-30, "USD")}}
	for _, tt := range []struct {
		dir  string
		want []string
	}{{txnfilter.Asc, []string{"b", "a", "z"}}, {txnfilter.Desc, []string{"a", "z", "b"}}} {
		got, issues := DeriveRows(rows, txnfilter.Criteria{Sort: "payee", Dir: tt.dir}, txnfilter.Labels{Payee: resolve}, nil, resolve)
		ids := []string{}
		for _, r := range got {
			ids = append(ids, r.ID)
		}
		if !slices.Equal(ids, tt.want) || len(issues) > 0 {
			t.Fatal(ids, issues)
		}
	}
	if rows[0].ID != "z" {
		t.Fatal("input reordered")
	}
	got, _ := DeriveRows(nil, txnfilter.Criteria{Sort: "date"}, txnfilter.Labels{}, nil, nil)
	if len(got) != 0 {
		t.Fatal(got)
	}
	tasks := []domain.Task{{RelatedType: domain.RelatedTransaction, RelatedID: "a"}, {RelatedType: domain.RelatedTransaction, RelatedID: "a", Status: domain.StatusDone}, {RelatedType: domain.RelatedTransaction, RelatedID: "b"}}
	if OpenTaskCount(tasks, "a") != 1 || OpenTaskCount(nil, "a") != 0 {
		t.Fatal("task count")
	}
	for _, tt := range []struct {
		row  domain.Transaction
		want string
	}{{domain.Transaction{Payee: " payee ", Desc: "desc"}, "payee"}, {domain.Transaction{Desc: " desc "}, "desc"}, {domain.Transaction{Payee: " ", Desc: "desc"}, ""}} {
		if RulePhrase(tt.row) != tt.want {
			t.Fatal(tt)
		}
	}
}

func FuzzPaginateBounds(f *testing.F) {
	f.Add(5, 25, 123)
	f.Add(math.MaxInt, 2, math.MaxInt)
	f.Add(-2, -1, 0)
	f.Fuzz(func(t *testing.T, page, size, total int) {
		p := Paginate(page, size, total)
		total = max(0, total)
		if p.Start < 0 || p.Start > p.End || p.End > total || p.Number < 1 {
			t.Fatalf("invalid %+v", p)
		}
		if p.Size > 0 && p.End-p.Start > p.Size {
			t.Fatal(p)
		}
	})
}

func FuzzSelectionRoundTrip(f *testing.F) {
	f.Add("a")
	f.Add("")
	f.Fuzz(func(t *testing.T, id string) {
		original := map[string]bool{"existing": true}
		once := ToggleSelection(original, "", id, false, nil)
		twice := ToggleSelection(once, "", id, false, nil)
		if !reflect.DeepEqual(twice, original) {
			t.Fatal(twice)
		}
	})
}

func TestVisibleTransfers(t *testing.T) {
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	rows := []domain.Transaction{
		{ID: "out", AccountID: "a", TransferAccountID: "b", TransferGroupID: "pair", Date: day, Amount: money.New(-100, "USD")},
		{ID: "in", AccountID: "b", TransferAccountID: "a", TransferGroupID: "pair", Date: day, Amount: money.New(100, "USD")},
		{ID: "ordinary", Date: day, Amount: money.New(-10, "USD")},
	}
	for _, tt := range []struct {
		name     string
		rows     []domain.Transaction
		criteria txnfilter.Criteria
		want     int
	}{
		{"household", rows, txnfilter.Criteria{}, 2}, {"single account", rows, txnfilter.Criteria{Account: " a "}, 3},
		{"single multi-selector", rows, txnfilter.Criteria{Accounts: "a"}, 3}, {"multiple accounts", rows, txnfilter.Criteria{Accounts: "a,b"}, 2},
		{"empty", nil, txnfilter.Criteria{}, 0}, {"one", rows[:1], txnfilter.Criteria{}, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := VisibleTransfers(tt.rows, tt.criteria, currency.Rates{Base: "USD"})
			if len(got) != tt.want {
				t.Fatal(got)
			}
			if len(got) > 1 && got[len(got)-1].ID != "ordinary" {
				t.Fatal("source order changed")
			}
		})
	}
	if len(rows) != 3 || rows[1].ID != "in" {
		t.Fatal("input changed")
	}
}

func TestRowIndex(t *testing.T) {
	for _, tt := range []struct {
		virtual     bool
		index, want int
	}{{false, 0, 0}, {true, 0, 2}, {true, 40, 42}, {true, -1, 0}, {true, math.MaxInt, 0}} {
		if got := (Page{Virtual: tt.virtual}).RowIndex(tt.index); got != tt.want {
			t.Fatal(tt, got)
		}
	}
}

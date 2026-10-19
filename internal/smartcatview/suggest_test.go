// SPDX-License-Identifier: MIT

package smartcatview

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCategoryDecisions(t *testing.T) {
	cats := fixtureCats()
	for _, tc := range []struct {
		amount  int64
		id      string
		kind    domain.CategoryKind
		matches bool
	}{
		{0, "food", domain.KindExpense, true}, {-1, "food", domain.KindExpense, true},
		{math.MinInt64, "pay", domain.KindExpense, false}, {math.MaxInt64, "pay", domain.KindIncome, true},
		{1, "food", domain.KindIncome, false}, {-1, "absent", domain.KindExpense, false}, {0, "", domain.KindExpense, false},
	} {
		txn := transaction("x", "", tc.amount)
		if CategoryKind(txn) != tc.kind || MatchesDirection(txn, tc.id, cats) != tc.matches {
			t.Fatal(tc)
		}
	}
	for _, tc := range []struct {
		id   string
		cats []domain.Category
		ok   bool
	}{
		{"", cats, false}, {"food", nil, false}, {"food", cats, true},
		{"food", append([]domain.Category{{ID: "food", Name: "First"}}, cats...), true},
	} {
		got, ok := CategoryByID(tc.cats, tc.id)
		if ok != tc.ok || ok && got.ID != tc.id {
			t.Fatal(got, ok)
		}
	}
	for _, tc := range []struct {
		cat  string
		has  bool
		want string
	}{
		{"food", true, "food"}, {"", true, "pay"}, {"", false, ""},
	} {
		if got := SeedCategory(transaction("x", tc.cat, 1), catsuggest.Suggestion{CategoryID: "pay"}, tc.has); got != tc.want {
			t.Fatal(got)
		}
	}
}

func TestChipVisible(t *testing.T) {
	for _, tc := range []struct {
		name  string
		txn   domain.Transaction
		s     catsuggest.Suggestion
		found bool
		want  bool
	}{
		{"medium", transaction("x", "", -1), catsuggest.Suggestion{CategoryID: "food", Confidence: catsuggest.ConfMedium}, true, true},
		{"low", transaction("x", "", -1), catsuggest.Suggestion{CategoryID: "food", Confidence: catsuggest.ConfLow}, true, false},
		{"missing", transaction("x", "", -1), catsuggest.Suggestion{CategoryID: "absent", Confidence: catsuggest.ConfHigh}, true, false},
		{"no match", transaction("x", "", -1), catsuggest.Suggestion{CategoryID: "food", Confidence: catsuggest.ConfHigh}, false, false},
		{"filed", transaction("x", "food", -1), catsuggest.Suggestion{CategoryID: "food", Confidence: catsuggest.ConfHigh}, true, false},
		{"wrong sign", transaction("x", "", 1), catsuggest.Suggestion{CategoryID: "food", Confidence: catsuggest.ConfHigh}, true, false},
		{"transfer", domain.Transaction{TransferAccountID: "other"}, catsuggest.Suggestion{CategoryID: "food", Confidence: catsuggest.ConfHigh}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ChipVisible(tc.txn, tc.s, tc.found, fixtureCats()); got != tc.want {
				t.Fatal(got)
			}
		})
	}
}

func TestIndexTransactions(t *testing.T) {
	a, b := transaction("a", "", -1), transaction("a", "food", -2)
	for _, tc := range []struct {
		name string
		txns []domain.Transaction
		want map[string]domain.Transaction
	}{
		{"nil", nil, map[string]domain.Transaction{}},
		{"one", []domain.Transaction{a}, map[string]domain.Transaction{"a": a}},
		{"last duplicate", []domain.Transaction{a, b}, map[string]domain.Transaction{"a": b}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IndexTransactions(tc.txns); !reflect.DeepEqual(got, tc.want) {
				t.Fatal(got)
			}
		})
	}
}

func TestMerchantScope(t *testing.T) {
	current := transaction("current", "food", -1)
	other := transaction("other", "", -2)
	other.Amount.Currency = "EUR"
	filed := transaction("filed", "food", -3)
	different := transaction("different", "", -4)
	different.Payee = "Another"
	transfer := transaction("transfer", "", -5)
	transfer.TransferAccountID = "other"
	empty := domain.Transaction{ID: "empty"}
	for _, tc := range []struct {
		name    string
		current domain.Transaction
		txns    []domain.Transaction
		others  int
		ids     []string
	}{
		{"same merchant", current, []domain.Transaction{current, other, filed, different, transfer}, 1, []string{"current", "other"}},
		{"current only", current, []domain.Transaction{current}, 0, []string{"current"}},
		{"nil", current, nil, 0, nil},
		{"current absent", current, []domain.Transaction{other}, 1, []string{"other"}},
		{"different merchant", current, []domain.Transaction{current, different}, 0, []string{"current"}},
		{"already filed peer", current, []domain.Transaction{current, filed}, 0, []string{"current"}},
		{"transfer peer", current, []domain.Transaction{current, transfer}, 0, []string{"current"}},
		{"empty key", empty, []domain.Transaction{current, other, empty}, 0, []string{"empty"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := MerchantOthers(tc.current, tc.txns); got != tc.others {
				t.Fatal(got)
			}
			before := append([]domain.Transaction(nil), tc.txns...)
			writes, ids := MerchantWrites(tc.current, tc.txns, "gas")
			if !reflect.DeepEqual(ids, tc.ids) || len(writes) != len(tc.ids) {
				t.Fatal(writes, ids)
			}
			for _, w := range writes {
				if w.CategoryID != "gas" || !w.Reviewed {
					t.Fatal(w)
				}
			}
			if !reflect.DeepEqual(tc.txns, before) {
				t.Fatal("write plan mutated input")
			}
			for _, w := range writes {
				if w.ID == "other" && w.Amount.Currency != "EUR" {
					t.Fatal("combined currencies")
				}
			}
		})
	}
}

func TestOptionsLabelsAnswers(t *testing.T) {
	for _, tc := range []struct {
		kind domain.CategoryKind
		cats []domain.Category
		n    int
	}{
		{domain.KindExpense, fixtureCats(), 3}, {domain.KindIncome, fixtureCats(), 2}, {domain.KindExpense, nil, 1},
	} {
		opts := CategoryOptions(tc.cats, tc.kind, "Choose")
		if len(opts) != tc.n || opts[0].Value != "" || opts[0].Label != "Choose" {
			t.Fatal(opts)
		}
	}
	for _, tc := range []struct {
		txn  domain.Transaction
		want string
	}{
		{domain.Transaction{Payee: " Store ", Desc: "desc"}, "Store"}, {domain.Transaction{Desc: " desc "}, "desc"}, {domain.Transaction{}, ""},
	} {
		if got := TransactionLabel(tc.txn); got != tc.want {
			t.Fatal(got)
		}
	}
	req := CategorizeRequest(transaction("x", "", -123), fixtureCats())
	if req.System == "" || req.User == "" {
		t.Fatal(req)
	}
	for _, tc := range []struct{ answer, id, key string }{
		{"  Food  ", "food", "catSuggest.aiSource"}, {"Food > Gas", "gas", "catSuggest.aiSource"},
		{"invention", "", "catSuggest.aiUnmatched"}, {"", "", "catSuggest.aiUnmatched"},
	} {
		id, key, _ := CategoryAnswer(tc.answer, fixtureCats())
		if id != tc.id || key != tc.key {
			t.Fatal(id, key)
		}
	}
	for _, n := range []int{0, 1, 2, 100000} {
		key := FiledLabelKey(n)
		want := "catSuggest.filedUndo"
		if n > 1 {
			want = "catSuggest.filedBatchUndo"
		}
		if key != want {
			t.Fatal(key)
		}
	}
}

func TestRevisionCache(t *testing.T) {
	var cache RevisionCache[string, int]
	builds := 0
	build := func() int { builds++; return builds }
	for _, tc := range []struct {
		identity string
		rev      int
		store    uint64
		want     int
	}{
		{"a", 1, 1, 1}, {"a", 1, 1, 1}, {"a", 2, 1, 2}, {"a", 2, 2, 3}, {"b", 2, 2, 4}, {"", 0, 0, 5},
	} {
		if got := cache.Get(tc.identity, tc.rev, tc.store, build); got != tc.want {
			t.Fatal(got, tc)
		}
	}
}

func FuzzMerchantCountMatchesWrites(f *testing.F) {
	f.Add("Store", "Store", false)
	f.Add("", "Other", true)
	f.Fuzz(func(t *testing.T, payee, other string, filed bool) {
		current := transaction("current", "old", -1)
		current.Payee = payee
		peer := transaction("peer", "", -1)
		peer.Payee = other
		if filed {
			peer.CategoryID = "old"
		}
		txns := []domain.Transaction{current, peer}
		writes, _ := MerchantWrites(current, txns, "food")
		if len(writes) != MerchantOthers(current, txns)+1 {
			t.Fatal("count/write scope diverged")
		}
	})
}

func TestCategoryAskKeyAndAmount(t *testing.T) {
	for _, tc := range []struct {
		loading bool
		key     string
	}{{false, "catSuggest.suggestAI"}, {true, "catSuggest.asking"}} {
		if CategoryAskKey(tc.loading) != tc.key {
			t.Fatal(tc)
		}
	}
	if got := TransactionAmount(transaction("zero", "", 0)); got != "$0.00" {
		t.Fatal(got)
	}
}

func TestTransactionAmountBoundaries(t *testing.T) {
	for _, tc := range []struct {
		amount         int64
		currency, want string
	}{
		{0, "USD", "$0.00"}, {-1, "USD", "($0.01)"},
		{math.MinInt64, "USD", "($92,233,720,368,547,758.08)"},
		{math.MaxInt64, "USD", "$92,233,720,368,547,758.07"},
		{123, "ZZZ", "ZZZ1.23"}, {123, "", "1.23"},
	} {
		txn := domain.Transaction{Amount: money.New(tc.amount, tc.currency)}
		if got := TransactionAmount(txn); got != tc.want {
			t.Fatalf("%+v: %q != %q", txn.Amount, got, tc.want)
		}
	}
}

func TestEligibleTransaction(t *testing.T) {
	for _, tc := range []struct {
		txn         domain.Transaction
		found, want bool
	}{
		{domain.Transaction{}, true, true}, {domain.Transaction{}, false, false},
		{domain.Transaction{CategoryID: "food"}, true, false}, {domain.Transaction{TransferAccountID: "other"}, true, false},
	} {
		if EligibleTransaction(tc.txn, tc.found) != tc.want {
			t.Fatal(tc)
		}
	}
}

func TestMemo(t *testing.T) {
	for _, tc := range []struct {
		name        string
		values      map[string]int
		want, calls int
	}{
		{"missing", map[string]int{}, 7, 1},
		{"cached zero", map[string]int{"one": 0}, 0, 0},
		{"cached nonzero", map[string]int{"one": 3}, 3, 0},
		{"nil map", nil, 7, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got := Memo(tc.values, "one", func() int { calls++; return 7 })
			if got != tc.want || calls != tc.calls {
				t.Fatal(got, calls)
			}
			if tc.values != nil {
				again := Memo(tc.values, "one", func() int { t.Fatal("resolved cached value"); return 9 })
				if again != got {
					t.Fatal(again)
				}
			}
		})
	}
}

func TestFiledLabel(t *testing.T) {
	translate := func(key string, args ...any) string { return fmt.Sprintf("%s%v", key, args) }
	for _, tc := range []struct {
		n    int
		want string
	}{
		{1, "catSuggest.filedUndo[Food]"}, {3, "catSuggest.filedBatchUndo[3 Food]"},
	} {
		if got := FiledLabel(tc.n, "Food", translate); got != tc.want {
			t.Fatal(got)
		}
	}
}

func TestCategorizeRequest(t *testing.T) {
	for _, tc := range []struct {
		name     string
		txn      domain.Transaction
		cats     []domain.Category
		fragment string
	}{
		{"payee", transaction("x", "", -123), fixtureCats(), "Store ($1.23)"},
		{"description", domain.Transaction{Desc: " Memo ", Amount: money.New(0, "USD")}, nil, "Memo $0.00"},
		{"unknown currency", domain.Transaction{Payee: "Merchant", Amount: money.New(123, "ZZZ")}, nil, "Merchant ZZZ1.23"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := CategorizeRequest(tc.txn, tc.cats)
			if got.System == "" || !strings.Contains(got.User, tc.fragment) {
				t.Fatal(got)
			}
		})
	}
}

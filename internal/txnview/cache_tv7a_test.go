// SPDX-License-Identifier: MIT

package txnview

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTV7CacheKeysExactRowInputs(t *testing.T) {
	coffee := domain.Transaction{ID: "coffee", CategoryID: "dining", Amount: money.New(-675, "USD")}
	groceries := domain.Transaction{ID: "groceries", CategoryID: "food", Amount: money.New(-21400, "USD")}
	out := domain.Transaction{ID: "out", AccountID: "checking", TransferAccountID: "loan", TransferGroupID: "pair", Amount: money.New(-48000, "USD")}
	in := domain.Transaction{ID: "in", AccountID: "loan", TransferAccountID: "checking", TransferGroupID: "pair", Amount: money.New(48000, "USD")}
	rates := currency.Rates{Base: "USD"}
	for _, tt := range []struct {
		name       string
		first      []domain.Transaction
		rows       []domain.Transaction
		categories []domain.Transaction
	}{
		{"search clear", []domain.Transaction{coffee}, []domain.Transaction{coffee, groceries, out, in}, []domain.Transaction{coffee, groceries}},
		{"same length replacement", []domain.Transaction{coffee}, []domain.Transaction{groceries}, []domain.Transaction{groceries}},
		{"transfer order", []domain.Transaction{out, in}, []domain.Transaction{in, out}, []domain.Transaction{in, out}},
		{"category scope only", []domain.Transaction{coffee}, []domain.Transaction{coffee}, []domain.Transaction{coffee, groceries}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var cache Cache
			if _, err := cache.Get(7, "same-scope", tt.first, rates, "USD"); err != nil {
				t.Fatal(err)
			}
			got, err := cache.GetWithCategories(7, "same-scope", tt.rows, tt.categories, rates, "USD")
			if err != nil {
				t.Fatal(err)
			}
			want, err := SummarizeWithCategories(tt.rows, tt.categories, rates, "USD")
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("cached view reused another input set: got %+v; want %+v; error %v", got, want, err)
			}
		})
	}
}

func TestTV7CacheKeysConversionInputs(t *testing.T) {
	var cache Cache
	rows := []domain.Transaction{{ID: "eur", Amount: money.New(-100, "EUR")}}
	if _, err := cache.Get(1, "month", rows, currency.Rates{Base: "USD"}, "USD"); err != nil {
		t.Fatal(err)
	}
	got, err := cache.Get(1, "month", rows, currency.Rates{Base: "EUR"}, "EUR")
	if err != nil || got.Spent != 100 || got.Unconverted != 0 {
		t.Fatalf("base currency change reused old conversion: %+v, %v", got, err)
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.2}}
	if _, err := cache.Get(1, "month", rows, rates, "USD"); err != nil {
		t.Fatal(err)
	}
	rates.Rates["EUR"] = 1.5
	got, err = cache.Get(1, "month", rows, rates, "USD")
	if err != nil || got.Spent != 150 || got.Unconverted != 0 {
		t.Fatalf("in-place rate change reused old conversion: %+v, %v", got, err)
	}
}

func TestTV7RowIdentityKeysHaveNoSeparatorCollisions(t *testing.T) {
	sets := [][]domain.Transaction{
		{{ID: "a"}, {ID: "bc"}},
		{{ID: "ab"}, {ID: "c"}},
		{{ID: "a|b"}, {ID: "c"}},
		{{ID: "a"}, {ID: "b|c"}},
		{{ID: "1:a"}},
		{{ID: ""}, {ID: "a"}},
	}
	seen := map[string]bool{}
	for _, rows := range sets {
		key := rowIDsKey(rows)
		if seen[key] {
			t.Fatalf("ambiguous ordered row identity key %q", key)
		}
		seen[key] = true
	}
}

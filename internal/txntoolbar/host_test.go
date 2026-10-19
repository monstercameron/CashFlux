// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestHostDeriveKeepsOrderingWithoutPaging(t *testing.T) {
	rows := []domain.Transaction{transaction("low", -100), transaction("high", 200), transaction("middle", 0)}
	for _, tt := range []struct {
		dir string
		ids []string
	}{
		{txnfilter.Asc, []string{"low", "middle", "high"}},
		{txnfilter.Desc, []string{"high", "middle", "low"}},
	} {
		got, err := Derive(rows, txnfilter.Criteria{Sort: "amount", Dir: tt.dir, Page: 2, PageSize: 1}, txnfilter.Labels{}, currency.Rates{Base: "USD"}, "USD", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, row := range got.Rows {
			ids = append(ids, row.ID)
		}
		if !reflect.DeepEqual(ids, tt.ids) || got.View.Count != 3 || got.View.Spent != 100 || got.View.Income != 200 {
			t.Fatalf("ids=%v view=%+v", ids, got.View)
		}
	}
}

func TestHostDeriveArithmeticFailureKeepsCorrectableRows(t *testing.T) {
	rows := []domain.Transaction{transaction("overflow", math.MaxInt64), transaction("one", 100), transaction("foreign", -100)}
	rows[2].Amount = money.New(-100, "EUR")
	rates := currency.Rates{Base: "USD"}
	labels := txnfilter.Labels{BaseCurrency: "USD", ToBase: func(m money.Money) (money.Money, error) { return rates.Convert(m, "USD") }}
	got, err := Derive(rows, txnfilter.Criteria{AmountMin: "1"}, labels, rates, "USD", time.Now())
	if err == nil || len(got.Rows) != 2 || len(got.Issues) != 1 {
		t.Fatalf("value=%+v err=%v", got, err)
	}
	if got.View.Spent != 0 || got.View.Income != 0 || got.View.Net != 0 || got.View.Count != 2 || len(got.View.Categories) != 0 {
		t.Fatalf("partial totals leaked: %+v", got.View)
	}
}

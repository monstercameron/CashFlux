// SPDX-License-Identifier: MIT

package txntable

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txntoolbar"
)

func TestHostCategoryCriteriaMatchesToolbar(t *testing.T) {
	for _, f := range []txnfilter.Criteria{
		{},
		{Category: "one", Categories: "two,three", Uncategorized: true, ScopeAny: true, Flow: "out", Sort: "amount", Dir: "asc", Page: 3, PageSize: 25, Text: "coffee", Account: "bank", Member: "member", From: "2026-10-01", To: "2026-10-31"},
	} {
		if got, want := CategoryCriteria(f), txntoolbar.CategoryScope(f); got != want {
			t.Fatalf("category criteria=%+v, want %+v", got, want)
		}
	}
}

func TestHostDescriptionOrderPreservesSnapshot(t *testing.T) {
	rows := []domain.Transaction{{ID: "b", Payee: "Alpha", Desc: "Zulu"}, {ID: "a", Payee: "Zulu", Desc: "Alpha"}, {ID: "c", Payee: "Other", Desc: "Alpha"}}
	original := append([]domain.Transaction(nil), rows...)
	for _, tt := range []struct {
		sort, dir string
		ids       []string
	}{
		{"payee", txnfilter.Asc, []string{"a", "c", "b"}},
		{"payee", txnfilter.Desc, []string{"b", "a", "c"}},
		{"date", txnfilter.Asc, []string{"b", "a", "c"}},
	} {
		got := OrderDescriptions(rows, txnfilter.Criteria{Sort: tt.sort, Dir: tt.dir}, nil, nil)
		var ids []string
		for _, row := range got {
			ids = append(ids, row.ID)
		}
		if !reflect.DeepEqual(ids, tt.ids) {
			t.Fatalf("%s %s: %v", tt.sort, tt.dir, ids)
		}
		got[0].Desc = "changed"
		if !reflect.DeepEqual(rows, original) {
			t.Fatal("shared snapshot mutated")
		}
	}
}

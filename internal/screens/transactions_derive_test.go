// SPDX-License-Identifier: MIT

package screens

import (
	"fmt"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestDeriveTxnRowsSortsDisplayedDescriptionWithStableTies(t *testing.T) {
	input := []domain.Transaction{
		{ID: "raw-first", Payee: "store-a", Desc: "A raw import"},
		{ID: "alias-first", Payee: "store-b", Desc: "Z raw import"},
		{ID: "blank", Payee: "store-c"},
		{ID: "tie-b", Desc: "same"},
		{ID: "tie-a", Desc: "Same"},
	}
	resolve := func(raw string) string {
		return map[string]string{"store-a": "Zulu Market", "store-b": "Alpha Market", "store-c": "Middle Market"}[raw]
	}
	hasLearned := func(raw string) bool { return raw == "store-a" || raw == "store-b" }
	for _, tc := range []struct {
		name string
		dir  string
		want []string
	}{
		{name: "ascending", dir: txnfilter.Asc, want: []string{"alias-first", "blank", "tie-a", "tie-b", "raw-first"}},
		{name: "descending", dir: txnfilter.Desc, want: []string{"raw-first", "tie-a", "tie-b", "blank", "alias-first"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := deriveTxnRows(input, txnfilter.Criteria{Sort: "payee", Dir: tc.dir}, txnfilter.Labels{}, hasLearned, resolve)
			for i, id := range tc.want {
				if rows[i].ID != id {
					t.Fatalf("row %d = %q, want %q", i, rows[i].ID, id)
				}
			}
		})
	}
}

type benchmarkTxnRowsMemo struct {
	revision uint64
	criteria txnfilter.Criteria
	rows     []domain.Transaction
	loaded   bool
}

func (m *benchmarkTxnRowsMemo) get(revision uint64, txns []domain.Transaction, criteria txnfilter.Criteria, labels txnfilter.Labels) []domain.Transaction {
	if !m.loaded || m.revision != revision || m.criteria != criteria {
		m.rows = deriveTxnRows(txns, criteria, labels, nil, nil)
		m.revision, m.criteria, m.loaded = revision, criteria, true
	}
	return m.rows
}

func benchmarkLedger3300() []domain.Transaction {
	rows := make([]domain.Transaction, 3300)
	for i := range rows {
		rows[i] = domain.Transaction{
			ID:        fmt.Sprintf("txn-%04d", i),
			Desc:      fmt.Sprintf("Market purchase %04d", i),
			Payee:     "Market",
			AccountID: fmt.Sprintf("account-%d", i%8),
			Amount:    money.New(int64((i%2500)-1250), "USD"),
		}
	}
	return rows
}

// BenchmarkTxnFilterSort3300 compares the full derivation with the warm render
// memo used by the surface host. Run with -benchmem to include allocation costs.
func BenchmarkTxnFilterSort3300(b *testing.B) {
	transactions := benchmarkLedger3300()
	criteria := txnfilter.Criteria{Text: "market", Sort: "amount", Dir: txnfilter.Desc}
	labels := txnfilter.Labels{}
	b.Run("derive", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = deriveTxnRows(transactions, criteria, labels, nil, nil)
		}
	})
	b.Run("memo-hit", func(b *testing.B) {
		memo := benchmarkTxnRowsMemo{}
		_ = memo.get(1, transactions, criteria, labels)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = memo.get(1, transactions, criteria, labels)
		}
	})
}

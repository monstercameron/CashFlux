// SPDX-License-Identifier: MIT

package screens

import (
	"fmt"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestSOLP1DescriptionKeysAreResolvedOncePerRow(t *testing.T) {
	rows := make([]domain.Transaction, 3300)
	for i := range rows {
		rows[i] = domain.Transaction{ID: fmt.Sprint(i), Payee: fmt.Sprint(3300 - i)}
	}
	resolved, learned := 0, 0
	sortTxnDescriptions(rows, txnfilter.Asc, func(string) bool { learned++; return true }, func(s string) string { resolved++; return s })
	if resolved != len(rows) || learned != len(rows) {
		t.Fatalf("sort resolved=%d learned=%d, want %d each", resolved, learned, len(rows))
	}
}

func TestSOLP1FilterAliasResolutionIsBoundedByDistinctPayees(t *testing.T) {
	rows := make([]domain.Transaction, 3300)
	for i := range rows {
		rows[i] = domain.Transaction{ID: fmt.Sprint(i), Payee: "raw merchant", Desc: "purchase"}
	}
	resolved := 0
	deriveTxnRows(rows, txnfilter.Criteria{Text: "clean", Sort: "payee"}, txnfilter.Labels{Payee: func(string) string { resolved++; return "clean" }}, nil, nil)
	if resolved != 1 {
		t.Fatalf("filter alias resolver called %d times, want 1", resolved)
	}
}

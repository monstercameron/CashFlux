// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR1BWidthsHaveOneStableClassRuleSet(t *testing.T) {
	resetSheet()
	registerUxbatch6()
	out := Build()
	for _, want := range []string{
		".bento-ledger .txn-table th.td-date, .bento-ledger .txn-table td.td-date{width:112px;}",
		".bento-ledger .txn-table th.td-cat, .bento-ledger .txn-table td.td-cat{width:170px;}",
		".bento-ledger .txn-table th.td-acct, .bento-ledger .txn-table td.td-acct{width:150px;}",
		".bento-ledger .txn-table th.td-user, .bento-ledger .txn-table td.td-user{width:56px;}",
		".bento-ledger .txn-table th.td-user{text-align:center;}",
		".bento-ledger .txn-table th.td-amount, .bento-ledger .txn-table td.td-amount{width:112px;}",
		".bento-ledger .txn-table th.td-actions, .bento-ledger .txn-table td.td-actions{width:96px;}",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("transaction widths missing %q", want)
		}
		if count := strings.Count(out, want); count != 1 {
			t.Errorf("transaction width rule %q appears %d times, want one stable rule", want, count)
		}
	}
	if strings.Contains(out, "td-status{width:") {
		t.Error("transaction table must not reserve width for a Status column")
	}
	if strings.Contains(out, "nth-child(") {
		t.Error("transaction widths must use stable column classes")
	}
}

func TestR1BSelectedRowsDoNotDependOnZebraStripes(t *testing.T) {
	resetSheet()
	registerGenerated()
	out := Build()
	if strings.Contains(out, ".txn-table tbody tr.row:nth-child(even)") {
		t.Fatal("transaction table still has zebra striping")
	}
	if !strings.Contains(out, ".txn-table tbody tr.row.selected") || !strings.Contains(out, "background:var(--accent-dim)") {
		t.Fatal("selected transaction row tint rule is missing")
	}
}

func TestR4CReviewAttentionAndCategoryMarkPlacement(t *testing.T) {
	resetSheet()
	registerTxnScope()
	registerGenerated()
	registerTxnToolbar()
	out := Build()
	for _, want := range []string{
		".td-cat-name{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;min-width:0;flex:0 1 auto;}",
		".btn-tool.bt-stale{",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("R4C attention/category rule missing %q", want)
		}
	}
}

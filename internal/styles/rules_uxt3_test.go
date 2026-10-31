// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestUXT3ResponsiveTransactionsRules(t *testing.T) {
	resetSheet()
	registerRulesUXT3()
	css := Build()
	for _, want := range []string{
		".txn-desc-account{display:none;}",
		"@media (max-width: 1180px)",
		".txn-responsive-toolbar .filter-toolbar-primary{flex-wrap:wrap;}",
		".txn-responsive-toolbar .filter-toolbar-primary .filters-trigger{white-space:nowrap;}",
		".txn-responsive-toolbar .filter-toolbar-actions{flex-wrap:wrap;min-width:0;}",
		".bento-ledger .txn-table thead{display:table-header-group !important;}",
		".bento-ledger .txn-table tbody tr.row{display:table-row;}",
		".bento-ledger .txn-table tbody tr.row > td{display:table-cell;vertical-align:middle;}",
		".bento-ledger .txn-table .txn-col-who{display:none !important;}",
		".bento-ledger .txn-table .txn-col-account{display:none !important;}",
		".txn-desc-account{display:block;flex:0 0 100%;min-width:0;font-size:var(--fs-meta);",
		".txn-table .txn-col-amount{width:112px;min-width:112px;text-align:right;}",
		".txn-table .txn-col-actions{width:104px;min-width:104px;white-space:nowrap;}",
		".txn-table .txn-desc-mainline{flex-wrap:wrap;}",
		".txn-table .txn-desc-mainline .row-desc-text{flex:1 1 120px;min-width:120px;}",
		"@media (max-width: 767px)",
		"main.cf-scroll:has(.txn-table){padding-bottom:calc(112px + env(safe-area-inset-bottom, 0px)) !important;}",
		".txn-table tbody tr.row > td.td-actions .txn-row-menu button{max-width:100%;box-sizing:border-box;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("UXT3 stylesheet missing %q", want)
		}
	}
}

// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR9CCalendarWeekCellsUseStylesheetRules(t *testing.T) {
	resetSheet()
	registerTxnCalendar()
	css := Build()
	for _, want := range []string{
		".txn-cal-weeks{display:grid;grid-template-columns:repeat(8,minmax(7rem,1fr));min-width:56rem;gap:4px;}",
		".txn-cal-viewport{max-width:100%;overflow-x:auto;",
		"@container (max-width: 960px){.txn-cal-scroll-hint{display:inline}}",
		"height:8rem;overflow:hidden;",
		".txn-cal-week-head, .txn-cal-week-row{display:contents;}",
		".txn-cal-week-total{display:flex;flex-direction:column;",
		".txn-cal-week-spent{font-weight:600;white-space:nowrap;}",
		"border:1px solid var(--border);",
		"background:var(--bg-card);",
		".txn-cal-week-income{font-weight:500;",
		".txn-cal-ghost-status, .txn-cal-ghost-label{display:block;overflow:hidden;",
		".txn-cal-ghost-missed{white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}",
		".txn-cal-more-due{display:block;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("calendar stylesheet missing %q", want)
		}
	}
}

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
		".txn-cal-weeks{display:grid;grid-template-columns:repeat(7,minmax(0,1fr)) 8rem;gap:4px;}",
		".txn-cal-week-head, .txn-cal-week-row{display:contents;}",
		".txn-cal-week-total{display:flex;flex-direction:column;",
		".txn-cal-week-spent{font-weight:600;white-space:nowrap;}",
		"border:1px solid var(--border);",
		"background:var(--bg-card);",
		".txn-cal-week-income{font-weight:500;",
		".txn-cal-ghost-status, .txn-cal-ghost-label{display:block;}",
		".txn-cal-ghost-missed{white-space:normal;overflow:visible;text-overflow:clip;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("calendar stylesheet missing %q", want)
		}
	}
}

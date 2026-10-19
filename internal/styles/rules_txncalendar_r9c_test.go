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
		".txn-cal-week-head",
		".txn-cal-week-row",
		"grid-template-columns:repeat(7,minmax(0,1fr)) minmax(5rem,auto)",
		".txn-cal-week-total{display:flex;flex-direction:column;",
		"border:1px solid var(--border);",
		"background:var(--bg-card);",
		".txn-cal-week-income{font-weight:500;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("calendar stylesheet missing %q", want)
		}
	}
}

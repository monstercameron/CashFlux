// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestTV7CCalendarPaddingDaysUseMutedContrastAndTypeScale(t *testing.T) {
	resetSheet()
	registerTV7C()
	css := Build()
	for _, want := range []string{
		".txn-cal-cell.out{opacity:1;}",
		".txn-cal-cell.out .cal-day{color:var(--muted);font-size:var(--fs-meta);font-weight:500;font-variant-numeric:tabular-nums;}",
		".txn-cal-week-moved, .txn-cal-moved, .txn-cal-ghost-status, .txn-cal-ghost-label{font-size:var(--fs-meta);font-weight:500;font-variant-numeric:tabular-nums;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("calendar stylesheet missing %q", want)
		}
	}
}

// SPDX-License-Identifier: MIT

package smartengine

import (
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-D1", d1AutoTodos)
}

const (
	d1MinUncategorized = 5  // surface a categorize chore at this many
	d1RecentDays       = 60 // only count recent uncategorized entries
)

// SMART-D1 — Auto-generated financial to-dos. Turns a detected housekeeping
// backlog (recent uncategorized transactions) into a one-tap to-do, so the chore
// is captured rather than forgotten. Other engines already offer per-insight
// to-dos; D1 owns the cross-cutting "tidy up" chore.
func d1AutoTodos(in Input) []smart.Insight {
	cut := in.Now.AddDate(0, 0, -d1RecentDays)
	var n int
	for _, t := range in.Transactions {
		if t.IsTransfer() || t.CategoryID != "" {
			continue
		}
		if t.Date.Before(cut) || t.Date.After(in.Now) {
			continue
		}
		n++
	}
	if n < d1MinUncategorized {
		return nil
	}
	ins := smart.Insight{
		Feature:  "SMART-D1",
		Page:     smart.PageTodos,
		Key:      "SMART-D1:uncategorized:" + in.Now.Format("2006-01"),
		Severity: smart.SeverityNudge,
	}.
		WithTitle("smart.d1.title", "%s still need a category", plural(int64(n), "transaction")).
		WithDetail("smart.d1.detail",
			"You have %s. Categorizing them keeps your budgets and reports accurate.",
			plural(int64(n), "recent uncategorized transaction")).
		WithAction(smart.Action{Kind: smart.ActionCreateTask,
			TaskTitle: "Categorize " + plural(int64(n), "transaction"),
			TaskNotes: "Review recent uncategorized transactions and assign categories.",
		}.WithLabel("smart.d1.action", "Add a to-do"))
	return []smart.Insight{ins}
}

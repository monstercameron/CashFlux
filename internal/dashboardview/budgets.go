// SPDX-License-Identifier: MIT

// Package dashboardview builds unformatted dashboard models independently of
// browser state, translation and markup.
package dashboardview

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// BudgetLimit is an effective monthly budget limit selected for the hero.
type BudgetLimit struct {
	Name         string
	Spent, Limit money.Money
	Percent      int
	Hot          bool
}

// BudgetSnapshot counts eligible budgets and ranks up to three limits for the
// dashboard. Spending is never summed across overlapping category budgets.
type BudgetSnapshot struct {
	Period           time.Time
	OnPace, Measured int
	Total            int
	Limits           []BudgetLimit
}

// SummarizeBudgets reads the same effective statuses as the budgets screen.
// measuredHot contains a key only when a budget has a live pace measurement;
// its value says whether that pace is hot. Unmeasured budgets do not count as
// on pace. Category names are fallbacks for unnamed budgets.
func SummarizeBudgets(statuses []budgeting.Status, categoryNames map[string]string, measuredHot map[string]bool) BudgetSnapshot {
	out := BudgetSnapshot{}
	rows := make([]BudgetLimit, 0, len(statuses))
	for _, st := range statuses {
		if st.Budget.Period != domain.PeriodMonthly || st.Budget.IsSaving() || st.Error != nil || st.Limit.Amount <= 0 {
			continue
		}
		out.Total++
		if out.Period.IsZero() {
			out.Period = st.Start
		}
		hot, measured := measuredHot[st.Budget.ID]
		if measured {
			out.Measured++
			if !hot {
				out.OnPace++
			}
		}
		name := st.Budget.Name
		if name == "" {
			name = categoryNames[st.Budget.CategoryID]
		}
		rows = append(rows, BudgetLimit{Name: name, Spent: st.Spent, Limit: st.Limit, Percent: st.Percent, Hot: measured && hot})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Hot != rows[j].Hot {
			return rows[i].Hot
		}
		if rows[i].Percent != rows[j].Percent {
			return rows[i].Percent > rows[j].Percent
		}
		return rows[i].Name < rows[j].Name
	})
	if len(rows) > 3 {
		rows = rows[:3]
	}
	out.Limits = rows
	return out
}

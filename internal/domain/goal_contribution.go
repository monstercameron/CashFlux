// SPDX-License-Identifier: MIT

package domain

import (
	"fmt"
	"sort"
)

// RecordContributionChecked appends a contribution and rolls aged-out entries
// into checked monthly amount/count totals. It returns the original goal on any
// arithmetic failure, without modifying its detailed log or monthly history.
// The historical policy of skipping a conflicting monthly currency is retained.
func (g Goal) RecordContributionChecked(c GoalContribution) (Goal, error) {
	original := g
	g.Contributions = append(append([]GoalContribution(nil), g.Contributions...), c)
	if len(g.Contributions) <= MaxGoalContributions {
		return g, nil
	}
	drop := g.Contributions[:len(g.Contributions)-MaxGoalContributions]
	g.Contributions = g.Contributions[len(g.Contributions)-MaxGoalContributions:]
	for _, aged := range drop {
		var err error
		g, err = g.rollUpContributionChecked(aged)
		if err != nil {
			return original, err
		}
	}
	return g, nil
}

func (g Goal) rollUpContributionChecked(c GoalContribution) (Goal, error) {
	key := c.At.Format("2006-01")
	hist := append([]GoalMonthTotal(nil), g.ContributionHistory...)
	for i, h := range hist {
		if h.Month != key {
			continue
		}
		// Preserve the existing skip contract rather than adding different
		// currencies or silently replacing the month's currency.
		if h.Amount.Currency != c.Amount.Currency {
			return g, nil
		}
		total, err := h.Amount.Add(c.Amount)
		if err != nil {
			return g, fmt.Errorf("domain: goal %q contribution history %s: %w", g.ID, key, err)
		}
		const maxCount = int(^uint(0) >> 1)
		if h.Count == maxCount {
			return g, fmt.Errorf("domain: goal %q contribution history %s count overflow", g.ID, key)
		}
		hist[i].Amount, hist[i].Count = total, h.Count+1
		g.ContributionHistory = hist
		return g, nil
	}
	hist = append(hist, GoalMonthTotal{Month: key, Amount: c.Amount, Count: 1})
	sort.Slice(hist, func(i, j int) bool { return hist[i].Month < hist[j].Month })
	if len(hist) > ContributionHistoryCap {
		hist = hist[len(hist)-ContributionHistoryCap:]
	}
	g.ContributionHistory = hist
	return g, nil
}

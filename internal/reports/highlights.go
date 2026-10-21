// SPDX-License-Identifier: MIT

package reports

import "sort"

// BestSavingsMonth returns the earliest bucket with the highest positive net,
// or -1 when no bucket saved anything.
func BestSavingsMonth(flows []PeriodFlow) int {
	best, idx := int64(0), -1
	for i, flow := range flows {
		if net := flow.Net(); net > best {
			best, idx = net, i
		}
	}
	return idx
}

// TopSpendingCuts selects up to n categories whose comparable spending fell by
// more than minimumCut base minor units, largest decrease first. Equal decreases
// retain input order. The source rows are never reordered.
func TopSpendingCuts(rows []CategorySpend, minimumCut int64, n int) []CategorySpend {
	if n <= 0 {
		return nil
	}
	var cuts []CategorySpend
	for _, row := range rows {
		if row.HasDelta && row.Prior > row.Amount && row.Prior-row.Amount > minimumCut {
			cuts = append(cuts, row)
		}
	}
	sort.SliceStable(cuts, func(i, j int) bool { return cuts[i].Prior-cuts[i].Amount > cuts[j].Prior-cuts[j].Amount })
	if len(cuts) > n {
		cuts = cuts[:n]
	}
	return cuts
}

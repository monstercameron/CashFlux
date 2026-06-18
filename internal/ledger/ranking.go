package ledger

import "sort"

// CategoryTotal is one category's summed amount (minor units), used for spend
// breakdowns. CategoryID is empty for uncategorized spend.
type CategoryTotal struct {
	CategoryID string
	Amount     int64
}

// RankSpending sorts category totals by amount, largest first, and collapses the
// long tail: when there are more than n+1 categories it returns the top n plus the
// summed remainder as `other`. With n+1 or fewer categories it returns them all
// (other = 0), so a lone tail category is shown by name rather than as "Other".
// n <= 0 returns every category sorted, with no collapsing. The returned
// CategoryIDs are the real ids (including "" for uncategorized); the caller labels
// the `other` bucket.
func RankSpending(totals map[string]int64, n int) (top []CategoryTotal, other int64) {
	all := make([]CategoryTotal, 0, len(totals))
	for id, amt := range totals {
		all = append(all, CategoryTotal{CategoryID: id, Amount: amt})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Amount > all[j].Amount })
	if n <= 0 || len(all) <= n+1 {
		return all, 0
	}
	for _, c := range all[n:] {
		other += c.Amount
	}
	return all[:n], other
}

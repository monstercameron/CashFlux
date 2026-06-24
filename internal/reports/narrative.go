// SPDX-License-Identifier: MIT

package reports

import (
	"fmt"
	"strings"
)

// SpendingNarrative renders a short, deterministic, plain-English summary of a
// spending-by-category report — the "narrative descriptions" of B21. It is
// purely template-based (no AI), so the same numbers always produce the same
// text. It stays decoupled from formatting and naming via callbacks: format
// renders a base-currency minor-units amount for display (e.g.
// money.FormatAccounting), and name resolves a category id to a label (an empty
// id, or a name callback returning "", falls back to "uncategorized").
//
// rows are expected as returned by SpendingByCategory (sorted largest-first).
// When compared is true and the rows carry deltas, a sentence calls out the
// single biggest mover versus the prior period.
func SpendingNarrative(rows []CategorySpend, compared bool, format func(int64) string, name func(id string) string) string {
	label := func(id string) string {
		l := strings.TrimSpace(name(id))
		if l == "" {
			return "uncategorized"
		}
		return l
	}

	// Categories with actual spend this period (rows may include zero-amount
	// movers that dropped to zero, which shouldn't count toward "across N").
	spent := 0
	for _, r := range rows {
		if r.Amount > 0 {
			spent++
		}
	}
	if spent == 0 {
		return "No spending in this period."
	}

	var b strings.Builder
	total := Total(rows)
	fmt.Fprintf(&b, "You spent %s across %s.", format(total), plural(spent, "category", "categories"))

	// rows are sorted largest-first, so the first positive row is the biggest.
	for _, r := range rows {
		if r.Amount > 0 {
			fmt.Fprintf(&b, " Your biggest expense was %s at %s.", label(r.CategoryID), format(r.Amount))
			break
		}
	}

	if compared {
		if m, ok := topMover(rows); ok {
			verb := "rose"
			if m.DeltaPct < 0 {
				verb = "fell"
			}
			pct := m.DeltaPct
			if pct < 0 {
				pct = -pct
			}
			fmt.Fprintf(&b, " %s %s %d%% to %s versus the prior period.", label(m.CategoryID), verb, pct, format(m.Amount))
		}
	}

	return b.String()
}

// topMover returns the single biggest change versus the prior period, or ok
// false when nothing moved. It delegates to TopMovers so the ranking rule lives
// in one place.
func topMover(rows []CategorySpend) (CategorySpend, bool) {
	if m := TopMovers(rows, 1); len(m) > 0 {
		return m[0], true
	}
	return CategorySpend{}, false
}

// plural renders "n singular" or "n plural" with the count.
func plural(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

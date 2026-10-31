// SPDX-License-Identifier: MIT

package txntable

import (
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"strings"
	"time"
)

// Translator binds catalog lookup at the rendering edge without importing UI state.
type Translator func(string, ...any) string

// LedgerDate formats a UTC calendar date; the current year is explicit for deterministic tests.
func LedgerDate(date time.Time, from, to string, now time.Time) string {
	format := "Jan 2, 2006"
	start, startErr := dateutil.ParseDate(from)
	end, endErr := dateutil.ParseDate(to)
	if startErr == nil && endErr == nil && start.Year() == now.Year() && end.Year() == start.Year() {
		format = "Jan 2"
	}
	return date.Format(format)
}

// CategoryKind selects the income picker only for strictly positive amounts.
func CategoryKind(amount money.Money) domain.CategoryKind {
	if amount.IsPositive() {
		return domain.KindIncome
	}
	return domain.KindExpense
}

// TransferCategoryKey names the account-relative direction of a transfer.
func TransferCategoryKey(amount money.Money) string {
	if amount.Amount < 0 {
		return "r4c.transferTo"
	}
	return "r4c.transferFrom"
}

// NeedsTooltip measures Unicode code points rather than encoded bytes.
func NeedsTooltip(label string, limit int) bool { return len([]rune(label)) > limit }

// OwnerNames resolves owner IDs in order, falling back to unresolved IDs.
func OwnerNames(ids []string, names map[string]string) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		name := strings.TrimSpace(names[id])
		if name == "" {
			name = id
		}
		result = append(result, name)
	}
	return result
}

// OwnerColors preserves one color slot per owner, including unresolved owners.
func OwnerColors(ids []string, colors map[string]string) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		result = append(result, colors[id])
	}
	return result
}

// OwnerColor returns an optional color without indexing outside a short slice.
func OwnerColor(colors []string, index int) string {
	if index < 0 || index >= len(colors) {
		return ""
	}
	return colors[index]
}

// RowClass derives the existing selected, cleared and excluded markers.
func RowClass(selected, cleared, excluded bool) string {
	result := "row txn-ledger-row"
	if selected {
		result += " selected"
	}
	if cleared {
		result += " cleared"
	}
	if excluded {
		result += " txn-excluded"
	}
	return result
}

// SourceClass dims an absent or explicitly unrecorded source.
func SourceClass(source string) string {
	if source == "" || source == "\u2014" {
		return "td-source text-dim"
	}
	return "td-source"
}

// TagWindow limits visible chips and returns their overflow count.
func TagWindow(tags []string) ([]string, int) { return tags[:min(3, len(tags))], max(0, len(tags)-3) }

// CategoryDisplay derives the category cell's accessible name and tooltip precedence.
func CategoryDisplay(category, suggestion, tooltip string, unfiled, transfer bool, t Translator) (string, string) {
	name := category
	title := ""
	if unfiled && suggestion != "" {
		name = t("r5w.suggestedCategory", category, suggestion)
		title = name
	}
	if tooltip != "" {
		title = tooltip
	} else if !transfer && NeedsTooltip(category, 23) {
		title = category
	}
	return name, title
}

// ActionName gives row actions the same contextual accessible name as before.
func ActionName(action, description, date, amount string, t Translator) string {
	return t("txw1.rowAction", action, description, date, amount)
}

// Badge is the one primary attention signal shown beside a description.
type Badge uint8

const (
	// BadgeNone means the row needs no attention marker.
	BadgeNone Badge = iota
	// BadgeSpike marks an unusual amount.
	BadgeSpike
	// BadgeReview marks a queued non-transfer row.
	BadgeReview
	// BadgeDuplicate takes priority over review and spike.
	BadgeDuplicate
)

// DescriptionBadge selects the single primary attention badge in priority order.
func DescriptionBadge(duplicate, review, spike bool) Badge {
	if duplicate {
		return BadgeDuplicate
	}
	if review {
		return BadgeReview
	}
	if spike {
		return BadgeSpike
	}
	return BadgeNone
}

// Info appends a nonblank detail without changing the existing separator convention.
func Info(current, item string) string {
	if strings.TrimSpace(item) == "" {
		return current
	}
	if current == "" {
		return item
	}
	if strings.HasSuffix(strings.TrimSpace(current), ".") {
		return current + " " + item
	}
	return current + ". " + item
}

// BaseCurrency supplies the historical USD default when no preference is set.
func BaseCurrency(base string) string {
	if base == "" {
		return "USD"
	}
	return base
}

// SurfaceIDs chooses placements while retaining the table or calendar's stable identity.
func SurfaceIDs(selected bool, calendar bool) []string {
	ids := []string{"txn-toolbar"}
	if selected {
		ids = append(ids, "txn-bulkbar")
	}
	if calendar {
		ids = append(ids, "txn-calendar")
	} else {
		ids = append(ids, "txn-table")
	}
	return ids
}

// SurfaceClass marks whether the surface hosts selection controls.
func SurfaceClass(selected bool) string {
	if selected {
		return "bento bento-ledger txn-has-selection"
	}
	return "bento bento-ledger"
}

// ExcludeNoticeKey describes the reporting change, independent of persistence.
func ExcludeNoticeKey(exclude bool) string {
	if exclude {
		return "transactions.excludedOne"
	}
	return "transactions.includedOne"
}

// TransferDisplay keeps a fully named movement, falling back to the existing direction label.
func TransferDisplay(category, counterparty string, outgoing bool, t Translator) string {
	if strings.Contains(category, " \u2192 ") {
		return category
	}
	if outgoing {
		return t("r4c.transferTo", counterparty)
	}
	return t("r4c.transferFrom", counterparty)
}

// TransferTooltip keeps an exact supplied detail or names the counterparty.
func TransferTooltip(tooltip, counterparty string, t Translator) string {
	if tooltip != "" {
		return tooltip
	}
	return t("r4c.transferCounterparty", counterparty)
}

// ReviewLabel describes only a non-transfer row actually held in the review queue.
func ReviewLabel(queued, transfer bool, t Translator) string {
	if queued && !transfer {
		return t("acctxn.legendNeedsReview")
	}
	return ""
}

// SPDX-License-Identifier: MIT

package importdraft

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importai"
	"github.com/monstercameron/CashFlux/internal/learntally"
	"github.com/monstercameron/CashFlux/internal/payeealias"
	"github.com/monstercameron/CashFlux/internal/staging"
)

// CategoryTextMatchPercent is the minimum cleaned-description similarity for
// borrowing a category from an earlier transaction.
const CategoryTextMatchPercent = 90

type categoryHistoryEntry struct {
	payee, description string
	categoryID         string
	income             bool
}

type categoryHistoryMatch struct {
	categoryID string
	count      int
}

type categoryHistoryIndex struct {
	entries []categoryHistoryEntry
	names   map[string]string
	kinds   map[string]domain.CategoryKind
}

func newCategoryHistoryIndex(transactions []domain.Transaction, categories []domain.Category) categoryHistoryIndex {
	idx := categoryHistoryIndex{names: make(map[string]string, len(categories)), kinds: make(map[string]domain.CategoryKind, len(categories))}
	for _, category := range categories {
		idx.names[category.ID] = category.Name
		idx.kinds[category.ID] = category.Kind
	}
	for _, txn := range transactions {
		kind, known := idx.kinds[txn.CategoryID]
		if !known || txn.Amount.Amount == 0 || txn.TransferAccountID != "" || len(txn.Splits) != 0 {
			continue
		}
		income := txn.Amount.Amount > 0
		if income != (kind == domain.KindIncome) {
			continue
		}
		payee, description := categoryMatchText(txn.Payee), categoryMatchText(txn.Desc)
		if payee == "" && description == "" {
			continue
		}
		idx.entries = append(idx.entries, categoryHistoryEntry{payee, description, txn.CategoryID, income})
	}
	return idx
}

func categoryMatchText(raw string) string {
	return learntally.NormalizePayee(payeealias.Normalize(raw))
}

// categoryTextSimilarity is the percentage of equal characters after the
// smallest number of edits, or zero when the score cannot reach the threshold.
// The normalizer above removes common processor and trailing reference noise.
func categoryTextSimilarity(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 100
	}
	ra, rb := []rune(a), []rune(b)
	length := max(len(ra), len(rb))
	lengthDifference := len(ra) - len(rb)
	if lengthDifference < 0 {
		lengthDifference = -lengthDifference
	}
	if length > 256 || lengthDifference*100 > (100-CategoryTextMatchPercent)*length {
		return 0
	}
	limit := length * (100 - CategoryTextMatchPercent) / 100
	previous := make([]int, len(rb)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, left := range ra {
		current := make([]int, len(rb)+1)
		current[0] = i + 1
		rowMin := current[0]
		for j, right := range rb {
			cost := 1
			if left == right {
				cost = 0
			}
			current[j+1] = min(previous[j+1]+1, current[j]+1, previous[j]+cost)
			rowMin = min(rowMin, current[j+1])
		}
		if rowMin > limit {
			return 0
		}
		previous = current
	}
	return 100 * (length - previous[len(rb)]) / length
}

func (idx categoryHistoryIndex) match(description string, income bool) ([]categoryHistoryMatch, int) {
	needle := categoryMatchText(description)
	if needle == "" {
		return nil, 0
	}
	best := CategoryTextMatchPercent - 1
	counts := map[string]int{}
	for _, entry := range idx.entries {
		if entry.income != income {
			continue
		}
		score := max(categoryTextSimilarity(needle, entry.payee), categoryTextSimilarity(needle, entry.description))
		if score < CategoryTextMatchPercent || score < best {
			continue
		}
		if score > best {
			clear(counts)
			best = score
		}
		counts[entry.categoryID]++
	}
	if len(counts) == 0 {
		return nil, 0
	}
	matches := make([]categoryHistoryMatch, 0, len(counts))
	for categoryID, count := range counts {
		matches = append(matches, categoryHistoryMatch{categoryID, count})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].count != matches[j].count {
			return matches[i].count > matches[j].count
		}
		return matches[i].categoryID < matches[j].categoryID
	})
	return matches, best
}

func rowIncome(row extract.Row, decimals int) (bool, bool) {
	amount, err := staging.ParseAmount(row.Amount, decimals)
	return amount > 0, err == nil && amount != 0
}

// CategoryHistoryHints shares only aggregate category counts for closely
// matching past descriptions with Smart+. It never sends past amounts, dates,
// accounts, or raw historical descriptions.
func CategoryHistoryHints(rows []extract.Row, transactions []domain.Transaction, categories []domain.Category, decimals int) []importai.CategoryHistoryHint {
	idx := newCategoryHistoryIndex(transactions, categories)
	var hints []importai.CategoryHistoryHint
	requestIndex := 0
	for _, row := range rows {
		if row.Category != "" {
			continue
		}
		income, valid := rowIncome(row, decimals)
		if valid {
			matches, _ := idx.match(row.Description, income)
			total := 0
			for _, match := range matches {
				total += match.count
			}
			for _, match := range matches {
				if len(hints) >= importai.MaxCategoryHistoryHints {
					break
				}
				hints = append(hints, importai.CategoryHistoryHint{
					Index: requestIndex, Category: idx.names[match.categoryID], Count: match.count, Total: total,
				})
			}
		}
		requestIndex++
	}
	return hints
}

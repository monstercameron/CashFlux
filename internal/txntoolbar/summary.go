// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"sort"
	"strings"
)

// SelectedIDs indexes the rows in a generated write draft.
func SelectedIDs(rows []domain.Transaction) map[string]bool {
	result := make(map[string]bool, len(rows))
	for _, row := range rows {
		result[row.ID] = true
	}
	return result
}

// BillAmounts selects ordinary scheduled cashflow without counting transfers.
func BillAmounts(items []bills.Bill) []money.Money {
	result := make([]money.Money, 0, len(items))
	for _, item := range items {
		if item.TransferAccountID == "" {
			result = append(result, item.Amount)
		}
	}
	return result
}

// IssueCount groups currency conversion failures for an explainable summary.
type IssueCount struct {
	Currency string
	Count    int
}

// ConversionIssueCounts returns currency groups in deterministic label order.
func ConversionIssueCounts(issues []txnfilter.ConversionIssue) []IssueCount {
	counts := make(map[string]int)
	for _, issue := range issues {
		code := strings.ToUpper(strings.TrimSpace(issue.Transaction.Amount.Currency))
		if code == "" {
			code = "unknown"
		}
		counts[code]++
	}
	result := make([]IssueCount, 0, len(counts))
	for code, n := range counts {
		result = append(result, IssueCount{Currency: code, Count: n})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Currency < result[j].Currency })
	return result
}

// UniqueFacts removes facts already provided by the table while retaining
// conversion-filter issues and perspective labels in their original order.
func UniqueFacts(facts []string, moved, unpaired, unconverted, excluded int, lens bool) []string {
	drop := 1
	if moved > 0 {
		drop++
		if unpaired > 0 {
			drop++
		}
	}
	if unconverted > 0 {
		drop++
	}
	result := make([]string, 0, len(facts))
	excludedIndex := len(facts) - 1
	if lens {
		excludedIndex--
	}
	for i, fact := range facts {
		if i < drop {
			continue
		}
		if excluded > 0 && i == excludedIndex {
			continue
		}
		result = append(result, fact)
	}
	return result
}

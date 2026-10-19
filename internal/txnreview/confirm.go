// SPDX-License-Identifier: MIT

// Package txnreview applies an explicit human category decision.
package txnreview

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
)

// Confirm confirms a category, replacing any previous split allocation and
// removing every review tag. Other transaction facts and the input stay intact.
func Confirm(t domain.Transaction, categoryID string) domain.Transaction {
	t.Reviewed = true
	t.CategoryID = categoryID
	t.Splits = nil
	tags := make([]string, 0, len(t.Tags))
	for _, tag := range t.Tags {
		if !strings.EqualFold(strings.TrimSpace(tag), reviewqueue.ReviewTag) {
			tags = append(tags, tag)
		}
	}
	if len(tags) == 0 {
		tags = nil
	}
	t.Tags = tags
	return t
}

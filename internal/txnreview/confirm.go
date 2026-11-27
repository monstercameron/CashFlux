// SPDX-License-Identifier: MIT

// Package txnreview applies an explicit human category decision.
package txnreview

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
)

// MarkReviewed records that a person reviewed the transaction and removes
// review tags while preserving its existing CategoryID and Splits intact.
func MarkReviewed(t domain.Transaction) domain.Transaction {
	t.Reviewed = true
	t.Tags = stripReviewTags(t.Tags)
	return t
}

// Confirm confirms a category, replacing any previous split allocation and
// removing every review tag. Other transaction facts and the input stay intact.
func Confirm(t domain.Transaction, categoryID string) domain.Transaction {
	t.Reviewed = true
	t.CategoryID = categoryID
	t.Splits = nil
	t.Tags = stripReviewTags(t.Tags)
	return t
}

func stripReviewTags(in []string) []string {
	tags := make([]string, 0, len(in))
	for _, tag := range in {
		if !strings.EqualFold(strings.TrimSpace(tag), reviewqueue.ReviewTag) {
			tags = append(tags, tag)
		}
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}

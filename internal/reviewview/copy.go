// SPDX-License-Identifier: MIT

package reviewview

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/CashFlux/internal/textutil"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// Text is the caller's translator, supplied so copy decisions remain testable.
type Text func(string, ...any) string

// EmptyCopy preserves the existing review copy decision.
func (text Text) EmptyCopy(scope reviewscope.Scope, queueTotal int, suggestionCount int) (string, string) {
	title, sub := text("review.emptyTitle"), text("review.emptySub")
	if suggestionCount > 0 {
		title = text("r4loan1.suggestionOnlyTitle")
		sub = text("r4loan1.suggestionOnlySub")
	}
	if scope == reviewscope.ScopeCurrentView && queueTotal > 0 {
		title = text("review.emptyViewTitle")
		sub = text("review.emptyViewSub", text.LeftPhrase(queueTotal))
	}
	return title, sub
}

// ReturnLabel preserves the existing review copy decision.
func (text Text) ReturnLabel(scope reviewscope.Scope, idx Index) string {
	if scope == reviewscope.ScopeCurrentView {
		return text("review.returnLabelView", idx.Total)
	}
	return text("review.returnLabelQueue", idx.Total)
}

// WorkingSetPhrase preserves the existing review copy decision.
func (text Text) WorkingSetPhrase(c txnfilter.Criteria) string {
	if q := strings.TrimSpace(c.Text); q != "" {
		return text("review.scopeBySearch", q)
	}
	if n := len(c.PageFilters()); n > 0 {
		return text("review.scopeByFilters", textutil.Plural(n, "filter"))
	}
	return text("review.scopeByView")
}

// ScopeNote preserves the existing review copy decision.
func (text Text) ScopeNote(scope reviewscope.Scope, view, queue int, working string) string {
	outside := queue - view
	if outside < 0 {
		outside = 0
	}
	if scope == reviewscope.ScopeCurrentView {
		if outside == 0 {
			return text("review.scopeViewNoteAll", textutil.Plural(view, "charge"), working)
		}
		return text("review.scopeViewNote", textutil.Plural(view, "charge"), working, text.WaitingPhrase(outside))
	}
	return text("review.scopeQueueNote", textutil.Plural(queue, "charge"), textutil.Plural(view, "charge"), working)
}

// WaitingPhrase preserves the existing review copy decision.
func (text Text) WaitingPhrase(n int) string {
	if n == 1 {
		return text("review.waitingOne")
	}
	return text("review.waitingMany", n)
}

// LeftPhrase preserves the existing review copy decision.
func (text Text) LeftPhrase(n int) string {
	if n == 1 {
		return text("review.leftOne")
	}
	return text("review.leftMany", n)
}

// TierLabel preserves the existing review copy decision.
func (text Text) TierLabel(t Tier) string {
	switch t {
	case TierReady:
		return text("review.tierReady")
	case TierLook:
		return text("review.tierLook")
	}
	return text("review.tierNone")
}

// ReasonLabel preserves the existing review copy decision.
func (text Text) ReasonLabel(r reviewqueue.Reason) string {
	switch r {
	case reviewqueue.ReasonFlagged:
		return text("review.reasonFlagged")
	case reviewqueue.ReasonSplitUnbalanced:
		return text("review.reasonSplit")
	}
	return text("review.reasonUncategorized")
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"
	"time"

	"github.com/monstercameron/CashFlux/internal/reviewqueue"
)

// reviewResolutionsKey stores the user's snooze/dismiss decisions for the review
// queue (C493). It lives in the PRESERVED settings KV alongside the correction
// tally: a decision about what needs your attention is learned behaviour, not
// transaction data, and must survive a dataset reset.
const reviewResolutionsKey = "cashflux:review-resolutions"

// LoadReviewResolutions reads the persisted decisions, pruning any snooze that
// has since expired so the map cannot grow without bound. Returns an empty
// (non-nil) map when the key is absent or unparseable.
func LoadReviewResolutions() reviewqueue.Resolutions {
	raw := SettingKVGet(reviewResolutionsKey)
	if raw == "" {
		return reviewqueue.Resolutions{}
	}
	var rs reviewqueue.Resolutions
	if err := json.Unmarshal([]byte(raw), &rs); err != nil || rs == nil {
		return reviewqueue.Resolutions{}
	}
	rs.Prune(time.Now())
	return rs
}

// SaveReviewResolutions persists the decisions. Silent on marshal error.
func SaveReviewResolutions(rs reviewqueue.Resolutions) {
	b, err := json.Marshal(rs)
	if err != nil {
		return
	}
	SettingKVSet(reviewResolutionsKey, string(b))
}

// SnoozeReviewItem hides a transaction from the queue until `until`, durably —
// the previous skip list was in-memory and lost on reload, so a skipped charge
// re-blocked the head of the queue on the next visit.
func SnoozeReviewItem(txnID string, until time.Time) {
	if txnID == "" {
		return
	}
	rs := LoadReviewResolutions()
	rs.Snooze(txnID, until)
	SaveReviewResolutions(rs)
}

// DismissReviewItem hides a transaction from the queue permanently: the user has
// looked at it and judged it fine as it is.
func DismissReviewItem(txnID string) {
	if txnID == "" {
		return
	}
	rs := LoadReviewResolutions()
	rs.Dismiss(txnID)
	SaveReviewResolutions(rs)
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package uistate — monthlyreview.go persists where somebody is up to in the
// month-end review (AG10).
//
// It lives in localStorage rather than in the dataset, because progress through a
// ritual is about this person on this device rather than about the household's
// money. Syncing it would mean one partner's "not now" silencing the review for
// the other, which is the opposite of what a shared review is for.
package uistate

import (
	"encoding/json"
	"time"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/monthlyreview"
	"github.com/monstercameron/GoWebComponents/v5/state"
)

const (
	monthlyReviewKey       = "cashflux:monthly-review"
	monthlyReviewRevAtomID = "review:revision"
)

// storedReview is the on-disk shape. It mirrors monthlyreview.Progress rather than
// embedding it so a change to the package's internals cannot silently invalidate
// everybody's saved position.
type storedReview struct {
	Month       string   `json:"month"`
	StepIndex   int      `json:"stepIndex"`
	Done        bool     `json:"done,omitempty"`
	DismissedAt string   `json:"dismissedAt,omitempty"`
	Skipped     []string `json:"skipped,omitempty"`
}

// LoadReviewProgress reads the saved position. Anything unreadable returns the
// zero value, which starts the review at its first step — a corrupt stored
// position should cost somebody a restart, never an error.
func LoadReviewProgress() monthlyreview.Progress {
	raw := browserstore.GetString(monthlyReviewKey)
	if raw == "" {
		return monthlyreview.Progress{}
	}
	var s storedReview
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return monthlyreview.Progress{}
	}
	p := monthlyreview.Progress{Month: s.Month, StepIndex: s.StepIndex, Done: s.Done}
	if s.DismissedAt != "" {
		if at, err := time.Parse(time.RFC3339, s.DismissedAt); err == nil {
			p.DismissedAt = at
		}
	}
	for _, step := range s.Skipped {
		p.Skipped = append(p.Skipped, monthlyreview.Step(step))
	}
	return p
}

// SaveReviewProgress writes the position and re-renders the review.
func SaveReviewProgress(p monthlyreview.Progress) {
	s := storedReview{Month: p.Month, StepIndex: p.StepIndex, Done: p.Done}
	if !p.DismissedAt.IsZero() {
		s.DismissedAt = p.DismissedAt.Format(time.RFC3339)
	}
	for _, step := range p.Skipped {
		s.Skipped = append(s.Skipped, string(step))
	}
	if raw, err := json.Marshal(s); err == nil {
		browserstore.Set(monthlyReviewKey, string(raw))
	}
	bumpReview()
}

// UseReviewRevision returns the atom bumped whenever the review's position
// changes, so its component re-renders. Read it in the component.
func UseReviewRevision() state.Atom[int] {
	a := state.UseAtom(monthlyReviewRevAtomID, 0)
	capturedReviewRev = a
	reviewRevCaptured = true
	return a
}

var (
	capturedReviewRev state.Atom[int]
	reviewRevCaptured bool
)

func bumpReview() {
	if reviewRevCaptured {
		capturedReviewRev.Set(capturedReviewRev.Get() + 1)
	}
}

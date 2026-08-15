// SPDX-License-Identifier: MIT

// Package catsuggest ranks every source that can propose a category for a
// transaction and returns one answer with its provenance (C515).
//
// Four things can claim the same charge — a user rule, the correction history
// (internal/learntally), the shipped merchant dictionary (internal/merchantdict),
// and a SMART+ model pass — and without a single ranking the UI has no way to
// order them or to say where an answer came from. Everything downstream reads
// this package rather than the individual sources.
//
// Two rules are load-bearing:
//
//   - A user rule always wins. The user told the app what to do; nothing
//     inferred, and certainly no LLM, may override that.
//   - SMART+ is not consulted here. This package resolves the FREE, local
//     sources; a caller asks the model only for what comes back unresolved,
//     which is what keeps the paid pass small and its cost estimate honest.
package catsuggest

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/learntally"
	"github.com/monstercameron/CashFlux/internal/merchantdict"
)

// Source says which mechanism produced a suggestion. Ordered by strength, so
// callers may compare with < and >.
type Source int

const (
	// SourceNone: nothing could resolve this charge.
	SourceNone Source = iota
	// SourceDictionary: the shipped merchant dictionary recognized the payee.
	SourceDictionary
	// SourceHistoryMerchant: other charges from this merchant were categorized
	// this way before.
	SourceHistoryMerchant
	// SourceHistoryExact: this exact descriptor was categorized this way before.
	SourceHistoryExact
	// SourceRule: a rule the user wrote matched.
	SourceRule
)

// String returns a stable, non-localized identifier for logs and test names.
// User-facing copy lives in the i18n layer, never here.
func (s Source) String() string {
	switch s {
	case SourceDictionary:
		return "dictionary"
	case SourceHistoryMerchant:
		return "history-merchant"
	case SourceHistoryExact:
		return "history-exact"
	case SourceRule:
		return "rule"
	}
	return "none"
}

// Confidence is how much weight a caller should give the suggestion. It drives
// the review inbox's tiering: High is bulk-confirmable, Medium wants a glance,
// Low should never be applied without the user looking.
type Confidence int

const (
	// ConfNone: no suggestion.
	ConfNone Confidence = iota
	// ConfLow: a real signal, but a split or thin one.
	ConfLow
	// ConfMedium: a good default that deserves a glance.
	ConfMedium
	// ConfHigh: safe to confirm in bulk.
	ConfHigh
)

// Suggestion is one resolved proposal plus the evidence behind it. Evidence is
// DATA, never a sentence: the UI formats it through en_*.go so the screenlint
// ratchet stays at zero.
type Suggestion struct {
	// CategoryID is the proposed category. Always a real id from Input.Categories.
	CategoryID string
	// Source is which mechanism produced it.
	Source Source
	// Confidence is how much to trust it.
	Confidence Confidence
	// Count and Total describe history evidence ("6 of 7 charges"). Both zero
	// for rule and dictionary hits.
	Count, Total int
	// Merchant is the canonical merchant name for a dictionary hit, empty otherwise.
	Merchant string
}

// Input is everything the resolver needs. The caller runs the rules engine
// itself and passes the result in, so this package stays free of that
// dependency and remains trivially testable.
type Input struct {
	// Payee and Desc are the raw descriptor fields, exactly as imported.
	Payee, Desc string
	// AmountMinor is the signed amount. Its sign gates which category KINDS may
	// be suggested, so an income charge is never handed an expense category.
	AmountMinor int64
	// RuleCategoryID is the category a user rule matched, or "" for no match.
	RuleCategoryID string
	// Tally is the persisted correction history. May be nil.
	Tally learntally.Tally
	// Threshold is the minimum correction count; <= 0 uses learntally's default.
	Threshold int
	// Categories is the household's real category list, used to validate any
	// proposed id and to resolve a dictionary slug onto a category that exists.
	Categories []domain.Category
}

// descriptor is the string the payee-based sources match on. Payee is preferred;
// Desc is the fallback for imports that only populate a description.
func (in Input) descriptor() string {
	if p := strings.TrimSpace(in.Payee); p != "" {
		return p
	}
	return strings.TrimSpace(in.Desc)
}

// wantKind is the category kind this transaction can accept. A positive amount
// is income, anything else is an expense.
func (in Input) wantKind() domain.CategoryKind {
	if in.AmountMinor > 0 {
		return domain.KindIncome
	}
	return domain.KindExpense
}

// Resolve returns the strongest available suggestion, or ok=false when no local
// source can categorize the charge — which is precisely the set a SMART+ pass
// should be asked about.
func Resolve(in Input) (Suggestion, bool) {
	byID := make(map[string]domain.Category, len(in.Categories))
	for _, c := range in.Categories {
		byID[c.ID] = c
	}

	// 1. A rule the user wrote. Highest confidence, and never overridden.
	if id := strings.TrimSpace(in.RuleCategoryID); id != "" {
		if _, ok := byID[id]; ok {
			return Suggestion{CategoryID: id, Source: SourceRule, Confidence: ConfHigh}, true
		}
	}

	desc := in.descriptor()
	if desc == "" {
		return Suggestion{}, false
	}

	// 2. What the user did with this charge, or this merchant, before.
	if in.Tally != nil {
		if hs, ok := in.Tally.Suggest(desc, in.Threshold); ok {
			if _, exists := byID[hs.CategoryID]; exists {
				s := Suggestion{
					CategoryID: hs.CategoryID,
					Count:      hs.Count,
					Total:      hs.Total,
				}
				if hs.Match == learntally.MatchExact {
					s.Source = SourceHistoryExact
					s.Confidence = ConfMedium
					if hs.Consistent() {
						s.Confidence = ConfHigh
					}
				} else {
					s.Source = SourceHistoryMerchant
					s.Confidence = ConfLow
					if hs.Consistent() {
						s.Confidence = ConfMedium
					}
				}
				// A history that is a coin flip is a signal, not an answer.
				if !hs.Consistent() {
					s.Confidence = ConfLow
				}
				return s, true
			}
		}
	}

	// 3. The shipped dictionary — the only source that works on day one.
	if e, ok := merchantdict.Lookup(desc); ok {
		if id, ok := resolveSlug(e.Category, in.Categories, in.wantKind()); ok {
			return Suggestion{
				CategoryID: id,
				Source:     SourceDictionary,
				Confidence: ConfMedium,
				Merchant:   e.Merchant,
			}, true
		}
	}

	return Suggestion{}, false
}

// resolveSlug maps a dictionary bucket onto a category the household actually
// has, trying the slug's aliases most-specific first. It never invents a
// category: a household with no plausible match gets no suggestion, which is the
// honest outcome rather than a confident wrong one.
func resolveSlug(slug merchantdict.Category, cats []domain.Category, want domain.CategoryKind) (string, bool) {
	if len(cats) == 0 {
		return "", false
	}
	byName := make(map[string]domain.Category, len(cats))
	for _, c := range cats {
		// First definition wins so a deterministic category is picked when a
		// household has duplicate names under different parents.
		k := strings.ToLower(strings.TrimSpace(c.Name))
		if _, dup := byName[k]; !dup {
			byName[k] = c
		}
	}
	for _, alias := range slug.Aliases() {
		c, ok := byName[strings.ToLower(alias)]
		if !ok {
			continue
		}
		if c.Kind != want {
			continue // never hand an expense category to income, or the reverse
		}
		return c.ID, true
	}
	return "", false
}

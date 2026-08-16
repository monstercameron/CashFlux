// SPDX-License-Identifier: MIT

// Package aispend is the running record of what the app's AI features actually
// cost (EC-15).
//
// Every AI surface in CashFlux already shows an estimate BEFORE a call and a token
// line after it, but nothing added them up. So the honest answer to "what is this
// costing me?" was "look at your OpenAI bill", which is both the wrong place and a
// month late. This package keeps the total, split by the feature that spent it and
// by the month it was spent in, so the question can be answered where it is asked.
//
// Two design choices worth naming. The ledger stores TOKENS as the fact and cost
// as a derived estimate, because tokens are what the provider actually reports and
// prices drift — a stored dollar figure would silently become wrong. And a month
// with no recorded spend is absent rather than zero, so an empty meter reads as
// "nothing yet" instead of as a suspiciously round $0.00.
//
// Pure: no syscall/js, no appstate. Table-tested on native Go.
package aispend

import (
	"sort"
	"strings"
	"time"
)

// Entry is one AI call worth recording.
type Entry struct {
	// Feature names what spent it, in the app's own vocabulary ("assistant",
	// "categorize", "receipt"). It is a free string rather than an enum so a new
	// surface can record spend without a change here.
	Feature string
	// At is when the call happened; only its month is kept.
	At time.Time
	// Tokens is the call's total token usage.
	Tokens int
	// CostUSD is the estimated dollar cost, and HasCost is false when the model's
	// pricing is unknown. An unknown cost still records its tokens: "we don't know
	// what this cost" is a fact worth keeping, and dropping the row entirely would
	// understate the total.
	CostUSD float64
	HasCost bool
}

// Bucket is one month's spend on one feature — the shape that persists.
type Bucket struct {
	// Month is the first day of the month, at midnight UTC.
	Month time.Time `json:"month"`
	// Feature is what spent it.
	Feature string `json:"feature"`
	// Calls is how many calls were recorded.
	Calls int `json:"calls"`
	// Tokens is the total token usage.
	Tokens int `json:"tokens"`
	// CostUSD is the accumulated estimate for the calls whose model was priced.
	CostUSD float64 `json:"costUsd,omitempty"`
	// UnpricedCalls is how many of Calls had no known price, so a total can say
	// "at least $X" rather than pretending to completeness it does not have.
	UnpricedCalls int `json:"unpricedCalls,omitempty"`
}

// Ledger is the accumulated record. The zero value is ready to use.
type Ledger struct {
	buckets []Bucket
}

// NewLedger builds a ledger over already-persisted buckets.
func NewLedger(buckets []Bucket) *Ledger {
	return &Ledger{buckets: append([]Bucket(nil), buckets...)}
}

// Buckets returns the persisted form, sorted newest month first and then by
// feature, so the stored order is stable and a diff of the dataset stays readable.
func (l *Ledger) Buckets() []Bucket {
	out := append([]Bucket(nil), l.buckets...)
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Month.Equal(out[j].Month) {
			return out[i].Month.After(out[j].Month)
		}
		return out[i].Feature < out[j].Feature
	})
	return out
}

// Record adds one call to the ledger. A call with no feature name is recorded
// under "other" rather than dropped — spend that happened has to appear somewhere,
// and an unattributed total is still a true total.
func (l *Ledger) Record(e Entry) {
	feature := strings.TrimSpace(e.Feature)
	if feature == "" {
		feature = "other"
	}
	month := MonthOf(e.At)
	for i := range l.buckets {
		if l.buckets[i].Feature == feature && l.buckets[i].Month.Equal(month) {
			l.add(&l.buckets[i], e)
			return
		}
	}
	b := Bucket{Month: month, Feature: feature}
	l.add(&b, e)
	l.buckets = append(l.buckets, b)
}

// add folds one entry into a bucket.
func (l *Ledger) add(b *Bucket, e Entry) {
	b.Calls++
	if e.Tokens > 0 {
		b.Tokens += e.Tokens
	}
	if e.HasCost {
		b.CostUSD += e.CostUSD
		return
	}
	b.UnpricedCalls++
}

// Trim drops buckets older than the given number of months, so the record stays
// bounded in a dataset that syncs between devices. Twelve months is a year of
// history, which is as far back as "what has this cost me?" is ever asked.
func (l *Ledger) Trim(now time.Time, keepMonths int) {
	if keepMonths <= 0 {
		return
	}
	cutoff := MonthOf(now).AddDate(0, -(keepMonths - 1), 0)
	kept := l.buckets[:0]
	for _, b := range l.buckets {
		if !b.Month.Before(cutoff) {
			kept = append(kept, b)
		}
	}
	l.buckets = append([]Bucket(nil), kept...)
}

// FeatureSpend is one feature's share of a month.
type FeatureSpend struct {
	Feature       string
	Calls         int
	Tokens        int
	CostUSD       float64
	UnpricedCalls int
}

// Summary is one month's total and its split by feature.
type Summary struct {
	// Month is the month summarised.
	Month time.Time
	// Calls, Tokens and CostUSD are the month's totals.
	Calls   int
	Tokens  int
	CostUSD float64
	// UnpricedCalls is how many calls had no known price. When it is non-zero the
	// cost is a floor, not a total, and the UI must say so.
	UnpricedCalls int
	// ByFeature is the split, biggest spender first — that is the row somebody is
	// looking for when they open this at all.
	ByFeature []FeatureSpend
	// Recorded reports whether anything at all was spent this month. A month with
	// nothing recorded is not a $0.00 month; it is a month with no data.
	Recorded bool
}

// Complete reports whether every call in the month had a known price, so a caller
// can render "$1.20" rather than "at least $1.20".
func (s Summary) Complete() bool { return s.UnpricedCalls == 0 }

// Month summarises one month.
func (l *Ledger) Month(t time.Time) Summary {
	month := MonthOf(t)
	out := Summary{Month: month}
	for _, b := range l.buckets {
		if !b.Month.Equal(month) {
			continue
		}
		out.Recorded = true
		out.Calls += b.Calls
		out.Tokens += b.Tokens
		out.CostUSD += b.CostUSD
		out.UnpricedCalls += b.UnpricedCalls
		out.ByFeature = append(out.ByFeature, FeatureSpend{
			Feature: b.Feature, Calls: b.Calls, Tokens: b.Tokens,
			CostUSD: b.CostUSD, UnpricedCalls: b.UnpricedCalls,
		})
	}
	sort.SliceStable(out.ByFeature, func(i, j int) bool {
		if out.ByFeature[i].CostUSD != out.ByFeature[j].CostUSD {
			return out.ByFeature[i].CostUSD > out.ByFeature[j].CostUSD
		}
		// Ties break on tokens, then name, so the order never depends on map or
		// slice order and the same data always renders the same way.
		if out.ByFeature[i].Tokens != out.ByFeature[j].Tokens {
			return out.ByFeature[i].Tokens > out.ByFeature[j].Tokens
		}
		return out.ByFeature[i].Feature < out.ByFeature[j].Feature
	})
	return out
}

// Pace is how a month's spending is tracking against a cap.
type Pace int

const (
	// PaceNoCap means no cap is set, so there is nothing to be on or off.
	PaceNoCap Pace = iota
	// PaceComfortable means the month is projected to finish under the cap.
	PaceComfortable
	// PaceTight means the projection is close to the cap — within a tenth of it.
	PaceTight
	// PaceOverPace means the projection exceeds the cap, though the cap has not
	// been reached yet. This is the state worth surfacing: it is the only one
	// where saying something early changes the outcome.
	PaceOverPace
	// PaceExceeded means the cap has already been passed.
	PaceExceeded
	// PaceUnknown means the month contains calls whose price is not known, so the
	// recorded cost is a floor and NO honest verdict against a cap is possible.
	// It is a distinct state rather than an optimistic Comfortable: a household
	// running an unpriced model could be well past its cap while the priced-only
	// total still looks fine, and reporting that as comfortable defeats the cap.
	PaceUnknown
)

// tightBand is how close to the cap a projection has to be to read as tight.
const tightBand = 0.9

// ProjectSpend estimates what the month will total at the current rate, from how
// much of the month has elapsed. ok is false before there is enough of the month
// to project from — extrapolating a year from one day produces a number that is
// alarming and meaningless in equal measure.
func (s Summary) ProjectSpend(now time.Time) (float64, bool) {
	if !s.Recorded || s.CostUSD <= 0 {
		return 0, false
	}
	elapsed, total := monthProgress(now)
	// Under two days in, the sample is too small for the projection to mean
	// anything.
	if elapsed < 2 {
		return 0, false
	}
	return s.CostUSD * float64(total) / float64(elapsed), true
}

// PaceAgainst reports how the month is tracking against a monthly cap in dollars.
func (s Summary) PaceAgainst(capUSD float64, now time.Time) Pace {
	if capUSD <= 0 {
		return PaceNoCap
	}
	if s.CostUSD >= capUSD {
		// Already past the cap on the priced calls alone — true whatever the
		// unpriced ones cost, so this verdict is safe to give either way.
		return PaceExceeded
	}
	if !s.Complete() {
		// Below the cap on what can be priced, but some calls could not be priced
		// at all. The real total is unknown and could be anywhere above this, so
		// the honest answer is that there isn't one.
		return PaceUnknown
	}
	projected, ok := s.ProjectSpend(now)
	if !ok {
		return PaceComfortable
	}
	switch {
	case projected > capUSD:
		return PaceOverPace
	case projected > capUSD*tightBand:
		return PaceTight
	default:
		return PaceComfortable
	}
}

// monthProgress returns how many days of the month have elapsed (counting today)
// and how many the month has in total.
func monthProgress(now time.Time) (elapsed, total int) {
	start := MonthOf(now)
	next := start.AddDate(0, 1, 0)
	total = int(next.Sub(start).Hours() / 24)
	elapsed = now.Day()
	if elapsed > total {
		elapsed = total
	}
	return elapsed, total
}

// MonthOf normalises a time to the first instant of its month in UTC, which is the
// key buckets are stored under. UTC rather than local so a dataset that syncs
// across time zones does not split one month into two.
func MonthOf(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
}

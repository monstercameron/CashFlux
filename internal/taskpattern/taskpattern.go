// SPDX-License-Identifier: MIT

// Package taskpattern spots a chore somebody keeps re-creating by hand, and who
// keeps doing it (EC-14).
//
// # What it reads, and what it cannot
//
// A task records no completion time — only its status and its due date — so the
// rhythm is read from the DUE DATES of completed tasks sharing a title. That is
// a fair proxy for a scheduled chore ("bins out, due the 1st, done three times")
// and no proxy at all for an undated one, which is why undated tasks are
// ignored rather than guessed at. A DoneAt stamp on Task would widen this to
// chores nobody dates; the same change Transaction.ClearedAt made for EC-4.
//
// # Suggesting an owner is where this can do harm
//
// If two people alternate a chore, naming one of them is the app taking sides in
// a household, and being wrong costs more than staying quiet. An owner is
// suggested only when one person has clearly been doing it.
package taskpattern

import (
	"sort"
	"strings"
	"time"
)

// MinOccurrences is how many completions establish a habit.
//
// Three. Two is a coincidence with an even chance of happening by itself, and
// offering to automate a coincidence is how an app ends up creating chores
// nobody asked for.
const MinOccurrences = 3

// MaxGapDriftDays is how much the intervals may disagree and still count as a
// rhythm.
//
// Seven days. A monthly chore done on the 3rd, the 1st and the 5th is monthly; a
// stricter rule would reject every real household, and a looser one would call
// anything a cadence.
const MaxGapDriftDays = 7

// OwnerMajority is the share of completions one person must have before they are
// suggested as the owner.
//
// Three quarters. A simple majority in a two-person household is one extra turn,
// which is not evidence of whose job it is.
const OwnerMajority = 0.75

// Cadence is the rhythm a repeated chore runs on.
type Cadence string

const (
	CadenceNone      Cadence = ""
	CadenceWeekly    Cadence = "weekly"
	CadenceFortnight Cadence = "fortnightly"
	CadenceMonthly   Cadence = "monthly"
	CadenceQuarterly Cadence = "quarterly"
)

// Occurrence is one completed instance of a chore.
type Occurrence struct {
	Title    string
	Due      time.Time
	MemberID string
}

// Suggestion is a chore worth making recurring.
type Suggestion struct {
	Title   string
	Cadence Cadence
	// Count is how many completions it rests on, and MedianGapDays the interval
	// they ran at — the evidence, so the offer can be argued with.
	Count          int
	MedianGapDays  int
	// OwnerID is the member who has clearly been doing it, or "" when nobody has
	// clearly been doing it. Empty is a real answer here, not a missing one.
	OwnerID string
	// OwnerShare is that member's share of the completions, for a caller that
	// wants to say "Priya has done 4 of the last 5".
	OwnerShare float64
	OwnerCount int
}

// Detect finds the chores worth offering to automate.
//
// alreadyRecurring holds the titles that are already set up, so the app never
// offers to automate something it is already automating — the fastest way to
// make a suggestion feel like noise.
func Detect(done []Occurrence, alreadyRecurring map[string]bool) []Suggestion {
	byTitle := map[string][]Occurrence{}
	for _, o := range done {
		key := normalizeTitle(o.Title)
		if key == "" || o.Due.IsZero() {
			continue // an undated chore establishes no rhythm
		}
		if alreadyRecurring[key] {
			continue
		}
		byTitle[key] = append(byTitle[key], o)
	}

	titles := make([]string, 0, len(byTitle))
	for k := range byTitle {
		titles = append(titles, k)
	}
	sort.Strings(titles) // deterministic output for the same data

	var out []Suggestion
	for _, key := range titles {
		occ := byTitle[key]
		if len(occ) < MinOccurrences {
			continue
		}
		sort.Slice(occ, func(i, j int) bool { return occ[i].Due.Before(occ[j].Due) })
		gaps := make([]int, 0, len(occ)-1)
		for i := 1; i < len(occ); i++ {
			gaps = append(gaps, int(occ[i].Due.Sub(occ[i-1].Due).Hours()/24))
		}
		med := medianInt(gaps)
		if !regular(gaps, med) {
			continue // things done three times at random are not a cadence
		}
		cad := cadenceFor(med)
		if cad == CadenceNone {
			continue
		}
		s := Suggestion{
			Title: occ[len(occ)-1].Title, Cadence: cad,
			Count: len(occ), MedianGapDays: med,
		}
		s.OwnerID, s.OwnerCount, s.OwnerShare = dominantOwner(occ)
		out = append(out, s)
	}
	return out
}

// regular reports whether the intervals agree closely enough to be a rhythm.
func regular(gaps []int, med int) bool {
	for _, g := range gaps {
		if g < 0 {
			return false
		}
		if diff := g - med; diff > MaxGapDriftDays || diff < -MaxGapDriftDays {
			return false
		}
	}
	return true
}

// cadenceFor names an interval, or reports none when it matches no rhythm worth
// offering. A chore done every 200 days is real, but "offer to make this
// recurring every 200 days" is not a sentence anybody wants.
func cadenceFor(days int) Cadence {
	switch {
	case days >= 5 && days <= 9:
		return CadenceWeekly
	case days >= 12 && days <= 17:
		return CadenceFortnight
	case days >= 26 && days <= 35:
		return CadenceMonthly
	case days >= 80 && days <= 100:
		return CadenceQuarterly
	}
	return CadenceNone
}

// dominantOwner returns the member who has clearly been doing this, if anyone
// has. Naming one of two people who alternate is the app taking sides.
func dominantOwner(occ []Occurrence) (string, int, float64) {
	counts := map[string]int{}
	for _, o := range occ {
		if o.MemberID != "" {
			counts[o.MemberID]++
		}
	}
	bestID, bestN := "", 0
	ids := make([]string, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	sort.Strings(ids) // a tie must not depend on map order
	for _, id := range ids {
		if counts[id] > bestN {
			bestID, bestN = id, counts[id]
		}
	}
	if bestID == "" {
		return "", 0, 0
	}
	share := float64(bestN) / float64(len(occ))
	if share < OwnerMajority {
		return "", 0, 0
	}
	return bestID, bestN, share
}

func medianInt(v []int) int {
	if len(v) == 0 {
		return 0
	}
	s := append([]int(nil), v...)
	sort.Ints(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

// normalizeTitle collapses the incidental differences between two typings of the
// same chore — case and surrounding space. It deliberately does no stemming or
// fuzzy matching: "Pay the gas bill" and "Pay the water bill" are different
// chores, and an engine loose enough to merge them would offer to automate a
// chore that does not exist.
func normalizeTitle(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

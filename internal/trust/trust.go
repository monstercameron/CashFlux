// SPDX-License-Identifier: MIT

// Package trust scores how much a calculated figure can be relied on, and says
// exactly why (WF4).
//
// Every other feature in this app produces conclusions — a runway, a payoff
// date, a projected net worth — from inputs the household supplied, forgot to
// supply, or last updated months ago. The conclusions all render identically. A
// payoff date computed from a real APR and one computed from a blank APR look
// the same on screen, and the second is a guess wearing the first one's clothes.
//
// # Why a reason, never a bare score
//
// "Confidence: 62%" is the thing this package exists NOT to do. A score with no
// reason cannot be acted on, cannot be argued with, and cannot be improved — the
// reader learns only that the app is unsure, not what would make it sure. Every
// assessment here names the specific inputs that weakened it, so the next step
// is obvious: go and fill that field in.
package trust

import (
	"sort"
	"strings"
)

// Level is how far a figure can be trusted.
type Level string

const (
	// LevelSolid means every input the figure needs is present and current.
	LevelSolid Level = "solid"
	// LevelQualified means the figure is usable but rests on something stale or
	// assumed.
	LevelQualified Level = "qualified"
	// LevelUnreliable means a REQUIRED input is missing entirely.
	//
	// Distinct from qualified on purpose. "Computed from a balance you last
	// updated in March" and "computed with no interest rate at all" are different
	// kinds of wrong: the first is roughly right, the second is arithmetic over a
	// number nobody supplied, and collapsing them into one warning teaches people
	// to ignore both.
	LevelUnreliable Level = "unreliable"
)

// StaleDays is when a supplied figure stops counting as current.
//
// Sixty days. Long enough that a household updating balances monthly is never
// nagged, short enough that a quarter-old balance cannot silently anchor a
// projection about next year.
const StaleDays = 60

// Input is one thing a conclusion depends on.
type Input struct {
	// Name is what the reader would call it — "the card's APR", "your savings
	// balance". Used verbatim in the reasons, so it must read as a noun phrase.
	Name string
	// Required marks an input the conclusion cannot be computed honestly without.
	// A missing OPTIONAL input qualifies the figure; a missing required one
	// invalidates it.
	Required bool
	// Missing says the household never supplied it.
	Missing bool
	// AgeDays is how long since it was last updated; zero means unknown age,
	// which is NOT treated as fresh — an input with no known age is exactly the
	// kind of thing that quietly rots.
	AgeDays int
	// Assumed marks a value the app supplied itself (a default rate, a convention)
	// rather than one the household stated.
	Assumed bool
}

// Stale reports whether this input is past its shelf life.
func (i Input) Stale() bool { return i.AgeDays > StaleDays }

// Assessment is a figure's reliability and the reasons for it.
type Assessment struct {
	Level Level
	// MissingRequired, Stale and Assumed name the inputs behind the level, each
	// sorted, so a surface can say "we do not have the APR" rather than "62%".
	MissingRequired, MissingOptional, Stale, Assumed []string
}

// Trustworthy reports whether the figure is safe to lean on without caveat.
func (a Assessment) Trustworthy() bool { return a.Level == LevelSolid }

// Reasons is every weakening input, most serious first.
//
// Ordered missing-required, then stale, then assumed, then missing-optional:
// that is the order in which they change what the reader should DO. A missing
// required input means go and enter it; a stale one means go and refresh it; an
// assumption means decide whether you agree with it.
func (a Assessment) Reasons() []string {
	out := make([]string, 0, len(a.MissingRequired)+len(a.Stale)+len(a.Assumed)+len(a.MissingOptional))
	out = append(out, a.MissingRequired...)
	out = append(out, a.Stale...)
	out = append(out, a.Assumed...)
	out = append(out, a.MissingOptional...)
	return out
}

// Assess scores a conclusion from its inputs.
//
// An empty input list reports LevelUnreliable, not solid. A figure whose
// dependencies nobody declared has not been shown to be trustworthy — it has
// merely not been examined, and defaulting that to "solid" would make the whole
// package a rubber stamp.
func Assess(inputs []Input) Assessment {
	a := Assessment{Level: LevelSolid}
	if len(inputs) == 0 {
		a.Level = LevelUnreliable
		return a
	}
	for _, in := range inputs {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			continue
		}
		switch {
		case in.Missing && in.Required:
			a.MissingRequired = append(a.MissingRequired, name)
		case in.Missing:
			a.MissingOptional = append(a.MissingOptional, name)
		case in.Stale():
			a.Stale = append(a.Stale, name)
		}
		if in.Assumed && !in.Missing {
			a.Assumed = append(a.Assumed, name)
		}
	}
	sort.Strings(a.MissingRequired)
	sort.Strings(a.MissingOptional)
	sort.Strings(a.Stale)
	sort.Strings(a.Assumed)

	switch {
	case len(a.MissingRequired) > 0:
		a.Level = LevelUnreliable
	case len(a.Stale) > 0 || len(a.Assumed) > 0 || len(a.MissingOptional) > 0:
		a.Level = LevelQualified
	}
	return a
}

// Worst returns the least trustworthy of several assessments.
//
// A dashboard figure built from three others is only as good as the worst of
// them, and averaging confidence would let two solid inputs hide one that is
// missing entirely — which is precisely the failure this package exists to
// prevent.
func Worst(as ...Assessment) Assessment {
	rank := map[Level]int{LevelSolid: 0, LevelQualified: 1, LevelUnreliable: 2}
	out := Assessment{Level: LevelSolid}
	found := false
	for _, a := range as {
		if !found || rank[a.Level] > rank[out.Level] {
			out, found = a, true
		}
	}
	if !found {
		out.Level = LevelUnreliable
	}
	return out
}

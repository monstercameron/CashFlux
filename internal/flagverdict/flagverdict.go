// SPDX-License-Identifier: MIT

// Package flagverdict models what happens after somebody judges a flag
// (WF-SM1's second clause): one-time, expected, wrong category, new normal, or
// investigate.
//
// # A verdict is not a synonym for "hide"
//
// The obvious build is five buttons that all dismiss the flag and differ only in
// the label stored alongside. That would be worse than one Dismiss button,
// because it would look like the app understood the answer while doing the same
// thing with all five.
//
// Two of the five must keep the flag on screen. "Wrong category" says the
// SPENDING is real and the DATA is wrong — hiding it leaves the miscategorised
// money exactly where it was while the reader believes they dealt with it. And
// "investigate" is the opposite of hiding: it is somebody saying they are not
// finished looking.
//
// Of the three that do hide, they hide different things for different lengths of
// time, because they are different claims: a one-off is a statement about one
// month, an expected charge is a statement about a recurring event, and a new
// normal is a statement that the baseline is out of date and will catch up on its
// own.
//
// The package holds no storage and no clock authority — callers pass the time in,
// so suppression is testable without waiting a year.
package flagverdict

import (
	"encoding/json"
	"strings"
	"time"
)

// Verdict is a judgement recorded against a flag.
type Verdict string

const (
	// OneTime: it happened, it was unusual, and it is not going to repeat.
	OneTime Verdict = "one_time"
	// Expected: this is a known recurring event, not a surprise.
	Expected Verdict = "expected"
	// WrongCategory: the money is real but filed in the wrong place.
	WrongCategory Verdict = "wrong_category"
	// NewNormal: spending genuinely moved and the baseline has not caught up.
	NewNormal Verdict = "new_normal"
	// Investigate: not resolved — somebody is still looking.
	Investigate Verdict = "investigate"
)

// Verdicts lists the recordable verdicts in the order a surface should offer
// them: the two conclusive answers first, then the two that change the data or
// the baseline, then the one that resolves nothing.
func Verdicts() []Verdict {
	return []Verdict{OneTime, Expected, WrongCategory, NewNormal, Investigate}
}

// Valid reports whether v is a verdict this package knows.
func (v Verdict) Valid() bool {
	switch v {
	case OneTime, Expected, WrongCategory, NewNormal, Investigate:
		return true
	}
	return false
}

// Scope is what a hiding verdict hides.
type Scope string

const (
	// ScopeNone: nothing is hidden.
	ScopeNone Scope = "none"
	// ScopeOccurrence: this one flag, in this one period. The same category
	// flagging again next month is new information and must be shown.
	ScopeOccurrence Scope = "occurrence"
	// ScopeRecurring: this flag wherever it recurs, until the suppression expires.
	ScopeRecurring Scope = "recurring"
)

// Follow is the work a verdict leaves behind.
type Follow string

const (
	// FollowNone: the verdict settles it.
	FollowNone Follow = "none"
	// FollowRecategorize: the spending needs refiling — the flag was right.
	FollowRecategorize Follow = "recategorize"
	// FollowTrack: still open, and should stay visible as such.
	FollowTrack Follow = "track"
)

// ExpectedDays is how long "expected" silences a recurring flag.
//
// Four hundred days — just past a year, so an annual charge judged expected in
// March is still expected the following March, and one judged expected because
// of a job or a lease is re-asked once that year is out. A suppression with no
// end is how a real change goes unnoticed: the reason for it expires long before
// the silence would.
const ExpectedDays = 400

// NewNormalDays is how long "new normal" silences a flag.
//
// Ninety days, which is exactly the trailing window the detector averages over.
// After it, a baseline that has absorbed the new level stops flagging by itself
// and the suppression was never needed; one that is STILL flagging is telling
// you the level moved again, which is news. Either way the right thing is to
// stop suppressing.
const NewNormalDays = 90

// Outcome is what a verdict does to the flag it was recorded against.
type Outcome struct {
	// Hides is whether the flag stops being shown.
	Hides bool
	// Scope is what Hides applies to. ScopeNone when Hides is false.
	Scope Scope
	// Days is how long the suppression lasts. Zero with ScopeOccurrence means
	// "this occurrence only" — not "forever" and not "no time at all".
	Days int
	// Follow is the work left behind.
	Follow Follow
}

// Effect returns what a verdict does. An unknown verdict hides nothing: the app
// does not silence a flag on the strength of a judgement it cannot interpret.
func Effect(v Verdict) Outcome {
	switch v {
	case OneTime:
		return Outcome{Hides: true, Scope: ScopeOccurrence, Follow: FollowNone}
	case Expected:
		return Outcome{Hides: true, Scope: ScopeRecurring, Days: ExpectedDays, Follow: FollowNone}
	case NewNormal:
		return Outcome{Hides: true, Scope: ScopeRecurring, Days: NewNormalDays, Follow: FollowNone}
	case WrongCategory:
		// The flag was RIGHT — the money is real and filed in the wrong place.
		// Hiding it would leave the miscategorised spending exactly where it was.
		return Outcome{Scope: ScopeNone, Follow: FollowRecategorize}
	case Investigate:
		return Outcome{Scope: ScopeNone, Follow: FollowTrack}
	}
	return Outcome{Scope: ScopeNone, Follow: FollowNone}
}

// Record is one judgement, kept so the app can say what was decided and when
// rather than silently behaving differently.
type Record struct {
	// Key identifies the flag. A caller building an occurrence-scoped key
	// includes the period in it; a recurring key does not.
	Key string `json:"key"`
	// Subject is the recurring thing the flag was about (a category), so an
	// occurrence verdict and a recurring one can be told apart on read.
	Subject string  `json:"subject,omitempty"`
	Verdict Verdict `json:"verdict"`
	// At is when the judgement was recorded.
	At time.Time `json:"at"`
	// Note is anything the user typed. Never required.
	Note string `json:"note,omitempty"`
}

// Memory is the set of recorded judgements. The zero value is usable.
type Memory struct {
	Records []Record `json:"records,omitempty"`
}

// Load parses a Memory from JSON. Empty or malformed input yields an empty
// memory: a memory that cannot be read means nothing was decided, which shows
// every flag — the failure that costs the least.
func Load(raw string) Memory {
	var m Memory
	if strings.TrimSpace(raw) == "" {
		return m
	}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return Memory{}
	}
	m.Records = valid(m.Records)
	return m
}

// Marshal encodes the memory for persistence.
func (m Memory) Marshal() string {
	b, err := json.Marshal(Memory{Records: valid(m.Records)})
	if err != nil {
		return "{}"
	}
	return string(b)
}

// valid drops records this package cannot act on rather than carrying them
// forward as data that looks like a decision.
func valid(in []Record) []Record {
	out := make([]Record, 0, len(in))
	for _, r := range in {
		r.Key = strings.TrimSpace(r.Key)
		if r.Key == "" || !r.Verdict.Valid() {
			continue
		}
		out = append(out, r)
	}
	return out
}

// Record stores a judgement, replacing any earlier one for the same key —
// somebody who changes their mind has changed their mind, and two verdicts on
// one flag is not a history worth keeping at the cost of an ambiguous read.
func (m Memory) Record(r Record) Memory {
	if strings.TrimSpace(r.Key) == "" || !r.Verdict.Valid() {
		return m
	}
	out := make([]Record, 0, len(m.Records)+1)
	for _, e := range m.Records {
		if e.Key != r.Key {
			out = append(out, e)
		}
	}
	return Memory{Records: append(out, r)}
}

// Forget drops the judgement for a key, so a flag silenced by mistake can be
// brought back. Every suppression this package grants has to be reversible, or
// one wrong tap permanently blinds the app to a category.
func (m Memory) Forget(key string) Memory {
	out := make([]Record, 0, len(m.Records))
	for _, e := range m.Records {
		if e.Key != key {
			out = append(out, e)
		}
	}
	return Memory{Records: out}
}

// For returns the judgement recorded against a key.
func (m Memory) For(key string) (Record, bool) {
	for _, e := range m.Records {
		if e.Key == key {
			return e, true
		}
	}
	return Record{}, false
}

// Suppressed reports whether a flag should be hidden now, and why not when it
// should not be.
//
// occurrenceKey identifies this period's flag; subject identifies the recurring
// thing (the category). A one-off judged last month suppresses only last month's
// key, so this month's flag appears; an expected or new-normal judgement on the
// subject suppresses until it expires.
func (m Memory) Suppressed(occurrenceKey, subject string, now time.Time) bool {
	if r, ok := m.For(occurrenceKey); ok {
		if eff := Effect(r.Verdict); eff.Hides && eff.Scope == ScopeOccurrence {
			return true
		}
	}
	if subject == "" {
		return false
	}
	for _, r := range m.Records {
		if r.Subject != subject {
			continue
		}
		eff := Effect(r.Verdict)
		if !eff.Hides || eff.Scope != ScopeRecurring {
			continue
		}
		if now.Before(r.At.AddDate(0, 0, eff.Days)) {
			return true
		}
	}
	return false
}

// Open lists the judgements that left work behind — the flags somebody said were
// miscategorised or still being looked into. They are the reason the two
// non-hiding verdicts exist, and a surface that records them without ever
// showing them back has quietly turned them into a Dismiss button.
func (m Memory) Open() []Record {
	out := make([]Record, 0, len(m.Records))
	for _, r := range m.Records {
		if Effect(r.Verdict).Follow != FollowNone {
			out = append(out, r)
		}
	}
	return out
}

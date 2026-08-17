// SPDX-License-Identifier: MIT

// Package casequeue merges the several signals one problem produces into a
// single case (E2).
//
// A bill that did not get paid currently generates four separate things: a
// notification, an entry in the review queue for the unmatched payment when it
// does arrive, a to-do the user or a workflow created, and an insight on the
// dashboard. Four surfaces each report a fragment of one situation, and the
// reader has to work out that they are the same situation — then dismiss it four
// times, in four places, in four different ways.
//
// The contract this package exists to satisfy is "one issue = ONE finding across
// Dashboard / Notifications / Insights / Smart / To-do". Three things follow:
//
//   - Signals are grouped by the SUBJECT they are about, not by their kind. Two
//     signals about account a-checking's overdraft are one case; two
//     notifications about different accounts are two.
//   - A case ranks by what acting on it is worth, not by how loud its loudest
//     signal was. Severity says how bad; actionability says whether anything can
//     be done, which is what decides where it goes in a queue.
//   - A case whose trigger has cleared closes ITSELF. The alternative is a queue
//     that only ever grows, because the common way a problem ends is that the
//     underlying situation resolves and nobody goes back to tick it off.
//
// Pure Go: grouping, ranking and closure only. The caller supplies the signals
// and applies whatever the case proposes.
package casequeue

import (
	"sort"
	"strings"
)

// SignalKind names where a signal came from.
type SignalKind string

const (
	// SignalNotification is an entry in the notification feed.
	SignalNotification SignalKind = "notification"
	// SignalTask is a to-do.
	SignalTask SignalKind = "task"
	// SignalInsight is a Smart finding.
	SignalInsight SignalKind = "insight"
	// SignalReview is an item in the review queue.
	SignalReview SignalKind = "review"
	// SignalContradiction is a finding from the contradiction detector (E3).
	SignalContradiction SignalKind = "contradiction"
)

// Severity ranks how bad a signal's situation is.
type Severity int

const (
	// SeverityInfo is worth knowing.
	SeverityInfo Severity = iota
	// SeverityWarning is worth doing something about.
	SeverityWarning
	// SeverityCritical is costing money or about to.
	SeverityCritical
)

// Signal is one surface's report about something.
type Signal struct {
	Kind SignalKind
	// ID is the signal's own id on its surface, so a case can dismiss it there.
	ID string
	// SubjectKind and SubjectID name what the signal is ABOUT — the account, the
	// bill, the budget. This is the join key: signals about the same subject are
	// the same case regardless of which surface raised them.
	SubjectKind string
	SubjectID   string
	// Title is the signal's own one-line description.
	Title    string
	Severity Severity
	// Actionable says whether this signal has something the user can DO. A case
	// with nothing actionable is information; one with an action is work.
	Actionable bool
	// Cleared says the situation this signal reported has resolved. A signal that
	// reports itself cleared lets its case close without anyone ticking it off.
	Cleared bool
	// AmountMinor is what the situation is worth, where it has a figure.
	AmountMinor int64
	HasAmount   bool
}

// SubjectKey is a signal's join key.
func (s Signal) SubjectKey() string {
	return strings.ToLower(strings.TrimSpace(s.SubjectKind)) + ":" +
		strings.TrimSpace(s.SubjectID)
}

// Case is one situation, with every signal that reported it.
type Case struct {
	// Key is the subject key; it is stable across recomputes, so a dismissal
	// sticks and the case survives its signals changing.
	Key         string
	SubjectKind string
	SubjectID   string
	// Title is the case headline: the highest-severity signal's title, because
	// the worst thing about a situation is what should name it.
	Title string
	// Severity is the highest of its signals'.
	Severity Severity
	// Signals are everything that reported this situation, most severe first.
	Signals []Signal
	// AmountMinor is the largest figure any signal attached, which is the honest
	// summary: the same money reported by three surfaces is one amount, and
	// SUMMING them would triple it.
	AmountMinor int64
	HasAmount   bool
	// Actionable is true when at least one signal offers something to do.
	Actionable bool
	// Closed is true when EVERY signal reports its situation cleared. One
	// outstanding signal keeps the case open — a case that closed on the first
	// clearing would hide the parts that had not.
	Closed bool
}

// SurfaceCount is how many distinct surfaces reported this case. It is the
// measure of how much noise the merge removed, and the number worth showing:
// "this appeared in 4 places" is what tells a reader why one row is enough.
func (c Case) SurfaceCount() int {
	seen := map[SignalKind]bool{}
	for _, s := range c.Signals {
		seen[s.Kind] = true
	}
	return len(seen)
}

// Has reports whether the case includes a signal of a kind.
func (c Case) Has(k SignalKind) bool {
	for _, s := range c.Signals {
		if s.Kind == k {
			return true
		}
	}
	return false
}

// IDsFor returns the signal ids of one kind, so a surface can dismiss its own
// entries when the case is resolved.
func (c Case) IDsFor(k SignalKind) []string {
	var out []string
	for _, s := range c.Signals {
		if s.Kind == k && s.ID != "" {
			out = append(out, s.ID)
		}
	}
	return out
}

// Rank scores a case for queue order.
//
// Actionability outranks severity, which is the decision that makes the queue
// useful rather than merely sorted: a critical situation with nothing to do
// about it belongs BELOW a warning the reader can clear in one click, because a
// queue is a list of work and an unactionable item at the top is a wall. The
// amount breaks ties, and the surface count breaks those — a situation four
// surfaces noticed is more likely to be real than one only a heuristic saw.
func (c Case) Rank() int {
	r := int(c.Severity) * 10
	if c.Actionable {
		r += 100
	}
	if c.HasAmount && c.AmountMinor > 0 {
		r++
	}
	if c.SurfaceCount() > 1 {
		r++
	}
	return r
}

// Build groups signals into cases.
//
// Signals with no subject are each their own case rather than being lumped
// together: an unattributed signal is not evidence that two unattributed signals
// are about the same thing, and merging them would produce a case that means
// nothing. Closed cases are included — the caller decides whether to show a case
// that has resolved itself, and a self-closing case that vanished silently would
// give no sign the work was ever done.
func Build(signals []Signal) []Case {
	order := []string{}
	groups := map[string][]Signal{}
	for i, s := range signals {
		key := s.SubjectKey()
		if strings.TrimSpace(s.SubjectID) == "" {
			key = "unattributed:" + string(s.Kind) + ":" + s.ID + ":" + itoa(i)
		}
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], s)
	}
	out := make([]Case, 0, len(order))
	for _, key := range order {
		out = append(out, buildOne(key, groups[key]))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Rank() != out[j].Rank() {
			return out[i].Rank() > out[j].Rank()
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func buildOne(key string, sigs []Signal) Case {
	c := Case{Key: key, Closed: true}
	if len(sigs) > 0 {
		c.SubjectKind, c.SubjectID = sigs[0].SubjectKind, sigs[0].SubjectID
	}
	sorted := make([]Signal, len(sigs))
	copy(sorted, sigs)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Severity > sorted[j].Severity })
	c.Signals = sorted

	for _, s := range sorted {
		if s.Severity > c.Severity {
			c.Severity = s.Severity
		}
		if s.Actionable {
			c.Actionable = true
		}
		if !s.Cleared {
			c.Closed = false
		}
		// The LARGEST figure, not the sum: the same overdraft reported by three
		// surfaces is one amount, and adding them would triple it.
		if s.HasAmount && (!c.HasAmount || abs64(s.AmountMinor) > abs64(c.AmountMinor)) {
			c.AmountMinor, c.HasAmount = s.AmountMinor, true
		}
	}
	// The worst thing about a situation is what should name it.
	if len(sorted) > 0 {
		c.Title = sorted[0].Title
	}
	return c
}

// Open returns the cases that have not resolved themselves.
func Open(cases []Case) []Case {
	out := make([]Case, 0, len(cases))
	for _, c := range cases {
		if !c.Closed {
			out = append(out, c)
		}
	}
	return out
}

// SelfClosed returns the cases whose situations have all resolved — the ones a
// caller should tidy away on the surfaces that raised them.
func SelfClosed(cases []Case) []Case {
	out := make([]Case, 0, len(cases))
	for _, c := range cases {
		if c.Closed {
			out = append(out, c)
		}
	}
	return out
}

// Top returns the first n open cases, which is what a "today's top three" strip
// wants. A non-positive n returns nothing rather than everything — a caller
// asking for zero items means zero.
func Top(cases []Case, n int) []Case {
	if n <= 0 {
		return nil
	}
	open := Open(cases)
	if len(open) > n {
		open = open[:n]
	}
	return open
}

// Summary describes a queue in one line.
type Summary struct {
	// Cases is how many open cases there are.
	Cases int
	// Signals is how many raw signals they came from. The gap between the two is
	// the noise the merge removed.
	Signals int
	// Actionable is how many cases have something to do.
	Actionable int
	// Critical is how many are critical.
	Critical int
	// Closed is how many resolved themselves.
	Closed int
}

// Merged is how many signal-rows the merge removed from the reader's day.
func (s Summary) Merged() int {
	if s.Signals <= s.Cases {
		return 0
	}
	return s.Signals - s.Cases
}

// Summarize counts a built queue.
func Summarize(cases []Case) Summary {
	var s Summary
	for _, c := range cases {
		s.Signals += len(c.Signals)
		if c.Closed {
			s.Closed++
			continue
		}
		s.Cases++
		if c.Actionable {
			s.Actionable++
		}
		if c.Severity == SeverityCritical {
			s.Critical++
		}
	}
	return s
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

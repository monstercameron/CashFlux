// SPDX-License-Identifier: MIT

// Package acctref reads the account a transaction description NAMES.
//
// Bank descriptors for a transfer almost always identify the far side by its
// trailing digits, and in a small number of shapes:
//
//	Transfer to Savings *6500
//	Transfer to Account Ending 1677
//	Transfer to account ending in 8945
//	Transfer to Account XXXXXXXXX1677
//	Transfer from checking ending 8945 to savings
//	ACH deposit — Internet transfer from account ending in 8945
//
// That is the only durable evidence a one-sided import carries about where the
// money went, and nothing in the app read it. Pairing transfers on amount and
// date alone cannot tell two $750 movements on the same day apart; the digits
// can, and they are already sitting in the description.
//
// # What it refuses to do
//
// A bare number is not an account reference. "AMZN 12345", an invoice number, a
// date, an amount — every one of those would produce a confident wrong answer,
// and a wrong pairing moves money between the wrong two accounts. So a run of
// digits counts only when a MARKER says it is an account: a leading star or hash,
// a mask of x's, or one of the "account/acct/ending/ending in" phrasings. No
// marker, no reference.
package acctref

import (
	"strings"
	"unicode"
)

// Direction is which way the description says the money went, relative to the
// account the row is posted on.
type Direction int

const (
	// Unknown means the description names an account but not a direction.
	Unknown Direction = iota
	// To means the money left this row's account for the named one.
	To
	// From means the money arrived from the named account.
	From
)

// String renders the direction for logs and test failures.
func (d Direction) String() string {
	switch d {
	case To:
		return "to"
	case From:
		return "from"
	default:
		return "unknown"
	}
}

// Ref is one account the description names.
type Ref struct {
	// Digits is the trailing digits as printed, e.g. "6500". Never empty.
	Digits string
	// Direction is which way the money moved, when the description says.
	Direction Direction
}

// MinDigits and MaxDigits bound what counts as an account's trailing digits.
//
// Four is the near-universal print length and the floor is three because some
// statements shorten to three. The ceiling is generous on purpose — a descriptor
// occasionally prints more of the number — but a long run of digits is far more
// likely to be a reference number, so anything longer is taken as its LAST four,
// which is what a mask like "XXXXXXXXX1677" means anyway.
const (
	MinDigits = 3
	MaxDigits = 5
)

// markers are the phrases that promote a run of digits into an account
// reference. Matched against a lower-cased, punctuation-normalized description.
var markers = []string{
	"ending in",
	"ending",
	"account",
	"acct",
	"acc",
	"a/c",
}

// Refs returns every account reference the description names, in the order they
// appear. An empty result means the description identifies no account — which is
// a normal and common answer, not a failure.
func Refs(desc string) []Ref {
	lower := strings.ToLower(desc)
	runes := []rune(lower)

	var out []Ref
	for i := 0; i < len(runes); {
		if !unicode.IsDigit(runes[i]) {
			i++
			continue
		}
		j := i
		for j < len(runes) && unicode.IsDigit(runes[j]) {
			j++
		}
		digits := string(runes[i:j])
		if marked(runes, i, digits) {
			out = append(out, Ref{Digits: lastFour(digits), Direction: directionBefore(runes, i)})
		}
		i = j
	}
	return out
}

// One returns the single account a description names.
//
// It reports ok=false when the description names none, and also when it names
// two DIFFERENT accounts — "transfer from 8945 to 6500" is a sentence about two
// accounts, and picking one of them would be a guess dressed up as a reading.
func One(desc string) (Ref, bool) {
	refs := Refs(desc)
	if len(refs) == 0 {
		return Ref{}, false
	}
	first := refs[0]
	for _, r := range refs[1:] {
		if r.Digits != first.Digits {
			return Ref{}, false
		}
	}
	return first, true
}

// Names reports whether the description names the given account digits. Used to
// spot a row filed onto the very account its own description calls the far side
// — the signature of a statement export that put both legs on one account.
func Names(desc, digits string) bool {
	if digits == "" {
		return false
	}
	want := lastFour(strings.TrimSpace(digits))
	for _, r := range Refs(desc) {
		if r.Digits == want {
			return true
		}
	}
	return false
}

// lastFour trims an over-long digit run down to the part a bank actually prints,
// so "XXXXXXXXX1677" and "1677" are the same reference.
func lastFour(digits string) string {
	if len(digits) > 4 {
		return digits[len(digits)-4:]
	}
	return digits
}

// marked reports whether this run of digits is introduced as an account.
//
// Three ways qualify: an immediately preceding star or hash ("*6500", "#6500");
// a mask of x's running straight into it ("XXXXXXXXX1677"); or one of the
// account-word markers within a short window before it, so "account ending in
// 8945" and "acct 8945" both count while a stray number does not.
func marked(runes []rune, start int, digits string) bool {
	if len(digits) < MinDigits {
		return false
	}
	if len(digits) > MaxDigits && !maskedBefore(runes, start) {
		// A long bare run is a reference number, not an account — unless a mask of
		// x's precedes it, which is exactly how a full account number is printed.
		return false
	}
	if k := prevNonSpace(runes, start); k >= 0 {
		switch runes[k] {
		case '*', '#':
			return true
		case 'x':
			return true
		}
	}
	// Look back a short window for an account WORD. 24 characters covers
	// "account ending in " with room to spare and stops the marker from reaching
	// across a whole descriptor to bless an unrelated number.
	const window = 24
	from := start - window
	if from < 0 {
		from = 0
	}
	return hasMarkerWord(string(runes[from:start]))
}

// hasMarkerWord reports whether the lookback contains an account marker as a
// WHOLE WORD.
//
// A plain substring test was wrong in a way that fires on ordinary descriptors:
// "acc" is a marker, and "Acceleration Fee 12345" merely STARTS with it, so the
// invoice number was promoted to an account reference. Every merchant beginning
// with those letters — Accenture, Access Self Storage, Accessorize — followed by
// any three-to-five digit reference within the window had the same problem, and a
// wrong account reference is the input to a wrong pairing.
func hasMarkerWord(prefix string) bool {
	fields := strings.FieldsFunc(prefix, func(r rune) bool {
		return !unicode.IsLetter(r) && r != '/'
	})
	for _, f := range fields {
		for _, m := range markers {
			// Multi-word markers ("ending in") are matched against the whole
			// lookback, since the split has already separated their words.
			if f == m {
				return true
			}
		}
	}
	// "ending in" and friends: every word of the marker present, in order.
	for _, m := range markers {
		if !strings.Contains(m, " ") {
			continue
		}
		if strings.Contains(prefix, m) {
			return true
		}
	}
	return false
}

// maskedBefore reports whether a run of x's immediately precedes the digits.
func maskedBefore(runes []rune, start int) bool {
	k := prevNonSpace(runes, start)
	return k >= 0 && runes[k] == 'x'
}

// prevNonSpace returns the index of the last non-space rune before i, or -1.
func prevNonSpace(runes []rune, i int) int {
	for k := i - 1; k >= 0; k-- {
		if !unicode.IsSpace(runes[k]) {
			return k
		}
	}
	return -1
}

// directionBefore reads the nearest "to"/"from" preceding the digits.
//
// Nearest, not first: "transfer from checking ending 8945 to savings" is a
// movement FROM 8945, and the trailing "to savings" describes the other end. The
// word closest to the digits is the one that governs them.
func directionBefore(runes []rune, start int) Direction {
	words := strings.Fields(string(runes[:start]))
	for i := len(words) - 1; i >= 0; i-- {
		switch strings.Trim(words[i], ".,;:()[]-—–") {
		case "to", "into":
			return To
		case "from":
			return From
		}
	}
	return Unknown
}

// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

// A translated string is passed through fmt.Sprintf whenever the caller supplies
// arguments, so a literal percent sign in such a string has to be written "%%".
// Getting it wrong does not fail a build or a type check — it silently ships
// mangled copy:
//
//	"5% more, -10 for 10% less. Between %s%% and %s%%."
//	→ "5%!m(string=-90)ore, -10 for 10%!l(string=500)ess. Between %!s(MISSING)% …"
//
// which is what the first draft of the C592 "Adjust all" hint put on screen. The
// class of bug is invisible in review — the string reads correctly in the source
// — and only appears when rendered, so it is checked here.
//
// Two deliberate limits keep this a ratchet people will leave switched on:
//
//   - Only strings carrying a placeholder are checked. A sentence of pure prose
//     with a "30%" in it is never formatted (T formats only when given arguments),
//     and demanding "30%%" of every such sentence would be noise.
//   - A placeholder means '%' followed IMMEDIATELY by a verb, which is how every
//     formatted string in this catalog is written. Flags and widths are not
//     recognised, so "50% savings" is read as prose rather than as a space-flagged
//     %s — the reading that produced four false alarms on copy that is fine.
func TestNoStrayPercentInFormattedStrings(t *testing.T) {
	for key, msg := range english {
		if !hasPlaceholder(msg) {
			continue
		}
		if bad, ok := strayPercent(msg); ok {
			t.Errorf("%s: %q — %q is not a placeholder.\n"+
				"This string takes arguments, so every literal percent sign in it must be "+
				"written %%%%; otherwise fmt.Sprintf eats the following character and the "+
				"user sees %%!x(...).", key, msg, bad)
		}
	}
}

// verbs are the fmt verbs this catalog's copy uses. Deliberately short: a UI
// string has no business formatting a pointer or a Go-syntax value, and a wider
// list would let a real typo through as a "verb".
const verbs = "sdvqtfgex"

// isPlaceholderAt reports whether a placeholder starts at msg[i] — a '%',
// optional width/precision, then a verb — and where its verb sits.
//
// Width and precision are recognised ("%.0f", "%2d") because the catalog uses
// them. Flag characters are NOT: a space, '+' or '-' between the percent and a
// letter is far more often English ("50% savings") than a space-flagged verb,
// and reading it as the latter flagged four perfectly good sentences.
func isPlaceholderAt(msg string, i int) (verbAt int, ok bool) {
	j := i + 1
	for j < len(msg) && strings.ContainsRune("0123456789.", rune(msg[j])) {
		j++
	}
	if j < len(msg) && strings.ContainsRune(verbs, rune(msg[j])) {
		return j, true
	}
	return i, false
}

// hasPlaceholder reports whether msg contains at least one "%<verb>".
func hasPlaceholder(msg string) bool {
	for i := 0; i < len(msg); i++ {
		if msg[i] != '%' {
			continue
		}
		if i+1 < len(msg) && msg[i+1] == '%' {
			i++ // an escaped literal
			continue
		}
		if _, ok := isPlaceholderAt(msg, i); ok {
			return true
		}
	}
	return false
}

// strayPercent finds the first '%' that is neither an escaped "%%" nor a
// placeholder, and returns the offending sequence.
func strayPercent(msg string) (string, bool) {
	for i := 0; i < len(msg); i++ {
		if msg[i] != '%' {
			continue
		}
		if i+1 >= len(msg) {
			return "%", true // a trailing '%' in a formatted string is always a mistake
		}
		if msg[i+1] == '%' {
			i++
			continue
		}
		verbAt, ok := isPlaceholderAt(msg, i)
		if !ok {
			return msg[i : i+2], true
		}
		i = verbAt
	}
	return "", false
}

// The detector's own regression: the exact string that shipped broken must be
// caught, and the corrected one must pass. A ratchet nobody has seen fire is a
// ratchet nobody knows works.
func TestStrayPercentDetector(t *testing.T) {
	broken := "A positive number raises every limit, a negative one lowers it — 5 for 5% more, -10 for 10% less. Between %s%% and %s%%."
	if !hasPlaceholder(broken) {
		t.Fatal("the broken hint does carry placeholders; hasPlaceholder said otherwise")
	}
	if bad, ok := strayPercent(broken); !ok {
		t.Error("the broken C592 hint was not flagged")
	} else if bad != "% m" && bad != "%!" && !strings.HasPrefix(bad, "% ") {
		t.Errorf("flagged %q, expected the '5%% more' literal", bad)
	}

	fixed := "A positive number raises every limit, a negative one lowers it — 5 for 5%% more, -10 for 10%% less. Between %s%% and %s%%."
	if bad, ok := strayPercent(fixed); ok {
		t.Errorf("the corrected hint was flagged at %q", bad)
	}

	// Width and precision are placeholders, not strays.
	if _, ok := strayPercent("returns %.0f · stability %2d%%"); ok {
		t.Error("a width/precision placeholder was read as a stray percent")
	}
	// Prose with no placeholder is never formatted, so it is not this test's business.
	if hasPlaceholder("Create starter budgets (50% needs / 30% wants / 20% savings)") {
		t.Error("prose with a bare percent was read as a formatted string")
	}
}

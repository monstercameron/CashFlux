// SPDX-License-Identifier: MIT

package smartengine

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
)

// plural renders a count with a noun, correctly pluralized for n != 1
// ("1 month" / "3 months", "1 category" / "2 categories", "1 entry" / "97
// entries"). It pluralizes the LAST word of a phrase so "active goal" → "active
// goals" works.
func plural(n int64, noun string) string {
	word := noun
	if n != 1 {
		word = pluralize(noun)
	}
	return strconv.FormatInt(n, 10) + " " + word
}

// pluralize returns the English plural of a (possibly multi-word) noun, applying
// the common spelling rules so product copy never reads "categorys"/"entrys":
// a consonant+"y" → "ies"; a sibilant ending (s/x/z/ch/sh) → "es"; otherwise "+s".
func pluralize(noun string) string {
	noun = strings.TrimRight(noun, " ")
	if noun == "" {
		return noun
	}
	// Pluralize only the final word of a phrase.
	prefix, w := "", noun
	if i := strings.LastIndexByte(noun, ' '); i >= 0 {
		prefix, w = noun[:i+1], noun[i+1:]
	}
	switch {
	case len(w) >= 2 && w[len(w)-1] == 'y' && !isVowel(w[len(w)-2]):
		w = w[:len(w)-1] + "ies"
	case strings.HasSuffix(w, "s"), strings.HasSuffix(w, "x"), strings.HasSuffix(w, "z"),
		strings.HasSuffix(w, "ch"), strings.HasSuffix(w, "sh"):
		w += "es"
	default:
		w += "s"
	}
	return prefix + w
}

func isVowel(b byte) bool {
	switch b {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return true
	}
	return false
}

// fmtPct renders a percentage value (e.g. 4.5 -> "4.5%"), trimming a trailing
// ".0" so whole percents read cleanly ("4%").
func fmtPct(p float64) string {
	s := strconv.FormatFloat(p, 'f', 1, 64)
	if len(s) > 2 && s[len(s)-2:] == ".0" {
		s = s[:len(s)-2]
	}
	return s + "%"
}

// itoa64 formats an int64 in base 10.
func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

// hmoneyc renders a money amount as product-ready copy: the currency symbol, a
// thousands separator, and whole-dollar rounding for sizable amounts (≥ 100
// units, e.g. "$519", "$5,434") while keeping cents for small ones (e.g.
// "$0.99", "$22.50"). This is the human-facing alternative to the raw
// decimal-only Money.Format used internally.
func hmoneyc(minor int64, cur string) string {
	if cur == "" {
		cur = "USD"
	}
	sym := currency.Symbol(cur)
	dec := currency.Decimals(cur)
	neg := minor < 0
	if neg {
		minor = -minor
	}
	div := int64(1)
	for i := 0; i < dec; i++ {
		div *= 10
	}
	var body string
	switch {
	case dec == 0:
		body = group(minor)
	case minor >= 100*div: // ≥ 100 major units → round to whole, drop cents
		body = group((minor + div/2) / div)
	default:
		whole, frac := minor/div, minor%div
		body = group(whole) + "." + zeroPad(frac, dec)
	}
	s := sym + body
	if neg {
		s = "-" + s
	}
	return s
}

// hmoney renders a base-currency minor amount as product-ready copy.
func (in Input) hmoney(minor int64) string { return hmoneyc(minor, in.Base) }

// hm renders a money.Money value as product-ready copy (symbol, grouping, and
// whole-unit rounding for large amounts) — the humanized alternative to
// Money.Format used inside user-facing insight text.
func hm(m money.Money) string { return hmoneyc(m.Amount, m.Currency) }

// group inserts thousands separators into a non-negative integer's digits.
func group(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	lead := len(s) % 3
	if lead == 0 {
		lead = 3
	}
	b.WriteString(s[:lead])
	for i := lead; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// zeroPad renders frac as a dec-digit, zero-padded fraction ("5" -> "05").
func zeroPad(frac int64, dec int) string {
	s := strconv.FormatInt(frac, 10)
	for len(s) < dec {
		s = "0" + s
	}
	return s
}

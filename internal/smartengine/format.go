// SPDX-License-Identifier: MIT

package smartengine

import "strconv"

// plural renders a count with a noun, pluralized with a trailing "s" for n != 1
// ("1 month", "3 months"). Good enough for the regular nouns used in insights.
func plural(n int64, noun string) string {
	s := strconv.FormatInt(n, 10) + " " + noun
	if n != 1 {
		s += "s"
	}
	return s
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

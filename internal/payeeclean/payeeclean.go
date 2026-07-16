// SPDX-License-Identifier: MIT

// Package payeeclean is the deterministic (SMART, no-LLM) merchant-name normalizer:
// given a raw processor descriptor like "SQ *BLUE BOTTLE COFFEE #47", it proposes a
// clean display name ("Blue Bottle Coffee"). It handles the common, mechanical cases —
// processor prefixes, star separators, trailing store/reference numbers, and ALL-CAPS
// noise — and leaves the genuinely ambiguous ones (e.g. cryptic codes) to the SMART+
// AI cleanup. Pure and table-tested; no syscall/js.
package payeeclean

import (
	"regexp"
	"strings"
)

var (
	// A leading processor code + star, e.g. "SQ *", "TST* ", "PP*", "PAYPAL *".
	reProcessorPrefix = regexp.MustCompile(`(?i)^\s*(sq|tst|pp|sp|pos|dd|ci|pmt|paypal|venmo|cash app|sumup|toast|clover)\b\s*\*+\s*`)
	// Any remaining star separators → space ("AMZN*MKTP" → "AMZN MKTP").
	reStar = regexp.MustCompile(`\*+`)
	// A trailing store/reference/masked-card token: "#123", "x1234", "*4821", or a run
	// of 3+ digits at the end ("… STORE 004821").
	reTrailingRef = regexp.MustCompile(`(?i)\s+(#\s*\d{2,}|x{2,}\s*\d+|\d{3,}|store\s*#?\d+)\s*$`)
	// A trailing US "CITY ST" tail some processors append ("… PORTLAND OR"). Only a
	// SINGLE-word city is stripped (no spaces in the city token) so it can't gobble the
	// merchant name itself — multi-word cities are left for the SMART+ AI cleanup.
	reTrailingCityState = regexp.MustCompile(`(?i)\s+[A-Za-z][A-Za-z.\-]+\s+(a[klrz]|c[aot]|d[ce]|fl|ga|hi|i[adln]|k[sy]|la|m[adeinost]|n[cdehjmvy]|o[hkr]|pa|ri|s[cd]|t[nx]|ut|v[at]|w[aivy])\s*$`)
	reMultiSpace        = regexp.MustCompile(`\s{2,}`)
)

// Suggest returns a cleaned display name for a raw payee descriptor, or "" for empty
// input. It never returns something wildly different — if nothing matches, it just
// normalizes whitespace and casing, so the caller can always show the result safely.
func Suggest(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = reProcessorPrefix.ReplaceAllString(s, "")
	s = reStar.ReplaceAllString(s, " ")
	s = reMultiSpace.ReplaceAllString(strings.TrimSpace(s), " ")
	// Strip trailing city/state before trailing refs (order matters: "… PORTLAND OR 12").
	s = strings.TrimSpace(reTrailingRef.ReplaceAllString(s, ""))
	s = strings.TrimSpace(reTrailingCityState.ReplaceAllString(s, ""))
	s = strings.TrimSpace(reTrailingRef.ReplaceAllString(s, ""))
	s = reMultiSpace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return strings.TrimSpace(raw) // never blank out the whole name
	}
	if isMostlyUpper(s) {
		s = titleCase(s)
	}
	return s
}

// WouldChange reports whether Suggest(raw) differs from the raw string (after trim), so
// a caller can skip offering a cleanup that does nothing.
func WouldChange(raw string) bool {
	s := Suggest(raw)
	return s != "" && !strings.EqualFold(strings.TrimSpace(raw), s)
}

// isMostlyUpper reports whether the letters in s are predominantly uppercase — the
// signature of a processor descriptor worth title-casing.
func isMostlyUpper(s string) bool {
	var upper, lower int
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			upper++
		case r >= 'a' && r <= 'z':
			lower++
		}
	}
	return upper > 0 && upper >= lower*2
}

// titleCase capitalizes the first letter of each word and lowercases the rest, keeping
// short all-caps tokens that look like acronyms (2 letters, e.g. "US", "BP") uppercase.
func titleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		if w == "" {
			continue
		}
		r := []rune(w)
		r[0] = upperRune(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

func upperRune(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

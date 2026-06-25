// SPDX-License-Identifier: MIT

// Package smarttext provides deterministic, on-device text-normalisation helpers
// for the SMART layer's Free (no-AI) field assists. All functions are pure Go with
// no syscall/js dependency, so they unit-test on native Go and run safely inside
// the wasm build.
package smarttext

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ── CleanMerchant ────────────────────────────────────────────────────────────

// posPatterns lists common bank/POS prefix patterns to strip before title-casing.
// Each is tried in order; the first match wins.
var posPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^SQ\s*\*\s*`),
	regexp.MustCompile(`(?i)^TST\*`),
	regexp.MustCompile(`(?i)^PP\*`),
	regexp.MustCompile(`(?i)^POS\s+`),
	regexp.MustCompile(`(?i)^DEBIT\s+`),
	regexp.MustCompile(`(?i)^ACH\s+`),
}

// trailPattern strips a trailing store/ref code: a "#" followed by digits,
// OR 5+ consecutive digits at the end (bank reference numbers).
var trailPattern = regexp.MustCompile(`(?i)\s*#\d+\s*$|\s*\b\d{5,}\s*$`)

// acronymRE matches exactly 2 uppercase letters — always treated as acronyms (BP, AC, etc.).
var acronymRE = regexp.MustCompile(`^[A-Z]{2}$`)

// knownAcronyms3 is the allow-list of 3-letter all-caps tokens that should stay
// ALL-CAPS rather than being title-cased to "Atm". Common bank/POS terms only.
var knownAcronyms3 = map[string]bool{
	"ATM": true,
	"ACH": true,
	"POS": true,
	"LLC": true,
	"INC": true,
	"USA": true,
}

// CleanMerchant normalises messy bank-style merchant strings into a
// human-readable name:
//
//   - Strips common POS prefixes (SQ *, TST*, PP*, POS, DEBIT, ACH).
//   - Strips trailing store numbers / reference codes (#1234, long numeric suffixes).
//   - Title-cases the result, keeping short ALL-CAPS acronyms (≤3 chars, e.g. ATM, BP).
//   - Collapses excess whitespace.
//
// Returns the raw (trimmed) input unchanged when the result would be identical to
// what a plain strings.TrimSpace produces — i.e. only returns something different
// when there is real cleaning to do.
func CleanMerchant(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}

	// Strip POS prefixes.
	for _, re := range posPatterns {
		if loc := re.FindStringIndex(s); loc != nil {
			s = s[loc[1]:]
			s = strings.TrimSpace(s)
			break
		}
	}

	// Strip trailing store/ref codes.
	s = trailPattern.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)

	if s == "" {
		return strings.TrimSpace(raw)
	}

	// Title-case word by word, preserving known ALL-CAPS acronyms.
	words := strings.Fields(s)
	for i, w := range words {
		if acronymRE.MatchString(w) || knownAcronyms3[w] {
			// Keep 2-char tokens and known 3-char acronyms as-is.
			continue
		}
		words[i] = titleWord(w)
	}
	cleaned := strings.Join(words, " ")

	// If cleaning produced no real change, return the trimmed original.
	if cleaned == strings.TrimSpace(raw) {
		return strings.TrimSpace(raw)
	}
	return cleaned
}

// titleWord title-cases a single word (first rune upper, rest lower),
// handling mixed-case and digits gracefully.
func titleWord(w string) string {
	if w == "" {
		return w
	}
	runes := []rune(w)
	var b strings.Builder
	for i, r := range runes {
		if i == 0 {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// ── ParseWish ────────────────────────────────────────────────────────────────

// amountRE matches a dollar amount optionally preceded by "$" and with optional
// commas: e.g. "$2,000", "2000", "1,500.50".
var amountRE = regexp.MustCompile(`\$?([\d,]+(?:\.\d{1,2})?)`)

// forKW lists connecting prepositions to strip when separating amount from name.
var forKW = []string{
	"for a ", "for an ", "for my ", "for the ", "for ",
	"toward a ", "toward an ", "toward my ", "toward the ", "toward ",
}

// ParseWish attempts to parse a free-text "savings wish" into a structured goal.
// Supported patterns (case-insensitive):
//
//	"save $2,000 for a new laptop"  → name="New Laptop", amountMinor=200000
//	"laptop 2000"                   → name="Laptop",     amountMinor=200000
//	"$500 vacation fund"            → name="Vacation Fund", amountMinor=50000
//
// amountMinor is in the base currency's minor unit (e.g. cents for USD).
// ok is false when no usable amount or name can be extracted.
func ParseWish(text string) (name string, amountMinor int64, ok bool) {
	s := strings.TrimSpace(text)
	if s == "" {
		return "", 0, false
	}

	// Locate the first amount token.
	loc := amountRE.FindStringIndex(s)
	if loc == nil {
		return "", 0, false
	}
	amtStr := amountRE.FindString(s)
	minor, err := parseMinorAmount(amtStr)
	if err != nil || minor <= 0 {
		return "", 0, false
	}

	// Strip the amount token from the string to extract the name portion.
	before := strings.TrimSpace(s[:loc[0]])
	after := strings.TrimSpace(s[loc[1]:])

	// Strip leading save/put/set verbs that precede the amount.
	before = stripLeadingVerbs(before)

	// Strip connecting keywords (for a/toward/etc.) from the after fragment.
	after = stripLeadingConnectors(after)

	// Pick the longer, non-empty fragment as the name.
	before = strings.TrimSpace(before)
	after = strings.TrimSpace(after)

	switch {
	case before != "" && after != "":
		// Use whichever is longer; tie goes to "after" (typically more specific).
		if len(before) >= len(after) {
			name = before
		} else {
			name = after
		}
	case before != "":
		name = before
	case after != "":
		name = after
	default:
		return "", 0, false
	}

	// Title-case the extracted name.
	name = titleCaseName(name)
	return name, minor, true
}

// parseMinorAmount converts a string like "2,000" or "$1,500.50" to minor units
// (i.e. cents for a 2-decimal currency). Only 2-decimal currencies are assumed.
func parseMinorAmount(s string) (int64, error) {
	// Strip leading "$".
	s = strings.TrimPrefix(s, "$")
	// Remove commas.
	s = strings.ReplaceAll(s, ",", "")
	// Parse as float then convert to minor units.
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(f * 100), nil
}

// stripLeadingVerbs removes common leading save/spend verbs from a wish phrase.
var leadingVerbs = []string{
	"save up ", "save for ", "save ",
	"put aside ", "set aside ",
	"i want to ", "i need to ", "i need ",
}

func stripLeadingVerbs(s string) string {
	lower := strings.ToLower(s)
	for _, v := range leadingVerbs {
		// Match verb with trailing space (verb + more text) OR verb as the whole string.
		if strings.HasPrefix(lower, v) {
			s = strings.TrimSpace(s[len(v):])
			return s
		}
		// Verb without trailing space — exact match of the trimmed verb.
		vtrim := strings.TrimSpace(v)
		if lower == vtrim {
			return ""
		}
	}
	return s
}

// stripLeadingConnectors removes "for a/an/my/the/…" and "toward …" prefixes.
func stripLeadingConnectors(s string) string {
	lower := strings.ToLower(s)
	for _, kw := range forKW {
		if strings.HasPrefix(lower, kw) {
			s = strings.TrimSpace(s[len(kw):])
			break
		}
	}
	return s
}

// titleCaseName title-cases a multi-word name, keeping known ALL-CAPS acronyms.
func titleCaseName(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		upper := strings.ToUpper(w)
		if acronymRE.MatchString(upper) || knownAcronyms3[upper] {
			words[i] = upper
			continue
		}
		words[i] = titleWord(w)
	}
	return strings.Join(words, " ")
}

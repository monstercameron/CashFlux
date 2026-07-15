// SPDX-License-Identifier: MIT

// Package payeealias cleans up noisy processor payee strings ("AMZN Mktp
// US*2K4RT0" → "Amazon") and resolves a raw payee to a clean display name (TX1).
//
// It has no syscall/js dependency and imports only internal/domain + stdlib, so
// it compiles and unit-tests on native Go.
//
// Resolution order (see Resolver.Resolve):
//  1. a learned alias (exact match on the raw payee, case-insensitive) — the user
//     renamed this payee once and asked to always show it that way;
//  2. the built-in normalizer rule pack (Normalize) — strips common processor
//     prefixes/suffixes and title-cases the remainder;
//  3. the raw payee unchanged, when nothing cleaner is known.
//
// The mapping is view-layer only: the raw payee is never mutated on the
// transaction (single-source rule). Callers apply it at display and at
// rules/search/recurring matching so one clean name unifies everything.
package payeealias

import (
	"regexp"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// trailingRef matches trailing processor reference noise: a run of store
// numbers, auth codes, or hashes at the end of a merchant string, optionally
// introduced by '#', '*', or whitespace. Examples stripped: " *2K4RT0",
// " #1234", " 08842", "  X5J2Q".
var trailingRef = regexp.MustCompile(`(?i)[\s#*]+[A-Z0-9]*\d[A-Z0-9]*$`)

// collapseSpace collapses internal whitespace runs to a single space.
var collapseSpace = regexp.MustCompile(`\s+`)

// prefixRule strips a known processor prefix and optionally forces a canonical
// display name. When Canonical is non-empty the whole string resolves to it;
// otherwise the remainder (after the prefix) is cleaned and title-cased.
type prefixRule struct {
	// Prefix is matched case-insensitively at the very start of the trimmed
	// payee. It should include its own trailing separator (space, '*') so it
	// does not swallow a real name character.
	Prefix string
	// Canonical, when set, is the exact display name to use (e.g. an "AMZN Mktp"
	// prefix always means "Amazon"). When empty, the remainder is cleaned.
	Canonical string
}

// rulePack is the starter set of processor-noise rules. Order matters: the first
// matching prefix wins, so longer/more-specific prefixes are listed first.
var rulePack = []prefixRule{
	{Prefix: "AMZN MKTP", Canonical: "Amazon"},
	{Prefix: "AMZN.COM", Canonical: "Amazon"},
	{Prefix: "AMAZON MKTPL", Canonical: "Amazon"},
	{Prefix: "AMZN", Canonical: "Amazon"},
	{Prefix: "AMAZON PRIME", Canonical: "Amazon Prime"},
	{Prefix: "VENMO PAYMENT", Canonical: "Venmo"},
	{Prefix: "VENMO", Canonical: "Venmo"},
	{Prefix: "PAYPAL *"}, // "PAYPAL *STEAMGAMES" → "Steamgames"
	{Prefix: "PAYPAL"},
	{Prefix: "SQ *"},    // Square: "SQ *BLUE BOTTLE" → "Blue Bottle"
	{Prefix: "TST*"},    // Toast: "TST* JOES PIZZA" → "Joes Pizza"
	{Prefix: "CKE*"},    // Clover: "CKE*THE CORNER CAFE" → "The Corner Cafe"
	{Prefix: "SP*"},     // "SP*" merchant code
	{Prefix: "SP "},     // "SP MERCHANT CO" → "Merchant Co"
	{Prefix: "APLPAY "}, // Apple Pay wrapper: "APLPAY TARGET" → "Target"
	{Prefix: "APLPAY"},
}

// Normalize applies the built-in rule pack to a raw payee and returns a clean
// display name. When no rule matches it strips only trailing reference noise and
// tidies casing. It never returns an empty string for a non-empty trimmed input;
// if cleaning would empty the string it falls back to the trimmed raw payee.
func Normalize(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}

	upper := strings.ToUpper(s)
	for _, r := range rulePack {
		p := strings.ToUpper(r.Prefix)
		if !strings.HasPrefix(upper, p) {
			continue
		}
		if r.Canonical != "" {
			return r.Canonical
		}
		rest := strings.TrimSpace(s[len(r.Prefix):])
		rest = strings.TrimLeft(rest, "*# ")
		cleaned := tidy(rest)
		if cleaned != "" {
			return cleaned
		}
		// Prefix with no meaningful remainder — fall back to tidying the whole.
		break
	}

	return tidy(s)
}

// tidy strips trailing reference noise and normalises casing/whitespace. If the
// result would be empty it returns the collapsed-but-otherwise-unchanged input.
func tidy(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	stripped := trailingRef.ReplaceAllString(s, "")
	stripped = strings.TrimSpace(stripped)
	if stripped == "" {
		stripped = s
	}
	stripped = collapseSpace.ReplaceAllString(stripped, " ")
	return titleCase(stripped)
}

// knownAcronyms are business-suffix/acronym words preserved uppercase by
// titleCase (so "JOES BBQ LLC" → "Joes BBQ LLC" rather than "Llc").
var knownAcronyms = map[string]bool{
	"LLC": true, "INC": true, "LLP": true, "LTD": true, "CO": false,
	"USA": true, "ATM": true, "BBQ": true, "DVD": true, "TV": true,
	"US": false, "PC": true, "AMC": true, "IHOP": true, "KFC": true,
}

// titleCase upper-cases the first letter of each word and lower-cases the rest,
// so "JOES PIZZA" → "Joes Pizza". Words in knownAcronyms are preserved uppercase.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if knownAcronyms[strings.ToUpper(w)] {
			words[i] = strings.ToUpper(w)
			continue
		}
		lower := strings.ToLower(w)
		r := []rune(lower)
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// Resolver maps raw payee strings to clean display names using learned aliases
// first and the rule pack as a fallback. Build one with NewResolver from the
// current alias table; it is read-only and cheap to construct.
type Resolver struct {
	// learned maps a lower-cased raw payee to its learned display name.
	learned map[string]string
}

// NewResolver builds a Resolver from the persisted alias table. Later entries for
// the same raw payee (case-insensitively) win, matching last-write semantics.
func NewResolver(aliases []domain.PayeeAlias) *Resolver {
	m := make(map[string]string, len(aliases))
	for _, a := range aliases {
		raw := strings.TrimSpace(a.RawPayee)
		disp := strings.TrimSpace(a.Display)
		if raw == "" || disp == "" {
			continue
		}
		m[strings.ToLower(raw)] = disp
	}
	return &Resolver{learned: m}
}

// Resolve returns the clean display name for a raw payee: a learned alias if one
// exists (exact, case-insensitive), otherwise the rule-pack normalization,
// otherwise the trimmed raw payee. An all-whitespace input returns "".
func (r *Resolver) Resolve(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if r != nil {
		if disp, ok := r.learned[strings.ToLower(s)]; ok {
			return disp
		}
	}
	return Normalize(s)
}

// HasLearned reports whether a learned alias exists for the raw payee (exact,
// case-insensitive). Callers use this to avoid re-offering "always show X as Y"
// for a payee that is already aliased.
func (r *Resolver) HasLearned(raw string) bool {
	if r == nil {
		return false
	}
	_, ok := r.learned[strings.ToLower(strings.TrimSpace(raw))]
	return ok
}

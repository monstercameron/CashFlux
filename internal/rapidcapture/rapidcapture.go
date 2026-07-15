// SPDX-License-Identifier: MIT

// Package rapidcapture parses a free-typed or dictated capture line — e.g.
// "coffee 4.50, gas 38, costco 122 split with priya" — into a list of DRAFT
// transactions for a bulk quick-add review (AG11). It is the FREE, deterministic
// tier of rapid capture: a small amount+word grammar that needs no API key. The
// AI fallback (for messier input) is layered on at the caller.
//
// Each comma- (or newline-/semicolon-) separated clause yields one Draft: the
// numeric token is the amount, the surrounding words are the label/payee, and a
// "split" / "split with <name>" marker flags the draft as shared. Amounts are
// returned as integer minor units (cents, 2 decimals) so the domain layer never
// touches a float; the caller re-converts MajorString through currency.MinorFromMajor
// for non-cent currencies.
//
// Pure Go, no syscall/js: unit-tested on native Go.
package rapidcapture

import (
	"strconv"
	"strings"
)

// Draft is one parsed capture entry awaiting review. It is intentionally a plain
// value (not a domain.Transaction) so the package stays dependency-free and the
// review UI decides account, category, and sign.
type Draft struct {
	// Label is the human words of the clause with amount and split markers removed
	// (e.g. "coffee", "costco"). Empty if the clause was only a number.
	Label string
	// MajorString is the cleaned major-unit amount as written ("4.50", "38"), so the
	// caller can convert it precisely for the active currency.
	MajorString string
	// Cents is MajorString parsed to integer minor units at 2 decimals — the default
	// for USD-like currencies and the value the tests assert on.
	Cents int64
	// Split flags that the user marked this entry as shared with someone.
	Split bool
	// SplitWith is the named counterpart when the clause said "split with <name>";
	// empty when the split marker named no one.
	SplitWith string
	// Raw is the original clause text, for showing the user what was parsed.
	Raw string
}

// Parse splits s into clauses and parses each into a Draft. Clauses with no usable
// amount are skipped. The result preserves input order. A nil result means nothing
// parseable was found.
func Parse(s string) []Draft {
	var out []Draft
	for _, clause := range splitClauses(s) {
		if d, ok := parseClause(clause); ok {
			out = append(out, d)
		}
	}
	return out
}

// splitClauses breaks the input on commas, newlines, and semicolons — the natural
// separators between captured items — and trims each piece.
func splitClauses(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// parseClause parses one clause into a Draft. It first lifts a trailing "split
// [with <name>]" marker, then finds the amount token and treats the remaining
// words as the label.
func parseClause(clause string) (Draft, bool) {
	d := Draft{Raw: strings.TrimSpace(clause)}
	words := strings.Fields(clause)

	// Detect and strip the split marker. "split with priya and me" → SplitWith names.
	kept := make([]string, 0, len(words))
	for i := 0; i < len(words); i++ {
		if strings.EqualFold(strings.Trim(words[i], ".,"), "split") {
			d.Split = true
			// Consume an optional "with <names...>" tail.
			j := i + 1
			if j < len(words) && strings.EqualFold(words[j], "with") {
				j++
				var names []string
				for ; j < len(words); j++ {
					w := strings.Trim(words[j], ".,")
					if strings.EqualFold(w, "and") || w == "" {
						continue
					}
					names = append(names, w)
				}
				d.SplitWith = strings.Join(names, ", ")
			}
			break
		}
		kept = append(kept, words[i])
	}

	// Find the amount among the kept words: prefer the last numeric token.
	amtIdx := -1
	for i := len(kept) - 1; i >= 0; i-- {
		if _, cents, ok := parseAmount(kept[i]); ok {
			amtIdx = i
			_ = cents
			break
		}
	}
	if amtIdx < 0 {
		return Draft{}, false
	}
	major, cents, _ := parseAmount(kept[amtIdx])
	d.MajorString = major
	d.Cents = cents

	label := make([]string, 0, len(kept))
	for i, w := range kept {
		if i == amtIdx {
			continue
		}
		label = append(label, w)
	}
	d.Label = strings.TrimSpace(strings.Join(label, " "))
	return d, true
}

// parseAmount cleans a money token ("$4.50", "1,000", "38") into its major-unit
// string and integer cents (2 decimals). It reports false when the token is not a
// usable number, so plain words never read as amounts.
func parseAmount(tok string) (major string, cents int64, ok bool) {
	s := strings.TrimSpace(tok)
	s = strings.TrimPrefix(s, "$")
	s = strings.TrimSuffix(s, ".")
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return "", 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return "", 0, false
	}
	// Round to cents without float drift beyond the ledger's tolerance.
	c := int64(f*100 + sign(f)*0.5)
	return s, c, true
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}

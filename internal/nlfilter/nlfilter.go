// SPDX-License-Identifier: MIT

// Package nlfilter compiles a plain-language transaction query into the existing
// txnfilter.Criteria shape. It is the FREE tier of the SMART-series natural-
// language search (SMART-T3F): a small, fully deterministic grammar covering the
// common phrasings — amount comparators ("over $20", ">50", "between 10 and 50"),
// date words ("last month", "in june", "since march", "yesterday"), category /
// tag words matched against the caller's name sets, alias-resolved payee words,
// cleared / uncleared, and income / expense ("spent" / "received"). Anything the
// grammar does not recognize becomes the plain text search, so no query is ever
// lost. It produces FILTERS, never rows — the txnfilter engine still does the
// selecting, so results stay explainable and materialize as the normal removable
// chips.
//
// Pure Go, no syscall/js: it is unit-tested on native Go and the wasm screen just
// hands it the typed query plus a Context built from the current entity lists.
package nlfilter

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// NameID pairs a display name with the entity ID the filter stores. Category
// clauses compile to IDs (txnfilter.Criteria.Categories holds IDs), so the caller
// passes the id→name mapping as NameID values.
type NameID struct {
	Name string
	ID   string
}

// Context is the vocabulary and clock the parser matches against. All of it is
// optional: an empty Context still parses amounts, dates, cleared state, and
// income/expense — the fields only add category/tag/payee word matching.
type Context struct {
	// Now anchors the relative date words ("today", "last month", "this year").
	// A zero value falls back to time.Now().UTC() so the parser is never date-blind.
	Now time.Time
	// WeekStart is the first weekday of the user's week, for "this week" / "last
	// week". The zero value (time.Sunday) is a sensible default.
	WeekStart time.Weekday
	// Categories are the selectable categories (name→ID); a matched category word
	// compiles to its ID in Criteria.Categories.
	Categories []NameID
	// Tags are the tags in use; a matched tag word compiles to Criteria.Tags.
	Tags []string
	// Payees are the clean (alias-resolved) merchant display names in use. A query
	// word that names one is canonicalized into the text search.
	Payees []string
	// ResolvePayee, when set, maps a raw/typed payee fragment to its clean display
	// name (the payee-alias resolver). Used so "amzn" in a query resolves to
	// "Amazon" for the text search. Optional.
	ResolvePayee func(string) string
}

// Parse compiles q into a txnfilter.Criteria using ctx's vocabulary and clock. ok
// is false only when NOTHING structured was recognized — i.e. the query is just
// free text — so callers can suppress the "interpret" affordance when there is no
// real filter to extract. When ok is true the returned Criteria carries the
// recognized clauses, with any unrecognized words left as the text search.
func Parse(q string, ctx Context) (txnfilter.Criteria, bool) {
	now := ctx.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	// Normalize comparator symbols into standalone tokens so ">$20" and ">20"
	// tokenize the same as "over 20".
	norm := strings.NewReplacer(">", " > ", "<", " < ").Replace(q)
	orig := strings.Fields(norm)
	if len(orig) == 0 {
		return txnfilter.Criteria{}, false
	}
	low := make([]string, len(orig))
	for i, t := range orig {
		low[i] = strings.ToLower(t)
	}
	consumed := make([]bool, len(orig))

	var out txnfilter.Criteria
	structured := 0
	mark := func(idxs ...int) {
		for _, i := range idxs {
			consumed[i] = true
		}
	}

	// amountAt reports the cleaned major-unit amount string at index i, when that
	// token is present, unconsumed, and numeric.
	amountAt := func(i int) (string, bool) {
		if i < 0 || i >= len(orig) || consumed[i] {
			return "", false
		}
		return parseAmount(low[i])
	}

	// ── Amount comparators ────────────────────────────────────────────────────
	for i := 0; i < len(orig); i++ {
		if consumed[i] {
			continue
		}
		w := low[i]
		// Two-word comparators ("more than 20", "less than 50").
		if i+1 < len(orig) {
			switch w + " " + low[i+1] {
			case "more than", "greater than", "larger than", "bigger than":
				if a, ok := amountAt(i + 2); ok {
					out.AmountMin = a
					mark(i, i+1, i+2)
					structured++
					continue
				}
			case "less than", "lower than", "smaller than", "fewer than":
				if a, ok := amountAt(i + 2); ok {
					out.AmountMax = a
					mark(i, i+1, i+2)
					structured++
					continue
				}
			}
		}
		switch w {
		case "over", "above", ">", "atleast", "min":
			if a, ok := amountAt(i + 1); ok {
				out.AmountMin = a
				mark(i, i+1)
				structured++
			}
		case "under", "below", "<", "atmost", "max":
			if a, ok := amountAt(i + 1); ok {
				out.AmountMax = a
				mark(i, i+1)
				structured++
			}
		case "between":
			// "between X and Y".
			if lo, ok := amountAt(i + 1); ok && i+3 < len(orig) && low[i+2] == "and" {
				if hi, ok2 := amountAt(i + 3); ok2 {
					out.AmountMin, out.AmountMax = lo, hi
					mark(i, i+1, i+2, i+3)
					structured++
				}
			}
		}
	}

	// ── Dates ─────────────────────────────────────────────────────────────────
	structured += parseDates(orig, low, consumed, now, ctx.WeekStart, &out)

	// ── Cleared state and income/expense flow ─────────────────────────────────
	// "not cleared" / "not reconciled" → uncleared. Handled before the single-word
	// pass so the bare "cleared" case doesn't claim the token first.
	for i := 0; i+1 < len(orig); i++ {
		if consumed[i] || consumed[i+1] {
			continue
		}
		if low[i] == "not" && (low[i+1] == "cleared" || low[i+1] == "reconciled") {
			out.Cleared = "no"
			mark(i, i+1)
			structured++
		}
	}
	for i := 0; i < len(orig); i++ {
		if consumed[i] {
			continue
		}
		switch low[i] {
		case "cleared", "reconciled":
			out.Cleared = "yes"
			mark(i)
			structured++
		case "uncleared", "unreconciled", "pending", "outstanding":
			out.Cleared = "no"
			mark(i)
			structured++
		case "spent", "spending", "expense", "expenses", "purchases", "debits", "outgoing":
			out.Flow = "out"
			mark(i)
			structured++
		case "received", "income", "deposits", "credits", "earned", "incoming":
			out.Flow = "in"
			mark(i)
			structured++
		}
	}

	// ── Category / tag name words (multi-word, longest match first) ────────────
	structured += matchNames(orig, low, consumed, ctx, &out)

	// ── Residue → text search (with payee canonicalization) ───────────────────
	var residue []string
	for i, t := range orig {
		if !consumed[i] {
			residue = append(residue, t)
		}
	}
	if text := strings.TrimSpace(strings.Join(residue, " ")); text != "" {
		out.Text = canonicalizePayee(text, ctx)
	}

	return out, structured > 0
}

// parseDates recognizes date words and sets out.From / out.To, returning the
// number of date clauses recognized (0 or 1 in practice). Consumed tokens are
// marked so they don't leak into the text search.
func parseDates(orig, low []string, consumed []bool, now time.Time, weekStart time.Weekday, out *txnfilter.Criteria) int {
	mark := func(idxs ...int) {
		for _, i := range idxs {
			consumed[i] = true
		}
	}
	set := func(from, toInclusive time.Time) {
		out.From = dateutil.FormatDate(from)
		out.To = dateutil.FormatDate(toInclusive)
	}
	monthRange := func(anchor time.Time) {
		s, e := dateutil.MonthRange(anchor)
		set(s, e.AddDate(0, 0, -1))
	}

	// Two-word relative phrases first.
	for i := 0; i+1 < len(orig); i++ {
		if consumed[i] || consumed[i+1] {
			continue
		}
		switch low[i] + " " + low[i+1] {
		case "this month":
			monthRange(now)
			mark(i, i+1)
			return 1
		case "last month":
			monthRange(dateutil.AddMonths(now, -1))
			mark(i, i+1)
			return 1
		case "this week":
			ws := dateutil.WeekStart(now, weekStart)
			set(ws, ws.AddDate(0, 0, 6))
			mark(i, i+1)
			return 1
		case "last week":
			ws := dateutil.WeekStart(now, weekStart).AddDate(0, 0, -7)
			set(ws, ws.AddDate(0, 0, 6))
			mark(i, i+1)
			return 1
		case "this year":
			set(yearStart(now.Year()), yearEnd(now.Year()))
			mark(i, i+1)
			return 1
		case "last year":
			set(yearStart(now.Year()-1), yearEnd(now.Year()-1))
			mark(i, i+1)
			return 1
		}
	}

	// Single-word relative days.
	for i := 0; i < len(orig); i++ {
		if consumed[i] {
			continue
		}
		switch low[i] {
		case "today":
			d := dateOnly(now)
			set(d, d)
			mark(i)
			return 1
		case "yesterday":
			d := dateOnly(now).AddDate(0, 0, -1)
			set(d, d)
			mark(i)
			return 1
		}
	}

	// "since <month>" — open-ended lower bound at the start of that month.
	for i := 0; i+1 < len(orig); i++ {
		if consumed[i] || consumed[i+1] {
			continue
		}
		if low[i] == "since" {
			if m, ok := monthNum(low[i+1]); ok {
				y := now.Year()
				out.From = dateutil.FormatDate(time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC))
				out.To = ""
				mark(i, i+1)
				return 1
			}
		}
	}

	// "in <month> [year]" or a bare "<month> [year]".
	for i := 0; i < len(orig); i++ {
		if consumed[i] {
			continue
		}
		mi := i
		if low[i] == "in" && i+1 < len(orig) && !consumed[i+1] {
			mi = i + 1
		}
		m, ok := monthNum(low[mi])
		if !ok {
			continue
		}
		y := now.Year()
		yTokens := 0
		if mi+1 < len(orig) && !consumed[mi+1] {
			if yr, err := strconv.Atoi(low[mi+1]); err == nil && yr >= 1900 && yr <= 3000 {
				y = yr
				yTokens = 1
			}
		}
		anchor := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		monthRange(anchor)
		if mi == i {
			mark(i)
		} else {
			mark(i, mi)
		}
		if yTokens == 1 {
			mark(mi + 1)
		}
		return 1
	}
	return 0
}

// matchNames greedily matches the longest run of consecutive unconsumed tokens
// against the category and tag vocabularies (categories win ties), setting the
// corresponding Criteria field. Returns the number of name clauses recognized.
func matchNames(orig, low []string, consumed []bool, ctx Context, out *txnfilter.Criteria) int {
	catByName := map[string]string{}
	for _, c := range ctx.Categories {
		if n := strings.ToLower(strings.TrimSpace(c.Name)); n != "" {
			catByName[n] = c.ID
		}
	}
	tagByName := map[string]struct{}{}
	for _, t := range ctx.Tags {
		if n := strings.ToLower(strings.TrimSpace(t)); n != "" {
			tagByName[n] = struct{}{}
		}
	}
	const maxWords = 5
	count := 0
	for i := 0; i < len(orig); i++ {
		if consumed[i] {
			continue
		}
		// Try the longest window first so "coffee shops" beats "coffee".
		matched := false
		for w := maxWords; w >= 1 && !matched; w-- {
			j := i + w
			if j > len(orig) {
				continue
			}
			ok := true
			for k := i; k < j; k++ {
				if consumed[k] {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			phrase := strings.ToLower(strings.Join(orig[i:j], " "))
			if id, is := catByName[phrase]; is {
				out.Categories = addCSV(out.Categories, id)
				for k := i; k < j; k++ {
					consumed[k] = true
				}
				count++
				matched = true
			} else if _, is := tagByName[phrase]; is {
				out.Tags = addCSV(out.Tags, phrase)
				for k := i; k < j; k++ {
					consumed[k] = true
				}
				count++
				matched = true
			}
		}
	}
	return count
}

// canonicalizePayee replaces the residue text with a clean payee display name
// when it names (or resolves to) a known merchant, so alias gibberish in a query
// ("amzn") becomes the name the ledger shows ("Amazon"). Otherwise the text is
// returned unchanged. This is a text refinement, not a structured clause.
func canonicalizePayee(text string, ctx Context) string {
	known := map[string]string{}
	for _, p := range ctx.Payees {
		if n := strings.TrimSpace(p); n != "" {
			known[strings.ToLower(n)] = n
		}
	}
	if n, ok := known[strings.ToLower(text)]; ok {
		return n
	}
	if ctx.ResolvePayee != nil {
		if r := strings.TrimSpace(ctx.ResolvePayee(text)); r != "" {
			if n, ok := known[strings.ToLower(r)]; ok {
				return n
			}
			if !strings.EqualFold(r, text) {
				return r
			}
		}
	}
	return text
}

// parseAmount cleans a money token ("$20", "1,000.50", "20") to a plain major-unit
// string, reporting whether it is a usable number. A leading currency symbol and
// thousands separators are stripped; the result must parse as a float.
func parseAmount(tok string) (string, bool) {
	s := strings.TrimSpace(tok)
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return "", false
	}
	if _, err := strconv.ParseFloat(s, 64); err != nil {
		return "", false
	}
	return s, true
}

// addCSV appends val to a comma-joined set unless it is already present.
func addCSV(csv, val string) string {
	if csv == "" {
		return val
	}
	for _, p := range strings.Split(csv, ",") {
		if p == val {
			return csv
		}
	}
	return csv + "," + val
}

// monthNum maps a full or 3-letter month name to its 1–12 number.
func monthNum(w string) (int, bool) {
	switch w {
	case "january", "jan":
		return 1, true
	case "february", "feb":
		return 2, true
	case "march", "mar":
		return 3, true
	case "april", "apr":
		return 4, true
	case "may":
		return 5, true
	case "june", "jun":
		return 6, true
	case "july", "jul":
		return 7, true
	case "august", "aug":
		return 8, true
	case "september", "sep", "sept":
		return 9, true
	case "october", "oct":
		return 10, true
	case "november", "nov":
		return 11, true
	case "december", "dec":
		return 12, true
	}
	return 0, false
}

// dateOnly reduces t to its UTC calendar day at midnight.
func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// yearStart / yearEnd bound a calendar year in the inclusive [Jan 1, Dec 31] form
// the txnfilter date bounds expect.
func yearStart(y int) time.Time { return time.Date(y, time.January, 1, 0, 0, 0, 0, time.UTC) }
func yearEnd(y int) time.Time   { return time.Date(y, time.December, 31, 0, 0, 0, 0, time.UTC) }

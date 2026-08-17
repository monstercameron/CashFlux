// SPDX-License-Identifier: MIT

// Package duedate reads a due date, a repeat cadence, and a clean title out of a
// sentence a person typed — "pay rent friday", "every month on the 1st review
// subscriptions", "call the bank in 3 days" (SM-14).
//
// It is the FREE tier of task quick-add: a small, explicit grammar that needs no
// API key and never leaves the device. The model is only worth asking about the
// phrasings this cannot read, which is the same split internal/nlfilter uses for
// transaction search — and the reason the feature still works for someone who has
// configured no provider at all.
//
// Two properties matter more than breadth of grammar:
//
//   - It never guesses. A phrase it does not recognize is left in the title,
//     where the user can see it, rather than silently resolved to a wrong date.
//   - The words it consumed are reported (Matched), so the UI can show what it
//     read instead of asking for trust.
//
// Pure Go, no syscall/js; the clock is a parameter, so results are deterministic
// and unit-tested on native Go.
package duedate

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Draft is the parse result.
type Draft struct {
	// Title is the sentence with the date and repeat phrases lifted out. It keeps
	// the user's own words and capitalization; only the consumed phrases and any
	// preposition left dangling by them are removed.
	Title string
	// Due is the resolved date at midnight in now's location, or the zero time
	// when the sentence named no date.
	Due time.Time
	// Cadence is the repeat the sentence asked for, or "" for a one-shot.
	Cadence domain.RecurringCadence
	// Matched is the phrase (in the user's words) that produced Due/Cadence, so
	// the UI can show what was read. Empty when nothing was recognized.
	Matched string
}

// Found reports whether anything at all was recognized.
func (d Draft) Found() bool { return !d.Due.IsZero() || d.Cadence != "" }

// weekdays maps the day names and their common short forms onto time.Weekday.
var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday, "tues": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

// months maps month names and their three-letter forms onto time.Month.
var months = map[string]time.Month{
	"january": time.January, "jan": time.January,
	"february": time.February, "feb": time.February,
	"march": time.March, "mar": time.March,
	"april": time.April, "apr": time.April,
	"may":  time.May,
	"june": time.June, "jun": time.June,
	"july": time.July, "jul": time.July,
	"august": time.August, "aug": time.August,
	"september": time.September, "sep": time.September, "sept": time.September,
	"october": time.October, "oct": time.October,
	"november": time.November, "nov": time.November,
	"december": time.December, "dec": time.December,
}

// bareCadences are the single words that name a repeat outright.
var bareCadences = map[string]domain.RecurringCadence{
	"daily": domain.CadenceDaily, "weekly": domain.CadenceWeekly,
	"biweekly": domain.CadenceBiweekly, "fortnightly": domain.CadenceBiweekly,
	"semimonthly": domain.CadenceSemimonthly, "monthly": domain.CadenceMonthly,
	"quarterly": domain.CadenceQuarterly, "yearly": domain.CadenceYearly,
	"annually": domain.CadenceYearly, "annual": domain.CadenceYearly,
}

// unitCadences maps a bare time unit to the cadence "every <unit>" means.
var unitCadences = map[string]domain.RecurringCadence{
	"day": domain.CadenceDaily, "days": domain.CadenceDaily,
	"week": domain.CadenceWeekly, "weeks": domain.CadenceWeekly,
	"month": domain.CadenceMonthly, "months": domain.CadenceMonthly,
	"quarter": domain.CadenceQuarterly, "quarters": domain.CadenceQuarterly,
	"year": domain.CadenceYearly, "years": domain.CadenceYearly,
}

// fillers are words that only ever glued a date phrase to the rest of a sentence.
// They are dropped ONLY when they sit directly against something consumed, so
// "pay the bill" keeps its "the" and "pay rent on friday" loses its "on".
var fillers = map[string]bool{
	"on": true, "by": true, "due": true, "at": true, "before": true,
	"this": true, "the": true, "of": true, "starting": true, "start": true,
}

// parser holds one parse in progress: the original words, their lower-cased
// forms, and which have been consumed.
type parser struct {
	orig     []string
	low      []string
	consumed []bool
	now      time.Time
}

// Parse reads s against now (its date and location are the reference point) and
// returns the draft. A sentence with no recognizable date and no repeat comes
// back with the title equal to the trimmed input, which is exactly what a caller
// should put in the title field.
func Parse(s string, now time.Time) Draft {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return Draft{}
	}
	p := &parser{
		orig:     fields,
		low:      make([]string, len(fields)),
		consumed: make([]bool, len(fields)),
		now:      dayOf(now),
	}
	for i, f := range fields {
		p.low[i] = strings.ToLower(strings.TrimFunc(f, isTrailingPunct))
	}

	cadence := p.takeCadence()
	due, ok := p.takeDate()
	// A repeat with no start date starts at its first occurrence from today, so
	// "every month" lands on a real date rather than an empty due field.
	if !ok && cadence != "" {
		due, ok = cadence.Next(p.now), true
	}

	d := Draft{Cadence: cadence}
	if ok {
		d.Due = due
	}
	d.Matched = p.matchedPhrase()
	d.Title = p.title()
	if d.Title == "" {
		// Everything was a date phrase. Better to hand back the user's words than
		// an empty task the UI would have to reject.
		d.Title = strings.TrimSpace(s)
	}
	return d
}

// take marks a span consumed.
func (p *parser) take(from, to int) {
	for i := from; i < to && i < len(p.consumed); i++ {
		p.consumed[i] = true
	}
}

// free reports whether index i exists and is still unconsumed.
func (p *parser) free(i int) bool { return i >= 0 && i < len(p.low) && !p.consumed[i] }

// word returns the lower-cased token at i, or "" when out of range/consumed.
func (p *parser) word(i int) string {
	if !p.free(i) {
		return ""
	}
	return p.low[i]
}

// takeCadence lifts a repeat phrase: a bare cadence word ("weekly"), "every
// <unit>", "every <n> <unit>", or "every <weekday>". It does NOT consume the
// weekday in the last form — that is also the start date, and takeDate reads it.
func (p *parser) takeCadence() domain.RecurringCadence {
	for i := range p.low {
		if !p.free(i) {
			continue
		}
		if c, ok := bareCadences[p.low[i]]; ok {
			p.take(i, i+1)
			return c
		}
		if p.low[i] != "every" && p.low[i] != "each" {
			continue
		}
		// "every other week" — a fortnight by another name.
		if p.word(i+1) == "other" {
			if c, ok := unitCadences[p.word(i+2)]; ok {
				p.take(i, i+3)
				return doubled(c)
			}
			continue
		}
		// "every 2 weeks"
		if n, err := strconv.Atoi(p.word(i + 1)); err == nil && n > 0 {
			if c, ok := unitCadences[p.word(i+2)]; ok {
				p.take(i, i+3)
				if n == 2 {
					return doubled(c)
				}
				return c
			}
			continue
		}
		// "every month"
		if c, ok := unitCadences[p.word(i+1)]; ok {
			p.take(i, i+2)
			return c
		}
		// "every friday" — weekly, and the weekday still dates the first one.
		if _, ok := weekdays[p.word(i+1)]; ok {
			p.take(i, i+1)
			return domain.CadenceWeekly
		}
	}
	return ""
}

// doubled maps a cadence onto its every-other form where the app has one.
func doubled(c domain.RecurringCadence) domain.RecurringCadence {
	if c == domain.CadenceWeekly {
		return domain.CadenceBiweekly
	}
	return c
}

// takeDate lifts the first date phrase it recognizes, most specific form first so
// "jan 5" is never read as the bare ordinal "5".
func (p *parser) takeDate() (time.Time, bool) {
	for _, try := range []func() (time.Time, bool){
		p.tryISO, p.trySlash, p.tryMonthDay, p.tryRelativeWord,
		p.tryInN, p.tryNextUnit, p.tryWeekday, p.tryOrdinal,
	} {
		if d, ok := try(); ok {
			return d, true
		}
	}
	return time.Time{}, false
}

// tryISO reads 2026-08-20.
func (p *parser) tryISO() (time.Time, bool) {
	for i := range p.low {
		if !p.free(i) || len(p.low[i]) != 10 {
			continue
		}
		t, err := time.ParseInLocation("2006-01-02", p.low[i], p.now.Location())
		if err != nil {
			continue
		}
		p.take(i, i+1)
		return t, true
	}
	return time.Time{}, false
}

// trySlash reads 8/20 and 8/20/2026 (and the two-digit year form).
func (p *parser) trySlash() (time.Time, bool) {
	for i := range p.low {
		if !p.free(i) || !strings.Contains(p.low[i], "/") {
			continue
		}
		parts := strings.Split(p.low[i], "/")
		if len(parts) < 2 || len(parts) > 3 {
			continue
		}
		m, err1 := strconv.Atoi(parts[0])
		d, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil || m < 1 || m > 12 || d < 1 || d > 31 {
			continue
		}
		year := p.now.Year()
		if len(parts) == 3 {
			y, err := strconv.Atoi(parts[2])
			if err != nil {
				continue
			}
			if y < 100 {
				y += 2000
			}
			year = y
		}
		t := time.Date(year, time.Month(m), d, 0, 0, 0, 0, p.now.Location())
		if len(parts) == 2 && t.Before(p.now) {
			t = t.AddDate(1, 0, 0) // a bare month/day already past means next year
		}
		p.take(i, i+1)
		return t, true
	}
	return time.Time{}, false
}

// tryMonthDay reads "jan 5", "january 5th", and the reversed "5 jan".
func (p *parser) tryMonthDay() (time.Time, bool) {
	for i := range p.low {
		if !p.free(i) {
			continue
		}
		if m, ok := months[p.low[i]]; ok {
			if d, ok := ordinal(p.word(i + 1)); ok {
				p.take(i, i+2)
				return p.monthDay(m, d), true
			}
			continue
		}
		if d, ok := ordinal(p.low[i]); ok {
			if m, ok := months[p.word(i+1)]; ok {
				p.take(i, i+2)
				return p.monthDay(m, d), true
			}
		}
	}
	return time.Time{}, false
}

// monthDay resolves a month/day to this year, rolling to next year when the date
// has already passed — "jan 5" said in December means the January coming.
func (p *parser) monthDay(m time.Month, day int) time.Time {
	t := time.Date(p.now.Year(), m, day, 0, 0, 0, 0, p.now.Location())
	if t.Before(p.now) {
		t = t.AddDate(1, 0, 0)
	}
	return t
}

// tryRelativeWord reads today / tonight / tomorrow / yesterday.
func (p *parser) tryRelativeWord() (time.Time, bool) {
	for i := range p.low {
		if !p.free(i) {
			continue
		}
		switch p.low[i] {
		case "today", "tonight":
			p.take(i, i+1)
			return p.now, true
		case "tomorrow", "tmrw":
			p.take(i, i+1)
			return p.now.AddDate(0, 0, 1), true
		case "yesterday":
			p.take(i, i+1)
			return p.now.AddDate(0, 0, -1), true
		}
	}
	return time.Time{}, false
}

// tryInN reads "in 3 days" / "in 2 weeks" / "in a month".
func (p *parser) tryInN() (time.Time, bool) {
	for i := range p.low {
		if !p.free(i) || p.low[i] != "in" {
			continue
		}
		n, ok := 0, false
		if v, err := strconv.Atoi(p.word(i + 1)); err == nil && v > 0 {
			n, ok = v, true
		} else if w := p.word(i + 1); w == "a" || w == "an" || w == "one" {
			n, ok = 1, true
		}
		if !ok {
			continue
		}
		switch p.word(i + 2) {
		case "day", "days":
			p.take(i, i+3)
			return p.now.AddDate(0, 0, n), true
		case "week", "weeks":
			p.take(i, i+3)
			return p.now.AddDate(0, 0, 7*n), true
		case "month", "months":
			p.take(i, i+3)
			return p.now.AddDate(0, n, 0), true
		case "year", "years":
			p.take(i, i+3)
			return p.now.AddDate(n, 0, 0), true
		}
	}
	return time.Time{}, false
}

// tryNextUnit reads "next week" / "next month" / "next year".
func (p *parser) tryNextUnit() (time.Time, bool) {
	for i := range p.low {
		if !p.free(i) || p.low[i] != "next" {
			continue
		}
		switch p.word(i + 1) {
		case "week":
			p.take(i, i+2)
			return p.now.AddDate(0, 0, 7), true
		case "month":
			p.take(i, i+2)
			return p.now.AddDate(0, 1, 0), true
		case "year":
			p.take(i, i+2)
			return p.now.AddDate(1, 0, 0), true
		}
	}
	return time.Time{}, false
}

// tryWeekday reads "friday", "this friday", and "next friday".
//
// A bare weekday means the SOONEST one, today included: someone typing "pay rent
// friday" on a Friday means today, and pushing it a week would quietly make the
// task late. "next friday" always skips to the following week's.
func (p *parser) tryWeekday() (time.Time, bool) {
	for i := range p.low {
		if !p.free(i) {
			continue
		}
		wd, ok := weekdays[p.low[i]]
		if !ok {
			continue
		}
		skipWeek := false
		from := i
		if j := i - 1; p.free(j) {
			switch p.low[j] {
			case "next":
				skipWeek, from = true, j
			case "this", "on", "by", "coming":
				from = j
			}
		}
		delta := (int(wd) - int(p.now.Weekday()) + 7) % 7
		if skipWeek {
			// "next <weekday>" is the one in the following week, so a same-name day
			// later THIS week is not it.
			if delta == 0 {
				delta = 7
			}
			delta += 7
		}
		p.take(from, i+1)
		return p.now.AddDate(0, 0, delta), true
	}
	return time.Time{}, false
}

// tryOrdinal reads a bare day-of-month: "the 15th", "on the 1st".
//
// It requires the ordinal SUFFIX (15th, not 15). A bare number in a task sentence
// is far more often an amount or a count than a day, and guessing there produces
// exactly the silent wrong date this package refuses to make.
func (p *parser) tryOrdinal() (time.Time, bool) {
	for i := range p.low {
		if !p.free(i) || !hasOrdinalSuffix(p.low[i]) {
			continue
		}
		d, ok := ordinal(p.low[i])
		if !ok || d < 1 || d > 31 {
			continue
		}
		from := i
		if j := i - 1; p.free(j) && (p.low[j] == "the" || p.low[j] == "on") {
			from = j
			if k := j - 1; p.free(k) && p.low[k] == "on" {
				from = k
			}
		}
		t := time.Date(p.now.Year(), p.now.Month(), d, 0, 0, 0, 0, p.now.Location())
		if t.Before(p.now) {
			t = addMonth(t)
		}
		p.take(from, i+1)
		return t, true
	}
	return time.Time{}, false
}

// addMonth advances one month, clamping to the last day of a shorter target
// month so the 31st of a 31-day month never lands in the month after next.
func addMonth(t time.Time) time.Time {
	y, m, d := t.Year(), t.Month(), t.Day()
	first := time.Date(y, m, 1, 0, 0, 0, 0, t.Location()).AddDate(0, 1, 0)
	last := first.AddDate(0, 1, -1).Day()
	if d > last {
		d = last
	}
	return time.Date(first.Year(), first.Month(), d, 0, 0, 0, 0, t.Location())
}

// matchedPhrase renders the consumed words, in order, in the user's own spelling.
func (p *parser) matchedPhrase() string {
	var out []string
	for i, c := range p.consumed {
		if c {
			out = append(out, p.orig[i])
		}
	}
	return strings.Join(out, " ")
}

// title rebuilds the sentence from the unconsumed words, dropping any filler left
// stranded against a consumed span (the "on" in "pay rent on friday").
func (p *parser) title() string {
	keep := make([]bool, len(p.orig))
	for i := range p.orig {
		keep[i] = !p.consumed[i]
	}
	for i := range p.orig {
		if !keep[i] || !fillers[p.low[i]] {
			continue
		}
		if (i > 0 && p.consumed[i-1]) || (i+1 < len(p.consumed) && p.consumed[i+1]) {
			keep[i] = false
		}
	}
	var out []string
	for i, k := range keep {
		if k {
			out = append(out, p.orig[i])
		}
	}
	return strings.TrimSpace(strings.TrimRight(strings.Join(out, " "), " ,;:-"))
}

// ordinal parses "5", "5th", "1st", "22nd" into a day number.
func ordinal(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	digits := strings.TrimRightFunc(s, func(r rune) bool { return !unicode.IsDigit(r) })
	suffix := s[len(digits):]
	if suffix != "" && suffix != "st" && suffix != "nd" && suffix != "rd" && suffix != "th" {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n < 1 || n > 31 {
		return 0, false
	}
	return n, true
}

// hasOrdinalSuffix reports whether s ends in an ordinal marker (15th, 1st).
func hasOrdinalSuffix(s string) bool {
	for _, suf := range []string{"st", "nd", "rd", "th"} {
		if strings.HasSuffix(s, suf) && len(s) > len(suf) && unicode.IsDigit(rune(s[len(s)-len(suf)-1])) {
			return true
		}
	}
	return false
}

// isTrailingPunct is the trim set for tokens; a token's meaning never depends on
// the comma or period stuck to it.
func isTrailingPunct(r rune) bool {
	return r == ',' || r == '.' || r == ';' || r == ':' || r == '!' || r == '?'
}

// dayOf strips the clock off a time, keeping its location.
func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

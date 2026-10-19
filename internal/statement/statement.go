// SPDX-License-Identifier: MIT

// Package statement parses a delimited bank/card statement (CSV, semicolon, tab, or
// pipe) into normalized transaction rows — the pure extraction + column-mapping core
// of the import engine (C74). It detects the delimiter, maps header columns to roles
// by name, and normalizes the messy real-world value formats: amounts in parentheses
// or with currency symbols, thousands separators, and DR/CR markers; separate
// debit/credit columns; and several common date layouts.
//
// Amounts come out as signed minor units (credits positive, debits negative).
// Date and number formats are inferred per file and can be overridden by callers.
// Pure Go, no syscall/js; unit-tested on native Go.
package statement

import (
	"encoding/csv"
	"errors"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

// Row is one normalized statement line.
type Row struct {
	Date        time.Time
	Description string
	Amount      int64 // signed minor units: credit positive, debit negative
	Balance     int64
	HasBalance  bool
	Category    string
}

// Columns maps each role to a 0-based column index, or -1 when the statement has no
// such column. Amount is used directly when present; otherwise Credit minus Debit.
type Columns struct {
	Date        int
	Description int
	Amount      int
	Debit       int
	Credit      int
	Balance     int
}

// RowError records a value row that could not be normalized (1-based line number).
type RowError struct {
	Line int
	Err  string
}

// Statement is the parse result: the detected column mapping, the normalized rows,
// and any per-row errors (a bad row is skipped, not fatal).
type Statement struct {
	Delimiter        rune
	Columns          Columns
	Rows             []Row
	Errors           []RowError
	NumberFormat     NumberFormat
	AmbiguousNumbers int
	DateOrder        DateOrder
	AmbiguousDates   int
}

// NumberFormat describes the decimal and grouping separators used by numeric
// amount columns. A zero separator in ParseOptions means infer it from the file.
type NumberFormat struct {
	DecimalSep rune
	GroupSep   rune
}

// DateOrder is the interpretation of numeric dates with day and month both <= 12.
type DateOrder string

const (
	// MonthDay reads 01/02/2026 as January 2, 2026.
	MonthDay DateOrder = "MDY"
	// DayMonth reads 01/02/2026 as February 1, 2026.
	DayMonth DateOrder = "DMY"
)

// ParseOptions supplies user preferences or explicit overrides. A zero
// NumberFormat or DateOrder asks the parser to infer from unambiguous file values;
// if dates remain ambiguous, MonthDay is the fallback until the caller supplies
// the account or user preference.
type ParseOptions struct {
	NumberFormat NumberFormat
	DateOrder    DateOrder
}

// DetectDelimiter picks the most likely field delimiter from a header line by
// counting candidates (comma, semicolon, tab, pipe); comma wins ties and an
// empty or delimiter-free line.
func DetectDelimiter(line string) rune {
	best, bestN := ',', -1
	for _, d := range []rune{',', ';', '\t', '|'} {
		if n := strings.Count(line, string(d)); n > bestN {
			best, bestN = d, n
		}
	}
	return best
}

// MapColumns assigns header columns to roles by case-insensitive name match. The
// first column matching a role wins; unmatched roles are -1.
func MapColumns(header []string) Columns {
	c := Columns{Date: -1, Description: -1, Amount: -1, Debit: -1, Credit: -1, Balance: -1}
	for i, raw := range header {
		h := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case c.Date < 0 && (containsAny(h, "date", "posted")):
			c.Date = i
		case c.Debit < 0 && containsAny(h, "debit", "withdrawal", "paid out", "money out"):
			c.Debit = i
		case c.Credit < 0 && containsAny(h, "credit", "deposit", "paid in", "money in"):
			c.Credit = i
		case c.Amount < 0 && containsAny(h, "amount", "value"):
			c.Amount = i
		case c.Balance < 0 && containsAny(h, "balance"):
			c.Balance = i
		case c.Description < 0 && containsAny(h, "description", "memo", "payee", "narrative", "details", "name", "reference"):
			c.Description = i
		}
	}
	return c
}

var dateLayouts = []string{
	"2006-01-02", "2006/01/02",
	"01/02/2006", "1/2/2006", "01-02-2006", // MM/DD first
	"02/01/2006", "2/1/2006", "02-01-2006", // then DD/MM (resolves day>12 unambiguously)
	"02 Jan 2006", "2 Jan 2006", "Jan 2, 2006", "02-Jan-2006", "2-Jan-2006", "20060102",
}

// ParseDate parses a statement date in several common layouts (UTC). Numeric
// slash/dash dates are read MM/DD-first, falling back to DD/MM (so an unambiguous
// day > 12 like 15/06/2026 still parses).
func ParseDate(s string) (time.Time, error) { return parseDateOrder(s, MonthDay) }

func parseDateOrder(s string, order DateOrder) (time.Time, error) {
	t := strings.TrimSpace(s)
	layouts := dateLayouts
	if order == DayMonth {
		layouts = []string{"2006-01-02", "2006/01/02", "02/01/2006", "2/1/2006", "02-01-2006", "01/02/2006", "1/2/2006", "01-02-2006", "02 Jan 2006", "2 Jan 2006", "Jan 2, 2006", "02-Jan-2006", "2-Jan-2006", "20060102"}
	}
	for _, layout := range layouts {
		if d, err := time.Parse(layout, t); err == nil {
			return d.UTC(), nil
		}
	}
	return time.Time{}, errors.New("statement: unrecognized date: " + s)
}

// ParseAmount normalizes a statement amount into signed minor units with the given
// decimal places. It handles a leading currency symbol, thousands separators,
// surrounding parentheses (negative), a trailing DR/CR marker (DR negative), and a
// leading +/-. Credits are positive, debits negative.
func ParseAmount(s string, decimals int) (int64, error) {
	return parseAmountFormat(s, decimals, NumberFormat{DecimalSep: '.', GroupSep: ','}, false)
}

func parseAmountFormat(s string, decimals int, format NumberFormat, ambiguous bool) (int64, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, errors.New("statement: empty amount")
	}
	neg := false
	if strings.HasPrefix(t, "(") && strings.HasSuffix(t, ")") {
		neg = !neg
		t = strings.TrimSpace(t[1 : len(t)-1])
	}
	switch up := strings.ToUpper(t); {
	case strings.HasSuffix(up, "DR"):
		neg = !neg
		t = strings.TrimSpace(t[:len(t)-2])
	case strings.HasSuffix(up, "CR"):
		t = strings.TrimSpace(t[:len(t)-2])
	}
	if strings.HasPrefix(t, "-") {
		neg = !neg
		t = t[1:]
	} else if strings.HasPrefix(t, "+") {
		t = t[1:]
	} else if strings.HasSuffix(t, "-") {
		neg = !neg
		t = strings.TrimSpace(t[:len(t)-1])
	}
	t = stripRunes(t, "$£€¥ ")
	if ambiguous {
		return 0, errors.New("statement: ambiguous number format; choose decimal and grouping separators")
	}
	if format.GroupSep != 0 {
		t = strings.ReplaceAll(t, string(format.GroupSep), "")
	}
	if format.DecimalSep != 0 && format.DecimalSep != '.' {
		if strings.Count(t, string(format.DecimalSep)) > 1 {
			return 0, errors.New("statement: invalid number separators")
		}
		t = strings.ReplaceAll(t, string(format.DecimalSep), ".")
	}
	if t == "" {
		return 0, errors.New("statement: empty amount")
	}
	v, err := money.ParseMinor(t, decimals)
	if err != nil {
		return 0, err
	}
	if neg {
		v = -v
	}
	return v, nil
}

// Parse reads a whole delimited statement: it detects the delimiter from the first
// line, treats the first record as a header, maps columns, and normalizes each
// value row. A row missing a date or amount, or with an unparseable one, is
// recorded in Errors and skipped. decimals is the currency's minor-unit precision.
func Parse(text string, decimals int) (Statement, error) {
	return ParseWithOptions(text, decimals, ParseOptions{})
}

// ParseWithOptions parses a statement using optional date and number-format
// overrides. The result reports the detected formats and ambiguous value counts
// so an import UI can present and let the user change those choices before import.
func ParseWithOptions(text string, decimals int, options ParseOptions) (Statement, error) {
	if options.DateOrder != "" && options.DateOrder != MonthDay && options.DateOrder != DayMonth {
		return Statement{}, errors.New("statement: date order must be MDY or DMY")
	}
	if options.NumberFormat.DecimalSep != 0 && options.NumberFormat.DecimalSep == options.NumberFormat.GroupSep {
		return Statement{}, errors.New("statement: decimal and grouping separators must differ")
	}
	text = strings.TrimPrefix(text, "\ufeff") // strip a UTF-8 BOM
	firstLine := text
	if i := strings.IndexAny(text, "\r\n"); i >= 0 {
		firstLine = text[:i]
	}
	st := Statement{Delimiter: DetectDelimiter(firstLine)}

	r := csv.NewReader(strings.NewReader(text))
	r.Comma = st.Delimiter
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return st, err
	}
	if len(records) < 2 {
		return st, errors.New("statement: need a header and at least one row")
	}
	st.Columns = MapColumns(records[0])
	if st.Columns.Date < 0 || (st.Columns.Amount < 0 && st.Columns.Credit < 0 && st.Columns.Debit < 0) {
		return st, errors.New("statement: could not find a date and an amount/debit/credit column")
	}
	dateOrder, ambiguousDates := detectDateOrder(records[1:], st.Columns.Date)
	if options.DateOrder != "" {
		dateOrder = options.DateOrder
	} else if dateOrder == "" {
		dateOrder = MonthDay
	}
	st.DateOrder, st.AmbiguousDates = dateOrder, ambiguousDates
	format, ambiguousCells := detectNumberFormat(records[1:], st.Columns, options.NumberFormat)
	st.NumberFormat = format
	st.AmbiguousNumbers = ambiguousCells

	for i, rec := range records[1:] {
		row, rerr := st.normalize(rec, decimals, format, ambiguousCells > 0 && options.NumberFormat.DecimalSep == 0)
		if rerr != "" {
			st.Errors = append(st.Errors, RowError{Line: i + 2, Err: rerr})
			continue
		}
		st.Rows = append(st.Rows, row)
	}
	return st, nil
}

// normalize turns one raw record into a Row, returning an error string when a
// required field is missing or unparseable.
func (st Statement) normalize(rec []string, decimals int, format NumberFormat, unresolvedNumbers bool) (Row, string) {
	c := st.Columns
	get := func(i int) string {
		if i >= 0 && i < len(rec) {
			return rec[i]
		}
		return ""
	}
	d, err := parseDateOrder(get(c.Date), st.DateOrder)
	if err != nil {
		return Row{}, err.Error()
	}
	row := Row{Date: d, Description: strings.TrimSpace(get(c.Description))}

	switch {
	case c.Amount >= 0 && strings.TrimSpace(get(c.Amount)) != "":
		amt, aerr := parseAmountFormat(get(c.Amount), decimals, format, unresolvedNumbers && hasAmbiguousNumber(get(c.Amount)))
		if aerr != nil {
			return Row{}, aerr.Error()
		}
		row.Amount = amt
	case c.Debit >= 0 || c.Credit >= 0:
		var amt int64
		if s := strings.TrimSpace(get(c.Debit)); s != "" {
			v, derr := parseAmountFormat(s, decimals, format, unresolvedNumbers && hasAmbiguousNumber(s))
			if derr != nil {
				return Row{}, derr.Error()
			}
			amt -= absInt64(v)
		}
		if s := strings.TrimSpace(get(c.Credit)); s != "" {
			v, cerr := parseAmountFormat(s, decimals, format, unresolvedNumbers && hasAmbiguousNumber(s))
			if cerr != nil {
				return Row{}, cerr.Error()
			}
			amt += absInt64(v)
		}
		row.Amount = amt
	default:
		return Row{}, "missing amount"
	}

	if c.Balance >= 0 {
		if s := strings.TrimSpace(get(c.Balance)); s != "" {
			if b, berr := parseAmountFormat(s, decimals, format, unresolvedNumbers && hasAmbiguousNumber(s)); berr == nil {
				row.Balance, row.HasBalance = b, true
			}
		}
	}
	return row, ""
}

func containsAny(h string, subs ...string) bool {
	for _, s := range subs {
		if strings.Contains(h, s) {
			return true
		}
	}
	return false
}

func stripRunes(s, set string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(set, r) {
			return -1
		}
		return r
	}, s)
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func detectDateOrder(records [][]string, column int) (DateOrder, int) {
	var evidence DateOrder
	ambiguous := 0
	conflict := false
	for _, row := range records {
		if column < 0 || column >= len(row) {
			continue
		}
		value := strings.TrimSpace(row[column])
		md, mdErr := parseNumericDate(value, MonthDay)
		dm, dmErr := parseNumericDate(value, DayMonth)
		switch {
		case mdErr == nil && dmErr == nil && !md.Equal(dm):
			ambiguous++
		case mdErr == nil && dmErr != nil:
			if evidence != "" && evidence != MonthDay {
				conflict = true
			}
			evidence = MonthDay
		case dmErr == nil && mdErr != nil:
			if evidence != "" && evidence != DayMonth {
				conflict = true
			}
			evidence = DayMonth
		}
	}
	if conflict {
		return "", ambiguous
	}
	return evidence, ambiguous
}

func parseNumericDate(value string, order DateOrder) (time.Time, error) {
	layouts := []string{"01/02/2006", "1/2/2006", "01-02-2006", "1-2-2006"}
	if order == DayMonth {
		layouts = []string{"02/01/2006", "2/1/2006", "02-01-2006", "2-1-2006"}
	}
	for _, layout := range layouts {
		if d, err := time.Parse(layout, value); err == nil {
			return d, nil
		}
	}
	return time.Time{}, errors.New("not an ambiguous numeric date")
}

func detectNumberFormat(records [][]string, columns Columns, override NumberFormat) (NumberFormat, int) {
	if override.DecimalSep != 0 {
		return override, 0
	}
	format := NumberFormat{}
	votes := map[NumberFormat]int{}
	ambiguous := 0
	clueCount := 0
	indices := []int{columns.Amount, columns.Debit, columns.Credit, columns.Balance}
	for _, row := range records {
		for _, col := range indices {
			if col < 0 || col >= len(row) {
				continue
			}
			candidate, isAmbiguous := numberFormatClue(row[col])
			if isAmbiguous {
				ambiguous++
			}
			if candidate.DecimalSep != 0 {
				votes[candidate]++
				clueCount++
			}
		}
	}
	best, bestVotes, tied := NumberFormat{}, 0, false
	for candidate, count := range votes {
		switch {
		case count > bestVotes:
			best, bestVotes, tied = candidate, count, false
		case count == bestVotes:
			tied = true
		}
	}
	if bestVotes > 0 && !tied {
		format = best
		// Ambiguous individual values are resolved by the rest of the file.
		return format, 0
	}
	if bestVotes > 0 && tied {
		if ambiguous < clueCount {
			ambiguous = clueCount
		}
		return NumberFormat{}, ambiguous
	}
	if ambiguous > 0 {
		return NumberFormat{}, ambiguous
	}
	return NumberFormat{DecimalSep: '.', GroupSep: ','}, 0
}

func numberFormatClue(raw string) (NumberFormat, bool) {
	t := strings.TrimSpace(raw)
	t = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(t, "DR"), "CR"))
	t = strings.Trim(t, "() +-£$€¥")
	dot, comma := strings.LastIndexByte(t, '.'), strings.LastIndexByte(t, ',')
	switch {
	case dot >= 0 && comma >= 0:
		if dot > comma {
			return NumberFormat{DecimalSep: '.', GroupSep: ','}, false
		}
		return NumberFormat{DecimalSep: ',', GroupSep: '.'}, false
	case dot >= 0 || comma >= 0:
		sep := byte('.')
		if comma >= 0 {
			sep = ','
		}
		parts := strings.Split(t, string(sep))
		if len(parts) > 2 {
			return NumberFormat{DecimalSep: map[byte]rune{'.': ',', ',': '.'}[sep], GroupSep: rune(sep)}, false
		}
		digits := len(parts[1])
		if digits == 3 {
			return NumberFormat{}, true
		}
		if digits > 0 && digits <= 2 {
			return NumberFormat{DecimalSep: rune(sep), GroupSep: map[byte]rune{'.': ',', ',': '.'}[sep]}, false
		}
	}
	return NumberFormat{}, false
}

func hasAmbiguousNumber(raw string) bool {
	_, ambiguous := numberFormatClue(raw)
	return ambiguous
}

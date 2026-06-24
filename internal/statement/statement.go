// SPDX-License-Identifier: MIT

// Package statement parses a delimited bank/card statement (CSV, semicolon, tab, or
// pipe) into normalized transaction rows — the pure extraction + column-mapping core
// of the import engine (C74). It detects the delimiter, maps header columns to roles
// by name, and normalizes the messy real-world value formats: amounts in parentheses
// or with currency symbols, thousands separators, and DR/CR markers; separate
// debit/credit columns; and several common date layouts.
//
// Amounts come out as signed minor units (credits positive, debits negative).
// Date-format ambiguity (MM/DD vs DD/MM) is resolved MM/DD-first for slash dates.
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
	Delimiter rune
	Columns   Columns
	Rows      []Row
	Errors    []RowError
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
func ParseDate(s string) (time.Time, error) {
	t := strings.TrimSpace(s)
	for _, layout := range dateLayouts {
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
	}
	t = stripRunes(t, "$£€¥, ")
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

	for i, rec := range records[1:] {
		row, rerr := st.normalize(rec, decimals)
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
func (st Statement) normalize(rec []string, decimals int) (Row, string) {
	c := st.Columns
	get := func(i int) string {
		if i >= 0 && i < len(rec) {
			return rec[i]
		}
		return ""
	}
	d, err := ParseDate(get(c.Date))
	if err != nil {
		return Row{}, err.Error()
	}
	row := Row{Date: d, Description: strings.TrimSpace(get(c.Description))}

	switch {
	case c.Amount >= 0 && strings.TrimSpace(get(c.Amount)) != "":
		amt, aerr := ParseAmount(get(c.Amount), decimals)
		if aerr != nil {
			return Row{}, aerr.Error()
		}
		row.Amount = amt
	case c.Debit >= 0 || c.Credit >= 0:
		var amt int64
		if s := strings.TrimSpace(get(c.Debit)); s != "" {
			v, derr := ParseAmount(s, decimals)
			if derr != nil {
				return Row{}, derr.Error()
			}
			amt -= absInt64(v)
		}
		if s := strings.TrimSpace(get(c.Credit)); s != "" {
			v, cerr := ParseAmount(s, decimals)
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
			if b, berr := ParseAmount(s, decimals); berr == nil {
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

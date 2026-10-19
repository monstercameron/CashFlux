// SPDX-License-Identifier: MIT

package importmap

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/statement"
)

// NumberFormat selects the decimal and grouping separators used by a bank export.
type NumberFormat string

const (
	// NumberFormatAuto detects the format from the amount's separators.
	NumberFormatAuto NumberFormat = "auto"
	// NumberFormatDotDecimal uses a dot for decimals and a comma for grouping.
	NumberFormatDotDecimal NumberFormat = "dot-decimal"
	// NumberFormatCommaDecimal uses a comma for decimals and a dot for grouping.
	NumberFormatCommaDecimal NumberFormat = "comma-decimal"
)

// Profile describes how a particular bank's CSV export maps to statement rows.
type Profile struct {
	Name         string       // e.g. "Chase Checking"
	DateCol      int          // 0-based column index, -1 if absent
	DescCol      int          // 0-based column index
	AmountCol    int          // 0-based column index, -1 if absent
	DebitCol     int          // -1 if absent; used when no AmountCol
	CreditCol    int          // -1 if absent
	BalanceCol   int          // -1 if absent
	CurrencyCol  int          // -1 if absent (informational)
	SkipRows     int          // header rows to skip before data rows
	Decimals     int          // minor-unit precision, default 2
	DateLayout   string       // Go time layout; empty = auto-detect
	NumberFormat NumberFormat // decimal separator choice; empty behaves as auto-detect
}

// Apply maps raw CSV rows (already split, after SkipRows) to []statement.Row.
// Rows that cannot be parsed are skipped; use ApplyWithErrors to report them.
func Apply(p Profile, rows [][]string) []statement.Row {
	out, _ := ApplyWithErrors(p, rows)
	return out
}

// RowError describes a mapping row that could not be converted.
type RowError struct {
	// Index is the zero-based position in the supplied rows.
	Index int
	// Field names the value that could not be parsed.
	Field string
	// Err explains why the value could not be parsed.
	Err error
}

// Error returns a row-oriented description suitable for a review message.
func (e RowError) Error() string {
	return fmt.Sprintf("row %d %s: %v", e.Index+1, e.Field, e.Err)
}

// ApplyWithErrors maps rows and reports every row that could not be converted.
// Invalid amounts never become zero-valued rows.
func ApplyWithErrors(p Profile, rows [][]string) ([]statement.Row, []RowError) {
	parsed, errors, _ := ApplyWithDiagnostics(p, rows)
	return parsed, errors
}

// RowWarning describes a value that parsed but could reasonably mean another value.
type RowWarning struct {
	Index   int
	Field   string
	Warning string
}

// ApplyWithDiagnostics maps rows and returns both unparseable rows and ambiguous values.
func ApplyWithDiagnostics(p Profile, rows [][]string) ([]statement.Row, []RowError, []RowWarning) {
	out := make([]statement.Row, 0, len(rows))
	var rowErrors []RowError
	var warnings []RowWarning
	for i, cols := range rows {
		row, ok, field, err, warning := applyRow(p, cols)
		if ok {
			out = append(out, row)
			if warning != "" {
				warnings = append(warnings, RowWarning{Index: i, Field: "date", Warning: warning})
			}
		} else if err != nil {
			rowErrors = append(rowErrors, RowError{Index: i, Field: field, Err: err})
		} else {
			rowErrors = append(rowErrors, RowError{Index: i, Field: field, Err: fmt.Errorf("value is required")})
		}
	}
	return out, rowErrors, warnings
}

func applyRow(p Profile, cols []string) (statement.Row, bool, string, error, string) {
	get := func(idx int) string {
		if idx < 0 || idx >= len(cols) {
			return ""
		}
		return strings.TrimSpace(cols[idx])
	}

	// Date
	var date time.Time
	dateStr := get(p.DateCol)
	if dateStr == "" {
		return statement.Row{}, false, "date", nil, ""
	}
	var err error
	if p.DateLayout != "" {
		date, err = time.Parse(p.DateLayout, dateStr)
	} else {
		date, err = autoParseDate(dateStr)
	}
	if err != nil {
		return statement.Row{}, false, "date", err, ""
	}
	dateWarning := ambiguousDateWarning(dateStr, p.DateLayout)

	// Description
	desc := get(p.DescCol)

	// Amount
	var amount int64
	if p.AmountCol >= 0 {
		amtStr := get(p.AmountCol)
		if amtStr == "" {
			return statement.Row{}, false, "amount", nil, ""
		}
		amount, err = parseMinorUnits(amtStr, p.Decimals, p.NumberFormat)
		if err != nil {
			return statement.Row{}, false, "amount", err, ""
		}
	} else {
		// Debit/Credit columns
		debitStr := get(p.DebitCol)
		creditStr := get(p.CreditCol)
		if debitStr == "" && creditStr == "" {
			return statement.Row{}, false, "amount", nil, ""
		}
		if debitStr != "" {
			d, e := parseMinorUnits(debitStr, p.Decimals, p.NumberFormat)
			if e != nil {
				return statement.Row{}, false, "debit", e, ""
			}
			d, e = money.Abs(d)
			if e != nil {
				return statement.Row{}, false, "debit", e, ""
			}
			amount, e = money.Sub(amount, d) // debit is negative even when the source includes a sign
			if e != nil {
				return statement.Row{}, false, "debit", e, ""
			}
		}
		if creditStr != "" {
			c, e := parseMinorUnits(creditStr, p.Decimals, p.NumberFormat)
			if e != nil {
				return statement.Row{}, false, "credit", e, ""
			}
			c, e = money.Abs(c)
			if e != nil {
				return statement.Row{}, false, "credit", e, ""
			}
			amount, e = money.Add(amount, c)
			if e != nil {
				return statement.Row{}, false, "credit", e, ""
			}
		}
	}

	// Balance
	var balance int64
	hasBalance := false
	if p.BalanceCol >= 0 {
		balStr := get(p.BalanceCol)
		if balStr != "" {
			b, e := parseMinorUnits(balStr, p.Decimals, p.NumberFormat)
			if e != nil {
				return statement.Row{}, false, "balance", e, ""
			}
			balance = b
			hasBalance = true
		}
	}

	return statement.Row{
		Date:        date,
		Description: desc,
		Amount:      amount,
		Balance:     balance,
		HasBalance:  hasBalance,
	}, true, "", nil, dateWarning
}

// DefaultProfile returns a Profile pre-filled from header column names.
func DefaultProfile(name string, header []string, decimals int) Profile {
	p := Profile{
		Name:        name,
		DateCol:     -1,
		DescCol:     -1,
		AmountCol:   -1,
		DebitCol:    -1,
		CreditCol:   -1,
		BalanceCol:  -1,
		CurrencyCol: -1,
		Decimals:    decimals,
	}
	for i, h := range header {
		lower := strings.ToLower(strings.TrimSpace(h))
		switch lower {
		case "date", "transaction date", "posted date", "post date":
			if p.DateCol < 0 {
				p.DateCol = i
			}
		case "description", "memo", "name", "payee", "transaction description":
			if p.DescCol < 0 {
				p.DescCol = i
			}
		case "amount":
			if p.AmountCol < 0 {
				p.AmountCol = i
			}
		case "debit":
			if p.DebitCol < 0 {
				p.DebitCol = i
			}
		case "credit":
			if p.CreditCol < 0 {
				p.CreditCol = i
			}
		case "balance", "running balance":
			if p.BalanceCol < 0 {
				p.BalanceCol = i
			}
		case "currency":
			if p.CurrencyCol < 0 {
				p.CurrencyCol = i
			}
		}
	}
	return p
}

var dateLayouts = []string{
	"2006-01-02",
	"01/02/2006",
	"1/2/2006",
	"01/02/06",
	"2006/01/02",
	"Jan 2, 2006",
	"January 2, 2006",
	"02-Jan-2006",
	"20060102",
}

func autoParseDate(s string) (time.Time, error) {
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, strconv.ErrSyntax
}

func ambiguousDateWarning(value, layout string) string {
	if layout != "" {
		return ""
	}
	if len(value) != 10 || value[2] != '/' || value[5] != '/' || !digits(value[:2]) || !digits(value[3:5]) {
		return ""
	}
	first, _ := strconv.Atoi(value[:2])
	second, _ := strconv.Atoi(value[3:5])
	if first > 12 || second > 12 || first == second {
		return ""
	}
	return "This date can mean either month/day or day/month; it was read as month/day."
}

func parseMinorUnits(s string, decimals int, format NumberFormat) (int64, error) {
	s = strings.TrimSpace(s)
	negative := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		negative = true
		s = strings.TrimSpace(s[1 : len(s)-1])
		if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
			return 0, fmt.Errorf("accounting parentheses cannot include a sign")
		}
	} else if strings.ContainsAny(s, "()") {
		return 0, fmt.Errorf("invalid accounting amount %q", s)
	}
	if s != "" && (s[0] == '-' || s[0] == '+') {
		if s[0] == '-' {
			if negative {
				return 0, fmt.Errorf("amount has multiple negative signs")
			}
			negative = true
		}
		s = strings.TrimSpace(s[1:])
	}
	for _, symbol := range []string{"$", "£", "€", "¥"} {
		s = strings.ReplaceAll(s, symbol, "")
	}
	s = strings.ReplaceAll(strings.TrimSpace(s), " ", "")
	if negative && (strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+")) {
		return 0, fmt.Errorf("accounting parentheses cannot include a sign")
	}
	decimal, group, err := separators(s, decimals, format)
	if err != nil {
		return 0, err
	}
	if group != 0 {
		parts := strings.Split(s, string(decimal))
		if len(parts) > 2 {
			return 0, fmt.Errorf("invalid amount %q", s)
		}
		whole := parts[0]
		if strings.ContainsRune(whole, group) {
			groups := strings.Split(whole, string(group))
			if len(groups[0]) < 1 || len(groups[0]) > 3 || !digits(groups[0]) {
				return 0, fmt.Errorf("invalid digit grouping in %q", s)
			}
			for _, part := range groups[1:] {
				if len(part) != 3 || !digits(part) {
					return 0, fmt.Errorf("invalid digit grouping in %q", s)
				}
			}
		}
		s = strings.ReplaceAll(s, string(group), "")
	}
	if decimal == ',' {
		s = strings.ReplaceAll(s, ",", ".")
	}
	value, err := money.ParseMinor(s, decimals)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", s, err)
	}
	if negative && value > 0 {
		value = -value
	}
	return value, nil
}

func separators(s string, decimals int, format NumberFormat) (decimal, group rune, err error) {
	switch format {
	case "", NumberFormatAuto:
		comma, dot := strings.LastIndexByte(s, ','), strings.LastIndexByte(s, '.')
		switch {
		case comma >= 0 && dot >= 0:
			if comma > dot {
				return ',', '.', nil
			}
			return '.', ',', nil
		case comma >= 0:
			if decimals == 3 && comma == len(s)-4 && comma > 0 && digits(s[:comma]) && digits(s[comma+1:]) {
				return '.', ',', nil
			}
			if shouldBeDecimal(s, comma, decimals) {
				return ',', '.', nil
			}
			return '.', ',', nil
		default:
			return '.', ',', nil
		}
	case NumberFormatDotDecimal:
		return '.', ',', nil
	case NumberFormatCommaDecimal:
		return ',', '.', nil
	default:
		return 0, 0, fmt.Errorf("unsupported number format %q", format)
	}
}

func shouldBeDecimal(s string, sep int, decimals int) bool {
	digitsAfter := len(s) - sep - 1
	return digitsAfter > 0 && digitsAfter <= decimals
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

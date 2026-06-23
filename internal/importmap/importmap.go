package importmap

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/statement"
)

// Profile describes how a particular bank's CSV export maps to statement rows.
type Profile struct {
	Name        string // e.g. "Chase Checking"
	DateCol     int    // 0-based column index, -1 if absent
	DescCol     int    // 0-based column index
	AmountCol   int    // 0-based column index, -1 if absent
	DebitCol    int    // -1 if absent; used when no AmountCol
	CreditCol   int    // -1 if absent
	BalanceCol  int    // -1 if absent
	CurrencyCol int    // -1 if absent (informational)
	SkipRows    int    // header rows to skip before data rows
	Decimals    int    // minor-unit precision, default 2
	DateLayout  string // Go time layout; empty = auto-detect
}

// Apply maps raw CSV rows (already split, after SkipRows) to []statement.Row.
// Rows that cannot be parsed (bad date, bad amount) are silently skipped.
func Apply(p Profile, rows [][]string) []statement.Row {
	out := make([]statement.Row, 0, len(rows))
	for _, cols := range rows {
		row, ok := applyRow(p, cols)
		if ok {
			out = append(out, row)
		}
	}
	return out
}

func applyRow(p Profile, cols []string) (statement.Row, bool) {
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
		return statement.Row{}, false
	}
	var err error
	if p.DateLayout != "" {
		date, err = time.Parse(p.DateLayout, dateStr)
	} else {
		date, err = autoParseDate(dateStr)
	}
	if err != nil {
		return statement.Row{}, false
	}

	// Description
	desc := get(p.DescCol)

	// Amount
	var amount int64
	if p.AmountCol >= 0 {
		amtStr := get(p.AmountCol)
		if amtStr == "" {
			return statement.Row{}, false
		}
		amount, err = parseMinorUnits(amtStr, p.Decimals)
		if err != nil {
			return statement.Row{}, false
		}
	} else {
		// Debit/Credit columns
		debitStr := get(p.DebitCol)
		creditStr := get(p.CreditCol)
		if debitStr == "" && creditStr == "" {
			return statement.Row{}, false
		}
		if debitStr != "" {
			d, e := parseMinorUnits(debitStr, p.Decimals)
			if e == nil {
				amount -= d // debit is negative
			}
		}
		if creditStr != "" {
			c, e := parseMinorUnits(creditStr, p.Decimals)
			if e == nil {
				amount += c
			}
		}
	}

	// Balance
	var balance int64
	hasBalance := false
	if p.BalanceCol >= 0 {
		balStr := get(p.BalanceCol)
		if balStr != "" {
			b, e := parseMinorUnits(balStr, p.Decimals)
			if e == nil {
				balance = b
				hasBalance = true
			}
		}
	}

	return statement.Row{
		Date:        date,
		Description: desc,
		Amount:      amount,
		Balance:     balance,
		HasBalance:  hasBalance,
	}, true
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

func parseMinorUnits(s string, decimals int) (int64, error) {
	// Remove currency symbols and commas
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimLeft(s, "$£€¥")
	s = strings.TrimSpace(s)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	factor := 1.0
	for i := 0; i < decimals; i++ {
		factor *= 10
	}
	// Round to nearest
	v := f * factor
	if v < 0 {
		v -= 0.5
	} else {
		v += 0.5
	}
	return int64(v), nil
}

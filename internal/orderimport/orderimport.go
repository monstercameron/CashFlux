// SPDX-License-Identifier: MIT

// Package orderimport parses a retailer's own order-history export (TX4) into a
// structured order list and matches those orders to the card transactions that
// paid for them. It is local-first and pure: no network, no scraping, no
// syscall/js — the two inputs are a privacy-export CSV and a copy-paste of the
// orders page, both fed in as strings, and it unit-tests on native Go.
//
// The flagship retailer is Amazon (Amazon retired the self-serve order-report
// CSV in 2023, so the dependable local inputs are the "Request My Data" privacy
// export — Retail.OrderHistory.*.csv — and a paste from the orders page). Column
// names in the privacy export drift, so ParseRetailCSV maps headers by FUZZY
// match rather than fixed position; ParseOrdersPaste reads the semi-structured
// page text leniently, best-effort. Other retailers land later as new parsers
// against the same Order shape.
package orderimport

import (
	"encoding/csv"
	"strings"
	"time"
)

// Item is one line of an order: a product name and its per-unit price and
// quantity. Prices are integer minor units (positive magnitude).
type Item struct {
	Name      string
	UnitMinor int64
	Qty       int
}

// LineTotalMinor returns the item's extended cost (unit price × quantity).
func (it Item) LineTotalMinor() int64 {
	q := it.Qty
	if q <= 0 {
		q = 1
	}
	return it.UnitMinor * int64(q)
}

// Order is one parsed purchase: a stable order id, its date, the grand total the
// buyer owed (positive minor units), the currency, and the line items. Items may
// be empty for a paste that only exposed the order total.
type Order struct {
	ID         string
	Date       time.Time
	TotalMinor int64
	Currency   string
	Items      []Item
}

// ItemsSubtotalMinor sums the extended cost of every line item. When it differs
// from TotalMinor the gap is tax, shipping, gift-card credit, or a promo — the
// caller states that drift plainly (never hides it).
func (o Order) ItemsSubtotalMinor() int64 {
	var sum int64
	for _, it := range o.Items {
		sum += it.LineTotalMinor()
	}
	return sum
}

// parseMoneyMinor parses a currency-ish string ("$45.99", "1,234.56", "-3.00")
// into signed minor units at two-decimal precision. ok is false when no numeric
// value is present. It is lenient: it strips currency symbols, thousands commas,
// and surrounding whitespace.
func parseMoneyMinor(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	neg := false
	// Keep only digits, a decimal point, and a leading sign.
	var b strings.Builder
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.':
			b.WriteRune(r)
		case (r == '-' || r == '(') && i == 0:
			neg = true
		case r == '-' && b.Len() == 0:
			neg = true
		}
	}
	num := b.String()
	if num == "" || num == "." {
		return 0, false
	}
	whole, frac, hasDot := cut(num, ".")
	var cents int64
	for _, r := range whole {
		cents = cents*10 + int64(r-'0')
	}
	cents *= 100
	if hasDot {
		// Take the first two fractional digits, rounding is not attempted (order
		// totals are exact to the cent).
		f := frac + "00"
		cents += int64(f[0]-'0')*10 + int64(f[1]-'0')
	}
	if neg {
		cents = -cents
	}
	return cents, true
}

// cut splits s around the first occurrence of sep (a tiny strings.Cut for older
// clarity); found reports whether sep was present.
func cut(s, sep string) (before, after string, found bool) {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}

// parseInt parses a small positive integer from a lenient string ("2", "x2",
// "Qty: 3"), defaulting to 1 when no digits are present.
func parseInt(s string) int {
	n, seen := 0, false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
			seen = true
		} else if seen {
			break
		}
	}
	if !seen || n == 0 {
		return 1
	}
	return n
}

// parseDate reads the date formats the two inputs use, most-specific first. It
// returns the zero time when none match (the order still imports; it just won't
// match on date).
func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	formats := []string{
		"2006-01-02T15:04:05Z", "2006-01-02T15:04:05", "2006-01-02 15:04:05",
		"2006-01-02", "01/02/2006", "1/2/2006",
		"January 2, 2006", "Jan 2, 2006", "2 January 2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		}
	}
	return time.Time{}
}

// headerIndex maps a set of fuzzy field aliases to the CSV column index whose
// header best matches, or -1 when none do. Matching is case-insensitive and
// ignores spaces/underscores, so "Total Owed", "total_owed", and "TotalOwed" all
// hit the same field.
func headerIndex(headers []string, aliases ...string) int {
	norm := func(s string) string {
		s = strings.ToLower(s)
		s = strings.NewReplacer(" ", "", "_", "", "-", "").Replace(s)
		return strings.TrimSpace(s)
	}
	normed := make([]string, len(headers))
	for i, h := range headers {
		normed[i] = norm(h)
	}
	// Exact-normalized match first, then substring.
	for _, a := range aliases {
		na := norm(a)
		for i, h := range normed {
			if h == na {
				return i
			}
		}
	}
	for _, a := range aliases {
		na := norm(a)
		for i, h := range normed {
			if na != "" && strings.Contains(h, na) {
				return i
			}
		}
	}
	return -1
}

// ParseRetailCSV parses a privacy-export order-history CSV into orders, mapping
// columns by fuzzy header match (column names drift between exports). Each data
// row is one shipment/item line carrying its order id, order date, order total,
// product name, unit price, and quantity; rows are grouped by order id into one
// Order with its items. defaultCurrency is used when the CSV carries no currency
// column. Rows without an order id are skipped. A CSV with no recognizable
// header returns no orders and a nil error (nothing to import).
func ParseRetailCSV(data string, defaultCurrency string) ([]Order, error) {
	r := csv.NewReader(strings.NewReader(data))
	r.FieldsPerRecord = -1 // tolerate ragged rows
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, nil
	}
	headers := records[0]
	idCol := headerIndex(headers, "Order ID", "OrderID", "order")
	dateCol := headerIndex(headers, "Order Date", "OrderDate", "date")
	totalCol := headerIndex(headers, "Total Owed", "Order Total", "Grand Total", "total")
	nameCol := headerIndex(headers, "Product Name", "Title", "Item", "name")
	priceCol := headerIndex(headers, "Unit Price", "Purchase Price Per Unit", "Item Price", "price")
	qtyCol := headerIndex(headers, "Quantity", "Qty", "quantity")
	curCol := headerIndex(headers, "Currency", "Currency Code")
	if idCol < 0 {
		return nil, nil // no order id column — nothing dependable to group on
	}

	at := func(row []string, i int) string {
		if i < 0 || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	byID := map[string]*Order{}
	var order []string // preserve first-seen order for a stable result
	for _, row := range records[1:] {
		oid := at(row, idCol)
		if oid == "" {
			continue
		}
		o, ok := byID[oid]
		if !ok {
			cur := at(row, curCol)
			if cur == "" {
				cur = defaultCurrency
			}
			total, _ := parseMoneyMinor(at(row, totalCol))
			o = &Order{
				ID:         oid,
				Date:       parseDate(at(row, dateCol)),
				TotalMinor: absMinor(total),
				Currency:   cur,
			}
			byID[oid] = o
			order = append(order, oid)
		}
		name := at(row, nameCol)
		if name != "" {
			unit, has := parseMoneyMinor(at(row, priceCol))
			if !has {
				unit = 0
			}
			o.Items = append(o.Items, Item{
				Name:      name,
				UnitMinor: absMinor(unit),
				Qty:       parseInt(at(row, qtyCol)),
			})
		}
	}
	out := make([]Order, 0, len(order))
	for _, oid := range order {
		out = append(out, *byID[oid])
	}
	return out, nil
}

func absMinor(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// SPDX-License-Identifier: MIT

package orderimport

import (
	"strings"
)

// ParseOrdersPaste reads a best-effort order list from text pasted off the
// retailer's orders page. The page renders each order as a block introduced by
// an "ORDER PLACED <date>" header with a "TOTAL $x" and an "Order # <id>" line,
// followed by item title lines. Real pastes are noisy (navigation text, prices
// repeated per item), so this parser is deliberately lenient: it starts a new
// order on an "order placed" line, captures the first total/order-number/date it
// sees for that order, and treats other non-empty lines as item titles. Anything
// it cannot classify is ignored. Orders with neither a total nor items are
// dropped. defaultCurrency labels the parsed amounts.
func ParseOrdersPaste(text string, defaultCurrency string) []Order {
	lines := strings.Split(text, "\n")
	var orders []Order
	var cur *Order
	flush := func() {
		if cur != nil && (cur.TotalMinor > 0 || len(cur.Items) > 0) {
			orders = append(orders, *cur)
		}
		cur = nil
	}
	seq := 0
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		low := strings.ToLower(line)

		switch {
		case strings.Contains(low, "order placed"):
			flush()
			seq++
			o := Order{Currency: defaultCurrency}
			if d := parseDate(afterLabel(line, "order placed")); !d.IsZero() {
				o.Date = d
			}
			cur = &o
		case cur == nil:
			// Preamble before the first order header — ignore.
			continue
		case strings.HasPrefix(low, "order #") || strings.HasPrefix(low, "order#") || strings.Contains(low, "order number"):
			if cur.ID == "" {
				cur.ID = strings.TrimSpace(afterAny(line, "order #", "order#", "order number", "order number:"))
			}
		case strings.HasPrefix(low, "total") || strings.Contains(low, "grand total"):
			if cur.TotalMinor == 0 {
				if v, ok := parseMoneyMinor(line); ok {
					cur.TotalMinor = absMinor(v)
				}
			}
		case isItemLine(line):
			// A product title line, optionally with a trailing price. Split a trailing
			// price off the name when present.
			name, unit := splitNamePrice(line)
			if name != "" {
				cur.Items = append(cur.Items, Item{Name: name, UnitMinor: absMinor(unit), Qty: 1})
			}
		}
	}
	flush()

	// Synthesize a stable id for orders that never exposed an order number, so the
	// review card and any created links have something to key on.
	for i := range orders {
		if strings.TrimSpace(orders[i].ID) == "" {
			orders[i].ID = "paste-" + itoa(i+1)
		}
	}
	return orders
}

// afterLabel returns the text after a case-insensitive label occurrence.
func afterLabel(line, label string) string {
	low := strings.ToLower(line)
	if i := strings.Index(low, strings.ToLower(label)); i >= 0 {
		return strings.TrimSpace(line[i+len(label):])
	}
	return ""
}

// afterAny returns the text after the first matching label from labels.
func afterAny(line string, labels ...string) string {
	for _, l := range labels {
		if s := afterLabel(line, l); s != "" || strings.Contains(strings.ToLower(line), strings.ToLower(l)) {
			return strings.TrimLeft(s, ": ")
		}
	}
	return ""
}

// isItemLine reports whether a line looks like a product title rather than
// metadata (dates, statuses, delivery lines). It is a heuristic: a reasonably
// long line that is not a known metadata prefix.
func isItemLine(line string) bool {
	low := strings.ToLower(line)
	for _, skip := range []string{"delivered", "arriving", "shipped", "return", "buy it again",
		"track package", "view order", "get product support", "leave", "write a review", "ship to"} {
		if strings.Contains(low, skip) {
			return false
		}
	}
	return len([]rune(line)) >= 8
}

// splitNamePrice separates a trailing "$x.xx" price off a product-title line.
// When no price is present the whole line is the name and unit is 0.
func splitNamePrice(line string) (name string, unit int64) {
	if i := strings.LastIndex(line, "$"); i > 0 {
		if v, ok := parseMoneyMinor(line[i:]); ok {
			return strings.TrimSpace(strings.TrimRight(line[:i], " -\t")), v
		}
	}
	return strings.TrimSpace(line), 0
}

// itoa renders a small non-negative int without importing strconv into this
// synthetic-id path.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

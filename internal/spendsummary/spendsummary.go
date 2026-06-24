// SPDX-License-Identifier: MIT

// Package spendsummary turns a set of extracted document rows into a per-month
// spend summary — how much went out and came in each calendar month — so the
// Documents screen can show "what does this statement say I spent?" before any
// rows are imported as transactions.
//
// It follows the import flow's amount convention: negative amounts are expenses
// (spend), positive amounts are money in. Amounts are kept as integer minor
// units; date parsing is deliberately tolerant because document dates arrive in
// varied formats. Nothing is silently dropped: rows with an unreadable date are
// grouped under an empty month and surfaced.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package spendsummary

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
)

// MonthSpend summarizes one calendar month. Amounts are minor units (e.g. cents):
// In totals incoming money (positive amounts), Out totals spend (the absolute
// value of negative amounts), and Count is how many rows fell in this month.
type MonthSpend struct {
	Month string // "YYYY-MM"; empty when the row's date could not be read
	Count int
	In    int64
	Out   int64
}

// Net is money in minus money out for the month (positive = net inflow).
func (m MonthSpend) Net() int64 { return m.In - m.Out }

// dateLayouts are the formats Summarize tries, in order, when reading a row's
// date. The canonical ISO form comes first since the vision prompt asks for it.
var dateLayouts = []string{
	"2006-01-02",
	"2006/01/02",
	"01/02/2006",
	"1/2/2006",
	"2006-01",
	"01/2006",
	"Jan 2, 2006",
	"January 2, 2006",
	"02 Jan 2006",
	"2 Jan 2006",
}

// monthKey reads a row's date and returns its "YYYY-MM" bucket. It reports ok as
// false when no known layout matches, so the caller can group those separately.
func monthKey(date string) (string, bool) {
	s := strings.TrimSpace(date)
	if s == "" {
		return "", false
	}
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01"), true
		}
	}
	return "", false
}

// cleanAmount strips currency symbols, grouping commas, and surrounding spaces so
// the remainder is a plain decimal money.ParseMinor can read.
func cleanAmount(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "$", "")
	return strings.TrimSpace(s)
}

// Summarize buckets rows by calendar month and totals spend (negative amounts)
// versus money in (positive amounts), with amounts read at the given decimal
// precision (2 for most currencies). Months are returned ascending; rows whose
// date can't be parsed are grouped under an empty Month and sorted last. A row
// whose amount can't be parsed still counts toward Count but adds nothing to the
// totals, so the row count stays honest.
func Summarize(rows []extract.Row, decimals int) []MonthSpend {
	byMonth := make(map[string]*MonthSpend)
	for _, r := range rows {
		key, _ := monthKey(r.Date) // empty key collects undated rows
		m := byMonth[key]
		if m == nil {
			m = &MonthSpend{Month: key}
			byMonth[key] = m
		}
		m.Count++
		amt, err := money.ParseMinor(cleanAmount(r.Amount), decimals)
		if err != nil {
			continue
		}
		if amt < 0 {
			m.Out += -amt
		} else {
			m.In += amt
		}
	}

	out := make([]MonthSpend, 0, len(byMonth))
	for _, m := range byMonth {
		out = append(out, *m)
	}
	// Ascending by month; the empty-month (undated) bucket sorts last.
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Month == "") != (out[j].Month == "") {
			return out[j].Month == ""
		}
		return out[i].Month < out[j].Month
	})
	return out
}

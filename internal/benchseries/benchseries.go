// SPDX-License-Identifier: MIT

// Package benchseries holds a user-imported comparison series and aligns it
// against the portfolio's own curve (C380).
//
// The app has no market-data feed and is not getting one — that is the
// local-first constraint, not a gap. But "is my portfolio doing well" is not
// answerable in isolation, and the user already has the answer: every brokerage
// and index provider will hand them a date,value CSV. Importing one turns a line
// with no reference into a comparison.
//
// The hard part is not parsing. It is that the two series are in different
// UNITS: a portfolio is measured in dollars, an index in points, a peer fund in
// its own NAV. Overlaying them raw produces a chart where one line is a flat
// smear at the bottom. So both are INDEXED to 100 at their common start, which
// makes the only honest comparison — relative growth over the same window — the
// one the chart actually shows.
//
// Two consequences are load-bearing and are tested:
//
//   - The comparison window is the OVERLAP. A benchmark that starts later than
//     the portfolio cannot say anything about the earlier months, and stretching
//     it to cover them would invent performance.
//   - Indexing needs a non-zero base. A series whose first in-window value is
//     zero has no meaningful percentage growth, and dividing by it would produce
//     an infinity that renders as a blank chart.
//
// Pure Go, no platform dependencies.
package benchseries

import (
	"encoding/csv"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MaxPoints bounds an imported series. Twenty years of daily closes is about
// 5,000 rows; the cap is well past any window the app charts and exists so a
// pasted file cannot make the store unbounded.
const MaxPoints = 20000

// Point is one dated observation. Value is a plain float because a benchmark is
// not money — it may be index points, a NAV, or a percentage — and forcing it
// into minor units would imply a currency it does not have.
type Point struct {
	Date  time.Time `json:"d"`
	Value float64   `json:"v"`
}

// Series is a named comparison series, oldest first.
type Series struct {
	// Name is what the chart legend calls it ("S&P 500", "My old 401k").
	Name   string  `json:"name"`
	Points []Point `json:"points"`
}

// Empty reports whether the series has anything to compare against.
func (s Series) Empty() bool { return len(s.Points) == 0 }

// Span returns the first and last dates covered, or zeroes when empty.
func (s Series) Span() (time.Time, time.Time) {
	if len(s.Points) == 0 {
		return time.Time{}, time.Time{}
	}
	return s.Points[0].Date, s.Points[len(s.Points)-1].Date
}

// ErrNoRows is returned when a paste contains no usable observation.
var ErrNoRows = errors.New("no dated values found — expected two columns: a date and a value")

// dateLayouts are the formats an exported series realistically uses. ISO first
// because it is unambiguous; the US and European orders are tried after, and a
// two-digit year is not accepted at all — "03/04/05" has three readings and
// guessing one silently would misdate a whole series.
var dateLayouts = []string{
	"2006-01-02", "2006/01/02", "01/02/2006", "02/01/2006",
	"Jan 2, 2006", "2 Jan 2006", "2006-01",
}

// Parse reads a two-column date,value table into a series, sorted oldest-first
// with duplicate dates collapsed to the LAST value seen (an export that revises
// a close should not leave both).
//
// A header row is detected rather than assumed: the first row is skipped only
// when its first cell does not parse as a date. Requiring the caller to declare
// it would be one more thing to get wrong for no benefit.
//
// Unparseable rows are skipped rather than failing the whole import — exports
// carry disclaimer lines and blank separators — but a file with NO usable rows
// is an error, because silently importing an empty series would present as
// success and then show nothing.
func Parse(name, text string) (Series, error) {
	rows, err := readTable(text)
	if err != nil {
		return Series{}, err
	}
	byDay := map[string]Point{}
	for i, cols := range rows {
		if len(cols) < 2 {
			continue
		}
		d, ok := parseDate(strings.TrimSpace(cols[0]))
		if !ok {
			// A first row that is not a date is the header; anything later is junk.
			if i == 0 {
				continue
			}
			continue
		}
		v, vErr := strconv.ParseFloat(cleanNumber(cols[1]), 64)
		if vErr != nil {
			continue
		}
		byDay[d.Format("2006-01-02")] = Point{Date: d, Value: v}
	}
	if len(byDay) == 0 {
		return Series{}, ErrNoRows
	}
	pts := make([]Point, 0, len(byDay))
	for _, p := range byDay {
		pts = append(pts, p)
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].Date.Before(pts[j].Date) })
	if len(pts) > MaxPoints {
		// Keep the most RECENT points: a chart of the last N years is useful, a
		// chart of the first N is not.
		pts = pts[len(pts)-MaxPoints:]
	}
	return Series{Name: strings.TrimSpace(name), Points: pts}, nil
}

func readTable(text string) ([][]string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, ErrNoRows
	}
	comma := ','
	if first, _, _ := strings.Cut(text, "\n"); !strings.Contains(first, ",") && strings.Contains(first, "\t") {
		comma = '\t'
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = comma
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

func parseDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

func cleanNumber(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '+' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ValueAt returns the series value in effect on or before t — the last observed
// close, carried forward. A benchmark quoted weekly must still have a value on a
// Wednesday, and interpolating between points would invent observations the
// source never made.
//
// Reports false when t precedes the series' first observation: carrying the
// first value BACKWARD would claim the index was flat before it existed.
func (s Series) ValueAt(t time.Time) (float64, bool) {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	var out float64
	var ok bool
	for _, p := range s.Points {
		if p.Date.After(day) {
			break
		}
		out, ok = p.Value, true
	}
	return out, ok
}

// Comparison is two series indexed to a shared base of 100 over their common
// window, ready to draw on one axis.
type Comparison struct {
	// Portfolio and Benchmark are indexed values, one per requested date, in the
	// same order as the dates that produced them.
	Portfolio []float64
	Benchmark []float64
	// Dates are the points actually compared — the OVERLAP, which may be shorter
	// than the window asked for.
	Dates []time.Time
	// PortfolioPct and BenchmarkPct are the total percentage change across the
	// compared window.
	PortfolioPct, BenchmarkPct float64
	// Skipped is how many requested dates fell outside the benchmark's coverage.
	// Saying so is what keeps a short overlay honest rather than mysterious.
	Skipped int
}

// Known reports whether there is a comparison worth drawing. Fewer than two
// aligned points is a dot, not a trend.
func (c Comparison) Known() bool { return len(c.Dates) >= 2 }

// LeadPct is how far ahead the portfolio is, in percentage points. Negative
// means behind.
func (c Comparison) LeadPct() float64 { return c.PortfolioPct - c.BenchmarkPct }

// Align indexes a portfolio series and a benchmark to 100 at their common start.
//
// values are the portfolio's own numbers at dates (same length; a mismatch
// yields an empty comparison rather than a silently truncated one). Dates the
// benchmark does not cover are dropped and counted, because a benchmark that
// starts later than the portfolio cannot speak for the earlier months and
// stretching it there would invent performance.
//
// A zero base on either side yields an empty comparison: percentage growth from
// zero is not a number, and dividing by it would render as a blank chart with no
// explanation.
func Align(dates []time.Time, values []float64, bench Series) Comparison {
	var c Comparison
	if len(dates) != len(values) || len(dates) == 0 || bench.Empty() {
		return c
	}
	type pair struct {
		d    time.Time
		p, b float64
	}
	var pairs []pair
	for i, d := range dates {
		bv, ok := bench.ValueAt(d)
		if !ok {
			c.Skipped++
			continue
		}
		pairs = append(pairs, pair{d: d, p: values[i], b: bv})
	}
	if len(pairs) == 0 {
		return c
	}
	pBase, bBase := pairs[0].p, pairs[0].b
	if pBase == 0 || bBase == 0 {
		// Indexing needs a non-zero base. Returning nothing is better than an
		// infinity that draws as an empty chart nobody can explain.
		c.Skipped += len(pairs)
		return c
	}
	for _, pr := range pairs {
		c.Dates = append(c.Dates, pr.d)
		c.Portfolio = append(c.Portfolio, pr.p/pBase*100)
		c.Benchmark = append(c.Benchmark, pr.b/bBase*100)
	}
	c.PortfolioPct = c.Portfolio[len(c.Portfolio)-1] - 100
	c.BenchmarkPct = c.Benchmark[len(c.Benchmark)-1] - 100
	return c
}

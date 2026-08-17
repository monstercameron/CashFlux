// SPDX-License-Identifier: MIT

package benchseries

import (
	"math"
	"strings"
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestParseReadsACommonExport(t *testing.T) {
	s, err := Parse("S&P 500", "Date,Close\n2026-01-02,4700.50\n2026-02-02,4800.00\n2026-03-02,4650.25\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Name != "S&P 500" {
		t.Errorf("Name = %q", s.Name)
	}
	if len(s.Points) != 3 {
		t.Fatalf("got %d points, want 3 (the header must be skipped)", len(s.Points))
	}
	if !s.Points[0].Date.Equal(d(2026, time.January, 2)) || !near(s.Points[0].Value, 4700.50) {
		t.Errorf("first point = %+v", s.Points[0])
	}
	from, to := s.Span()
	if !from.Equal(d(2026, time.January, 2)) || !to.Equal(d(2026, time.March, 2)) {
		t.Errorf("Span = %s..%s", from.Format("2006-01-02"), to.Format("2006-01-02"))
	}
}

func TestParseSortsAndCollapsesDuplicateDates(t *testing.T) {
	// Out of order, with the same day revised. The later value wins — an export
	// that revises a close should not leave both.
	s, err := Parse("x", "2026-03-02,300\n2026-01-02,100\n2026-01-02,110\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Points) != 2 {
		t.Fatalf("got %d points, want 2", len(s.Points))
	}
	if !s.Points[0].Date.Before(s.Points[1].Date) {
		t.Error("points are not oldest-first")
	}
	if !near(s.Points[0].Value, 110) {
		t.Errorf("duplicate date resolved to %v, want the later 110", s.Points[0].Value)
	}
}

// Exports carry disclaimer lines and blank separators; those are skipped. A file
// with NO usable rows is an error, because importing an empty series would
// present as success and then show nothing.
func TestParseSkipsJunkButRefusesAnEmptyResult(t *testing.T) {
	s, err := Parse("x", "Date,Close\nSource: some provider\n2026-01-02,100\n\n,,\nnot a row\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.Points) != 1 {
		t.Errorf("got %d points, want 1", len(s.Points))
	}
	if _, err := Parse("x", "Date,Close\nSource: nothing usable\n"); err != ErrNoRows {
		t.Errorf("err = %v, want ErrNoRows", err)
	}
	if _, err := Parse("x", "   "); err != ErrNoRows {
		t.Errorf("empty input err = %v, want ErrNoRows", err)
	}
}

// A two-digit year has three readings; guessing one silently would misdate a
// whole series.
func TestParseRejectsAmbiguousTwoDigitYears(t *testing.T) {
	if _, err := Parse("x", "03/04/05,100\n06/07/08,200\n"); err != ErrNoRows {
		t.Errorf("a two-digit-year file was accepted: %v", err)
	}
}

func TestParseHandlesTabPaste(t *testing.T) {
	s, err := Parse("x", "Date\tClose\n2026-01-02\t1,234.50\n2026-02-02\t1,300.00\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Points) != 2 || !near(s.Points[0].Value, 1234.50) {
		t.Errorf("points = %+v", s.Points)
	}
}

func TestParseCapsTheSeries(t *testing.T) {
	var b strings.Builder
	base := d(2000, time.January, 1)
	for i := range MaxPoints + 50 {
		b.WriteString(base.AddDate(0, 0, i).Format("2006-01-02"))
		b.WriteString(",1\n")
	}
	s, err := Parse("x", b.String())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Points) != MaxPoints {
		t.Fatalf("got %d points, want the cap %d", len(s.Points), MaxPoints)
	}
	// The MOST RECENT points are kept: a chart of the last N years is useful, a
	// chart of the first N is not.
	last := base.AddDate(0, 0, MaxPoints+49)
	if !s.Points[len(s.Points)-1].Date.Equal(last) {
		t.Errorf("the cap dropped the newest points instead of the oldest")
	}
}

// A benchmark quoted weekly must still have a value on a Wednesday. Carrying the
// last close forward is honest; interpolating would invent observations.
func TestValueAtCarriesTheLastCloseForward(t *testing.T) {
	s, _ := Parse("x", "2026-01-05,100\n2026-01-12,110\n")
	if v, ok := s.ValueAt(d(2026, time.January, 8)); !ok || !near(v, 100) {
		t.Errorf("mid-week = %v,%v want 100,true", v, ok)
	}
	if v, ok := s.ValueAt(d(2026, time.January, 12)); !ok || !near(v, 110) {
		t.Errorf("on the observation = %v,%v", v, ok)
	}
	// After the last point, the last close still stands.
	if v, ok := s.ValueAt(d(2026, time.June, 1)); !ok || !near(v, 110) {
		t.Errorf("after the end = %v,%v", v, ok)
	}
	// BEFORE the first: carrying it backward would claim the index was flat
	// before it existed.
	if _, ok := s.ValueAt(d(2025, time.December, 1)); ok {
		t.Error("a date before the series reported a value")
	}
}

// ─── alignment ───────────────────────────────────────────────────────────────

func TestAlignIndexesBothToOneHundred(t *testing.T) {
	bench, _ := Parse("idx", "2026-01-01,4000\n2026-02-01,4200\n2026-03-01,4400\n")
	dates := []time.Time{d(2026, time.January, 1), d(2026, time.February, 1), d(2026, time.March, 1)}
	// A portfolio that grew 25% against a benchmark that grew 10%.
	vals := []float64{10000, 11000, 12500}

	c := Align(dates, vals, bench)
	if !c.Known() {
		t.Fatal("no comparison")
	}
	if !near(c.Portfolio[0], 100) || !near(c.Benchmark[0], 100) {
		t.Errorf("bases = %v / %v, want 100 / 100", c.Portfolio[0], c.Benchmark[0])
	}
	if !near(c.PortfolioPct, 25) {
		t.Errorf("PortfolioPct = %v, want 25", c.PortfolioPct)
	}
	if !near(c.BenchmarkPct, 10) {
		t.Errorf("BenchmarkPct = %v, want 10", c.BenchmarkPct)
	}
	if !near(c.LeadPct(), 15) {
		t.Errorf("LeadPct = %v, want 15", c.LeadPct())
	}
}

// A benchmark that starts later cannot speak for the earlier months; stretching
// it there would invent performance.
func TestAlignComparesOnlyTheOverlapAndSaysWhatItDropped(t *testing.T) {
	bench, _ := Parse("idx", "2026-02-01,4000\n2026-03-01,4400\n")
	dates := []time.Time{d(2026, time.January, 1), d(2026, time.February, 1), d(2026, time.March, 1)}
	vals := []float64{9000, 10000, 12000}

	c := Align(dates, vals, bench)
	if len(c.Dates) != 2 {
		t.Fatalf("compared %d dates, want 2", len(c.Dates))
	}
	if c.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", c.Skipped)
	}
	if !c.Dates[0].Equal(d(2026, time.February, 1)) {
		t.Errorf("the comparison starts at %s, want February", c.Dates[0].Format("2006-01-02"))
	}
	// Indexed from FEBRUARY, so the portfolio's January value must not affect it.
	if !near(c.PortfolioPct, 20) {
		t.Errorf("PortfolioPct = %v, want 20 (Feb->Mar), not the Jan-based figure", c.PortfolioPct)
	}
}

// Percentage growth from zero is not a number, and dividing by it would render
// as a blank chart nobody can explain.
func TestAlignRefusesAZeroBase(t *testing.T) {
	bench, _ := Parse("idx", "2026-01-01,100\n2026-02-01,110\n")
	dates := []time.Time{d(2026, time.January, 1), d(2026, time.February, 1)}
	if c := Align(dates, []float64{0, 5000}, bench); c.Known() {
		t.Error("a zero portfolio base produced a comparison")
	}
	zeroBench, _ := Parse("idx", "2026-01-01,0\n2026-02-01,110\n")
	if c := Align(dates, []float64{100, 200}, zeroBench); c.Known() {
		t.Error("a zero benchmark base produced a comparison")
	}
}

func TestAlignGuards(t *testing.T) {
	bench, _ := Parse("idx", "2026-01-01,100\n2026-02-01,110\n")
	dates := []time.Time{d(2026, time.January, 1)}
	// A length mismatch must yield nothing, not a silently truncated comparison.
	if c := Align(dates, []float64{1, 2}, bench); c.Known() || len(c.Dates) != 0 {
		t.Errorf("a length mismatch produced %+v", c)
	}
	if c := Align(nil, nil, bench); c.Known() {
		t.Error("empty inputs produced a comparison")
	}
	if c := Align(dates, []float64{1}, Series{}); c.Known() {
		t.Error("an empty benchmark produced a comparison")
	}
	// One aligned point is a dot, not a trend.
	if c := Align(dates, []float64{1}, bench); c.Known() {
		t.Error("a single point reported as a known comparison")
	}
}

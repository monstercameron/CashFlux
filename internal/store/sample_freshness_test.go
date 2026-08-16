// SPDX-License-Identifier: MIT

package store

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// ─── C349: the demo must look healthy whenever it is loaded ──────────────────
//
// The sample carried absolute timestamps, so a first run years after it was
// written opened on "It's been 1464 days since the balance was confirmed"
// fourteen times over in /notifications, 4y+ chips on the Freshness tile, OUT OF
// DATE badges on every /accounts row, and a year-old "charged after
// cancellation" alert. That is a first impression of a neglected ledger, and it
// is the first thing every screenshot and every reviewer sees.
//
// These read the fixture at several plausible "nows" — including one far in the
// future — because the property that matters is that it never rots, not that it
// happens to look right today.
func futureNows() []time.Time {
	return []time.Time{
		time.Date(2026, time.August, 16, 10, 0, 0, 0, time.UTC),
		time.Date(2027, time.March, 3, 10, 0, 0, 0, time.UTC),
		time.Date(2031, time.November, 27, 10, 0, 0, 0, time.UTC),
	}
}

func TestSampleBalancesAreConfirmedRecently(t *testing.T) {
	for _, now := range futureNows() {
		ds := SampleDatasetAt(now)
		if len(ds.Accounts) == 0 {
			t.Fatal("sample has no accounts")
		}
		for _, a := range ds.Accounts {
			if a.BalanceAsOf.IsZero() {
				continue // never-confirmed is its own (deliberate) state
			}
			days := int(now.Sub(a.BalanceAsOf).Hours() / 24)
			if days < 0 {
				t.Errorf("at %s, account %q was confirmed in the FUTURE (%s)",
					now.Format(time.DateOnly), a.Name, a.BalanceAsOf.Format(time.DateOnly))
				continue
			}
			// The 401(k)/Roth deliberately sit past their 90-day override so the
			// stale-balance nudge has something honest to point at; nothing should
			// be a year out.
			if days > 200 {
				t.Errorf("at %s, account %q was last confirmed %d days ago (%s) — the demo "+
					"opens on STALE badges instead of a working ledger (C349)",
					now.Format(time.DateOnly), a.Name, days, a.BalanceAsOf.Format(time.DateOnly))
			}
		}
	}
}

func TestSampleLedgerRunsUpToToday(t *testing.T) {
	for _, now := range futureNows() {
		ds := SampleDatasetAt(now)
		var latest time.Time
		for _, tx := range ds.Transactions {
			if tx.Date.After(latest) {
				latest = tx.Date
			}
		}
		if latest.IsZero() {
			t.Fatal("sample has no transactions")
		}
		if gap := int(now.Sub(latest).Hours() / 24); gap > 45 {
			t.Errorf("at %s, the newest transaction is %d days old (%s) — every trend, "+
				"report and forecast opens on a dead ledger (C349)",
				now.Format(time.DateOnly), gap, latest.Format(time.DateOnly))
		}
	}
}

// The history has to keep its depth as well as its recency: the whole point of
// sixty months is that charts and year-over-year comparisons have something to
// draw.
func TestSampleKeepsFiveYearsOfHistory(t *testing.T) {
	now := time.Date(2029, time.February, 14, 0, 0, 0, 0, time.UTC)
	ds := SampleDatasetAt(now)
	earliest := now
	for _, tx := range ds.Transactions {
		if tx.Date.Before(earliest) {
			earliest = tx.Date
		}
	}
	if months := int(now.Sub(earliest).Hours() / 24 / 30.4); months < 58 {
		t.Errorf("history spans ~%d months, want ~60 — the shift moved the window "+
			"instead of sliding it", months)
	}
}

// Seasonality has to survive the shift, or the demo reads as a household with no
// calendar: holiday gifts belong in December whatever month the fixture is
// loaded in.
func TestSeasonalEventsFollowTheShiftedCalendar(t *testing.T) {
	for _, now := range futureNows() {
		ds := SampleDatasetAt(now)
		gifts, offSeason := 0, 0
		for _, tx := range ds.Transactions {
			if tx.CategoryID != "cat-gifts" || !strings.Contains(tx.Desc, "Holiday") {
				continue
			}
			gifts++
			if tx.Date.Month() != time.December {
				offSeason++
			}
		}
		if gifts < 4 {
			t.Fatalf("at %s: found %d holiday-gift charges, want one per modelled December",
				now.Format(time.DateOnly), gifts)
		}
		if offSeason > 0 {
			t.Errorf("at %s: %d of %d holiday-gift charges fell outside December (C349)",
				now.Format(time.DateOnly), offSeason, gifts)
		}
	}
}

// Prose that names a date must name the SHIFTED one, or the demo's own copy
// contradicts its ledger.
func TestDatedCopyFollowsTheShiftedTimeline(t *testing.T) {
	now := time.Date(2028, time.May, 9, 0, 0, 0, 0, time.UTC)
	ds := SampleDatasetAt(now)

	var baby domain.Goal
	for _, g := range ds.Goals {
		if g.ID == "goal-baby" {
			baby = g
		}
	}
	if baby.ID == "" {
		t.Fatal("goal-baby missing")
	}
	if want := baby.TargetDate.Format("Jan 2006"); !strings.Contains(baby.Name, want) {
		t.Errorf("goal name %q does not name its own target date %q", baby.Name, want)
	}
	for _, ins := range ds.SavedInsights {
		if strings.Contains(ins.Text, "2023") || strings.Contains(ins.Text, "2026") {
			t.Errorf("insight %q still carries an authored year: %q", ins.ID, ins.Text)
		}
	}
}

// shiftMonths is the whole mechanism; a day that does not exist in the target
// month must clamp rather than spill into the next one.
func TestShiftMonthsClampsShortMonths(t *testing.T) {
	jan31 := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	if got := shiftMonths(jan31, 1); got.Month() != time.February || got.Day() != 28 {
		t.Errorf("Jan 31 + 1mo = %s, want Feb 28", got.Format(time.DateOnly))
	}
	// Across a year boundary, forwards and backwards.
	dec15 := time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC)
	if got := shiftMonths(dec15, 2); got.Year() != 2027 || got.Month() != time.February {
		t.Errorf("Dec 2026 + 2mo = %s, want Feb 2027", got.Format(time.DateOnly))
	}
	if got := shiftMonths(dec15, -13); got.Year() != 2025 || got.Month() != time.November {
		t.Errorf("Dec 2026 − 13mo = %s, want Nov 2025", got.Format(time.DateOnly))
	}
	if got := shiftMonths(jan31, 0); !got.Equal(jan31) {
		t.Errorf("a zero shift changed the date: %s", got.Format(time.DateOnly))
	}
}

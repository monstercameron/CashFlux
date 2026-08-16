// SPDX-License-Identifier: MIT

package store

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
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

// ─── C350: the demo has to add up ────────────────────────────────────────────
//
// The sample is what every screenshot, every reviewer and every first run sees,
// so an internal contradiction in it is not a fixture bug — it is the product
// appearing to be wrong. Three were live: goals claiming more money than the
// account they were linked to held, payoff goals disagreeing with the loans they
// were about, and "Joint" accounts owned by one person so "Group (shared)" read
// $0.00 on the net-worth-by-member breakdown.

// TestGoalsDoNotClaimMoreThanTheirAccountHolds pins the first.
func TestGoalsDoNotClaimMoreThanTheirAccountHolds(t *testing.T) {
	ds := SampleDatasetAt(sampleAuthoredNow)
	bals, err := ledger.Balances(ds.Accounts, ds.Transactions)
	if err != nil {
		t.Fatalf("balances: %v", err)
	}
	claimed := map[string]int64{}
	for _, g := range ds.Goals {
		if g.AccountID == "" {
			continue
		}
		claimed[g.AccountID] += g.CurrentAmount.Amount
	}
	if len(claimed) == 0 {
		t.Fatal("no account-linked goals — this guard would pass vacuously")
	}
	for id, sum := range claimed {
		held := bals[id].Amount
		if sum > held {
			t.Errorf("goals linked to %s claim %d saved, but the account holds %d — "+
				"the demo shows more money earmarked than exists (C350)", id, sum, held)
		}
	}
}

// TestPayoffGoalsAgreeWithTheirLoans pins the second: "Pay off Priya's student
// loan" showed $25,000 still to go beside a ledger balance of $18,640.
func TestPayoffGoalsAgreeWithTheirLoans(t *testing.T) {
	ds := SampleDatasetAt(sampleAuthoredNow)
	bals, err := ledger.Balances(ds.Accounts, ds.Transactions)
	if err != nil {
		t.Fatalf("balances: %v", err)
	}
	abs := func(v int64) int64 {
		if v < 0 {
			return -v
		}
		return v
	}
	for _, tc := range []struct{ goalID, acctID string }{
		{"goal-studentloan", "acct-studentloan"},
		{"goal-car", "acct-carloan-marcus"},
	} {
		var g domain.Goal
		for _, cand := range ds.Goals {
			if cand.ID == tc.goalID {
				g = cand
			}
		}
		if g.ID == "" {
			t.Errorf("%s missing", tc.goalID)
			continue
		}
		remaining := g.TargetAmount.Amount - g.CurrentAmount.Amount
		owed := abs(bals[tc.acctID].Amount)
		if remaining != owed {
			t.Errorf("%s says %d still to go; %s owes %d — a payoff goal that disagrees "+
				"with its own loan makes both numbers unreadable (C350)",
				tc.goalID, remaining, tc.acctID, owed)
		}
	}
}

// TestSharedAccountsAreOwnedByTheHousehold pins the third.
func TestSharedAccountsAreOwnedByTheHousehold(t *testing.T) {
	ds := SampleDatasetAt(sampleAuthoredNow)
	shared := 0
	for _, a := range ds.Accounts {
		if a.Scope != domain.ScopeShared {
			continue
		}
		shared++
		if a.OwnerID != domain.GroupOwnerID {
			t.Errorf("%q is shared but owned by %q — net worth by member then reports "+
				"\"Group (shared) $0.00\" while a joint account sits under one person (C350)",
				a.Name, a.OwnerID)
		}
	}
	if shared == 0 {
		t.Fatal("no shared accounts — this guard would pass vacuously")
	}
}

// Every spending row carries a member, or the by-member breakdown is a single
// "(unassigned)" bar.
func TestEverySpendingRowIsAttributed(t *testing.T) {
	ds := SampleDatasetAt(sampleAuthoredNow)
	unattributed := 0
	for _, tx := range ds.Transactions {
		if tx.Amount.Amount >= 0 || tx.TransferAccountID != "" {
			continue
		}
		if tx.MemberID == "" {
			unattributed++
		}
	}
	if unattributed > 0 {
		t.Errorf("%d spending rows have no member — Spending-by-member reads "+
			"\"(unassigned)\" for all of it (C350)", unattributed)
	}
}

// ─── C351: demo content should not editorialize ──────────────────────────────
//
// "Cigarettes" as a 240-transaction habit under a category called "Guilty
// pleasures" is what a reviewer screenshots. The detectors need a small,
// frequent, cash-paid habit to find; they do not need it to be that one.
func TestDemoContentStaysNeutral(t *testing.T) {
	ds := SampleDatasetAt(sampleAuthoredNow)
	banned := []string{"Cigarette", "cigarette", "Smoke Shop", "Guilty pleasure", "Cheap ", "worthless"}
	check := func(where, text string) {
		for _, b := range banned {
			if strings.Contains(text, b) {
				t.Errorf("%s contains %q: %q — neutral demo content reads better in "+
					"screenshots, reviews and first runs (C351)", where, b, text)
			}
		}
	}
	for _, tx := range ds.Transactions {
		check("transaction desc", tx.Desc)
		check("transaction payee", tx.Payee)
		for _, tg := range tx.Tags {
			check("transaction tag", tg)
		}
	}
	for _, c := range ds.Categories {
		check("category name", c.Name)
	}
	for _, b := range ds.Budgets {
		check("budget name", b.Name)
	}

	// And the habit itself must survive the rewording — the small-leaks
	// detectors, the tag report and its budget all depend on it existing.
	habit := 0
	for _, tx := range ds.Transactions {
		if tx.CategoryID == "cat-vices" {
			habit++
		}
	}
	if habit < 200 {
		t.Errorf("only %d everyday-extras charges — the neutral rewrite must keep the "+
			"habit, not delete it", habit)
	}
}

// TestEverySampleRuleCatchesSomething pins C357.
//
// The demo shipped a rule matching the word "streaming", which appears in none
// of the charges it generates. A rule catching zero transactions is a broken
// example of the feature it exists to demonstrate — and it is the first rule a
// first-run user sees.
func TestEverySampleRuleCatchesSomething(t *testing.T) {
	ds := SampleDatasetAt(sampleAuthoredNow)
	if len(ds.Rules) == 0 {
		t.Fatal("sample has no rules — this guard would pass vacuously")
	}
	for _, r := range ds.Rules {
		if r.Match == "" {
			continue // a conditions-only rule matches by its conditions, not a phrase
		}
		needle := strings.ToLower(r.Match)
		hits := 0
		for _, tx := range ds.Transactions {
			if strings.Contains(strings.ToLower(tx.Payee), needle) ||
				strings.Contains(strings.ToLower(tx.Desc), needle) {
				hits++
			}
		}
		if hits == 0 {
			t.Errorf("sample rule %q matches %q, which nothing in the demo's ledger "+
				"contains — a rule catching nothing is a broken example of rules (C357)",
				r.ID, r.Match)
		}
	}
}

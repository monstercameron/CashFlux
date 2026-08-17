// SPDX-License-Identifier: MIT

package goalhistory

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var now = time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)

func contrib(y int, m time.Month, d int, minor int64) domain.GoalContribution {
	return domain.GoalContribution{
		Amount: money.New(minor, "USD"),
		At:     time.Date(y, m, d, 9, 0, 0, 0, time.UTC),
	}
}

// A month in which nothing landed is the finding. Dropping it would draw a chart
// of only the good months.
func TestSeriesHasNoGaps(t *testing.T) {
	g := domain.Goal{Contributions: []domain.GoalContribution{
		contrib(2026, time.June, 3, 20000),
		contrib(2026, time.August, 1, 20000),
	}}
	pts := Series(g, 20000, 4, now)
	if len(pts) != 4 {
		t.Fatalf("got %d points, want 4", len(pts))
	}
	want := []struct {
		month  string
		actual int64
	}{
		{"2026-05", 0}, {"2026-06", 20000}, {"2026-07", 0}, {"2026-08", 20000},
	}
	for i, w := range want {
		if pts[i].Month != w.month || pts[i].ActualMinor != w.actual {
			t.Errorf("point %d = %s/%d, want %s/%d", i, pts[i].Month, pts[i].ActualMinor, w.month, w.actual)
		}
		if pts[i].PlannedMinor != 20000 {
			t.Errorf("point %d planned = %d", i, pts[i].PlannedMinor)
		}
	}
	// The series must END at the current month, not start there.
	if pts[len(pts)-1].Month != MonthKey(now) {
		t.Errorf("last point = %s, want %s", pts[len(pts)-1].Month, MonthKey(now))
	}
}

func TestSeriesSumsSeveralContributionsInOneMonth(t *testing.T) {
	g := domain.Goal{Contributions: []domain.GoalContribution{
		contrib(2026, time.August, 1, 5000),
		contrib(2026, time.August, 14, 7000),
	}}
	pts := Series(g, 10000, 1, now)
	if pts[0].ActualMinor != 12000 || pts[0].Count != 2 {
		t.Errorf("point = %+v, want 12000 over 2 contributions", pts[0])
	}
	if !pts[0].Detailed {
		t.Error("a month from the itemized log should be marked detailed")
	}
}

// Roll-ups carry the history that aged out of the detailed log.
func TestSeriesReadsRolledUpMonths(t *testing.T) {
	g := domain.Goal{ContributionHistory: []domain.GoalMonthTotal{
		{Month: "2026-06", Amount: money.New(18000, "USD"), Count: 3},
	}}
	pts := Series(g, 20000, 3, now)
	if pts[0].Month != "2026-06" || pts[0].ActualMinor != 18000 || pts[0].Count != 3 {
		t.Errorf("point = %+v", pts[0])
	}
	if pts[0].Detailed {
		t.Error("a rolled-up month must not claim to be itemized")
	}
}

// The month the detailed log filled up has some entries rolled up and the rest
// still itemized. Only the SUM is that month's real funding — this is the case
// that made "detail replaces the roll-up" wrong.
func TestSeriesSumsAPartlyRolledUpMonth(t *testing.T) {
	g := domain.Goal{
		ContributionHistory: []domain.GoalMonthTotal{
			{Month: "2026-08", Amount: money.New(20000, "USD"), Count: 2},
		},
		Contributions: []domain.GoalContribution{contrib(2026, time.August, 5, 15000)},
	}
	pts := Series(g, 20000, 1, now)
	if pts[0].ActualMinor != 35000 {
		t.Errorf("actual = %d, want 35000 — the rolled-up half must not be dropped", pts[0].ActualMinor)
	}
	if pts[0].Count != 3 {
		t.Errorf("count = %d, want 3", pts[0].Count)
	}
	if !pts[0].Detailed {
		t.Error("a partly-itemized month should still offer drill-down")
	}
}

func TestMetAndShortfall(t *testing.T) {
	// An overshoot is not a negative shortfall.
	over := Point{PlannedMinor: 10000, ActualMinor: 15000}
	if !over.Met() || over.ShortfallMinor() != 0 {
		t.Errorf("over = met %v shortfall %d", over.Met(), over.ShortfallMinor())
	}
	under := Point{PlannedMinor: 10000, ActualMinor: 4000}
	if under.Met() || under.ShortfallMinor() != 6000 {
		t.Errorf("under = met %v shortfall %d", under.Met(), under.ShortfallMinor())
	}
	// No plan means nothing to fall short of.
	none := Point{ActualMinor: 0}
	if !none.Met() || none.ShortfallMinor() != 0 {
		t.Errorf("no-plan point = met %v shortfall %d", none.Met(), none.ShortfallMinor())
	}
}

// The shortfall total is the sum of monthly shortfalls, NOT planned minus
// actual: one generous month must not erase five thin ones, because catching up
// later does not put the earlier months back.
func TestSummarizeDoesNotLetAGoodMonthCancelBadOnes(t *testing.T) {
	pts := []Point{
		{PlannedMinor: 10000, ActualMinor: 0},
		{PlannedMinor: 10000, ActualMinor: 0},
		{PlannedMinor: 10000, ActualMinor: 40000},
	}
	s := Summarize(pts)
	if s.ShortfallMinor != 20000 {
		t.Errorf("ShortfallMinor = %d, want 20000", s.ShortfallMinor)
	}
	if s.ActualMinor != 40000 || s.PlannedMinor != 30000 {
		t.Errorf("totals = %d/%d", s.ActualMinor, s.PlannedMinor)
	}
	if s.FundedMonths != 1 || s.MissedMonths != 2 {
		t.Errorf("funded/missed = %d/%d", s.FundedMonths, s.MissedMonths)
	}
	if s.OnPlan() {
		t.Error("OnPlan said yes with two missed months")
	}
}

func TestSummarizeIgnoresMonthsWithNoPlan(t *testing.T) {
	s := Summarize([]Point{{ActualMinor: 0}, {ActualMinor: 500}})
	if s.FundedMonths != 0 || s.MissedMonths != 0 {
		t.Errorf("funded/missed = %d/%d, want 0/0", s.FundedMonths, s.MissedMonths)
	}
	if !s.OnPlan() {
		t.Error("a series with no plan is trivially on plan")
	}
}

func TestPeakSpansBothSeries(t *testing.T) {
	pts := []Point{{PlannedMinor: 10000, ActualMinor: 2000}, {PlannedMinor: 10000, ActualMinor: 25000}}
	if got := PeakMinor(pts); got != 25000 {
		t.Errorf("PeakMinor = %d, want 25000", got)
	}
	if got := PeakMinor(nil); got != 0 {
		t.Errorf("PeakMinor(nil) = %d", got)
	}
}

func TestSeriesWithNoPlanHasNoPlannedLine(t *testing.T) {
	pts := Series(domain.Goal{}, 0, 2, now)
	for _, p := range pts {
		if p.PlannedMinor != 0 {
			t.Errorf("%s planned = %d, want 0", p.Month, p.PlannedMinor)
		}
	}
	if Series(domain.Goal{}, 100, 0, now) != nil {
		t.Error("a zero-month window produced points")
	}
}

// ─── the roll-up itself (domain) ─────────────────────────────────────────────

// The detailed log is for undo and the monthly totals are for history;
// discarding the fifty-first contribution served the first and destroyed the
// second.
func TestRecordContributionRollsUpInsteadOfDiscarding(t *testing.T) {
	var g domain.Goal
	// 55 contributions across two months: 30 in July, 25 in August.
	for i := range 30 {
		g = g.RecordContribution(contrib(2026, time.July, 1+i%28, 1000))
	}
	for i := range 25 {
		g = g.RecordContribution(contrib(2026, time.August, 1+i%28, 1000))
	}
	if len(g.Contributions) != domain.MaxGoalContributions {
		t.Fatalf("detailed log = %d, want %d", len(g.Contributions), domain.MaxGoalContributions)
	}
	// Five aged out, all from July.
	var rolled int64
	for _, h := range g.ContributionHistory {
		rolled += h.Amount.Amount
	}
	if rolled != 5000 {
		t.Errorf("rolled up %d, want 5000", rolled)
	}

	// And the total across both sources still equals everything contributed —
	// which is the property the whole roll-up exists to preserve.
	pts := Series(g, 0, 3, now)
	var total int64
	for _, p := range pts {
		total += p.ActualMinor
	}
	if total != 55000 {
		t.Errorf("series total = %d, want 55000 — money was lost to the cap", total)
	}
}

// A contribution in a different currency cannot be summed into an existing
// month's total; a silently wrong history is worse than a short one.
func TestRollUpSkipsAMismatchedCurrency(t *testing.T) {
	var g domain.Goal
	for range domain.MaxGoalContributions {
		g = g.RecordContribution(contrib(2026, time.July, 1, 1000))
	}
	g = g.RecordContribution(contrib(2026, time.July, 2, 1000)) // rolls up one USD
	g = g.RecordContribution(domain.GoalContribution{
		Amount: money.New(1000, "EUR"),
		At:     time.Date(2026, time.July, 3, 9, 0, 0, 0, time.UTC),
	})
	for _, h := range g.ContributionHistory {
		if h.Month == "2026-07" && h.Amount.Currency != "USD" {
			t.Errorf("July total became %s", h.Amount.Currency)
		}
	}
}

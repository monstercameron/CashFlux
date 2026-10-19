// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBC5B2PaceMatchesRecurringInDescendantCategory(t *testing.T) {
	in := viewTestInput()
	in.Period.Anchor = mustDate("2026-10-01")
	in.Period.Now = mustDate("2026-10-10")
	in.Categories = []domain.Category{{ID: "rent", ParentID: "cat1"}}
	recurring := cmtRec("rent", -80000, domain.CadenceMonthly, mustDate("2026-10-05"), false)
	recurring.CategoryID = "rent"
	in.Recurring = []domain.Recurring{recurring}
	posted := cmtTxn(-80000, mustDate("2026-10-06"))
	posted.ID, posted.CategoryID = "posted-rent", "rent"
	in.Transactions = []domain.Transaction{posted}

	got := EvaluateViewBudget(cmtBudget(domain.PeriodMonthly, 100000), in)
	if !got.Valid {
		t.Fatalf("view failed: %+v", got.Error)
	}
	if got.Projection.Projected.Amount != 80000 {
		t.Fatalf("projected spend = %d, want posted rent excluded from discretionary pace; covered=%v committed=%+v pace=%+v", got.Projection.Projected.Amount, got.Covers, got.Committed, got.Projection)
	}
}

func TestBC5B2SavingPaceTreatsAheadOfTargetAsOnTrack(t *testing.T) {
	start, end, now := mustDate("2026-10-01"), mustDate("2026-11-01"), mustDate("2026-10-10")
	status := Status{
		Budget: domain.Budget{Direction: domain.DirectionSave},
		Spent:  usd(40000), Limit: usd(50000), Remaining: usd(10000),
	}
	zero := money.Zero("USD")
	pace := ProjectBudgetPace(status, zero, zero, start, end, now)
	if !pace.OnTrack || pace.OverBy.Amount != 0 {
		t.Fatalf("saving pace = %+v; being ahead of a target is not overspending", pace)
	}
	marker := ProjectPaceMarker(status, zero, start, end, now)
	if marker.Hot {
		t.Fatalf("saving pace marker is hot despite being ahead of target: %+v", marker)
	}

	in := viewTestInput()
	in.Period.Anchor, in.Period.Now = start, now
	in.Transactions = []domain.Transaction{cmtTxn(-40000, mustDate("2026-10-09"))}
	b := cmtBudget(domain.PeriodMonthly, 50000)
	b.Direction = domain.DirectionSave
	view := EvaluateViewBudget(b, in)
	if !view.Valid || view.PaceOver.Amount != 0 || !view.Projection.OnTrack {
		t.Fatalf("saving view reports overspend risk while ahead of target: %+v", view)
	}
}

func TestBC5B2EnvelopeUsesSmoothedRecurringOffset(t *testing.T) {
	b := cmtBudget(domain.PeriodMonthly, 100000)
	in := ViewInput{
		Period:       ViewPeriod{Anchor: mustDate("2026-06-15"), Now: mustDate("2026-06-15"), WeekStart: time.Monday},
		Transactions: []domain.Transaction{cmtTxn(-60000, mustDate("2026-06-10"))},
		Recurring:    []domain.Recurring{cmtRec("annual", -60000, domain.CadenceYearly, mustDate("2026-06-15"), true)},
		Rates:        currency.Rates{Base: "USD"},
		Base:         "USD",
	}
	got, err := EnvelopeAvailableForView(b, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 100000 {
		t.Fatalf("envelope available = %d, want full $1,000 after smoothing offsets the posted bill", got.Amount)
	}
}

func TestBC5B2RolloverStartAssumptionIsInformational(t *testing.T) {
	in := viewTestInput()
	b := cmtBudget(domain.PeriodMonthly, 45000)
	b.Rollover = true
	got := EvaluateViewBudget(b, in)
	if !got.Valid || len(got.Issues) != 1 {
		t.Fatalf("view = %+v; want one informational note without invalidating the calculation", got)
	}
	issue := got.Issues[0]
	if issue.Kind != ViewIssueRolloverStartAssumed || !issue.Informational || issue.Err != nil || got.Status.Error != nil {
		t.Fatalf("issue = %+v; want typed informational diagnostic and valid status", issue)
	}
}

func TestBC5B2IssueDedupKeepsFailureAlongsideInformation(t *testing.T) {
	issues := []ViewIssue{
		{BudgetID: "b1", Operation: "rollover", Kind: ViewIssueRolloverStartAssumed, Informational: true},
		{BudgetID: "b1", Operation: "pace", Kind: ViewIssueKindFailure, Err: money.ErrOverflow},
	}
	got := DeduplicateViewIssues(issues)
	if len(got) != 2 || !got[0].Informational || got[1].Err == nil {
		t.Fatalf("deduplicated issues = %+v; informational note must not mask calculation failure", got)
	}
}

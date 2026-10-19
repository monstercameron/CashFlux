// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBC4B1RollupUsesStandingPlanAndActualSpend(t *testing.T) {
	statuses := []Status{
		{
			Budget: domain.Budget{ID: "weekly", Period: domain.PeriodWeekly, Limit: usd(6000)},
			Limit:  usd(11000), Spent: usd(6000), Remaining: usd(5000), State: StateOK,
		},
		{
			Budget: domain.Budget{ID: "yearly", Period: domain.PeriodYearly, Limit: usd(120000)},
			Limit:  usd(120000), Spent: usd(120000), Remaining: usd(0), State: StateOK,
		},
	}
	rates := currency.Rates{Base: "USD"}
	expected, err := NormalizeBudgets([]domain.Budget{statuses[0].Budget, statuses[1].Budget}, domain.PeriodMonthly, rates)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := SummarizeViewStatusesForPeriod(statuses, domain.PeriodMonthly, "USD", rates)
	if err != nil {
		t.Fatal(err)
	}
	if got.LimitMinor != expected.Total.Amount {
		t.Fatalf("monthly plan = %d, want standing-plan total %d", got.LimitMinor, expected.Total.Amount)
	}
	if got.SpentMinor != 126000 {
		t.Fatalf("spent = %d, want actual 126000 minor units", got.SpentMinor)
	}
}

func TestBC4B1PooledRolloverUsesPriorHistoryAndBoost(t *testing.T) {
	in := viewTestInput()
	in.Period.Anchor = mustDate("2026-10-01")
	in.Period.Now = mustDate("2026-10-01")
	in.PoolRollover = true
	in.Transactions = []domain.Transaction{cmtTxn(-2000, mustDate("2026-09-12"))}
	b := cmtBudget(domain.PeriodMonthly, 1000)
	b.LimitHistory = map[string]int64{"2026-09-01": 3000, "2026-10-01": 1000}
	b.PeriodBoosts = map[string]int64{"2026-09-01": 500}

	got := EvaluateViewBudget(b, in)
	if !got.Valid {
		t.Fatalf("view failed: %+v", got.Error)
	}
	if got.PooledRollover != 1500 {
		t.Fatalf("pooled rollover = %d, want 1500 from prior historical cap 3500 less 2000 spent", got.PooledRollover)
	}
}

func TestBC4B1RemainingPerDayCountsPartialCalendarDay(t *testing.T) {
	in := viewTestInput()
	in.Period.Anchor = mustDate("2026-10-01")
	in.Period.Now = time.Date(2026, time.October, 30, 10, 0, 0, 0, time.UTC)
	got := EvaluateViewBudget(cmtBudget(domain.PeriodMonthly, 10000), in)
	if !got.Valid {
		t.Fatalf("view failed: %+v", got.Error)
	}
	if got.RestPerDay != 5000 {
		t.Fatalf("remaining per day = %d, want 5000 across Oct 30 and Oct 31", got.RestPerDay)
	}
}

func TestBC4B1AssumedRolloverStartUsesTypedIssueWithoutEnglishCopy(t *testing.T) {
	in := viewTestInput()
	b := cmtBudget(domain.PeriodMonthly, 45000)
	b.Rollover = true
	got := EvaluateViewBudget(b, in)
	if !got.Valid || len(got.Issues) != 1 || got.Issues[0].Kind != ViewIssueRolloverStartAssumed || got.Issues[0].Message != "" {
		t.Fatalf("assumed-start issue = %+v; want typed issue for screen translation", got.Issues)
	}
}

func TestBC4B1TopProblemsKeepsMixedCurrencyOveragesSeparate(t *testing.T) {
	statuses := []Status{
		{Budget: domain.Budget{ID: "usd"}, Remaining: money.New(-100, "USD"), State: StateOver, Percent: 120},
		{Budget: domain.Budget{ID: "eur"}, Remaining: money.New(-100000, "EUR"), State: StateOver, Percent: 130},
	}
	got := TopProblems(statuses, nil, 2)
	if len(got) != 2 || got[0].Status.Budget.ID != "eur" || got[1].Status.Budget.ID != "usd" {
		t.Fatalf("mixed currencies were ranked by raw minor units: %+v", got)
	}
}

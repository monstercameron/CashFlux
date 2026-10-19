// SPDX-License-Identifier: MIT

package budgeting

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSOLB1PartialAndNormalizedTotals(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cur      string
		excluded int
		total    int64
	}{
		{"blank splurges currency is USD", "", 0, 331353},
		{"unavailable FX excludes only splurges", "XXX", 1, 281353},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Review: $3,130.20 raw excluding Splurges, including $25/week and $600/quarter.
			budgets := []domain.Budget{
				{ID: "monthly", Period: domain.PeriodMonthly, Limit: usd(250520)},
				{ID: "entertainment", Period: domain.PeriodWeekly, Limit: usd(2500)},
				{ID: "travel", Period: domain.PeriodQuarterly, Limit: usd(60000)},
				{ID: "bud-splurges", Period: domain.PeriodYearly, Limit: money.New(600000, tc.cur)},
			}
			got, err := NormalizeBudgets(budgets, domain.PeriodMonthly, currency.Rates{Base: "USD"})
			if got.Total.Amount != tc.total || got.Excluded != tc.excluded || (err != nil) != (tc.excluded > 0) {
				t.Fatalf("normalization = %+v, %v", got, err)
			}
			in := viewTestInput()
			var statuses []Status
			for _, b := range budgets {
				v := EvaluateViewBudget(b, in)
				statuses = append(statuses, v.Status)
				if b.ID == "bud-splurges" && tc.excluded > 0 && (v.Error == nil || v.Status.Budget.ID != b.ID) {
					t.Fatalf("failed budget vanished: %+v", v)
				}
			}
			roll, issues, err := SummarizeViewStatusesForPeriod(statuses, domain.PeriodMonthly, "USD", in.Rates)
			if err != nil || roll.LimitMinor != tc.total || roll.Excluded != tc.excluded || len(issues) != tc.excluded {
				t.Fatalf("rollup=%+v issues=%v err=%v", roll, issues, err)
			}
		})
	}
}

func TestSOLB1GroceriesCarryUsesHistoricalEffectiveCaps(t *testing.T) {
	in := viewTestInput()
	in.Period.Now = mustDate("2026-10-19")
	b := cmtBudget(domain.PeriodMonthly, 45000)
	b.Rollover = true
	b.RolloverStart = mustDate("2026-08-01")
	// The review's displayed Sep cap is $264.36 and Oct cap is $205.20.
	// Those caps require the recorded Oct base $635.64 (not the latest $450).
	b.LimitHistory = map[string]int64{"2026-08-01": 45000, "2026-10-01": 63564, "2026-11-01": 45000}
	in.Transactions = []domain.Transaction{cmtTxn(-63564, mustDate("2026-08-15")), cmtTxn(-69480, mustDate("2026-09-15")), cmtTxn(-24250, mustDate("2026-10-15"))}
	// Unrelated old ledger history must never start the rollover chain.
	old := cmtTxn(-100000, mustDate("2021-01-01"))
	old.CategoryID = "other"
	in.Transactions = append(in.Transactions, old)
	for _, tc := range []struct {
		month      string
		carry, cap int64
	}{{"2026-09-01", -18564, 26436}, {"2026-10-01", -43044, 20520}, {"2026-11-01", -3730, 41270}} {
		t.Run(tc.month, func(t *testing.T) {
			in.Period.Anchor = mustDate(tc.month)
			v := EvaluateViewBudget(b, in)
			if !v.Valid || v.Carry.Amount != tc.carry || v.Status.Limit.Amount != tc.cap {
				t.Fatalf("view=%+v", v)
			}
			if len(v.Rollover.Prior) > 3 || v.Rollover.Periods > 4 {
				t.Fatalf("chain=%+v", v.Rollover)
			}
			if v.Status.Budget.Limit.Amount != 45000 {
				t.Fatal("effective cap overwrote stored annual-plan input")
			}
		})
	}
}

func TestSOLB1ProjectionCommitmentsAndFuture(t *testing.T) {
	for _, tc := range []struct {
		name, anchor, now                     string
		limit, spent, charge, projected, free int64
		state                                 PeriodStatus
	}{
		{"transport ahead of pace", "2026-10-01", "2026-10-19", 130000, 110000, 0, 189444, 20000, PeriodActive},
		{"subscriptions committed", "2026-10-01", "2026-10-19", 3999, 0, 3800, 3800, 199, PeriodActive},
		{"November planned", "2026-11-01", "2026-10-19", 3999, 0, 3800, 3800, 199, PeriodPlanned},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := viewTestInput()
			in.Period.Anchor = mustDate(tc.anchor)
			in.Period.Now = mustDate(tc.now)
			b := cmtBudget(domain.PeriodMonthly, tc.limit)
			if tc.spent > 0 {
				in.Transactions = []domain.Transaction{cmtTxn(-tc.spent, in.Period.Anchor.AddDate(0, 0, 1))}
			}
			if tc.charge > 0 {
				in.Recurring = []domain.Recurring{cmtRec("streaming", -tc.charge, domain.CadenceMonthly, in.Period.Anchor.AddDate(0, 0, 20), false)}
			}
			got := EvaluateViewBudget(b, in)
			if !got.Valid || got.Projection.Projected.Amount != tc.projected || got.FreeAfterCommitted.Amount != tc.free || got.PeriodStatus != tc.state || (tc.state == PeriodPlanned && got.Early) {
				t.Fatalf("view=%+v", got)
			}
		})
	}
}

func TestSOLB1AnnualPlanStableAcrossViewedMonth(t *testing.T) {
	in := viewTestInput()
	b := cmtBudget(domain.PeriodMonthly, 45000)
	b.Rollover = true
	b.RolloverStart = mustDate("2026-09-01")
	b.PeriodBoosts = map[string]int64{"2026-10-01": 10000}
	for _, month := range []string{"2026-10-01", "2026-11-01"} {
		in.Period.Anchor = mustDate(month)
		v := EvaluateViewBudget(b, in)
		plan, err := NormalizeBudgets([]domain.Budget{v.Status.Budget}, domain.PeriodYearly, in.Rates)
		if err != nil || plan.Total.Amount != 540000 {
			t.Fatalf("%s annual=%+v, %v", month, plan, err)
		}
	}
}

func TestSOLB1CheckedViewArithmetic(t *testing.T) {
	in := viewTestInput()
	b := cmtBudget(domain.PeriodMonthly, math.MaxInt64)
	b.PeriodBoosts = map[string]int64{"2026-06-01": 1}
	got := EvaluateViewBudget(b, in)
	if got.Valid || !errors.Is(got.Error, money.ErrOverflow) {
		t.Fatalf("overflow must be typed/incomplete: %+v", got)
	}
	start := mustDate("2026-10-01")
	pace := ProjectPace(status(math.MaxInt64, math.MaxInt64), start, start.AddDate(0, 1, 0), start.Add(time.Second))
	if !errors.Is(pace.Error, money.ErrOverflow) {
		t.Fatalf("projection overflow=%+v", pace)
	}
}

func TestSOLB1RolloverHistoryAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name    string
		start   string
		history map[string]int64
		boosts  map[string]int64
		want    int64
		prior   int
		fail    bool
	}{
		{"own historical limits", "2026-01-01", map[string]int64{"2026-01-01": 100, "2026-03-01": 200}, nil, 2000, 3, false},
		{"one period boost once", "2026-09-01", nil, map[string]int64{"2026-09-01": 50}, 350, 2, false},
		{"future start has no carry", "2026-12-01", nil, nil, 100, 0, false},
		{"invalid history is visible", "2026-09-01", map[string]int64{"bad": 200}, nil, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := viewTestInput()
			in.Period.Anchor = mustDate("2026-11-01")
			b := cmtBudget(domain.PeriodMonthly, 100)
			b.Rollover = true
			b.RolloverStart = mustDate(tc.start)
			b.LimitHistory = tc.history
			b.PeriodBoosts = tc.boosts
			v := EvaluateViewBudget(b, in)
			if tc.fail {
				if v.Valid || v.Error == nil {
					t.Fatalf("view=%+v", v)
				}
				return
			}
			if !v.Valid || v.Status.Limit.Amount != tc.want || len(v.Rollover.Prior) != tc.prior {
				t.Fatalf("view=%+v", v)
			}
			if len(v.Rollover.Prior) > 0 && v.Rollover.Prior[len(v.Rollover.Prior)-1].CarryOut.Amount != v.Carry.Amount {
				t.Fatal("explanation disagrees with actual carry")
			}
		})
	}
}

func TestSOLB1UnavailableIsExcludedAndDiagnosticsDeduplicate(t *testing.T) {
	err := &BudgetEvaluationError{BudgetID: "broken", Operation: "calculate", Err: money.ErrOverflow}
	broken := Status{Budget: domain.Budget{ID: "broken"}, Error: err, State: StateUnavailable}
	good := Status{Budget: domain.Budget{ID: "good"}, Spent: usd(1), Limit: usd(2), Remaining: usd(1)}
	got, e := SummarizeRollupChecked([]Status{broken, good})
	if e != nil || got.Excluded != 1 || got.LimitMinor != 2 {
		t.Fatalf("rollup=%+v err=%v", got, e)
	}
	issues := DeduplicateViewIssues([]ViewIssue{{BudgetID: "broken", Operation: "calculate", Err: err}, {BudgetID: "broken", Operation: "convert", Err: err}, {Operation: "income", Err: err}, {Operation: "income", Err: err}})
	if len(issues) != 2 {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestSOLB1CalendarStatus(t *testing.T) {
	for _, tc := range []struct {
		now  string
		want PeriodStatus
	}{{"2026-05-31", PeriodPlanned}, {"2026-06-01", PeriodEarly}, {"2026-06-02", PeriodEarly}, {"2026-06-15", PeriodActive}, {"2026-07-01", PeriodEnded}} {
		t.Run(tc.now, func(t *testing.T) {
			in := viewTestInput()
			in.Period.Now = mustDate(tc.now)
			v := EvaluateViewBudget(cmtBudget(domain.PeriodMonthly, 10000), in)
			if v.PeriodStatus != tc.want || v.Status.PeriodStatus != tc.want {
				t.Fatalf("status=%s row=%s", v.PeriodStatus, v.Status.PeriodStatus)
			}
		})
	}
}

func TestSOLB1MissingRolloverStartAssumesOnlyThePreviousPeriod(t *testing.T) {
	in := viewTestInput()
	b := cmtBudget(domain.PeriodMonthly, 45000)
	b.Rollover = true
	in.Transactions = []domain.Transaction{cmtTxn(-69480, mustDate("2021-09-01"))}
	got := EvaluateViewBudget(b, in)
	if !got.Valid || got.Error != nil || got.Status.Budget.ID != b.ID || got.Status.Limit.Amount != 90000 || got.Carry.Amount != 45000 || got.Rollover.Periods != 2 {
		t.Fatalf("unknown start=%+v", got)
	}
	if !got.Rollover.StartAssumed || !got.Rollover.Start.Equal(mustDate("2026-05-01")) {
		t.Fatalf("fallback start=%+v", got.Rollover)
	}
	if len(got.Issues) != 1 || got.Issues[0].Kind != ViewIssueRolloverStartAssumed || got.Issues[0].BudgetID != b.ID || got.Issues[0].Message != "" {
		t.Fatalf("assumed-start diagnostic=%+v", got.Issues)
	}
}

func TestSOLB1MissingRolloverStartUsesOnePeriodCarryFigures(t *testing.T) {
	in := viewTestInput()
	b := cmtBudget(domain.PeriodMonthly, 45000)
	b.Rollover = true
	in.Transactions = []domain.Transaction{
		cmtTxn(-30000, mustDate("2026-05-15")),
		cmtTxn(-12500, mustDate("2026-06-10")),
		cmtTxn(-69480, mustDate("2021-09-01")),
	}

	got := EvaluateViewBudget(b, in)
	if !got.Valid || got.Status.Limit.Amount != 60000 || got.Carry.Amount != 15000 || got.Status.Spent.Amount != 12500 || got.Status.Remaining.Amount != 47500 {
		t.Fatalf("one-period fallback figures: limit=%d carry=%d spent=%d remaining=%d; view=%+v", got.Status.Limit.Amount, got.Carry.Amount, got.Status.Spent.Amount, got.Status.Remaining.Amount, got)
	}
	if len(got.Rollover.Prior) != 1 || got.Rollover.Prior[0].Start.Format(time.DateOnly) != "2026-05-01" || got.Rollover.Prior[0].Spend.Amount != 30000 {
		t.Fatalf("prior fallback period=%+v", got.Rollover.Prior)
	}
}

func TestSOLB1RolloverUsesTheSameSmoothedSpendAsTheRow(t *testing.T) {
	in := viewTestInput()
	in.ShowPrior = true
	b := cmtBudget(domain.PeriodMonthly, 10000)
	b.Rollover = true
	b.RolloverStart = mustDate("2026-05-01")
	in.Recurring = []domain.Recurring{cmtRec("annual", -60000, domain.CadenceYearly, mustDate("2026-05-15"), true)}
	in.Transactions = []domain.Transaction{cmtTxn(-60000, mustDate("2026-05-15"))}
	got := EvaluateViewBudget(b, in)
	if !got.Valid || got.Carry.Amount != 10000 || got.Status.Limit.Amount != 20000 || got.Prior.Spent.Amount != 0 || got.Rollover.Prior[0].Spend.Amount != 0 {
		t.Fatalf("smoothing/carry disagreement: %+v", got)
	}
}

func TestSOLB1SmoothingUsesCheckedFXAmounts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		rate   float64
		paused bool
		want   int64
		fail   bool
	}{{"converted", 1.1, false, 66000, false}, {"missing FX", 0, false, 0, true}, {"paused", 0, true, 0, false}} {
		t.Run(tc.name, func(t *testing.T) {
			b := cmtBudget(domain.PeriodMonthly, 10000)
			r := cmtRec("annual", -60000, domain.CadenceYearly, mustDate("2026-06-15"), true)
			r.Amount.Currency = "EUR"
			r.Paused = tc.paused
			txn := cmtTxn(-60000, mustDate("2026-06-15"))
			txn.Amount.Currency = "EUR"
			rates := currency.Rates{Base: "USD"}
			if tc.rate > 0 {
				rates.Rates = map[string]float64{"EUR": tc.rate}
			}
			got, items, err := SmoothingLandingItemsWithRates(b, []domain.Recurring{r}, []domain.Transaction{txn}, mustDate("2026-06-01"), mustDate("2026-07-01"), rates)
			if (err != nil) != tc.fail || got != tc.want {
				t.Fatalf("offset=%d items=%v err=%v", got, items, err)
			}
		})
	}
	_, err := ApplySmoothingOffsetChecked(Status{Limit: usd(math.MaxInt64), Spent: usd(1), Remaining: usd(math.MaxInt64)}, 1, DefaultNearThreshold)
	if !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("smoothing overflow=%v", err)
	}
}

func TestSOLB1CommitmentsIncludeTrackedSubcategories(t *testing.T) {
	in := viewTestInput()
	in.Categories = []domain.Category{{ID: "child", ParentID: "cat1"}}
	r := cmtRec("child bill", -3800, domain.CadenceMonthly, mustDate("2026-06-21"), false)
	r.CategoryID = "child"
	in.Recurring = []domain.Recurring{r}
	got := EvaluateViewBudget(cmtBudget(domain.PeriodMonthly, 3999), in)
	if got.CommitmentError != nil || got.FreeAfterCommitted.Amount != 199 || got.Status.FreeAfterCommitted.Amount != 199 {
		t.Fatalf("child commitment=%+v", got)
	}
}

func TestSOLB1NormalizedRemainderReconciles(t *testing.T) {
	// Normalize the standing weekly plan, retaining actual posted window spend.
	// The effective cap differs to prove it cannot inflate the standing plan.
	st := Status{Budget: domain.Budget{ID: "weekly", Period: domain.PeriodWeekly, Limit: usd(2)}, Spent: usd(1), Limit: usd(3), Remaining: usd(2), State: StateOver}
	got, issues, err := SummarizeViewStatusesForPeriod([]Status{st}, domain.PeriodMonthly, "USD", currency.Rates{Base: "USD"})
	if err != nil || len(issues) != 0 || got.LimitMinor != 9 || got.SpentMinor != 1 || got.LimitMinor-got.SpentMinor != 8 {
		t.Fatalf("rollup=%+v issues=%v err=%v", got, issues, err)
	}
}

func TestSOLB1RollupPreservesFXBase(t *testing.T) {
	rates := currency.Rates{Base: "EUR", Rates: map[string]float64{"USD": 0.5}}
	st := Status{Budget: domain.Budget{ID: "eur", Period: domain.PeriodMonthly, Limit: money.New(100, "EUR")}, Limit: money.New(150, "EUR"), Spent: money.Zero("EUR"), Remaining: money.New(150, "EUR")}
	got, issues, err := SummarizeViewStatusesForPeriod([]Status{st}, domain.PeriodMonthly, "USD", rates)
	if err != nil || len(issues) != 0 || got.LimitMinor != 200 {
		t.Fatalf("rollup=%+v issues=%v err=%v", got, issues, err)
	}
}

func TestSOLB1CommitmentsRemainHonestBeyondAvailableFunds(t *testing.T) {
	for _, tc := range []struct {
		name                string
		limit, charge, free int64
		paused              bool
	}{{"charges exceed room", 100, 3800, -3700, false}, {"already over", -100, 3800, -3900, false}, {"income is not a charge", 3999, -3800, 3999, false}, {"paused charge", 3999, 3800, 3999, true}} {
		t.Run(tc.name, func(t *testing.T) {
			in := viewTestInput()
			in.Period.Anchor = mustDate("2026-10-01")
			in.Period.Now = mustDate("2026-10-19")
			r := cmtRec("streaming", -tc.charge, domain.CadenceMonthly, mustDate("2026-10-21"), false)
			r.Paused = tc.paused
			in.Recurring = []domain.Recurring{r}
			got := EvaluateViewBudget(cmtBudget(domain.PeriodMonthly, tc.limit), in)
			if got.CommitmentError != nil || got.FreeAfterCommitted.Amount != tc.free {
				t.Fatalf("view=%+v", got)
			}
		})
	}
	in := viewTestInput()
	r := cmtRec("fx", -3800, domain.CadenceMonthly, mustDate("2026-06-21"), false)
	r.Amount.Currency = "EUR"
	in.Recurring = []domain.Recurring{r}
	got := EvaluateViewBudget(cmtBudget(domain.PeriodMonthly, 3999), in)
	if got.CommitmentError == nil || got.Status.CommitmentError == nil || got.FreeAfterCommitted.Currency != "" {
		t.Fatalf("unknown free money must be explicit: %+v", got)
	}
}

// SPDX-License-Identifier: MIT

package budgeting

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestResolveViewPeriod(t *testing.T) {
	start, end := mustDate("2026-06-01"), mustDate("2026-07-01")
	for _, tc := range []struct{ name, now, want string }{
		{"live", "2026-06-15", "2026-06-15"}, {"inclusive start", "2026-06-01", "2026-06-01"},
		{"exclusive end", "2026-07-01", "2026-06-01"}, {"future", "2026-05-31", "2026-06-01"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pay := mustDate("2026-05-29")
			got := ResolveViewPeriod(start, end, mustDate(tc.now), time.Monday, pay)
			if !got.Anchor.Equal(mustDate(tc.want)) || got.WeekStart != time.Monday || !got.PayCycleAnchor.Equal(pay) {
				t.Fatalf("period = %+v", got)
			}
		})
	}
}

func viewTestInput() ViewInput {
	return ViewInput{Period: ViewPeriod{Anchor: mustDate("2026-06-15"), Now: mustDate("2026-06-15"), WeekStart: time.Monday}, Base: "USD", Rates: currency.Rates{Base: "USD"}}
}

func TestEvaluateViewBudget(t *testing.T) {
	for _, tc := range []struct {
		name                                                   string
		rollover, pool                                         bool
		boost, previous, spent, wantLimit, wantCarry, wantPool int64
	}{
		{"plain", false, false, 0, 3000, 2000, 10000, 0, 0},
		{"carry and boost", true, false, 500, 3000, 2000, 17500, 7000, 0},
		{"pooled leftover", false, true, 0, 3000, 2000, 10000, 0, 7000},
		{"carry excludes pool", true, true, 0, 3000, 2000, 17000, 7000, 0},
		{"deficit and release", true, true, -500, 12000, 2000, 7500, -2000, 0},
		{"pool excludes deficit", false, true, 0, 12000, 2000, 10000, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := viewTestInput()
			in.PoolRollover = tc.pool
			in.ShowPrior = true
			b := cmtBudget(domain.PeriodMonthly, 10000)
			b.Rollover = tc.rollover
			b.RolloverStart = mustDate("2026-05-01")
			b.CoveredAt = mustDate("2026-06-01")
			b.PeriodBoosts = map[string]int64{PeriodKeyWithWeekStart(b.Period, in.Period.Anchor, time.Time{}, time.Monday): tc.boost}
			in.Transactions = []domain.Transaction{cmtTxn(-tc.previous, mustDate("2026-05-15")), cmtTxn(-tc.spent, mustDate("2026-06-10")), cmtTxn(-90000, mustDate("2026-07-01"))}
			before := b.Limit
			got := EvaluateViewBudget(b, in)
			if !got.Valid || len(got.Issues) != 0 || got.Status.Limit.Amount != tc.wantLimit || got.Status.Spent.Amount != tc.spent || got.CarryDelta != tc.wantCarry || got.PooledRollover != tc.wantPool {
				t.Fatalf("view = %+v", got)
			}
			if !got.Covered || !got.HasPrior || got.Prior.Spent.Amount != tc.previous || got.PriorLimitBase != 10000 || got.PriorSpentBase != tc.previous {
				t.Fatalf("prior/coverage = %+v", got)
			}
			if b.Limit != before || got.Status.Budget.Limit != before || got.Status.Limit.Amount != tc.wantLimit {
				t.Fatal("stored/effective cap distinction lost")
			}
			if got.ProratedRest <= 0 || got.ProratedRest >= got.Status.Remaining.Amount {
				t.Fatalf("prorated rest = %d", got.ProratedRest)
			}
		})
	}
}

func TestEvaluateViewBudgetCadenceAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		change    func(*domain.Budget, *ViewInput)
		valid     bool
		operation string
	}{
		{"category descendants", func(b *domain.Budget, in *ViewInput) {
			in.Categories = []domain.Category{{ID: "child", ParentID: "cat1"}}
			in.Transactions = []domain.Transaction{{ID: "child-txn", CategoryID: "child", Date: mustDate("2026-06-10"), Amount: usd(-1000)}}
		}, true, ""},
		{"missing spending rate", func(b *domain.Budget, in *ViewInput) {
			in.Transactions = []domain.Transaction{{ID: "fx", CategoryID: "cat1", Date: mustDate("2026-06-10"), Amount: money.New(-1000, "EUR")}}
		}, false, "evaluate budget"},
		{"missing commitment rate", func(b *domain.Budget, in *ViewInput) {
			r := cmtRec("fx", -1000, domain.CadenceMonthly, mustDate("2026-06-20"), false)
			r.Amount.Currency = "EUR"
			in.Recurring = []domain.Recurring{r}
		}, true, "calculate committed budget"},
		{"unknown prior carry makes only this budget unavailable", func(b *domain.Budget, in *ViewInput) {
			b.Rollover = true
			b.RolloverStart = mustDate("2026-05-01")
			in.Transactions = []domain.Transaction{{ID: "fx", CategoryID: "cat1", Date: mustDate("2026-05-10"), Amount: money.New(-1000, "EUR")}}
		}, false, "evaluate prior rollover period"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := viewTestInput()
			b := cmtBudget(domain.PeriodMonthly, 10000)
			tc.change(&b, &in)
			got := EvaluateViewBudget(b, in)
			if got.Valid != tc.valid {
				t.Fatalf("valid = %v", got.Valid)
			}
			if tc.operation != "" {
				if len(got.Issues) == 0 || got.Issues[0].Operation != tc.operation || got.Issues[0].Err == nil {
					t.Fatalf("issues = %+v", got.Issues)
				}
			} else if got.Status.Spent.Amount != 1000 {
				t.Fatalf("descendant spend = %+v", got.Status)
			}
		})
	}
	in := viewTestInput()
	in.Period.Anchor = mustDate("2026-06-03")
	in.Period.Now = in.Period.Anchor
	b := cmtBudget(domain.PeriodBiweekly, 10000)
	in.Period.PayCycleAnchor = mustDate("2026-05-29")
	got := EvaluateViewBudget(b, in)
	if !got.Start.Equal(mustDate("2026-05-29")) || !got.End.Equal(mustDate("2026-06-12")) {
		t.Fatalf("pay cadence = %+v", got)
	}
	in = viewTestInput()
	in.Period.Now = mustDate("2026-06-02")
	in.Period.Anchor = in.Period.Now
	if got := EvaluateViewBudget(cmtBudget(domain.PeriodMonthly, 10000), in); !got.Early || !got.HasPrior {
		t.Fatalf("early period = %+v", got)
	}
}

func TestEvaluateViewBudgetSmoothingAndCarryCap(t *testing.T) {
	in := viewTestInput()
	b := cmtBudget(domain.PeriodMonthly, 10000)
	b.Rollover = true
	b.RolloverStart = mustDate("2026-05-01")
	b.RolloverCapPeriods = 1
	in.Transactions = []domain.Transaction{cmtTxn(-60000, mustDate("2026-06-15"))}
	in.Recurring = []domain.Recurring{cmtRec("annual", -60000, domain.CadenceYearly, mustDate("2026-06-15"), true)}
	got := EvaluateViewBudget(b, in)
	if got.Status.Limit.Amount != 20000 || got.Status.Spent.Amount != 0 || len(got.LandingItems) != 1 || len(got.Issues) != 0 {
		t.Fatalf("smoothed view = %+v", got)
	}
}

func TestEvaluateViewBudgetProjectionTreatsRecurringChargesAsFixed(t *testing.T) {
	in := viewTestInput()
	in.Period.Anchor = mustDate("2026-10-01")
	in.Period.Now = mustDate("2026-10-19")
	b := cmtBudget(domain.PeriodMonthly, 120000)
	in.Recurring = []domain.Recurring{
		cmtRec("car-a", -62000, domain.CadenceMonthly, mustDate("2026-10-05"), false),
		cmtRec("car-b", -48000, domain.CadenceMonthly, mustDate("2026-10-20"), false),
	}
	in.Transactions = []domain.Transaction{cmtTxn(-62000, mustDate("2026-10-06"))}
	got := EvaluateViewBudget(b, in)
	if !got.Valid || got.Status.Spent.Amount != 62000 || got.Committed.Required.Amount != 48000 {
		t.Fatalf("car payment inputs = %+v", got)
	}
	if got.Projection.Projected.Amount != 110000 || !got.Projection.OnTrack || got.PaceOver.IsPositive() {
		t.Fatalf("projection=%+v pace over=%v; want fixed $1,100 car costs within $1,200", got.Projection, got.PaceOver)
	}
	if !got.Projection.CommittedDominates || got.Projection.Committed.Amount != 110000 {
		t.Fatalf("committed explanation=%+v; want recurring charges to dominate projection", got.Projection)
	}
}

func TestEvaluateViewBudgetClosedPeriodExposesEndedOverByWithoutPace(t *testing.T) {
	in := viewTestInput()
	in.Period.Anchor = mustDate("2026-09-01")
	in.Period.Now = mustDate("2026-10-19")
	in.Transactions = []domain.Transaction{cmtTxn(-10500, mustDate("2026-09-10"))}
	got := EvaluateViewBudget(cmtBudget(domain.PeriodMonthly, 10000), in)
	if !got.Valid || got.PeriodStatus != PeriodEnded || !got.Projection.PeriodEnded || got.Projection.Projected.Amount != 0 || got.Projection.OverBy.Amount != 0 || got.Projection.EndedOverBy.Amount != 500 {
		t.Fatalf("closed-period projection=%+v status=%s; want ended over by $5 without pace", got.Projection, got.PeriodStatus)
	}
}

func TestEvaluateViewBudgetRestPerDayIsPerBudget(t *testing.T) {
	in := viewTestInput()
	in.Period.Now = mustDate("2026-06-15")
	in.Transactions = []domain.Transaction{cmtTxn(-1000, mustDate("2026-06-10"))}
	a := cmtBudget(domain.PeriodMonthly, 10000)
	b := cmtBudget(domain.PeriodMonthly, 20000)
	first, second := EvaluateViewBudget(a, in), EvaluateViewBudget(b, in)
	if first.RestPerDay == 0 || second.RestPerDay == 0 || first.RestPerDay == second.RestPerDay {
		t.Fatalf("rest-per-day values first=%d second=%d; expected independent budget values", first.RestPerDay, second.RestPerDay)
	}
}

func TestViewSelectionAndAttention(t *testing.T) {
	all := []domain.Budget{{ID: "shared", OwnerID: domain.GroupOwnerID}, {ID: "a", OwnerID: "a"}, {ID: "b", OwnerID: "b"}}
	for _, tc := range []struct {
		member string
		ids    []string
	}{{"", []string{"shared", "a", "b"}}, {"a", []string{"shared", "a"}}, {"missing", []string{"shared"}}} {
		got := ViewBudgets(all, tc.member)
		var ids []string
		for _, b := range got {
			ids = append(ids, b.ID)
		}
		if !reflect.DeepEqual(ids, tc.ids) {
			t.Fatalf("member %q: %v", tc.member, ids)
		}
	}
	statuses := []Status{{Budget: domain.Budget{ID: "ok"}, State: StateOK, Percent: 90}, {Budget: domain.Budget{ID: "risk"}, State: StateOK, Percent: 80}, {Budget: domain.Budget{ID: "near"}, State: StateNear, Percent: 80}, {Budget: domain.Budget{ID: "over"}, State: StateOver, Percent: 110}}
	SortViewStatuses(statuses, map[string]bool{"risk": true})
	var ids []string
	for _, s := range statuses {
		ids = append(ids, s.Budget.ID)
	}
	if !reflect.DeepEqual(ids, []string{"over", "risk", "near", "ok"}) {
		t.Fatalf("order = %v", ids)
	}
	selected := ViewAttention(statuses)
	if len(selected) != 2 || selected[0].Budget.ID != "over" || selected[1].Budget.ID != "near" {
		t.Fatalf("attention = %+v", selected)
	}
}

func TestSummarizeViewStatuses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		rates  currency.Rates
		amount int64
		want   int64
		issues int
		fail   bool
	}{
		{"converted", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}, 10000, 11000, 0, false},
		{"missing rate", currency.Rates{Base: "USD"}, 10000, 0, 1, false},
		{"overflow", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1}}, math.MaxInt64, 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statuses := []Status{{Budget: domain.Budget{ID: "eur"}, Spent: money.New(tc.amount, "EUR"), Limit: money.New(tc.amount, "EUR"), Remaining: money.Zero("EUR")}}
			if tc.fail {
				statuses[0].Spent.Currency = "USD"
				statuses[0].Limit.Currency = "USD"
				statuses[0].Remaining.Currency = "USD"
				statuses = append(statuses, Status{Budget: domain.Budget{ID: "usd"}, Spent: usd(1), Limit: usd(1), Remaining: usd(0)})
			}
			got, issues, err := SummarizeViewStatuses(statuses, "USD", tc.rates)
			if (err != nil) != tc.fail || len(issues) != tc.issues || got.SpentMinor != tc.want {
				t.Fatalf("summary = %+v; issues = %+v; err = %v", got, issues, err)
			}
			if !tc.fail && statuses[0].Spent.Currency != "EUR" {
				t.Fatal("input status modified")
			}
		})
	}
}

func TestResolveViewIncome(t *testing.T) {
	in := viewTestInput()
	txns := []domain.Transaction{{ID: "apr", Date: mustDate("2026-04-15"), Amount: usd(10000)}, {ID: "may", Date: mustDate("2026-05-15"), Amount: usd(20000)}, {ID: "june", Date: mustDate("2026-06-01"), Amount: usd(90000)}}
	for _, tc := range []struct {
		name  string
		basis ViewIncomeBasis
		want  int64
	}{
		{"legacy actual", ViewIncomeBasis{}, 20000}, {"legacy configured", ViewIncomeBasis{Fixed: 30000}, 30000},
		{"averaged", ViewIncomeBasis{Mode: IncomeModeAll, AverageMonths: 2}, 15000}, {"fixed not averaged", ViewIncomeBasis{Mode: IncomeModeFixed, Fixed: 40000, AverageMonths: 6}, 40000},
		{"paychecks", ViewIncomeBasis{Mode: IncomeModePaychecks, PaycheckMin: 30000}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveViewIncome(tc.basis, txns, nil, in.Period, in.Base, in.Rates)
			if err != nil || got != tc.want {
				t.Fatalf("income = %d, %v", got, err)
			}
		})
	}
	txns[1].Amount.Currency = "EUR"
	if _, err := ResolveViewIncome(ViewIncomeBasis{Mode: IncomeModeAll}, txns, nil, in.Period, in.Base, in.Rates); err == nil {
		t.Fatal("missing income rate must be reported")
	}
}

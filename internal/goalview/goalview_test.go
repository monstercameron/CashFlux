// SPDX-License-Identifier: MIT

package goalview

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

func goal(id, currency string, saved, target int64) domain.Goal {
	return domain.Goal{ID: id, Name: id, CurrentAmount: money.New(saved, currency), TargetAmount: money.New(target, currency)}
}

func TestBuildFXSummaryUsesSameSubsetAndUnits(t *testing.T) {
	usd := goal("usd", "USD", 100, 100)
	eur := goal("eur", "EUR", 0, 100)
	unknown := goal("unknown", "GBP", 10000, 10000)
	eur.Allocations = []domain.GoalAllocation{{AccountID: "bank", Amount: money.New(20, "EUR")}}
	v := Build(Input{Goals: []domain.Goal{usd, eur, unknown}, Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}})
	if v.SavedTotal.Amount != 100 || v.TargetTotal.Amount != 300 || v.SetAsideMin.Amount != 40 || v.OverallPct != 33 {
		t.Fatalf("summary saved=%v target=%v aside=%v percent=%d", v.SavedTotal, v.TargetTotal, v.SetAsideMin, v.OverallPct)
	}
	if v.FundedTotal.Amount != 140 || v.LeftTotal.Amount != 200 {
		t.Fatalf("funded/left = %v/%v", v.FundedTotal, v.LeftTotal)
	}
	if !reflect.DeepEqual(v.ExcludedCurrencies, []string{"GBP"}) {
		t.Fatal(v.ExcludedCurrencies)
	}
}

func TestBuildSectionsScopeAndDayTransition(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	active := goal("active", "USD", 1, 100)
	active.OwnerID = "m"
	active.TargetDate = now.AddDate(0, 0, 1)
	missed := goal("missed", "USD", 2, 100)
	missed.OwnerID = domain.GroupOwnerID
	missed.TargetDate = now.AddDate(0, 0, -1)
	archived := goal("archived", "USD", 50, 100)
	archived.Archived = true
	archived.OwnerID = "m"
	fund := goal("fund", "USD", 50, 100)
	fund.IsSinkingFund = true
	fund.OwnerID = "m"
	other := goal("other", "USD", 10000, 10000)
	other.OwnerID = "other"
	checklist := goal("check", "USD", 10000, 10000)
	checklist.Kind = domain.GoalKindChecklist
	checklist.OwnerID = "m"
	in := Input{MemberID: "m", Goals: []domain.Goal{active, missed, archived, fund, other, checklist}, Rates: currency.Rates{Base: "USD"}, Now: now}
	v := Build(in)
	if len(v.All) != 5 || len(v.Active) != 2 || len(v.Missed) != 1 || len(v.Fund) != 1 || len(v.Achieved) != 1 {
		t.Fatalf("wrong sections: %+v", v)
	}
	if v.SavedTotal.Amount != 3 || v.TargetTotal.Amount != 200 {
		t.Fatalf("wrong in-flight financial summary: %v/%v", v.SavedTotal, v.TargetTotal)
	}
	in.Now = now.AddDate(0, 0, 2)
	if got := Build(in); len(got.Missed) != 2 {
		t.Fatalf("deadline did not advance: %+v", got.Missed)
	}
}

func TestRatioPercentDoesNotOverflow(t *testing.T) {
	for _, tt := range []struct {
		n, d int64
		want int
	}{{1, 3, 33}, {math.MaxInt64 - 1, math.MaxInt64, 99}, {math.MaxInt64 / 2, math.MaxInt64, 49}, {99, 100, 99}} {
		if got := ratioPercent(tt.n, tt.d); got != tt.want {
			t.Errorf("%d/%d = %d, want %d", tt.n, tt.d, got, tt.want)
		}
	}
}

func TestOverbookedComparesNativeAmountsWithoutFX(t *testing.T) {
	a := domain.Account{ID: "bank", Currency: "EUR", OpeningBalance: money.New(100, "EUR")}
	g := goal("goal", "EUR", 0, 500)
	g.Allocations = []domain.GoalAllocation{{AccountID: "bank", Amount: money.New(101, "EUR")}}
	if !Overbooked([]domain.Goal{g}, []domain.Account{a}, nil, currency.Rates{Base: "USD"})[g.ID] {
		t.Fatal("native reservation should not need EUR/USD FX")
	}
	g.Allocations[0].Amount = money.New(101, "GBP")
	if Overbooked([]domain.Goal{g}, []domain.Account{a}, nil, currency.Rates{Base: "USD"})[g.ID] {
		t.Fatal("unknown currency treated as parity")
	}
}

func TestHealthFairShareIgnoresUnknownCurrencyAndNonfinancialGoals(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	first := goal("first", "USD", 0, 100)
	first.TargetDate = now.AddDate(0, 1, 0)
	risky := goal("risky", "USD", 0, 500)
	risky.TargetDate = first.TargetDate
	unknown := goal("unknown", "GBP", 0, 100)
	unknown.TargetDate = first.TargetDate
	check := first
	check.ID = "check"
	check.Kind = domain.GoalKindChecklist
	archived := first
	archived.ID = "archived"
	archived.Archived = true
	paused := first
	paused.ID = "paused"
	paused.PausedUntil = now.AddDate(0, 2, 0)
	var txns []domain.Transaction
	for month := 1; month <= 3; month++ {
		txns = append(txns, domain.Transaction{Date: now.AddDate(0, -month, 0), Amount: money.New(300, "USD")})
	}
	got := Health([]domain.Goal{first, risky, unknown, check, archived, paused}, txns, currency.Rates{Base: "USD"}, "USD", now)
	if len(got) != 2 || got["first"].Health != goals.HealthOnTrack || got["risky"].Health != goals.HealthAtRisk || got["first"].FairMinor != 150 || got["first"].SurplusMinor != 300 {
		t.Fatalf("health = %+v", got)
	}
	if got := Health([]domain.Goal{first}, nil, currency.Rates{Base: "USD"}, "USD", now); len(got) != 0 {
		t.Fatalf("invented a pace verdict without cashflow: %+v", got)
	}
}

func TestBuildHandlesLegacyZeroSavedAndCombinedOverflow(t *testing.T) {
	legacy := goal("legacy", "USD", 0, 100)
	legacy.CurrentAmount = money.Money{}
	v := Build(Input{Goals: []domain.Goal{legacy}, Rates: currency.Rates{Base: "USD"}})
	if v.TargetTotal.Amount != 100 || len(v.ExcludedCurrencies) != 0 {
		t.Fatalf("legacy zero excluded: %+v", v)
	}
	huge := goal("huge", "USD", math.MaxInt64, math.MaxInt64)
	huge.Allocations = []domain.GoalAllocation{{AccountID: "a", Amount: money.New(1, "USD")}}
	v = Build(Input{Goals: []domain.Goal{huge}, Rates: currency.Rates{Base: "USD"}})
	if v.FundedTotal.Amount < 0 || len(v.ExcludedCurrencies) != 1 {
		t.Fatalf("overflow silently misrepresented: %+v", v)
	}
}

func TestSummaryChecksReservationSumBeforeConversion(t *testing.T) {
	for _, tt := range []struct {
		name        string
		allocations []domain.GoalAllocation
		want        int64
		excluded    bool
	}{
		{"round once", []domain.GoalAllocation{{Amount: money.New(1, "EUR")}, {Amount: money.New(1, "EUR")}}, 1, false},
		{"overflow", []domain.GoalAllocation{{Amount: money.New(math.MaxInt64, "EUR")}, {Amount: money.New(1, "EUR")}}, 0, true},
		{"wrong currency", []domain.GoalAllocation{{Amount: money.New(1, "GBP")}}, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := goal("g", "EUR", 0, 100)
			g.Allocations = tt.allocations
			v := Build(Input{Goals: []domain.Goal{g}, Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 0.5}}})
			if v.SetAsideMin.Amount != tt.want || (len(v.ExcludedCurrencies) > 0) != tt.excluded {
				t.Fatalf("reservation summary = %+v", v)
			}
		})
	}
}

func TestOverbookedOverflowStillFlagsEveryReservingGoal(t *testing.T) {
	a := domain.Account{ID: "bank", Currency: "USD", OpeningBalance: money.New(math.MaxInt64, "USD")}
	first, second := goal("first", "USD", 0, 100), goal("second", "USD", 0, 100)
	first.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(math.MaxInt64, "USD")}}
	second.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1, "USD")}}
	got := Overbooked([]domain.Goal{first, second}, []domain.Account{a}, nil, currency.Rates{Base: "USD"})
	if !got[first.ID] || !got[second.ID] {
		t.Fatalf("overflowed reservations were hidden: %+v", got)
	}
}

func TestTimeKeyTracksExactDeadlineAndPauseTransitions(t *testing.T) {
	day := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	deadline, resume := day.Add(10*time.Hour), day.Add(15*time.Hour)
	g := goal("g", "USD", 0, 100)
	g.TargetDate, g.PausedUntil = deadline, resume
	all := []domain.Goal{g}
	before := TimeKey(all, deadline.Add(-time.Nanosecond))
	atDeadline := TimeKey(all, deadline)
	if before == atDeadline || atDeadline != TimeKey(all, resume.Add(-time.Nanosecond)) {
		t.Fatal("deadline key did not change at its exact transition")
	}
	if atDeadline == TimeKey(all, resume) || TimeKey(all, resume) == TimeKey(all, day.AddDate(0, 0, 1)) {
		t.Fatal("pause or calendar-day transition did not invalidate")
	}
	beforeView := Build(Input{Goals: all, Now: resume.Add(-time.Nanosecond)})
	afterView := Build(Input{Goals: all, Now: resume})
	if len(beforeView.Active) != 1 || len(afterView.Missed) != 1 {
		t.Fatal("pause transition did not move the missed goal")
	}
}

func TestSortMatchesProgressAndPreservesInput(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	gs := []domain.Goal{goal("financial", "USD", 75, 100), {ID: "check", Name: "check", Kind: domain.GoalKindChecklist}, {ID: "habit", Name: "habit", Kind: domain.GoalKindHabit, HabitTarget: 2, CheckIns: []time.Time{now, now.AddDate(0, 0, -8)}}}
	tasks := []domain.Task{{RelatedType: domain.RelatedGoal, RelatedID: "check", Status: domain.StatusDone}, {RelatedType: domain.RelatedGoal, RelatedID: "check"}}
	for _, key := range []string{"closest", "farthest", "complexity", "deadline", "name", "priority", "actionable"} {
		t.Run(key, func(t *testing.T) {
			got := append([]domain.Goal(nil), gs...)
			want := append([]domain.Goal(nil), gs...)
			if key == "closest" || key == "farthest" || key == "complexity" {
				sort.SliceStable(want, func(i, j int) bool {
					a, z := goals.EvaluateProgress(want[i], tasks, now).Percent, goals.EvaluateProgress(want[j], tasks, now).Percent
					if key == "complexity" {
						_, a = goals.TaskCounts(tasks, want[i].ID)
						_, z = goals.TaskCounts(tasks, want[j].ID)
					}
					if a != z {
						if key == "farthest" {
							return a < z
						}
						return a > z
					}
					return goals.LessForList(want[i], want[j])
				})
				Sort(got, key, tasks, now)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("got %v, want %v", got, want)
				}
			} else {
				Sort(got, key, tasks, now)
			}
		})
	}
	if gs[0].ID != "financial" || !gs[2].CheckIns[0].Equal(now) {
		t.Fatal("mutated caller data")
	}
}

func TestExplicitSortSpansFundingAndLifecycleSections(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	missed, healthy, fund, archived := goal("Zed", "USD", 0, 100), goal("Baby", "USD", 0, 100), goal("Alpha fund", "USD", 0, 100), goal("Archived", "USD", 0, 100)
	missed.TargetDate, fund.IsSinkingFund, archived.Archived = now.Add(-time.Hour), true, true
	v := Build(Input{Goals: []domain.Goal{missed, healthy, fund, archived}, Rates: currency.Rates{Base: "USD"}, Now: now})
	got := OrderedOpen(v, "name", now)
	if len(got) != 3 || got[0].ID != fund.ID || got[1].ID != healthy.ID || got[2].ID != missed.ID {
		t.Fatalf("explicit sort still pinned sections first: %+v", got)
	}
	if len(v.Missed) != 1 || len(v.Fund) != 1 || len(v.Active) != 1 || v.All[0].ID != missed.ID {
		t.Fatal("explicit sort mutated the shared default sections")
	}
}

func BenchmarkSortClosest(b *testing.B) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	gs := make([]domain.Goal, 200)
	tasks := make([]domain.Task, 20000)
	for i := range gs {
		gs[i] = domain.Goal{ID: fmt.Sprint(i), Name: fmt.Sprint(i), Kind: domain.GoalKindChecklist}
	}
	for i := range tasks {
		tasks[i] = domain.Task{RelatedType: domain.RelatedGoal, RelatedID: gs[i%len(gs)].ID}
		if i%3 == 0 {
			tasks[i].Status = domain.StatusDone
		}
	}
	b.Run("scan-comparator", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			out := append([]domain.Goal(nil), gs...)
			sort.SliceStable(out, func(i, j int) bool {
				a, z := goals.EvaluateProgress(out[i], tasks, now).Percent, goals.EvaluateProgress(out[j], tasks, now).Percent
				if a != z {
					return a > z
				}
				return goals.LessForList(out[i], out[j])
			})
		}
	})
	b.Run("indexed", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			out := append([]domain.Goal(nil), gs...)
			Sort(out, "closest", tasks, now)
		}
	})
}

// SPDX-License-Identifier: MIT

package assistantdata

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

func planningFixture() PlanningInput {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	return PlanningInput{Now: now, Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, Accounts: []domain.Account{{ID: "a", Name: "Bank", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD")}}, Goals: []domain.Goal{{ID: "g", Name: "Trip", TargetAmount: money.New(10000, "EUR"), CurrentAmount: money.New(2000, "USD"), TargetDate: now.AddDate(0, 4, 0), Allocations: []domain.GoalAllocation{{AccountID: "a", Amount: money.New(1000, "EUR")}}}}}
}

func TestPlanningGoalFactsMatchCanonicalAndRetainUnknown(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rates map[string]float64
		saved int64
	}{
		{"converted", map[string]float64{"EUR": 2}, 2000}, {"missing FX", nil, 2000}, {"invalid FX", map[string]float64{"EUR": math.Inf(1)}, 2000}, {"overflow", map[string]float64{"EUR": 0.5}, math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := planningFixture()
			in.Rates.Rates = tc.rates
			in.Goals[0].CurrentAmount = money.New(tc.saved, "USD")
			p := NewPlanning(in)
			facts, err := p.Goals("g", false)
			if err != nil {
				t.Fatal(err)
			}
			f := facts[0]
			e := goals.EvaluateGoal(in.Goals[0], goals.TaskSummary{}, in.Rates, in.Now)
			if f.Known != (e.Err == nil) || f.Complete != e.Complete || f.State != e.State || f.Progress != e.Progress {
				t.Fatalf("canonical mismatch: %+v vs %+v", f, e)
			}
			if e.Err != nil {
				if f.Covered.Known || f.Covered.Amount != nil || f.Remaining.Known || f.ProjectedFinish != nil || f.CoveragePercent != nil || f.Error == "" {
					t.Fatalf("unknown reported as money: %+v", f)
				}
				return
			}
			if f.Covered.Amount.Minor != "2000" || f.Covered.Amount.Currency != "EUR" || f.Remaining.Amount.Minor != "8000" || !f.MonthlyNeeded.Known || f.MonthlyNeeded.Amount.Minor != "2000" {
				t.Fatalf("incorrect converted facts: %+v", f)
			}
		})
	}
}

func TestPlanningTasksArchivesAndPlanErrors(t *testing.T) {
	in := planningFixture()
	in.Goals = append(in.Goals, domain.Goal{ID: "check", Kind: domain.GoalKindChecklist, Archived: true})
	in.Tasks = []domain.Task{{ID: "done", RelatedType: domain.RelatedGoal, RelatedID: "check", Status: domain.StatusDone}, {ID: "open", RelatedType: domain.RelatedGoal, RelatedID: "check", Status: domain.StatusOpen}}
	in.Goals[0].MonthlyContribution = money.New(10, "JPY")
	p := NewPlanning(in)
	active, _ := p.Goals("", false)
	all, _ := p.Goals("", true)
	archived, _ := p.Goals("check", false)
	if len(active) != 1 || len(all) != 2 || len(archived) != 1 || archived[0].TasksDone != 1 || archived[0].TasksTotal != 2 || archived[0].Progress.Percent != 50 || archived[0].Target != nil {
		t.Fatalf("kind/archive facts: %+v / %+v", all, archived)
	}
	if !active[0].Known || active[0].PlanError == "" || active[0].Assignment.Known || active[0].ProjectedFinish != nil {
		t.Fatalf("unknown assignment changed known coverage: %+v", active[0])
	}
	if _, err := p.Goals("missing", false); err == nil {
		t.Fatal("missing ID became an empty list")
	}
}

func TestPlanningReservationNativeParityRetainsArchivedAndMissing(t *testing.T) {
	in := planningFixture()
	retained := in.Goals[0]
	retained.ID = "old"
	retained.Archived = true
	retained.Allocations = []domain.GoalAllocation{{AccountID: "a", Amount: money.New(3000, "USD")}, {AccountID: "gone", Amount: money.New(100, "USD")}}
	in.Goals = append(in.Goals, retained)
	p := NewPlanning(in)
	facts, err := p.Reservations("")
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 2 || facts[0].Reserved.Amount.Minor != "5000" || facts[0].Free.Amount.Minor != "5000" || len(facts[0].Goals) != 2 || facts[1].Exists || facts[1].Known || facts[1].Free.Amount != nil {
		t.Fatalf("retained reservation mismatch: %+v", facts)
	}
	account, err := p.Account("a")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(account.Reservation, facts[0]) || account.Balance.Amount.Minor != "10000" {
		t.Fatalf("account disagrees with shared position: %+v", account)
	}
	if _, err := p.Account("gone"); err == nil {
		t.Fatal("missing account became zero")
	}
}

func TestPlanningAccountNativeFXAndExactExtremeJSON(t *testing.T) {
	in := planningFixture()
	in.Accounts[0].Currency = "JPY"
	in.Accounts[0].OpeningBalance = money.New(math.MaxInt64, "JPY")
	in.Goals = nil
	f, err := NewPlanning(in).Account("a")
	if err != nil {
		t.Fatal(err)
	}
	if !f.Balance.Known || f.BaseBalance.Known || f.BaseBalance.Amount != nil || f.Balance.Amount.Minor != "9223372036854775807" || f.Balance.Amount.Decimals != 0 {
		t.Fatalf("extreme or unavailable facts lost: %+v", f)
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"minor":"9223372036854775807"`) {
		t.Fatalf("minor was not a string: %s", b)
	}
	in.Transactions = []domain.Transaction{{AccountID: "a", Amount: money.New(1, "JPY")}}
	f, err = NewPlanning(in).Account("a")
	if err != nil {
		t.Fatal(err)
	}
	if f.Balance.Known || f.Reservation.Balance.Known {
		t.Fatalf("overflow became known: %+v", f)
	}
}

func TestPlanningProjectionCanonicalActiveDriversAndFailures(t *testing.T) {
	in := planningFixture()
	now := in.Now
	active := domain.Recurring{ID: "r", Label: "Rent", AccountID: "a", Amount: money.New(-100, "USD"), NextDue: now.AddDate(0, 0, 2), Cadence: domain.CadenceWeekly}
	paused := active
	paused.ID = "paused"
	paused.Label = "Paused"
	paused.Paused = true
	paused.Amount = money.New(-9000, "USD")
	in.Recurring = []domain.Recurring{active, paused}
	p := NewPlanning(in)
	f, err := p.Project("a", now, 30)
	if err != nil {
		t.Fatal(err)
	}
	want, werr := accountview.PrepareProjectionInputs([]domain.Recurring{active}).Project("a", in.Accounts[0].OpeningBalance, now, 30)
	if werr != nil {
		t.Fatal(werr)
	}
	if !f.Known || f.End.Amount.Minor != "9500" || f.Low.Amount.Minor != "9500" || len(f.Drivers) != len(want.Drivers) || f.LowDate != want.LowDate {
		t.Fatalf("projection mismatch: %+v / %+v", f, want)
	}
	for _, tc := range []struct {
		name  string
		start int64
		flow  money.Money
	}{
		{"overflow", math.MaxInt64, money.New(1, "USD")}, {"currency", 100, money.New(1, "EUR")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in.Accounts[0].OpeningBalance = money.New(tc.start, "USD")
			in.Recurring = []domain.Recurring{active}
			in.Recurring[0].Amount = tc.flow
			f, err := NewPlanning(in).Project("a", now, 30)
			if err != nil {
				t.Fatal(err)
			}
			if f.Known || f.End.Amount != nil || f.Error == "" {
				t.Fatalf("invalid projection claimed zero: %+v", f)
			}
		})
	}
	if _, err := p.Project("a", now, 367); err == nil {
		t.Fatal("unbounded projection accepted")
	}
	if _, err := p.Project("missing", now, 30); err == nil {
		t.Fatal("missing account projected at zero")
	}
}

func TestPlanningFundingDisclosesMissingFXInsteadOfNoAvailableMoney(t *testing.T) {
	in := planningFixture()
	in.Goals[0].CurrentAmount = money.Zero("EUR")
	in.Goals[0].Allocations = nil
	p := NewPlanning(in)
	f, err := p.Funding("g")
	if err != nil {
		t.Fatal(err)
	}
	if !f.Known || f.Suggestion == nil || f.Suggestion.Amount.Minor != "5000" {
		t.Fatalf("canonical suggestion missing: %+v", f)
	}
	in.Rates.Rates = nil
	f, err = NewPlanning(in).Funding("g")
	if err != nil {
		t.Fatal(err)
	}
	if f.Known || f.Suggestion != nil || len(f.SourceIssues) != 1 {
		t.Fatalf("missing FX claimed no funding: %+v", f)
	}
}

func TestPlanningRecurringWindowPaginationPauseAndCalendar(t *testing.T) {
	in := planningFixture()
	ny := time.FixedZone("New York", -4*3600)
	in.Recurring = []domain.Recurring{{ID: "daily", Label: "Daily", AccountID: "a", Amount: money.New(math.MinInt64, "USD"), Cadence: domain.CadenceDaily, NextDue: time.Date(2026, 10, 20, 23, 0, 0, 0, ny)}, {ID: "paused", Label: "Paused", AccountID: "a", Paused: true, Amount: money.New(100, "USD"), Cadence: domain.CadenceDaily, NextDue: in.Now}}
	from := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	before := from.AddDate(0, 0, 3)
	p := NewPlanning(in)
	f, err := p.Recurring("", "", false, from, before, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if f.Total != 3 || len(f.Occurrences) != 2 || f.NextOffset == nil || *f.NextOffset != 2 || f.PausedExcluded != 1 || f.DefinitionCount != 1 || f.Occurrences[0].Due != from || f.Occurrences[0].Amount.Minor != "-9223372036854775808" {
		t.Fatalf("calendar or pagination mismatch: %+v", f)
	}
	second, _ := p.Recurring("", "", true, from, before, 2, 2)
	if second.DefinitionCount != 2 || second.Total != 3 || len(second.Occurrences) != 1 || second.NextOffset != nil {
		t.Fatalf("paused/last page mismatch: %+v", second)
	}
	if _, err := p.Recurring("", "", false, from, before, -1, 2); err == nil {
		t.Fatal("invalid page accepted")
	}
	if _, err := p.Recurring("", "", false, from, from.AddDate(0, 0, 367), 0, 2); err == nil {
		t.Fatal("unbounded occurrence window accepted")
	}
	if _, err := p.Recurring("unknown", "", false, from, before, 0, 2); err == nil {
		t.Fatal("missing recurring became empty")
	}
}

func TestPlanningRecurringInvalidAndArchivedSourcesRemainVisible(t *testing.T) {
	in := planningFixture()
	in.Accounts[0].Archived = true
	in.Recurring = []domain.Recurring{{ID: "archive", Label: "Kept", AccountID: "a", Amount: money.New(1, "USD"), Cadence: domain.CadenceWeekly, NextDue: in.Now}, {ID: "bad", AccountID: "a", Cadence: "invalid", NextDue: in.Now}, {ID: "missing", AccountID: "gone", Cadence: domain.CadenceDaily, NextDue: in.Now}, {ID: "ancient", AccountID: "a", Cadence: domain.CadenceDaily, NextDue: in.Now.AddDate(-20, 0, 0)}}
	f, err := NewPlanning(in).Recurring("", "", true, in.Now, in.Now.AddDate(0, 0, 7), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !f.Incomplete || f.DefinitionCount != 4 || !f.Definitions[0].AccountArchived || f.Definitions[1].Error == "" || f.Definitions[2].AccountExists || f.Definitions[3].Error == "" {
		t.Fatalf("unavailable schedule disappeared: %+v", f)
	}
}

func TestPlanningRecurringPreservesSavedMonthEndCursorAndInclusiveStart(t *testing.T) {
	in := planningFixture()
	in.Recurring = []domain.Recurring{{ID: "month", Label: "Monthly", AccountID: "a", Amount: money.New(-1, "USD"), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)}}
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	f, err := NewPlanning(in).Recurring("", "", false, from, from.AddDate(0, 2, 0), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if f.Total != 2 || f.Occurrences[0].Due.Day() != 31 || f.Occurrences[1].Due.Day() != 30 {
		t.Fatalf("saved month-end cursor changed: %+v", f)
	}
	in.Recurring[0].Cadence = domain.CadenceDaily
	in.Recurring[0].NextDue = from.AddDate(0, 0, 2)
	f, err = NewPlanning(in).Recurring("", "", false, from, from.AddDate(0, 0, 4), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if f.Total != 2 || !f.Occurrences[0].Due.Equal(in.Recurring[0].NextDue) {
		t.Fatalf("fabricated occurrences before saved cursor: %+v", f)
	}
}

func TestPlanningProjectionRejectsTruncatedOrInvalidSchedules(t *testing.T) {
	for _, tc := range []struct {
		name    string
		due     time.Time
		cadence domain.RecurringCadence
	}{
		{"stale daily", planningFixture().Now.AddDate(0, 0, -100), domain.CadenceDaily},
		{"invalid", planningFixture().Now, "broken"},
		{"missing date", time.Time{}, domain.CadenceMonthly},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := planningFixture()
			in.Recurring = []domain.Recurring{{ID: "r", AccountID: "a", Amount: money.New(-1, "USD"), Cadence: tc.cadence, NextDue: tc.due}}
			f, err := NewPlanning(in).Project("a", in.Now, 366)
			if err != nil {
				t.Fatal(err)
			}
			if f.Known || f.Error == "" || f.End.Amount != nil {
				t.Fatalf("partial schedule exposed a known projection: %+v", f)
			}
		})
	}
}

func TestPlanningRecurringUnassignedSourceKeepsKnownCadenceDates(t *testing.T) {
	in := planningFixture()
	from := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, source := range []string{"", "deleted"} {
		t.Run("source "+source, func(t *testing.T) {
			in.Recurring = []domain.Recurring{{ID: "income", Label: "Pay", AccountID: source, Amount: money.New(500, "USD"), Cadence: domain.CadenceDaily, NextDue: from}}
			f, err := NewPlanning(in).Recurring("", "", false, from, from.AddDate(0, 0, 3), 0, 100)
			if err != nil {
				t.Fatal(err)
			}
			if f.DefinitionCount != 1 || f.Definitions[0].AccountExists || f.Definitions[0].SourceKnown || !f.Definitions[0].ScheduleKnown || !f.OccurrencesComplete || f.Definitions[0].SourceError == "" || f.Definitions[0].Error == "" || f.Total != 3 || len(f.Occurrences) != 3 || !f.Occurrences[0].Due.Equal(from) {
				t.Fatalf("unknown source suppressed known cadence: %+v", f)
			}
		})
	}
}

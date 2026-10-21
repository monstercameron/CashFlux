// SPDX-License-Identifier: MIT

package goalview

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBuildRowForeignCoverageMatchesFundingAndLifecycle(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	g := goal("foreign", "EUR", 2000, 10000)
	g.TargetDate = now.AddDate(0, 2, 0)
	g.Allocations = []domain.GoalAllocation{{Amount: money.New(1000, "USD")}}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	v := Build(Input{Goals: []domain.Goal{g}, Rates: rates, Now: now})
	r := v.Rows[g.ID]
	if r.Evaluation.Err != nil || r.Evaluation.Coverage.Covered.Amount != 2500 || r.Evaluation.Coverage.Remaining.Amount != 7500 || r.Evaluation.Coverage.CoveragePercent != 25 {
		t.Fatalf("row: %+v", r)
	}
	gap, err := fundingGap(g, rates)
	if err != nil || gap != r.Evaluation.Coverage.Remaining {
		t.Fatalf("funding gap differs: %v %v", gap, err)
	}
	when, ok := Retarget(g, r.Evaluation, Pace{FairMinor: 1000}, "USD", rates, now)
	want, _ := goals.WorkableDate(7500, 500, now)
	if !ok || !when.Equal(want) {
		t.Fatalf("retarget ignored converted fair share: %v want %v", when, want)
	}
	g.TargetDate = now.Add(-time.Hour)
	v = Build(Input{Goals: []domain.Goal{g}, Rates: currency.Rates{Base: "USD"}, Now: now})
	if len(v.Missed) != 0 || len(v.Active) != 1 || v.Rows[g.ID].Evaluation.Err == nil || v.Health[g.ID].Health != goals.HealthNone {
		t.Fatalf("unknown FX produced lifecycle/pace: %+v", v)
	}
}

func TestBuildRowSharesChecklistProgressAndPreview(t *testing.T) {
	g := domain.Goal{ID: "g", Name: "Checklist", Kind: domain.GoalKindChecklist}
	tasks := []domain.Task{{ID: "done", RelatedType: domain.RelatedGoal, RelatedID: "g", Status: domain.StatusDone}, {ID: "open", RelatedType: domain.RelatedGoal, RelatedID: "g", Status: domain.StatusOpen}, {ID: "unrelated", Status: domain.StatusDone}}
	before := append([]domain.Task(nil), tasks...)
	v := Build(Input{Goals: []domain.Goal{g}, Tasks: tasks, Now: time.Now()})
	r := v.Rows[g.ID]
	if r.Tasks.Total != 2 || r.Tasks.Done != 1 || r.Evaluation.Progress.Percent != 50 || r.Tasks.Preview[0].ID != "open" {
		t.Fatalf("checklist model: %+v", r)
	}
	if !reflect.DeepEqual(tasks, before) {
		t.Fatal("shared task collection changed")
	}
}

// BenchmarkRowPreparation measures the repeated task scans removed from row
// rendering against the shared index and canonical native row evaluation.
func BenchmarkRowPreparation(b *testing.B) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	gs := make([]domain.Goal, 200)
	tasks := make([]domain.Task, 20000)
	for i := range gs {
		gs[i] = domain.Goal{ID: fmt.Sprint(i), Kind: domain.GoalKindChecklist}
	}
	for i := range tasks {
		tasks[i] = domain.Task{ID: fmt.Sprint(i), RelatedType: domain.RelatedGoal, RelatedID: gs[i%len(gs)].ID, Status: domain.StatusOpen}
		if i%3 == 0 {
			tasks[i].Status = domain.StatusDone
		}
	}
	b.Run("previous-row-scans", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			for _, g := range gs {
				_ = goals.EvaluateProgress(g, tasks, now)
				_ = goals.Reached(g, tasks, now)
				_ = goals.LinkedTasks(tasks, g.ID)
				_, _ = goals.TaskCounts(tasks, g.ID)
			}
		}
	})
	b.Run("shared-row-models", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			index := goals.BuildTaskIndex(tasks)
			for _, g := range gs {
				_ = BuildRow(g, index[g.ID], currency.Rates{}, now)
			}
		}
	})
}

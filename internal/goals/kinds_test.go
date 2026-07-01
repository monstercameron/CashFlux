// SPDX-License-Identifier: MIT

package goals

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func task(id, goalID string, status domain.TaskStatus) domain.Task {
	return domain.Task{ID: id, Title: id, Status: status, Priority: domain.PriorityMedium,
		RelatedType: domain.RelatedGoal, RelatedID: goalID}
}

func TestLinkedTasks(t *testing.T) {
	tasks := []domain.Task{
		task("a", "g1", domain.StatusOpen),
		task("b", "g2", domain.StatusOpen),
		task("c", "g1", domain.StatusDone),
		{ID: "d", Title: "unlinked", Status: domain.StatusOpen, RelatedType: domain.RelatedNone},
		{ID: "e", Title: "acct", Status: domain.StatusOpen, RelatedType: domain.RelatedAccount, RelatedID: "g1"},
	}
	got := LinkedTasks(tasks, "g1")
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Fatalf("LinkedTasks(g1) = %+v, want [a c]", got)
	}
	if LinkedTasks(tasks, "") != nil {
		t.Errorf("LinkedTasks with blank id should match nothing")
	}
	if got := LinkedTasks(tasks, "none"); got != nil {
		t.Errorf("LinkedTasks(none) = %+v, want nil", got)
	}
}

func TestTaskCountsAndChecklistPercent(t *testing.T) {
	tasks := []domain.Task{
		task("a", "g1", domain.StatusDone),
		task("b", "g1", domain.StatusOpen),
		task("c", "g1", domain.StatusDone),
		task("d", "g2", domain.StatusDone),
	}
	done, total := TaskCounts(tasks, "g1")
	if done != 2 || total != 3 {
		t.Fatalf("TaskCounts(g1) = %d/%d, want 2/3", done, total)
	}

	cases := []struct {
		done, total, want int
	}{
		{0, 0, 0},
		{0, 4, 0},
		{2, 3, 66},
		{3, 3, 100},
		{5, 3, 100}, // clamp
		{1, 0, 0},   // zero total
	}
	for _, c := range cases {
		if got := ChecklistPercent(c.done, c.total); got != c.want {
			t.Errorf("ChecklistPercent(%d,%d) = %d, want %d", c.done, c.total, got, c.want)
		}
	}
}

func TestHabitStreak(t *testing.T) {
	base := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	weekly := func(offsets ...int) domain.Goal {
		g := domain.Goal{Kind: domain.GoalKindHabit, HabitCadence: domain.CadenceWeekly, HabitTarget: 12}
		for _, d := range offsets {
			g.CheckIns = append(g.CheckIns, base.AddDate(0, 0, d))
		}
		return g
	}
	cases := []struct {
		name string
		g    domain.Goal
		want int
	}{
		{"none", weekly(), 0},
		{"single", weekly(0), 1},
		{"three-consecutive", weekly(0, -7, -14), 3},
		{"gap-breaks", weekly(0, -7, -30), 2}, // -30 is >1.5 weeks from -7
		{"drift-tolerated", weekly(0, -8, -17), 3},
		{"unsorted-input", weekly(-14, 0, -7), 3},
	}
	for _, c := range cases {
		if got := HabitStreak(c.g, base); got != c.want {
			t.Errorf("%s: HabitStreak = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestEvaluateProgress(t *testing.T) {
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	usd := func(a int64) money.Money { return money.New(a, "USD") }

	t.Run("financial", func(t *testing.T) {
		g := domain.Goal{ID: "f", TargetAmount: usd(1000), CurrentAmount: usd(250)}
		p := EvaluateProgress(g, nil, now)
		if p.Kind != domain.GoalKindFinancial || p.Percent != 25 || p.Complete {
			t.Fatalf("financial: %+v, want kind=financial pct=25 complete=false", p)
		}
	})

	t.Run("financial-complete", func(t *testing.T) {
		g := domain.Goal{ID: "f", TargetAmount: usd(1000), CurrentAmount: usd(1000)}
		p := EvaluateProgress(g, nil, now)
		if p.Percent != 100 || !p.Complete {
			t.Fatalf("financial-complete: %+v", p)
		}
	})

	t.Run("checklist", func(t *testing.T) {
		g := domain.Goal{ID: "c", Kind: domain.GoalKindChecklist}
		tasks := []domain.Task{
			task("t1", "c", domain.StatusDone),
			task("t2", "c", domain.StatusDone),
			task("t3", "c", domain.StatusOpen),
			task("t4", "c", domain.StatusOpen),
		}
		p := EvaluateProgress(g, tasks, now)
		if p.Kind != domain.GoalKindChecklist || p.Done != 2 || p.Total != 4 || p.Percent != 50 || p.Complete {
			t.Fatalf("checklist: %+v, want 2/4 50%% not-complete", p)
		}
	})

	t.Run("checklist-empty", func(t *testing.T) {
		g := domain.Goal{ID: "c", Kind: domain.GoalKindChecklist}
		p := EvaluateProgress(g, nil, now)
		if p.Percent != 0 || p.Total != 0 || p.Complete {
			t.Fatalf("checklist-empty should be 0%% and not complete: %+v", p)
		}
	})

	t.Run("checklist-complete", func(t *testing.T) {
		g := domain.Goal{ID: "c", Kind: domain.GoalKindChecklist}
		tasks := []domain.Task{task("t1", "c", domain.StatusDone), task("t2", "c", domain.StatusDone)}
		p := EvaluateProgress(g, tasks, now)
		if p.Percent != 100 || !p.Complete {
			t.Fatalf("checklist-complete: %+v", p)
		}
	})

	t.Run("milestone-open", func(t *testing.T) {
		g := domain.Goal{ID: "m", Kind: domain.GoalKindMilestone}
		p := EvaluateProgress(g, nil, now)
		if p.Percent != 0 || p.Done != 0 || p.Total != 1 || p.Complete {
			t.Fatalf("milestone-open: %+v, want 0/1 not-complete", p)
		}
	})

	t.Run("milestone-done", func(t *testing.T) {
		g := domain.Goal{ID: "m", Kind: domain.GoalKindMilestone, DoneAt: now}
		p := EvaluateProgress(g, nil, now)
		if p.Percent != 100 || p.Done != 1 || p.Total != 1 || !p.Complete {
			t.Fatalf("milestone-done: %+v, want 1/1 complete", p)
		}
	})

	t.Run("habit", func(t *testing.T) {
		g := domain.Goal{ID: "h", Kind: domain.GoalKindHabit, HabitCadence: domain.CadenceWeekly, HabitTarget: 4,
			CheckIns: []time.Time{now, now.AddDate(0, 0, -7)}}
		p := EvaluateProgress(g, nil, now)
		if p.Kind != domain.GoalKindHabit || p.Done != 2 || p.Total != 4 || p.Percent != 50 || p.Streak != 2 || p.Complete {
			t.Fatalf("habit: %+v, want 2/4 50%% streak=2 not-complete", p)
		}
	})

	t.Run("habit-complete", func(t *testing.T) {
		g := domain.Goal{ID: "h", Kind: domain.GoalKindHabit, HabitCadence: domain.CadenceWeekly, HabitTarget: 2,
			CheckIns: []time.Time{now, now.AddDate(0, 0, -7)}}
		p := EvaluateProgress(g, nil, now)
		if p.Percent != 100 || !p.Complete {
			t.Fatalf("habit-complete: %+v", p)
		}
	})
}

// SPDX-License-Identifier: MIT

package workflow

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestIsScheduledWorkflowDue(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		w    Workflow
		want bool
	}{
		{
			name: "manual trigger never due",
			w:    Workflow{Trigger: Trigger{Kind: TriggerManual, NextRun: now.AddDate(0, -1, 0)}},
			want: false,
		},
		{
			name: "due exactly at now",
			w:    Workflow{Trigger: Trigger{Kind: TriggerScheduled, Cadence: domain.CadenceMonthly, NextRun: now}},
			want: true,
		},
		{
			name: "overdue",
			w:    Workflow{Trigger: Trigger{Kind: TriggerScheduled, Cadence: domain.CadenceMonthly, NextRun: now.AddDate(0, -2, 0)}},
			want: true,
		},
		{
			name: "future — not due",
			w:    Workflow{Trigger: Trigger{Kind: TriggerScheduled, Cadence: domain.CadenceMonthly, NextRun: now.AddDate(0, 1, 0)}},
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsScheduledWorkflowDue(c.w, now); got != c.want {
				t.Errorf("IsScheduledWorkflowDue = %v, want %v", got, c.want)
			}
		})
	}
}

func TestAdvanceScheduledNextRun(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	t.Run("monthly catch-up 2 missed periods", func(t *testing.T) {
		w := Workflow{Trigger: Trigger{
			Kind: TriggerScheduled, Cadence: domain.CadenceMonthly,
			NextRun: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		}}
		AdvanceScheduledNextRun(&w, now)
		if !w.Trigger.NextRun.After(now) {
			t.Errorf("NextRun should be past now, got %v", w.Trigger.NextRun)
		}
	})

	t.Run("weekly advance", func(t *testing.T) {
		w := Workflow{Trigger: Trigger{
			Kind: TriggerScheduled, Cadence: domain.CadenceWeekly,
			NextRun: now.AddDate(0, 0, -14),
		}}
		AdvanceScheduledNextRun(&w, now)
		if !w.Trigger.NextRun.After(now) {
			t.Errorf("NextRun should be past now, got %v", w.Trigger.NextRun)
		}
	})

	t.Run("non-scheduled no-op", func(t *testing.T) {
		w := Workflow{Trigger: Trigger{Kind: TriggerManual, NextRun: now.AddDate(-1, 0, 0)}}
		before := w.Trigger.NextRun
		AdvanceScheduledNextRun(&w, now)
		if !w.Trigger.NextRun.Equal(before) {
			t.Errorf("non-scheduled should not advance, got %v", w.Trigger.NextRun)
		}
	})

	t.Run("quarterly advance", func(t *testing.T) {
		w := Workflow{Trigger: Trigger{
			Kind: TriggerScheduled, Cadence: domain.CadenceQuarterly,
			NextRun: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		}}
		AdvanceScheduledNextRun(&w, now)
		if !w.Trigger.NextRun.After(now) {
			t.Errorf("NextRun should be past now, got %v", w.Trigger.NextRun)
		}
	})
}

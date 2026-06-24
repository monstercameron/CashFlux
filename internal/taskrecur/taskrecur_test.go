// SPDX-License-Identifier: MIT

package taskrecur

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func date(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

func TestNextOccurrence(t *testing.T) {
	now := date(2026, 6, 1)

	cases := []struct {
		name      string
		task      domain.Task
		wantOK    bool
		wantDue   time.Time
		wantOpen  bool
		wantTitle string
		wantPrio  domain.TaskPriority
		wantRecur domain.RecurringCadence
	}{
		{
			name:   "no recurrence → false",
			task:   domain.Task{ID: "a", Title: "one-shot", Status: domain.StatusDone},
			wantOK: false,
		},
		{
			name: "weekly advances 7 days",
			task: domain.Task{
				ID: "b", Title: "weekly chore", Status: domain.StatusDone,
				Priority: domain.PriorityHigh, Recurrence: domain.CadenceWeekly,
				Due: date(2026, 6, 1),
			},
			wantOK:    true,
			wantDue:   date(2026, 6, 8),
			wantOpen:  true,
			wantTitle: "weekly chore",
			wantPrio:  domain.PriorityHigh,
			wantRecur: domain.CadenceWeekly,
		},
		{
			name: "monthly advances one month",
			task: domain.Task{
				ID: "c", Title: "review budget", Status: domain.StatusDone,
				Priority: domain.PriorityMedium, Recurrence: domain.CadenceMonthly,
				Due: date(2026, 6, 15),
			},
			wantOK:    true,
			wantDue:   date(2026, 7, 15), // same day next month (no overflow)
			wantOpen:  true,
			wantTitle: "review budget",
			wantPrio:  domain.PriorityMedium,
			wantRecur: domain.CadenceMonthly,
		},
		{
			name: "quarterly advances 3 months",
			task: domain.Task{
				ID: "d", Title: "rebalance 401k", Status: domain.StatusDone,
				Priority: domain.PriorityLow, Recurrence: domain.CadenceQuarterly,
				Due: date(2026, 3, 15),
			},
			wantOK:    true,
			wantDue:   date(2026, 6, 15),
			wantOpen:  true,
			wantTitle: "rebalance 401k",
			wantPrio:  domain.PriorityLow,
			wantRecur: domain.CadenceQuarterly,
		},
		{
			name: "yearly advances 12 months",
			task: domain.Task{
				ID: "e", Title: "annual review", Status: domain.StatusDone,
				Recurrence: domain.CadenceYearly,
				Due:        date(2026, 6, 1),
			},
			wantOK:    true,
			wantDue:   date(2027, 6, 1),
			wantOpen:  true,
			wantTitle: "annual review",
			wantRecur: domain.CadenceYearly,
		},
		{
			name: "zero Due falls back to now",
			task: domain.Task{
				ID: "f", Title: "undated recurring", Status: domain.StatusDone,
				Recurrence: domain.CadenceWeekly,
				// Due is zero
			},
			wantOK:    true,
			wantDue:   date(2026, 6, 8), // now + 7 days
			wantOpen:  true,
			wantTitle: "undated recurring",
			wantRecur: domain.CadenceWeekly,
		},
		{
			name: "preserves RelatedType/RelatedID/MemberID/ParentID",
			task: domain.Task{
				ID: "g", Title: "linked", Status: domain.StatusDone,
				Recurrence:  domain.CadenceMonthly,
				Due:         date(2026, 6, 1),
				RelatedType: domain.RelatedAccount, RelatedID: "acc1",
				MemberID: "mem1", ParentID: "par1",
			},
			wantOK:    true,
			wantDue:   date(2026, 7, 1),
			wantOpen:  true,
			wantTitle: "linked",
			wantRecur: domain.CadenceMonthly,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := NextOccurrence(tc.task, "new-id", now)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.ID != "new-id" {
				t.Errorf("ID = %q, want %q", got.ID, "new-id")
			}
			if !got.Due.Equal(tc.wantDue) {
				t.Errorf("Due = %v, want %v", got.Due, tc.wantDue)
			}
			if got.Status != domain.StatusOpen {
				t.Errorf("Status = %v, want StatusOpen", got.Status)
			}
			if tc.wantTitle != "" && got.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tc.wantTitle)
			}
			if tc.wantPrio != "" && got.Priority != tc.wantPrio {
				t.Errorf("Priority = %v, want %v", got.Priority, tc.wantPrio)
			}
			if got.Recurrence != tc.wantRecur {
				t.Errorf("Recurrence = %v, want %v", got.Recurrence, tc.wantRecur)
			}
			// Check link fields preserved for the linked test case
			if tc.task.RelatedID != "" && got.RelatedID != tc.task.RelatedID {
				t.Errorf("RelatedID = %q, want %q", got.RelatedID, tc.task.RelatedID)
			}
			if tc.task.MemberID != "" && got.MemberID != tc.task.MemberID {
				t.Errorf("MemberID = %q, want %q", got.MemberID, tc.task.MemberID)
			}
			if tc.task.ParentID != "" && got.ParentID != tc.task.ParentID {
				t.Errorf("ParentID = %q, want %q", got.ParentID, tc.task.ParentID)
			}
		})
	}
}

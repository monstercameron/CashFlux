// SPDX-License-Identifier: MIT
package taskview

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/tasksort"
)

func day(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }
func ids(tasks []domain.Task) []string {
	out := make([]string, len(tasks))
	for i, task := range tasks {
		out[i] = task.ID
	}
	return out
}

func TestSelect(t *testing.T) {
	tasks := []domain.Task{
		{ID: "parent", Title: "Budget review", Status: domain.StatusDone, Priority: domain.PriorityHigh, Due: day("2026-10-19")},
		{ID: "child", ParentID: "parent", Title: "Call bank", Notes: "REVIEW charge", Status: domain.StatusOpen, Priority: domain.PriorityLow, Due: day("2026-10-20"), RelatedType: domain.RelatedAccount, RelatedID: "a"},
		{ID: "late", Title: "Pay bill", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Due: day("2026-10-19"), RelatedType: domain.RelatedAccount, RelatedID: "b"},
		{ID: "undated", Title: "Plan", Status: domain.StatusOpen, RelatedType: domain.RelatedAccount},
	}
	before := append([]domain.Task(nil), tasks...)
	for _, tc := range []struct {
		name   string
		filter Filter
		want   []string
	}{
		{"all", Filter{}, []string{"parent", "child", "late", "undated"}},
		{"hidden parent preserves child", Filter{HideDone: true}, []string{"child", "late", "undated"}},
		{"title and notes", Filter{Search: "  review "}, []string{"parent", "child"}},
		{"priority", Filter{Priority: domain.PriorityHigh}, []string{"parent", "late"}},
		{"invalid priority ignored", Filter{Priority: "invalid"}, []string{"parent", "child", "late", "undated"}},
		{"complete links", Filter{Link: "account"}, []string{"child", "late"}},
		{"incomplete links count unlinked", Filter{Link: "none"}, []string{"parent", "undated"}},
		{"specific linked entity", Filter{Link: "account", LinkID: "a"}, []string{"child"}},
		{"today excludes done and undated", Filter{Quick: tasksort.QuickToday, TodayISO: "2026-10-20"}, []string{"child"}},
		{"unknown quick view falls back to all", Filter{Quick: "invalid", TodayISO: "2026-10-20"}, []string{"parent", "child", "late", "undated"}},
		{"overdue excludes done", Filter{Quick: tasksort.QuickOverdue, TodayISO: "2026-10-20"}, []string{"late"}},
		{"combined", Filter{HideDone: true, Search: "bill", Link: "account", Priority: domain.PriorityHigh, Quick: tasksort.QuickOverdue, TodayISO: "2026-10-20"}, []string{"late"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Select(tasks, tc.filter)
			if !reflect.DeepEqual(ids(got), tc.want) {
				t.Fatalf("got %v want %v", ids(got), tc.want)
			}
			if len(got) > 0 {
				got[0].Title = "changed"
			}
			if !reflect.DeepEqual(tasks, before) {
				t.Fatal("input mutated")
			}
		})
	}
}

func TestCount(t *testing.T) {
	tasks := []domain.Task{{Status: domain.StatusDone, Due: day("2026-10-19")}, {Status: domain.StatusOpen, Due: day("2026-10-19")}, {Status: domain.StatusOpen, Due: day("2026-10-20")}, {Status: domain.StatusOpen}, {Status: domain.StatusOpen, Due: day("2026-10-21")}}
	if got := Count(tasks, "2026-10-20"); got != (Counts{Open: 4, Overdue: 1, Today: 1, Done: 1, Total: 5, DonePct: 20}) {
		t.Fatalf("counts %+v", got)
	}
	if got := Count(nil, "2026-10-20"); got != (Counts{}) {
		t.Fatalf("empty %+v", got)
	}
	if got := Count(tasks, ""); got.Today != 0 || got.Overdue != 0 {
		t.Fatalf("unset today %+v", got)
	}
}

func TestByDueDay(t *testing.T) {
	d := day("2026-10-20")
	tasks := []domain.Task{{ID: "low", Due: d, Priority: domain.PriorityLow}, {ID: "medium", Due: d, Priority: domain.PriorityMedium}, {ID: "h2", Title: "B", Due: d, Priority: domain.PriorityHigh}, {ID: "h1", Title: "A", Due: d, Priority: domain.PriorityHigh}, {ID: "tie", Title: "A", Due: d, Priority: domain.PriorityHigh}, {ID: "undated"}}
	got := ByDueDay(tasks)
	if len(got) != 1 || !reflect.DeepEqual(ids(got["2026-10-20"]), []string{"h1", "tie", "h2", "medium", "low"}) {
		t.Fatalf("buckets %v", got)
	}
	got["2026-10-20"][0].Title = "changed"
	if tasks[3].Title != "A" {
		t.Fatal("input mutated")
	}
}

func TestTaskDatesRetainTheirCalendarDayAcrossTimeZones(t *testing.T) {
	// The UTC instant falls on the previous day; the task's own calendar label
	// remains Oct 20 in counts, filters and calendar buckets.
	due := time.Date(2026, 10, 20, 0, 30, 0, 0, time.FixedZone("UTC+14", 14*60*60))
	tasks := []domain.Task{{ID: "local-day", Due: due, Status: domain.StatusOpen}}
	if got := Count(tasks, "2026-10-20"); got.Today != 1 || got.Overdue != 0 {
		t.Fatalf("calendar counts converted the date's zone: %+v", got)
	}
	if got := Select(tasks, Filter{Quick: tasksort.QuickToday, TodayISO: "2026-10-20"}); len(got) != 1 {
		t.Fatal("today filter converted the task's calendar day")
	}
	if got := ByDueDay(tasks); len(got["2026-10-20"]) != 1 {
		t.Fatalf("calendar bucket converted the task's calendar day: %v", got)
	}
}

func BenchmarkSelect(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			tasks := make([]domain.Task, n)
			for i := range tasks {
				tasks[i] = domain.Task{ID: fmt.Sprint(i), Title: "Review household account", Notes: "check payment", Priority: domain.PriorityHigh, Status: domain.StatusOpen, Due: day("2026-10-19"), RelatedType: domain.RelatedAccount, RelatedID: "a"}
			}
			f := Filter{HideDone: true, Search: "review", Priority: domain.PriorityHigh, Link: "account", Quick: tasksort.QuickOverdue, TodayISO: "2026-10-20"}
			for _, impl := range []struct {
				name        string
				selectTasks func([]domain.Task, Filter) []domain.Task
			}{
				{"single-pass", Select}, {"previous-multiple-passes", previousSelect},
			} {
				b.Run(impl.name, func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_ = impl.selectTasks(tasks, f)
					}
				})
			}
		})
	}
}

// previousSelect preserves the pre-extraction screen pipeline as a benchmark
// baseline. It exercises the common combination of all active filter controls.
func previousSelect(tasks []domain.Task, f Filter) []domain.Task {
	out := tasksort.Visible(tasks, f.HideDone)
	if f.Priority.Valid() {
		kept := out[:0:0]
		for _, task := range out {
			if task.Priority == f.Priority {
				kept = append(kept, task)
			}
		}
		out = kept
	}
	if q := strings.ToLower(strings.TrimSpace(f.Search)); q != "" {
		kept := out[:0:0]
		for _, task := range out {
			if strings.Contains(strings.ToLower(task.Title), q) || strings.Contains(strings.ToLower(task.Notes), q) {
				kept = append(kept, task)
			}
		}
		out = kept
	}
	if f.Link != "" {
		kept := out[:0:0]
		for _, task := range out {
			linked := task.RelatedID != "" && task.RelatedType != "" && task.RelatedType != domain.RelatedNone
			if (f.Link == "none" && !linked) || (f.Link != "none" && linked && string(task.RelatedType) == f.Link) {
				kept = append(kept, task)
			}
		}
		out = kept
	}
	if f.LinkID != "" {
		kept := out[:0:0]
		for _, task := range out {
			if task.RelatedID == f.LinkID {
				kept = append(kept, task)
			}
		}
		out = kept
	}
	return tasksort.FilterQuickView(out, tasksort.ParseQuickView(string(f.Quick)), f.TodayISO)
}

func TestSelectMatchesPreviousPipeline(t *testing.T) {
	now := day("2026-10-20")
	tasks := make([]domain.Task, 100)
	for i := range tasks {
		tasks[i] = domain.Task{ID: fmt.Sprint(i), Title: fmt.Sprintf("Review %d", i), Status: domain.StatusOpen, Priority: []domain.TaskPriority{domain.PriorityHigh, domain.PriorityMedium, domain.PriorityLow}[i%3], Due: now.AddDate(0, 0, i%5-2), RelatedType: domain.RelatedAccount, RelatedID: fmt.Sprint(i % 4)}
		if i%7 == 0 {
			tasks[i].Status = domain.StatusDone
		}
		if i%9 == 0 {
			tasks[i].Due = time.Time{}
		}
		if i%11 == 0 {
			tasks[i].RelatedID = ""
		}
	}
	for _, hide := range []bool{true, false} {
		for _, priority := range []domain.TaskPriority{"", domain.PriorityHigh, domain.PriorityLow} {
			for _, query := range []string{"", " Review ", "2"} {
				for _, link := range []string{"", "none", "account", "goal"} {
					for _, quick := range []tasksort.QuickView{tasksort.QuickAll, tasksort.QuickToday, tasksort.QuickOverdue} {
						f := Filter{HideDone: hide, Priority: priority, Search: query, Link: link, Quick: quick, TodayISO: dateutil.FormatDate(now)}
						if a, b := ids(Select(tasks, f)), ids(previousSelect(tasks, f)); !reflect.DeepEqual(a, b) {
							t.Fatalf("filter %+v: got %v want %v", f, a, b)
						}
					}
				}
			}
		}
	}
}

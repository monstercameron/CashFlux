// SPDX-License-Identifier: MIT

package notifyfeed

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/copytext"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/notify"
)

func taskText(taskTitle string, days int) (title, body copytext.Text) {
	return copytext.Of("notify.taskTitle", "To-do: "+taskTitle, taskTitle),
		copytext.Of("notify.taskBody", "in %d days", days)
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestTaskReminderCandidates(t *testing.T) {
	now := time.Date(2026, time.August, 16, 17, 0, 0, 0, time.UTC)
	tasks := []domain.Task{
		// Inside its own 3-day lead window.
		{ID: "t-lead", Title: "Renew tags", Status: domain.StatusOpen,
			Due: day(2026, time.August, 18), ReminderLeadDays: 3},
		// Due later than its lead window opens — not yet.
		{ID: "t-early", Title: "File taxes", Status: domain.StatusOpen,
			Due: day(2026, time.August, 30), ReminderLeadDays: 3},
		// No lead: reminds on the due date itself, which is today.
		{ID: "t-today", Title: "Pay sitter", Status: domain.StatusOpen,
			Due: day(2026, time.August, 16)},
		// Overdue but inside the backlog cutoff.
		{ID: "t-late", Title: "Call bank", Status: domain.StatusOpen,
			Due: day(2026, time.August, 1)},
		// Backlog: months past due, no longer a reminder.
		{ID: "t-ancient", Title: "Shred 2019 files", Status: domain.StatusOpen,
			Due: day(2026, time.January, 4)},
		// Already done.
		{ID: "t-done", Title: "Order checks", Status: domain.StatusDone,
			Due: day(2026, time.August, 15)},
		// No due date at all.
		{ID: "t-someday", Title: "Read the annuity docs", Status: domain.StatusOpen},
	}

	got := TaskReminderCandidates("default-task-reminder", tasks, 0, now, taskText)

	want := map[string]notify.Severity{
		"t-lead@2026-08-18":  notify.SeverityWarning,
		"t-today@2026-08-16": notify.SeverityCritical,
		"t-late@2026-08-01":  notify.SeverityCritical,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates, want %d: %+v", len(got), len(want), keysOf(got))
	}
	for _, c := range got {
		sev, ok := want[c.OccurrenceKey]
		if !ok {
			t.Errorf("unexpected candidate %q", c.OccurrenceKey)
			continue
		}
		if c.Severity != sev {
			t.Errorf("%s severity = %v, want %v", c.OccurrenceKey, c.Severity, sev)
		}
		if c.Event != notify.EventTaskReminder {
			t.Errorf("%s event = %v", c.OccurrenceKey, c.Event)
		}
		// The copy must travel as key+args, not only as rendered English, or the
		// archived item can never be re-rendered in another language (C362).
		if c.TitleText.Key == "" || c.Title == "" {
			t.Errorf("%s title = %q key %q, want both", c.OccurrenceKey, c.Title, c.TitleText.Key)
		}
	}
}

func keysOf(cs []notify.Candidate) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.OccurrenceKey)
	}
	return out
}

// The occurrence key is what makes a reminder idempotent across app opens, and
// what the notifications screen parses back into "complete this task".
func TestTaskReminderKeyIsStableAcrossOpens(t *testing.T) {
	task := domain.Task{ID: "t-1", Title: "Move savings", Status: domain.StatusOpen,
		Due: day(2026, time.August, 20), ReminderLeadDays: 7}

	morning := TaskReminderCandidates("r", []domain.Task{task}, 0, day(2026, time.August, 16).Add(9*time.Hour), taskText)
	evening := TaskReminderCandidates("r", []domain.Task{task}, 0, day(2026, time.August, 17).Add(22*time.Hour), taskText)

	if len(morning) != 1 || len(evening) != 1 {
		t.Fatalf("got %d then %d candidates, want 1 each", len(morning), len(evening))
	}
	if morning[0].OccurrenceKey != evening[0].OccurrenceKey {
		t.Errorf("key drifted: %q then %q", morning[0].OccurrenceKey, evening[0].OccurrenceKey)
	}
}

// A due time in the middle of the day is still "today", not yesterday.
func TestTaskReminderCountsWholeDays(t *testing.T) {
	if got := daysBetween(
		time.Date(2026, time.August, 16, 17, 0, 0, 0, time.UTC),
		time.Date(2026, time.August, 16, 9, 0, 0, 0, time.UTC),
	); got != 0 {
		t.Errorf("daysBetween same-day = %d, want 0", got)
	}
	if got := daysBetween(
		time.Date(2026, time.August, 16, 23, 0, 0, 0, time.UTC),
		time.Date(2026, time.August, 18, 1, 0, 0, 0, time.UTC),
	); got != 2 {
		t.Errorf("daysBetween = %d, want 2", got)
	}
}

// ─── C408: the evidence behind an alert ──────────────────────────────────────

// A title is a conclusion. Without the trigger, the configured level and the
// observed value, a reader who disagrees has nowhere to look and a reader who
// wants the alert to stop has nothing to adjust.
func TestTaskReminderCarriesItsEvidence(t *testing.T) {
	task := domain.Task{ID: "t-1", Title: "Renew tags", Status: domain.StatusOpen,
		Due: day(2026, time.August, 18), ReminderLeadDays: 3}
	got := TaskReminderCandidates("r", []domain.Task{task}, 0,
		time.Date(2026, time.August, 16, 9, 0, 0, 0, time.UTC), taskText)
	if len(got) != 1 {
		t.Fatalf("got %d candidates", len(got))
	}
	r := got[0].Reason
	if r.Empty() {
		t.Fatal("no evidence on a task reminder")
	}
	if r.Trigger.Key == "" {
		t.Error("the trigger is not a catalog key — a persisted alert could never be re-translated")
	}
	if r.Threshold.Empty() || r.Observed.Empty() {
		t.Errorf("evidence is missing its numbers: %+v", r)
	}
	if r.EntityLabel.Fallback != "Renew tags" {
		t.Errorf("EntityLabel = %q, want the task title", r.EntityLabel.Fallback)
	}
	if r.EntityHref == "" {
		t.Error("no link to the thing the alert is about")
	}
}

// A generator with nothing to add leaves the Reason zero, and the UI must then
// show no drawer at all rather than an empty one.
func TestEmptyReasonIsEmpty(t *testing.T) {
	if !(notify.Reason{}).Empty() {
		t.Error("the zero Reason claimed to carry evidence")
	}
	// A link alone is not evidence — the row already links.
	linkOnly := notify.Reason{EntityHref: "/bills"}
	if !linkOnly.Empty() {
		t.Error("a bare link counted as evidence")
	}
}

// SPDX-License-Identifier: MIT

package taskbulk

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

var now = time.Date(2026, time.August, 16, 15, 30, 0, 0, time.UTC)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func sample() []domain.Task {
	return []domain.Task{
		{ID: "a", Title: "Call bank", Status: domain.StatusOpen, MemberID: "m-cam", Due: d(2026, time.August, 10)},
		{ID: "b", Title: "Renew tags", Status: domain.StatusOpen, Due: d(2026, time.August, 20)},
		{ID: "c", Title: "Order checks", Status: domain.StatusDone, MemberID: "m-cam"},
		{ID: "e", Title: "Read docs", Status: domain.StatusOpen},
	}
}

func sel(ids ...string) map[string]bool {
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func TestPlanOnlyReturnsRealChanges(t *testing.T) {
	// "a" already belongs to m-cam, so assigning m-cam to {a, b} changes only b.
	got := Plan(sample(), sel("a", "b"), Edit{MemberID: "m-cam"}, now)
	if len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("Plan = %+v, want only b", ids(got))
	}
	if got[0].MemberID != "m-cam" {
		t.Errorf("b MemberID = %q, want m-cam", got[0].MemberID)
	}
	// Unselected rows are never touched, even when they would change.
	for _, x := range got {
		if x.ID == "e" {
			t.Error("an unselected task appeared in the plan")
		}
	}
}

func TestPlanUnassignNeedsItsOwnSentinel(t *testing.T) {
	// Empty means "leave the owner alone" — it must not clear anyone.
	if got := Plan(sample(), sel("a"), Edit{MemberID: ""}, now); len(got) != 0 {
		t.Errorf("an empty MemberID changed %v", ids(got))
	}
	got := Plan(sample(), sel("a", "b"), Edit{MemberID: Unassigned}, now)
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("Plan = %v, want only a (b has no owner to clear)", ids(got))
	}
	if got[0].MemberID != "" {
		t.Errorf("a MemberID = %q, want cleared", got[0].MemberID)
	}
}

func TestDueShifts(t *testing.T) {
	task := domain.Task{ID: "a", Due: d(2026, time.August, 10)}
	noDue := domain.Task{ID: "e"}

	cases := []struct {
		shift DueShift
		task  domain.Task
		want  time.Time
		has   bool
	}{
		{ShiftToday, task, d(2026, time.August, 16), true},
		{ShiftTomorrow, task, d(2026, time.August, 17), true},
		{ShiftNextWeek, task, d(2026, time.August, 23), true},
		{ShiftNextMonth, task, d(2026, time.September, 16), true},
		// Relative: from the task's OWN date, so a set that slipped together keeps
		// its internal order.
		{ShiftPushWeek, task, d(2026, time.August, 17), true},
		// …but a task with no date has nothing to push, so it lands a week out.
		{ShiftPushWeek, noDue, d(2026, time.August, 23), true},
		{ShiftClear, task, time.Time{}, false},
		{ShiftNone, task, d(2026, time.August, 10), true},
	}
	for _, c := range cases {
		got, has := DueAfter(c.task, c.shift, now)
		if has != c.has || !got.Equal(c.want) {
			t.Errorf("DueAfter(%s) = %v,%v want %v,%v", c.shift, got.Format("2006-01-02"), has,
				c.want.Format("2006-01-02"), c.has)
		}
	}
}

// A reschedule must land on midnight — a due time inherited from whenever the
// task was created makes same-day tasks sort in an order nobody chose.
func TestDueShiftLandsOnMidnight(t *testing.T) {
	got, _ := DueAfter(domain.Task{Due: time.Date(2026, time.August, 10, 13, 45, 0, 0, time.UTC)}, ShiftPushWeek, now)
	if h, m, s := got.Clock(); h != 0 || m != 0 || s != 0 {
		t.Errorf("shifted due = %v, want midnight", got)
	}
}

func TestPlanClearingADateThatIsAlreadyClearChangesNothing(t *testing.T) {
	if got := Plan(sample(), sel("e"), Edit{Due: ShiftClear}, now); len(got) != 0 {
		t.Errorf("clearing an absent due date produced %v", ids(got))
	}
}

func TestCompletableSkipsWhatIsAlreadyDone(t *testing.T) {
	// "c" is done. Re-completing a recurring task would spawn a second successor.
	got := Completable(sample(), sel("a", "c", "e"))
	want := []string{"a", "e"}
	if len(got) != len(want) {
		t.Fatalf("Completable = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Completable = %v, want %v", got, want)
		}
	}
}

// A selection can name rows that have since gone; the plan must simply skip them.
func TestSelectionOfVanishedRows(t *testing.T) {
	if got := Plan(sample(), sel("ghost"), Edit{MemberID: "m-cam"}, now); len(got) != 0 {
		t.Errorf("a stale id produced %v", ids(got))
	}
	if got := Deletable(sample(), sel("ghost", "b")); len(got) != 1 || got[0] != "b" {
		t.Errorf("Deletable = %v, want [b]", got)
	}
}

func TestRange(t *testing.T) {
	order := []string{"a", "b", "c", "e"}
	if got := Range(order, "b", "e"); len(got) != 3 || got[0] != "b" || got[2] != "e" {
		t.Errorf("Range(b,e) = %v", got)
	}
	// Backwards is the same range — you can drag a selection either way.
	if got := Range(order, "e", "b"); len(got) != 3 || got[0] != "b" {
		t.Errorf("Range(e,b) = %v", got)
	}
	if got := Range(order, "a", "ghost"); got != nil {
		t.Errorf("Range with a missing anchor = %v, want nil", got)
	}
}

func TestEmptyEditIsANoOp(t *testing.T) {
	if !(Edit{}).Empty() {
		t.Error("the zero Edit should be empty")
	}
	if (Edit{Due: ShiftToday}).Empty() {
		t.Error("a due shift is not an empty edit")
	}
}

func ids(ts []domain.Task) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.ID)
	}
	return out
}

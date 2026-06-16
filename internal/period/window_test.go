package period

import (
	"testing"
	"time"
)

func TestNewWindow(t *testing.T) {
	w := NewWindow(Month, d(2026, time.June, 15), time.Monday)
	if !w.From.Equal(d(2026, time.June, 1)) || !w.To.Equal(d(2026, time.June, 1)) {
		t.Errorf("NewWindow anchors = %s..%s, want both 2026-06-01", w.From.Format("2006-01-02"), w.To.Format("2006-01-02"))
	}
	if w.FromLabel() != "Jun 2026" {
		t.Errorf("FromLabel = %q, want Jun 2026", w.FromLabel())
	}
}

func TestSetResolutionResnaps(t *testing.T) {
	w := NewWindow(Month, d(2026, time.May, 10), time.Monday).SetResolution(Quarter)
	if w.Res != Quarter || !w.From.Equal(d(2026, time.April, 1)) {
		t.Errorf("SetResolution(Quarter) from = %s (%s), want 2026-04-01 quarter", w.From.Format("2006-01-02"), w.Res)
	}
}

func TestStepFromClampsTo(t *testing.T) {
	w := NewWindow(Month, d(2026, time.June, 1), time.Monday) // from=to=Jun
	w = w.StepFrom(2)                                         // from -> Aug, to pushed to Aug
	if !w.From.Equal(d(2026, time.August, 1)) || !w.To.Equal(d(2026, time.August, 1)) {
		t.Errorf("StepFrom(+2) = %s..%s, want Aug..Aug", w.From.Format("2006-01-02"), w.To.Format("2006-01-02"))
	}
}

func TestStepToClampsFrom(t *testing.T) {
	w := NewWindow(Month, d(2026, time.June, 1), time.Monday) // from=to=Jun
	w = w.StepTo(-1)                                          // to -> May, from pulled to May
	if !w.From.Equal(d(2026, time.May, 1)) || !w.To.Equal(d(2026, time.May, 1)) {
		t.Errorf("StepTo(-1) = %s..%s, want May..May", w.From.Format("2006-01-02"), w.To.Format("2006-01-02"))
	}
}

func TestWithWeekStartResnapsWeek(t *testing.T) {
	w := NewWindow(Week, d(2026, time.June, 17), time.Monday) // anchors snap to Mon Jun 15
	if !w.From.Equal(d(2026, time.June, 15)) {
		t.Fatalf("setup: From = %s, want 2026-06-15", w.From.Format("2006-01-02"))
	}
	got := w.WithWeekStart(time.Sunday)
	if got.WeekStart != time.Sunday || !got.From.Equal(d(2026, time.June, 14)) {
		t.Errorf("WithWeekStart(Sunday) = %s (weekStart %s), want 2026-06-14 Sunday", got.From.Format("2006-01-02"), got.WeekStart)
	}
}

func TestWithWeekStartNoOpWhenUnchanged(t *testing.T) {
	w := NewWindow(Week, d(2026, time.June, 17), time.Monday)
	if got := w.WithWeekStart(time.Monday); got != w {
		t.Errorf("WithWeekStart(Monday) = %+v, want unchanged %+v", got, w)
	}
}

func TestWithWeekStartLeavesMonthAnchorsAlone(t *testing.T) {
	w := NewWindow(Month, d(2026, time.June, 17), time.Monday) // From = Jun 1
	got := w.WithWeekStart(time.Sunday)
	if !got.From.Equal(d(2026, time.June, 1)) || got.WeekStart != time.Sunday {
		t.Errorf("WithWeekStart on Month = %s (weekStart %s), want 2026-06-01 Sunday", got.From.Format("2006-01-02"), got.WeekStart)
	}
}

func TestWindowRange(t *testing.T) {
	w := NewWindow(Month, d(2026, time.June, 1), time.Monday).StepTo(2) // Jun..Aug
	start, end := w.Range()
	if !start.Equal(d(2026, time.June, 1)) || !end.Equal(d(2026, time.September, 1)) {
		t.Errorf("Range = [%s, %s), want [2026-06-01, 2026-09-01)", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
}

// SPDX-License-Identifier: MIT

package smartdigest

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/notify"
	"github.com/monstercameron/CashFlux/internal/smart"
)

// fixedNow is a stable reference time used across all tests so output is
// deterministic regardless of when the suite runs.
var fixedNow = time.Date(2026, 6, 24, 9, 0, 0, 0, time.UTC)

// ins builds a minimal Insight for use in tests.
func ins(key string, severity smart.Severity, title string) smart.Insight {
	return smart.Insight{
		Feature:  "SMART-B8",
		Page:     smart.PageBudgets,
		Key:      key,
		Title:    title,
		Severity: severity,
	}
}

func TestBuild_SelectionOrder(t *testing.T) {
	// Highest severity surfaces first, then stable Key tie-break.
	active := []smart.Insight{
		ins("k-info", smart.SeverityInfo, "Info note"),
		ins("k-alert", smart.SeverityAlert, "Alert thing"),
		ins("k-nudge", smart.SeverityNudge, "Nudge thing"),
		ins("k-warn", smart.SeverityWarn, "Warning thing"),
	}
	item, ok := Build(active, fixedNow, smart.CadenceWeekly, notify.NewDeliveredLog())
	if !ok {
		t.Fatal("expected ok=true, got false")
	}
	if len(item.Body) == 0 {
		t.Fatal("body is empty")
	}
	lines := bodyLines(item.Body)
	wantOrder := []string{"• Alert thing", "• Warning thing", "• Nudge thing", "• Info note"}
	for i, want := range wantOrder {
		if i >= len(lines) {
			t.Errorf("line %d missing; want %q", i, want)
			continue
		}
		if lines[i] != want {
			t.Errorf("line %d: got %q, want %q", i, lines[i], want)
		}
	}
}

func TestBuild_CapAtN(t *testing.T) {
	// More than digestTopN insights: only the top N appear in the body.
	active := make([]smart.Insight, 10)
	for i := range active {
		active[i] = ins("k"+string(rune('a'+i)), smart.SeverityInfo, "Insight "+string(rune('a'+i)))
	}
	item, ok := Build(active, fixedNow, smart.CadenceMonthly, notify.NewDeliveredLog())
	if !ok {
		t.Fatal("expected ok=true")
	}
	lines := bodyLines(item.Body)
	if len(lines) != digestTopN {
		t.Errorf("got %d lines, want %d", len(lines), digestTopN)
	}
}

func TestBuild_EmptyReturnsNotOK(t *testing.T) {
	_, ok := Build(nil, fixedNow, smart.CadenceWeekly, notify.NewDeliveredLog())
	if ok {
		t.Error("expected ok=false for nil input, got true")
	}

	_, ok = Build([]smart.Insight{}, fixedNow, smart.CadenceWeekly, notify.NewDeliveredLog())
	if ok {
		t.Error("expected ok=false for empty slice, got true")
	}
}

func TestBuild_DedupeSecondCallSamePeriod(t *testing.T) {
	active := []smart.Insight{ins("k1", smart.SeverityNudge, "A nudge")}
	log := notify.NewDeliveredLog()

	item1, ok1 := Build(active, fixedNow, smart.CadenceWeekly, log)
	if !ok1 {
		t.Fatal("first call: expected ok=true")
	}
	if item1.ID == "" {
		t.Error("first call: item.ID should be non-empty")
	}

	// Second call with the same period key — must be suppressed.
	_, ok2 := Build(active, fixedNow, smart.CadenceWeekly, log)
	if ok2 {
		t.Error("second call in same period: expected ok=false (dedupe), got true")
	}
}

func TestBuild_DedupeNextPeriodAllowed(t *testing.T) {
	active := []smart.Insight{ins("k1", smart.SeverityWarn, "A warning")}
	log := notify.NewDeliveredLog()

	_, ok1 := Build(active, fixedNow, smart.CadenceWeekly, log)
	if !ok1 {
		t.Fatal("first call: expected ok=true")
	}

	// Advance by 8 days — into the next ISO week.
	nextWeek := fixedNow.Add(8 * 24 * time.Hour)
	_, ok2 := Build(active, nextWeek, smart.CadenceWeekly, log)
	if !ok2 {
		t.Error("next-week call: expected ok=true (new period), got false")
	}
}

func TestBuild_SeverityTieBreakByKey(t *testing.T) {
	// Two insights with identical severity — stable tie-break on Key ascending.
	active := []smart.Insight{
		ins("z-last", smart.SeverityWarn, "Z warning"),
		ins("a-first", smart.SeverityWarn, "A warning"),
	}
	item, ok := Build(active, fixedNow, smart.CadenceDaily, notify.NewDeliveredLog())
	if !ok {
		t.Fatal("expected ok=true")
	}
	lines := bodyLines(item.Body)
	if len(lines) < 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "• A warning" {
		t.Errorf("tie-break: line 0 got %q, want %q", lines[0], "• A warning")
	}
	if lines[1] != "• Z warning" {
		t.Errorf("tie-break: line 1 got %q, want %q", lines[1], "• Z warning")
	}
}

func TestBuild_FeedItemIDMatchesPeriodKey(t *testing.T) {
	active := []smart.Insight{ins("k1", smart.SeverityInfo, "Info")}
	item, ok := Build(active, fixedNow, smart.CadenceMonthly, notify.NewDeliveredLog())
	if !ok {
		t.Fatal("expected ok=true")
	}
	expectedKey := PeriodKey(smart.CadenceMonthly, fixedNow)
	if item.ID != expectedKey {
		t.Errorf("item.ID %q != period key %q", item.ID, expectedKey)
	}
}

func TestBuild_TitleContainsPeriodLabel(t *testing.T) {
	active := []smart.Insight{ins("k1", smart.SeverityNudge, "Budget nudge")}
	item, ok := Build(active, fixedNow, smart.CadenceWeekly, notify.NewDeliveredLog())
	if !ok {
		t.Fatal("expected ok=true")
	}
	if item.Title == "" {
		t.Error("title is empty")
	}
	if !contains(item.Title, "weekly") {
		t.Errorf("title %q does not contain 'weekly'", item.Title)
	}
}

func TestBuild_AtTimestamp(t *testing.T) {
	active := []smart.Insight{ins("k1", smart.SeverityInfo, "Info")}
	item, ok := Build(active, fixedNow, smart.CadenceDaily, notify.NewDeliveredLog())
	if !ok {
		t.Fatal("expected ok=true")
	}
	if item.At != fixedNow.Unix() {
		t.Errorf("At: got %d, want %d", item.At, fixedNow.Unix())
	}
}

func TestBuild_PeriodKeyStable(t *testing.T) {
	// Two independent logs — same input, same time → same ID.
	active := []smart.Insight{ins("k1", smart.SeverityInfo, "An insight")}
	item1, _ := Build(active, fixedNow, smart.CadenceWeekly, notify.NewDeliveredLog())
	item2, _ := Build(active, fixedNow, smart.CadenceWeekly, notify.NewDeliveredLog())
	if item1.ID != item2.ID {
		t.Errorf("IDs differ: %q vs %q", item1.ID, item2.ID)
	}
}

func TestPeriodKey_Variants(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		cadence smart.Cadence
		want    string
	}{
		{smart.CadenceDaily, "digest:2026-06-24"},
		{smart.CadenceWeekly, "digest:2026-W26"},
		{smart.CadenceMonthly, "digest:2026-06"},
		{smart.CadenceOnOpen, "digest:2026-06-24"},   // falls back to day key
		{smart.CadenceOnChange, "digest:2026-06-24"}, // falls back to day key
	}
	for _, tt := range tests {
		got := PeriodKey(tt.cadence, now)
		if got != tt.want {
			t.Errorf("PeriodKey(%q): got %q, want %q", tt.cadence, got, tt.want)
		}
	}
}

// --- helpers -----------------------------------------------------------------

// bodyLines splits a digest body into its non-empty bullet lines.
func bodyLines(body string) []string {
	var lines []string
	start := 0
	for i := 0; i <= len(body); i++ {
		if i == len(body) || body[i] == '\n' {
			line := body[start:i]
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	return lines
}

// contains reports whether s contains sub.
func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

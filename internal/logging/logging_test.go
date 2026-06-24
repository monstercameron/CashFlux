// SPDX-License-Identifier: MIT

package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRingEvictsOldest(t *testing.T) {
	r := NewRing(3)
	for i := 1; i <= 5; i++ {
		r.Add(Entry{Message: string(rune('0' + i))})
	}
	if r.Len() != 3 {
		t.Fatalf("len = %d, want 3", r.Len())
	}
	got := r.Entries()
	want := []string{"3", "4", "5"} // oldest two evicted
	for i, e := range got {
		if e.Message != want[i] {
			t.Errorf("entry %d = %q, want %q", i, e.Message, want[i])
		}
	}
}

func TestHandlerWritesAndRecords(t *testing.T) {
	var buf bytes.Buffer
	logger, ring := New(&buf, 10, slog.LevelInfo)

	logger.Info("hello world", "user", "alice", "count", 3)

	out := buf.String()
	if !strings.Contains(out, "[INFO] hello world") {
		t.Errorf("output missing message: %q", out)
	}
	if !strings.Contains(out, "user=alice") || !strings.Contains(out, "count=3") {
		t.Errorf("output missing attrs: %q", out)
	}

	entries := ring.Entries()
	if len(entries) != 1 {
		t.Fatalf("ring len = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Message != "hello world" || e.Level != slog.LevelInfo {
		t.Errorf("entry = %+v", e)
	}
	if e.Attrs["user"] != "alice" || e.Attrs["count"] != int64(3) {
		t.Errorf("entry attrs = %+v", e.Attrs)
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger, ring := New(&buf, 10, slog.LevelInfo)

	logger.Debug("this is filtered out")
	logger.Warn("this is kept")

	if strings.Contains(buf.String(), "filtered out") {
		t.Error("debug should be filtered at info level")
	}
	if ring.Len() != 1 {
		t.Errorf("ring len = %d, want 1", ring.Len())
	}
}

func TestWithAttrsAndGroup(t *testing.T) {
	var buf bytes.Buffer
	logger, ring := New(&buf, 10, slog.LevelInfo)

	logger.With("service", "store").Info("saved")
	if !strings.Contains(buf.String(), "service=store") {
		t.Errorf("preset attr missing: %q", buf.String())
	}

	buf.Reset()
	logger.WithGroup("db").Info("query", "rows", 5)
	if !strings.Contains(buf.String(), "db.rows=5") {
		t.Errorf("grouped attr missing: %q", buf.String())
	}

	if ring.Len() != 2 {
		t.Errorf("ring len = %d, want 2", ring.Len())
	}
}

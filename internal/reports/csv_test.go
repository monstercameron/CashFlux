package reports

import (
	"strconv"
	"strings"
	"testing"
)

func TestCategoryCSV(t *testing.T) {
	rows := []CategorySpend{
		{CategoryID: "rent", Amount: 90000, Prior: 90000, DeltaPct: 0, HasDelta: true},
		{CategoryID: "food", Amount: 15000, Prior: 10000, DeltaPct: 50, HasDelta: true},
		{CategoryID: "new", Amount: 8000, Prior: 0, HasDelta: false}, // no baseline → blank change
	}
	name := func(id string) string {
		return map[string]string{"rent": "Rent", "food": "Food", "new": "New"}[id]
	}
	// Plain-decimal formatter (cents → dollars), spreadsheet-friendly.
	amount := func(v int64) string { return strconv.FormatInt(v/100, 10) }

	out := string(CategoryCSV(rows, name, amount))
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4 (header + 3): %q", len(lines), out)
	}
	if strings.TrimRight(lines[0], "\r") != "Category,Amount,Prior,Change %" {
		t.Errorf("header = %q", lines[0])
	}
	if strings.TrimRight(lines[1], "\r") != "Rent,900,900,0" {
		t.Errorf("rent row = %q", lines[1])
	}
	if strings.TrimRight(lines[2], "\r") != "Food,150,100,50" {
		t.Errorf("food row = %q", lines[2])
	}
	// No-delta row leaves the change column blank.
	if strings.TrimRight(lines[3], "\r") != "New,80,0," {
		t.Errorf("new row = %q", lines[3])
	}
}

func TestMemberCSV(t *testing.T) {
	rows := []MemberSpend{
		{MemberID: "alice", Amount: 40000},
		{MemberID: "", Amount: 5000}, // unassigned
	}
	name := func(id string) string {
		if id == "" {
			return "(unassigned)"
		}
		return map[string]string{"alice": "Alice"}[id]
	}
	amount := func(v int64) string { return strconv.FormatInt(v/100, 10) }

	out := string(MemberCSV(rows, name, amount))
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (header + 2): %q", len(lines), out)
	}
	if strings.TrimRight(lines[0], "\r") != "Member,Amount" {
		t.Errorf("header = %q", lines[0])
	}
	if strings.TrimRight(lines[1], "\r") != "Alice,400" {
		t.Errorf("alice row = %q", lines[1])
	}
	if strings.TrimRight(lines[2], "\r") != "(unassigned),50" {
		t.Errorf("unassigned row = %q", lines[2])
	}
}

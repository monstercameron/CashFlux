package split

import (
	"strconv"
	"strings"
	"testing"
)

func TestCSV(t *testing.T) {
	transfers := []Transfer{
		{From: "bob", To: "alice", Amount: 2500},
		{From: "cara", To: "alice", Amount: 1000},
	}
	name := func(id string) string {
		return map[string]string{"alice": "Alice", "bob": "Bob", "cara": "Cara"}[id]
	}
	amount := func(v int64) string { return strconv.FormatInt(v/100, 10) }

	out := string(CSV(transfers, name, amount))
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (header + 2): %q", len(lines), out)
	}
	if strings.TrimRight(lines[0], "\r") != "From,To,Amount" {
		t.Errorf("header = %q", lines[0])
	}
	if strings.TrimRight(lines[1], "\r") != "Bob,Alice,25" {
		t.Errorf("row 1 = %q", lines[1])
	}
	if strings.TrimRight(lines[2], "\r") != "Cara,Alice,10" {
		t.Errorf("row 2 = %q", lines[2])
	}
}

func TestCSVEmpty(t *testing.T) {
	out := string(CSV(nil, func(string) string { return "" }, func(int64) string { return "" }))
	lines := strings.Split(strings.TrimRight(out, "\r\n"), "\n")
	if len(lines) != 1 || strings.TrimRight(lines[0], "\r") != "From,To,Amount" {
		t.Errorf("empty CSV should be header only, got %q", out)
	}
}

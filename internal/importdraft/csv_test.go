// SPDX-License-Identifier: MIT

package importdraft

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/extract"
)

func TestCSVRecordLimits(t *testing.T) {
	for _, tt := range []struct {
		source string
		limit  int
		bad    bool
	}{
		{"", 0, false}, {"", 1, true}, {"date,desc,amount\n2026-01-01,\"a\nb\",1", 1, false},
		{"date;desc;amount\n2026-01-01;a;1\n2026-01-02;b;2", 1, true},
		{"date,desc\n\"unterminated", 10, true},
	} {
		if err := ValidateCSVLimit([]byte(tt.source), tt.limit); (err != nil) != tt.bad {
			t.Fatalf("%#v: %v", tt, err)
		}
	}
}

func TestCleanCSVSanitizesTextWithoutChangingMoneyOrRecords(t *testing.T) {
	data := []byte("date;description;amount;extra\n2026-03-01;\"Shop\nMain\u202e\";-1,25;keep\n")
	clean, err := CleanCSV(data)
	if err != nil || strings.ContainsAny(string(clean), "\u202e") || !strings.Contains(string(clean), "Shop Main;-1,25;keep") {
		t.Fatal(string(clean), err)
	}
	if _, err := CleanCSV([]byte("date,description\n\"bad")); err == nil {
		t.Fatal("invalid CSV accepted")
	}
	if _, err := CleanCSV([]byte{}); err != nil {
		t.Fatal(err)
	}
}

func TestCSVFailedRowsKeepSourceFieldsAndIdentity(t *testing.T) {
	data := "date,description,amount,category,account,currency,status\n2026-03-01,Shop,bad,Food,Checking,USD,cleared\n2026-03-02,Good,-1,Food,Checking,USD,pending"
	rows := CSVFailureRows(data, []Outcome{{"a", "failed", "amount"}, {"b", "imported", ""}}, map[string]int{"a": 2, "b": 3})
	if len(rows) != 1 || rows[0].ID != "a" || rows[0].Date != "2026-03-01" || rows[0].Amount != "bad" || rows[0].Category != "Food" || rows[0].AccountID != "Checking" || rows[0].Currency != "USD" || rows[0].Status != "cleared" {
		t.Fatal(rows)
	}
	if CSVFailureRows("", nil, nil) != nil {
		t.Fatal("empty source")
	}
	if rows := CSVFailureRows("date,payee,extra,amount\n2026-01-01,Shop,x\n2026-01-02,Good,x,1", []Outcome{{"x", "failed", ""}}, map[string]int{"x": 2}); len(rows) != 1 || rows[0].Description != "Shop" {
		t.Fatal(rows)
	}
}

func TestSourceKindsAndIDsNeverReused(t *testing.T) {
	for _, tt := range []struct{ source, kind string }{{"csv", "csv"}, {"stmt", "statement"}, {"pdf", "pdf"}, {"receipt", "image"}} {
		m := New(nil)
		m.Append(tt.source, []extract.Row{{}})
		if m.Kind() != tt.kind {
			t.Fatal(tt, m.Kind())
		}
		old := m.Rows[0].ID
		m.Remove(old)
		m.Append(tt.source, []extract.Row{{}})
		if m.Rows[0].ID == old {
			t.Fatal("reused removed ID")
		}
		m.Append("other", []extract.Row{{}})
		if m.Kind() != "mixed" {
			t.Fatal(m.Kind())
		}
	}
	m := New([]extract.Row{{Description: "x"}})
	if m.Kind() != "image" {
		t.Fatal(m.Kind())
	}
	if got := Clean("a\x00\u202eb\nc"); strings.ContainsAny(got, "\x00\u202e\n") {
		t.Fatal(got)
	}
}

func TestTransportCapabilities(t *testing.T) {
	for _, tt := range []struct {
		source, key string
		backend     bool
		want        string
	}{
		{"receipt", "", true, "proxy"}, {"receipt", "key", false, "direct"},
		{"stmt", "", true, "proxy"}, {"stmt", "key", true, "proxy"}, {"stmt", "key", false, "direct"},
		{"stmt", "", false, "unavailable"}, {"stmt", "  ", false, "unavailable"},
		{"pdf", "", true, "unavailable"}, {"pdf", "key", true, "direct"}, {"categories", "", true, "unavailable"},
	} {
		if got := Transport(tt.source, tt.key, tt.backend); got != tt.want {
			t.Fatal(tt, got)
		}
	}
}

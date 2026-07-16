// SPDX-License-Identifier: MIT

package notifyhistory

import (
	"strings"
	"testing"
)

func TestAddDedupeOrderingAndPrune(t *testing.T) {
	t.Run("newest first ordering", func(t *testing.T) {
		var a Archive
		a.Add(Record{ID: "a", Message: "first", At: 100})
		a.Add(Record{ID: "b", Message: "second", At: 300})
		a.Add(Record{ID: "c", Message: "third", At: 200})
		got := []string{a.Items[0].ID, a.Items[1].ID, a.Items[2].ID}
		want := []string{"b", "c", "a"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("order[%d] = %q, want %q (got %v)", i, got[i], want[i], got)
			}
		}
	})

	t.Run("dedupe by ID replaces in place", func(t *testing.T) {
		var a Archive
		a.Add(Record{ID: "x", Message: "old", At: 100})
		a.Add(Record{ID: "x", Message: "new", At: 150})
		if len(a.Items) != 1 {
			t.Fatalf("len = %d, want 1 (dedupe)", len(a.Items))
		}
		if a.Items[0].Message != "new" || a.Items[0].At != 150 {
			t.Fatalf("record not replaced: %+v", a.Items[0])
		}
	})

	t.Run("prune to cap keeps newest", func(t *testing.T) {
		var a Archive
		total := maxRecords + 25
		for i := 0; i < total; i++ {
			a.Add(Record{ID: string(rune('A')) + itoa(i), Message: "m", At: int64(i)})
		}
		if len(a.Items) != maxRecords {
			t.Fatalf("len = %d, want cap %d", len(a.Items), maxRecords)
		}
		// Newest (highest At) must survive; oldest must be pruned.
		if a.Items[0].At != int64(total-1) {
			t.Fatalf("newest At = %d, want %d", a.Items[0].At, total-1)
		}
		last := a.Items[len(a.Items)-1].At
		if last != int64(total-maxRecords) {
			t.Fatalf("oldest kept At = %d, want %d", last, total-maxRecords)
		}
	})
}

func TestFilter(t *testing.T) {
	seed := func() Archive {
		var a Archive
		a.Add(Record{ID: "1", Message: "Low balance on Checking", Severity: "critical", At: 300})
		a.Add(Record{ID: "2", Message: "Budget nearly spent", Severity: "warning", At: 200})
		a.Add(Record{ID: "3", Message: "New month started", Severity: "info", At: 100})
		return a
	}
	cases := []struct {
		name     string
		query    string
		severity string
		wantIDs  []string
	}{
		{"empty matches all", "", "", []string{"1", "2", "3"}},
		{"case-insensitive message", "BALANCE", "", []string{"1"}},
		{"substring match", "month", "", []string{"3"}},
		{"severity only", "", "warning", []string{"2"}},
		{"query and severity", "budget", "warning", []string{"2"}},
		{"query and severity no overlap", "balance", "warning", []string{}},
		{"whitespace query trimmed", "  new  ", "", []string{"3"}},
		{"no match", "nonexistent", "", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := seed()
			got := a.Filter(tc.query, tc.severity)
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tc.wantIDs), ids(got))
			}
			for i, id := range tc.wantIDs {
				if got[i].ID != id {
					t.Fatalf("id[%d] = %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	var a Archive
	a.Add(Record{ID: "1", Message: "hello", Severity: "info", Route: "budgets", At: 10, Read: true})
	a.Add(Record{ID: "2", Message: "world", Severity: "critical", At: 20})

	s, err := Marshal(a)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	back, err := Unmarshal(s)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(back.Items) != 2 {
		t.Fatalf("round-trip len = %d, want 2", len(back.Items))
	}
	if back.Items[0].ID != "2" || back.Items[1].ID != "1" {
		t.Fatalf("round-trip order lost: %v", ids(back.Items))
	}
	if !back.Items[1].Read || back.Items[1].Route != "budgets" {
		t.Fatalf("fields lost on round-trip: %+v", back.Items[1])
	}
}

func TestUnmarshalTolerant(t *testing.T) {
	cases := []string{"", "   ", "not json", "{", "[1,2,3]", "null"}
	for _, s := range cases {
		got, err := Unmarshal(s)
		if err != nil {
			t.Fatalf("Unmarshal(%q) returned error %v, want tolerant", s, err)
		}
		if len(got.Items) != 0 {
			t.Fatalf("Unmarshal(%q) items = %d, want empty archive", s, len(got.Items))
		}
	}
}

func TestMarkAllReadAndUnreadCount(t *testing.T) {
	var a Archive
	a.Add(Record{ID: "1", Message: "a", At: 30})
	a.Add(Record{ID: "2", Message: "b", At: 20, Read: true})
	a.Add(Record{ID: "3", Message: "c", At: 10})

	if got := a.UnreadCount(); got != 2 {
		t.Fatalf("UnreadCount = %d, want 2", got)
	}
	a.MarkAllRead()
	if got := a.UnreadCount(); got != 0 {
		t.Fatalf("UnreadCount after MarkAllRead = %d, want 0", got)
	}
	for _, it := range a.Items {
		if !it.Read {
			t.Fatalf("record %q still unread after MarkAllRead", it.ID)
		}
	}
}

// --- helpers -----------------------------------------------------------------

func ids(rs []Record) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b strings.Builder
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if neg {
		b.WriteByte('-')
	}
	for i := len(digits) - 1; i >= 0; i-- {
		b.WriteByte(digits[i])
	}
	return b.String()
}

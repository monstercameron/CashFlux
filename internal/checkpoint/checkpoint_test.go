// SPDX-License-Identifier: MIT

package checkpoint

import (
	"testing"
	"time"
)

func mk(id string) Checkpoint {
	return Checkpoint{ID: id, At: time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC), Label: "before " + id, Size: 100}
}

func ids(cs []Checkpoint) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPush(t *testing.T) {
	cases := []struct {
		name        string
		start       []string
		add         string
		max         int
		wantKept    []string
		wantDropped []string
	}{
		{"empty ring", nil, "a", 3, []string{"a"}, nil},
		{"under cap appends", []string{"a"}, "b", 3, []string{"a", "b"}, nil},
		{"at cap drops oldest", []string{"a", "b", "c"}, "d", 3, []string{"b", "c", "d"}, []string{"a"}},
		{"over cap drops several", []string{"a", "b", "c", "d"}, "e", 2, []string{"d", "e"}, []string{"a", "b", "c"}},
		{"max below one clamps", []string{"a"}, "b", 0, []string{"b"}, []string{"a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var start []Checkpoint
			for _, id := range tc.start {
				start = append(start, mk(id))
			}
			startLen := len(start)
			kept, dropped := Push(start, mk(tc.add), tc.max)
			if !eq(ids(kept), tc.wantKept) {
				t.Errorf("kept = %v, want %v", ids(kept), tc.wantKept)
			}
			if !eq(ids(dropped), tc.wantDropped) {
				t.Errorf("dropped = %v, want %v", ids(dropped), tc.wantDropped)
			}
			if len(start) != startLen {
				t.Errorf("input mutated: len %d → %d", startLen, len(start))
			}
		})
	}
}

func TestRemoveAndFind(t *testing.T) {
	idx := []Checkpoint{mk("a"), mk("b"), mk("c")}
	out, ok := Remove(idx, "b")
	if !ok || !eq(ids(out), []string{"a", "c"}) {
		t.Fatalf("Remove(b) = %v ok=%v", ids(out), ok)
	}
	if _, ok := Remove(idx, "zz"); ok {
		t.Fatal("Remove of a missing id reported found")
	}
	if len(idx) != 3 {
		t.Fatal("Remove mutated its input")
	}
	if c, ok := Find(idx, "c"); !ok || c.Label != "before c" {
		t.Fatalf("Find(c) = %+v ok=%v", c, ok)
	}
	if _, ok := Find(idx, "zz"); ok {
		t.Fatal("Find of a missing id reported found")
	}
}

func TestIndexRoundTripAndCorruption(t *testing.T) {
	idx := []Checkpoint{mk("a"), mk("b")}
	raw := EncodeIndex(idx)
	back := DecodeIndex(raw)
	if !eq(ids(back), []string{"a", "b"}) {
		t.Fatalf("round trip = %v", ids(back))
	}
	if back[0].Label != "before a" || back[0].Size != 100 || !back[0].At.Equal(idx[0].At) {
		t.Fatalf("fields lost in round trip: %+v", back[0])
	}
	if EncodeIndex(nil) != "" {
		t.Fatal("empty index should encode to empty string")
	}
	if DecodeIndex("") != nil {
		t.Fatal("empty raw should decode to nil")
	}
	if DecodeIndex("{not json]") != nil {
		t.Fatal("corrupt raw should decode to nil, not error state")
	}
}

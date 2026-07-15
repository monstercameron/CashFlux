// SPDX-License-Identifier: MIT

package agentmemory

import (
	"strings"
	"testing"
)

func TestAddTrimDedupeBlank(t *testing.T) {
	s := Store{}
	s, ok := s.Add("  paid biweekly  ")
	if !ok || s.Len() != 1 || s.Facts[0] != "paid biweekly" {
		t.Fatalf("Add should trim and store, got %+v ok=%v", s.Facts, ok)
	}
	if _, ok := s.Add("PAID BIWEEKLY"); ok {
		t.Error("case-insensitive duplicate should not be added")
	}
	if _, ok := s.Add("   "); ok {
		t.Error("blank fact should not be added")
	}
	s2, ok := s.Add("don't suggest cutting eating out")
	if !ok || s2.Len() != 2 {
		t.Fatalf("second distinct fact should be added, got %+v", s2.Facts)
	}
}

func TestRoundTrip(t *testing.T) {
	s := Store{}
	s, _ = s.Add("paid biweekly")
	s, _ = s.Add("don't suggest cutting eating out")
	raw := s.Marshal()

	got := Load(raw)
	if got.Len() != 2 {
		t.Fatalf("round-trip lost facts: %+v", got.Facts)
	}
	if got.Facts[0] != "paid biweekly" || got.Facts[1] != "don't suggest cutting eating out" {
		t.Errorf("round-trip changed order/content: %+v", got.Facts)
	}
	// Re-marshal must be stable.
	if got.Marshal() != raw {
		t.Errorf("re-marshal not stable:\n%s\n%s", raw, got.Marshal())
	}
}

func TestLoadMalformed(t *testing.T) {
	for _, raw := range []string{"", "   ", "not json", "{"} {
		if s := Load(raw); s.Len() != 0 {
			t.Errorf("Load(%q) should be empty, got %+v", raw, s.Facts)
		}
	}
}

func TestEditAndDelete(t *testing.T) {
	s := Store{}
	s, _ = s.Add("a")
	s, _ = s.Add("b")
	s, _ = s.Add("c")

	s = s.Edit(1, "  B-edited ")
	if s.Facts[1] != "B-edited" {
		t.Errorf("Edit failed: %+v", s.Facts)
	}
	s = s.Edit(99, "ignored")
	if s.Len() != 3 {
		t.Error("out-of-range edit changed length")
	}
	s = s.Delete(0)
	if s.Len() != 2 || s.Facts[0] != "B-edited" {
		t.Errorf("Delete failed: %+v", s.Facts)
	}
	s = s.Delete(-1)
	if s.Len() != 2 {
		t.Error("out-of-range delete changed length")
	}
}

func TestCap(t *testing.T) {
	s := Store{}
	for i := 0; i < MaxFacts+10; i++ {
		s, _ = s.Add(string(rune('A'+i%26)) + strings.Repeat("x", i))
	}
	if s.Len() > MaxFacts {
		t.Fatalf("store exceeded cap: %d", s.Len())
	}
}

func TestPrompt(t *testing.T) {
	if (Store{}).Prompt() != "" {
		t.Error("empty memory should render empty prompt")
	}
	s := Store{}
	s, _ = s.Add("paid biweekly")
	p := s.Prompt()
	if !strings.Contains(p, "paid biweekly") || !strings.Contains(p, "Remembered about the user") {
		t.Errorf("prompt missing content: %q", p)
	}
}

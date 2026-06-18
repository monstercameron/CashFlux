package lockquotes

import "testing"

func TestForIndexWrapsAndIsDeterministic(t *testing.T) {
	n := Count()
	if n == 0 {
		t.Fatal("expected a non-empty quote set")
	}
	// Deterministic: same index → same quote.
	if ForIndex(3) != ForIndex(3) {
		t.Error("ForIndex should be deterministic")
	}
	// Wraps: index n maps to index 0, and a big index stays in range.
	if ForIndex(n) != ForIndex(0) {
		t.Errorf("ForIndex(%d) should wrap to ForIndex(0)", n)
	}
	if ForIndex(n*7+2) != ForIndex(2) {
		t.Error("ForIndex should wrap by modulo")
	}
	// Negative indices wrap into range too (no panic, non-empty).
	if ForIndex(-1) != ForIndex(n-1) {
		t.Errorf("ForIndex(-1) should wrap to the last quote")
	}
	// Every index yields a non-empty quote.
	for i := 0; i < n; i++ {
		if ForIndex(i) == "" {
			t.Errorf("quote %d is empty", i)
		}
	}
}

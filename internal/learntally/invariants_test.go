// SPDX-License-Identifier: MIT

package learntally

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestTallySurvivesJSONRoundTrip: the tally is persisted as JSON in the
// preserved settings KV and reloaded on every boot. If the shape does not
// round-trip, everything the app learned is silently forgotten each session and
// the user just sees suggestions stop appearing.
func TestTallySurvivesJSONRoundTrip(t *testing.T) {
	orig := Tally{}
	for i := 0; i < 4; i++ {
		orig.Record("AMZN MKTP US*2H4RT9", "cat-shopping")
	}
	orig.Record("AMZN MKTP US*8K1QP2", "cat-shopping")
	orig.Record("SQ *BLUE BOTTLE", "cat-coffee")

	blob, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Tally
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The suggestion a user would actually see must be identical either side.
	for _, payee := range []string{"AMZN MKTP US*2H4RT9", "AMZN MKTP US*NEW", "SQ *BLUE BOTTLE"} {
		a, okA := orig.Suggest(payee, DefaultMinCount)
		b, okB := back.Suggest(payee, DefaultMinCount)
		if okA != okB || a != b {
			t.Errorf("Suggest(%q) differs after a round trip: %+v/%v vs %+v/%v", payee, a, okA, b, okB)
		}
	}
	if len(orig) != len(back) {
		t.Errorf("key count changed: %d -> %d", len(orig), len(back))
	}
}

// TestRecordIsMonotonic: recording can only ever add evidence. A corrected
// payee must never REDUCE the count for another, or an unrelated merchant's
// suggestion would vanish when you categorize something else.
func TestRecordIsMonotonic(t *testing.T) {
	tally := Tally{}
	tally.Record("SHELL OIL 1", "cat-gas")
	tally.Record("SHELL OIL 2", "cat-gas")
	tally.Record("SHELL OIL 3", "cat-gas")
	before, ok := tally.Suggest("SHELL OIL 4", DefaultMinCount)
	if !ok {
		t.Fatal("expected a suggestion after three corrections")
	}
	// Now teach it about a completely different merchant, many times.
	for i := 0; i < 20; i++ {
		tally.Record("NETFLIX.COM", "cat-subs")
	}
	after, ok := tally.Suggest("SHELL OIL 4", DefaultMinCount)
	if !ok {
		t.Fatal("the Shell suggestion disappeared after learning about Netflix")
	}
	if after.Count < before.Count || after.CategoryID != before.CategoryID {
		t.Errorf("unrelated learning changed the Shell suggestion: %+v -> %+v", before, after)
	}
}

// TestThresholdIsAFloorNotAWindow: once evidence clears the threshold it must
// stay cleared as more arrives.
func TestThresholdIsAFloorNotAWindow(t *testing.T) {
	tally := Tally{}
	cleared := false
	for i := 1; i <= 25; i++ {
		tally.Record("SOMEPLACE", "cat-x")
		_, ok := tally.Suggest("SOMEPLACE", DefaultMinCount)
		if ok {
			cleared = true
		} else if cleared {
			t.Fatalf("suggestion disappeared again at %d corrections", i)
		}
		if i < DefaultMinCount && ok {
			t.Fatalf("cleared the threshold early, at %d of %d", i, DefaultMinCount)
		}
	}
	if !cleared {
		t.Fatal("25 corrections never produced a suggestion")
	}
}

// TestNamespacesCannotCollide: exact and merchant keys share ONE persisted map,
// so a payee that happens to look like a namespaced key must not be able to
// forge or clobber the other tier.
func TestNamespacesCannotCollide(t *testing.T) {
	tally := Tally{}
	// A hostile-looking payee that starts with the namespace marker.
	for i := 0; i < 5; i++ {
		tally.Record("~amazon", "cat-forged")
	}
	// A genuine merchant whose stem key is "~amazon" would be Amazon.
	for i := 0; i < 5; i++ {
		tally.Record("AMZN MKTP US*1", "cat-real")
	}
	got, ok := tally.Suggest("AMZN MKTP US*NEW", DefaultMinCount)
	if !ok {
		t.Fatal("the real merchant lost its suggestion")
	}
	if got.CategoryID == "cat-forged" {
		t.Error("a payee spelled like a namespaced key hijacked the merchant tier")
	}
}

// TestSuggestNeverReportsMoreThanItHas: Count must never exceed Total, or the
// UI renders "7 of 4 times".
func TestSuggestNeverReportsMoreThanItHas(t *testing.T) {
	tally := Tally{}
	for i := 0; i < 7; i++ {
		tally.Record("PLACE", "cat-a")
	}
	for i := 0; i < 3; i++ {
		tally.Record("PLACE", "cat-b")
	}
	got, ok := tally.Suggest("PLACE", 1)
	if !ok {
		t.Fatal("expected a suggestion")
	}
	if got.Count > got.Total {
		t.Errorf("Count %d exceeds Total %d", got.Count, got.Total)
	}
	if got.Total != 10 {
		t.Errorf("Total = %d, want 10", got.Total)
	}
	if !got.Consistent() {
		t.Error("7 of 10 is a strict majority")
	}
}

// TestMerchantKeyNeverEmptyForRealDescriptors: an empty key would bucket every
// unrecognized merchant together and cross-contaminate their histories.
func TestMerchantKeyNeverEmptyForRealDescriptors(t *testing.T) {
	for _, d := range []string{
		"AMZN MKTP US*1", "SQ *BLUE BOTTLE", "POS DEBIT 4471", "ACH DEBIT ELAN",
		"x", "123", "!!!x!!!",
	} {
		if MerchantKey(d) == "" {
			t.Errorf("MerchantKey(%q) is empty", d)
		}
	}
	// Only genuinely empty input produces an empty key.
	for _, d := range []string{"", " ", "\t\n"} {
		if MerchantKey(d) != "" {
			t.Errorf("MerchantKey(%q) should be empty", d)
		}
	}
}

// TestRecordAndIncrementAgreeOnTheExactKey: quick-add still calls Increment, so
// the two writers must file under the same exact key or a merchant's history
// would be split across two buckets depending on which screen was used.
func TestRecordAndIncrementAgreeOnTheExactKey(t *testing.T) {
	viaIncrement, viaRecord := Tally{}, Tally{}
	for i := 0; i < 3; i++ {
		viaIncrement.Increment("STARBUCKS 123", "cat-coffee")
		viaRecord.Record("STARBUCKS 123", "cat-coffee")
	}
	a, okA := viaIncrement.Suggest("STARBUCKS 123", DefaultMinCount)
	b, okB := viaRecord.Suggest("STARBUCKS 123", DefaultMinCount)
	if !okA || !okB {
		t.Fatalf("both writers should clear the threshold: %v %v", okA, okB)
	}
	if a.CategoryID != b.CategoryID || a.Count != b.Count || a.Match != b.Match {
		t.Errorf("Increment and Record disagree on the exact tier: %+v vs %+v", a, b)
	}
}

// TestNormalizePayeeIsIdempotentAndTotal: it keys a persisted map, so drift
// between versions would orphan everything already learned.
func TestNormalizePayeeIsIdempotentAndTotal(t *testing.T) {
	for _, in := range []string{
		"", " ", "AMZN  MKTP   US*1", "\tTabbed\t", "MiXeD CaSe", "\x00", strings.Repeat("a b ", 100),
	} {
		once := NormalizePayee(in)
		if twice := NormalizePayee(once); twice != once {
			t.Errorf("NormalizePayee is not idempotent for %q: %q -> %q", in, once, twice)
		}
		if once != strings.TrimSpace(once) {
			t.Errorf("%q left edge whitespace: %q", in, once)
		}
		if strings.Contains(once, "  ") {
			t.Errorf("%q left a doubled space: %q", in, once)
		}
	}
}

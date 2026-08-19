// SPDX-License-Identifier: MIT

package datasetguard

import (
	"fmt"
	"strings"
	"testing"
)

// dataset builds a payload with the given populations.
func dataset(txns, accounts, categories int) []byte {
	var b strings.Builder
	b.WriteString(`{"schemaVersion":1,"transactions":[`)
	for i := 0; i < txns; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"t%d"}`, i)
	}
	b.WriteString(`],"accounts":[`)
	for i := 0; i < accounts; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"a%d"}`, i)
	}
	b.WriteString(`],"categories":[`)
	for i := 0; i < categories; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"c%d"}`, i)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

func mustInspect(t *testing.T, raw []byte) Counts {
	t.Helper()
	c, ok := Inspect(raw)
	if !ok {
		t.Fatalf("Inspect could not read the payload")
	}
	return c
}

// The incident this package exists for: 528 transactions replaced by 432 in a
// single write, which every layer accepted without comment.
func TestTheAugust19ClobberIsRefused(t *testing.T) {
	prev := mustInspect(t, dataset(528, 15, 52))
	next := mustInspect(t, dataset(432, 15, 10))

	v := Check(prev, next, DefaultPolicy)
	if !v.Destructive {
		t.Fatal("the clobber that started this would still be accepted")
	}
	// It is caught on CATEGORIES (52 -> 10, 81%), not transactions (528 -> 432,
	// 18%). That is the point of judging every collection: the transaction loss
	// alone is inside any tolerance a real user needs, and judging it alone would
	// have let this through.
	if v.Collection != "categories" {
		t.Fatalf("Collection = %q, want categories — the worst loss should decide", v.Collection)
	}
	if v.Lost != 42 {
		t.Fatalf("Lost = %d, want 42", v.Lost)
	}
	for _, want := range []string{"42", "52", "categories"} {
		if !strings.Contains(v.Reason, want) {
			t.Fatalf("reason should name the numbers, got %q", v.Reason)
		}
	}
}

// The second clobber in the same incident — 565 down to 528 — is a 6.5% loss,
// under the threshold. Pinned so the tradeoff is explicit rather than assumed:
// this guard is a backstop against catastrophe, and the account-binding fix is
// what stops the smaller overwrites.
func TestASmallLossIsAllowedThrough(t *testing.T) {
	prev := mustInspect(t, dataset(565, 15, 58))
	next := mustInspect(t, dataset(528, 15, 52))
	if v := Check(prev, next, DefaultPolicy); v.Destructive {
		t.Fatalf("a 6.5%% loss should pass the backstop: %s", v.Reason)
	}
}

func TestCheckVerdicts(t *testing.T) {
	for _, tc := range []struct {
		name        string
		prev, next  Counts
		destructive bool
	}{
		{"adding records is never destructive", Counts{Transactions: 500}, Counts{Transactions: 900}, false},
		{"an unchanged count is not destructive", Counts{Transactions: 500}, Counts{Transactions: 500}, false},
		{"a loss inside the threshold passes", Counts{Transactions: 500}, Counts{Transactions: 400}, false},
		{"a loss past the threshold is refused", Counts{Transactions: 500}, Counts{Transactions: 300}, true},
		{"wiping everything is refused", Counts{Transactions: 500}, Counts{}, true},
		{"a small household is left alone", Counts{Transactions: 8}, Counts{Transactions: 1}, false},
		{"an empty prior copy never blocks a first write", Counts{}, Counts{Transactions: 900}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Check(tc.prev, tc.next, DefaultPolicy).Destructive; got != tc.destructive {
				t.Fatalf("Destructive = %v, want %v", got, tc.destructive)
			}
		})
	}
}

// Exactly at the threshold passes; one record past it does not. Boundaries are
// where a policy either means something or does not.
func TestThresholdBoundary(t *testing.T) {
	prev := Counts{Transactions: 100}
	if v := Check(prev, Counts{Transactions: 75}, DefaultPolicy); v.Destructive {
		t.Fatalf("exactly 25%% should pass: %s", v.Reason)
	}
	if v := Check(prev, Counts{Transactions: 74}, DefaultPolicy); !v.Destructive {
		t.Fatal("26% should be refused")
	}
}

// An encrypted dataset is unreadable BY DESIGN, and must be reported as such
// rather than counted as empty — counting it as empty would make every App Lock
// user's sync look like a total wipe.
func TestInspectReportsUnreadablePayloads(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{"encrypted envelope", append([]byte("\x00cf1\x00"), []byte(`{"v":1,"alg":"AES-GCM-PBKDF2"}`)...)},
		{"empty", nil},
		{"not json", []byte("just some text")},
		{"a json array", []byte(`[1,2,3]`)},
		{"truncated json", []byte(`{"transactions":[{"id":`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if c, ok := Inspect(tc.raw); ok {
				t.Fatalf("Inspect claimed to read it: %+v", c)
			}
		})
	}
}

func TestInspectCounts(t *testing.T) {
	c := mustInspect(t, dataset(565, 15, 58))
	if c.Transactions != 565 || c.Accounts != 15 || c.Categories != 58 {
		t.Fatalf("counts = %+v", c)
	}
	if c.Total() != 565+15+58 {
		t.Fatalf("Total = %d", c.Total())
	}
}

// Leading whitespace is legal JSON and must not be mistaken for a binary
// prefix — a pretty-printed payload is still a payload.
func TestInspectToleratesLeadingWhitespace(t *testing.T) {
	raw := append([]byte("\n\t  "), dataset(30, 2, 3)...)
	c, ok := Inspect(raw)
	if !ok || c.Transactions != 30 {
		t.Fatalf("ok=%v counts=%+v", ok, c)
	}
}

// Fields this build has never seen must not break the count — the guard has to
// keep working against a payload written by a newer client.
func TestInspectIgnoresUnknownFields(t *testing.T) {
	raw := []byte(`{"schemaVersion":99,"transactions":[{"id":"t1"},{"id":"t2"}],
	  "somethingFromTheFuture":[{"a":1},{"a":2},{"a":3}],"accounts":[{"id":"a1"}]}`)
	c, ok := Inspect(raw)
	if !ok || c.Transactions != 2 || c.Accounts != 1 {
		t.Fatalf("ok=%v counts=%+v", ok, c)
	}
}

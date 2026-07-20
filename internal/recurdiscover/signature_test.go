// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"math/rand"
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestSignatureQuarantine(t *testing.T) {
	tests := []struct {
		name  string
		payee string
		want  string
	}{
		{"plain word", "Spotify", "SPOTIFY"},
		{"hash rotation A", "SPOTIFY P1A2B3", "SPOTIFY #"},
		{"hash rotation B", "SPOTIFY K9X2M1", "SPOTIFY #"},
		{"long digit run", "COMCAST 084213771", "COMCAST #"},
		{"short digit token kept", "STORE 0842", "STORE 0842"},
		{"two words survive", "UBER EATS", "UBER EATS"},
		{"vowelless run quarantined", "PAYPAL XKCDQZ", "PAYPAL #"},
		{"empty", "   ", ""},
		{"punctuation stripped", "SQ *BLUEBOTTLE.", "SQ BLUEBOTTLE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Signature(tt.payee); got != tt.want {
				t.Errorf("Signature(%q) = %q, want %q", tt.payee, got, tt.want)
			}
		})
	}
}

// TestClusterHashRotationCollapses confirms rotating reference ids collapse into
// a single cluster with a canonical quarantined signature.
func TestClusterHashRotationCollapses(t *testing.T) {
	txns := []Txn{
		{ID: "a", Date: d(2026, 1, 9), Payee: "SPOTIFY P1A2B3", AmountMinor: 1099, Direction: Out},
		{ID: "b", Date: d(2026, 2, 9), Payee: "SPOTIFY K9X2M1", AmountMinor: 1099, Direction: Out},
		{ID: "c", Date: d(2026, 3, 9), Payee: "SPOTIFY Z0Q8W2", AmountMinor: 1099, Direction: Out},
	}
	got := Cluster(txns, Pins{})
	if len(got) != 1 {
		t.Fatalf("want 1 cluster, got %d: %+v", len(got), got)
	}
	if got[0].Canonical != "SPOTIFY #" {
		t.Errorf("canonical = %q, want %q", got[0].Canonical, "SPOTIFY #")
	}
	if len(got[0].Txns) != 3 {
		t.Errorf("want 3 txns in cluster, got %d", len(got[0].Txns))
	}
}

// TestClusterShortStringGuard confirms short brand tokens join only by exact
// match — "UBER" never fuzzes into "UBER EATS".
func TestClusterShortStringGuard(t *testing.T) {
	txns := []Txn{
		{ID: "u1", Date: d(2026, 1, 1), Payee: "UBER", AmountMinor: 1800, Direction: Out},
		{ID: "u2", Date: d(2026, 1, 8), Payee: "UBER", AmountMinor: 2200, Direction: Out},
		{ID: "e1", Date: d(2026, 1, 3), Payee: "UBER EATS", AmountMinor: 3400, Direction: Out},
		{ID: "e2", Date: d(2026, 1, 10), Payee: "UBER EATS", AmountMinor: 2900, Direction: Out},
	}
	got := Cluster(txns, Pins{})
	if len(got) != 2 {
		t.Fatalf("want 2 clusters (UBER vs UBER EATS), got %d: %+v", len(got), sigs(got))
	}
}

// TestClusterFuzzyJoin confirms a near-identical longer signature joins at ≥0.90.
func TestClusterFuzzyJoin(t *testing.T) {
	txns := []Txn{
		{ID: "1", Date: d(2026, 1, 1), Payee: "NETFLIX SUBSCRIPTION", AmountMinor: 1599, Direction: Out},
		{ID: "2", Date: d(2026, 2, 1), Payee: "NETFLIX SUBSCRIPTON", AmountMinor: 1599, Direction: Out}, // typo
	}
	got := Cluster(txns, Pins{})
	if len(got) != 1 {
		t.Fatalf("want 1 fuzzy-joined cluster, got %d: %+v", len(got), sigs(got))
	}
}

// TestClusterHardKeys confirms account and direction never merge across.
func TestClusterHardKeys(t *testing.T) {
	txns := []Txn{
		{ID: "1", Date: d(2026, 1, 1), Payee: "ACME", AmountMinor: 1000, AccountID: "chk", Direction: Out},
		{ID: "2", Date: d(2026, 2, 1), Payee: "ACME", AmountMinor: 1000, AccountID: "sav", Direction: Out},
		{ID: "3", Date: d(2026, 3, 1), Payee: "ACME", AmountMinor: 1000, AccountID: "chk", Direction: In},
	}
	got := Cluster(txns, Pins{})
	if len(got) != 3 {
		t.Fatalf("want 3 clusters across hard keys, got %d", len(got))
	}
}

// TestClusterPins covers never-merge and force-merge overrides.
func TestClusterPins(t *testing.T) {
	// Never-merge: two fuzzy-close signatures forced apart.
	never := Cluster([]Txn{
		{ID: "1", Date: d(2026, 1, 1), Payee: "NETFLIX SUBSCRIPTION", AmountMinor: 1599, Direction: Out},
		{ID: "2", Date: d(2026, 2, 1), Payee: "NETFLIX SUBSCRIPTON", AmountMinor: 1599, Direction: Out},
	}, Pins{NeverMerge: [][2]string{{"NETFLIX SUBSCRIPTION", "NETFLIX SUBSCRIPTON"}}})
	if len(never) != 2 {
		t.Errorf("never-merge: want 2 clusters, got %d", len(never))
	}

	// Force-merge: two unrelated signatures joined by pin.
	force := Cluster([]Txn{
		{ID: "1", Date: d(2026, 1, 1), Payee: "GYM DOWNTOWN", AmountMinor: 5000, Direction: Out},
		{ID: "2", Date: d(2026, 2, 1), Payee: "FITNESS CLUB", AmountMinor: 5000, Direction: Out},
	}, Pins{ForceMerge: [][2]string{{"GYM DOWNTOWN", "FITNESS CLUB"}}})
	if len(force) != 1 {
		t.Errorf("force-merge: want 1 cluster, got %d: %+v", len(force), sigs(force))
	}
}

// TestClusterDeterministicOrder confirms shuffled insertion order yields the same
// clusters after the internal chronological sort.
func TestClusterDeterministicOrder(t *testing.T) {
	base := []Txn{
		{ID: "a", Date: d(2026, 1, 9), Payee: "SPOTIFY AA11BB", AmountMinor: 1099, Direction: Out},
		{ID: "b", Date: d(2026, 2, 9), Payee: "SPOTIFY CC22DD", AmountMinor: 1099, Direction: Out},
		{ID: "c", Date: d(2026, 1, 15), Payee: "COMCAST 084213771", AmountMinor: 8000, Direction: Out},
		{ID: "e", Date: d(2026, 2, 15), Payee: "COMCAST 084299900", AmountMinor: 8000, Direction: Out},
		{ID: "f", Date: d(2026, 1, 20), Payee: "SALARY", AmountMinor: 300000, Direction: In},
	}
	want := canonSet(Cluster(base, Pins{}))
	rng := rand.New(rand.NewSource(42))
	for iter := 0; iter < 8; iter++ {
		shuf := append([]Txn(nil), base...)
		rng.Shuffle(len(shuf), func(i, j int) { shuf[i], shuf[j] = shuf[j], shuf[i] })
		got := canonSet(Cluster(shuf, Pins{}))
		if len(got) != len(want) {
			t.Fatalf("iter %d: cluster count %d != %d", iter, len(got), len(want))
		}
		for k, n := range want {
			if got[k] != n {
				t.Errorf("iter %d: canonical %q member count %d, want %d", iter, k, got[k], n)
			}
		}
	}
}

func sigs(cs []SignatureCluster) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Canonical
	}
	return out
}

// canonSet maps canonical signature → member count for order-independent compare.
func canonSet(cs []SignatureCluster) map[string]int {
	m := map[string]int{}
	for _, c := range cs {
		m[c.Canonical+"|"+c.Direction.String()] = len(c.Txns)
	}
	return m
}

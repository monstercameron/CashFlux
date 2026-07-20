// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"math/rand"
	"testing"
	"time"
)

// monthlyTxns builds n monthly outflow txns from start at a fixed amount with a
// rotating hash suffix, so signature quarantine is exercised end to end.
func monthlyTxns(prefix, payeeBase string, start time.Time, n int, amount int64, dir Direction) []Txn {
	out := make([]Txn, 0, n)
	cur := start
	for i := 0; i < n; i++ {
		out = append(out, Txn{
			ID:          prefix + itoa(i),
			Date:        cur,
			Payee:       payeeBase + " " + hashSuffix(i),
			AmountMinor: amount,
			Direction:   dir,
			Currency:    "USD",
		})
		cur = cur.AddDate(0, 1, 0)
	}
	return out
}

func hashSuffix(i int) string {
	tags := []string{"P1A2B3", "K9X2M1", "Z0Q8W2", "R3T7Y4", "M2N6V8", "A1S9D3", "F4G7H2", "J5K8L1", "Q3W6E9"}
	return tags[i%len(tags)]
}

// TestDiscoverLikelyCandidate: a clean monthly fixed charge becomes a Likely
// candidate with full evidence.
func TestDiscoverLikelyCandidate(t *testing.T) {
	txns := monthlyTxns("s", "SPOTIFY", d(2025, 11, 9), 9, 1099, Out)
	now := d(2026, 7, 11)
	res := Discover(txns, nil, Pins{}, Options{Now: now})
	if len(res.Candidates) != 1 {
		t.Fatalf("want 1 candidate, got %d: %+v", len(res.Candidates), res.Candidates)
	}
	c := res.Candidates[0]
	if c.Tier != TierLikely {
		t.Errorf("tier = %v (conf %.2f), want Likely", c.Tier, c.Confidence)
	}
	if c.Signature != "SPOTIFY #" {
		t.Errorf("signature = %q, want %q", c.Signature, "SPOTIFY #")
	}
	if c.Evidence.Count != 9 || c.Evidence.Cadence != CadenceMonthly {
		t.Errorf("evidence = %d×%v, want 9×monthly", c.Evidence.Count, c.Evidence.Cadence)
	}
	if c.Evidence.Amount.Kind != AmountFixed || c.Evidence.Amount.Typical != 1099 {
		t.Errorf("amount = %v/%d, want fixed/1099", c.Evidence.Amount.Kind, c.Evidence.Amount.Typical)
	}
	if len(c.Evidence.TxnIDs) != 9 {
		t.Errorf("evidence carries %d txn refs, want 9", len(c.Evidence.TxnIDs))
	}
}

// TestDiscoverSteppedCreep: a mid-history price change stays ONE candidate and
// carries the creep signal.
func TestDiscoverSteppedCreep(t *testing.T) {
	var txns []Txn
	cur := d(2025, 10, 9)
	amts := []int64{1310, 1310, 1310, 1440, 1440, 1440}
	for i, a := range amts {
		txns = append(txns, Txn{ID: itoa(i), Date: cur, Payee: "NEWSPLUS DIGITAL", AmountMinor: a, Direction: Out, Currency: "USD"})
		cur = cur.AddDate(0, 1, 0)
	}
	res := Discover(txns, nil, Pins{}, Options{Now: cur})
	if len(res.Candidates) != 1 {
		t.Fatalf("stepped price change must be ONE candidate, got %d", len(res.Candidates))
	}
	step := res.Candidates[0].Evidence.Amount.Step
	if step == nil || step.FromMinor != 1310 || step.ToMinor != 1440 {
		t.Errorf("creep signal = %+v, want 1310→1440", step)
	}
}

// TestDiscoverTwoClusterSplit: one signature with two interleaved price levels
// becomes two candidates.
func TestDiscoverTwoClusterSplit(t *testing.T) {
	var txns []Txn
	cur := d(2025, 12, 3)
	for i := 0; i < 5; i++ {
		txns = append(txns,
			Txn{ID: "big" + itoa(i), Date: cur, Payee: "PATREON MEMBERSHIP", AmountMinor: 2500, Direction: Out, Currency: "USD"},
			Txn{ID: "sml" + itoa(i), Date: cur, Payee: "PATREON MEMBERSHIP", AmountMinor: 500, Direction: Out, Currency: "USD"},
		)
		cur = cur.AddDate(0, 1, 0)
	}
	res := Discover(txns, nil, Pins{}, Options{Now: cur})
	if len(res.Candidates) != 2 {
		t.Fatalf("two-level same-signature cluster must yield 2 candidates, got %d", len(res.Candidates))
	}
	got := map[int64]bool{}
	for _, c := range res.Candidates {
		got[c.Evidence.Amount.Typical] = true
	}
	if !got[2500] || !got[500] {
		t.Errorf("candidate amounts = %v, want both 2500 and 500", got)
	}
}

// TestDiscoverVenmoNoise: random amounts on an irregular rhythm yield NOTHING.
func TestDiscoverVenmoNoise(t *testing.T) {
	dates := []time.Time{d(2026, 1, 3), d(2026, 1, 19), d(2026, 2, 2), d(2026, 3, 11), d(2026, 3, 28), d(2026, 5, 4)}
	amts := []int64{2500, 800, 15000, 300, 4200, 9100}
	var txns []Txn
	for i := range dates {
		txns = append(txns, Txn{ID: itoa(i), Date: dates[i], Payee: "VENMO PAYMENT " + hashSuffix(i), AmountMinor: amts[i], Direction: Out, Currency: "USD"})
	}
	res := Discover(txns, nil, Pins{}, Options{Now: d(2026, 5, 5)})
	if len(res.Candidates) != 0 {
		t.Errorf("venmo noise must produce no candidate, got %d: %+v", len(res.Candidates), res.Candidates)
	}
}

// TestDiscoverPaycheck: a stable large inbound flow is flagged as income.
func TestDiscoverPaycheck(t *testing.T) {
	txns := stepTxns("pay", "ACME PAYROLL", d(2026, 1, 2), 8, 14, 250000, In)
	res := Discover(txns, nil, Pins{}, Options{Now: d(2026, 4, 20)})
	if len(res.Candidates) != 1 {
		t.Fatalf("want 1 income candidate, got %d", len(res.Candidates))
	}
	c := res.Candidates[0]
	if !c.IsIncome || c.Direction != In {
		t.Errorf("candidate IsIncome=%v dir=%v, want income/in", c.IsIncome, c.Direction)
	}
	if c.Evidence.Cadence != CadenceBiweekly {
		t.Errorf("paycheck cadence = %v, want biweekly", c.Evidence.Cadence)
	}
}

// TestDiscoverDedupeToCycles: a cluster matching an existing commitment is
// reported as that commitment's cycles, not a new candidate.
func TestDiscoverDedupeToCycles(t *testing.T) {
	txns := monthlyTxns("n", "NETFLIX", d(2025, 11, 15), 8, 1599, Out)
	existing := []Commitment{{ID: "rec-netflix", Payee: "Netflix", Direction: Out}}
	res := Discover(txns, existing, Pins{}, Options{Now: d(2026, 6, 16)})
	if len(res.Candidates) != 0 {
		t.Errorf("matched cluster must not be a candidate, got %d", len(res.Candidates))
	}
	if len(res.CycleMatches) != 1 {
		t.Fatalf("want 1 cycle match, got %d", len(res.CycleMatches))
	}
	cm := res.CycleMatches[0]
	if cm.CommitmentID != "rec-netflix" {
		t.Errorf("cycle commitment = %q, want rec-netflix", cm.CommitmentID)
	}
	if len(cm.TxnIDs) != 8 {
		t.Errorf("cycle carries %d txns, want 8", len(cm.TxnIDs))
	}
}

// TestDiscoverSuppressed: a suppressed signature yields no candidate.
func TestDiscoverSuppressed(t *testing.T) {
	txns := monthlyTxns("s", "SPOTIFY", d(2025, 11, 9), 9, 1099, Out)
	pins := Pins{Suppressed: map[string]bool{"SPOTIFY #": true}}
	res := Discover(txns, nil, pins, Options{Now: d(2026, 7, 11)})
	if len(res.Candidates) != 0 {
		t.Errorf("suppressed signature must yield no candidate, got %d", len(res.Candidates))
	}
}

// TestDiscoverAnnualReducedFloor: an annual bill with only 2 occurrences is still
// proposed (floor 2) but not as Likely.
func TestDiscoverAnnualReducedFloor(t *testing.T) {
	txns := []Txn{
		{ID: "a", Date: d(2025, 3, 12), Payee: "STATE FARM INSURANCE", AmountMinor: 84000, Direction: Out, Currency: "USD"},
		{ID: "b", Date: d(2026, 3, 12), Payee: "STATE FARM INSURANCE", AmountMinor: 84000, Direction: Out, Currency: "USD"},
	}
	res := Discover(txns, nil, Pins{}, Options{Now: d(2026, 4, 1)})
	if len(res.Candidates) != 1 {
		t.Fatalf("annual with 2 occurrences should propose 1 candidate, got %d", len(res.Candidates))
	}
	if res.Candidates[0].Evidence.Cadence != CadenceAnnual {
		t.Errorf("cadence = %v, want annual", res.Candidates[0].Evidence.Cadence)
	}
	if res.Candidates[0].Tier == TierLikely {
		t.Errorf("2-occurrence annual should not be Likely")
	}
}

// TestDiscoverDeterministic: shuffling insertion order yields identical results.
func TestDiscoverDeterministic(t *testing.T) {
	var base []Txn
	base = append(base, monthlyTxns("sp", "SPOTIFY", d(2025, 11, 9), 9, 1099, Out)...)
	base = append(base, monthlyTxns("nf", "NETFLIX", d(2025, 10, 15), 9, 1599, Out)...)
	base = append(base, stepTxns("pay", "ACME PAYROLL", d(2026, 1, 2), 8, 14, 250000, In)...)
	now := d(2026, 7, 20)

	want := Discover(base, nil, Pins{}, Options{Now: now})
	rng := rand.New(rand.NewSource(7))
	for iter := 0; iter < 6; iter++ {
		shuf := append([]Txn(nil), base...)
		rng.Shuffle(len(shuf), func(i, j int) { shuf[i], shuf[j] = shuf[j], shuf[i] })
		got := Discover(shuf, nil, Pins{}, Options{Now: now})
		if len(got.Candidates) != len(want.Candidates) {
			t.Fatalf("iter %d: %d candidates != %d", iter, len(got.Candidates), len(want.Candidates))
		}
		for i := range want.Candidates {
			if got.Candidates[i].Signature != want.Candidates[i].Signature ||
				got.Candidates[i].Tier != want.Candidates[i].Tier ||
				got.Candidates[i].Evidence.Amount.Typical != want.Candidates[i].Evidence.Amount.Typical {
				t.Errorf("iter %d idx %d: %+v != %+v", iter, i, got.Candidates[i], want.Candidates[i])
			}
		}
	}
}

// stepTxns builds n txns stepping by stepDays with a rotating hash suffix.
func stepTxns(prefix, payeeBase string, start time.Time, n, stepDays int, amount int64, dir Direction) []Txn {
	out := make([]Txn, 0, n)
	cur := start
	for i := 0; i < n; i++ {
		out = append(out, Txn{
			ID:          prefix + itoa(i),
			Date:        cur,
			Payee:       payeeBase + " " + hashSuffix(i),
			AmountMinor: amount,
			Direction:   dir,
			Currency:    "USD",
		})
		cur = cur.AddDate(0, 0, stepDays)
	}
	return out
}

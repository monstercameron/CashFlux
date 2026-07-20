// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// acctMonthlyTxns builds n monthly charges for a payee on the given account.
func acctMonthlyTxns(payee, account string, amountMinor int64, n int) []Txn {
	var out []Txn
	start := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		out = append(out, Txn{
			ID:          payee + "-" + time.Month(i+1).String(),
			Date:        start.AddDate(0, i, 0),
			Payee:       payee,
			AmountMinor: amountMinor,
			AccountID:   account,
			Direction:   Out,
			Currency:    "USD",
		})
	}
	return out
}

// TestDedupeBySettledSignature is the core noise fix: a household names its
// mortgage flow "Mortgage payment" while the bank posts "MERIDIAN DATA", so
// matching on the display name alone kept re-proposing an already-tracked
// obligation. Declaring the signatures the commitment actually pays fixes it.
func TestDedupeBySettledSignature(t *testing.T) {
	txns := acctMonthlyTxns("MERIDIAN DATA", "acct-joint", 148000, 8)
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	// Without the extra signal the label does not match, so it is proposed.
	bare := []Commitment{{ID: "rec-mortgage", Payee: "Mortgage payment", AccountID: "acct-joint", Direction: Out}}
	if got := Discover(txns, bare, Pins{}, Options{Now: now}); len(got.Candidates) != 1 {
		t.Fatalf("baseline: got %d candidates, want 1 (name alone cannot match)", len(got.Candidates))
	}

	// Declaring what it actually settles dedupes it into cycles.
	informed := []Commitment{{
		ID: "rec-mortgage", Payee: "Mortgage payment", AccountID: "acct-joint", Direction: Out,
		Signatures: []string{Signature("MERIDIAN DATA")},
	}}
	got := Discover(txns, informed, Pins{}, Options{Now: now})
	if len(got.Candidates) != 0 {
		t.Errorf("got %d candidates, want 0 — an already-tracked commitment must not resurface", len(got.Candidates))
	}
	if len(got.CycleMatches) != 1 || got.CycleMatches[0].CommitmentID != "rec-mortgage" {
		t.Errorf("got cycles %+v, want one for rec-mortgage", got.CycleMatches)
	}
}

// TestDedupeByFingerprint covers the case where nothing has been linked yet: the
// same account, cadence, and amount is the same obligation.
func TestDedupeByFingerprint(t *testing.T) {
	txns := acctMonthlyTxns("SAFEHARBOR INSURANCE", "acct-joint", 15333, 6)
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		comm  Commitment
		want  int // candidates
		cycle bool
	}{
		{
			name: "same account, cadence and amount dedupes",
			comm: Commitment{ID: "rec-carins", Payee: "Car insurance", AccountID: "acct-joint", Direction: Out,
				AmountMinor: 15333, Cadence: CadenceMonthly},
			want: 0, cycle: true,
		},
		{
			name: "within the 5% tolerance still dedupes",
			comm: Commitment{ID: "rec-carins", Payee: "Car insurance", AccountID: "acct-joint", Direction: Out,
				AmountMinor: 15000, Cadence: CadenceMonthly},
			want: 0, cycle: true,
		},
		{
			name: "a different amount is a different obligation",
			comm: Commitment{ID: "rec-carins", Payee: "Car insurance", AccountID: "acct-joint", Direction: Out,
				AmountMinor: 50000, Cadence: CadenceMonthly},
			want: 1,
		},
		{
			name: "a different cadence is a different obligation",
			comm: Commitment{ID: "rec-carins", Payee: "Car insurance", AccountID: "acct-joint", Direction: Out,
				AmountMinor: 15333, Cadence: CadenceAnnual},
			want: 1,
		},
		{
			name: "a different account never fingerprint-matches",
			comm: Commitment{ID: "rec-carins", Payee: "Car insurance", AccountID: "acct-other", Direction: Out,
				AmountMinor: 15333, Cadence: CadenceMonthly},
			want: 1,
		},
		{
			name: "no fingerprint declared leaves behaviour unchanged",
			comm: Commitment{ID: "rec-carins", Payee: "Car insurance", AccountID: "acct-joint", Direction: Out},
			want: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Discover(txns, []Commitment{tc.comm}, Pins{}, Options{Now: now})
			if len(got.Candidates) != tc.want {
				t.Errorf("got %d candidates, want %d", len(got.Candidates), tc.want)
			}
			if tc.cycle && len(got.CycleMatches) != 1 {
				t.Errorf("got %d cycle matches, want 1", len(got.CycleMatches))
			}
		})
	}
}

// TestIncomeFingerprintToleratesPayVariance covers the looser inbound tolerance:
// a household's one paycheck must not keep resurfacing as a second income just
// because net pay swings with hours and withholding.
func TestIncomeFingerprintToleratesPayVariance(t *testing.T) {
	txns := acctMonthlyTxns("MERIDIAN DATA", "acct-joint", 440000, 6)
	for i := range txns {
		txns[i].Direction = In
	}
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	// Declared net pay is 4700; deposits land at 4400 — 6.4% off, well outside the
	// 5% outflow band but inside the 15% income band.
	comm := Commitment{ID: "rec-salary", Payee: "Paycheck (net)", AccountID: "acct-joint", Direction: In,
		AmountMinor: 470000, Cadence: CadenceMonthly}
	got := Discover(txns, []Commitment{comm}, Pins{}, Options{Now: now})
	if len(got.Candidates) != 0 {
		t.Errorf("got %d candidates, want 0 — the tracked paycheck must absorb its own deposits", len(got.Candidates))
	}
	if len(got.CycleMatches) != 1 {
		t.Errorf("got %d cycle matches, want 1", len(got.CycleMatches))
	}

	// An outflow at the same relative distance is still a different obligation.
	// (Payee deliberately unrelated to the label, so only the fingerprint is in
	// play — a matching name would dedupe on identity before amount is consulted.)
	outTxns := acctMonthlyTxns("NORTHGATE STORAGE", "acct-joint", 440000, 6)
	outComm := Commitment{ID: "rec-bill", Payee: "Gym membership", AccountID: "acct-joint", Direction: Out,
		AmountMinor: 470000, Cadence: CadenceMonthly}
	if got := Discover(outTxns, []Commitment{outComm}, Pins{}, Options{Now: now}); len(got.Candidates) != 1 {
		t.Errorf("outflow: got %d candidates, want 1 (6%% is outside the 5%% band)", len(got.Candidates))
	}
}

// TestFingerprintNeverCrossesDirection guards the hard key: an inbound deposit
// must never dedupe into an outbound commitment even with a matching amount.
func TestFingerprintNeverCrossesDirection(t *testing.T) {
	txns := acctMonthlyTxns("ACME PAYROLL", "acct-joint", 470000, 6)
	for i := range txns {
		txns[i].Direction = In
	}
	comm := Commitment{ID: "rec-rent", Payee: "Rent", AccountID: "acct-joint", Direction: Out,
		AmountMinor: 470000, Cadence: CadenceMonthly}
	got := Discover(txns, []Commitment{comm}, Pins{}, Options{Now: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)})
	if len(got.CycleMatches) != 0 {
		t.Errorf("an inbound cluster must never match an outbound commitment: %+v", got.CycleMatches)
	}
}

// TestFromDomainCadence covers the persisted→discovery cadence mapping used to
// build the dedupe fingerprint.
func TestFromDomainCadence(t *testing.T) {
	cases := map[domain.RecurringCadence]Cadence{
		domain.CadenceWeekly:      CadenceWeekly,
		domain.CadenceBiweekly:    CadenceBiweekly,
		domain.CadenceSemimonthly: CadenceSemimonthly,
		domain.CadenceMonthly:     CadenceMonthly,
		domain.CadenceQuarterly:   CadenceQuarterly,
		domain.CadenceYearly:      CadenceAnnual,
		domain.CadenceDaily:       CadenceUnknown,
	}
	for in, want := range cases {
		if got := FromDomainCadence(in); got != want {
			t.Errorf("FromDomainCadence(%v) = %v, want %v", in, got, want)
		}
	}
}

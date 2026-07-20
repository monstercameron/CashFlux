// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"testing"
	"time"
)

func parseDay(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestDetectStopped(t *testing.T) {
	tests := []struct {
		name        string
		cadence     Cadence
		lastSeen    string
		now         string
		grace       int
		wantStopped bool
		wantMissed  int
	}{
		{
			name:     "monthly still live",
			cadence:  CadenceMonthly,
			lastSeen: "2026-06-09", now: "2026-07-05", grace: 5,
			wantStopped: false,
		},
		{
			name:     "monthly missed two",
			cadence:  CadenceMonthly,
			lastSeen: "2026-03-09", now: "2026-06-01", grace: 5,
			wantStopped: true, wantMissed: 2,
		},
		{
			name:     "weekly missed several",
			cadence:  CadenceWeekly,
			lastSeen: "2026-05-01", now: "2026-06-05", grace: 2,
			wantStopped: true, wantMissed: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig, stopped := DetectStopped("c1", tt.cadence, parseDay(tt.lastSeen), parseDay(tt.now), tt.grace)
			if stopped != tt.wantStopped {
				t.Fatalf("stopped = %v, want %v (missed %d)", stopped, tt.wantStopped, sig.MissedCount)
			}
			if stopped && sig.MissedCount != tt.wantMissed {
				t.Errorf("missed = %d, want %d", sig.MissedCount, tt.wantMissed)
			}
		})
	}
}

// TestVerifyConfirms: a claim matching the actual pattern is verified locally.
func TestVerifyConfirms(t *testing.T) {
	txns := monthlyTxns("s", "SPOTIFY", d(2025, 11, 9), 9, 1099, Out)
	claim := Claim{
		Signatures:  []string{"SPOTIFY #"},
		Direction:   Out,
		Cadence:     CadenceMonthly,
		AmountMinor: 1099,
		BandMinor:   50,
	}
	v := Verify(claim, txns, Options{Now: d(2026, 7, 11)})
	if !v.Verified {
		t.Fatalf("expected verified, got: %s", v.Reason)
	}
	if v.Evidence.Count != 9 {
		t.Errorf("evidence count = %d, want 9", v.Evidence.Count)
	}
}

// TestVerifyRejectsWrongCadence: a claim that misstates the cadence is honestly
// not verified.
func TestVerifyRejectsWrongCadence(t *testing.T) {
	txns := monthlyTxns("s", "SPOTIFY", d(2025, 11, 9), 9, 1099, Out)
	claim := Claim{
		Signatures:  []string{"SPOTIFY #"},
		Direction:   Out,
		Cadence:     CadenceWeekly, // wrong
		AmountMinor: 1099,
		BandMinor:   50,
	}
	v := Verify(claim, txns, Options{Now: d(2026, 7, 11)})
	if v.Verified {
		t.Errorf("weekly claim over monthly data should not verify")
	}
	if v.Reason == "" {
		t.Errorf("expected an honest reason")
	}
}

// TestVerifyInsufficientData: too few matches gives an honest unconfirmed result.
func TestVerifyInsufficientData(t *testing.T) {
	txns := monthlyTxns("s", "SPOTIFY", d(2026, 6, 9), 1, 1099, Out)
	claim := Claim{Signatures: []string{"SPOTIFY #"}, Direction: Out, Cadence: CadenceMonthly, AmountMinor: 1099, BandMinor: 50}
	v := Verify(claim, txns, Options{Now: d(2026, 7, 11)})
	if v.Verified {
		t.Errorf("a single transaction should not verify")
	}
}

// SPDX-License-Identifier: MIT

package goalinterest

import "testing"

func TestProject(t *testing.T) {
	tests := []struct {
		name                    string
		current, monthly, targ  int64
		apy                     float64
		wantReached             bool
		wantMonths              int
		wantInterestNonNegative bool
		wantInterestZero        bool
	}{
		{
			name: "zero apy degrades to linear ceil", current: 0, monthly: 100_00, targ: 1000_00,
			apy: 0, wantReached: true, wantMonths: 10, wantInterestZero: true,
		},
		{
			name: "zero apy with remainder rounds up", current: 0, monthly: 300_00, targ: 1000_00,
			apy: 0, wantReached: true, wantMonths: 4, wantInterestZero: true, // ceil(1000/300)=4
		},
		{
			name: "already complete", current: 1000_00, monthly: 100_00, targ: 1000_00,
			apy: 4.4, wantReached: true, wantMonths: 0, wantInterestZero: true,
		},
		{
			name: "interest reduces months vs linear", current: 0, monthly: 200_00, targ: 10000_00,
			apy: 4.4, wantReached: true, wantInterestNonNegative: true,
			// linear would be ceil(10000/200)=50; with interest it must be <= 50.
		},
		{
			name: "no contribution no yield unreachable", current: 100_00, monthly: 0, targ: 1000_00,
			apy: 0, wantReached: false,
		},
		{
			name: "yield alone can reach target", current: 9900_00, monthly: 0, targ: 10000_00,
			apy: 5.0, wantReached: true, wantInterestNonNegative: true,
		},
		{
			name: "non-positive target already reached", current: 0, monthly: 100_00, targ: 0,
			apy: 4.4, wantReached: true, wantMonths: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Project(tc.current, tc.monthly, tc.targ, tc.apy)
			if got.Reached != tc.wantReached {
				t.Fatalf("Reached = %v, want %v (%+v)", got.Reached, tc.wantReached, got)
			}
			if !tc.wantReached {
				return
			}
			if tc.wantMonths != 0 && got.Months != tc.wantMonths {
				t.Errorf("Months = %d, want %d", got.Months, tc.wantMonths)
			}
			if tc.wantInterestZero && got.InterestMinor != 0 {
				t.Errorf("InterestMinor = %d, want 0", got.InterestMinor)
			}
			if tc.wantInterestNonNegative && got.InterestMinor < 0 {
				t.Errorf("InterestMinor = %d, want >= 0", got.InterestMinor)
			}
			// Invariant: Final = Contributed + Interest.
			if got.FinalMinor != got.ContributedMinor+got.InterestMinor {
				t.Errorf("Final %d != Contributed %d + Interest %d", got.FinalMinor, got.ContributedMinor, got.InterestMinor)
			}
		})
	}
}

// TestInterestBeatsLinear checks that a positive APY never needs MORE months than
// the zero-APY linear projection for the same inputs.
func TestInterestBeatsLinear(t *testing.T) {
	current, monthly, targ := int64(0), int64(200_00), int64(10000_00)
	linear := Project(current, monthly, targ, 0)
	withAPY := Project(current, monthly, targ, 4.4)
	if !linear.Reached || !withAPY.Reached {
		t.Fatal("both projections should reach")
	}
	if withAPY.Months > linear.Months {
		t.Errorf("APY months %d > linear months %d", withAPY.Months, linear.Months)
	}
	if withAPY.InterestMinor <= 0 {
		t.Errorf("expected positive interest contribution, got %d", withAPY.InterestMinor)
	}
}

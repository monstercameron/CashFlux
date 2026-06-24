// SPDX-License-Identifier: MIT

package payoff

import "testing"

// TestProjectNegativeAPR covers the interest-floor branch: a negative APR would
// imply negative monthly interest, which the model floors at 0, so it behaves
// like 0% — $1000 at -12% APR, $100/month → 10 months, no interest.
func TestProjectNegativeAPR(t *testing.T) {
	r, ok := Project(100000, -12, 10000)
	if !ok {
		t.Fatal("expected a viable payoff at negative APR")
	}
	if r.Months != 10 || r.TotalInterest != 0 || r.TotalPaid != 100000 {
		t.Errorf("got months=%d interest=%d paid=%d, want 10/0/100000", r.Months, r.TotalInterest, r.TotalPaid)
	}
}

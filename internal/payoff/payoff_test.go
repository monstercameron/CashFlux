package payoff

import "testing"

func TestProjectZeroInterest(t *testing.T) {
	// $1000 at 0% APR, $100/month → 10 months, no interest.
	r, ok := Project(100000, 0, 10000)
	if !ok {
		t.Fatal("expected viable payoff")
	}
	if r.Months != 10 {
		t.Errorf("months = %d, want 10", r.Months)
	}
	if r.TotalInterest != 0 {
		t.Errorf("interest = %d, want 0", r.TotalInterest)
	}
	if r.TotalPaid != 100000 {
		t.Errorf("total paid = %d, want 100000", r.TotalPaid)
	}
}

func TestProjectWithInterestPaysOff(t *testing.T) {
	// $1000 at 12% APR (1%/month), $100/month. First month interest = $10, so the
	// payment comfortably covers it and the debt clears in a finite time with some
	// interest accrued.
	r, ok := Project(100000, 12, 10000)
	if !ok {
		t.Fatal("expected viable payoff")
	}
	if r.Months < 10 || r.Months > 12 {
		t.Errorf("months = %d, want ~11", r.Months)
	}
	if r.TotalInterest <= 0 {
		t.Errorf("interest = %d, want > 0", r.TotalInterest)
	}
	if r.TotalPaid != 100000+r.TotalInterest {
		t.Errorf("total paid = %d, want principal + interest", r.TotalPaid)
	}
}

func TestProjectPaymentTooSmall(t *testing.T) {
	// $1000 at 24% APR → ~$20/month interest; a $10 payment never covers it.
	if _, ok := Project(100000, 24, 1000); ok {
		t.Error("expected ok=false when payment cannot cover interest")
	}
}

func TestProjectAlreadyPaid(t *testing.T) {
	if r, ok := Project(0, 20, 100); !ok || r.Months != 0 {
		t.Errorf("zero balance should be already paid (ok, 0 months), got ok=%v months=%d", ok, r.Months)
	}
}

func TestProjectNonPositivePayment(t *testing.T) {
	if _, ok := Project(1000, 0, 0); ok {
		t.Error("zero payment on a positive balance should be non-viable")
	}
}

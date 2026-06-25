// SPDX-License-Identifier: MIT

package payoff

import (
	"testing"
	"time"
)

// startDate is a stable reference date used across all amortization tests.
var startDate = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

// TestAmortizeFixed_NilOnZeroTerm verifies the documented nil-return contract.
func TestAmortizeFixed_NilOnZeroTerm(t *testing.T) {
	if got := AmortizeFixed(100_00, 6.0, 0, startDate); got != nil {
		t.Errorf("AmortizeFixed with termMonths=0: want nil, got %v rows", len(got))
	}
	if got := AmortizeFixed(100_00, 6.0, -1, startDate); got != nil {
		t.Errorf("AmortizeFixed with termMonths=-1: want nil, got %v rows", len(got))
	}
}

// TestAmortizeFixed_Month1Interest verifies that the first row's interest equals
// round(balance * monthlyRate), exactly matching the spec formula.
func TestAmortizeFixed_Month1Interest(t *testing.T) {
	tests := []struct {
		name          string
		balanceMinor  int64
		aprPct        float64
		termMonths    int
		wantInterest1 int64
	}{
		{
			name:          "10000 USD at 6% APR over 60 months",
			balanceMinor:  10_000_00, // $10,000.00
			aprPct:        6.0,
			termMonths:    60,
			wantInterest1: 5000, // round(1000000 * 0.005) = 5000 cents = $50.00
		},
		{
			name:          "5000 USD at 12% APR over 36 months",
			balanceMinor:  5_000_00, // $5,000.00
			aprPct:        12.0,
			termMonths:    36,
			wantInterest1: 5000, // round(500000 * 0.01) = 5000 cents = $50.00
		},
		{
			name:          "1000 USD at 19.99% APR over 24 months",
			balanceMinor:  1_000_00, // $1,000.00
			aprPct:        19.99,
			termMonths:    24,
			wantInterest1: 1666, // round(100000 * (19.99/1200)) = round(1665.8) = 1666 cents
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := AmortizeFixed(tc.balanceMinor, tc.aprPct, tc.termMonths, startDate)
			if len(rows) == 0 {
				t.Fatal("expected rows, got none")
			}
			got := rows[0].InterestMinor
			if got != tc.wantInterest1 {
				t.Errorf("month-1 interest: got %d, want %d", got, tc.wantInterest1)
			}
		})
	}
}

// TestAmortizeFixed_FinalBalanceZero verifies that the last row in the schedule
// always has BalanceMinor == 0 (the final-payment clamp).
func TestAmortizeFixed_FinalBalanceZero(t *testing.T) {
	cases := []struct {
		balance int64
		apr     float64
		term    int
	}{
		{100_00, 6.0, 12},
		{10_000_00, 4.5, 360},
		{5_000_00, 18.0, 24},
		{1_000_00, 0.01, 12}, // very low APR
		{999_99, 7.5, 48},    // odd principal
	}
	for _, c := range cases {
		rows := AmortizeFixed(c.balance, c.apr, c.term, startDate)
		if len(rows) == 0 {
			t.Errorf("AmortizeFixed(%d, %.2f, %d): returned no rows", c.balance, c.apr, c.term)
			continue
		}
		last := rows[len(rows)-1]
		if last.BalanceMinor != 0 {
			t.Errorf("AmortizeFixed(%d, %.2f, %d): final balance = %d, want 0",
				c.balance, c.apr, c.term, last.BalanceMinor)
		}
	}
}

// TestAmortizeFixed_ZeroAPR verifies that a 0% loan produces equal principal
// payments (within 1 minor unit for rounding) summing exactly to the original
// balance, with zero total interest.
func TestAmortizeFixed_ZeroAPR(t *testing.T) {
	const balance int64 = 12_000_00 // $12,000.00
	const term = 12
	rows := AmortizeFixed(balance, 0.0, term, startDate)

	if len(rows) != term {
		t.Fatalf("expected %d rows, got %d", term, len(rows))
	}

	var totalPrincipal, totalInterest int64
	for _, r := range rows {
		totalPrincipal += r.PrincipalMinor
		totalInterest += r.InterestMinor
	}

	if totalPrincipal != balance {
		t.Errorf("totalPrincipal = %d, want %d", totalPrincipal, balance)
	}
	if totalInterest != 0 {
		t.Errorf("totalInterest = %d, want 0 for 0%% APR", totalInterest)
	}

	// Each principal payment should be uniform (ceiling-divided).
	expected := (balance + int64(term) - 1) / int64(term)
	for i, r := range rows {
		if r.PrincipalMinor != expected && r.PrincipalMinor != expected-1 {
			t.Errorf("row %d principal = %d, want %d (±1)", i+1, r.PrincipalMinor, expected)
		}
	}

	if rows[len(rows)-1].BalanceMinor != 0 {
		t.Errorf("final balance = %d, want 0", rows[len(rows)-1].BalanceMinor)
	}
}

// TestAmortizeFixed_PositiveAPRAccruesInterest verifies that total interest > 0
// for any positive APR.
func TestAmortizeFixed_PositiveAPRAccruesInterest(t *testing.T) {
	cases := []struct {
		balance int64
		apr     float64
		term    int
	}{
		{10_000_00, 5.0, 60},
		{500_00, 24.0, 12},
		{1_000_00, 0.1, 24},
	}
	for _, c := range cases {
		rows := AmortizeFixed(c.balance, c.apr, c.term, startDate)
		totalInterest, _, _ := AmortSummary(rows)
		if totalInterest <= 0 {
			t.Errorf("AmortizeFixed(%d, %.2f, %d): totalInterest = %d, want > 0",
				c.balance, c.apr, c.term, totalInterest)
		}
	}
}

// TestAmortizeFixed_DateProgression verifies that each row's date advances by
// one calendar month.
func TestAmortizeFixed_DateProgression(t *testing.T) {
	rows := AmortizeFixed(6_000_00, 6.0, 6, startDate)
	if len(rows) != 6 {
		t.Fatalf("expected 6 rows, got %d", len(rows))
	}
	for i, r := range rows {
		want := startDate.AddDate(0, i, 0)
		if !r.Date.Equal(want) {
			t.Errorf("row %d date = %v, want %v", i+1, r.Date, want)
		}
	}
}

// TestAmortizeFixed_RowInvariant verifies for every row that
// PaymentMinor == PrincipalMinor + InterestMinor.
func TestAmortizeFixed_RowInvariant(t *testing.T) {
	rows := AmortizeFixed(10_000_00, 7.5, 36, startDate)
	for _, r := range rows {
		if r.PaymentMinor != r.PrincipalMinor+r.InterestMinor {
			t.Errorf("row %d: PaymentMinor(%d) != PrincipalMinor(%d) + InterestMinor(%d)",
				r.PaymentNo, r.PaymentMinor, r.PrincipalMinor, r.InterestMinor)
		}
	}
}

// TestAmortizeWithExtra_NilOnZeroTerm mirrors the AmortizeFixed nil contract.
func TestAmortizeWithExtra_NilOnZeroTerm(t *testing.T) {
	if got := AmortizeWithExtra(100_00, 6.0, 0, 10_00, startDate); got != nil {
		t.Errorf("AmortizeWithExtra with termMonths=0: want nil, got %v rows", len(got))
	}
}

// TestAmortizeWithExtra_PaysOffFasterWithLessInterest verifies the core
// extra-payment guarantee: for the same loan, adding extra per month results in
// fewer months AND less total interest compared to AmortizeFixed.
func TestAmortizeWithExtra_PaysOffFasterWithLessInterest(t *testing.T) {
	cases := []struct {
		name    string
		balance int64
		apr     float64
		term    int
		extra   int64
	}{
		{"small extra", 10_000_00, 6.0, 60, 50_00},
		{"large extra", 5_000_00, 12.0, 36, 200_00},
		{"modest extra high APR", 8_000_00, 18.0, 48, 100_00},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixed := AmortizeFixed(tc.balance, tc.apr, tc.term, startDate)
			withExtra := AmortizeWithExtra(tc.balance, tc.apr, tc.term, tc.extra, startDate)

			fixedInterest, _, _ := AmortSummary(fixed)
			extraInterest, _, _ := AmortSummary(withExtra)

			if len(withExtra) >= len(fixed) {
				t.Errorf("extra payment did not shorten term: withExtra=%d months, fixed=%d months",
					len(withExtra), len(fixed))
			}
			if extraInterest >= fixedInterest {
				t.Errorf("extra payment did not reduce interest: withExtra=%d, fixed=%d",
					extraInterest, fixedInterest)
			}

			// Final balance must still be 0.
			if len(withExtra) > 0 && withExtra[len(withExtra)-1].BalanceMinor != 0 {
				t.Errorf("withExtra final balance = %d, want 0",
					withExtra[len(withExtra)-1].BalanceMinor)
			}
		})
	}
}

// TestAmortizeWithExtra_FinalBalanceZero mirrors the standard zero-balance check.
func TestAmortizeWithExtra_FinalBalanceZero(t *testing.T) {
	rows := AmortizeWithExtra(10_000_00, 5.0, 60, 100_00, startDate)
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}
	if last := rows[len(rows)-1].BalanceMinor; last != 0 {
		t.Errorf("final balance = %d, want 0", last)
	}
}

// TestAmortSummary verifies that AmortSummary correctly sums interest and
// payments, and returns the date of the last row.
func TestAmortSummary(t *testing.T) {
	rows := AmortizeFixed(12_000_00, 6.0, 12, startDate)
	if len(rows) == 0 {
		t.Fatal("expected rows")
	}

	gotInterest, gotPaid, gotDate := AmortSummary(rows)

	var wantInterest, wantPaid int64
	for _, r := range rows {
		wantInterest += r.InterestMinor
		wantPaid += r.PaymentMinor
	}
	wantDate := rows[len(rows)-1].Date

	if gotInterest != wantInterest {
		t.Errorf("totalInterest: got %d, want %d", gotInterest, wantInterest)
	}
	if gotPaid != wantPaid {
		t.Errorf("totalPaid: got %d, want %d", gotPaid, wantPaid)
	}
	if !gotDate.Equal(wantDate) {
		t.Errorf("payoffDate: got %v, want %v", gotDate, wantDate)
	}
}

// TestAmortSummary_Empty verifies that AmortSummary handles a nil/empty slice.
func TestAmortSummary_Empty(t *testing.T) {
	interest, paid, date := AmortSummary(nil)
	if interest != 0 || paid != 0 || !date.IsZero() {
		t.Errorf("AmortSummary(nil): got (%d, %d, %v), want (0, 0, zero)", interest, paid, date)
	}
}

// TestAmortSummary_TotalPaidEqualsInterestPlusPrincipal checks that totalPaid
// reported by AmortSummary equals the original balance plus total interest.
func TestAmortSummary_TotalPaidEqualsInterestPlusPrincipal(t *testing.T) {
	const balance int64 = 5_000_00
	rows := AmortizeFixed(balance, 9.0, 24, startDate)
	totalInterest, totalPaid, _ := AmortSummary(rows)

	// Total paid = principal repaid (= original balance) + all interest.
	// (Principal repaid must equal the original balance since final balance is 0.)
	var totalPrincipal int64
	for _, r := range rows {
		totalPrincipal += r.PrincipalMinor
	}
	if totalPrincipal != balance {
		t.Errorf("totalPrincipal = %d, want %d", totalPrincipal, balance)
	}
	if totalPaid != balance+totalInterest {
		t.Errorf("totalPaid(%d) != balance(%d) + totalInterest(%d)", totalPaid, balance, totalInterest)
	}
}

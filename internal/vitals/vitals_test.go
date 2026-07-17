// SPDX-License-Identifier: MIT

package vitals

import "testing"

// full household mirrors a realistic position: steady income, an essential
// month, liquid cash, a mortgage + a loan + a clear card.
func fullInputs() Inputs {
	return Inputs{
		IncomeMonthlyMinor:    894500, // $8,945.00
		ExpenseMonthlyMinor:   320386, // $3,203.86
		MonthsAveraged:        6,
		EssentialMonthlyMinor: 490686, // $4,906.86
		LiquidMinor:           625000, // $6,250.00
		Debts: []Debt{
			{Name: "Mortgage", BalanceMinor: 15000000, AprPercent: 3.0, MinPaymentMinor: 180000, IsMortgage: true, InPayoff: false},
			{Name: "Car loan", BalanceMinor: 4495300, AprPercent: 2.9, MinPaymentMinor: 50300, InPayoff: true},
			{Name: "Visa", BalanceMinor: 0, AprPercent: 24.99, MinPaymentMinor: 0, InPayoff: true},
		},
		Cards: Cards{HasCards: true, BalanceMinor: 0, LimitMinor: 3080000},
	}
}

func TestEvaluateCashFlow(t *testing.T) {
	r := Evaluate(fullInputs())
	if !r.HasIncome {
		t.Fatal("HasIncome = false, want true")
	}
	if r.SurplusMonthlyMinor != 574114 {
		t.Errorf("SurplusMonthlyMinor = %d, want 574114", r.SurplusMonthlyMinor)
	}
	if r.SurplusAnnualMinor != 574114*12 {
		t.Errorf("SurplusAnnualMinor = %d, want %d", r.SurplusAnnualMinor, 574114*12)
	}
	if r.SurplusTone != ToneUp {
		t.Errorf("SurplusTone = %q, want up", r.SurplusTone)
	}
	if r.SavingsRatePct != 64 {
		t.Errorf("SavingsRatePct = %d, want 64", r.SavingsRatePct)
	}
	if r.SavingsTone != ToneUp {
		t.Errorf("SavingsTone = %q, want up", r.SavingsTone)
	}
	// discretionary = surplus − minimums (180000+50300); 38% of income → up
	if want := int64(574114 - 230300); r.DiscretionaryMinor != want {
		t.Errorf("DiscretionaryMinor = %d, want %d", r.DiscretionaryMinor, want)
	}
	if r.DiscretionaryTone != ToneUp {
		t.Errorf("DiscretionaryTone = %q, want up", r.DiscretionaryTone)
	}
}

func TestEvaluateDiscretionaryTightIsWarn(t *testing.T) {
	// Positive but thin: $150 free on $7,000 income (~2%) is a tight month.
	r := Evaluate(Inputs{
		IncomeMonthlyMinor:  700000,
		ExpenseMonthlyMinor: 373000,
		Debts:               []Debt{{Name: "Loan", BalanceMinor: 100000, MinPaymentMinor: 312000, InPayoff: true}},
	})
	if r.DiscretionaryMinor != 15000 {
		t.Fatalf("DiscretionaryMinor = %d, want 15000", r.DiscretionaryMinor)
	}
	if r.DiscretionaryTone != ToneWarn {
		t.Errorf("DiscretionaryTone = %q, want warn for a thin positive buffer", r.DiscretionaryTone)
	}
	// Negative stays down.
	r2 := Evaluate(Inputs{IncomeMonthlyMinor: 100000, ExpenseMonthlyMinor: 150000})
	if r2.DiscretionaryTone != ToneDown {
		t.Errorf("DiscretionaryTone = %q, want down when negative", r2.DiscretionaryTone)
	}
}

func TestEvaluateDebts(t *testing.T) {
	r := Evaluate(fullInputs())
	if !r.HasDebts {
		t.Fatal("HasDebts = false, want true")
	}
	if want := int64(15000000 + 4495300); r.TotalDebtMinor != want {
		t.Errorf("TotalDebtMinor = %d, want %d", r.TotalDebtMinor, want)
	}
	if r.ExMortgageMinor != 4495300 {
		t.Errorf("ExMortgageMinor = %d, want 4495300", r.ExMortgageMinor)
	}
	if !r.HasMortgage {
		t.Error("HasMortgage = false, want true")
	}
	if r.MinPaymentsMinor != 230300 {
		t.Errorf("MinPaymentsMinor = %d, want 230300", r.MinPaymentsMinor)
	}
	if r.AnnualDebtServiceMinor != 230300*12 {
		t.Errorf("AnnualDebtServiceMinor = %d, want %d", r.AnnualDebtServiceMinor, 230300*12)
	}
	// minimums-only DTI: 230300*100/894500 = 25
	if r.PaymentShareOfIncomePct != 25 {
		t.Errorf("PaymentShareOfIncomePct = %d, want 25", r.PaymentShareOfIncomePct)
	}
	if r.PaymentShareTone != ToneUp {
		t.Errorf("PaymentShareTone = %q, want up", r.PaymentShareTone)
	}
	// weighted APR: (15000000×3.0 + 4495300×2.9) / 19495300 ≈ 2.977
	if r.WeightedAprPercent < 2.97 || r.WeightedAprPercent > 2.99 {
		t.Errorf("WeightedAprPercent = %.3f, want ≈2.977", r.WeightedAprPercent)
	}
	if r.WeightedAprTone != ToneUp {
		t.Errorf("WeightedAprTone = %q, want up", r.WeightedAprTone)
	}
	if r.InterestDragMonthlyMinor <= 0 {
		t.Errorf("InterestDragMonthlyMinor = %d, want > 0", r.InterestDragMonthlyMinor)
	}
	// payoff: only the car loan is in-payoff with a balance; it clears.
	if !r.PayoffApplicable || r.PayoffNeverClears {
		t.Fatalf("payoff applicable=%v neverClears=%v, want applicable and clearing", r.PayoffApplicable, r.PayoffNeverClears)
	}
	if r.PayoffMonths <= 0 {
		t.Errorf("PayoffMonths = %d, want > 0", r.PayoffMonths)
	}
}

func TestEvaluateCushion(t *testing.T) {
	r := Evaluate(fullInputs())
	if !r.HasCushion {
		t.Fatal("HasCushion = false, want true")
	}
	// 625000×10/490686 = 12 → 1.2 months
	if r.CoverageMonthsTenths != 12 {
		t.Errorf("CoverageMonthsTenths = %d, want 12", r.CoverageMonthsTenths)
	}
	if r.CoverageTone != ToneDown {
		t.Errorf("CoverageTone = %q, want down", r.CoverageTone)
	}
	if r.FundMonths != 6 {
		t.Errorf("FundMonths = %d, want default 6", r.FundMonths)
	}
	if want := int64(490686 * 6); r.FundTargetMinor != want {
		t.Errorf("FundTargetMinor = %d, want %d", r.FundTargetMinor, want)
	}
	if want := int64(490686*6 - 625000); r.FundGapMinor != want {
		t.Errorf("FundGapMinor = %d, want %d", r.FundGapMinor, want)
	}
	// runway after debt: 625000×10/(320386+230300) = 11 → 1.1 months
	if r.RunwayAfterDebtTenths != 11 {
		t.Errorf("RunwayAfterDebtTenths = %d, want 11", r.RunwayAfterDebtTenths)
	}
	if r.RunwayTone != ToneDown {
		t.Errorf("RunwayTone = %q, want down", r.RunwayTone)
	}
}

func TestEvaluateCards(t *testing.T) {
	r := Evaluate(fullInputs())
	if !r.HasCards || !r.HasUtilization {
		t.Fatalf("HasCards=%v HasUtilization=%v, want both true", r.HasCards, r.HasUtilization)
	}
	if r.UtilizationPct != 0 {
		t.Errorf("UtilizationPct = %d, want 0", r.UtilizationPct)
	}
	if r.UtilizationTone != ToneUp {
		t.Errorf("UtilizationTone = %q, want up", r.UtilizationTone)
	}
	if r.CardAvailableMinor != 3080000 {
		t.Errorf("CardAvailableMinor = %d, want 3080000", r.CardAvailableMinor)
	}
}

func TestEvaluateNoIncome(t *testing.T) {
	in := fullInputs()
	in.IncomeMonthlyMinor = 0
	r := Evaluate(in)
	if r.HasIncome {
		t.Error("HasIncome = true, want false")
	}
	if r.SavingsRatePct != 0 || r.SavingsTone != ToneNone {
		t.Errorf("savings = %d/%q, want 0 and no tone", r.SavingsRatePct, r.SavingsTone)
	}
	if r.PaymentShareTone != ToneNone {
		t.Errorf("PaymentShareTone = %q, want none without income", r.PaymentShareTone)
	}
	if r.SurplusTone != ToneDown {
		t.Errorf("SurplusTone = %q, want down (spending with no income)", r.SurplusTone)
	}
}

func TestEvaluateDeficitClampsRate(t *testing.T) {
	r := Evaluate(Inputs{IncomeMonthlyMinor: 1000, ExpenseMonthlyMinor: 500000})
	if r.SavingsRatePct != -100 {
		t.Errorf("SavingsRatePct = %d, want clamp to -100", r.SavingsRatePct)
	}
	if r.SavingsTone != ToneDown || r.SurplusTone != ToneDown {
		t.Errorf("tones = %q/%q, want down/down", r.SavingsTone, r.SurplusTone)
	}
}

func TestEvaluateNoDebtsNoCards(t *testing.T) {
	r := Evaluate(Inputs{IncomeMonthlyMinor: 100000, ExpenseMonthlyMinor: 60000, EssentialMonthlyMinor: 50000, LiquidMinor: 300000})
	if r.HasDebts || r.PayoffApplicable || r.HasMortgage {
		t.Errorf("debt flags = %v/%v/%v, want all false", r.HasDebts, r.PayoffApplicable, r.HasMortgage)
	}
	if r.HasCards || r.HasUtilization {
		t.Errorf("card flags = %v/%v, want both false", r.HasCards, r.HasUtilization)
	}
	if r.WeightedAprTone != ToneNone {
		t.Errorf("WeightedAprTone = %q, want none", r.WeightedAprTone)
	}
	// discretionary = surplus with no minimums
	if r.DiscretionaryMinor != 40000 {
		t.Errorf("DiscretionaryMinor = %d, want 40000", r.DiscretionaryMinor)
	}
	// coverage 300000×10/50000 = 60 tenths = 6.0 months → up
	if r.CoverageMonthsTenths != 60 || r.CoverageTone != ToneUp {
		t.Errorf("coverage = %d/%q, want 60/up", r.CoverageMonthsTenths, r.CoverageTone)
	}
	if r.FundGapMinor != 0 {
		t.Errorf("FundGapMinor = %d, want 0 (exactly met)", r.FundGapMinor)
	}
}

func TestEvaluatePayoffNeverClears(t *testing.T) {
	r := Evaluate(Inputs{
		IncomeMonthlyMinor: 100000,
		Debts: []Debt{
			{Name: "Toxic", BalanceMinor: 1000000, AprPercent: 60, MinPaymentMinor: 100, InPayoff: true},
		},
	})
	if !r.PayoffApplicable {
		t.Fatal("PayoffApplicable = false, want true")
	}
	if !r.PayoffNeverClears || r.PayoffTone != ToneDown {
		t.Errorf("neverClears=%v tone=%q, want true/down", r.PayoffNeverClears, r.PayoffTone)
	}
}

func TestEvaluateMortgageExcludedFromPayoff(t *testing.T) {
	r := Evaluate(Inputs{
		IncomeMonthlyMinor: 100000,
		Debts: []Debt{
			{Name: "Mortgage", BalanceMinor: 10000000, AprPercent: 3, MinPaymentMinor: 150000, IsMortgage: true, InPayoff: false},
		},
	})
	if r.PayoffApplicable {
		t.Error("PayoffApplicable = true, want false (mortgage-only household)")
	}
	if r.ExMortgageMinor != 0 {
		t.Errorf("ExMortgageMinor = %d, want 0", r.ExMortgageMinor)
	}
}

func TestEvaluateNoBurnRunwaySentinel(t *testing.T) {
	r := Evaluate(Inputs{IncomeMonthlyMinor: 100000, LiquidMinor: 500000})
	if r.RunwayAfterDebtTenths != -1 {
		t.Errorf("RunwayAfterDebtTenths = %d, want -1 sentinel with zero burn", r.RunwayAfterDebtTenths)
	}
	if r.RunwayTone != ToneNone {
		t.Errorf("RunwayTone = %q, want none", r.RunwayTone)
	}
	if r.HasCushion {
		t.Error("HasCushion = true, want false without an essential month")
	}
}

func TestEvaluateUtilizationBands(t *testing.T) {
	cases := []struct {
		name    string
		balance int64
		want    int
		tone    Tone
	}{
		{"clean", 0, 0, ToneUp},
		{"at target", 30000, 30, ToneUp},
		{"elevated", 45000, 45, ToneWarn},
		{"high", 90000, 90, ToneDown},
		{"over limit clamps", 200000, 100, ToneDown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Evaluate(Inputs{Cards: Cards{HasCards: true, BalanceMinor: tc.balance, LimitMinor: 100000}})
			if r.UtilizationPct != tc.want || r.UtilizationTone != tc.tone {
				t.Errorf("utilization = %d/%q, want %d/%q", r.UtilizationPct, r.UtilizationTone, tc.want, tc.tone)
			}
		})
	}
}

func TestEvaluateCardNoLimit(t *testing.T) {
	r := Evaluate(Inputs{Cards: Cards{HasCards: true, BalanceMinor: 50000}})
	if !r.HasCards || r.HasUtilization {
		t.Errorf("HasCards=%v HasUtilization=%v, want true/false without a limit", r.HasCards, r.HasUtilization)
	}
}

func TestEvaluateFundMonthsFallback(t *testing.T) {
	for _, bad := range []int{-1, 0, 25, 100} {
		r := Evaluate(Inputs{EssentialMonthlyMinor: 1000, FundMonths: bad})
		if r.FundMonths != 6 {
			t.Errorf("FundMonths(%d) = %d, want fallback 6", bad, r.FundMonths)
		}
	}
	r := Evaluate(Inputs{EssentialMonthlyMinor: 1000, FundMonths: 3})
	if r.FundMonths != 3 || r.FundTargetMinor != 3000 {
		t.Errorf("FundMonths=3 → %d/%d, want 3/3000", r.FundMonths, r.FundTargetMinor)
	}
}

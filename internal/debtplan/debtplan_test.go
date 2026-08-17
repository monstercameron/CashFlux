// SPDX-License-Identifier: MIT

package debtplan

import (
	"testing"
	"time"
)

var start = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

func TestBiweeklySavesTimeAndInterest(t *testing.T) {
	// A 30-year $300,000 mortgage at 6%.
	b, ok := SimulateBiweekly(30_000_000, 6, 360, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	if b.BiweeklyMonths >= b.MonthlyMonths {
		t.Errorf("biweekly took %d months against monthly's %d — it must be shorter",
			b.BiweeklyMonths, b.MonthlyMonths)
	}
	if b.InterestSavedMinor <= 0 {
		t.Errorf("interest saved = %d, want a positive saving", b.InterestSavedMinor)
	}
	if b.MonthsSaved != b.MonthlyMonths-b.BiweeklyMonths {
		t.Errorf("months saved = %d, want %d", b.MonthsSaved, b.MonthlyMonths-b.BiweeklyMonths)
	}
}

func TestTheExtraPaymentIsReportedBecauseItIsTheMechanism(t *testing.T) {
	// The whole effect is a thirteenth payment a year. Somebody who cannot afford
	// one cannot afford this plan, and hiding the cost makes it look free.
	b, ok := SimulateBiweekly(30_000_000, 6, 360, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	if b.ExtraPerYearMinor <= 0 {
		t.Error("the extra annual cost must be stated")
	}
	// It is one whole monthly payment, and each fortnightly payment is half of one.
	if b.HalfPaymentMinor*2 < b.ExtraPerYearMinor-2 || b.HalfPaymentMinor*2 > b.ExtraPerYearMinor+2 {
		t.Errorf("half payment %d doubled should be the monthly payment %d",
			b.HalfPaymentMinor, b.ExtraPerYearMinor)
	}
}

func TestBiweeklyRefusesWhatCannotBeModelled(t *testing.T) {
	cases := []struct {
		name    string
		balance int64
		apr     float64
		term    int
	}{
		{"no balance", 0, 6, 360},
		{"no term", 30_000_000, 6, 0},
		{"negative rate", 30_000_000, -1, 360},
	}
	for _, c := range cases {
		if _, ok := SimulateBiweekly(c.balance, c.apr, c.term, start); ok {
			t.Errorf("%s: expected a refusal", c.name)
		}
	}
}

func TestZeroInterestStillShortensTheTerm(t *testing.T) {
	// Nothing is saved in interest, but the loan still ends sooner — and
	// reporting no benefit at all would be wrong.
	b, ok := SimulateBiweekly(1_200_000, 0, 24, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	if b.InterestSavedMinor != 0 {
		t.Errorf("interest saved = %d, want 0 at 0%% APR", b.InterestSavedMinor)
	}
	if b.MonthsSaved <= 0 {
		t.Error("paying more each year must still finish sooner")
	}
}

func twoCards() []Debt {
	return []Debt{
		{ID: "a", Name: "Visa", BalanceMinor: 800_000, APRPct: 22, MinPaymentMinor: 25_000},
		{ID: "b", Name: "Store card", BalanceMinor: 400_000, APRPct: 27, MinPaymentMinor: 15_000},
	}
}

func TestConsolidatingAtALowerRateSavesInterest(t *testing.T) {
	c, ok := Consolidate(twoCards(), 9, 48, 0, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	if !c.Saves() {
		t.Errorf("moving 22%% and 27%% debt to 9%% should save: delta %d", c.InterestDeltaMinor)
	}
	if c.InterestDeltaMinor >= 0 {
		t.Errorf("delta = %d, want negative for a saving", c.InterestDeltaMinor)
	}
}

func TestConsolidatingAtAWorseRateIsReportedAsWorse(t *testing.T) {
	// The comparison has to be able to say no, or it is marketing.
	c, ok := Consolidate(twoCards(), 35, 48, 0, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	if c.Saves() {
		t.Errorf("moving to 35%% must not report a saving: delta %d", c.InterestDeltaMinor)
	}
}

func TestTheFeeIsFinancedAndCounted(t *testing.T) {
	// A comparison that ignores the origination fee flatters every consolidation
	// offer ever made.
	free, _ := Consolidate(twoCards(), 9, 48, 0, start)
	withFee, ok := Consolidate(twoCards(), 9, 48, 5, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	if withFee.FeeMinor != 60_000 {
		t.Errorf("fee = %d, want 5%% of 1,200,000 = 60000", withFee.FeeMinor)
	}
	if withFee.InterestDeltaMinor <= free.InterestDeltaMinor {
		t.Errorf("a 5%% fee must make the deal worse: %d vs %d",
			withFee.InterestDeltaMinor, free.InterestDeltaMinor)
	}
	if withFee.NewTotalMinor <= free.NewTotalMinor {
		t.Error("the financed fee must show up in what is repaid")
	}
}

func TestADebtWithNoPaymentIsNamedNotDropped(t *testing.T) {
	// A total that quietly excluded a debt reads as complete.
	debts := append(twoCards(), Debt{ID: "c", Name: "Old loan", BalanceMinor: 300_000, APRPct: 12})
	c, ok := Consolidate(debts, 9, 48, 0, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	if len(c.Unmodelled) != 1 || c.Unmodelled[0] != "Old loan" {
		t.Errorf("unmodelled = %v, want [Old loan]", c.Unmodelled)
	}
}

func TestADebtWhosePaymentNeverClearsIsNamedToo(t *testing.T) {
	// The single most important fact about that debt, and it would otherwise
	// vanish into a comparison that looked fine.
	debts := append(twoCards(),
		Debt{ID: "c", Name: "Sinking card", BalanceMinor: 500_000, APRPct: 24, MinPaymentMinor: 5_000})
	c, ok := Consolidate(debts, 9, 48, 0, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	found := false
	for _, n := range c.Unmodelled {
		if n == "Sinking card" {
			found = true
		}
	}
	if !found {
		t.Errorf("unmodelled = %v, want it to name the card whose payment never clears", c.Unmodelled)
	}
}

func TestKeepMonthsIsTheLastDebtNotTheFirst(t *testing.T) {
	// The current plan is finished when the last debt is.
	debts := []Debt{
		{ID: "a", Name: "Quick", BalanceMinor: 100_000, APRPct: 10, MinPaymentMinor: 50_000},
		{ID: "b", Name: "Slow", BalanceMinor: 900_000, APRPct: 10, MinPaymentMinor: 20_000},
	}
	c, ok := Consolidate(debts, 9, 48, 0, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	if c.KeepMonths < 40 {
		t.Errorf("keep months = %d, want the long debt's horizon, not the short one's", c.KeepMonths)
	}
}

func TestPaymentDeltaSaysWhatLeavesTheAccount(t *testing.T) {
	// Consolidating usually raises the monthly payment even while saving
	// interest, and someone deciding this month needs to know that.
	c, ok := Consolidate(twoCards(), 9, 24, 0, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	if c.PaymentDeltaMinor <= 0 {
		t.Errorf("payment delta = %d, want a rise for a 24-month payoff of $12,000", c.PaymentDeltaMinor)
	}
}

func TestConsolidationRefusesWhenThereIsNothingToCompare(t *testing.T) {
	one := twoCards()[:1]
	if _, ok := Consolidate(one, 9, 48, 0, start); ok {
		t.Error("one debt is not a consolidation")
	}
	if _, ok := Consolidate(twoCards(), 9, 0, 0, start); ok {
		t.Error("no term must refuse")
	}
	if _, ok := Consolidate(nil, 9, 48, 0, start); ok {
		t.Error("no debts must refuse")
	}
}

func TestWeightedAPRIsWhatAnOfferHasToBeat(t *testing.T) {
	// People compare an offer against their WORST rate, which almost anything
	// beats. This is the number that actually matters.
	got, ok := WeightedAPRPct(twoCards())
	if !ok {
		t.Fatal("expected a rate")
	}
	// (800000*22 + 400000*27) / 1200000 = 23.67
	if got < 23.6 || got > 23.7 {
		t.Errorf("weighted APR = %v, want about 23.67", got)
	}
	// And it is below the worst rate, which is the whole point.
	if got >= 27 {
		t.Error("the blended rate must sit below the worst individual rate")
	}
}

func TestWeightedAPRRefusesWithNoBalances(t *testing.T) {
	if _, ok := WeightedAPRPct(nil); ok {
		t.Error("an average of nothing is not zero")
	}
	if _, ok := WeightedAPRPct([]Debt{{ID: "a", BalanceMinor: 0, APRPct: 20}}); ok {
		t.Error("a zero balance contributes no weight and must not produce a rate")
	}
}

func TestConsolidationIsDeterministic(t *testing.T) {
	debts := []Debt{
		{ID: "z", Name: "Z", BalanceMinor: 300_000, APRPct: 20},
		{ID: "a", Name: "A", BalanceMinor: 300_000, APRPct: 20},
		{ID: "m", Name: "M", BalanceMinor: 300_000, APRPct: 20, MinPaymentMinor: 20_000},
	}
	for i := range 5 {
		c, ok := Consolidate(debts, 9, 48, 0, start)
		if !ok {
			t.Fatal("expected a comparison")
		}
		if len(c.Unmodelled) != 2 || c.Unmodelled[0] != "A" || c.Unmodelled[1] != "Z" {
			t.Fatalf("run %d listed %v, want a stable [A Z]", i, c.Unmodelled)
		}
	}
}

func TestATermCollapseIsNamedAsTheReasonForTheSaving(t *testing.T) {
	// The trap the comparison would otherwise walk into: rolling a long debt into
	// a short loan "saves" enormous interest at almost any rate, because the term
	// collapsed. Presenting that as the benefit of consolidating tells someone a
	// 12% loan beat their 6% mortgage.
	long := []Debt{
		{ID: "a", Name: "Mortgage", BalanceMinor: 20_000_000, APRPct: 6, MinPaymentMinor: 120_000},
		{ID: "b", Name: "Car", BalanceMinor: 2_000_000, APRPct: 6, MinPaymentMinor: 40_000},
	}
	c, ok := Consolidate(long, 12, 48, 0, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	if !c.Saves() {
		t.Fatalf("a 48-month payoff of long debt should still report less interest: %d", c.InterestDeltaMinor)
	}
	if !c.TermDriven() {
		t.Errorf("a %d-month plan replacing a %d-month one must be flagged as term-driven",
			c.NewMonths, c.KeepMonths)
	}
	// And the monthly payment is what buys it.
	if c.PaymentDeltaMinor <= 0 {
		t.Errorf("payment delta = %d, want a rise — the shorter term has to be paid for", c.PaymentDeltaMinor)
	}
}

func TestASimilarTermIsNotFlaggedAsTermDriven(t *testing.T) {
	// Below the threshold the two effects are comparable, and separating them
	// would be false precision.
	c, ok := Consolidate(twoCards(), 9, 48, 0, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	if c.KeepMonths > 0 && c.NewMonths >= int(float64(c.KeepMonths)*TermShortenFactor) && c.TermDriven() {
		t.Error("a comparable term must not be flagged as term-driven")
	}
}

func TestALossIsNeverTermDriven(t *testing.T) {
	c, ok := Consolidate(twoCards(), 40, 48, 0, start)
	if !ok {
		t.Fatal("expected a comparison")
	}
	if c.TermDriven() {
		t.Error("a plan that costs more has no saving to attribute")
	}
}

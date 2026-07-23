// SPDX-License-Identifier: MIT

package finplan

import "testing"

func TestAssessRamseyProgression(t *testing.T) {
	// Fresh user, no data at all: nothing is assessable/done, so the current step is Baby Step 1.
	p := Assess(Ramsey, Inputs{})
	if len(p.Steps) != 7 {
		t.Fatalf("ramsey should have 7 steps, got %d", len(p.Steps))
	}
	if cur, ok := p.Current(); !ok || cur.Num != 1 {
		t.Fatalf("no data → current step should be #1, got %+v ok=%v", cur, ok)
	}

	// Has a $1k+ starter fund and no non-mortgage debt, but only 1 month of reserves → current is
	// Baby Step 3 (full emergency fund).
	in := Inputs{
		HasLiquidData:    true,
		LiquidCashMinor:  2500 * 100,
		EmergencyMonths:  1.0,
		KnowsLiabilities: true,
		HasNonMortgageDebt: false,
	}
	p = Assess(Ramsey, in)
	if p.Steps[0].Status != Done {
		t.Fatalf("step 1 ($1k) should be Done with $2500 liquid")
	}
	if p.Steps[1].Status != Done {
		t.Fatalf("step 2 (debt) should be Done with no non-mortgage debt")
	}
	if cur, ok := p.Current(); !ok || cur.Num != 3 {
		t.Fatalf("current should be #3 (full emergency fund), got %+v", cur)
	}

	// Debt present → step 2 is Todo and becomes current (step 1 still done).
	in.HasNonMortgageDebt = true
	p = Assess(Ramsey, in)
	if cur, _ := p.Current(); cur.Num != 2 {
		t.Fatalf("with debt, current should be #2, got #%d", cur.Num)
	}
}

func TestAssessFOOHighInterestDebt(t *testing.T) {
	// High-interest debt present, deductibles + match answered done → current is step 3 (kill the debt).
	in := Inputs{
		KnowsLiabilities:    true,
		HasHighInterestDebt: true,
		HasNonMortgageDebt:  true,
		AnsweredDeductible:  true, DeductiblesCovered: true,
		AnsweredMatch: true, GetsFullMatch: true,
	}
	p := Assess(FOO, in)
	if len(p.Steps) != 9 {
		t.Fatalf("FOO should have 9 steps, got %d", len(p.Steps))
	}
	if cur, _ := p.Current(); cur.Num != 3 {
		t.Fatalf("high-interest debt → current should be #3, got #%d", cur.Num)
	}
	// Clear the high-interest debt → step 3 done, current advances to emergency fund (#4).
	in.HasHighInterestDebt = false
	p = Assess(FOO, in)
	if p.Steps[2].Status != Done {
		t.Fatalf("no high-interest debt → step 3 should be Done")
	}
	if cur, _ := p.Current(); cur.Num != 4 {
		t.Fatalf("current should advance to #4, got #%d", cur.Num)
	}
}

func TestNotAssessableSurfacesAsCurrent(t *testing.T) {
	// FOO with NO answers/data: step 1 (deductibles) is NotAssessable, which is not Done, so it's the
	// current step — the UI will ask rather than skip it.
	p := Assess(FOO, Inputs{})
	if p.Steps[0].Status != NotAssessable {
		t.Fatalf("step 1 should be NotAssessable without answers")
	}
	if cur, ok := p.Current(); !ok || cur.Num != 1 {
		t.Fatalf("NotAssessable step 1 should be current, got %+v ok=%v", cur, ok)
	}
}

func TestEmergencyMonthsGating(t *testing.T) {
	// No liquid data → emergency step NotAssessable; with 4 months → Done.
	if s := emergencyStatus(Inputs{}); s != NotAssessable {
		t.Fatalf("no liquid data → NotAssessable, got %v", s)
	}
	if s := emergencyStatus(Inputs{HasLiquidData: true, EmergencyMonths: 4}); s != Done {
		t.Fatalf("4 months → Done, got %v", s)
	}
	if s := emergencyStatus(Inputs{HasLiquidData: true, EmergencyMonths: 2}); s != Todo {
		t.Fatalf("2 months → Todo, got %v", s)
	}
}

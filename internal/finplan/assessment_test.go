// SPDX-License-Identifier: MIT

package finplan

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/money"
)

func assessmentData(code string) engineenv.Data {
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	d := engineenv.Data{Now: now, Rates: currency.Rates{Base: code}, Accounts: []domain.Account{{ID: "cash", Name: "Cash", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: code, OpeningBalance: money.New(300000, code)}}}
	for i := 1; i <= 3; i++ {
		date := time.Date(2026, time.Month(10-i), 10, 0, 0, 0, 0, time.UTC)
		d.Transactions = append(d.Transactions, domain.Transaction{ID: date.Format("2006-01") + "-in", AccountID: "cash", Date: date, Amount: money.New(100000, code)}, domain.Transaction{ID: date.Format("2006-01") + "-out", AccountID: "cash", Date: date, Amount: money.New(-50000, code)})
	}
	return d
}

func TestFinancialPlanChoicesPreserveUnknownAndStrictStoredValues(t *testing.T) {
	for _, answer := range []string{"", "unsure", "yes", "no"} {
		c, err := ParseChoices("", answer, answer)
		in := c.Apply(Inputs{})
		if err != nil || c.Kind() != FOO || in.AnsweredMatch != (answer == "yes" || answer == "no") || in.GetsFullMatch != (answer == "yes") || in.AnsweredDeductible != in.AnsweredMatch {
			t.Fatal(answer, c, in, err)
		}
	}
	for _, c := range []Choices{{Framework: "unknown"}, {Framework: " ramsey"}, {Match: "true"}, {Deductible: "maybe"}} {
		if _, err := ParseChoices(c.Framework, c.Match, c.Deductible); err == nil {
			t.Fatal("invalid stored choice became a healthy default", c)
		}
	}
}

func TestFinancialPlanAssessmentUsesCanonicalSignalsAndPreservesInputs(t *testing.T) {
	d := assessmentData("USD")
	before, _ := json.Marshal(d)
	c := Choices{Framework: "foo", Match: "yes", Deductible: "yes"}
	got, err := AssessData(d, c)
	if err != nil || got.Inputs.SavingsRatePct != 50 || got.Inputs.EmergencyMonths != 9 || got.Liquid == nil || got.Liquid.Amount != 450000 || !got.Inputs.HasIncome || !got.Inputs.HasLiquidData {
		t.Fatal(got, err)
	}
	for _, step := range got.Plan.Steps {
		if step.Num <= 7 && step.Status != Done {
			t.Fatal("known canonical signal changed", step)
		}
	}
	if current, ok := got.Plan.Current(); !ok || current.Num != 8 || current.Status != NotAssessable {
		t.Fatal("unknown future step inferred complete", current, ok)
	}
	after, _ := json.Marshal(d)
	if string(before) != string(after) {
		t.Fatal("assessment changed source data/maps")
	}
}

func TestFinancialPlanUnknownAPRAndMissingDataCannotCompleteDebtSteps(t *testing.T) {
	d := assessmentData("USD")
	d.Accounts = append(d.Accounts, domain.Account{ID: "card", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.Zero("USD")})
	got, err := AssessData(d, Choices{Framework: "foo"})
	if err != nil || got.Plan.Steps[2].Status != NotAssessable || got.Unavailable["apr:card"] == "" {
		t.Fatal("missing APR became a zero-interest/cleared step", got, err)
	}
	d.Accounts[1].InterestRateAPR = domain.APR(9)
	got, err = AssessData(d, Choices{Framework: "foo"})
	if err != nil || got.Plan.Steps[2].Status != Todo {
		t.Fatal("known high-interest recorded liability was ignored", got, err)
	}
	got, err = AssessData(engineenv.Data{Now: d.Now, Rates: d.Rates}, Choices{Framework: "foo"})
	if err != nil || got.Plan.Steps[2].Status != NotAssessable || got.Plan.Steps[8].Status != NotAssessable {
		t.Fatal("empty data became a debt-completion claim", got, err)
	}
}

func TestFinancialPlanFailedHealthFactorsStayIndependentlyUnavailable(t *testing.T) {
	d := assessmentData("USD")
	d.Transactions[0].Amount.Currency = "EUR"
	got, err := AssessData(d, Choices{Framework: "foo"})
	if err != nil || got.Inputs.HasIncome || got.Inputs.HasLiquidData || got.Plan.Steps[3].Status != NotAssessable || got.Plan.Steps[4].Status != NotAssessable || got.Unavailable["savings"] == "" {
		t.Fatal("missing FX became known savings/emergency completion", got, err)
	}
	d = assessmentData("USD")
	d.Transactions = []domain.Transaction{{ID: "max", AccountID: "cash", Date: d.Now.AddDate(0, -1, 0), Amount: money.New(math.MaxInt64, "USD")}, {ID: "extra", AccountID: "cash", Date: d.Now.AddDate(0, -1, 0), Amount: money.New(1, "USD")}}
	got, err = AssessData(d, Choices{Framework: "foo"})
	if err != nil || got.Inputs.HasIncome || got.Plan.Steps[4].Status != NotAssessable || got.Unavailable["savings"] == "" || got.Liquid != nil {
		t.Fatal("overflow became a zero/complete assessment", got, err)
	}
}

func TestFinancialPlanRamseyStarterUsesUSDThresholdWithExplicitFXAvailability(t *testing.T) {
	d := assessmentData("JPY")
	d.Accounts[0].OpeningBalance = money.New(100000, "JPY")
	for i := range d.Transactions {
		d.Transactions[i].Amount.Amount /= 100
	}
	d.Rates.Rates = map[string]float64{"USD": 150}
	got, err := AssessData(d, Choices{Framework: "ramsey"})
	if err != nil || got.StarterUSD == nil || got.StarterUSD.Currency != "USD" || got.StarterUSD.Amount != 67667 || got.Plan.Steps[0].Status != Todo || got.Plan.Steps[2].Status != Done {
		t.Fatal("JPY minor units were compared with a USD-cent threshold", got, err)
	}
	d.Rates.Rates = nil
	got, err = AssessData(d, Choices{Framework: "ramsey"})
	if err != nil || got.StarterUSD != nil || got.Unavailable["starter"] == "" || got.Plan.Steps[0].Status != NotAssessable || got.Plan.Steps[2].Status != Done {
		t.Fatal("missing USD FX corrupted an independent emergency-month result", got, err)
	}
}

// SPDX-License-Identifier: MIT

package finplan

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Choices are the persisted /plan questionnaire values. Empty answers and unsure
// remain unanswered; a missing framework uses FOO, without inventing consent.
type Choices struct {
	Framework  string `json:"framework"`
	Match      string `json:"employerMatch"`
	Deductible string `json:"deductibles"`
}

func ParseChoices(framework, match, deductible string) (Choices, error) {
	if framework != "" && framework != "foo" && framework != "ramsey" {
		return Choices{}, fmt.Errorf("financial plan framework is unavailable: expected foo or ramsey")
	}
	for _, v := range []string{match, deductible} {
		if v != "" && v != "yes" && v != "no" && v != "unsure" {
			return Choices{}, fmt.Errorf("financial plan answer is unavailable: expected yes, no or unsure")
		}
	}
	return Choices{Framework: framework, Match: match, Deductible: deductible}, nil
}

func (c Choices) Kind() Framework {
	if c.Framework == "ramsey" {
		return Ramsey
	}
	return FOO
}

func (c Choices) Apply(in Inputs) Inputs {
	in.AnsweredMatch, in.GetsFullMatch = c.Match == "yes" || c.Match == "no", c.Match == "yes"
	in.AnsweredDeductible, in.DeductiblesCovered = c.Deductible == "yes" || c.Deductible == "no", c.Deductible == "yes"
	return in
}

// Assessment composes existing native signal derivation and framework policy.
// Monetary evidence retains exact minor units; the Ramsey starter input uses
// USD cents because its canonical threshold is explicitly USD 1,000.
type Assessment struct {
	Framework   string            `json:"framework"`
	Currency    string            `json:"currency"`
	ObservedAt  time.Time         `json:"observedAt"`
	Choices     Choices           `json:"choices"`
	Inputs      Inputs            `json:"inputs"`
	Plan        Plan              `json:"plan"`
	Liquid      *money.Money      `json:"liquid,omitempty"`
	StarterUSD  *money.Money      `json:"starterUSD,omitempty"`
	Unavailable map[string]string `json:"unavailable,omitempty"`
}

// AssessData leaves failed factors unknown and delegates successful signals to
// HealthInputs and Assess. It never treats a missing APR as a zero-percent loan.
func AssessData(d engineenv.Data, choices Choices) (Assessment, error) {
	validated, err := ParseChoices(choices.Framework, choices.Match, choices.Deductible)
	if err != nil {
		return Assessment{}, err
	}
	if d.Now.IsZero() {
		return Assessment{}, fmt.Errorf("financial plan reference time is required")
	}
	d.Rates.Base = strings.ToUpper(strings.TrimSpace(d.Rates.Base))
	if d.Rates.Base == "" {
		d.Rates.Base = "USD"
	}
	out := Assessment{Framework: "foo", Currency: d.Rates.Base, ObservedAt: d.Now, Choices: validated, Unavailable: map[string]string{}}
	if choices.Kind() == Ramsey {
		out.Framework = "ramsey"
	}
	health := engineenv.HealthInputs(d)
	for factor, e := range health.Unavailable {
		if e != nil {
			out.Unavailable[factor] = e.Error()
		}
	}
	balances, _ := ledger.Balances(d.Accounts, d.Transactions)
	liquid, liquidErr := ledger.LiquidFromBalances(d.Accounts, balances, d.Rates)
	if liquidErr != nil {
		out.Unavailable["liquid"] = liquidErr.Error()
	}
	active, nonMortgage, highInterest, unknownAPR := 0, false, false, false
	for _, a := range d.Accounts {
		if a.Archived {
			continue
		}
		active++
		if !a.IsLiability() || a.Type == "mortgage" {
			continue
		}
		nonMortgage = true
		rate, known := a.RateAPR()
		if !known || math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
			unknownAPR = true
			out.Unavailable["apr:"+a.ID] = "active non-mortgage liability APR is unknown or invalid"
		} else if rate >= HighInterestAPR() {
			highInterest = true
		}
	}
	in := Inputs{HasLiquidData: health.HasLiquidData && health.Unavailable["emergency"] == nil && liquidErr == nil,
		EmergencyMonths: health.EmergencyMonths, HasIncome: health.HasIncome && health.Unavailable["savings"] == nil,
		SavingsRatePct: health.SavingsRatePct, KnowsLiabilities: active > 0 && health.Unavailable["debt"] == nil,
		HasNonMortgageDebt: nonMortgage, HasHighInterestDebt: highInterest}
	if active > 0 && liquidErr == nil {
		out.Liquid = &liquid
	}
	starterKnown := liquidErr == nil && active > 0
	if choices.Kind() == Ramsey && starterKnown {
		usd, e := d.Rates.Convert(liquid, "USD")
		if e != nil {
			starterKnown = false
			out.Unavailable["starter"] = e.Error()
		} else {
			out.StarterUSD = &usd
			in.LiquidCashMinor = usd.Amount
		}
	}
	in = validated.Apply(in)
	out.Inputs = in
	out.Plan = Assess(validated.Kind(), in)
	for i := range out.Plan.Steps {
		step := &out.Plan.Steps[i]
		if validated.Kind() == Ramsey {
			if step.Num == 1 && !starterKnown || step.Num == 2 && !in.KnowsLiabilities {
				step.Status = NotAssessable
			}
		} else if (step.Num == 3 || step.Num == 9) && !in.KnowsLiabilities || step.Num == 3 && unknownAPR && !highInterest {
			step.Status = NotAssessable
		}
	}
	out.Plan.CurrentIndex = firstNotDone(out.Plan.Steps)
	return out, nil
}

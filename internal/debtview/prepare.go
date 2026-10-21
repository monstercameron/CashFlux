// SPDX-License-Identifier: MIT

// Package debtview prepares checked, platform-independent facts for the debt screen.
package debtview

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
)

// Config is the payoff policy that affects debt preparation.
type Config struct {
	ExtraMinor         int64
	Strategy           payoff.Strategy
	CompareMinimumOnly bool
	CompareStrategies  bool
}

// AccountFact holds one liability's checked balance and base-currency availability.
type AccountFact struct {
	Account       domain.Account
	Owed          money.Money
	OwedKnown     bool
	BalanceErr    error
	BaseOwed      money.Money
	BaseKnown     bool
	ConversionErr error
	Included      bool
	Rank          int
}

// StrategyComparison contains checked outcomes for snowball and avalanche at
// the same monthly extra payment. It is populated only when requested.
type StrategyComparison struct {
	Requested           bool
	Known               bool
	Err                 error
	SnowballPlan        payoff.Plan
	SnowballValid       bool
	SnowballErrCalc     error
	AvalanchePlan       payoff.Plan
	AvalancheValid      bool
	AvalancheErrCalc    error
	Match               bool
	RecommendedStrategy payoff.Strategy
	HasRecommendation   bool
	InterestSaved       money.Money
	InterestSavedBy     payoff.Strategy
	HasInterestSaving   bool
	MonthsSaved         int
	MonthsSavedBy       payoff.Strategy
	HasTimeSaving       bool
}

// Result contains per-account facts and checked aggregate payoff calculations.
type Result struct {
	Accounts                   []AccountFact
	TotalOwed                  money.Money
	TotalKnown                 bool
	TotalErr                   error
	PlanOwed                   money.Money
	PlanKnown                  bool
	PlanErr                    error
	Debts                      []payoff.Debt
	Plan                       payoff.Plan
	PlanValid                  bool
	PlanErrCalc                error
	MinimumPlan                payoff.Plan // the same included debts and strategy with no extra payment
	MinimumPlanValid           bool        // whether the minimum-only plan clears every included debt
	MinimumPlanErrCalc         error       // arithmetic error while calculating the minimum-only plan
	MinimumComparisonRequested bool
	SavingsKnown               bool        // both plans clear and checked savings are non-negative
	MonthsSaved                int         // months saved against minimum-only payments
	InterestSaved              money.Money // interest saved against minimum-only payments, in base currency
	SavingsErr                 error       // why the scenario comparison is unavailable
	Comparison                 StrategyComparison
	SuggestedExtraMinor        int64
	SuggestedExtraKnown        bool
	MissingRates               []string // liability currencies with no usable FX rate because it is missing
	InvalidRates               []string // liability currencies with a configured but invalid FX rate
	PlanMissingRates           []string // missing rates, limited to liabilities included in payoff
	PlanInvalidRates           []string // invalid rates, limited to liabilities included in payoff
	DebtFree                   time.Time
}

// Prepare computes liability balances once, reports failures instead of substituting zero,
// and preserves Account.IncludedInPayoff and the selected payoff strategy.
func Prepare(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, base string, cfg Config, now time.Time) Result {
	if base == "" {
		base = rates.Base
	}
	out := Result{TotalOwed: money.Zero(base), TotalKnown: true, PlanOwed: money.Zero(base), PlanKnown: true}
	balances, batchErr := ledger.Balances(accounts, txns)
	_ = batchErr // Errors are attached to the specific account below.
	total, planTotal := money.Zero(base), money.Zero(base)
	var firstTotalErr, firstPlanErr error
	missingRates, invalidRates := map[string]struct{}{}, map[string]struct{}{}
	planMissingRates, planInvalidRates := map[string]struct{}{}, map[string]struct{}{}
	addRateIssue := func(missing, invalid map[string]struct{}, code string, err error) {
		switch {
		case errors.Is(err, currency.ErrUnknownRate):
			missing[code] = struct{}{}
		case errors.Is(err, currency.ErrInvalidRate):
			invalid[code] = struct{}{}
		}
	}
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassLiability {
			continue
		}
		fact := AccountFact{Account: a, Included: a.IncludedInPayoff()}
		bal, ok := balances[a.ID]
		if !ok {
			_, err := ledger.Balance(a, txns)
			if err == nil {
				err = fmt.Errorf("debtview: account %s: balance unavailable", a.ID)
			}
			fact.BalanceErr = err
			if firstTotalErr == nil {
				firstTotalErr = err
			}
			out.TotalKnown = false
			if fact.Included {
				if firstPlanErr == nil {
					firstPlanErr = err
				}
				out.PlanKnown = false
			}
			out.Accounts = append(out.Accounts, fact)
			continue
		}
		owed, err := bal.AbsChecked()
		if err != nil {
			fact.BalanceErr = err
			if firstTotalErr == nil {
				firstTotalErr = err
			}
			out.TotalKnown = false
			if fact.Included {
				if firstPlanErr == nil {
					firstPlanErr = err
				}
				out.PlanKnown = false
			}
			out.Accounts = append(out.Accounts, fact)
			continue
		}
		fact.Owed, fact.OwedKnown = owed, true
		converted, err := rates.Convert(owed, base)
		if err != nil {
			fact.ConversionErr = err
			addRateIssue(missingRates, invalidRates, a.Currency, err)
			if firstTotalErr == nil {
				firstTotalErr = err
			}
			out.TotalKnown = false
			if fact.Included {
				addRateIssue(planMissingRates, planInvalidRates, a.Currency, err)
				if firstPlanErr == nil {
					firstPlanErr = err
				}
				out.PlanKnown = false
			}
			out.Accounts = append(out.Accounts, fact)
			continue
		}
		fact.BaseOwed, fact.BaseKnown = converted, true
		if total, err = total.Add(converted); err != nil {
			if firstTotalErr == nil {
				firstTotalErr = err
			}
			out.TotalKnown = false
		}
		if fact.Included {
			if planTotal, err = planTotal.Add(converted); err != nil {
				if firstPlanErr == nil {
					firstPlanErr = err
				}
				out.PlanKnown = false
			}
			if owed.Amount > 0 {
				min := money.New(a.MinPayment.Amount, a.Currency)
				minBase := int64(0)
				if min.Amount > 0 {
					convertedMin, e := rates.Convert(min, base)
					if e != nil {
						fact.ConversionErr = e
						fact.BaseKnown = false
						out.PlanKnown = false
						addRateIssue(missingRates, invalidRates, a.Currency, e)
						addRateIssue(planMissingRates, planInvalidRates, a.Currency, e)
						if firstPlanErr == nil {
							firstPlanErr = e
						}
					} else {
						minBase = convertedMin.Amount
					}
				}
				if fact.BaseKnown {
					out.Debts = append(out.Debts, payoff.Debt{Name: a.Name, Balance: converted.Amount, AprPercent: a.RateAPROrZero(), MinPayment: minBase})
				}
			}
		}
		out.Accounts = append(out.Accounts, fact)
	}
	if out.TotalKnown {
		out.TotalOwed = total
	} else {
		out.TotalOwed = money.Zero(base)
	}
	if out.PlanKnown {
		out.PlanOwed = planTotal
	} else {
		out.PlanOwed = money.Zero(base)
	}
	out.TotalErr, out.PlanErr = firstTotalErr, firstPlanErr
	out.MissingRates = sortedRateCodes(missingRates)
	out.InvalidRates = sortedRateCodes(invalidRates)
	out.PlanMissingRates = sortedRateCodes(planMissingRates)
	out.PlanInvalidRates = sortedRateCodes(planInvalidRates)
	out.MinimumComparisonRequested = cfg.CompareMinimumOnly
	out.Comparison.Requested = cfg.CompareStrategies
	// A partial list must never escape preparation as though it were a complete
	// payoff universe. This also prevents view consumers from suggesting an extra
	// payment or ranking only the debts that happened to convert successfully.
	if !out.PlanKnown {
		out.Debts = nil
		if cfg.CompareMinimumOnly {
			out.SavingsErr = firstPlanErr
		}
		if cfg.CompareStrategies {
			out.Comparison.Err = firstPlanErr
		}
	}
	order := payoff.FocusOrder(out.Debts, cfg.Strategy)
	ranks := map[string]int{}
	for i, name := range order {
		if _, exists := ranks[name]; !exists {
			ranks[name] = i + 1
		}
	}
	for i := range out.Accounts {
		out.Accounts[i].Rank = ranks[out.Accounts[i].Account.Name]
	}
	if out.PlanKnown {
		out.Plan, out.PlanValid, out.PlanErrCalc = payoff.BuildPlanChecked(out.Debts, cfg.ExtraMinor, cfg.Strategy)
		if cfg.CompareMinimumOnly {
			if cfg.ExtraMinor == 0 {
				out.MinimumPlan, out.MinimumPlanValid, out.MinimumPlanErrCalc = out.Plan, out.PlanValid, out.PlanErrCalc
			} else {
				out.MinimumPlan, out.MinimumPlanValid, out.MinimumPlanErrCalc = payoff.BuildPlanChecked(out.Debts, 0, cfg.Strategy)
			}
			switch {
			case !out.PlanValid:
				out.SavingsErr = out.PlanErrCalc
				if out.SavingsErr == nil {
					out.SavingsErr = fmt.Errorf("debtview: chosen payoff plan cannot clear the included debts")
				}
			case !out.MinimumPlanValid:
				out.SavingsErr = out.MinimumPlanErrCalc
				if out.SavingsErr == nil {
					out.SavingsErr = fmt.Errorf("debtview: minimum-only plan cannot clear the included debts")
				}
			default:
				out.MonthsSaved, out.InterestSaved, out.SavingsErr = planSavings(out.Plan, out.MinimumPlan, base)
				out.SavingsKnown = out.SavingsErr == nil
			}
		}
		if cfg.CompareStrategies {
			out.Comparison = compareStrategies(out.Plan, out.PlanValid, out.PlanErrCalc, cfg.Strategy, out.Debts, cfg.ExtraMinor, base)
		}
		if cfg.CompareStrategies && cfg.ExtraMinor >= 0 && out.PlanErrCalc == nil {
			out.SuggestedExtraMinor = payoff.SuggestedExtra(out.Debts)
			out.SuggestedExtraKnown = true
		}
		if out.PlanValid && len(out.Debts) > 0 {
			out.DebtFree = payoff.DebtFreeMonth(now, out.Plan.Months)
		}
	}
	sort.SliceStable(out.Accounts, func(i, j int) bool {
		a, b := out.Accounts[i], out.Accounts[j]
		if a.Rank > 0 && b.Rank > 0 {
			return a.Rank < b.Rank
		}
		if a.Rank > 0 {
			return true
		}
		if b.Rank > 0 {
			return false
		}
		return a.Owed.Amount > b.Owed.Amount
	})
	return out
}

func sortedRateCodes(codes map[string]struct{}) []string {
	if len(codes) == 0 {
		return nil
	}
	out := make([]string, 0, len(codes))
	for code := range codes {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

func compareStrategies(chosen payoff.Plan, chosenValid bool, chosenErr error, strategy payoff.Strategy, debts []payoff.Debt, extra int64, base string) StrategyComparison {
	out := StrategyComparison{Requested: true}
	if strategy == payoff.Snowball {
		out.SnowballPlan, out.SnowballValid, out.SnowballErrCalc = chosen, chosenValid, chosenErr
		out.AvalanchePlan, out.AvalancheValid, out.AvalancheErrCalc = payoff.BuildPlanChecked(debts, extra, payoff.Avalanche)
	} else if strategy == payoff.Avalanche {
		out.AvalanchePlan, out.AvalancheValid, out.AvalancheErrCalc = chosen, chosenValid, chosenErr
		out.SnowballPlan, out.SnowballValid, out.SnowballErrCalc = payoff.BuildPlanChecked(debts, extra, payoff.Snowball)
	} else {
		out.SnowballPlan, out.SnowballValid, out.SnowballErrCalc = payoff.BuildPlanChecked(debts, extra, payoff.Snowball)
		out.AvalanchePlan, out.AvalancheValid, out.AvalancheErrCalc = payoff.BuildPlanChecked(debts, extra, payoff.Avalanche)
	}
	if !out.SnowballValid {
		out.Err = out.SnowballErrCalc
		if out.Err == nil {
			out.Err = fmt.Errorf("debtview: snowball plan cannot clear the included debts")
		}
		return out
	}
	if !out.AvalancheValid {
		out.Err = out.AvalancheErrCalc
		if out.Err == nil {
			out.Err = fmt.Errorf("debtview: avalanche plan cannot clear the included debts")
		}
		return out
	}
	out.Match = out.SnowballPlan.Months == out.AvalanchePlan.Months && out.SnowballPlan.TotalInterest == out.AvalanchePlan.TotalInterest
	if out.SnowballPlan.Months < out.AvalanchePlan.Months {
		out.RecommendedStrategy, out.HasRecommendation = payoff.Snowball, true
		out.MonthsSaved, out.MonthsSavedBy, out.HasTimeSaving = out.AvalanchePlan.Months-out.SnowballPlan.Months, payoff.Snowball, true
	} else if out.AvalanchePlan.Months < out.SnowballPlan.Months {
		out.RecommendedStrategy, out.HasRecommendation = payoff.Avalanche, true
		out.MonthsSaved, out.MonthsSavedBy, out.HasTimeSaving = out.SnowballPlan.Months-out.AvalanchePlan.Months, payoff.Avalanche, true
	} else if out.SnowballPlan.TotalInterest < out.AvalanchePlan.TotalInterest {
		out.RecommendedStrategy, out.HasRecommendation = payoff.Snowball, true
	} else if out.AvalanchePlan.TotalInterest < out.SnowballPlan.TotalInterest {
		out.RecommendedStrategy, out.HasRecommendation = payoff.Avalanche, true
	}
	if out.SnowballPlan.TotalInterest > out.AvalanchePlan.TotalInterest {
		saved, err := money.Sub(out.SnowballPlan.TotalInterest, out.AvalanchePlan.TotalInterest)
		if err != nil {
			out.Err = fmt.Errorf("debtview: compare strategy interest: %w", err)
			return out
		}
		out.InterestSaved, out.InterestSavedBy, out.HasInterestSaving = money.New(saved, base), payoff.Avalanche, true
	} else if out.AvalanchePlan.TotalInterest > out.SnowballPlan.TotalInterest {
		saved, err := money.Sub(out.AvalanchePlan.TotalInterest, out.SnowballPlan.TotalInterest)
		if err != nil {
			out.Err = fmt.Errorf("debtview: compare strategy interest: %w", err)
			return out
		}
		out.InterestSaved, out.InterestSavedBy, out.HasInterestSaving = money.New(saved, base), payoff.Snowball, true
	}
	out.Known = true
	return out
}

// planSavings returns non-negative differences between the minimum-only and
// chosen plans. A plan that appears worse with extra is treated as unavailable
// rather than presented as a negative saving.
func planSavings(chosen, minimum payoff.Plan, base string) (int, money.Money, error) {
	if chosen.Months > minimum.Months {
		return 0, money.Money{}, fmt.Errorf("debtview: chosen plan takes longer than the minimum-only plan")
	}
	if chosen.TotalInterest > minimum.TotalInterest {
		return 0, money.Money{}, fmt.Errorf("debtview: chosen plan costs more interest than the minimum-only plan")
	}
	interest, err := money.Sub(minimum.TotalInterest, chosen.TotalInterest)
	if err != nil {
		return 0, money.Money{}, fmt.Errorf("debtview: compare plan interest: %w", err)
	}
	return minimum.Months - chosen.Months, money.New(interest, base), nil
}

// SPDX-License-Identifier: MIT

package credithealth

import (
	"errors"
	"math"
	"sort"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
)

// CheckedDemerit preserves a confirmed factor or card callout. Percentages stay
// int64 on wasm; unavailable factors never produce a fabricated demerit.
type CheckedDemerit struct {
	Kind            DemeritKind
	PointsLost      int
	Pct             int64
	AccountID, Name string
}

// CheckedAdvice carries a native-currency payment and independently available
// score impact. A useful payment can survive unknown aggregate FX/proxy data.
type CheckedAdvice struct {
	Kind                        AdviceKind
	ImpactPts, ProjectedScore   int
	ImpactKnown, ProjectedKnown bool
	Pay                         money.Money
	TargetPct                   int
	AccountID, Name             string
	Err                         error
}

// CheckedDetails contains the confirmed issues/actions for a checked result.
// Incomplete prevents an empty list from asserting that no problem exists.
type CheckedDetails struct {
	Demerits   []CheckedDemerit
	Advice     []CheckedAdvice
	Incomplete bool
}

// PrepareDetails derives the legacy policies from checked facts and exact native
// payments. Hypotheticals reuse the canonical checked monetary fold with the
// existing factor scores; no transaction-history scan occurs per suggestion.
func PrepareDetails(in Inputs, rates currency.Rates, r CheckedResult) CheckedDetails {
	out := CheckedDetails{Incomplete: r.UtilErr != nil || r.OnTimeErr != nil}
	wu, wo, wa := renormWeights(r.UtilErr != nil || r.UtilPct >= 0, r.OnTimeErr != nil || r.OnTimeScore >= 0, r.AgeScore >= 0)
	addLoss := func(kind DemeritKind, score int, weight float64, pct int64) {
		if loss := int(math.Round(weight * float64(100-score))); loss > 0 {
			out.Demerits = append(out.Demerits, CheckedDemerit{Kind: kind, PointsLost: loss, Pct: pct})
		}
	}
	if r.UtilErr == nil && r.UtilPct >= 0 {
		addLoss(DemeritUtilization, r.UtilScore, wu, r.UtilPct)
	}
	if r.OnTimeErr == nil && r.OnTimeScore >= 0 {
		addLoss(DemeritOnTime, r.OnTimeScore, wo, 0)
	}
	if r.AgeScore >= 0 {
		addLoss(DemeritAge, r.AgeScore, wa, 0)
	}
	balances := make(map[string]int64, len(in.Balances))
	for id, amount := range in.Balances {
		balances[id] = amount
	}
	for _, c := range r.Cards {
		if c.Err != nil || c.UtilErr != nil {
			out.Incomplete = true
		}
		if c.Err == nil && c.UtilErr == nil {
			kind := DemeritKind("")
			if c.UtilPct > 100 {
				kind = DemeritOverLimit
			} else if c.UtilPct > 50 {
				kind = DemeritCardUtil
			}
			if kind != "" {
				out.Demerits = append(out.Demerits, CheckedDemerit{Kind: kind, Pct: c.UtilPct, AccountID: c.AccountID, Name: c.Name})
			}
		}
		if c.Err == nil && c.Target30.Amount > 0 && (c.UtilPct > 30 || c.UtilErr != nil) {
			advice := CheckedAdvice{Kind: AdviceLowerUtilization, Pay: c.Target30, TargetPct: 30, AccountID: c.AccountID, Name: c.Name}
			if r.ProxyKnown {
				owed, err := c.Balance.AbsChecked()
				var remaining int64
				if err == nil {
					remaining, err = money.Sub(owed.Amount, c.Target30.Amount)
				}
				if err == nil && c.Balance.Amount < 0 {
					remaining, err = money.Neg(remaining)
				}
				if err == nil {
					prior := balances[c.AccountID]
					balances[c.AccountID] = remaining
					hypothetical := in
					hypothetical.Balances = balances
					after := evaluateWithFactors(hypothetical, rates, r.OnTimeScore, r.AgeScore)
					balances[c.AccountID] = prior
					if after.ProxyKnown {
						advice.ImpactKnown = true
						advice.ProjectedKnown = true
						advice.ImpactPts = max(0, after.ProxyScore-r.ProxyScore)
						advice.ProjectedScore = r.ProxyScore + advice.ImpactPts
					} else {
						err = errors.Join(after.UtilErr, after.OnTimeErr)
					}
				}
				advice.Err = err
				if err != nil {
					out.Incomplete = true
				}
			}
			out.Advice = append(out.Advice, advice)
		}
	}
	if r.CardsMissingLimit > 0 {
		out.Demerits = append(out.Demerits, CheckedDemerit{Kind: DemeritMissingLimit, Pct: int64(r.CardsMissingLimit)})
	}
	if r.OnTimeErr == nil && r.OnTimeScore >= 0 && r.OnTimeScore < 100 {
		advice := CheckedAdvice{Kind: AdviceOnTime, ImpactPts: int(math.Round(wo * float64(100-r.OnTimeScore))), ImpactKnown: true}
		if r.ProxyKnown {
			advice.ProjectedKnown = true
			advice.ProjectedScore = min(100, r.ProxyScore+advice.ImpactPts)
		}
		out.Advice = append(out.Advice, advice)
	}
	for _, c := range r.Cards {
		if c.Limit.Amount == 0 {
			out.Advice = append(out.Advice, CheckedAdvice{Kind: AdviceAddLimit, AccountID: c.AccountID, Name: c.Name})
		}
	}
	sort.SliceStable(out.Demerits, func(i, j int) bool { return out.Demerits[i].PointsLost > out.Demerits[j].PointsLost })
	sort.SliceStable(out.Advice, func(i, j int) bool { return out.Advice[i].ImpactPts > out.Advice[j].ImpactPts })
	return out
}

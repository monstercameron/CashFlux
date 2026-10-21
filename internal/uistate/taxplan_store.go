// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

// taxPlanKey is the SettingKV key for the household's tax assumptions (FP-T1e).
//
// PRESERVED, not dataset: an effective rate and last year's tax are statements
// about the household's own situation, not financial records. Losing them on a
// dataset wipe would cost the user figures they had to look up on a tax return.
const taxPlanKey = "cashflux:tax-plan"

// LoadTaxPlan reads the household's tax assumptions.
//
// No defaults are filled in, deliberately. A default effective rate would be the
// app inventing the single number the whole estimate scales by, and the result
// would carry the authority of a calculation while resting on a guess nobody
// made.
func LoadTaxPlan() TaxPlan {
	p, err := ParseTaxPlan(SettingKVGet(taxPlanKey))
	if err != nil {
		return TaxPlan{}
	}
	return p
}

// SaveTaxPlan persists the assumptions, clamping values the engine cannot use so
// a stored figure can never make the estimate refuse on reload.
func SaveTaxPlan(p TaxPlan) {
	if p.EffectiveRatePct < 0 || p.EffectiveRatePct >= 100 {
		p.EffectiveRatePct = 0
	}
	if p.PriorYearTaxMinor < 0 {
		p.PriorYearTaxMinor = 0
	}
	if p.PriorYearIncomeMinor < 0 {
		p.PriorYearIncomeMinor = 0
	}
	if p.PaidToDateMinor < 0 {
		p.PaidToDateMinor = 0
	}
	b, err := json.Marshal(p)
	if err != nil {
		return
	}
	SettingKVSet(taxPlanKey, string(b))
	RequestPersist()
	BumpDataRevision()
}

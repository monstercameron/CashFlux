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

// TaxPlan is what an estimated-tax calculation needs beyond the ledger.
type TaxPlan struct {
	// EffectiveRatePct is the household's own blended rate — income tax plus
	// self-employment tax. Stated by the user, never derived: this app does not
	// know the filing status, the state, or the other income that sets it, and a
	// rate invented from a bracket table would be wrong in a way that looks
	// computed. Zero means unset, and the estimate refuses rather than guessing.
	EffectiveRatePct float64 `json:"effectiveRatePct,omitempty"`
	// PriorYearTaxMinor is what was owed last year — the number the safe harbour
	// is a rule about. Zero means unknown, which removes the harbour rather than
	// treating last year's tax as nothing.
	PriorYearTaxMinor int64 `json:"priorYearTaxMinor,omitempty"`
	// PriorYearIncomeMinor decides which safe-harbour tier applies.
	PriorYearIncomeMinor int64 `json:"priorYearIncomeMinor,omitempty"`
	// PaidToDateMinor is what has already been sent this year.
	PaidToDateMinor int64 `json:"paidToDateMinor,omitempty"`
}

// LoadTaxPlan reads the household's tax assumptions.
//
// No defaults are filled in, deliberately. A default effective rate would be the
// app inventing the single number the whole estimate scales by, and the result
// would carry the authority of a calculation while resting on a guess nobody
// made.
func LoadTaxPlan() TaxPlan {
	var p TaxPlan
	if raw := SettingKVGet(taxPlanKey); raw != "" {
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			return TaxPlan{}
		}
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

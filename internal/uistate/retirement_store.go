// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/retirement"
)

// retirementPlanKey is the SettingKV key for the household's retirement
// assumptions (FP-T1a/FP-T1b).
//
// PRESERVED settings, not dataset: these are statements of intent and belief —
// when someone plans to stop working, what return they expect — not financial
// records. Losing them on a dataset wipe would cost the user thinking the app
// cannot reconstruct.
const retirementPlanKey = "cashflux:retirement-plan"

// LoadRetirementPlan reads the household's assumptions, filling an unset return
// rate with the stated default and taking inflation from the household setting.
//
// Defaults are applied for the RATES only. The horizon and the amounts are left
// at zero deliberately: a default retirement age or contribution would be the
// app inventing a plan and then projecting from it, which is a confident number
// about a life nobody described.
func LoadRetirementPlan() RetirementPlan {
	p, err := ParseRetirementPlan(SettingKVGet(retirementPlanKey))
	if err != nil {
		p, _ = ParseRetirementPlan("")
	}
	p.InflationPct = LoadInflationPct()
	return p
}

// SaveRetirementPlan persists the assumptions, clamped to what the engine can
// use so a stored value can never make the projection refuse on reload.
func SaveRetirementPlan(p RetirementPlan) {
	if p.YearsToRetirement < 0 {
		p.YearsToRetirement = 0
	}
	if p.YearsToRetirement > retirement.MaxYears {
		p.YearsToRetirement = retirement.MaxYears
	}
	if p.AnnualContributionMinor < 0 {
		p.AnnualContributionMinor = 0
	}
	if p.AnnualSpendMinor < 0 {
		p.AnnualSpendMinor = 0
	}
	// Inflation is household-level: route it to the shared setting rather than
	// storing a second copy that can drift from the forecast's.
	SaveInflationPct(p.InflationPct)
	if p.ReturnPct <= -100 {
		p.ReturnPct = DefaultReturnPct
	}
	b, err := json.Marshal(p)
	if err != nil {
		return
	}
	SettingKVSet(retirementPlanKey, string(b))
	RequestPersist()
	BumpDataRevision()
}

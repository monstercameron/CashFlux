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

// RetirementPlan is what the projection needs beyond the account balances the
// app already holds.
type RetirementPlan struct {
	// YearsToRetirement is the accumulation horizon. Zero means unset, and the
	// surface must say so rather than projecting zero years.
	YearsToRetirement int `json:"years,omitempty"`
	// AnnualContributionMinor is what goes in each year.
	AnnualContributionMinor int64 `json:"contribution,omitempty"`
	// AnnualSpendMinor is the retirement spending the drawdown and the FIRE
	// number are both computed from — one number, so the two cannot disagree
	// about what the life costs.
	AnnualSpendMinor int64 `json:"spend,omitempty"`
	// ReturnPct is the assumed nominal annual return, as a percent.
	ReturnPct float64 `json:"returnPct,omitempty"`
	// InflationPct is the household's inflation assumption, as a percent. It is
	// NOT stored here (FP-T2d): Load reads it from the one household setting and
	// Save writes it back there, so the retirement card and the forecast can never
	// hold different beliefs about the same future.
	InflationPct float64 `json:"-"`
}

// DefaultReturnPct is the assumed nominal annual return when none is set.
//
// Seven percent: the commonly-cited long-run US equity average. It is a
// convention, not a forecast, which is why the surface states it back rather
// than applying it invisibly.
const DefaultReturnPct = 7.0

// LoadRetirementPlan reads the household's assumptions, filling an unset return
// rate with the stated default and taking inflation from the household setting.
//
// Defaults are applied for the RATES only. The horizon and the amounts are left
// at zero deliberately: a default retirement age or contribution would be the
// app inventing a plan and then projecting from it, which is a confident number
// about a life nobody described.
func LoadRetirementPlan() RetirementPlan {
	var p RetirementPlan
	if raw := SettingKVGet(retirementPlanKey); raw != "" {
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			p = RetirementPlan{}
		}
	}
	if p.ReturnPct == 0 {
		p.ReturnPct = DefaultReturnPct
	}
	// Inflation comes from the household setting, never from this record — one
	// belief about the future, shared by every surface that discounts to today.
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

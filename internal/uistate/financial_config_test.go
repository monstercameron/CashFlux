// SPDX-License-Identifier: MIT

package uistate

import (
	"math"
	"testing"
)

func TestFinancialConfigParsersPreserveDefaultsPartialOverridesAndNormalization(t *testing.T) {
	alloc, err := ParseAllocConfig("")
	if err != nil || alloc.Profile != "balanced" || alloc.Mode != "weighted" || alloc.AmountMinor != 0 {
		t.Fatalf("allocation defaults: %+v %v", alloc, err)
	}
	alloc, err = ParseAllocConfig(`{"amountMinor":9223372036854775807,"reserveMinor":12,"profile":"saved:custom","mode":"fill"}`)
	if err != nil || alloc.AmountMinor != math.MaxInt64 || alloc.ReserveMinor != 12 || alloc.Profile != "saved:custom" || alloc.Mode != "fill" || alloc.MaxPerMinor != 0 {
		t.Fatalf("allocation override: %+v %v", alloc, err)
	}
	alloc, err = ParseAllocConfig(`{"amountMinor":-1,"reserveMinor":-2,"maxPerMinor":-3,"profile":"","mode":"future-unknown"}`)
	if err != nil || alloc.AmountMinor != 0 || alloc.ReserveMinor != 0 || alloc.MaxPerMinor != 0 || alloc.Profile != "balanced" || alloc.Mode != "weighted" {
		t.Fatalf("allocation legacy normalization: %+v %v", alloc, err)
	}
	planning, err := ParsePlanningConfig(`{"runwayBufferMinor":9223372036854775807,"runwayDays":120}`)
	if err != nil || planning.RunwayBufferMinor != math.MaxInt64 || planning.RunwayDays != 120 || planning.ForecastMonths != 12 || planning.AffordReserveMinor != 0 {
		t.Fatalf("planning partial override: %+v %v", planning, err)
	}
	planning, err = ParsePlanningConfig(`{"runwayBufferMinor":-1,"affordReserveMinor":-2,"runwayDays":0,"forecastMonths":-3}`)
	if err != nil || planning.RunwayBufferMinor != 0 || planning.AffordReserveMinor != 0 || planning.RunwayDays != 60 || planning.ForecastMonths != 12 {
		t.Fatalf("planning legacy normalization: %+v %v", planning, err)
	}
	bills, err := ParseBillsSmartConfig(`{"enabled":true,"minKeepMinor":9223372036854775807}`)
	if err != nil || !bills.Enabled || bills.ViewSmart || bills.PayFrequency != "biweekly" || bills.MinKeepMinor != math.MaxInt64 {
		t.Fatalf("bill partial override: %+v %v", bills, err)
	}
	for _, frequency := range []string{"weekly", "biweekly", "semimonthly", "monthly"} {
		bills, err = ParseBillsSmartConfig(`{"payFrequency":"` + frequency + `"}`)
		if err != nil || bills.PayFrequency != frequency {
			t.Fatalf("frequency lost: %+v %v", bills, err)
		}
	}
	bills, err = ParseBillsSmartConfig(`{"payFrequency":"future-unknown","minKeepMinor":-1}`)
	if err != nil || bills.PayFrequency != "biweekly" || bills.MinKeepMinor != 0 {
		t.Fatalf("bills legacy normalization: %+v %v", bills, err)
	}
}

func TestFinancialConfigParsersKeepCorruptStorageUnavailable(t *testing.T) {
	for _, raw := range []string{" ", "{broken", "null", "[]", "true", `"text"`, `{"amountMinor":"not money","runwayDays":"bad","minKeepMinor":"bad"}`, `{} {}`} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseAllocConfig(raw); err == nil {
				t.Error("allocation accepted corrupt/wrong-shaped override")
			}
			if _, err := ParsePlanningConfig(raw); err == nil {
				t.Error("planning accepted corrupt/wrong-shaped override")
			}
			if _, err := ParseBillsSmartConfig(raw); err == nil {
				t.Error("bill schedule accepted corrupt/wrong-shaped override")
			}
		})
	}
}

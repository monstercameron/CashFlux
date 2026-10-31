// SPDX-License-Identifier: MIT

package uistate

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AllocConfig is the allocation page's persisted plan. Monetary values use the
// household base currency's minor units; a zero MaxPerMinor means uncapped.
type AllocConfig struct {
	AmountMinor  int64  `json:"amountMinor"`
	ReserveMinor int64  `json:"reserveMinor"`
	MaxPerMinor  int64  `json:"maxPerMinor"`
	Profile      string `json:"profile"`
	Mode         string `json:"mode"`
}

func defaultAllocConfig() AllocConfig { return AllocConfig{Profile: "balanced", Mode: "weighted"} }

// ParseAllocConfig reads the canonical effective allocation configuration.
// Empty input means no override. Malformed/wrong-shaped JSON remains an error;
// existing value normalization is shared with the allocation page.
func ParseAllocConfig(raw string) (AllocConfig, error) {
	c := defaultAllocConfig()
	if err := decodeFinancialConfig(raw, &c); err != nil {
		return AllocConfig{}, err
	}
	if c.Profile == "" {
		c.Profile = "balanced"
	}
	if c.Mode != "weighted" && c.Mode != "fill" {
		c.Mode = "weighted"
	}
	if c.AmountMinor < 0 {
		c.AmountMinor = 0
	}
	if c.ReserveMinor < 0 {
		c.ReserveMinor = 0
	}
	if c.MaxPerMinor < 0 {
		c.MaxPerMinor = 0
	}
	return c, nil
}

// PlanningConfig is the planning page's persisted cash-floor and horizon policy.
// Monetary values use the household base currency's minor units.
type PlanningConfig struct {
	RunwayBufferMinor  int64 `json:"runwayBufferMinor"`
	RunwayDays         int   `json:"runwayDays"`
	ForecastMonths     int   `json:"forecastMonths"`
	AffordReserveMinor int64 `json:"affordReserveMinor"`
}

func defaultPlanningConfig() PlanningConfig {
	return PlanningConfig{RunwayDays: 60, ForecastMonths: 12}
}

// ParsePlanningConfig shares the page's defaults and normalization while keeping
// malformed/wrong-shaped stored values distinguishable from absent overrides.
func ParsePlanningConfig(raw string) (PlanningConfig, error) {
	c := defaultPlanningConfig()
	if err := decodeFinancialConfig(raw, &c); err != nil {
		return PlanningConfig{}, err
	}
	if c.RunwayDays <= 0 {
		c.RunwayDays = 60
	}
	if c.ForecastMonths <= 0 {
		c.ForecastMonths = 12
	}
	if c.RunwayBufferMinor < 0 {
		c.RunwayBufferMinor = 0
	}
	if c.AffordReserveMinor < 0 {
		c.AffordReserveMinor = 0
	}
	return c, nil
}

// BillsSmartConfig is the smart bill schedule's persisted policy. MinKeepMinor
// is the base-currency liquidity floor; pay-cycle anchor lives in preferences.
type BillsSmartConfig struct {
	Enabled      bool   `json:"enabled"`
	ViewSmart    bool   `json:"viewSmart"`
	PayFrequency string `json:"payFrequency"`
	MinKeepMinor int64  `json:"minKeepMinor"`
}

func defaultBillsSmartConfig() BillsSmartConfig { return BillsSmartConfig{PayFrequency: "biweekly"} }

// ParseBillsSmartConfig shares the schedule's canonical defaults/normalization,
// returning an error for malformed or wrong-shaped stored JSON.
func ParseBillsSmartConfig(raw string) (BillsSmartConfig, error) {
	c := defaultBillsSmartConfig()
	if err := decodeFinancialConfig(raw, &c); err != nil {
		return BillsSmartConfig{}, err
	}
	switch c.PayFrequency {
	case "weekly", "biweekly", "semimonthly", "monthly":
	default:
		c.PayFrequency = "biweekly"
	}
	if c.MinKeepMinor < 0 {
		c.MinKeepMinor = 0
	}
	return c, nil
}

func decodeFinancialConfig(raw string, v any) error {
	if raw == "" {
		return nil
	}
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("configuration must be a JSON object")
	}
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		return fmt.Errorf("configuration is unavailable: %w", err)
	}
	return nil
}

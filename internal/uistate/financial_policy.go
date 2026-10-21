// SPDX-License-Identifier: MIT

package uistate

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/inflation"
)

const (
	RoundUpCadenceWeekly  = "weekly"
	RoundUpCadenceMonthly = "monthly"
)

// DebtConfig is the persisted, user-overridable policy for the debt page. It holds the
// tunables the tiles used to hardcode — the utilization bands that tint a card's rail and
// the payoff planner's defaults — so changing them is a config write, not a code change.
// The engine still computes the raw figures (owed, utilization, APR); this config only
// decides how they're banded and what the planner seeds.
type DebtConfig struct {
	// WarnUtilizationPct: a credit card at/above this utilization tints amber (default 30 —
	// the conventional "starts to weigh on a credit score" line).
	WarnUtilizationPct int `json:"warnUtilizationPct"`
	// HighUtilizationPct: at/above this tints red / reads as critical (default 75).
	HighUtilizationPct int `json:"highUtilizationPct"`
	// DefaultStrategy seeds the payoff planner: "avalanche" (highest APR first) or
	// "snowball" (smallest balance first). Default "avalanche".
	DefaultStrategy string `json:"defaultStrategy"`
	// DefaultExtraMinor is the payoff planner's seed extra monthly payment, in minor units
	// of the base currency (default 0).
	DefaultExtraMinor int64 `json:"defaultExtraMinor"`
	// ExcludeMortgage: exclude a mortgage from the payoff plan by default (default true —
	// most people don't snowball a 30-year mortgage).
	ExcludeMortgage bool `json:"excludeMortgage"`
}

// defaultDebtConfig is the seed config used until the user overrides it — the values the
// debt tiles previously baked in as literals, now data.
func defaultDebtConfig() DebtConfig {
	return DebtConfig{
		WarnUtilizationPct: 30,
		HighUtilizationPct: 75,
		DefaultStrategy:    "avalanche",
		DefaultExtraMinor:  0,
		ExcludeMortgage:    true,
	}
}

// UtilizationBand classifies a utilization percent into "good" | "warn" | "high" using the
// config's thresholds, so the tiles tint a card's rail without hardcoding the cutoffs.
func (c DebtConfig) UtilizationBand(pct float64) string {
	switch {
	case pct >= float64(c.HighUtilizationPct):
		return "high"
	case pct >= float64(c.WarnUtilizationPct):
		return "warn"
	default:
		return "good"
	}
}

// SweepConfig is the persisted leftover-sweep policy (XC6): whether the month-
// close sweep ritual is on, which budgets contribute their unspent remainder, and
// the goal the swept total earmarks toward. Stored as config so the ritual is
// data, not code.
type SweepConfig struct {
	// Enabled turns the month-close sweep card + ritual on.
	Enabled bool `json:"enabled"`
	// BudgetIDs are the budgets whose leftovers participate.
	BudgetIDs []string `json:"budgetIds"`
	// TargetGoalID is the goal the swept total is earmarked toward.
	TargetGoalID string `json:"targetGoalId"`
}

// Domain returns the config as the pure-logic budgeting.SweepConfig used by
// ComputeSweep, so the UI and the tested logic share one shape.
func (c SweepConfig) Domain() budgeting.SweepConfig {
	return budgeting.SweepConfig{
		Enabled:      c.Enabled,
		BudgetIDs:    c.BudgetIDs,
		TargetGoalID: c.TargetGoalID,
	}
}

// RoundUpConfig is the persisted virtual round-up policy (TX11): whether the
// round-up jar is on, the goal the jar sweeps into, the sweep cadence, which
// accounts participate, and the timestamp of the last approved sweep (the
// accrual window starts there). Stored as config so the ritual is data, not code.
type RoundUpConfig struct {
	// Enabled turns the round-up jar + sweep card on.
	Enabled bool `json:"enabled"`
	// TargetGoalID is the goal the swept jar is earmarked toward.
	TargetGoalID string `json:"targetGoalId"`
	// Cadence is RoundUpCadenceWeekly or RoundUpCadenceMonthly.
	Cadence string `json:"cadence"`
	// AccountIDs are the participating accounts; empty means every account.
	AccountIDs []string `json:"accountIds"`
	// LastSweepStamp is the RFC3339 time the jar was last swept, the exclusive
	// lower bound of the next accrual window. Empty means never swept.
	LastSweepStamp string `json:"lastSweepStamp"`
}

// EffectiveCadence returns the cadence, defaulting to weekly.
func (c RoundUpConfig) EffectiveCadence() string {
	if c.Cadence == RoundUpCadenceMonthly {
		return RoundUpCadenceMonthly
	}
	return RoundUpCadenceWeekly
}

// SinceOr returns the accrual window's start: the parsed LastSweepStamp, or the
// supplied fallback when no sweep has been recorded yet.
func (c RoundUpConfig) SinceOr(fallback time.Time) time.Time {
	if c.LastSweepStamp == "" {
		return fallback
	}
	t, err := time.Parse(time.RFC3339, c.LastSweepStamp)
	if err != nil {
		return fallback
	}
	return t
}

// ParticipatingSet returns the account-id set (empty map when all participate).
func (c RoundUpConfig) ParticipatingSet() map[string]bool {
	if len(c.AccountIDs) == 0 {
		return nil
	}
	m := make(map[string]bool, len(c.AccountIDs))
	for _, id := range c.AccountIDs {
		m[id] = true
	}
	return m
}

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

// NotifyRoute is one entry of the notification link config: a notification ID prefix (the
// rule that produced it, e.g. "default-bill-due") and the in-app path it links to.
type NotifyRoute struct {
	Prefix string `json:"prefix"`
	Route  string `json:"route"`
}

// defaultNotifyRoutes is the seed config used until the user overrides it. It is DATA
// (persisted + overridable at runtime via SetNotifyRoutes), not compiled-in behaviour —
// changing where a notification links needs no code change, just a new stored config.
func defaultNotifyRoutes() []NotifyRoute {
	return []NotifyRoute{
		{"default-bill-due", "/bills"},
		{"default-budget", "/budgets"},
		{"default-stale", "/accounts"},
		{"default-low", "/accounts"},
		{"default-goal", "/goals"},
		{"default-large", "/transactions"},
		{"default-unusual", "/transactions"},
		{"default-paycheck", "/transactions"},
		{"default-digest", "/reports"},
	}
}

// ParseDebtConfig shares the debt page's partial-override defaults and bands.
func ParseDebtConfig(raw string) (DebtConfig, error) {
	c := defaultDebtConfig()
	if err := decodeFinancialConfig(raw, &c); err != nil {
		return DebtConfig{}, err
	}
	if c.HighUtilizationPct < c.WarnUtilizationPct {
		c.HighUtilizationPct = c.WarnUtilizationPct
	}
	if c.DefaultStrategy != "snowball" && c.DefaultStrategy != "avalanche" {
		c.DefaultStrategy = "avalanche"
	}
	return c, nil
}

func ParseSweepConfig(raw string) (SweepConfig, error) {
	var c SweepConfig
	if err := decodeFinancialConfig(raw, &c); err != nil {
		return SweepConfig{}, err
	}
	return c, nil
}

func ParseRoundUpConfig(raw string) (RoundUpConfig, error) {
	if raw == "" {
		return RoundUpConfig{Cadence: RoundUpCadenceWeekly}, nil
	}
	var c RoundUpConfig
	if err := decodeFinancialConfig(raw, &c); err != nil {
		return RoundUpConfig{}, err
	}
	return c, nil
}

func ParseTaxPlan(raw string) (TaxPlan, error) {
	var p TaxPlan
	if err := decodeFinancialConfig(raw, &p); err != nil {
		return TaxPlan{}, err
	}
	return p, nil
}

// ParseRetirementPlan fills only the stated return assumption. Inflation comes
// from the separately persisted household assumption, not this JSON object.
func ParseRetirementPlan(raw string) (RetirementPlan, error) {
	var p RetirementPlan
	if err := decodeFinancialConfig(raw, &p); err != nil {
		return RetirementPlan{}, err
	}
	if p.ReturnPct == 0 {
		p.ReturnPct = DefaultReturnPct
	}
	return p, nil
}

// ParseInflationPct distinguishes an absent household assumption from bad data.
func ParseInflationPct(raw string) (float64, error) {
	if raw == "" {
		return inflation.DefaultRatePct, nil
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || !inflation.Valid(f) {
		return 0, fmt.Errorf("inflation assumption is unavailable: invalid annual percentage")
	}
	return f, nil
}

// ParseNotifyRoutes preserves first-match order and the page's empty-list default.
func ParseNotifyRoutes(raw string) ([]NotifyRoute, error) {
	if raw == "" {
		return defaultNotifyRoutes(), nil
	}
	if !strings.HasPrefix(strings.TrimSpace(raw), "[") {
		return nil, fmt.Errorf("notification routes must be a JSON array")
	}
	var rs []NotifyRoute
	if err := json.Unmarshal([]byte(raw), &rs); err != nil {
		return nil, fmt.Errorf("notification routes are unavailable: %w", err)
	}
	if len(rs) == 0 {
		return defaultNotifyRoutes(), nil
	}
	return rs, nil
}

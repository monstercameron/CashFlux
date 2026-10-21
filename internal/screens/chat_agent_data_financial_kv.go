// SPDX-License-Identifier: MIT
//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantdata"
	"github.com/monstercameron/CashFlux/internal/benchseries"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/notify"
	"github.com/monstercameron/CashFlux/internal/portfolio"
	"github.com/monstercameron/CashFlux/internal/reportsnap"
	"github.com/monstercameron/CashFlux/internal/savedreports"
	"github.com/monstercameron/CashFlux/internal/standing"
	"github.com/monstercameron/CashFlux/internal/txntemplate"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// Source keys are compile-time catalogue entries, never supplied by tool input.
// Reads bypass browser migrations, global caches and tolerant page fallbacks.
func assistantFinancialSource(app *appstate.App, key string, settings bool) (string, bool, error) {
	if settings {
		return app.Store().GetSettingKV(key)
	}
	return app.Store().GetKV(key)
}

func assistantFinancialObject[T any](raw string) (T, error) {
	var v T
	if raw == "" {
		return v, nil
	}
	if !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		return v, fmt.Errorf("stored financial record must be a JSON object")
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return v, fmt.Errorf("stored financial record unavailable: %w", err)
	}
	return v, nil
}

func assistantFinancialArray[T any](raw string) ([]T, error) {
	if raw == "" {
		return []T{}, nil
	}
	if !strings.HasPrefix(strings.TrimSpace(raw), "[") {
		return nil, fmt.Errorf("stored financial collection must be a JSON array")
	}
	var v []T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil, fmt.Errorf("stored financial collection unavailable: %w", err)
	}
	return v, nil
}

func assistantFinancialConfig[T any](app *appstate.App, name, key string, settings bool, parse func(string) (T, error)) assistantdata.Collection {
	c := assistantdata.NewCollection(name, "id", func() ([]assistantConfigRecord[T], error) {
		raw, present, err := assistantFinancialSource(app, key, settings)
		if err != nil {
			return nil, fmt.Errorf("financial configuration source unavailable: %w", err)
		}
		v, err := parse(raw)
		if err != nil {
			return nil, err
		}
		return []assistantConfigRecord[T]{{ID: "household", Stored: present && raw != "", Config: v}}, nil
	})
	c.Notes = "One household configuration record. stored=false means no persisted override; canonical defaults or an unset configuration are then disclosed here. Malformed storage is unavailable, never replaced with healthy zero. "
	if settings {
		c.Notes += "The source is preserved settings, which can survive a dataset wipe. "
	} else {
		c.Notes += "The source is this workspace's persisted dataset configuration. "
	}
	return c
}

type assistantPaidOccurrence struct {
	ID     string `json:"id"`
	PaidAt int64  `json:"paidAt"`
}

// Inflation is persisted separately from RetirementPlan (whose field has json:"-").
// List it explicitly so neither its value nor its stored/default provenance is lost.
type assistantRetirementAssumptions struct {
	YearsToRetirement       int     `json:"years"`
	AnnualContributionMinor int64   `json:"contribution"`
	AnnualSpendMinor        int64   `json:"spend"`
	ReturnPct               float64 `json:"returnPct"`
	InflationPct            float64 `json:"inflationPct"`
	InflationStored         bool    `json:"inflationStored"`
}

func assistantAdditionalFinancialCollections(app *appstate.App) []assistantdata.Collection {
	sweep := assistantFinancialConfig(app, "budget_sweep_config", "cashflux:budgets:sweepconfig", false, uistate.ParseSweepConfig)
	sweep.Notes += "Virtual monthly budget-leftover earmarks toward targetGoalId; this is distinct from account-transfer sweep preferences. Budget IDs remain exact, including archived/deleted references."
	roundup := assistantFinancialConfig(app, "goal_roundup_config", "cashflux:goals:roundupconfig", false, uistate.ParseRoundUpConfig)
	roundup.Notes += "Virtual round-up goal jar, distinct from account-transfer round-up preferences. Blank/unrecognized cadence is effectively weekly; absent policy is disabled weekly. Empty accountIds means all accounts. lastSweepStamp is an RFC3339 accrual boundary, not a balance."
	debt := assistantFinancialConfig(app, "debt_config", "cashflux:debt:config", false, uistate.ParseDebtConfig)
	debt.Notes += "Effective partial overrides use the debt page's defaults and band/strategy normalization. Utilization thresholds are percentages; defaultExtraMinor uses financial_settings.baseCurrency minor units."
	tax := assistantFinancialConfig(app, "tax_assumptions", "cashflux:tax-plan", true, uistate.ParseTaxPlan)
	tax.Notes += "effectiveRatePct is a user-stated percentage, not a decimal fraction. All *Minor fields use financial_settings.baseCurrency minor units. Zero rate or priorYearTaxMinor means unset/unknown, not a zero tax estimate. No filing status or tax liability is inferred."
	inflation := assistantFinancialConfig(app, "inflation_assumption", "cashflux:inflation-pct", true, uistate.ParseInflationPct)
	inflation.Notes += "config is the annual inflation assumption in percentage points, not a ratio, observed inflation or money. Missing input uses the app's stated inflation default. Invalid numeric input is unavailable."
	retirement := assistantFinancialConfig(app, "retirement_assumptions", "cashflux:retirement-plan", true, func(raw string) (assistantRetirementAssumptions, error) {
		p, err := uistate.ParseRetirementPlan(raw)
		if err != nil {
			return assistantRetirementAssumptions{}, err
		}
		inflationRaw, present, err := app.Store().GetSettingKV("cashflux:inflation-pct")
		if err != nil {
			return assistantRetirementAssumptions{}, fmt.Errorf("household inflation source unavailable: %w", err)
		}
		pct, err := uistate.ParseInflationPct(inflationRaw)
		if err != nil {
			return assistantRetirementAssumptions{}, err
		}
		return assistantRetirementAssumptions{p.YearsToRetirement, p.AnnualContributionMinor, p.AnnualSpendMinor, p.ReturnPct, pct, present && inflationRaw != ""}, nil
	})
	retirement.Notes += "years is a year count; zero means horizon unset. contribution and spend are ANNUAL amounts in financial_settings.baseCurrency minor units despite their field names. returnPct is a nominal annual percentage; zero/unset return uses the stated 7 percent assumption. inflationPct is the separately stored household annual percentage; inflationStored distinguishes its override from default. These are assumptions, not computed outcomes."
	routes := assistantFinancialConfig(app, "notification_routes", "cashflux:notify:routes", false, uistate.ParseNotifyRoutes)
	routes.Notes += "config is an ordered route array: the first matching prefix wins. Source order and duplicate prefixes are preserved; missing/empty arrays use the canonical default route table. Routes are data, never executed."
	rules := assistantFinancialConfig(app, "notification_rule_config", notify.RuleConfigKey(), true, notify.ParseRuleConfig)
	rules.Notes += "The legacy bare enabled-map is promoted exactly as in Settings. Missing rule IDs default enabled. Positive threshold overrides use the rule's units: default-bill-due uses days; default-large, default-low-balance, default-paycheck and default-unusual use base-currency minor units. A nonpositive override uses that rule's default; other rule thresholds must not be assumed to be money. Quiet times are local minutes since midnight; absent/unrecognized digestCadence is effectively weekly."
	targets := assistantFinancialConfig(app, "portfolio_targets", "cashflux:rebalance-targets", true, assistantFinancialArray[portfolio.Target])
	targets.Notes += "config is the complete ordered target list with exact stored AssetClass and Pct field names. Pct is desired allocation percentage, not money. Missing targets mean no target allocation. Invalid totals or duplicate classes are retained for inspection, not silently corrected or treated as a valid recommendation."
	benchmark := assistantFinancialConfig(app, "benchmark_series", "cashflux:benchmark-series", true, assistantFinancialObject[benchseries.Series])
	benchmark.Notes += "config contains name and every stored ordered point: d is an ISO datetime, v is a reference observation (index points/NAV/percentage), NOT currency or minor units. Missing points mean no comparison. Normal import caps at 20000 points; this read never recaps, sorts, deduplicates or computes returns. Use read_record continuation for long series."
	paid := assistantdata.NewCollection("occurrence_paid", "id", func() ([]assistantPaidOccurrence, error) {
		raw, _, err := app.Store().GetKV("cashflux:occurrences:paid")
		if err != nil {
			return nil, err
		}
		m, err := assistantFinancialObject[map[string]int64](raw)
		if err != nil {
			return nil, err
		}
		out := make([]assistantPaidOccurrence, 0, len(m))
		for id, at := range m {
			out = append(out, assistantPaidOccurrence{ID: id, PaidAt: at})
		}
		return out, nil
	})
	paid.UnixSecondsDateFields = map[string]assistantdata.UnixSecondsDateField{"paidAt": {ZeroIsMissing: true}}
	paid.Notes = "Every retained paid-status map entry. id is the EXACT billID|YYYY-MM-DD composite key (recurring bill IDs start recurring:); paidAt is the recorded Unix-second marking time, not the due date. References to removed bills are retained. No bill matching, paid-status recomputation or pruning occurs."
	standingInstructions := assistantdata.NewCollection("standing_instructions", "id", func() ([]standing.Instruction, error) {
		raw, _, err := app.Store().GetSettingKV("cashflux:standing-instructions")
		if err != nil {
			return nil, err
		}
		book, err := assistantFinancialObject[standing.Book](raw)
		return book.Instructions, err
	})
	standingInstructions.Notes = "Every retained instruction from preserved settings, including unsupported or currently inapplicable rows. keep_liquid amountMinor is the base-currency cash floor; never_draw_from subject names an account. Instructions constrain advice, not recording actual transactions. No validity filtering, inference or enforcement occurs on this read."
	templates := assistantdata.NewCollection("transaction_templates", "id", func() ([]domain.TxnTemplate, error) {
		raw, _, err := app.Store().GetKV("cashflux:txntemplates")
		if err != nil {
			return nil, err
		}
		v, err := assistantFinancialObject[txntemplate.Store](raw)
		return v.Items, err
	})
	templates.Notes = "All retained saved transaction quick-templates, including references to archived/deleted accounts. amountMinor is a magnitude in each record's currency; direction carries income/expense. No signing, posting, filtering or template execution occurs."
	health := assistantdata.NewCollection("health_history", "month", func() ([]uistate.HealthSnapshot, error) {
		raw, _, err := app.Store().GetKV("cashflux:health:trend")
		if err != nil {
			return nil, err
		}
		return assistantFinancialArray[uistate.HealthSnapshot](raw)
	})
	health.Notes = "All retained historical monthly health snapshots; exact month is YYYY-MM and identifies a stored month, not a daily timestamp. score is a historical dimensionless score and band its historical label, not a recomputed current assessment. Normal writes retain 12 months; this read does not cap or prune."
	reports := assistantdata.NewCollection("saved_reports", "id", func() ([]savedreports.Saved, error) {
		raw, _, err := app.Store().GetKV("cashflux:saved-reports")
		if err != nil {
			return nil, err
		}
		return assistantFinancialArray[savedreports.Saved](raw)
	})
	reports.Notes = "Every retained named report configuration, with exact persisted period and report-local scope. Normal saves cap at 20; this read does not cap, deduplicate names, alter scope or run a report. from/to are ISO datetimes."
	snapshots := assistantdata.NewCollection("report_snapshots", "id", func() ([]reportsnap.Snapshot, error) {
		raw, _, err := app.Store().GetKV("cashflux:report-snapshots")
		if err != nil {
			return nil, err
		}
		return assistantFinancialArray[reportsnap.Snapshot](raw)
	})
	snapshots.Notes = "Every retained frozen report snapshot, not current ledger facts. income, expense and nested amount values are integer MINOR units in each record's base currency. takenAt is an ISO datetime; periodLabel is a historical label. Normal saves retain 24 snapshots; no recapping, recomputation, net subtraction or currency conversion occurs on read."
	return []assistantdata.Collection{sweep, roundup, paid, standingInstructions, templates, tax, retirement, inflation, routes, rules, targets, benchmark, health, reports, snapshots, debt}
}

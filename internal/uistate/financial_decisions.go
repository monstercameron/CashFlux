// SPDX-License-Identifier: MIT

package uistate

import (
	"fmt"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/learntally"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
)

// SubsDetectPrefs controls which transactions the subscription detector considers.
// A transaction whose spending category is in IgnoredCategoryIDs, or whose source
// account type is in IgnoredAccountTypes, is excluded before detection runs —
// letting the user tune out noise (e.g. ignore "Dining" so restaurant charges
// never appear as subscriptions, or ignore "investment" accounts).
type SubsDetectPrefs struct {
	// IgnoredCategoryIDs is the set of category IDs whose transactions are
	// excluded from subscription detection. Keyed by ID so renames don't break it.
	IgnoredCategoryIDs []string `json:"ignoredCategoryIds,omitempty"`
	// IgnoredAccountTypes is the set of account-type strings (e.g. "checking",
	// "investment") whose transactions are excluded from detection.
	IgnoredAccountTypes []string `json:"ignoredAccountTypes,omitempty"`
	// MinOccurrences is the detection sensitivity: the minimum number of times a
	// charge must repeat before it counts as a subscription. 0 = unset (use the
	// default of 2). Higher = fewer, more-confident matches; lower = catch newer
	// or less-frequent charges. (C166)
	MinOccurrences int `json:"minOccurrences,omitempty"`
}

// defaultSubsMinOccurrences is the detection threshold used when the user hasn't
// chosen one — two sightings is the lightest evidence of a recurring charge.
const defaultSubsMinOccurrences = 2

// MinOccurrencesOrDefault returns the configured detection threshold, or the
// default (2) when unset, clamped to a sane 2..6 range.
func (p SubsDetectPrefs) MinOccurrencesOrDefault() int {
	n := p.MinOccurrences
	if n <= 0 {
		return defaultSubsMinOccurrences
	}
	if n > 6 {
		return 6
	}
	return n
}

// WithMinOccurrences returns a copy of p with the detection threshold set.
func (p SubsDetectPrefs) WithMinOccurrences(n int) SubsDetectPrefs {
	out := SubsDetectPrefs{
		IgnoredCategoryIDs:  append([]string(nil), p.IgnoredCategoryIDs...),
		IgnoredAccountTypes: append([]string(nil), p.IgnoredAccountTypes...),
		MinOccurrences:      n,
	}
	return out
}

// HasIgnoredCategory reports whether id appears in the ignored-category list.
func (p SubsDetectPrefs) HasIgnoredCategory(id string) bool {
	for _, v := range p.IgnoredCategoryIDs {
		if v == id {
			return true
		}
	}
	return false
}

// HasIgnoredAccountType reports whether typ appears in the ignored-account-type list.
func (p SubsDetectPrefs) HasIgnoredAccountType(typ string) bool {
	for _, v := range p.IgnoredAccountTypes {
		if v == typ {
			return true
		}
	}
	return false
}

// WithCategoryToggled returns a copy of p with id added to (or removed from)
// IgnoredCategoryIDs, depending on whether it is currently present.
func (p SubsDetectPrefs) WithCategoryToggled(id string) SubsDetectPrefs {
	out := SubsDetectPrefs{
		IgnoredCategoryIDs:  make([]string, 0, len(p.IgnoredCategoryIDs)),
		IgnoredAccountTypes: append([]string(nil), p.IgnoredAccountTypes...),
		MinOccurrences:      p.MinOccurrences,
	}
	found := false
	for _, v := range p.IgnoredCategoryIDs {
		if v == id {
			found = true
			continue // drop it (toggle off)
		}
		out.IgnoredCategoryIDs = append(out.IgnoredCategoryIDs, v)
	}
	if !found {
		out.IgnoredCategoryIDs = append(out.IgnoredCategoryIDs, id) // toggle on
	}
	return out
}

// WithAccountTypeToggled returns a copy of p with typ added to (or removed from)
// IgnoredAccountTypes, depending on whether it is currently present.
func (p SubsDetectPrefs) WithAccountTypeToggled(typ string) SubsDetectPrefs {
	out := SubsDetectPrefs{
		IgnoredCategoryIDs:  append([]string(nil), p.IgnoredCategoryIDs...),
		IgnoredAccountTypes: make([]string, 0, len(p.IgnoredAccountTypes)),
		MinOccurrences:      p.MinOccurrences,
	}
	found := false
	for _, v := range p.IgnoredAccountTypes {
		if v == typ {
			found = true
			continue // drop it (toggle off)
		}
		out.IgnoredAccountTypes = append(out.IgnoredAccountTypes, v)
	}
	if !found {
		out.IgnoredAccountTypes = append(out.IgnoredAccountTypes, typ) // toggle on
	}
	return out
}

// reportsViews are the valid report-type tabs, used to sanitize a persisted view.
var reportsViews = map[string]bool{"overview": true, "categories": true, "networth": true, "advanced": true}

// ReportsConfig is the persisted reports-page reading posture: which report
// tab the user was on and how they compare (year-over-year vs previous period,
// sub-categories rolled up or expanded). Persisting it means /reports reopens
// exactly how it was being read — the toggles were previously transient and
// reset on every visit.
type ReportsConfig struct {
	// View is the active report tab: overview | categories | networth | advanced.
	View string `json:"view"`
	// YoY compares against the same window one year prior instead of the
	// immediately preceding window.
	YoY bool `json:"yoy"`
	// Rollup folds sub-categories into their top-level parent in the
	// by-category breakdown.
	Rollup bool `json:"rollup"`
}

func defaultReportsConfig() ReportsConfig {
	return ReportsConfig{View: "overview"}
}

// netWorthWindows are the valid analysis windows (months), used to sanitize a
// persisted value. One month is "this month so far"; the rest are trailing
// calendar-month windows.
var netWorthWindows = map[int]bool{0: true, 1: true, 6: true, 12: true, 24: true}

// NetWorthConfig is the persisted net-worth-page reading posture: how many
// calendar months the bridge decomposes and the mirrored chart spans.
// Persisting it means /networth reopens on the window the user was studying.
type NetWorthConfig struct {
	// TrendMonths is the analysis window: 0 (all time), 1, 6, 12 or 24 months.
	// The name is
	// kept from the trend-only era so an already-persisted value still loads.
	TrendMonths int `json:"trendMonths"`
}

func defaultNetWorthConfig() NetWorthConfig {
	return NetWorthConfig{TrendMonths: 6}
}

// WidgetConfigs maps a widget id to its saved settings (per widgetcfg.Schema).
// Persisted to localStorage so widget settings survive reloads, like the layout
// and filter atoms.
type WidgetConfigs map[string]widgetcfg.Config

// For returns one widget's config, never nil.
func (c WidgetConfigs) For(id string) widgetcfg.Config {
	if cfg, ok := c[id]; ok && cfg != nil {
		return cfg
	}
	return widgetcfg.Config{}
}

// WithField returns a deep copy of the configs with one widget's field set, so
// callers can Set the atom without mutating the shared map.
func (c WidgetConfigs) WithField(id, key, value string) WidgetConfigs {
	out := make(WidgetConfigs, len(c)+1)
	for wid, cfg := range c {
		nc := make(widgetcfg.Config, len(cfg))
		for k, v := range cfg {
			nc[k] = v
		}
		out[wid] = nc
	}
	if out[id] == nil {
		out[id] = widgetcfg.Config{}
	}
	out[id][key] = value
	return out
}

// ParseSubsDetectPrefs keeps stored filters and sensitivity distinct from their
// effective detector interpretation (MinOccurrencesOrDefault).
func ParseSubsDetectPrefs(raw string) (SubsDetectPrefs, error) {
	var p SubsDetectPrefs
	if err := decodeFinancialConfig(raw, &p); err != nil {
		return SubsDetectPrefs{}, err
	}
	return p, nil
}

// ParseSmartSettings applies the same free-only defaults and legacy migration
// as the page, without persistence, scheduling, network calls or running features.
func ParseSmartSettings(raw string) (smart.Settings, error) {
	if raw == "" {
		return smart.EnableFreeOnly(smart.Settings{}), nil
	}
	var s smart.Settings
	if err := decodeFinancialConfig(raw, &s); err != nil {
		return smart.Settings{}, err
	}
	return smart.Migrate(s), nil
}

func ParseReportsConfig(raw string) (ReportsConfig, error) {
	c := defaultReportsConfig()
	if err := decodeFinancialConfig(raw, &c); err != nil {
		return ReportsConfig{}, err
	}
	if !reportsViews[c.View] {
		c.View = "overview"
	}
	return c, nil
}

func ParseNetWorthConfig(raw string) (NetWorthConfig, error) {
	c := defaultNetWorthConfig()
	if err := decodeFinancialConfig(raw, &c); err != nil {
		return NetWorthConfig{}, err
	}
	if !netWorthWindows[c.TrendMonths] {
		c.TrendMonths = 6
	}
	return c, nil
}

func ParseWidgetConfigs(raw string) (WidgetConfigs, error) {
	c := WidgetConfigs{}
	if err := decodeFinancialConfig(raw, &c); err != nil {
		return nil, err
	}
	return c, nil
}

func ParseLearnThreshold(raw string) (int, error) {
	if raw == "" {
		return learntally.DefaultMinCount, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("learned-category suggestion threshold is unavailable: expected positive integer")
	}
	return n, nil
}

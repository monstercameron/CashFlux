// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/reportrange"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// rangePresetOption pairs a window preset with its catalog key.
type rangePresetOption struct {
	preset reportrange.Preset
	key    string
}

// rangePresetOptions is the window menu, shortest-lookback last so the two
// twelve-month readings sit together at the top where the default lives.
var rangePresetOptions = []rangePresetOption{
	{reportrange.PresetTrailing12, "rptrange.trailing12"},
	{reportrange.PresetLastCalendarYear, "rptrange.lastYear"},
	{reportrange.PresetYearToDate, "rptrange.ytd"},
	{reportrange.PresetTrailing6, "rptrange.trailing6"},
	{reportrange.PresetTrailing3, "rptrange.trailing3"},
	{reportrange.PresetCustom, "rptrange.custom"},
}

// rangeCompareOption pairs a comparison mode with its catalog key.
type rangeCompareOption struct {
	mode reportrange.CompareMode
	key  string
}

var rangeCompareOptions = []rangeCompareOption{
	{reportrange.CompareSameLastYear, "rptrange.cmpLastYear"},
	{reportrange.ComparePriorPeriod, "rptrange.cmpPrior"},
	{reportrange.CompareNone, "rptrange.cmpNone"},
}

// reportRangePickerProps carries the current settings and the resolved window
// label, so the picker can state what it is currently showing without
// re-deriving it.
type reportRangePickerProps struct {
	Settings reportrange.Settings
	// PrimaryLabel and CompareLabel are the resolved windows in plain English.
	PrimaryLabel string
	CompareLabel string
}

// reportRangePicker is the report's window and comparison-period control (C383).
//
// It is its own component because it owns event hooks and the report function
// has early returns above it (the empty-year CTA), which is the same hook-order
// hazard that bit the to-do bulk bar.
//
// Custom is a MONTH range, not a date range: every figure in the report is
// bucketed by calendar month, so offering day precision would promise a
// resolution the numbers do not have and would make the final bucket a partial
// month silently compared against whole ones.
func reportRangePicker(props reportRangePickerProps) ui.Node {
	cfg := props.Settings

	onPreset := ui.UseEvent(func(e ui.Event) {
		next := cfg
		next.Preset = reportrange.Preset(e.GetValue())
		uistate.SaveReportRange(next)
	})
	onCompare := ui.UseEvent(func(e ui.Event) {
		next := cfg
		next.Compare = reportrange.CompareMode(e.GetValue())
		uistate.SaveReportRange(next)
	})
	onCustomStart := ui.UseEvent(func(e ui.Event) {
		next := cfg
		next.Preset = reportrange.PresetCustom
		next.CustomStart = e.GetValue()
		uistate.SaveReportRange(next)
	})
	onCustomEnd := ui.UseEvent(func(e ui.Event) {
		next := cfg
		next.Preset = reportrange.PresetCustom
		next.CustomEnd = e.GetValue()
		uistate.SaveReportRange(next)
	})

	presetOpts := make([]ui.Node, 0, len(rangePresetOptions))
	for _, o := range rangePresetOptions {
		presetOpts = append(presetOpts, Option(Value(string(o.preset)),
			SelectedIf(cfg.Preset == o.preset), uistate.T(o.key)))
	}
	compareOpts := make([]ui.Node, 0, len(rangeCompareOptions))
	for _, o := range rangeCompareOptions {
		compareOpts = append(compareOpts, Option(Value(string(o.mode)),
			SelectedIf(cfg.Compare == o.mode), uistate.T(o.key)))
	}

	// The month inputs appear only for the custom preset. Two always-visible
	// fields that do nothing under five of the six presets would read as broken.
	var customFields ui.Node = Fragment()
	if cfg.Preset == reportrange.PresetCustom {
		customFields = Div(css.Class("rpta-range-custom"),
			Label(css.Class("rpta-range-ctrl"),
				Span(css.Class("t-caption"), uistate.T("rptrange.from")),
				Input(css.Class("field"), Type("month"), Attr("data-testid", "rpt-range-from"),
					Attr("aria-label", uistate.T("rptrange.from")),
					Value(cfg.CustomStart), OnChange(onCustomStart))),
			Label(css.Class("rpta-range-ctrl"),
				Span(css.Class("t-caption"), uistate.T("rptrange.to")),
				Input(css.Class("field"), Type("month"), Attr("data-testid", "rpt-range-to"),
					Attr("aria-label", uistate.T("rptrange.to")),
					Value(cfg.CustomEnd), OnChange(onCustomEnd))),
			// The end month is inclusive; saying so costs a line and stops a
			// reader from wondering whether "to March" includes March.
			Span(css.Class("rpta-range-hint"), uistate.T("rptrange.inclusiveHint")),
		)
	}

	// The resolved windows are stated in words underneath. A picker that shows
	// "Year to date" without saying which months that came out as leaves the
	// reader to work out what they are looking at from the charts.
	resolved := uistate.T("rptrange.showing", props.PrimaryLabel)
	if props.CompareLabel != "" {
		resolved = uistate.T("rptrange.showingVs", props.PrimaryLabel, props.CompareLabel)
	}

	return Div(css.Class("rpta-range"), Attr("data-testid", "rpt-range"),
		Attr("role", "group"), Attr("aria-label", uistate.T("rptrange.label")),
		Div(css.Class("rpta-range-row"),
			Label(css.Class("rpta-range-ctrl"),
				Span(css.Class("t-caption"), uistate.T("rptrange.window")),
				Select(css.Class("field"), Attr("data-testid", "rpt-range-preset"),
					Attr("aria-label", uistate.T("rptrange.window")), OnChange(onPreset), presetOpts)),
			Label(css.Class("rpta-range-ctrl"),
				Span(css.Class("t-caption"), uistate.T("rptrange.compare")),
				Select(css.Class("field"), Attr("data-testid", "rpt-range-compare"),
					Attr("aria-label", uistate.T("rptrange.compare")), OnChange(onCompare), compareOpts)),
			customFields,
		),
		P(css.Class("rpta-range-resolved"), Attr("data-testid", "rpt-range-resolved"),
			Attr("role", "status"), Attr("aria-live", "polite"), resolved),
	)
}

// spanLabel renders a resolved window as "Sep 2025 – Aug 2026", or as the single
// month when the window is one month long — "Aug 2026 – Aug 2026" reads like a
// bug.
func spanLabel(s reportrange.Span) string {
	if !s.Valid() {
		return ""
	}
	last := s.LastMonth()
	if s.Months() == 1 {
		return last.Format("Jan 2006")
	}
	return uistate.T("rptrange.span", s.Start.Format("Jan 2006"), last.Format("Jan 2006"))
}

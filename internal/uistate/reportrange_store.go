// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/reportrange"
)

// reportRangeKey is the SettingKV key for the report's review-window choice
// (C383) — which preset and which comparison period.
//
// A reading posture, like the rollup and year-over-year toggles beside it, so it
// lives with the other reports config rather than in the dataset.
const reportRangeKey = "cashflux:report-range"

// LoadReportRange reads the persisted window choice. Anything missing or
// unparseable yields the historical default (twelve trailing months versus the
// same months last year) — a bad preference must degrade to the default report,
// never to a blank one.
func LoadReportRange() reportrange.Settings {
	raw := SettingKVGet(reportRangeKey)
	if raw == "" {
		return reportrange.Defaults()
	}
	var s reportrange.Settings
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return reportrange.Defaults()
	}
	if s.Preset == "" {
		s.Preset = reportrange.PresetTrailing12
	}
	if s.Compare == "" {
		s.Compare = reportrange.CompareSameLastYear
	}
	return s
}

// SaveReportRange persists the window choice.
func SaveReportRange(s reportrange.Settings) {
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	SettingKVSet(reportRangeKey, string(b))
	BumpDataRevision()
}

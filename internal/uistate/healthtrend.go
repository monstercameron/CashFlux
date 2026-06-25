// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/GoWebComponents/state"
)

const (
	healthTrendAtomID  = "app:health-trend"
	healthTrendStoreID = "cashflux:health:trend"
	healthTrendCap     = 12 // keep ~one year of monthly snapshots
)

// HealthSnapshot is one month's financial-health reading, persisted locally so
// the dashboard widget and /health page can show a trend (delta + sparkline)
// without ever leaving the device (R27).
type HealthSnapshot struct {
	Month string `json:"month"` // "YYYY-MM" — at most one snapshot per month
	Score int    `json:"score"`
	Band  string `json:"band"`
}

// capturedHealthTrend holds the live trend atom captured during render, so
// RecordHealthSnapshot can push updates from a UseEffect / boot code WITHOUT
// calling the UseAtom hook outside a component (which panics). Mirrors the
// captured-atom pattern in notice.go / notifyfeed.go.
var (
	capturedHealthTrend state.Atom[[]HealthSnapshot]
	healthTrendCaptured bool
)

// UseHealthTrend returns the shared, persisted list of monthly health snapshots
// (oldest first). The dashboard widget records into it; the /health page renders it.
// It also captures the atom so out-of-render code can update it safely.
func UseHealthTrend() state.Atom[[]HealthSnapshot] {
	a := state.UseAtom(healthTrendAtomID, loadHealthTrend())
	capturedHealthTrend = a
	healthTrendCaptured = true
	return a
}

func loadHealthTrend() []HealthSnapshot {
	raw := kvGet(healthTrendStoreID)
	if raw == "" {
		return nil
	}
	var snaps []HealthSnapshot
	if err := json.Unmarshal([]byte(raw), &snaps); err != nil {
		return nil
	}
	return snaps
}

// RecordHealthSnapshot upserts this month's snapshot (one per month, latest wins),
// caps history to a year, persists it, and pushes the new list into the live atom
// so subscribers update immediately regardless of mount order (the C270 pattern).
// It returns the resulting trend (oldest first).
func RecordHealthSnapshot(month string, score int, band string) []HealthSnapshot {
	if month == "" {
		return loadHealthTrend()
	}
	cur := loadHealthTrend()
	out := make([]HealthSnapshot, 0, len(cur)+1)
	replaced := false
	for _, s := range cur {
		if s.Month == month {
			out = append(out, HealthSnapshot{Month: month, Score: score, Band: band})
			replaced = true
			continue
		}
		out = append(out, s)
	}
	if !replaced {
		out = append(out, HealthSnapshot{Month: month, Score: score, Band: band})
	}
	if len(out) > healthTrendCap {
		out = out[len(out)-healthTrendCap:]
	}
	if data, err := json.Marshal(out); err == nil {
		kvSet(healthTrendStoreID, string(data))
	}
	// Push into the live atom via the captured reference (never UseHealthTrend()
	// here — this runs from a UseEffect, where calling a hook panics with
	// "GoUseAtom called outside component context"). Safe because the widget /page
	// always render (capturing the atom) before this effect fires.
	if healthTrendCaptured {
		capturedHealthTrend.Set(out)
	}
	return out
}

// PriorHealthScore returns the most recent snapshot score from a month BEFORE the
// given one (so a "since last month" delta ignores this month's own upsert), with
// ok=false when there's no earlier reading.
func PriorHealthScore(trend []HealthSnapshot, currentMonth string) (score int, ok bool) {
	for i := len(trend) - 1; i >= 0; i-- {
		if trend[i].Month < currentMonth {
			return trend[i].Score, true
		}
	}
	return 0, false
}

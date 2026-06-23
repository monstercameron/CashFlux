//go:build js && wasm

package uistate

import (
	"encoding/json"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/period"
)

const periodWindowStoreID = "cashflux:period-window"

// periodWindowJSON is the on-disk shape for the persisted window. Using
// explicit string fields avoids time.Time JSON subtleties and keeps the
// payload human-readable in DevTools.
type periodWindowJSON struct {
	Res       string `json:"res"`
	From      string `json:"from"` // RFC3339
	To        string `json:"to"`   // RFC3339
	WeekStart int    `json:"weekStart"`
}

// PersistPeriodWindow saves the full dashboard window (resolution + From/To
// anchors + week-start convention) to localStorage so /reports reopens on the
// last-viewed period after a hard reload. The resolution is also kept in its
// own key by PersistResolution; this key carries the anchors that
// PersistResolution intentionally omits.
func PersistPeriodWindow(w period.Window) {
	if !w.Res.Valid() {
		return
	}
	v := periodWindowJSON{
		Res:       string(w.Res),
		From:      w.From.UTC().Format(time.RFC3339),
		To:        w.To.UTC().Format(time.RFC3339),
		WeekStart: int(w.WeekStart),
	}
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	js.Global().Get("localStorage").Call("setItem", periodWindowStoreID, string(data))
}

// LoadPeriodWindow reads the persisted window from localStorage and returns
// (window, true) when a valid, non-stale entry exists. A window is considered
// stale when its From anchor is more than 366 days in the past relative to now
// — old navigations should not silently lock the user onto a year-old view; in
// that case the caller should fall back to the current period.
//
// Returns (zero, false) when nothing is persisted, the JSON is malformed, the
// resolution is unrecognised, or the window is stale.
func LoadPeriodWindow(now time.Time) (period.Window, bool) {
	v := js.Global().Get("localStorage").Call("getItem", periodWindowStoreID)
	if v.IsNull() || v.IsUndefined() {
		return period.Window{}, false
	}
	var j periodWindowJSON
	if err := json.Unmarshal([]byte(v.String()), &j); err != nil {
		return period.Window{}, false
	}
	res := period.Resolution(j.Res)
	if !res.Valid() {
		return period.Window{}, false
	}
	from, err := time.Parse(time.RFC3339, j.From)
	if err != nil {
		return period.Window{}, false
	}
	to, err := time.Parse(time.RFC3339, j.To)
	if err != nil {
		return period.Window{}, false
	}
	// Reject windows whose From anchor is stale (> 366 days ago). This prevents
	// a year-old "December 2024" window persisting forever after infrequent use.
	if now.Sub(from) > 366*24*time.Hour {
		return period.Window{}, false
	}
	weekStart := time.Weekday(j.WeekStart)
	return period.Window{
		Res:       res,
		From:      from.UTC(),
		To:        to.UTC(),
		WeekStart: weekStart,
	}, true
}

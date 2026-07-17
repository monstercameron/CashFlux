// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

// dashPresetStoreID persists the last dashboard Focus preset the user applied,
// so the "Focus" select can display the ACTIVE view after reloads instead of
// falling back to the "Choose a view…" placeholder while the applied layout
// silently persists underneath (QA task #44 / UX deep dive: the control must
// read "Everything" or "Daily check-in" when that IS the state).
const dashPresetStoreID = "cashflux:dash-preset"

// PersistDashPreset saves the applied preset key ("daily", "payday", …,
// "default"); empty clears it.
func PersistDashPreset(key string) { kvSet(dashPresetStoreID, key) }

// LoadDashPreset returns the last applied preset key, "" when none was ever
// applied (the picker then shows its placeholder).
func LoadDashPreset() string { return kvGet(dashPresetStoreID) }

// dashDailyNudgeStoreID remembers that the one-time "try Daily check-in"
// recommendation was answered (either way), so it never re-appears (#76).
const dashDailyNudgeStoreID = "cashflux:dash-daily-nudge"

// DashDailyNudgeAnswered reports whether the Daily check-in recommendation was
// already accepted or declined.
func DashDailyNudgeAnswered() bool { return kvGet(dashDailyNudgeStoreID) != "" }

// MarkDashDailyNudgeAnswered records the recommendation as handled. Callers
// follow with RequestPersist so the answer survives an immediate reload.
func MarkDashDailyNudgeAnswered() { kvSet(dashDailyNudgeStoreID, "done") }

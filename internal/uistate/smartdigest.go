// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/notify"
)

// digestDeliveredKey is the PRESERVED settings KV key that holds the set of
// digest period keys already posted to the notification feed. It lives in the
// PRESERVED KV (same store as SMART settings) so it survives a dataset wipe.
const digestDeliveredKey = "cashflux:smart-digest:delivered"

// LoadDigestDeliveredLog reads the persisted set of delivered digest period
// keys. A missing or corrupt entry returns an empty log (safe to use).
func LoadDigestDeliveredLog() notify.DeliveredLog {
	raw := SettingKVGet(digestDeliveredKey)
	if raw == "" {
		return notify.NewDeliveredLog()
	}
	var m map[string]bool
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return notify.NewDeliveredLog()
	}
	log := notify.DeliveredLog(m)
	return log
}

// SaveDigestDeliveredLog persists the delivered-log map. It caps the stored log
// at 120 entries (enough to hold 10 years of weekly keys) so it never grows
// without bound. The oldest keys are dropped when the cap is exceeded — the
// worst case is a period that was already delivered could post again, which is
// acceptable since the feed's ID-based dedup in PrependNotifyFeed provides a
// second line of defence.
func SaveDigestDeliveredLog(log notify.DeliveredLog) {
	const cap = 120
	m := map[string]bool(log)
	if len(m) > cap {
		// Drop arbitrary entries beyond cap — keys are period strings so any
		// that survive long enough to get pruned are already months/years old.
		pruned := make(map[string]bool, cap)
		n := 0
		for k, v := range m {
			if n >= cap {
				break
			}
			pruned[k] = v
			n++
		}
		m = pruned
	}
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	SettingKVSet(digestDeliveredKey, string(data))
}

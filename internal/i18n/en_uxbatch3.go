// SPDX-License-Identifier: MIT

// Package i18n — UX review batch 3 keys (tasks #15, #17).
//
// Pattern (mirrors en_mia.go): add new keys here only; init() merges them into the
// english catalog without touching the dirty en.go which is under concurrent WIP.
package i18n

var uxbatch3Keys = Catalog{
	// Dashboard goal widget (#15): tell the same combined story as /goals — the
	// headline % counts saved + set aside, with a breakdown sub-line.
	"dashboard.goalSavedSetAside": "saved %s · set aside %s / %s",
	"dashboard.goalSavedOf":       "saved %s / %s",

	// /health factor value meter (#17): the target-marker tooltip.
	"health.meterTarget": "Target",
}

func init() {
	for k, v := range uxbatch3Keys {
		english[k] = v
	}
}

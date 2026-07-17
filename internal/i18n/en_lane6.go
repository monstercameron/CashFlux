// SPDX-License-Identifier: MIT

package i18n

// lane6Keys holds English strings added by the 2026-07-17 lane-6 remediation
// batch (subscriptions honesty, notifications polish, assistant restructure,
// detection confidence, a11y). Merged via init so the shared en.go is never
// touched by this concurrent lane.
var lane6Keys = Catalog{
	// #49 — local-first "How to cancel": the button files the on-device
	// checklist; the external search is a small secondary link.
	"subs.guideBtnTitle": "File a step-by-step cancellation checklist as a to-do",
	"subs.guideTitle":    "Cancel %s — step-by-step checklist",
	"subs.guideAdded":    "Added a cancellation checklist for %s to your to-dos.",
	"subs.webSearch":     "Web search ↗",
}

func init() {
	for k, v := range lane6Keys {
		english[k] = v
	}
}

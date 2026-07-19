// SPDX-License-Identifier: MIT

package i18n

// goalPaceReasonKeys holds the one-line diagnostic shown under a goal's pace badge
// (2026-07-19): required monthly contribution + the available-money constraint that
// produced the On track / Watch / At risk verdict, so the status explains itself
// instead of being a decorative label. Own file, self-registered, so the shared en.go
// is untouched. %s = required/mo, then the fair-share (or whole-slack for At risk).
var goalPaceReasonKeys = Catalog{
	"goals.paceReasonOnTrack": "Needs %s/mo — within its ~%s/mo share of your free cash.",
	"goals.paceReasonWatch":   "Needs %s/mo — above its ~%s/mo share of your free cash, so it crowds your other goals.",
	"goals.paceReasonAtRisk":  "Needs %s/mo — more than your whole %s/mo of free cash. Extend the date or free up more.",
}

func init() {
	for k, v := range goalPaceReasonKeys {
		english[k] = v
	}
}

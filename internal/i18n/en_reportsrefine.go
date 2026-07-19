// SPDX-License-Identifier: MIT

package i18n

// reportsRefineKeys holds English strings added by the Reports Summary
// refinement pass: the accessible name for a health-factor score and the
// visible "out of 100" scale suffix, so no bare, unlabelled number is ever
// shown on the Annual Review. Merged via init so this file does not touch en.go.
var reportsRefineKeys = Catalog{
	"rptaref.factScoreAria": "%s score: %d out of 100",
	"rptaref.scaleOutOf100": "/100",
}

func init() {
	for k, v := range reportsRefineKeys {
		english[k] = v
	}
}

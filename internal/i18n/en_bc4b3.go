// SPDX-License-Identifier: MIT

package i18n

// bc4b3Keys contains copy for the B3 track-untracked and flex calculation fixes.
var bc4b3Keys = Catalog{
	"track.scanFailed":        "I couldn't scan untracked spending. Close this sheet and try again.",
	"track.calculationFailed": "I couldn't calculate the plan changes. Close this sheet and try again.",
	"track.appliedOne":        "Now tracking %d category.",
	"track.appliedOther":      "Now tracking %d categories.",
	"flex.calculationFailed":  "I couldn't calculate flex spending. Check your exchange rates and transaction amounts.",
}

func init() {
	for key, value := range bc4b3Keys {
		english[key] = value
	}
}

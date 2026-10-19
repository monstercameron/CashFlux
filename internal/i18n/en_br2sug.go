// SPDX-License-Identifier: MIT

package i18n

// br2sugKeys gives budget row recommendations an explicit direction and evidence.
var br2sugKeys = Catalog{
	"sm12.raise":                 "Raise to",
	"sm12.lower":                 "Lower to",
	"sm12.directionalHint":       "%s %s",
	"sm12.reasonNoOutliers":      "Based on a %d-month median.",
	"sm12.reasonOneOutlier":      "%d-month median, 1 unusual month left out.",
	"sm12.reasonManyOutliers":    "%d-month median, %d unusual months left out.",
	"sm12.raiseAccessibleAction": "Raise the %s limit to %s",
	"sm12.lowerAccessibleAction": "Lower the %s limit to %s",
}

func init() {
	for key, value := range br2sugKeys {
		english[key] = value
	}
}

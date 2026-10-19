// SPDX-License-Identifier: MIT

package i18n

// r4aKeys contains period-aware transaction count-line copy.
var r4aKeys = Catalog{
	"r4a.searchAllDates": "%s in %s · Search all dates (%s) →",
	"r4a.futureSummary":  "%s · %s posted · %s scheduled · %s to come",
}

func init() {
	for key, value := range r4aKeys {
		english[key] = value
	}
}

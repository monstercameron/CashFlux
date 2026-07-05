// SPDX-License-Identifier: MIT

package i18n

// subsLapsedKeys holds the English strings for the Subscriptions page's lapsed
// section — detected recurring patterns that stopped charging long ago (e.g. a
// layoff-era COBRA premium). Merged via init so this file does not touch en.go.
var subsLapsedKeys = Catalog{
	"subs.lapsedTitle": "No longer charging",
	"subs.lapsedDesc":  "These looked like subscriptions once, but nothing has charged in a long time — they're kept out of your totals.",
	"subs.lapsedLast":  "last charged %s",
}

func init() {
	for k, v := range subsLapsedKeys {
		english[k] = v
	}
}

// SPDX-License-Identifier: MIT

package i18n

// txnHistoryTimeframeKeys is the copy for the per-transaction history popover's
// timeframe filter (coworker feedback #7: let the history be scoped to a chosen
// window instead of always showing everything). Own file with an init()-merge so
// it lands here, not in the user's working en.go.
var txnHistoryTimeframeKeys = Catalog{
	"txnhistory.timeframe":      "Timeframe",
	"txnhistory.tfAll":          "All time",
	"txnhistory.tf90":           "90 days",
	"txnhistory.tf30":           "30 days",
	"txnhistory.tf7":            "7 days",
	"txnhistory.emptyForWindow": "No changes in this timeframe. Widen it to see older history.",
}

func init() {
	for k, v := range txnHistoryTimeframeKeys {
		english[k] = v
	}
}

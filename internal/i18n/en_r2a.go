// SPDX-License-Identifier: MIT

package i18n

// r2aKeys contains the transaction summary labels introduced by the R2A layout pass.
var r2aKeys = Catalog{
	"r2a.transactions": "transactions",
	"r2a.spent":        "Spent %s",
	"r2a.income":       "Income %s",
	"r2a.net":          "Net %s",
	"r2a.allDates":     "All dates",
	"r2a.month":        "%s %d",
	"r2a.customRange":  "Custom range",
	"r2a.member":       "Member",
	"r2a.upcomingOne":  "%d bill due: %s",
	"r2a.upcomingMany": "%d bills due: %s",
}

func init() {
	for key, value := range r2aKeys {
		english[key] = value
	}
}

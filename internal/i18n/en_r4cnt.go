// SPDX-License-Identifier: MIT

package i18n

// r4cntKeys fixes the transactions review and scheduled-payment copy.
var r4cntKeys = Catalog{
	"fut2.futureSummary":           "%s · %s posted · %s scheduled · ",
	"fut2.dueSuffix":               " due",
	"r2a.spent":                    "Spent %s",
	"r4cnt.loanPayment":            "Loan payment",
	"r4cnt.searchAllDates":         "Search all dates (%s)",
	"transactions.scheduledStatus": "Scheduled",
}

func init() {
	for key, value := range r4cntKeys {
		english[key] = value
	}
}

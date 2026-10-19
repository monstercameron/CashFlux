// SPDX-License-Identifier: MIT

package i18n

var fut2Keys = Catalog{
	"fut2.futureSummary":      "%s · %s posted · %s scheduled · ",
	"fut2.futureEmptyLine":    "No posted transactions. Scheduled items are listed below.",
	"fut2.scheduledMenuLabel": "Scheduled item actions",
	"fut2.scheduledMenuOpen":  "Open in Bills & repeating payments",
}

func init() {
	for key, value := range fut2Keys {
		english[key] = value
	}
}

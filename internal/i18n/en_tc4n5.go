// SPDX-License-Identifier: MIT

package i18n

var tc4n5Keys = Catalog{
	"tc4n5.rule.flow":         "Money direction",
	"tc4n5.rule.flowHint":     "Choose whether money came in or went out.",
	"tc4n5.rule.inflow":       "Money in",
	"tc4n5.rule.outflow":      "Money out",
	"tc4n5.rule.signedAmount": "Signed amount (saved condition)",
}

func init() {
	for k, v := range tc4n5Keys {
		english[k] = v
	}
}

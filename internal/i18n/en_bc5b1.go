// SPDX-License-Identifier: MIT

package i18n

var bc5b1Keys = Catalog{
	"sweep.errorSafe":          "Leftovers could not be calculated. Try again after reviewing your budgets.",
	"sweep.applyError":         "The sweep could not be saved. Your goal was not changed.",
	"sweep.capacityLimited":    "Limited to %s because that is all the linked account can safely earmark.",
	"sweep.ineligibleSaving":   "Savings goals cannot be swept.",
	"sweep.ineligibleRollover": "This budget already carries leftovers forward.",
	"sweep.ineligiblePeriod":   "Only monthly budgets can be swept.",
	"sweep.ineligibleEnvelope": "Envelope budgets already carry leftovers forward.",
	"sweep.poolRolloverOff":    "Sweep is off while leftovers roll over.",
}

func init() {
	for key, value := range bc5b1Keys {
		english[key] = value
	}
}

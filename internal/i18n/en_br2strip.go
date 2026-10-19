// SPDX-License-Identifier: MIT

package i18n

var br2StripKeys = Catalog{
	"bud2.excludesOneBudget":         "(excludes 1 budget)",
	"bud2.excludesManyBudgets":       "(excludes %d budgets)",
	"bud2.cantCalculate":             "Can't calculate · Fix",
	"bud2.fixBudget":                 "Fix",
	"bud2.fixRates":                  "Settings · exchange rates",
	"bud2.projectionOver":            "On pace to overspend by %s",
	"bud2.freeAfterCommitted":        "%s free after %s committed",
	"bud2.freeAfterCommittedUnknown": "Free after commitments is unknown.",
	"bud2.startsDate":                "Starts %s %d",
	"bud2.rolloverMonth":             "Month",
	"bud2.rolloverLimit":             "Limit",
	"bud2.rolloverCarriedIn":         "Carried in",
	"bud2.rolloverSpent":             "Spent",
	"bud2.rolloverCarriedOut":        "Carried out",
	"bud2.rolloverStartAssumed":      "The start of this rollover history is assumed.",
	"bud2.rolloverDetail":            "Rollover detail",
	"bud2.setStart":                  "Set start",
}

func init() {
	for key, value := range br2StripKeys {
		english[key] = value
	}
}

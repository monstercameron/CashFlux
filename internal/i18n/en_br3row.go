// SPDX-License-Identifier: MIT

package i18n

var br3rowKeys = Catalog{
	"br3row.endedOver":          "Ended %s over",
	"br3row.committedDominates": "On track — %s of this is bills",
	"br3row.unbudgetedOthers":   "%s (others)",
	"br3row.cadenceWeek":        "/ wk",
	"br3row.cadenceQuarter":     "/ qtr",
	"br3row.cadenceYear":        "/ yr",
	"br3row.goalMismatch":       "This goal is linked to a different category",
	"br3row.goalAvailable":      "%s available from",
	"br3row.restPerDay":         "About %s per day left in this budget",
	"br3row.effectiveBaseCarry": "Effective this month %s (base %s %s carried)",
	"br3row.effectiveBase":      "Effective this month %s (base limit)",
}

func init() {
	for key, value := range br3rowKeys {
		english[key] = value
	}
}

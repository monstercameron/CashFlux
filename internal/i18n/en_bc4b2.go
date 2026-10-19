// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"bc4b2.assumedStart":     "Rollover starts from an estimated period: %s.",
		"bc4b2.setStart":         "Choose the first period",
		"bc4b2.startPeriod":      "First funded period",
		"bc4b2.saveStart":        "Save start period",
		"bc4b2.startSaved":       "Rollover start saved for %s.",
		"bc4b2.chooseStartError": "Choose a month to save the rollover start.",
		"bc4b2.shortBy":          "short by %s",
		"bc4b2.targetMet":        "target met",
		"bc4b2.needsFunding":     "Needs funding",
		"bc4b2.fundFromOther":    "Fund from another budget",
		"bc4b2.increaseTarget":   "Increase target",
	} {
		english[key] = value
	}
}

// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"who2.label":        "Who",
		"who2.unassigned":   "Unassigned",
		"who2.sharedBy":     "Shared by %s",
		"who2.chooseOwners": "Choose owners",
		"who2.setOwnersTo":  "Set owners to…",
		"who2.setOwners":    "Set owners",
	} {
		english[key] = value
	}
}

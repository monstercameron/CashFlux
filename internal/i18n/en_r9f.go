// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"r9f.tools":                   "Tools",
		"r9f.deleteTransaction":       "Delete transaction…",
		"r9f.transferEditUnavailable": "Transfer conversion needs to update both accounts together and is unavailable here.",
	} {
		english[key] = value
	}
}

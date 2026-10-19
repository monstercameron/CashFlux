// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"r9f.tools":                   "Tools",
		"r9f.deleteTransaction":       "Delete transaction",
		"r9f.transferEditUnavailable": "To mark this as a transfer, choose the other account in More details.",
	} {
		english[key] = value
	}
}

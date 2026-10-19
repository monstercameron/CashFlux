// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"budgets.countOne":  "1 budget",
		"budgets.countMany": "%d budgets",
	} {
		english[key] = value
	}
}

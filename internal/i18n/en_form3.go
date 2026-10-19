// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"quickAdd.noCategory":     "No category",
		"transactions.noCategory": "No category",
	} {
		english[key] = value
	}
}

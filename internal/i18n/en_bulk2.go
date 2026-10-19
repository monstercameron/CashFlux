// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"bulk2.owners":     "Owners",
		"bulk2.unassigned": "Unassigned",
		"bulk2.apply":      "Apply",
	} {
		english[key] = value
	}
}

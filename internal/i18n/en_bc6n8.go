// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"budgets.autoErrorGeneric": "Auto budget could not be calculated. Try again.",
		"budgets.allocationError":  "We could not calculate this income split.",
	} {
		english[key] = value
	}
}

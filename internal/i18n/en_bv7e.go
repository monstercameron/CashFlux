// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"bv7e.billsFreeAfterRequired":        "%s free after %s in committed bills",
		"bv7e.billsFreeAfterRequiredAndPaid": "%s free after %s in committed bills, including %s already paid",
	} {
		english[key] = value
	}
}

// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, text := range (Catalog{
		"enc34.checkpointFailed": "Couldn't save a restore point, so nothing was changed. Try again.",
		"enc34.saving":           "Saving restore point…",
	}) {
		english[key] = text
	}
}

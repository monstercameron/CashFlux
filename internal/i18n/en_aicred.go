// SPDX-License-Identifier: MIT

package i18n

// AI credential copy: the API key is a device credential and is never part of
// a household dataset, sync payload or exported file.
func init() {
	for key, value := range map[string]string{
		"settings.aiKeyTrust":      "Your OpenAI key stays on this device, is encrypted when App Lock is on, and is not included in exports or sync. CashFlux sends it directly to OpenAI when you ask.",
		"settings.rememberKey":     "Remember my key on this device",
		"settings.rememberKeyNote": "When on, your key is saved in this device's private credential store. When off, it stays in memory only and is removed from this device's saved credentials.",
	} {
		english[key] = value
	}
}

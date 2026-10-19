// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, text := range (Catalog{
		"enc31.retry":         "Finish App Lock change",
		"enc31.title":         "CashFlux couldn't open your protected data",
		"enc31.preserved":     "Your saved data is being kept. Unlock it to continue.",
		"enc31.passcode":      "Passcode",
		"enc31.unlock":        "Unlock",
		"enc31.passkey":       "Unlock with passkey",
		"enc31.recovery":      "Use recovery key",
		"enc31.recoveryLabel": "Recovery key",
		"enc31.usePasscode":   "Use passcode",
		"enc31.help":          "Get help",
		"enc31.helpText":      "Use the passcode or recovery key for this device. If neither works, keep this browser's saved data and contact your CashFlux support contact. Nothing will be reset here.",
		"enc31.failed":        "CashFlux couldn't unlock your data. Check your passcode or recovery key. Your saved data has been kept.",
		"enc31.opening":       "Opening your protected data...",
		"enc31.repair":        "Repair this device's access",
		"enc31.repairPrompt":  "Your data is unlocked. You can repair this device's access now, or continue without changing it.",
		"enc31.repairFailed":  "CashFlux couldn't repair this device's access. Your saved data has been kept. You can continue and try again later.",
		"enc31.continue":      "Continue",
	}) {
		english[key] = text
	}
}

// SPDX-License-Identifier: MIT

package i18n

// webauthnKeys holds the English strings for the C282 WebAuthn PRF passkey
// unlock feature. Defined in its own file and merged via init so this does not
// touch the user-WIP en.go; mirrors the en_applockfix.go pattern.
var webauthnKeys = Catalog{
	// Gate unlock button — shown only when a credential is registered.
	"webauthn.unlockBtn":   "Unlock with passkey",
	"webauthn.unlockTitle": "Verifying with your fingerprint, face, or device PIN…",
	"webauthn.unlockFail":  "Passkey unlock failed — please enter your passcode instead.",

	// Passkey manager modal.
	"webauthn.setupTitle":   "Passkey",
	"webauthn.setupDesc":    "Add your fingerprint, face, or device PIN as a faster way to unlock CashFlux. Your passcode always remains as a fallback.",
	"webauthn.registerBtn":  "Register passkey",
	"webauthn.removeBtn":    "Remove passkey",
	"webauthn.removeConfirm": "Remove your passkey registration? You can still unlock with your passcode at any time.",
	"webauthn.setupOK":      "Passkey registered — you can now unlock with your fingerprint or device PIN.",
	"webauthn.setupFail":    "Passkey registration failed. Your passcode remains fully functional.",
	"webauthn.removedOK":    "Passkey removed.",
	"webauthn.notAvailable": "Passkeys are not supported on this device or browser.",
	"webauthn.manageBtn":    "Manage passkey",
}

func init() {
	for k, v := range webauthnKeys {
		english[k] = v
	}
}

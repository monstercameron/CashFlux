// SPDX-License-Identifier: MIT

package i18n

// appLockRecoveryKeys contains copy for the explicit unreadable-vault recovery
// path during a passcode change.
var appLockRecoveryKeys = Catalog{
	"vault.changeDiscardConfirm": "The saved bank logins cannot be read with the current passcode. Change the passcode and discard this vault? A recovery copy of its raw data will be saved on this device first.",
	"vault.changeDiscardButton":  "Change passcode and discard vault",
	"vault.quarantineFailed":     "The unreadable bank-login vault could not be safely copied. Your passcode was not changed.",
}

func init() {
	for k, v := range appLockRecoveryKeys {
		english[k] = v
	}
}

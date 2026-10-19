// SPDX-License-Identifier: MIT

package i18n

// vaultRekeyKeys contains the confirmation and failure copy for app-lock vault
// migration. It is merged here so the shared English catalog stays untouched.
var vaultRekeyKeys = Catalog{
	"vault.disableConfirm":       "Turning off the lock removes %d saved bank logins from this device.",
	"vault.disableConfirmButton": "Turn off lock and remove logins",
	"vault.rekeyFailed":          "The saved bank logins could not be moved. Your passcode was not changed; try again after checking the vault.",
	"vault.disableFailed":        "The saved bank logins could not be read, so the lock is still on.",
}

func init() {
	for k, v := range vaultRekeyKeys {
		english[k] = v
	}
}

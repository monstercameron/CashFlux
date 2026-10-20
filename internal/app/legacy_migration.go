// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"strings"

	"github.com/monstercameron/CashFlux/internal/applock"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// encryptedWorkspaceKey is scoped independently of mutable sync bookmarks.
// The authoritative latch is on the server; this remembers authenticated
// encrypted responses and acknowledgements as additional device protection.
func encryptedWorkspaceKey(workspaceID string) string {
	return encryptedWorkspaceKeyFor(currentDatasetServerID(), signedInUserID(), workspaceID)
}

func encryptedWorkspaceKeyFor(serverID, userID, workspaceID string) string {
	scope, _ := json.Marshal([3]string{serverID, userID, workspaceID})
	return "cashflux:server-encrypted:" + datasetHash(scope)
}

func serverWorkspaceWasEncrypted(workspaceID string) bool {
	return lsGet(encryptedWorkspaceKey(workspaceID)) == "true"
}

func rememberServerWorkspaceEncrypted(workspaceID string) {
	lsSet(encryptedWorkspaceKey(workspaceID), "true")
}

func legacyWorkspaceNeedsProtection() bool {
	meta := loadSyncMeta(activeWorkspaceID())
	return meta.LegacyPlaintext && meta.Owner == signedInUserID() && meta.Server == currentDatasetServerID()
}

func markHostedLegacyProtection() {
	setHostedHydration(hostedHydrationStatus{Required: true, Phase: hostedHydrationProtect})
	requireDatasetEncryption()
}

func legacyPasscodeError(passcode string) string {
	switch applock.PasscodeStrength(passcode) {
	case applock.StrengthTooShort:
		return "applock.tooShort"
	case applock.StrengthWeak:
		return "applock.tooWeak"
	default:
		return ""
	}
}

// enableHostedLegacyProtection installs the passcode only after the legacy
// server data is durable locally. The ordinary queue captures its pulled CAS
// version and encrypts it; there is no forced overwrite or plaintext retry.
func enableHostedLegacyProtection(passcode string) {
	passcode = strings.TrimSpace(passcode)
	if currentHostedHydration().Phase != hostedHydrationProtect || passcode == "" || legacyPasscodeError(passcode) != "" || appLockPasscodeChangeInProgress {
		return
	}
	appLockPasscodeChangeInProgress, suspendAutosave = true, true
	setHostedHydration(hostedHydrationStatus{Required: true, Phase: hostedHydrationVerifying})
	go func() {
		yieldToBrowser()
		salt := newSalt()
		if salt == "" {
			appLockPasscodeChangeInProgress, suspendAutosave = false, false
			setHostedHydration(hostedHydrationStatus{Required: true, Phase: hostedHydrationProtect, Message: uistate.T("migration.lockFailed")})
			return
		}
		err := runSecureLockTransition(passcode, loadAppLock().WithPasscode(passcode, salt, 0, ""), false)
		if err != nil {
			setHostedHydration(hostedHydrationStatus{Required: true, Phase: hostedHydrationProtect, Message: uistate.T("migration.lockFailed")})
		}
		// Reuse App Lock's journal recovery gate if the key wrap committed but
		// a later record write failed. Retrying must not strand a half-transition.
		finishSecureAppLockChange(err, false, func() {
			markHostedHydrationReady()
			finishAppLockEnable()
		})
	}()
}

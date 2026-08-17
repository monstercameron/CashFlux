// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/backup"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// ─── LF-2: a backup you can hand to someone else's cloud ─────────────────────
//
// `backupEverything` writes the whole install as plain JSON. That is the right
// default — a backup you cannot open without the app is a backup you cannot
// trust — but it means the file carries every balance, every payee and every
// note in the clear, and the natural places to put a backup are exactly the
// places a plaintext one should not go: a sync folder, an email to yourself, a
// USB stick in a drawer.
//
// So this is the same envelope, encrypted under a passphrase the user chooses.
//
// The passphrase is deliberately NOT the app-lock passcode. Three reasons, and
// the third is the one that decided it:
//
//   - A four-digit lock passcode is not a backup passphrase. The lock protects a
//     device someone must already be holding; a backup file protects itself
//     wherever it ends up, so it needs a real secret.
//   - The lock is optional. Half the installs have no passcode to encrypt with.
//   - A backup exists to be restored on a DIFFERENT device — often one where the
//     app is not installed yet, and therefore has no lock configured at all.
//     Keying the file to a credential the destination cannot have is a backup
//     that cannot be restored, which is the only unforgivable bug in a backup.

// minBackupPassphrase is the shortest accepted passphrase.
//
// Twelve, not four. This is not a device gate someone has to be holding — the
// file is the whole dataset and it will sit in a sync folder for years. The
// number is a floor on the cheapest attack, not a claim that twelve characters
// is strong; the copy asks for a phrase rather than a password for that reason.
const minBackupPassphrase = 12

// backupEverythingEncrypted is backupEverything with the envelope sealed under a
// passphrase the user supplies (LF-2).
func backupEverythingEncrypted() {
	promptModal(uistate.T("backup.passphrasePrompt"), "", func(pass string) {
		pass = strings.TrimSpace(pass)
		if pass == "" {
			return // cancelled
		}
		if len([]rune(pass)) < minBackupPassphrase {
			paletteNotify(uistate.T("backup.passphraseShort", minBackupPassphrase), true)
			return
		}
		// Ask twice. A mistyped passphrase on a backup is not recoverable and the
		// user finds out at the worst possible moment — restoring after losing the
		// device. This is the one confirmation prompt that genuinely earns itself.
		promptModal(uistate.T("backup.passphraseConfirm"), "", func(again string) {
			if strings.TrimSpace(again) != pass {
				paletteNotify(uistate.T("backup.passphraseMismatch"), true)
				return
			}
			plain, err := marshalFullBackup()
			if err != nil {
				paletteNotify(uistate.T("backup.everythingErr"), true)
				return
			}
			encryptDataset(plain, pass, func(env []byte, encErr error) {
				if encErr != nil {
					paletteNotify(uistate.T("backup.encryptErr"), true)
					return
				}
				downloadBytes("cashflux-backup.cfbak", "application/octet-stream", env)
				recordBackupNow()
				paletteNotify(uistate.T("backup.encryptedDone"), false)
			})
		})
	})
}

// restoreFromBackupEncrypted restores either an encrypted or a plain backup.
//
// It sniffs the file rather than asking which kind it is: the user knows they
// have "a backup", not which format it happens to be in, and a picker that makes
// them choose wrongly produces an error that looks like a corrupt file.
func restoreFromBackupEncrypted() {
	pickFile("", func(data []byte) {
		if !cryptobox.IsEnvelope(data) {
			// Plain backup — hand it to the existing path so there is one restore
			// implementation, not two that can drift.
			applyPlainBackup(data)
			return
		}
		env, ok := cryptobox.Parse(data)
		if !ok {
			paletteNotify(uistate.T("backup.restoreErr"), true)
			return
		}
		promptModal(uistate.T("backup.unlockPrompt"), "", func(pass string) {
			if strings.TrimSpace(pass) == "" {
				return // cancelled
			}
			decryptDataset(env, pass, func(plain []byte, err error) {
				if err != nil {
					// A wrong passphrase and a corrupted file are indistinguishable to
					// AES-GCM — both fail the auth tag. Saying "wrong passphrase, or the
					// file is damaged" is the honest reading; claiming either alone
					// would send the user down the wrong path half the time.
					paletteNotify(uistate.T("backup.unlockFailed"), true)
					return
				}
				applyPlainBackup(plain)
			})
		})
	})
}

// applyPlainBackup parses and applies a decrypted (or never-encrypted) backup,
// behind the same destructive confirm the plain path uses.
func applyPlainBackup(data []byte) {
	env, err := backup.UnmarshalEnvelope(data)
	if err != nil {
		paletteNotify(uistate.T("backup.restoreErr"), true)
		return
	}
	// The same before-and-after the plain path shows. Sharing the BUILDER rather
	// than the literal string is the point: this function's own doc comment says
	// it is behind "the same destructive confirm", and until now that was true of
	// the wording only — the two would have drifted the moment either changed.
	confirmModal(restoreConfirmMessage(env), true, func(ok bool) {
		if !ok {
			return
		}
		applyBackup(env)
	})
}

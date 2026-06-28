// SPDX-License-Identifier: MIT

package i18n

// syncConflictKeys holds the English strings for the sync-conflict resolve modal
// (C309 / #464). Merged via init so this file does not touch en.go; mirrors the
// en_profileswitch.go / en_webauthn.go pattern.
var syncConflictKeys = Catalog{
	// Modal title and description.
	"syncConflict.title":       "Sync Conflict",
	"syncConflict.description": "This device has local changes that conflict with a newer version on the server. Choose how to resolve this.",
	// Keep-local option.
	"syncConflict.keepLocalTitle": "Keep my changes",
	"syncConflict.keepLocalDesc":  "Your device's version will overwrite the server copy. Any edits made on other devices since the conflict will be replaced.",
	"syncConflict.keepLocalBtn":   "Keep my changes",
	// Use-server option.
	"syncConflict.useServerTitle": "Use server version",
	"syncConflict.useServerDesc":  "The server copy replaces your local changes. Your unsynced edits on this device will be permanently discarded.",
	"syncConflict.useServerBtn":   "Use server version",
	// Close / cancel.
	"syncConflict.close": "Cancel",
	// Post-resolution notices (shown in the notification feed).
	"sync.conflictResolvedKeepLocal": "Your changes were force-pushed to the server.",
	"sync.conflictResolvedUseServer": "Server version applied. Reloading…",
}

func init() {
	for k, v := range syncConflictKeys {
		english[k] = v
	}
}

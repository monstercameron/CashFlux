// SPDX-License-Identifier: MIT

package i18n

// syncPageKeys are the strings for the top-level /sync page — the promoted
// connect-a-backend surface (2026-07-17). Kept separate from en.go (concurrent
// WIP) like the other feature key files. Shared control labels (server URL/token,
// Test/Sync buttons, server mode) reuse the existing settings.* keys so the two
// surfaces read identically.
var syncPageKeys = Catalog{
	"nav.sync":             "Sync",
	"screen.syncSub":       "Connect a backend to sync your data across devices",
	"sync.pageTitle":       "Sync & backup",
	"sync.intro":           "Connect CashFlux to a backend server to keep your data in sync across your devices. This is optional and local-first — turn it off and the app runs entirely on this device, with nothing uploaded.",
	"sync.connectToggle":   "Sync with a backend server",
	"sync.offHint":         "Off — CashFlux is running entirely on this device. Nothing is uploaded.",
	"sync.whatSyncs":       "What syncs: your full database (accounts, transactions, budgets, goals — everything) and your attached files.",
	"sync.encOn":           "🔒 End-to-end encrypted. Your data is encrypted on this device with your passcode before it's uploaded — the server only ever stores ciphertext it can't read.",
	"sync.encOff":          "⚠ Not end-to-end encrypted. Without a passcode lock, your data is uploaded and stored on the server as readable JSON. Turn on a passcode to encrypt everything before it leaves this device.",
	"sync.encEnable":       "Turn on passcode lock",
	"sync.encTitle":        "Privacy",
	"sync.pendingCount":    "%d change(s) waiting to upload",
	"sync.cloudSignInHint": "Cloud mode signs in with Google or GitHub — use the sign-in buttons on the Cloud settings tab.",
	"sync.syncingNow":      "Syncing now…",
	"sync.openSettings":    "Open Cloud settings",
	"sync.manageMore":      "Subscription, sign-in, and devices",
}

func init() {
	for k, v := range syncPageKeys {
		english[k] = v
	}
}

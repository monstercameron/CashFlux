// SPDX-License-Identifier: MIT

package i18n

// syncPageKeys are the strings for the top-level /sync page — the promoted
// connect-a-backend surface (2026-07-17). Kept separate from en.go (concurrent
// WIP) like the other feature key files. Shared control labels (server URL/token,
// Test/Sync buttons, server mode) reuse the existing settings.* keys so the two
// surfaces read identically.
var syncPageKeys = Catalog{
	"nav.sync":           "Sync",
	"screen.syncSub":     "Connect a backend to sync your data across devices",
	"sync.pageTitle":     "Sync & backup",
	"sync.intro":         "Connect CashFlux to a backend server to keep your data in sync across your devices. This is optional and local-first — turn it off and the app runs entirely on this device, with nothing uploaded.",
	"sync.connectToggle": "Sync with a backend server",
	"sync.offHint":       "Off — CashFlux is running entirely on this device. Nothing is uploaded.",
	"sync.whatSyncs":     "What syncs: your full database (accounts, transactions, budgets, goals — everything) and your attached files.",
	"sync.encOn":         "🔒 End-to-end encrypted. Your data is encrypted on this device with your passcode before it's uploaded — the server only ever stores ciphertext it can't read.",
	"sync.encOff":        "⚠ Not end-to-end encrypted. Without a passcode lock, your data is uploaded and stored on the server as readable JSON. Turn on a passcode to encrypt everything before it leaves this device.",
	"sync.encEnable":     "Turn on passcode lock",
	"sync.encTitle":      "Privacy",
	"sync.pendingCount":  "%d change(s) waiting to upload",
	"sync.syncingNow":    "Syncing now…",
	"sync.openSettings":  "Manage subscription & devices",
	"sync.manageMore":    "Billing, plan, and linked devices",

	// Capability-aware connect flow (2026-07-23): the server address is the one
	// thing every modality needs; everything after it is chosen by what that
	// server actually reports supporting, not by a manually-picked mode.
	"sync.serverAddressIntro":  "Point this at your CashFlux server — your own, someone else's, or CashFlux Cloud.",
	"sync.useDifferentAddress": "Not this server? Enter a different address",
	"sync.discoveryChecking":   "Checking what this server supports…",
	"sync.discoveryOK":         "Connected.",
	"sync.tokenFieldPrimary":   "This server uses a fixed access token — paste it below.",
	"sync.advancedTokenToggle": "Paste an access token instead",
}

func init() {
	for k, v := range syncPageKeys {
		english[k] = v
	}
}

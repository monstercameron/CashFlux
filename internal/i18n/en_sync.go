// SPDX-License-Identifier: MIT

package i18n

// syncPageKeys are the strings for the top-level /sync page — the promoted
// connect-a-backend surface (2026-07-17). Kept separate from en.go (concurrent
// WIP) like the other feature key files. Shared control labels (server URL/token,
// Test/Sync buttons, server mode) reuse the existing settings.* keys so the two
// surfaces read identically.
var syncPageKeys = Catalog{
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
	"sync.statusDetail":  "Reason: %s",
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
	"sync.otherWaysHeading":    "Other ways to sign in",

	// Local/Remote/Commercial segments (2026-07-24 unification): one connection
	// surface instead of a plain Cloud/Self-hosted toggle, since "your own server
	// at a known address," "someone else's server," and "CashFlux's paid service"
	// are different trust postures, not just different URLs.
	"sync.segmentLabel":          "Where's this server?",
	"sync.segmentLocal":          "Local",
	"sync.segmentRemote":         "Remote",
	"sync.segmentCommercial":     "CashFlux Cloud",
	"sync.segmentLocalHint":      "Your own server, running alongside this app or on your network. CashFlux tries to find it automatically.",
	"sync.segmentRemoteHint":     "A server somewhere else — yours or someone else's. Type its address; nothing is auto-detected.",
	"sync.segmentCommercialHint": "CashFlux's own hosted, subscription-based service. No server to run yourself.",
	"sync.remoteTrustDisclosure": "You're connecting to a server you don't run yourself. Once you sign in, everything you sync — transactions, balances, everything — is visible to whoever operates it. Only continue if you trust them.",
}

func init() {
	for k, v := range syncPageKeys {
		english[k] = v
	}
}

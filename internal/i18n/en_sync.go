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

	// Account/workspace mismatch (C696, C697). "workspace not found" is the
	// server correctly refusing a workspace the signed-in account does not own.
	// Retrying cannot fix it, so the copy asks for a decision instead of
	// reporting a failure — and states the facts the decision needs, because
	// the old pane said only "1 change(s) waiting to upload / Reason: workspace
	// not found", which names neither the identity nor the workspace involved.
	"sync.rebindChip":          "Needs a decision",
	"sync.rebindTitle":         "This copy belongs to a different account",
	"sync.rebindReason":        "The account you are signed in as does not own this workspace, so nothing can be uploaded until you choose what to do.",
	"sync.rebindSignInReason":  "Sign in to upload the changes waiting on this device.",
	"sync.rebindExplain":       "Your changes are safe on this device. Nothing has been lost and nothing will be sent until you pick one of these.",
	"sync.rebindSignedInAs":    "Signed in as",
	"sync.rebindServer":        "Server",
	"sync.rebindWorkspace":     "This workspace",
	"sync.rebindLastSuccess":   "Last successful sync",
	"sync.rebindNeverSynced":   "never",
	"sync.rebindPendingDetail": "%d change(s) waiting, oldest queued %s",
	"sync.rebindUnknownUser":   "unknown — the server has not said who this session belongs to",
	"sync.rebindRemoteTitle":   "Workspaces this account can reach",
	"sync.rebindRemoteEmpty":   "This account owns no workspaces on the server yet.",
	"sync.rebindPickPrompt":    "Choose the workspace this data belongs to. Your local copy is exported first, and nothing is uploaded until the mapping is confirmed.",
	"sync.rebindAction":        "Move this data to…",
	"sync.rebindConfirm":       "Use this workspace",
	"sync.rebindCancel":        "Not now",
	"sync.rebindSignIn":        "Sign in to the other account",
	"sync.rebindKeepLocal":     "Keep this device local-only",
	"sync.rebindKeepLocalHint": "Turns cloud sync off here. Your data stays on this device, exactly as it is.",
	"sync.rebindExport":        "Export a backup first",
	"sync.rebindRetry":         "Try uploading again",
	"sync.rebindDone":          "Moved. This device now syncs to %s.",
	"sync.rebindFailed":        "Could not move the data: %s",
	"sync.rebindBackupSaved":   "Backup exported. Keep it until you have checked the result.",

	// Replace / merge (C695). The choice only appears once the target workspace
	// is known to ALSO hold records: with nothing on the other side there is
	// nothing to decide, and offering the choice anyway would invent a dilemma.
	"sync.mergeChoiceTitle":   "That workspace already has data",
	"sync.mergeChoiceIntro":   "Both copies have records in them. Choose what happens to the copy on the server.",
	"sync.mergeCompare":       "This device: %s. On the server: %s.",
	"sync.mergeWouldAdd":      "Merging would add %d record(s) the server does not have.",
	"sync.mergeWouldConflict": "%d record(s) exist in both copies with different contents.",
	"sync.mergeNoOverlap":     "The two copies have no records in common.",
	"sync.mergeKeepBoth":      "Keep the records from both",
	"sync.mergeReplace":       "Replace the server's copy with this device's",
	"sync.mergeReplaceHint":   "The server's copy is archived first and can be restored by an operator.",
	"sync.mergeConflictWins":  "When a record differs in both copies",
	"sync.mergeWinsLocal":     "Use this device's version",
	"sync.mergeWinsRemote":    "Use the server's version",
	"sync.mergeDone":          "Merged. %d record(s) added, %d resolved by your choice.",
	"sync.mergeReplaced":      "The server's copy now matches this device.",
	"sync.mergeFailed":        "Could not finish: %s",
	"sync.mergeChecking":      "Comparing the two copies…",
	"sync.syncingNow":         "Syncing now…",
	"sync.openSettings":       "Manage subscription & devices",
	"sync.manageMore":         "Billing, plan, and linked devices",

	// Capability-aware connect flow (2026-07-23): the server address is the one
	// thing every modality needs; everything after it is chosen by what that
	// server actually reports supporting, not by a manually-picked mode.
	"sync.serverAddressIntro":  "Point this at your CashFlux server — your own, someone else's, or CashFlux Cloud.",
	"sync.useDifferentAddress": "Not this server? Enter a different address",
	"sync.discoveryChecking":   "Checking what this server supports…",
	"sync.discoveryOK":         "Connected.",
	"sync.tokenFieldPrimary":   "This server uses a fixed access token.",
	"sync.advancedTokenToggle": "Paste an access token instead",
	"sync.otherWaysHeading":    "Other ways to sign in",
	// Shown instead of "Connected." on a server where an activation code is the only
	// way in and this device doesn't have a session yet. "Connected." is true of the
	// SERVER (an unauthenticated capability probe found it) but reads as "you're
	// done", which is the opposite of what someone still needing a code should hear.
	"sync.discoveryNeedsActivation": "Found your server. New accounts require administrator approval.",
	// One disclosure for every secondary sign-in path, replacing three competing
	// top-level links on a server where only the activation code actually works.
	"sync.moreWaysToggle": "More ways to sign in",
	// Shown when the server holds an encrypted snapshot this device has no passcode
	// for. The old state message ("unlock to sync encrypted data") named a action the
	// user could not take: unlocking runs from the passcode GATE, which never appears
	// on a device that has no lock set. Setting the matching passcode is the recovery.
	"sync.lockedTitle": "This account's data is encrypted",
	"sync.lockedHint":  "It was locked with a passcode on another device. Turn on the passcode lock here and enter that same passcode — this device can't read the data until it matches.",
	// Hosted first-boot gate. The financial shell stays unmounted until this flow
	// hydrates the account or confirms that the server is empty.
	"hostedHydration.title":       "Opening your CashFlux",
	"hostedHydration.intro":       "CashFlux is checking this account before it opens your budget. Sample data is never loaded on this server.",
	"hostedHydration.loading":     "Checking the server for your existing data...",
	"hostedHydration.verifying":   "Checking that App Lock passcode...",
	"hostedHydration.applying":    "Decrypting and saving your synced data...",
	"hostedHydration.lockedTitle": "Enable App Lock to decrypt your data",
	"hostedHydration.lockedHint":  "This account was encrypted with App Lock on another device. Enter and confirm that same passcode. CashFlux verifies it against the server data before saving the lock on this device.",
	"hostedHydration.enableLock":  "Enable App Lock and open my data",
	"hostedHydration.retry":       "Try loading again",
	// Reported after a sign-in finds workspaces this device didn't know about.
	"sync.workspacesAdded": "Added %d workspace(s) from your account. Switch to one to load its data.",

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

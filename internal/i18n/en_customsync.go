// SPDX-License-Identifier: MIT

package i18n

// customSyncKeys are the strings for the "Custom Sync" phone-number
// enrollment card on the /sync page (TODOS.md C420, plus the client half of
// C419). Kept in its own file (concurrent WIP on en_sync.go/en.go) like the
// other feature key files.
var customSyncKeys = Catalog{
	"customSync.title":               "Custom Sync",
	"customSync.intro":               "Sign in with your phone number instead — no server address or token to copy. We'll text you a one-time code.",
	"customSync.phoneLabel":          "Phone number",
	"customSync.phonePlaceholder":    "+1 555 123 4567",
	"customSync.phoneRequired":       "Enter your phone number first.",
	"customSync.sendCode":            "Text me a code",
	"customSync.sending":             "Sending…",
	"customSync.codeSent":            "Code sent — check your texts.",
	"customSync.codeSentTo":          "We texted a code to %s.",
	"customSync.codeLabel":           "Verification code",
	"customSync.codePlaceholder":     "123456",
	"customSync.codeRequired":        "Enter the code we texted you.",
	"customSync.verifyCode":          "Verify and sign in",
	"customSync.verifying":           "Verifying…",
	"customSync.startOver":           "Use a different number",
	"customSync.useDifferentPhone":   "Use a different number",
	"customSync.connectFailed":       "Couldn't reach the backend server.",
	"customSync.sendFailed":          "Sending the code failed. Check the number and try again.",
	"customSync.verifyFailed":        "That code didn't work. Check it and try again.",
	"customSync.signedIn":            "You're signed in.",
	"customSync.signedInAs":          "Signed in with %s.",
	"customSync.checkingEligibility": "Checking your cloud sync eligibility…",
	"customSync.gatedBillingLapsed":  "Your cloud plan needs a refresh before you can add phone sign-in. Update your billing to continue.",
	"customSync.gatedAdminSuspended": "This account is suspended, so phone sign-in isn't available right now.",
	"customSync.gatedPlanTier":       "Phone sign-in isn't included on your current plan. Upgrade to turn it on.",
	"customSync.gatedGeneric":        "Cloud sync isn't available on this account right now.",
	"customSync.upgradeCta":          "Manage plan",
	// setupCode* covers the optional single-use invite code some private/
	// embedded deployments require to create a brand-new account
	// (Config.SetupCode/TODOS.md C445). Blank on every ordinary deployment —
	// the field is low-key and optional-looking on purpose.
	"customSync.setupCodeLabel":       "Setup code",
	"customSync.setupCodePlaceholder": "Only if you were given one",
}

func init() {
	for k, v := range customSyncKeys {
		english[k] = v
	}
}

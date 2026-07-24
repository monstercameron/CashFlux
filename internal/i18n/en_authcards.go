// SPDX-License-Identifier: MIT

package i18n

// authCardsKeys are the strings for the username/password fallback card and
// the "link a new device" pairing-code card on the /sync page (TODOS.md
// C421/C422 client UI). Kept in its own file (concurrent WIP on other sync
// key files) like the other feature key files.
var authCardsKeys = Catalog{
	// Password fallback — collapsed link under the phone card.
	"authCards.usePasswordInstead": "Use a password instead",

	// Password fallback — expanded form.
	"authCards.passwordTitle":               "Sign in with a password",
	"authCards.passwordIntro":               "No phone number handy? Use a username and password instead.",
	"authCards.modeGroupLabel":              "Sign-in mode",
	"authCards.modeLogin":                   "Log in",
	"authCards.modeRegister":                "Create account",
	"authCards.usernameLabel":               "Username",
	"authCards.usernamePlaceholder":         "yourname",
	"authCards.passwordLabel":               "Password",
	"authCards.passwordPlaceholderLogin":    "Password",
	"authCards.passwordPlaceholderRegister": "At least 8 characters",
	"authCards.loginSubmit":                 "Log in",
	"authCards.loggingIn":                   "Logging in…",
	"authCards.registerSubmit":              "Create account",
	"authCards.registering":                 "Creating account…",
	"authCards.usernameRequired":            "Enter a username.",
	"authCards.passwordRequired":            "Enter a password.",
	"authCards.passwordTooShort":            "Password must be at least %d characters.",
	"authCards.connectFailed":               "Couldn't connect to the server. Check the address and try again.",
	"authCards.loginFailed":                 "That username or password didn't work.",
	"authCards.registerFailed":              "Couldn't create that account — the username may already be taken.",
	"authCards.loggedInAs":                  "Signed in as %s.",
	"authCards.useDifferentAccount":         "Use a different account",

	// Password fallback — one-time recovery code shown once at Register.
	"authCards.registerSuccess": "Account created — you're signed in.",
	"authCards.recoveryTitle":   "Save your recovery code",
	"authCards.recoveryIntro":   "There's no email on file to reset your password if you forget it. Write this code down somewhere safe — it won't be shown again.",
	"authCards.recoveryDismiss": "I've saved it",

	// Link a new device (pairing code) — existing-account-only redemption,
	// collapsed under a link the same way the password fallback is.
	"authCards.haveAnAccount":          "Already have an account? Link this device",
	"authCards.deviceLinkTitle":        "Link this device",
	"authCards.deviceLinkIntro":        "Link this device using a pairing code from your account settings on another device.",
	"authCards.pairingCodeLabel":       "Pairing code",
	"authCards.pairingCodePlaceholder": "123456",
	"authCards.pairingCodeRequired":    "Enter the 6-digit pairing code.",
	"authCards.pairingCodeInvalid":     "Pairing codes are 6 digits.",
	"authCards.linkDevice":             "Link this device",
	"authCards.linking":                "Linking…",
	"authCards.linkFailed":             "That code didn't work. Check it and try again — codes expire after a few minutes.",
	"authCards.deviceLinked":           "This device is linked.",
	"authCards.linkAnotherDevice":      "Link a different device",

	// Same card, promoted to the primary sign-in surface on a server where an
	// admin-minted activation code is the ONLY way in (Register disabled).
	// Worded for that case: nothing exists on "another device" yet, and the
	// code comes from the admin console, not from account settings.
	"authCards.activateTitle": "Activate this device",
	"authCards.activateIntro": "Generate an activation code in your site's admin console, then enter it here. Each code works once and expires after 5 minutes.",
	"authCards.activate":      "Activate",
	"authCards.activated":     "This device is activated — your data will start syncing.",

	// Admin-approved device pairing (TODOS.md C454) — for an invite-only
	// server where there's no self-signup: the device asks to be paired and
	// waits for an admin to approve it, rather than the user needing any
	// credential yet. Collapsed under a link like the other secondary
	// sign-in methods.
	"authCards.waitForApproval":           "Ask an admin to approve this device instead",
	"authCards.pendingTitle":              "Device pairing",
	"authCards.pendingConnecting":         "Requesting access…",
	"authCards.pendingWaiting":            "Waiting for an admin to approve this device…",
	"authCards.pendingWaitingHint":        "Someone with access to this server needs to approve your request before you can sign in.",
	"authCards.pendingCancel":             "Cancel request",
	"authCards.pendingApprovedTitle":      "You've been approved",
	"authCards.pendingApprovedHint":       "Check that this code matches what the admin sees, then accept.",
	"authCards.pendingAccept":             "Accept — this is my code",
	"authCards.pendingReject":             "This isn't right — reject",
	"authCards.pendingRejectedByAdmin":    "This request was declined.",
	"authCards.pendingCanceled":           "Request canceled.",
	"authCards.pendingExpired":            "This request expired before anyone approved it. Try again.",
	"authCards.pendingRequestFailed":      "Couldn't ask to be paired. Try again.",
	"authCards.pendingWatchFailed":        "Lost the connection while waiting. Try again.",
	"authCards.pendingRedeemFailed":       "Couldn't finish pairing. Try again.",
	"authCards.pendingSetPasswordTitle":   "Set a password",
	"authCards.pendingSetPasswordHint":    "Add a username and password so you can sign back in next time without needing approval again.",
	"authCards.pendingSetPasswordSkip":    "Skip for now",
	"authCards.pendingSetPasswordSubmit":  "Save password",
	"authCards.pendingSetPasswordSaving":  "Saving…",
	"authCards.pendingSetPasswordFailed":  "Couldn't save the password. Try again.",
	"authCards.pendingSetPasswordSuccess": "Password saved. You can log in with it next time.",
	"authCards.pendingDone":               "You're signed in.",
}

func init() {
	for k, v := range authCardsKeys {
		english[k] = v
	}
}

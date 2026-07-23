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
	"authCards.loginFailed":                 "That username or password didn't work.",
	"authCards.registerFailed":              "Couldn't create that account — the username may already be taken.",
	"authCards.loggedInAs":                  "Signed in as %s.",
	"authCards.useDifferentAccount":         "Use a different account",

	// Password fallback — one-time recovery code shown once at Register.
	"authCards.registerSuccess": "Account created — you're signed in.",
	"authCards.recoveryTitle":   "Save your recovery code",
	"authCards.recoveryIntro":   "There's no email on file to reset your password if you forget it. Write this code down somewhere safe — it won't be shown again.",
	"authCards.recoveryDismiss": "I've saved it",

	// Link a new device (pairing code) — existing-account-only redemption.
	"authCards.deviceLinkTitle":        "Already have an account?",
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
}

func init() {
	for k, v := range authCardsKeys {
		english[k] = v
	}
}

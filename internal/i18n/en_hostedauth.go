// SPDX-License-Identifier: MIT

package i18n

var hostedAuthKeys = Catalog{
	"hostedAuth.title":        "Your CashFlux, securely synced",
	"hostedAuth.intro":        "Sign in to open your budget on this server, recover an account, or request access.",
	"hostedAuth.checking":     "Checking your saved session…",
	"hostedAuth.checkFailed":  "CashFlux couldn't verify your session. Check your connection and try again.",
	"hostedAuth.sessionEnded": "Your session ended. Sign in again to continue.",
	"hostedAuth.retry":        "Try again",
	"hostedAuth.newAccount":   "New account",
	"hostedAuth.adminConsole": "Open the owner console",
}

func init() {
	for k, v := range hostedAuthKeys {
		english[k] = v
	}
}

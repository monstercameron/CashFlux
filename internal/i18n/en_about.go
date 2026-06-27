// SPDX-License-Identifier: MIT

package i18n

// aboutKeys holds English strings for the dedicated /about screen (C290 / C293).
// Kept in a separate file from en.go (concurrent WIP) and merged at init time
// using the same pattern as en_home.go.
var aboutKeys = Catalog{
	// Section headings.
	"about.headingIdentity":  "CashFlux",
	"about.headingPrivacy":   "Privacy & your data",
	"about.headingCloudSync": "Cloud sync",
	"about.headingAI":        "AI features",
	"about.headingVersion":   "Version & changelog",

	// App identity.
	"about.tagline": "A local-first, household-aware budgeting app. Track spending, set budgets and goals, and see where your money goes — entirely on your device.",

	// Privacy / local-first statement (C290, C293).
	"about.privacyLocalFirst": "CashFlux is local-first. Your financial data is stored on this device only — no account is required, and nothing is uploaded anywhere by default.",
	"about.privacyExport":     "You can export a full backup of your data at any time from Settings → Data. The app works offline; nothing reaches the network unless you explicitly turn on a feature that does.",
	"about.privacyNoTracking": "CashFlux collects no analytics, no crash reports, and has no ads. There is no user account and no server that sees your transactions.",

	// Cloud-sync disclosure (C291).
	"about.cloudSyncOff":     "Cloud sync is off by default. When it is turned off, no financial data ever leaves your device.",
	"about.cloudSyncOn":      "If you turn on cloud sync (Settings → Cloud), CashFlux sends an encrypted snapshot of your full dataset — accounts, transactions, budgets, goals, and members — to the self-hosted backend you configure. The snapshot is sent over HTTPS. The backend URL and credentials are yours; Anthropic and the CashFlux project do not operate any shared sync server.",
	"about.cloudSyncControl": "You can turn sync off at any time and delete data from your backend independently of the app.",

	// AI-key disclosure (C292).
	"about.aiKeyOwnKey":   "AI features use OpenAI's API with your own API key (bring-your-own-key). CashFlux does not provide or proxy an API key.",
	"about.aiKeyStorage":  "Your key is stored only on this device (in the browser's local storage). It is never sent to a CashFlux server.",
	"about.aiKeyUsage":    "Your key and a summary of the relevant data (not raw transactions) are sent directly from your browser to OpenAI only when you explicitly invoke an AI feature — for example, asking a question in Insights or running the allocation assistant.",
	"about.aiKeySettings": "Add or remove your key in Settings → AI.",

	// Version card.
	"about.versionLabel":  "Version",
	"about.changelogLink": "See the full changelog →",
	"about.changelogHref": "https://github.com/monstercameron/CashFlux/blob/main/CHANGELOG.md",
	"about.licenseNote":   "CashFlux is open source, released under the MIT License.",
	"about.licenseHref":   "https://github.com/monstercameron/CashFlux/blob/main/LICENSE",
	"about.licenseLink":   "View license →",
	"about.sourceHref":    "https://github.com/monstercameron/CashFlux",
	"about.sourceLink":    "Source on GitHub →",
}

func init() {
	for k, v := range aboutKeys {
		english[k] = v
	}
}

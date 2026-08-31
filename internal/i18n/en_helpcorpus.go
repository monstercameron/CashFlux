// SPDX-License-Identifier: MIT

package i18n

import "maps"

// helpCorpusKeys hold the /help centre's topic corpus (C361).
//
// These were plain string arguments to help.go's local section builder, which
// is a display position the hardcoded-copy scanner did not look at — so the
// whole help centre sat outside the language setting while every screen around
// it was at zero. The scanner covers the builder now; these are its findings,
// converted byte-for-byte so the rendered English is unchanged.
var helpCorpusKeys = Catalog{
	"help.startTitle":     "Getting started",
	"help.dataTitle":      "Bringing in your data",
	"help.reportsTitle":   "Budgets, goals & reports",
	"help.smartTitle":     "The Smart layer",
	"help.shortcutsTitle": "Keyboard shortcuts",
	"help.privacyTitle":   "Your privacy",
	"help.supportTitle":   "Support & feedback",
	"help.offlineTitle":   "Works offline",
	"help.startBody1":     "Add an account (Accounts → Add account), then record what you spend and earn from the + button or the dashboard's Add transaction.",
	"help.startBody2":     "Set a budget per category in Budgets, and track savings targets in Goals — the dashboard rolls it all up.",
	"help.dataBody1":      "Import a bank CSV from Documents → import; CashFlux maps the columns and flags duplicates before anything is saved.",
	"help.dataBody2":      "Most banks let you export a CSV from your transactions or statements page — look for an Export or Download option.",
	"help.reportsBody1":   "Budgets show what's left for the period; Goals show pace toward a target. Reports breaks down spending by category, payee, and member, with trends over time.",
	"help.reportsBody2":   "Financial health (in Plan & analyze) scores your overall position and suggests the next step.",
	"help.smartBody1":     "Smart surfaces optional, opt-in insights and recommendations. Free insights run entirely on your device at no cost; AI features are clearly labelled and only run when you add your own key.",
	"help.smartBody2":     "Turn features on or off in Smart → Manage, and dial how much they surface in Appearance.",
	"help.shortcutsBody1": "Press ? anytime to see the full shortcut list. Ctrl/⌘ K opens the command palette to jump anywhere or run an action.",
	"help.shortcutsBody2": "Alt + 1–9 and Alt + 0 jump to your ten pinned screens — pin any screen with the star beside it in the menu. Alt + M puts the cursor in the menu filter, and Alt + N adds a transaction.",
	"help.privacyBody1":   "CashFlux is local-first: your financial data is stored on this device and is never uploaded or shared. You can export a backup at any time from Settings.",
	"help.privacyBody2":   "An optional passcode lock (Settings) keeps the app's screens behind a code and can encrypt your data at rest.",
}

func init() {
	maps.Copy(english, helpCorpusKeys)
}

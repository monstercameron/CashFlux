// SPDX-License-Identifier: MIT

package i18n

// homeKeys is the EC4 home-hero band set of English strings registered into the
// source catalog at init time — kept separate from en.go (user WIP) so EC4
// changes do not land in the user's working tree. The four keys already in
// en.go (home.welcomeBack / heroTitle / heroSubtitle / syncCta) are deliberately
// omitted here; they remain in en.go and the init loop below adds only new keys.
var homeKeys = Catalog{
	// Time-of-day greetings (hero heading, non-empty dataset).
	"home.greetingMorning":   "Good morning.",
	"home.greetingAfternoon": "Good afternoon.",
	"home.greetingEvening":   "Good evening.",

	// Hero stat labels (non-empty dataset).
	"home.netWorth":  "Net worth",
	"home.thisMonth": "this month",

	// Quote of the day (SMART-QUOTE, opt-in AI).
	"home.quoteEnable":       "Add a daily quote",
	"home.quoteNeedKey":      "Add your OpenAI key in Settings to get a daily quote.",
	"home.quoteLoading":      "Composing today's quote…",
	"home.quoteThemeLabel":   "Quote theme",
	"home.quoteNew":          "New quote",
	"home.quoteError":        "Couldn't load today's quote — try again.",
	"home.quoteContext":      "Personalize",
	"home.quoteContextTitle": "Use your goals and finances to pick a more relevant quote (sends a snapshot to your AI provider)",
	"home.income":            "Income",
	"home.spending":          "Spending",
	"home.net":               "Net",
	"home.savingsRate":       "Savings rate",

	// Quick-action button labels (non-empty dataset).
	"home.quickAddTxn":     "Add transaction",
	"home.quickAddAccount": "Add account",

	// First-run welcome state (empty dataset).
	"home.welcomeTitle": "Your money, beautifully organized.",
	"home.welcomeBody":  "Track spending, set budgets, and watch your net worth grow — all on this device, completely private.",
	"home.loadSample":   "Load sample data",
	"home.addFirst":     "Add your first account",

	// Aria labels for hero buttons.
	"home.loadSampleAria":      "Load sample financial data to explore CashFlux",
	"home.addFirstAria":        "Open the add account form",
	"home.quickAddTxnAria":     "Open quick-add transaction panel",
	"home.quickAddAccountAria": "Open the add account form",
}

func init() {
	for k, v := range homeKeys {
		english[k] = v
	}
}

// SPDX-License-Identifier: MIT

// Package help provides a curated FAQ dataset for CashFlux, together with a
// simple case-insensitive substring filter. It is a pure data package: no
// platform dependencies, no syscall/js, and no domain-layer imports, so it
// can be unit-tested on native Go and rendered by any UI layer.
package help

import "strings"

// FAQItem is a single FAQ entry. Question and Answer are plain-English prose.
// Keywords holds additional terms (synonyms, feature names, action verbs) that
// the filter also searches so users can find an item without knowing the exact
// phrasing of its question.
type FAQItem struct {
	Question string
	Answer   string
	Keywords []string
}

// Items returns the full, ordered list of FAQ entries. The slice is freshly
// allocated on each call so callers may safely sort or filter without
// affecting other callers.
func Items() []FAQItem {
	src := []FAQItem{
		{
			Question: "Where is my data stored?",
			Answer:   "Everything is saved locally on this device in an encrypted in-browser database. Nothing is sent anywhere unless you explicitly enable Cloud Sync or an AI feature.",
			Keywords: []string{"storage", "local", "device", "database", "where", "save", "offline", "local-first"},
		},
		{
			Question: "Does CashFlux share my financial data with anyone?",
			Answer:   "No. CashFlux is local-first: your data never leaves your device unless you turn on Cloud Sync (which you opt into) or use an AI feature that contacts an external API (like the OpenAI assistant, which requires your own key).",
			Keywords: []string{"privacy", "share", "data", "cloud", "sync", "ai", "openai", "send", "third-party", "security"},
		},
		{
			Question: "How do I import transactions from a CSV file?",
			Answer:   "Go to Documents → Import CSV. Select or drag-and-drop your bank's CSV export, map the columns (date, amount, description) in the preview, choose the destination account, and click Import. CashFlux flags likely duplicates before committing.",
			Keywords: []string{"csv", "import", "upload", "bank", "transactions", "file", "columns", "mapping", "documents"},
		},
		{
			Question: "How do categories and budgets work?",
			Answer:   "Every transaction belongs to a category (e.g. Groceries, Rent). Budgets set a spending limit per category for a chosen period (weekly, monthly, etc.). The budget bar turns red when you exceed the limit. You can also create sub-categories for finer tracking.",
			Keywords: []string{"categories", "budgets", "budget", "limit", "spending", "period", "monthly", "weekly", "envelope", "subcategory"},
		},
		{
			Question: "How do I back up or export my data?",
			Answer:   "Open Settings → Data. Use \"Back up everything\" to download a full encrypted JSON backup of all accounts, transactions, goals, and documents. You can also export transactions as a CSV from the Transactions screen.",
			Keywords: []string{"backup", "export", "download", "restore", "json", "csv", "data", "settings"},
		},
		{
			Question: "What does Cloud Sync do, and does it cost anything?",
			Answer:   "Cloud Sync keeps your CashFlux data in sync across multiple devices through Anthropic's hosted relay. It is part of the paid plan. Your free plan keeps data local only. When enabled, data is encrypted before leaving your device.",
			Keywords: []string{"cloud", "sync", "multi-device", "devices", "paid", "plan", "subscription", "cost", "price", "encrypted"},
		},
		{
			Question: "How do I use the AI assistant?",
			Answer:   "CashFlux uses a bring-your-own-key (BYOK) model: go to Settings → AI and paste your OpenAI API key. Once set, open the Insights screen and type a question — the assistant has read-only access to your aggregated data and cannot move money. Without a key, you can still use the free built-in Q&A for common questions.",
			Keywords: []string{"ai", "assistant", "openai", "key", "byok", "api", "insights", "chat", "question", "gpt"},
		},
		{
			Question: "What keyboard shortcuts are available?",
			Answer:   "Press Ctrl+K (or Cmd+K on Mac) to open the command palette where you can search any action by name. Press ? anywhere to see a cheat sheet of the most useful shortcuts, including quick-add (N), search (F), and settings (G S).",
			Keywords: []string{"keyboard", "shortcuts", "hotkeys", "ctrl+k", "cmd+k", "palette", "cheat sheet", "navigation", "accessibility"},
		},
		{
			Question: "How do I add another person to my household?",
			Answer:   "Go to Settings → Household Members and click Add member. Enter their name and choose a role (Viewer, Editor, or Admin). Each member can have their own budgets, goals, and spending views. Note: multi-user access is local only — each person uses the app on this device (or syncs via Cloud Sync on the paid plan).",
			Keywords: []string{"household", "member", "family", "partner", "spouse", "add user", "role", "viewer", "editor", "admin", "multi-user"},
		},
		{
			Question: "How does multi-currency work?",
			Answer:   "Each account can be denominated in a different currency. CashFlux converts all balances to your chosen base currency (set in Settings → Preferences) using the exchange rates you enter in Settings → FX Rates. Net worth and budget totals always display in your base currency.",
			Keywords: []string{"currency", "multi-currency", "forex", "fx", "exchange rate", "foreign", "usd", "eur", "cad", "base currency", "convert"},
		},
		{
			Question: "How do I wipe or reset the app?",
			Answer:   "Go to Settings → Data → Wipe all data. You will be asked to confirm. This permanently deletes all accounts, transactions, budgets, goals, and settings from this device. Consider exporting a backup first.",
			Keywords: []string{"wipe", "reset", "delete", "clear", "start over", "factory reset", "erase", "data"},
		},
		{
			Question: "How do I report a bug or request a feature?",
			Answer:   "Open a GitHub issue at github.com/monstercameron/CashFlux/issues. Please include the app version (shown in Settings → About), a description of what you expected versus what happened, and steps to reproduce. For security issues, email the maintainer directly rather than filing a public issue.",
			Keywords: []string{"bug", "report", "issue", "github", "feedback", "feature", "request", "problem", "error", "crash", "contact"},
		},
	}
	// Return a fresh copy so callers cannot mutate the canonical slice.
	out := make([]FAQItem, len(src))
	copy(out, src)
	return out
}

// Filter returns the subset of items whose Question, Answer, or any Keyword
// contains query as a case-insensitive substring. An empty or whitespace-only
// query returns all items (a copy of items). The original slice is not
// modified.
func Filter(items []FAQItem, query string) []FAQItem {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]FAQItem, len(items))
		copy(out, items)
		return out
	}
	var out []FAQItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Question), q) ||
			strings.Contains(strings.ToLower(item.Answer), q) ||
			keywordMatch(item.Keywords, q) {
			out = append(out, item)
		}
	}
	return out
}

// keywordMatch reports whether any keyword in kws contains q as a
// case-insensitive substring.
func keywordMatch(kws []string, q string) bool {
	for _, kw := range kws {
		if strings.Contains(strings.ToLower(kw), q) {
			return true
		}
	}
	return false
}

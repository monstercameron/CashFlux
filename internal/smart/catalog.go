// SPDX-License-Identifier: MIT

package smart

import "sort"

// Feature is one catalog entry in the SMART series: a single optional capability
// the user can turn on. The catalog is data, not code, so the settings UI, the
// cost-preview, and the engines all read from one source of truth and adding a
// feature is a table row.
//
// Cost transparency lives here: Tier says Free or AI; for AI features the
// Typical*Tokens seed an indicative per-call cost estimate (see cost.go), and
// RuleCore flags AI features that still do something useful with no provider
// configured (the model only sharpens phrasing or handles the fuzzy long tail).
type Feature struct {
	Code    string // stable code, e.g. "SMART-A1"
	Page    Page
	Title   string
	Summary string // one-line plain-English description
	Tier    Tier

	// RuleCore is true for AI features whose core is deterministic — they degrade
	// gracefully to a rule-only form when no inference provider is configured. It
	// is always false for Free features (they have no AI part to degrade from).
	RuleCore bool

	// TypicalInputTokens / TypicalOutputTokens are the indicative token footprint
	// of one AI call, used only to seed the cost preview. Zero for Free features.
	TypicalInputTokens  int64
	TypicalOutputTokens int64
}

// IsAI reports whether the feature needs an inference provider to run (at all,
// or in full for a RuleCore feature).
func (f Feature) IsAI() bool { return f.Tier == TierAI }

// ai is a small constructor for an AI catalog entry, keeping the table compact.
func ai(code string, page Page, title, summary string, ruleCore bool, inTok, outTok int64) Feature {
	return Feature{Code: code, Page: page, Title: title, Summary: summary, Tier: TierAI,
		RuleCore: ruleCore, TypicalInputTokens: inTok, TypicalOutputTokens: outTok}
}

// rule is a small constructor for a Free (deterministic) catalog entry.
func rule(code string, page Page, title, summary string) Feature {
	return Feature{Code: code, Page: page, Title: title, Summary: summary, Tier: TierFree}
}

// Indicative token footprints for the three shapes of AI call in the series, so
// the cost preview is consistent across features of the same kind.
const (
	// A short classification / suggestion (categorize one txn, clean one name).
	tokClassifyIn, tokClassifyOut = 600, 120
	// A free-form language turn (Q&A, narration, plain-language parse).
	tokLanguageIn, tokLanguageOut = 2200, 500
	// A vision read (a receipt image).
	tokVisionIn, tokVisionOut = 3000, 400
)

// catalog is the curated SMART series. Order within a page follows the backlog.
var catalog = []Feature{
	// ── Accounts ──────────────────────────────────────────────────────────────
	rule("SMART-A1", PageAccounts, "Balance anomaly watch", "Flag an account whose balance moved unusually versus its own history."),
	rule("SMART-A2", PageAccounts, "Dormant account nudge", "Spot accounts with no activity in months and estimate their idle cost."),
	ai("SMART-A3", PageAccounts, "Smart type & name cleanup", "Infer account type and propose a clean display name on add/import.", true, tokClassifyIn, tokClassifyOut),
	rule("SMART-A4", PageAccounts, "Cash-positioning suggestions", "Recommend moving idle cash to a higher-yield account, with the yearly gain."),
	ai("SMART-A5", PageAccounts, "Natural-language account Q&A", "Ask questions about your accounts in plain English.", false, tokLanguageIn, tokLanguageOut),
	rule("SMART-A7", PageAccounts, "Recurring-charge detection", "Group recurring debits per account and flag price increases or new ones."),
	rule("SMART-A8", PageAccounts, "Low-balance / overdraft forecast", "Project each account forward and warn before it dips below zero."),
	ai("SMART-A10", PageAccounts, "Account health score", "A 0–100 health score per account, explained in plain language.", true, tokClassifyIn, tokClassifyOut),
	ai("SMART-A11", PageAccounts, "AI credit-health analysis", "A personalized read of your credit-health estimate — the biggest demerits and the highest-impact fix.", true, tokLanguageIn, tokLanguageOut),

	// ── Transactions ──────────────────────────────────────────────────────────
	ai("SMART-T1", PageTransactions, "Auto-categorization", "Assign a category from merchant, amount, and your history. Learns from corrections.", true, tokClassifyIn, tokClassifyOut),
	rule("SMART-T2", PageTransactions, "Smart duplicate detection", "Flag likely duplicate entries and offer one-tap merge or dismiss."),
	rule("SMART-T3F", PageTransactions, "Natural-language search", "Type \"coffee over $20 last month\" and turn it into removable filter chips — fully local, no key."),
	ai("SMART-T3", PageTransactions, "Natural-language search (AI)", "When the local parser can't read a phrasing, the assistant compiles it into the same filter chips.", false, tokLanguageIn, tokLanguageOut),
	rule("SMART-T4", PageTransactions, "Bulk-edit suggestions", "After one edit, offer to apply it to other similar untouched entries."),
	ai("SMART-T5", PageTransactions, "Merchant name cleanup", "Normalize raw import gibberish into clean merchant names.", true, tokClassifyIn, tokClassifyOut),
	rule("SMART-T6", PageTransactions, "Spending-spike alerts", "Flag a transaction that is unusually large for its category."),
	rule("SMART-T7", PageTransactions, "Missing-transaction detection", "Notice when an expected recurring charge hasn't shown up."),
	ai("SMART-T8", PageTransactions, "Receipt OCR", "Snap a receipt; extract merchant, amount, date, and line items.", false, tokVisionIn, tokVisionOut),
	ai("SMART-T10", PageTransactions, "Smart import field-mapping", "Auto-detect which CSV columns are date/amount/merchant/category.", true, tokClassifyIn, tokClassifyOut),
	rule("SMART-T11", PageTransactions, "Cash-flow timeline annotations", "Auto-label notable moments in the transaction stream."),
	ai("SMART-T12", PageTransactions, "Tax-relevant tagging", "Auto-flag potentially deductible transactions into a year-end bucket.", true, tokClassifyIn, tokClassifyOut),
	rule("SMART-T13", PageTransactions, "Refund / reversal matching", "Pair a refund with its original charge and net them visually."),
	ai("SMART-T14", PageTransactions, "Smart+ rule suggestions", "Scan your transactions with AI and suggest categorization rules for the ones your rules don't cover yet.", false, tokClassifyIn, tokClassifyOut),
	ai("SMART-T15", PageTransactions, "Suggest new categories", "Scan your uncategorized transactions and propose new categories to create — you pick which to add.", false, tokClassifyIn, tokClassifyOut),
	ai("SMART-T16", PageTransactions, "Auto-categorize (with review)", "Scan your uncategorized transactions and propose a category for each — you confirm before anything changes.", false, tokClassifyIn, tokClassifyOut),
	ai("SMART-T17", PageTransactions, "Miscategorization review", "Scan your categorized transactions for likely mistakes and propose fixes — you confirm each change.", false, tokClassifyIn, tokClassifyOut),
	ai("SMART-T18", PageTransactions, "Statement import", "Attach a bank or credit-card statement PDF; the AI reads it and lists the transactions to review — categories mapped to your existing ones — before you import.", false, tokVisionIn, tokVisionOut),
	rule("SMART-T19", PageTransactions, "New-merchant awareness", "Flag the first time you've ever paid a merchant — a fraud/awareness signal keyed on the clean merchant name."),
	rule("SMART-T20", PageTransactions, "New-subscription detection", "Notice a second similar charge about a month after the first and offer to track it as recurring."),

	// ── Budgets ───────────────────────────────────────────────────────────────
	rule("SMART-B7", PageBudgets, "Seasonal budget adjustment", "Detect seasonal categories and suggest month-specific budget amounts."),
	rule("SMART-B8", PageBudgets, "Safe-to-spend indicator", "One number: discretionary cash genuinely free after commitments."),
	rule("SMART-B9", PageBudgets, "Budget goal pacing nudges", "Show whether you're ahead or behind pace and the adjustment to fix it."),
	rule("SMART-B10", PageBudgets, "Uncovered-spending finder", "Surface recurring spending with no budget category yet."),
	rule("SMART-B11", PageBudgets, "Auto budget", "Suggest a monthly budget per category from your recent spending, tunable with a slider before you create them."),
	rule("SMART-B12", PageBudgets, "Healthy budget average", "Review each category's spending over time and suggest a sustainable monthly target that ignores one-off spikes."),

	// ── Goals ─────────────────────────────────────────────────────────────────
	rule("SMART-G1", PageGoals, "Suggested contribution amount", "Compute the per-month amount needed and check it against your slack."),
	rule("SMART-G2", PageGoals, "Goal completion forecast", "Project the real finish date from your actual contribution history."),
	rule("SMART-G3", PageGoals, "Auto-allocate surplus to goals", "Suggest splitting month-end leftover cash across goals by priority."),
	ai("SMART-G4", PageGoals, "Goal drafting from a wish", "Describe a goal in plain English; draft target, deadline, and plan.", true, tokLanguageIn, tokLanguageOut),
	rule("SMART-G5", PageGoals, "Trade-off / conflict detection", "Flag when goals collectively demand more than your surplus."),
	rule("SMART-G6", PageGoals, "Milestone celebration & nudges", "Mark 25/50/75/100% milestones and nudge stalled goals."),
	rule("SMART-G8", PageGoals, "Goal-impact preview on spending", "Show a large purchase's cost in goal terms."),
	ai("SMART-G9", PageGoals, "Priority re-ordering suggestions", "Recommend which goal to fund first; the model explains the order.", true, tokClassifyIn, tokClassifyOut),
	rule("SMART-G10", PageGoals, "What-if goal simulator", "Test \"add $100/mo\" or \"push the deadline\" and see new finish dates."),
	rule("SMART-G11", PageGoals, "Emergency-fund adequacy", "Measure the emergency fund against real monthly essentials and flag the gap."),
	rule("SMART-G12", PageGoals, "Auto-create suggested goals", "Suggest goals you likely need but haven't set (emergency, sinking funds)."),
	rule("SMART-G13", PageGoals, "Windfall routing", "Detect an unusually large deposit and suggest an allocation across goals."),
	rule("SMART-G14", PageGoals, "Goal-linked account binding", "Track progress from a linked account's real balance, automatically."),
	rule("SMART-G15", PageGoals, "Debt-payoff strategy optimizer", "Compare avalanche vs snowball; show interest saved and payoff date."),
	rule("SMART-G17", PageGoals, "Recurring auto-contribution", "Set a standing \"on payday, move $X\" rule that auto-logs the contribution."),
	rule("SMART-G18", PageGoals, "Goal feasibility traffic-light", "Green/amber/red on each goal: is its deadline realistic at current pace?"),
	rule("SMART-G19", PageGoals, "Borrow-from-goal warning", "Warn about the setback before a withdrawal pulls from a goal-linked account."),
	rule("SMART-G20", PageGoals, "Shared goal contributions", "Track who contributed what toward a shared household goal."),

	// ── To-dos ────────────────────────────────────────────────────────────────
	rule("SMART-D1", PageTodos, "Auto-generated financial to-dos", "Turn detected events (spikes, missed bills, unused subs) into to-dos."),
	ai("SMART-D4", PageTodos, "Natural-language quick-add", "Type \"move $200 to savings next Friday\" into a structured to-do.", false, tokLanguageIn, tokLanguageOut),

	// ── Planning ──────────────────────────────────────────────────────────────
	rule("SMART-P1", PagePlanning, "Auto-discovered recurring flows", "Scan history and propose the recurring set, pre-filled for one-tap accept."),
	ai("SMART-P2", PagePlanning, "Plain-language scenario builder", "Type a scenario; draft a saved what-if plan and debt-strategy extra.", true, tokLanguageIn, tokLanguageOut),
	ai("SMART-P3", PagePlanning, "Narrated forecast summary", "A one-paragraph plain-English read of the forecast and runway cards.", true, tokLanguageIn, tokLanguageOut),
	rule("SMART-P4", PagePlanning, "Suggested affordability inputs", "Pre-fill the reserve and runway buffer from real essential spend."),
	rule("SMART-P5", PagePlanning, "Goal-aware forecast overlay", "Overlay goals' required contributions onto the 12-month forecast."),
	rule("SMART-P6", PagePlanning, "Forecast confidence band", "Shade a high/low band around the projection from monthly variance."),
	rule("SMART-P8", PagePlanning, "Auto-suggested extra debt payment", "Recommend the largest extra payment that keeps runway above buffer."),
	rule("SMART-P9", PagePlanning, "Sensitivity / break-even finder", "Compute the threshold that flips a what-if plan's outcome."),
	rule("SMART-P10", PagePlanning, "Bill-shock early warning", "Project irregular large charges onto the runway and warn ahead."),

	// ── Allocate ──────────────────────────────────────────────────────────────
	rule("SMART-AL1", PageAllocate, "Auto-suggested profile", "Recommend the allocation profile that fits your current situation."),
	rule("SMART-AL3", PageAllocate, "Smart reserve suggestion", "Pre-fill the emergency-buffer reserve from real essential spend."),
	ai("SMART-AL4", PageAllocate, "Plain-language allocation intent", "Type your intent; set profile, reserve, and per-destination caps.", true, tokLanguageIn, tokLanguageOut),
	rule("SMART-AL5", PageAllocate, "Allocation outcome preview", "Show the projected impact before applying a split."),

	// ── Subscriptions ─────────────────────────────────────────────────────────
	rule("SMART-SU1", PageSubscriptions, "Cancel-candidate recommendations", "Rank the best cuts by combining stale, price-hike, and high-spend signals."),
	ai("SMART-SU2", PageSubscriptions, "Overlapping service detection", "Spot redundant subscriptions serving the same need.", true, tokClassifyIn, tokClassifyOut),
	rule("SMART-SU3", PageSubscriptions, "Free-trial conversion watch", "Detect a first real charge after a trial and warn at conversion."),
	rule("SMART-SU4", PageSubscriptions, "Annual-vs-monthly savings", "Flag monthly subs where switching to annual typically saves money."),
	rule("SMART-SU6", PageSubscriptions, "Cost-creep history", "A sparkline and \"costs 32% more than 2 years ago\" per subscription."),
	rule("SMART-SU7", PageSubscriptions, "Usage-vs-cost flag", "Flag a sub whose category shows little other engagement."),
	rule("SMART-SU8", PageSubscriptions, "Forgotten-since surfacing", "Rank subs by how long since you last interacted with them."),
	rule("SMART-SU9", PageSubscriptions, "Renewal-timed reminders", "Auto-create a \"keep this?\" to-do a few days before each renewal."),
	ai("SMART-SU10", PageSubscriptions, "Category-benchmark context", "Add light context like \"$22/mo is high for music streaming.\"", true, tokClassifyIn, tokClassifyOut),
	rule("SMART-SU11", PageSubscriptions, "Zombie-charge detection", "Flag long-flat low-value charges tied to something clearly stopped."),
	rule("SMART-SU12", PageSubscriptions, "Household sub attribution", "Attribute each subscription to who pays/uses it; flag unclaimed ones."),
	ai("SMART-SU13", PageSubscriptions, "Bundle-opportunity finder", "Spot subscriptions that would be cheaper bundled.", true, tokClassifyIn, tokClassifyOut),
	rule("SMART-SU14", PageSubscriptions, "Cancellation-saved tally", "A running \"you've cancelled N subs, saving $X/year\" scoreboard."),
	rule("SMART-SU15", PageSubscriptions, "Pause-instead-of-cancel", "For seasonal subs, suggest pausing off-months rather than cancelling."),

	// ── Bills ─────────────────────────────────────────────────────────────────
	rule("SMART-BL1", PageBills, "Predicted amount for variable bills", "Predict the likely amount of a variable bill from its recent history."),
	rule("SMART-BL2", PageBills, "Can-you-cover-it check", "Cross-reference upcoming bills against the cash runway."),
	rule("SMART-BL3", PageBills, "Missed / overdue bill detection", "Flag a bill whose due date passed with no matching payment."),
	rule("SMART-BL4", PageBills, "Autopay reconciliation", "Mark bills paid by autopay automatically; flag failed attempts."),
	rule("SMART-BL5", PageBills, "Optimal pay-date suggestion", "Suggest the best day to pay each bill to smooth the month."),
	rule("SMART-BL6", PageBills, "Late-fee risk warning", "Estimate the late fee + interest of missing a due date."),
	rule("SMART-BL7", PageBills, "Bill increase / new-bill detection", "Flag a recurring bill that jumped or a brand-new biller."),
	rule("SMART-BL8", PageBills, "Paycheck-aligned grouping", "Group upcoming bills by which paycheck should cover them."),
	rule("SMART-BL9", PageBills, "Annual-bill sinking-fund nudge", "Suggest setting aside a monthly amount ahead of large irregular bills."),
	rule("SMART-BL10", PageBills, "One-tap pay-all-due", "Mark several due bills paid in a single confirm."),
	rule("SMART-BL13", PageBills, "Statement-vs-minimum clarity", "Show statement balance, minimum, and the interest cost side by side."),
	rule("SMART-BL14", PageBills, "Seasonal bill forecast", "Project seasonal swings for variable bills into upcoming amounts."),
	rule("SMART-BL15", PageBills, "Grace-period confidence", "Learn each biller's real posting pattern and show the last-safe-pay date."),
	rule("SMART-BL16", PageBills, "Price-creep watch", "Flag a recurring bill charging above its expected amount for two cycles running, with a one-tap accept-the-new-price flow."),

	// ── Hub (cross-app) ───────────────────────────────────────────────────────
	rule("SMART-DIGEST", PageHub, "Proactive money digest", "Post a brief summary of your top active insights to the notification feed on a chosen cadence."),
	ai("SMART-QUOTE", PageHub, "Quote of the day", "A short money-mindset quote on the dashboard, written fresh each day in a theme you choose.", false, 120, 40),
}

// byCode indexes the catalog for O(1) lookups, built once at init.
var byCode = func() map[string]Feature {
	m := make(map[string]Feature, len(catalog))
	for _, f := range catalog {
		m[f.Code] = f
	}
	return m
}()

// Catalog returns the full SMART series in backlog order.
func Catalog() []Feature {
	out := make([]Feature, len(catalog))
	copy(out, catalog)
	return out
}

// ByCode returns the feature with the given code. ok is false if unknown.
func ByCode(code string) (Feature, bool) {
	f, ok := byCode[code]
	return f, ok
}

// FeaturesForPage returns the features on a page, in catalog order.
func FeaturesForPage(p Page) []Feature {
	var out []Feature
	for _, f := range catalog {
		if f.Page == p {
			out = append(out, f)
		}
	}
	return out
}

// Pages returns the pages that have at least one feature, in display order.
func Pages() []Page {
	return []Page{
		PageAccounts, PageTransactions, PageBudgets, PageGoals, PageTodos,
		PagePlanning, PageAllocate, PageSubscriptions, PageBills,
	}
}

// Counts returns how many Free and AI features exist, for the settings summary.
func Counts() (free, aiCount int) {
	for _, f := range catalog {
		if f.Tier == TierAI {
			aiCount++
		} else {
			free++
		}
	}
	return free, aiCount
}

// SortInsights orders insights for display: highest severity first, then by
// feature code, then key — a total order so rendering is deterministic.
func SortInsights(in []Insight) {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].Severity != in[j].Severity {
			return in[i].Severity > in[j].Severity
		}
		if in[i].Feature != in[j].Feature {
			return in[i].Feature < in[j].Feature
		}
		return in[i].Key < in[j].Key
	})
}

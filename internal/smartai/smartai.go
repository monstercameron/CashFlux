// SPDX-License-Identifier: MIT

// Package smartai holds the pure, platform-independent parts of the SMART
// series' AI ("[AI]") features: the prompt templates, the small registry of
// which AI features have a shipped UI, and the response-acceptability checks
// that drive the gpt-5.4-mini → gpt-5.5 escalation. It holds no transport — the
// wasm layer reads these to build a request, place the call (defaulting to the
// cheap model and escalating only when Acceptable returns false), and render the
// result. Pure Go, unit-tested on native Go.
package smartai

import "strings"

// Request is a built AI request: a system prompt that frames the task and a user
// message carrying the question and context. Both go to the model verbatim.
type Request struct {
	System string
	User   string
}

// implemented is the set of AI feature codes that have a working UI today, so the
// /smart catalog only offers a toggle for AI features that actually do something
// (mirroring the Free-engine HasEngine gate). It grows as features ship.
var implemented = map[string]bool{
	"SMART-A3":    true, // account name/type cleanup
	"SMART-A5":    true, // natural-language account Q&A
	"SMART-A10":   true, // account health explanation
	"SMART-A11":   true, // AI credit-health analysis (demerits + advice)
	"SMART-T1":    true, // auto-categorization
	"SMART-T3":    true, // natural-language search
	"SMART-T5":    true, // merchant name cleanup
	"SMART-T12":   true, // tax-relevant tagging
	"SMART-G4":    true, // goal drafting from a wish
	"SMART-G9":    true, // goal-priority suggestion
	"SMART-P2":    true, // plain-language scenario draft
	"SMART-P3":    true, // narrated forecast/outlook summary
	"SMART-AL4":   true, // plain-language allocation intent
	"SMART-SU2":   true, // overlapping-service detection
	"SMART-SU10":  true, // category-benchmark context
	"SMART-SU13":  true, // bundle-opportunity finder
	"SMART-T10":   true, // smart import field-mapping
	"SMART-T8":    true, // receipt OCR (vision)
	"SMART-D4":    true, // natural-language to-do quick-add
	"SMART-T14":   true, // Smart+ rule suggestions (/rules AI scan)
	"SMART-T15":   true, // suggest new categories from uncategorized txns
	"SMART-T16":   true, // auto-categorize uncategorized txns (with review)
	"SMART-T17":   true, // miscategorization review
	"SMART-T18":   true, // statement import (AI PDF → review → import)
	"SMART-QUOTE": true, // daily money-mindset quote (hub)
}

// Implemented reports whether the AI feature has a shipped UI.
func Implemented(code string) bool { return implemented[code] }

// ImplementedCodes returns the shipped AI feature codes (order-independent; the
// caller sorts by catalog order).
func ImplementedCodes() []string {
	out := make([]string, 0, len(implemented))
	for c := range implemented {
		out = append(out, c)
	}
	return out
}

// Acceptable reports whether a model's answer is good enough to show, or whether
// the call should escalate to the stronger model. It is deliberately
// conservative: a blank answer, an obvious "I can't"/"I don't know" refusal, or a
// suspiciously truncated reply fails, triggering one escalation. This is the
// "if it's not smart enough" signal the routing policy keys on.
func Acceptable(answer string) bool {
	a := strings.TrimSpace(answer)
	if len(a) < 2 {
		return false
	}
	low := strings.ToLower(a)
	for _, refusal := range []string{
		"i can't", "i cannot", "i'm not able", "i am not able",
		"i don't know", "i do not know", "as an ai", "i'm unable", "i am unable",
	} {
		if strings.HasPrefix(low, refusal) {
			return false
		}
	}
	return true
}

// AccountQASystem is the system prompt for SMART-A5: a tight frame that keeps the
// model grounded in the supplied figures and answering in plain English.
const AccountQASystem = "You are a concise personal-finance assistant inside a budgeting app. " +
	"Answer the user's question using ONLY the account figures provided. " +
	"Be specific and quote the relevant numbers. If the figures don't contain the answer, say so plainly. " +
	"Reply in one or two short sentences — no preamble, no disclaimers."

// AccountQA builds the SMART-A5 request from the user's question and a compact,
// pre-formatted textual snapshot of their accounts (built by the caller, which
// has the live balances). The question is trimmed; an empty question yields an
// empty user message the caller should refuse to send.
func AccountQA(question, accountContext string) Request {
	q := strings.TrimSpace(question)
	user := "Accounts:\n" + strings.TrimSpace(accountContext) + "\n\nQuestion: " + q
	return Request{System: AccountQASystem, User: user}
}

// OutlookSystem is the system prompt for SMART-P3: narrate the figures into one
// short, calm paragraph a person can act on.
const OutlookSystem = "You are a calm personal-finance assistant. Given the figures below, write ONE short " +
	"paragraph (2–4 sentences) summarizing the person's financial outlook in plain English: what's going " +
	"well, what to watch, and the single most useful next step. Use the actual numbers. No headings, no lists, no disclaimers."

// Outlook builds the SMART-P3 request from a pre-formatted snapshot of the
// household's position (net worth, this-month flows, runway, upcoming bills).
func Outlook(context string) Request {
	return Request{System: OutlookSystem, User: "Figures:\n" + strings.TrimSpace(context)}
}

// goalDraftSystem frames SMART-G4: turn a plain-English wish into a concrete plan.
const goalDraftSystem = "You help set savings goals. From the user's wish, draft a concrete goal: a target amount, " +
	"a sensible deadline, and the monthly contribution it implies. If they gave an amount or date, use it; otherwise " +
	"propose reasonable ones and say they're estimates. Two or three short sentences, plain English, no lists."

// GoalDraft builds the SMART-G4 request from the user's plain-language wish and a
// short snapshot of what they can afford (e.g. typical monthly surplus).
func GoalDraft(wish, financialContext string) Request {
	return Request{System: goalDraftSystem,
		User: "Your situation:\n" + strings.TrimSpace(financialContext) + "\n\nWish: " + strings.TrimSpace(wish)}
}

// healthSystem frames SMART-A10: explain account health in plain language.
const healthSystem = "You are a finance assistant. Given the accounts and balances, give a brief plain-English read " +
	"of the person's account health — what looks healthy, what to watch (idle cash, high utilization, thin buffers). " +
	"Two or three sentences, specific numbers, no lists, no disclaimers."

// AccountHealth builds the SMART-A10 request from an account/balance snapshot.
func AccountHealth(accountContext string) Request {
	return Request{System: healthSystem, User: "Accounts:\n" + strings.TrimSpace(accountContext)}
}

// creditSystem frames SMART-A11: a personalized read of the local credit-health proxy.
const creditSystem = "You are a concise credit-health coach inside a budgeting app. You are given a LOCAL " +
	"credit-health estimate (not a FICO score): an overall score/band, per-card utilization, the factors " +
	"dragging it down, and the app's suggested actions. Give a short, personalized read: name the ONE or TWO " +
	"things hurting the score most, then the single highest-impact action to take next, using the actual numbers " +
	"and card names. Be specific and encouraging. Two or three short sentences, plain English, no lists, no " +
	"disclaimers (the app shows its own)."

// CreditAnalysis builds the SMART-A11 request from a pre-formatted snapshot of the credit-
// health result (score, per-card utilization, demerits, and suggested actions).
func CreditAnalysis(creditContext string) Request {
	return Request{System: creditSystem, User: "Credit-health estimate:\n" + strings.TrimSpace(creditContext)}
}

// quoteSystem frames SMART-QUOTE: one real, ATTRIBUTED quote about money fitting
// the requested theme, returned in a fixed "<quote> — <author>" shape so the UI
// can show the citation. Accuracy of attribution is required; "Unknown" is the
// honest fallback rather than a fabricated author.
const quoteSystem = "You select one genuine, well-known quotation about money, wealth, saving, or financial wisdom " +
	"that fits the requested theme. It must be a real quote with a correct attribution — never invent the wording or the " +
	"author; if you are not confident of the author, attribute it to \"Unknown\". When a person's financial situation is " +
	"provided, prefer a quote whose message is especially relevant to it (their goals, debts, or habits) — but keep it a " +
	"real, accurate quote, never tailored or fabricated. " +
	"Return EXACTLY one line in the form: <quote> — <author>. No surrounding quotation marks, no preamble, no commentary."

// QuoteOfDay builds the SMART-QUOTE request: a real, cited quote in the user's
// chosen theme (e.g. "Stoic", "Playful", "Zen"). When financialContext is
// non-empty (the user opted to personalize), it is appended so the model can pick
// a quote relevant to their situation; the citation must still be genuine.
func QuoteOfDay(theme, financialContext string) Request {
	t := strings.TrimSpace(theme)
	if t == "" {
		t = "calm and encouraging"
	}
	user := "Theme: " + t
	if c := strings.TrimSpace(financialContext); c != "" {
		user += "\n\nThis person's financial situation (pick a quote relevant to it):\n" + c
	}
	return Request{System: quoteSystem, User: user}
}

// overlapSystem frames SMART-SU2: spot redundant subscriptions.
const overlapSystem = "You review subscriptions for redundancy. Given the list, name any that overlap in purpose " +
	"(e.g. two music services, several streaming, overlapping cloud storage) and suggest keeping one. If none overlap, " +
	"say so. Two or three sentences, plain English, no lists."

// OverlapDetect builds the SMART-SU2 request from a subscription list snapshot.
func OverlapDetect(subscriptionContext string) Request {
	return Request{System: overlapSystem, User: "Subscriptions:\n" + strings.TrimSpace(subscriptionContext)}
}

// allocationSystem frames SMART-AL4: turn intent into allocation settings.
const allocationSystem = "You help allocate spare cash. From the user's plain-English intent, recommend an allocation " +
	"approach: which profile (debt, safety, goals, balanced), how much to hold in reserve, and any per-destination cap. " +
	"Two or three short sentences, plain English, no lists."

// AllocationIntent builds the SMART-AL4 request from intent + a money snapshot.
func AllocationIntent(intent, financialContext string) Request {
	return Request{System: allocationSystem,
		User: "Your situation:\n" + strings.TrimSpace(financialContext) + "\n\nIntent: " + strings.TrimSpace(intent)}
}

// scenarioSystem frames SMART-P2: draft a what-if scenario in plain English.
const scenarioSystem = "You help plan what-if scenarios. From the user's sentence, describe the scenario as a concrete " +
	"plan: the recurring changes (raises, extra payments) and any one-time items, and a one-line read of the likely effect. " +
	"Two or three short sentences, plain English, no lists."

// ScenarioDraft builds the SMART-P2 request from the sentence + a money snapshot.
func ScenarioDraft(sentence, financialContext string) Request {
	return Request{System: scenarioSystem,
		User: "Your situation:\n" + strings.TrimSpace(financialContext) + "\n\nScenario: " + strings.TrimSpace(sentence)}
}

// todoSystem frames SMART-D4: parse a sentence into a clean to-do.
const todoSystem = "You turn a sentence into a single concise financial to-do. Reply with just the to-do text " +
	"(imperative, includes any amount and date the user gave). One line, no quotes, no preamble."

// TodoParse builds the SMART-D4 request from the user's sentence.
func TodoParse(sentence string) Request {
	return Request{System: todoSystem, User: strings.TrimSpace(sentence)}
}

// cleanupSystem frames SMART-A3: clean up an account name + infer its type.
const cleanupSystem = "You clean up account names. Given a raw account label, infer the account type " +
	"(checking/savings/credit card/loan/brokerage) and propose a clean display name, e.g. " +
	"\"PLAID-CHK-8842\" -> \"Chase Checking ••8842\". Reply with just the suggested name and type. One line."

// AccountCleanup builds the SMART-A3 request from a raw account label.
func AccountCleanup(rawName string) Request {
	return Request{System: cleanupSystem, User: "Raw account name: " + strings.TrimSpace(rawName)}
}

// categorizeSystem frames SMART-T1: pick a category for a transaction.
const categorizeSystem = "You categorize transactions. Given a transaction description and the list of available " +
	"categories, reply with the single best-fitting category name from the list (exact text). If none fit, reply " +
	"\"Uncategorized\". Reply with just the category name."

// Categorize builds the SMART-T1 request from a description and the category list.
func Categorize(description, categories string) Request {
	return Request{System: categorizeSystem,
		User: "Categories:\n" + strings.TrimSpace(categories) + "\n\nTransaction: " + strings.TrimSpace(description)}
}

// searchSystem frames SMART-T3: parse a query into structured filter terms.
const searchSystem = "You translate a plain-English transaction query into concrete filter terms: any merchant/text, " +
	"amount comparison, category, and date range. Reply with a short plain-English restatement of the exact filter, " +
	"e.g. \"merchant contains 'coffee', amount > $10, last month\". One or two lines."

// SearchParse builds the SMART-T3 request from the user's query.
func SearchParse(query string) Request {
	return Request{System: searchSystem, User: strings.TrimSpace(query)}
}

// merchantSystem frames SMART-T5: normalize a raw merchant string.
const merchantSystem = "You normalize messy merchant strings from bank imports into clean names, e.g. " +
	"\"SQ *BLUE BOTTLE 8829 SF\" -> \"Blue Bottle Coffee\". Reply with just the clean merchant name. One line."

// MerchantCleanup builds the SMART-T5 request from a raw merchant string.
func MerchantCleanup(raw string) Request {
	return Request{System: merchantSystem, User: "Raw merchant: " + strings.TrimSpace(raw)}
}

// taxSystem frames SMART-T12: flag potentially deductible transactions.
const taxSystem = "You flag potentially tax-relevant transactions (charity, medical, home-office, business). " +
	"Given the list, name the ones worth reviewing for a deduction and why, briefly. If none, say so. " +
	"Two or three sentences, plain English."

// TaxRelevant builds the SMART-T12 request from a transaction list snapshot.
func TaxRelevant(transactionContext string) Request {
	return Request{System: taxSystem, User: "Transactions:\n" + strings.TrimSpace(transactionContext)}
}

// goalPrioritySystem frames SMART-G9: recommend which goal to fund first.
const goalPrioritySystem = "You advise which savings goal to fund first, weighing deadline urgency, interest cost " +
	"(paying high-interest debt beats low-yield saving), and emergency-fund adequacy. Given the goals, recommend an " +
	"order and explain briefly. Two or three sentences, plain English."

// GoalPriority builds the SMART-G9 request from a goals snapshot.
func GoalPriority(goalsContext string) Request {
	return Request{System: goalPrioritySystem, User: "Goals:\n" + strings.TrimSpace(goalsContext)}
}

// benchmarkSystem frames SMART-SU10: add price context for a subscription.
const benchmarkSystem = "You add light pricing context to a subscription, e.g. \"$22/mo is on the higher end for " +
	"music streaming\". Given the service and its monthly price, give one sentence of context. No disclaimers."

// SubBenchmark builds the SMART-SU10 request from a subscription description.
func SubBenchmark(subscription string) Request {
	return Request{System: benchmarkSystem, User: "Subscription: " + strings.TrimSpace(subscription)}
}

// bundleSystem frames SMART-SU13: spot subscriptions cheaper bundled.
const bundleSystem = "You spot subscriptions that are usually cheaper bundled (e.g. Disney+/Hulu/ESPN, phone + " +
	"streaming perks). Given the list, suggest any bundle opportunities. If none apply, say so. Two or three sentences."

// BundleFinder builds the SMART-SU13 request from a subscription list snapshot.
func BundleFinder(subscriptionContext string) Request {
	return Request{System: bundleSystem, User: "Subscriptions:\n" + strings.TrimSpace(subscriptionContext)}
}

// importSystem frames SMART-T10: map CSV columns for a bank import.
const importSystem = "You map CSV columns for a bank-statement import. Given the header row (and any sample line), " +
	"say which column is the date, which is the amount, which is the merchant/description, and which is the category " +
	"if present. Reply concisely as \"date: <col>, amount: <col>, merchant: <col>, category: <col-or-none>\"."

// ImportMapping builds the SMART-T10 request from a pasted CSV header (and
// optionally a sample line).
func ImportMapping(header string) Request {
	return Request{System: importSystem, User: "CSV header (and optional sample):\n" + strings.TrimSpace(header)}
}

// receiptSystem frames SMART-T8: read a receipt image.
const receiptSystem = "You read receipt images. Extract the merchant name, the date, the total amount, and the main " +
	"line items. Reply concisely as \"merchant: ... | date: ... | total: ... | items: ...\". Use 'unknown' for any " +
	"field you can't read."

// ReceiptOCR builds the SMART-T8 vision request's text parts. The image itself is
// supplied separately to the vision transport by the caller.
func ReceiptOCR() Request {
	return Request{System: receiptSystem, User: "Extract the details from this receipt."}
}

// RuleSuggestSystem frames SMART-T14: propose categorization rules from a
// sample of transactions the user's rules don't cover, in a STRICT line format
// the app parses (one suggestion per line: match phrase => category name).
const RuleSuggestSystem = "You suggest auto-categorization rules for a budgeting app. " +
	"You are given transactions that no existing rule covers, and the list of the household's categories. " +
	"Propose up to 6 rules. Each rule is a short case-insensitive phrase found in the payee or description " +
	"(a merchant name works best) plus the single best-fitting category FROM THE PROVIDED LIST. " +
	"Reply with ONE rule per line in exactly this format and nothing else:\n" +
	"phrase => Category Name\n" +
	"Never invent categories, never suggest a phrase shorter than 3 characters, and skip anything ambiguous."

// RuleSuggest builds the SMART-T14 request from a pre-formatted sample of
// uncovered transactions and the category-name list (both built by the caller).
func RuleSuggest(txnContext, categoryList string) Request {
	return Request{System: RuleSuggestSystem,
		User: "Categories:\n" + strings.TrimSpace(categoryList) + "\n\nUncovered transactions:\n" + strings.TrimSpace(txnContext)}
}

// SuggestedRule is one parsed SMART-T14 suggestion: the match phrase and the
// resolved category (ID + display name).
type SuggestedRule struct {
	Match        string
	CategoryID   string
	CategoryName string
}

// ParseRuleSuggestions parses the model's "phrase => Category" lines against
// the household's real categories (name → id, matched case-insensitively).
// Lines with unknown categories, short phrases, or duplicates are dropped —
// the model can never invent a category or a junk rule. Capped at 6.
func ParseRuleSuggestions(answer string, categoryIDByName map[string]string) []SuggestedRule {
	// Case-insensitive name lookup.
	byLower := make(map[string]struct{ id, name string }, len(categoryIDByName))
	for name, id := range categoryIDByName {
		byLower[strings.ToLower(strings.TrimSpace(name))] = struct{ id, name string }{id, name}
	}
	var out []SuggestedRule
	seen := map[string]bool{}
	for _, line := range strings.Split(answer, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		phrase, cat, ok := strings.Cut(line, "=>")
		if !ok {
			continue
		}
		phrase = strings.Trim(strings.TrimSpace(phrase), "\"'`")
		cat = strings.TrimSpace(cat)
		if len(phrase) < 3 || cat == "" {
			continue
		}
		hit, known := byLower[strings.ToLower(cat)]
		if !known {
			continue
		}
		key := strings.ToLower(phrase)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, SuggestedRule{Match: phrase, CategoryID: hit.id, CategoryName: hit.name})
		if len(out) >= 6 {
			break
		}
	}
	return out
}

// --- SMART-T15: suggest NEW categories from uncategorized transactions --------

// SuggestCategoriesSystem frames SMART-T15: propose NEW budget categories that
// would cover the household's uncategorized transactions, in a strict line format.
const SuggestCategoriesSystem = "You propose NEW budget categories for a household. " +
	"You are given a sample of their UNCATEGORIZED transactions and the list of categories they ALREADY have. " +
	"Suggest up to 8 new categories that would cover these transactions and that do NOT already exist. " +
	"Each is a short, broad, reusable Title Case name plus its kind (expense or income). " +
	"Reply with ONE category per line in exactly this format and nothing else:\n" +
	"Category Name | expense\n" +
	"Never repeat a category that already exists, never propose a name shorter than 3 characters, and skip anything too narrow or ambiguous."

// SuggestCategories builds the SMART-T15 request from a sample of uncategorized
// transactions and the existing-category list (both formatted by the caller).
func SuggestCategories(txnContext, existingCategories string) Request {
	return Request{System: SuggestCategoriesSystem,
		User: "Existing categories:\n" + strings.TrimSpace(existingCategories) + "\n\nUncategorized transactions:\n" + strings.TrimSpace(txnContext)}
}

// SuggestedCategory is one parsed SMART-T15 suggestion: a new category name and
// its kind ("expense" or "income").
type SuggestedCategory struct {
	Name string
	Kind string
}

// ParseCategorySuggestions parses the model's "Name | kind" lines, dropping any
// category that already exists (existingLower holds lower-cased existing names),
// blanks, too-short names, invalid kinds, and duplicates. Kind defaults to
// "expense" when omitted. Capped at 8 so the review list stays scannable.
func ParseCategorySuggestions(answer string, existingLower map[string]bool) []SuggestedCategory {
	var out []SuggestedCategory
	seen := map[string]bool{}
	for _, line := range strings.Split(answer, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if line == "" {
			continue
		}
		name, kind := line, "expense"
		if n, k, ok := strings.Cut(line, "|"); ok {
			name = strings.TrimSpace(n)
			if kk := strings.ToLower(strings.TrimSpace(k)); kk == "income" || kk == "expense" {
				kind = kk
			}
		}
		name = strings.Trim(strings.TrimSpace(name), "\"'`")
		lower := strings.ToLower(name)
		if len(name) < 3 || existingLower[lower] || seen[lower] {
			continue
		}
		seen[lower] = true
		out = append(out, SuggestedCategory{Name: name, Kind: kind})
		if len(out) >= 8 {
			break
		}
	}
	return out
}

// --- SMART-T16 / T17: per-transaction category assignments --------------------

// AutoCategorizeSystem frames SMART-T16: assign an existing category to each
// uncategorized transaction the model is confident about.
const AutoCategorizeSystem = "You categorize a household's UNCATEGORIZED transactions. " +
	"Each transaction is numbered. Use ONLY the categories provided — never invent one. " +
	"For each transaction you are confident about, reply with its number and the single best-fitting category. " +
	"Reply with ONE assignment per line in exactly this format and nothing else:\n" +
	"3 => Category Name\n" +
	"Skip anything ambiguous rather than guessing."

// AutoCategorize builds the SMART-T16 request from a NUMBERED sample of
// uncategorized transactions and the category-name list.
func AutoCategorize(txnContext, categoryList string) Request {
	return Request{System: AutoCategorizeSystem,
		User: "Categories:\n" + strings.TrimSpace(categoryList) + "\n\nUncategorized transactions (numbered):\n" + strings.TrimSpace(txnContext)}
}

// RecategorizeSystem frames SMART-T17: flag likely MIS-categorized transactions
// and propose a better existing category.
const RecategorizeSystem = "You review a household's ALREADY-categorized transactions for likely MIS-categorizations. " +
	"Each numbered transaction shows its current category. Use ONLY the categories provided. " +
	"ONLY when a DIFFERENT category clearly fits better than the current one, reply with the transaction's number and that better category. " +
	"Reply with ONE correction per line in exactly this format and nothing else:\n" +
	"3 => Category Name\n" +
	"Never suggest a transaction's current category, and skip anything you are not confident is wrong."

// Recategorize builds the SMART-T17 request from a NUMBERED sample of already-
// categorized transactions (each showing its current category) and the list.
func Recategorize(txnContext, categoryList string) Request {
	return Request{System: RecategorizeSystem,
		User: "Categories:\n" + strings.TrimSpace(categoryList) + "\n\nTransactions (numbered, with current category):\n" + strings.TrimSpace(txnContext)}
}

// CategoryAssignment is one parsed "N => Category" line: a 1-based reference into
// the scanned transaction slice plus the resolved category.
type CategoryAssignment struct {
	Ref          int
	CategoryID   string
	CategoryName string
}

// ParseCategoryAssignments parses "N => Category Name" lines against the real
// category list (name → id, case-insensitive). Refs outside [1, maxRef], unknown
// categories, and duplicate refs are dropped — the model can never invent a
// category or point past the sample. Capped at maxRef.
func ParseCategoryAssignments(answer string, maxRef int, categoryIDByName map[string]string) []CategoryAssignment {
	byLower := make(map[string]struct{ id, name string }, len(categoryIDByName))
	for name, id := range categoryIDByName {
		byLower[strings.ToLower(strings.TrimSpace(name))] = struct{ id, name string }{id, name}
	}
	var out []CategoryAssignment
	seen := map[int]bool{}
	for _, line := range strings.Split(answer, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		refStr, cat, ok := strings.Cut(line, "=>")
		if !ok {
			continue
		}
		ref := atoiSafe(strings.TrimSpace(refStr))
		cat = strings.Trim(strings.TrimSpace(cat), "\"'`")
		if ref < 1 || ref > maxRef || cat == "" || seen[ref] {
			continue
		}
		hit, known := byLower[strings.ToLower(cat)]
		if !known {
			continue
		}
		seen[ref] = true
		out = append(out, CategoryAssignment{Ref: ref, CategoryID: hit.id, CategoryName: hit.name})
		if len(out) >= maxRef {
			break
		}
	}
	return out
}

// atoiSafe parses a leading run of digits into an int (0 on none), tolerating a
// model that writes "3." or "#3" around the number.
func atoiSafe(s string) int {
	n, started := 0, false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
			started = true
			continue
		}
		if started {
			break
		}
	}
	return n
}

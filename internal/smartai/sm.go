// SPDX-License-Identifier: MIT

package smartai

import (
	"strconv"
	"strings"
	"time"
)

// This file holds the SMART+ half of the item-scoped micro-features (SM-2…SM-16):
// the ones that act on ONE row or card rather than scanning the whole ledger.
//
// They share a discipline the batch scanners established and that matters more
// here, because these answers land inline next to a real number:
//
//   - Anything the model returns that names a category, an account, or a date is
//     parsed against the household's OWN data and dropped when it does not match.
//     A model cannot invent a category here any more than it can in T16.
//   - The model never does arithmetic. It proposes shares, labels and dates; the
//     app computes every amount from them (see splitsuggest.Distribute), so a
//     confident-sounding wrong sum can never reach the ledger.
//   - Every prompt asks for ONE short answer. These are single-row affordances,
//     not a thread, and a paragraph would not fit where they render.

// --- SM-3: propose a split for one charge -------------------------------------

// SplitSuggestSystem frames SM-3's Smart+ tier: propose how ONE charge divides
// across the household's existing categories, as SHARES rather than amounts.
//
// Asking for percentages instead of money is deliberate. The app has the exact
// charge in integer minor units and apportions it with largest-remainder, so the
// lines always sum to the total; a model asked for dollars produces a set that
// misses by a cent often enough to matter and looks authoritative doing it.
const SplitSuggestSystem = "You propose how a single purchase divides across spending categories. " +
	"Use ONLY the categories provided — never invent one. " +
	"Categories are given as full paths with their kind, e.g. \"Auto > Gas | expense\". " +
	"Always reply with the FULL path exactly as given, never just the last part. " +
	"Reply with ONE line per category in exactly this format and nothing else:\n" +
	"Auto > Gas | 40\n" +
	"The number is that category's PERCENT of the purchase; the percentages should total about 100. " +
	"Propose between 2 and 6 categories. If the purchase clearly belongs to a single category, reply with " +
	"nothing at all — a one-category answer is not a split."

// SplitSuggest builds the SM-3 request from a description of the charge (merchant,
// amount and any line items the caller has) and the category list.
func SplitSuggest(charge, categoryList string) Request {
	return Request{System: SplitSuggestSystem,
		User: "Categories:\n" + strings.TrimSpace(categoryList) + "\n\nPurchase:\n" + strings.TrimSpace(charge)}
}

// SplitShare is one parsed split proposal line: a resolved category and the
// percent of the charge the model gave it. The caller turns the shares into
// money — this type deliberately carries no amount.
type SplitShare struct {
	CategoryID   string
	CategoryName string // the qualified path
	Percent      int
}

// ParseSplitShares parses the model's "Path | percent" lines against a Catalog.
// Unknown categories, non-positive or absurd percentages, and duplicate
// categories are dropped. Fewer than two surviving lines returns nil: a
// one-category answer is a categorization, not a split, and the caller has SM-2
// for that. Capped at six lines so the review stays glanceable.
func ParseSplitShares(answer string, catalog Catalog) []SplitShare {
	var out []SplitShare
	seen := map[string]bool{}
	for _, line := range strings.Split(answer, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if line == "" {
			continue
		}
		// The percent is the LAST pipe-separated field, so a category path that
		// itself contains a pipe survives.
		i := strings.LastIndex(line, "|")
		if i < 0 {
			continue
		}
		catPart, pctPart := line[:i], line[i+1:]
		hit, known := catalog.Lookup(catPart)
		if !known || seen[hit.ID] {
			continue
		}
		pct := atoiSafe(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(pctPart), "%")))
		if pct <= 0 || pct > 100 {
			continue
		}
		seen[hit.ID] = true
		out = append(out, SplitShare{CategoryID: hit.ID, CategoryName: hit.Path, Percent: pct})
		if len(out) >= 6 {
			break
		}
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

// --- SM-4: explain one over-budget --------------------------------------------

// whyOverSystem frames SM-4's Smart+ tier: narrate an overage the app has already
// diagnosed. The app supplies the finding and the contributors; the model's job
// is phrasing and a next step, never re-deriving the cause.
const whyOverSystem = "You explain, in ONE or TWO short sentences, why a spending budget went over this period. " +
	"You are given the budget, the figures, and the merchants that contributed most — all already computed. " +
	"Use those numbers; do not recompute or estimate anything. Name the biggest contributor and end with the single " +
	"most useful thing to do next. Plain English, no lists, no preamble, no disclaimers."

// WhyOver builds the SM-4 request from a pre-formatted snapshot of one budget's
// overage (limit, spend, the deterministic reason, and the top contributors).
func WhyOver(budgetContext string) Request {
	return Request{System: whyOverSystem, User: "Budget:\n" + strings.TrimSpace(budgetContext)}
}

// --- SM-5: explain one unusual balance move ------------------------------------

// balanceAnomalySystem frames SM-5's Smart+ tier: say what an already-flagged
// balance move most likely was, and whether it needs attention.
const balanceAnomalySystem = "You explain an unusual change in one account's balance. You are given the account, the " +
	"move, how it compares with that account's own history, and the transactions in the window — all already computed. " +
	"Say in ONE or TWO short sentences what most likely explains it and whether it needs attention. Use the actual " +
	"numbers and merchant names. If the transactions plainly account for it, say so plainly. No lists, no disclaimers."

// BalanceAnomaly builds the SM-5 request from a snapshot of the flagged move.
func BalanceAnomaly(accountContext string) Request {
	return Request{System: balanceAnomalySystem, User: "Account:\n" + strings.TrimSpace(accountContext)}
}

// --- SM-7: explain one notification --------------------------------------------

// explainAlertSystem frames SM-7: gloss a single alert into what it means and
// what to do. This one is Smart+ only — the alert's own copy is already the
// deterministic version, so a rule-based "explanation" would just restate it.
const explainAlertSystem = "You explain one alert from a budgeting app to the person who received it. " +
	"Say what it means and what to do about it, in TWO short sentences at most. " +
	"Use the figures given and never invent any. If the alert is purely informational, say that no action is needed. " +
	"Plain English, second person, no lists, no preamble, no disclaimers."

// ExplainAlert builds the SM-7 request from one notification's title and body,
// plus whatever figures the caller can attach.
func ExplainAlert(title, body, financialContext string) Request {
	user := "Alert: " + strings.TrimSpace(title)
	if b := strings.TrimSpace(body); b != "" {
		user += "\n" + b
	}
	if c := strings.TrimSpace(financialContext); c != "" {
		user += "\n\nRelevant figures:\n" + c
	}
	return Request{System: explainAlertSystem, User: user}
}

// --- SM-13: one goal-pace line -------------------------------------------------

// goalPaceSystem frames SM-13: a single sentence that augments the deterministic
// ETA already on the trajectory card. The app has done the projection; the model
// adds the "what would fix it" the numbers imply.
const goalPaceSystem = "You write ONE short sentence of encouragement-with-a-number about a savings goal. " +
	"You are given the goal, what has been saved, the monthly contribution, the projected finish date, and the " +
	"target date — all already computed. If it will land late, say what monthly amount would hit the target instead. " +
	"If it is on track or early, say so and name the date. Use the given figures only; never recompute. " +
	"One sentence, plain English, no preamble, no disclaimers."

// GoalPace builds the SM-13 request from a pre-formatted goal snapshot.
func GoalPace(goalContext string) Request {
	return Request{System: goalPaceSystem, User: "Goal:\n" + strings.TrimSpace(goalContext)}
}

// --- SM-14: parse one sentence into a to-do ------------------------------------

// TaskParseSystem frames SM-14's Smart+ tier: turn a sentence into a structured
// to-do. It runs only on what internal/duedate could not read, so the prompt is
// written for the awkward tail ("the Tuesday after payday"), not the easy case.
const TaskParseSystem = "You turn a sentence into a single financial to-do. " +
	"Reply with EXACTLY one line in this format and nothing else:\n" +
	"title | YYYY-MM-DD | repeat\n" +
	"The title is imperative and includes any amount the user gave, but never the date words. " +
	"The date is when it is due, resolved against today's date, which is given to you. " +
	"The repeat is one of: none, daily, weekly, biweekly, monthly, quarterly, yearly. " +
	"Write \"none\" for the date if the sentence names no date, and \"none\" for the repeat if it does not repeat. " +
	"Never invent a date the sentence does not imply."

// TaskParseStructured builds the SM-14 request. today is stamped into the prompt
// rather than left to the model's own idea of the date, which is the difference
// between "friday" landing this week and landing in whatever week the model's
// training data thought it was.
func TaskParseStructured(sentence string, today time.Time) Request {
	return Request{System: TaskParseSystem,
		User: "Today is " + today.Format("2006-01-02") + " (" + today.Format("Monday") + ").\n\nSentence: " +
			strings.TrimSpace(sentence)}
}

// TaskDraft is one parsed to-do: a title, an optional due date, and an optional
// repeat. Repeat is the app's own cadence token (domain.RecurringCadence's
// underlying string) or "" for a one-shot; the caller converts.
type TaskDraft struct {
	Title  string
	Due    time.Time
	Repeat string
}

// validRepeats is the closed set of cadence tokens the parser will accept, so an
// invented repeat ("fortnightly-ish") degrades to a one-shot rather than being
// written to a task nobody can edit back.
var validRepeats = map[string]bool{
	"daily": true, "weekly": true, "biweekly": true, "semimonthly": true,
	"monthly": true, "quarterly": true, "yearly": true,
}

// ParseTaskDraft parses the model's "title | date | repeat" line. A missing or
// unreadable date leaves Due zero rather than guessing one, an unrecognized
// repeat leaves Repeat empty, and an empty title returns ok=false — there is no
// useful to-do without one.
func ParseTaskDraft(answer string) (TaskDraft, bool) {
	for _, line := range strings.Split(answer, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if line == "" {
			continue
		}
		fields := strings.Split(line, "|")
		title := strings.Trim(strings.TrimSpace(fields[0]), "\"'`")
		if title == "" {
			continue
		}
		d := TaskDraft{Title: title}
		if len(fields) > 1 {
			if s := strings.TrimSpace(fields[1]); s != "" && !strings.EqualFold(s, "none") {
				if t, err := time.Parse("2006-01-02", s); err == nil {
					d.Due = t
				}
			}
		}
		if len(fields) > 2 {
			if r := strings.ToLower(strings.TrimSpace(fields[2])); validRepeats[r] {
				d.Repeat = r
			}
		}
		return d, true
	}
	return TaskDraft{}, false
}

// --- SM-15: parse one sentence into a transaction draft -------------------------

// TxnDraftSystem frames SM-15: turn "spent 40 at whole foods yesterday" into a
// filled draft. Like T16 it must choose from the household's real categories, and
// like SM-14 it is told today's date rather than trusting its own.
const TxnDraftSystem = "You turn a sentence into a single transaction a person is recording. " +
	"Use ONLY the categories provided — never invent one. " +
	"Categories are given as full paths with their kind, e.g. \"Auto > Gas | expense\". " +
	"Reply with EXACTLY one line in this format and nothing else:\n" +
	"YYYY-MM-DD | merchant | amount | Auto > Gas\n" +
	"The date is resolved against today's date, which is given to you; use today if the sentence names no date. " +
	"The merchant is the payee in clean title case. " +
	"The amount is a plain number with no currency symbol, NEGATIVE for money spent and POSITIVE for money received. " +
	"Write \"none\" for the category if none of the provided ones clearly fit — never guess a category."

// TxnDraftRequest builds the SM-15 request from the sentence, the category list,
// and today's date.
func TxnDraftRequest(sentence, categoryList string, today time.Time) Request {
	return Request{System: TxnDraftSystem,
		User: "Today is " + today.Format("2006-01-02") + " (" + today.Format("Monday") + ").\n\nCategories:\n" +
			strings.TrimSpace(categoryList) + "\n\nSentence: " + strings.TrimSpace(sentence)}
}

// TxnDraft is one parsed transaction draft. AmountMajor is kept as the model
// wrote it (a major-unit decimal string) rather than converted here: minor-unit
// conversion depends on the currency's exponent, which this package has no
// business knowing. The caller converts, as it does for rapidcapture drafts.
type TxnDraft struct {
	Date        time.Time
	Payee       string
	AmountMajor string
	// Negative reports the sign the model gave, so a caller can honor "spent"
	// versus "received" without re-parsing the string.
	Negative bool
	// CategoryID / CategoryName are empty when the model declined to choose or
	// named something the household does not have.
	CategoryID   string
	CategoryName string
}

// ParseTxnDraft parses the model's "date | merchant | amount | category" line
// against a Catalog. It returns ok=false unless BOTH a usable amount and a
// merchant survive: a draft missing either is not worth pre-filling a form with,
// and half a draft is more work to correct than an empty one. An unresolvable
// category is dropped (leaving the field for the user) rather than guessed, and
// an unreadable date falls back to fallbackDate.
func ParseTxnDraft(answer string, catalog Catalog, fallbackDate time.Time) (TxnDraft, bool) {
	for _, line := range strings.Split(answer, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if line == "" {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) < 3 {
			continue
		}
		d := TxnDraft{Date: fallbackDate}
		if s := strings.TrimSpace(fields[0]); s != "" {
			if t, err := time.Parse("2006-01-02", s); err == nil {
				d.Date = t
			}
		}
		d.Payee = strings.Trim(strings.TrimSpace(fields[1]), "\"'`")
		amt, neg, ok := parseAmountMajor(fields[2])
		if d.Payee == "" || !ok {
			continue
		}
		d.AmountMajor, d.Negative = amt, neg
		if len(fields) > 3 {
			if hit, known := catalog.Lookup(fields[3]); known {
				d.CategoryID, d.CategoryName = hit.ID, hit.Path
			}
		}
		return d, true
	}
	return TxnDraft{}, false
}

// parseAmountMajor reads the model's amount field into a clean major-unit
// decimal string plus its sign. It tolerates a currency symbol, thousands
// separators, and parentheses-for-negative, because models write money the way
// people do. A zero or unreadable amount fails.
func parseAmountMajor(field string) (string, bool, bool) {
	s := strings.TrimSpace(field)
	neg := strings.HasPrefix(s, "-")
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		neg = true
	}
	var digits strings.Builder
	dotSeen := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r == '.' && !dotSeen:
			dotSeen = true
			digits.WriteRune(r)
		}
	}
	clean := strings.TrimSuffix(digits.String(), ".")
	if clean == "" {
		return "", false, false
	}
	v, err := strconv.ParseFloat(clean, 64)
	if err != nil || v == 0 {
		return "", false, false
	}
	return clean, neg, true
}

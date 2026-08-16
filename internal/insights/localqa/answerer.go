// Package localqa provides a deterministic keyword/regex intent matcher for the
// no-API-key AI fallback path.
//
// This file implements Answer, the deterministic response generator that maps a
// classified Intent to a plain-English string using only data supplied through
// the Source interface. It has no dependency on syscall/js, appstate, or any
// external service — the wasm layer wires a concrete Source at runtime.
package localqa

import "fmt"

// Source abstracts the financial data required to answer the supported
// intents. All monetary values are expressed in minor units (e.g. cents for
// USD) so the package stays free of floating-point arithmetic.
//
// Implementations live in the wasm/appstate layer; this package defines only
// the contract.
type Source interface {
	// LiquidBalanceMinor returns the total liquid (checking + savings) balance
	// in minor units.
	LiquidBalanceMinor() int64

	// NetWorthMinor returns the total assets and total liabilities, each in
	// minor units. Net worth = assetsMinor − liabilitiesMinor.
	NetWorthMinor() (assetsMinor, liabilitiesMinor int64)

	// SafeToSpendMinor returns the discretionary spending capacity in minor
	// units (after committed expenses and savings goals).
	SafeToSpendMinor() int64

	// SpendingOnCategoryMinor returns the amount spent in the named category
	// (case-insensitive match expected from the implementation) during the
	// current period, in minor units.
	SpendingOnCategoryMinor(category string) int64

	// UpcomingBillsMinor returns the count of bills due in the near future and
	// their combined total in minor units.
	UpcomingBillsMinor() (count int, totalMinor int64)

	// TopGoal returns the name, current saved amount, and target amount (all in
	// minor units) for the highest-priority savings goal. ok is false when no
	// goal exists.
	TopGoal() (name string, currentMinor, targetMinor int64, ok bool)

	// HealthScore returns a numeric score (0–100) and a qualitative band
	// (e.g. "Good", "Fair", "Poor"). ok is false when there is insufficient
	// data to compute the score.
	HealthScore() (score int, band string, ok bool)

	// BudgetStatus returns how many budgets are set, how many are over, and the
	// name and overspend of the worst one. ok is false when no budgets exist.
	BudgetStatus() (total, over int, worstName string, worstOverMinor int64, ok bool)

	// RecentTransactions returns the most recent transactions, newest first, as
	// (payee, minor amount) pairs. The implementation decides how many; the
	// answerer names as many as read well in a sentence.
	RecentTransactions() []RecentTxn

	// Subscriptions returns how many recurring charges are active and their
	// combined monthly cost in minor units.
	Subscriptions() (count int, monthlyMinor int64)

	// LargestExpense returns the biggest single expense of the current period.
	// ok is false when nothing has been spent.
	LargestExpense() (payee string, amountMinor int64, ok bool)
}

// RecentTxn is one recent transaction, reduced to what an answer needs to say.
type RecentTxn struct {
	Payee       string
	AmountMinor int64
}

// recentTxnsNamed is how many recent transactions an answer lists. Three fits a
// sentence; more turns the answer into a table, which the ledger already is and
// does better.
const recentTxnsNamed = 3

// Answer produces a plain-English response for the given intent using live
// financial data supplied by src.
//
// fmtMoney converts a minor-unit integer to a display string (e.g. "$12.34").
// Callers should supply their locale-aware formatter; the function is required
// and must not be nil.
//
// For IntentSpendingByCategory, the category is extracted from rawText via
// ExtractCategory; an empty extraction produces a polite fallback.
//
// Returns ("", false) for IntentNone or when data is genuinely unavailable
// (e.g. no goals configured, health score not yet computable). For all other
// intents a non-empty string and true are returned.
func Answer(intent Intent, src Source, rawText string, fmtMoney func(int64) string) (string, bool) {
	switch intent {
	case IntentBalance:
		bal := src.LiquidBalanceMinor()
		return fmt.Sprintf("Your current liquid balance is %s.", fmtMoney(bal)), true

	case IntentSafeToSpend:
		s2s := src.SafeToSpendMinor()
		return fmt.Sprintf("You have %s available to spend right now.", fmtMoney(s2s)), true

	case IntentNetWorth:
		assets, liabilities := src.NetWorthMinor()
		nw := assets - liabilities
		return fmt.Sprintf(
			"Your net worth is %s (%s in assets minus %s in liabilities).",
			fmtMoney(nw), fmtMoney(assets), fmtMoney(liabilities),
		), true

	case IntentSpendingByCategory:
		cat := ExtractCategory(rawText)
		if cat == "" {
			return "I couldn't tell which category you meant — try asking \"how much did I spend on groceries?\"", true
		}
		spent := src.SpendingOnCategoryMinor(cat)
		return fmt.Sprintf("You've spent %s on %s this period.", fmtMoney(spent), cat), true

	case IntentUpcomingBills:
		count, total := src.UpcomingBillsMinor()
		if count == 0 {
			return "You have no upcoming bills due soon.", true
		}
		return fmt.Sprintf(
			"You have %d upcoming bill%s totalling %s.",
			count, pluralS(count), fmtMoney(total),
		), true

	case IntentGoalProgress:
		name, current, target, ok := src.TopGoal()
		if !ok {
			return "You haven't set up any savings goals yet.", true
		}
		if target <= 0 {
			return fmt.Sprintf("Your goal \"%s\" has no target amount set.", name), true
		}
		pct := current * 100 / target
		return fmt.Sprintf(
			"You're %d%% of the way to your \"%s\" goal (%s saved of %s).",
			pct, name, fmtMoney(current), fmtMoney(target),
		), true

	case IntentHealthScore:
		score, band, ok := src.HealthScore()
		if !ok {
			return "Not enough data yet to score your finances.", true
		}
		return fmt.Sprintf("Your financial health score is %d/100 — %s.", score, band), true

	case IntentBudgetStatus:
		total, over, worst, worstOver, ok := src.BudgetStatus()
		if !ok || total == 0 {
			return "You haven't set up any budgets yet.", true
		}
		if over == 0 {
			return fmt.Sprintf("All %d budget%s are within their limits.", total, pluralS(total)), true
		}
		if over == 1 {
			return fmt.Sprintf("1 of your %d budgets is over: %s, by %s.", total, worst, fmtMoney(worstOver)), true
		}
		return fmt.Sprintf("%d of your %d budgets are over. The furthest over is %s, by %s.",
			over, total, worst, fmtMoney(worstOver)), true

	case IntentRecentTransactions:
		txns := src.RecentTransactions()
		if len(txns) == 0 {
			return "You haven't recorded any transactions yet.", true
		}
		named := txns
		if len(named) > recentTxnsNamed {
			named = named[:recentTxnsNamed]
		}
		parts := make([]string, 0, len(named))
		for _, t := range named {
			parts = append(parts, fmt.Sprintf("%s %s", t.Payee, fmtMoney(t.AmountMinor)))
		}
		out := "Your most recent: " + joinList(parts) + "."
		if len(txns) > len(named) {
			out += fmt.Sprintf(" There %s %d more.", isAre(len(txns)-len(named)), len(txns)-len(named))
		}
		return out, true

	case IntentSubscriptions:
		count, monthly := src.Subscriptions()
		if count == 0 {
			return "No recurring subscriptions are set up.", true
		}
		return fmt.Sprintf("You have %d subscription%s costing about %s a month.",
			count, pluralS(count), fmtMoney(monthly)), true

	case IntentLargestExpense:
		payee, amount, ok := src.LargestExpense()
		if !ok {
			return "Nothing has been spent this period yet.", true
		}
		return fmt.Sprintf("Your biggest expense this period was %s at %s.", fmtMoney(amount), payee), true

	default:
		// IntentNone or any future intent not yet handled.
		return "", false
	}
}

// joinList renders a list the way a person reads one out: "a, b and c".
func joinList(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		out := ""
		for i, p := range parts[:len(parts)-1] {
			if i > 0 {
				out += ", "
			}
			out += p
		}
		return out + " and " + parts[len(parts)-1]
	}
}

// isAre picks the verb for a count, so "there is 1 more" and "there are 4 more"
// both read correctly.
func isAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

// pluralS returns "s" when n != 1, and "" otherwise — used for grammatically
// correct bill counts.
func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

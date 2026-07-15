// SPDX-License-Identifier: MIT

package i18n

// sweepKeys holds English strings for the leftover-sweep month-close ritual (XC6)
// and the earmark-integrity warning (XC7). Kept in a separate file (not en.go) so
// this change does not touch the concurrent-WIP catalog. Registered at init time.
var sweepKeys = Catalog{
	// --- XC6: month-close leftover sweep card ---
	// Card headline. %s = total left (e.g. "$87"), %s = budget count phrase
	// (e.g. "3 budgets"), %s = goal name.
	"sweep.cardBody": "Last month left %s unspent across %s — sweep it to %s?",
	// Primary action. %s = total, %s = goal name.
	"sweep.sweepAction": "Sweep %s to %s",
	// Dismiss action.
	"sweep.dismiss":   "Not this time",
	"sweep.cardTitle": "Month-close sweep",
	// Toast after a successful sweep. %s = total, %s = goal name.
	"sweep.done": "Swept %s to %s.",
	// Shown in place of the action when the target goal's account is over-earmarked.
	"sweep.blocked": "Can't sweep yet — %s's account already has more earmarked than it holds. Review goals first.",
	// Budget-count phrase (singular/plural handled by the caller via a count).
	"sweep.budgetsOne":   "1 budget",
	"sweep.budgetsMany":  "%d budgets",
	"sweep.fallbackGoal": "your goal",

	// --- XC6: config section (flip modal from the budgets toolbar) ---
	"sweep.configTitle":     "Sweep leftovers",
	"sweep.configIntro":     "At month's end, move whatever selected budgets didn't spend into a savings goal.",
	"sweep.configEnable":    "Sweep leftover budget money to a goal each month",
	"sweep.configBudgets":   "Budgets to sweep",
	"sweep.configGoal":      "Send the leftovers to",
	"sweep.configGoalNone":  "Choose a goal…",
	"sweep.configNoGoals":   "Add a savings goal first, then choose it here.",
	"sweep.configNoBudgets": "Add a budget first — there's nothing to sweep yet.",
	"sweep.save":            "Save",
	"sweep.cancel":          "Cancel",
	"sweep.openConfig":      "Sweep leftovers",

	// --- XC7: earmark-integrity warning on an accounts row ---
	// %s = real balance, %s = earmarked total, %s = shortfall (spent goal money).
	"integrity.warnLine":    "Holds %s but %s is earmarked — %s of goal money has been spent.",
	"integrity.transfer":    "Transfer to savings",
	"integrity.reviewGoals": "Review goals",
}

func init() {
	for k, v := range sweepKeys {
		english[k] = v
	}
}

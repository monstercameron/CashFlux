// SPDX-License-Identifier: MIT

package i18n

// workflowSurfaceKeys holds the English strings for the redesigned /workflows
// surface: the masthead, the savings quick-start panels, the automation
// registry, and the composer's plain-English "what this will do" read-back.
// Merged via init so this file does not touch en.go.
var workflowSurfaceKeys = Catalog{
	"wfs.eyebrow": "Automations",
	"wfs.title":   "Workflows",
	"wfs.lede":    "Little machines that watch your money — a trigger, an optional condition, and write-safe actions. Dry-run anything before it touches your data.",

	"wfs.quickKicker": "Savings quick-starts",
	"wfs.compLede":    "A trigger, an optional condition, then one or more actions. The preview below reads it back before you save.",
	"wfs.cadence":     "How often",
	"wfs.details":     "Details",
	"wfs.actionsHead": "Actions",

	"wfs.footTitle":     "What this will do",
	"wfs.whenManual":    "When you run it by hand",
	"wfs.whenTxn":       "When a transaction is added",
	"wfs.whenWeekly":    "Every week",
	"wfs.whenMonthly":   "Every month",
	"wfs.whenQuarterly": "Every quarter",
	"wfs.whenYearly":    "Every year",
	"wfs.whenBudget":    "When a budget goes over its limit",
	"wfs.whenGoal":      "When a goal is reached",
	"wfs.whenBill":      "When a bill comes due",
	"wfs.ifPart":        "if %s",
	"wfs.thenNothing":   "…then nothing yet — add at least one action.",
	"wfs.thenPart":      "then:",

	"wfs.pyfActiveTitle":  "Already set up",
	"wfs.pyfActiveHint":   "Turn it off — or delete it — under Your workflows below. To change the amount, delete it and set it up again.",
	"wfs.pyfAddAnother":   "Add another",
	"wfs.pyfDuplicate":    "You already have a transfer on this route and schedule — it's listed under Your workflows.",
	"wfs.pyfDuplicateOff": "You already set this transfer up — it's just turned off. Re-enable it under Your workflows instead of creating a second one.",
	"wfs.condPlaceholder": "e.g. txn_abs > 200",

	"wfs.subjAbs":      "the amount",
	"wfs.subjAmount":   "the signed amount",
	"wfs.subjPayee":    "the payee",
	"wfs.subjCategory": "the category",
	"wfs.condContains": "%s contains \"%s\"",
	"wfs.condOver":     "%s is over %s",
	"wfs.condAtLeast":  "%s is at least %s",
	"wfs.condUnder":    "%s is under %s",
	"wfs.condAtMost":   "%s is at most %s",
	"wfs.condIs":       "%s is %s",
	"wfs.condIsNot":    "%s is not %s",
	"wfs.condMoneyOut": "money is going out",
	"wfs.condMoneyIn":  "money is coming in",

	"wfs.effectWord":    "effect",
	"wfs.emptyRegistry": "Nothing automated yet — start from a savings quick-start above, or build a workflow on the right.",
	"wfs.disabledTag":   "Off",
	"wfs.deleteWarn":    "Delete this workflow? The automation stops; past runs stay in the history.",
	"wfs.deleteYes":     "Delete workflow",
	"wfs.diagramShow":   "Show diagram",
	"wfs.diagramHide":   "Hide diagram",
}

func init() {
	for k, v := range workflowSurfaceKeys {
		english[k] = v
	}
}

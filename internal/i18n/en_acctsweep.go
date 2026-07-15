// SPDX-License-Identifier: MIT

package i18n

// acctSweepKeys holds English copy for the AC7 sweep-rule proposal card. Kept in
// its own file (not en.go) so it doesn't touch the concurrent-WIP catalog.
var acctSweepKeys = Catalog{
	"acctSweep.title": "Move your extra cash",
	// %s = amount, %s = source account, %s = destination account.
	"acctSweep.body": "%s is sitting above what you keep in %s. Move it to %s?",
	// %s balance, %s keep amount, %s earmarked.
	"acctSweep.breakdown":    "Balance %s − keep %s − set aside %s",
	"acctSweep.approve":      "Move %s",
	"acctSweep.dismiss":      "Not now",
	"acctSweep.done":         "Moved %s to %s.",
	"acctSweep.transferDesc": "Sweep to %s",
	"acctSweep.aria":         "Sweep extra cash out of %s",

	// --- AC7 config manager ---
	"acctSweepCfg.title": "Sweep rules",
	"acctSweepCfg.intro": "Keep a set amount in an account and move whatever's over it somewhere else — we'll suggest the move, you approve it.",
	"acctSweepCfg.empty": "No sweep rules yet. Add one below.",
	// %s from, %s keep amount, %s to, %s cadence.
	"acctSweepCfg.ruleLine":    "Keep %s in %s, move the rest to %s %s",
	"acctSweepCfg.keep":        "Keep",
	"acctSweepCfg.moveTo":      "move extra to",
	"acctSweepCfg.source":      "Account to keep money in",
	"acctSweepCfg.dest":        "Account to move extra into",
	"acctSweepCfg.keepAmount":  "Amount to keep",
	"acctSweepCfg.cadence":     "How often",
	"acctSweepCfg.pickAccount": "Choose an account…",
	"acctSweepCfg.pickTwo":     "Pick two different accounts.",
	"acctSweepCfg.add":         "Add sweep rule",
	"acctSweepCfg.remove":      "Remove",
	"acctSweepCfg.cadence7":    "Weekly",
	"acctSweepCfg.cadence14":   "Every 2 weeks",
	"acctSweepCfg.cadence30":   "Monthly",
	"acctSweepCfg.cadence90":   "Quarterly",
}

func init() {
	for k, v := range acctSweepKeys {
		english[k] = v
	}
}

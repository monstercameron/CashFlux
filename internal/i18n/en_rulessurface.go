// SPDX-License-Identifier: MIT

package i18n

// rulesSurfaceKeys holds the English strings for the redesigned /rules
// automation-ledger surface: the hero (auto-filing coverage + figure chips +
// the plain-English takeaway) and the per-rule weight rows. Merged via init so
// this file does not touch en.go.
var rulesSurfaceKeys = Catalog{
	"rules.heroTitle":     "Auto-filing",
	"rules.heroLabel":     "Filed automatically",
	"rules.countWord":     "%d rules",
	"rules.countWordOne":  "1 rule",
	"rules.firstWins":     "first match wins",
	"rules.chipRules":     "Rules",
	"rules.chipCovered":   "Auto-filed",
	"rules.chipShadowed":  "Never fire",
	"rules.chipSuggested": "Suggestions ready",
	"rls.coverTake":       "Your rules file %s of your %d transactions on their own.",
	"rls.noneTake":        "No rules yet — add one below and matching transactions will file themselves.",
	"rls.shadowClauseOne": "1 rule never fires — an earlier rule always matches first.",
	"rls.shadowClauseN":   "%d rules never fire — earlier rules always match first.",
	"rls.suggestClause":   "%d ready-made rules are waiting below.",
	"rules.caughtSub":     "caught",
	"rules.menuAria":      "Rule actions",
	"rules.editKeepsConditions": "This rule also carries %d additional condition(s) — they're kept as-is when you save.",
}

func init() {
	for k, v := range rulesSurfaceKeys {
		english[k] = v
	}
}

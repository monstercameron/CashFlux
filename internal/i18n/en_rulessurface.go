// SPDX-License-Identifier: MIT

package i18n

// rulesSurfaceKeys holds the English strings for the redesigned /rules
// automation-ledger surface: the hero (auto-filing coverage + figure chips +
// the plain-English takeaway) and the per-rule weight rows. Merged via init so
// this file does not touch en.go.
var rulesSurfaceKeys = Catalog{
	"rules.heroTitle":           "Auto-filing",
	"rules.heroLabel":           "Filed automatically",
	"rules.countWord":           "%d rules",
	"rules.countWordOne":        "1 rule",
	"rules.firstWins":           "first match wins",
	"rules.chipRules":           "Rules",
	"rules.chipCovered":         "Auto-filed",
	"rules.chipShadowed":        "Never fire",
	"rules.chipSuggested":       "Suggestions ready",
	"rls.coverTake":             "Your rules file %s of your %d transactions on their own.",
	"rls.noneTake":              "No rules yet — add one below and matching transactions will file themselves.",
	"rls.shadowClauseOne":       "1 rule never fires — an earlier rule always matches first.",
	"rls.shadowClauseN":         "%d rules never fire — earlier rules always match first.",
	"rls.suggestClause":         "%d ready-made rules are waiting below.",
	"rules.caughtSub":           "caught",
	"rules.menuAria":            "Rule actions",
	"rules.editKeepsConditions": "This rule also carries %d additional condition(s) — they're kept as-is when you save.",
	"rules.condLabel":           "Matches when %s",
	"rules.moveUp":              "Move up (runs earlier)",
	"rules.moveDown":            "Move down (runs later)",
	"rules.applyConfirm":        "Re-file %s now? Categories are overwritten and tags are added:",
	"rulecond.overridesHint":    "When a condition is on, the rule matches by its conditions — the match text above becomes optional and is ignored.",
	"rls.cond.joiner":           " and ",
	"rls.cond.fieldPayee":       "the payee",
	"rls.cond.fieldDesc":        "the description",
	"rls.cond.textContains":     "%s contains \"%s\"",
	"rls.cond.textEquals":       "%s is exactly \"%s\"",
	"rls.cond.amtGt":            "the amount is over %s",
	"rls.cond.amtGte":           "the amount is at least %s",
	"rls.cond.amtLt":            "the amount is under %s",
	"rls.cond.amtLte":           "the amount is at most %s",
	"rls.cond.amtEq":            "the amount is exactly %s",
	"rls.cond.amtNeq":           "the amount is not %s",
	"rls.cond.acctIs":           "the account is %s",
	"rls.cond.acctIsNot":        "the account is not %s",
	"rls.cond.dateOn":           "it's dated %s",
	"rls.cond.dateBefore":       "it's dated before %s",
	"rls.cond.dateAfter":        "it's dated after %s",
	"rls.cond.dateInMonth":      "it's dated in %s",
}

func init() {
	for k, v := range rulesSurfaceKeys {
		english[k] = v
	}
}

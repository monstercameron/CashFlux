// SPDX-License-Identifier: MIT

// Package auditimpact derives, for one recorded audit entry, the plain-English
// list of downstream figures the app recomputes because of that change (#54).
// A financial audit trail that shows "amount: $40 → $60" answers WHAT changed;
// this package answers SO WHAT — which balances, budgets, and report figures
// moved because of it.
//
// The mapping is deliberately derived at DISPLAY time from the entry's entity
// type, action, and changed field names (all already captured), so historical
// entries recorded before this package existed gain the same explanation and
// nothing new needs to be persisted. Pure Go, no syscall/js, table-tested.
package auditimpact

// Figure names reused across entity types. They are display-ready English —
// the same convention auditlog uses for Summary and FieldChange.
const (
	figAccountBalance = "account balance"
	figNetWorth       = "net worth"
	figBudgets        = "budget progress"
	figReports        = "income & spending reports"
	figSafeToSpend    = "safe to spend"
	figHealthScore    = "health score"
	figForecast       = "cash forecast"
	figGoalPace       = "goal pace"
	figUtilization    = "credit utilization"
	figSubscriptions  = "subscription detection"
	figMemberSplit    = "spending by member"
	figConversions    = "every converted total"
	figAutoCategory   = "future auto-categorization"
	figUpcomingBills  = "upcoming bills"
	figCategoryRollup = "category rollups"
)

// moneyMovingTxnFields are transaction fields whose change moves money between
// figures (as opposed to labeling fields, which only re-bucket reports).
var moneyMovingTxnFields = map[string]bool{
	"amount": true, "date": true, "accountId": true, "transferAccountId": true,
	"cleared": true, "splits": true,
}

// Recalculated returns the downstream figures recomputed after a change to
// entityType (auditlog's singular display type: "transaction", "account", …).
// action is the entry's verb ("added"/"updated"/"deleted"); changedFields are
// the JSON field names from the entry's before→after details (empty for adds,
// deletes, and bulk changes — which recalculate everything their type touches).
// The result is de-duplicated, in stable priority order; nil when the change
// type has no derived figures worth naming.
func Recalculated(entityType, action string, changedFields []string) []string {
	fields := map[string]bool{}
	for _, f := range changedFields {
		fields[f] = true
	}
	// An add/delete (or a bulk change with no field diff) moves everything the
	// entity type feeds; a field-scoped update can be narrower.
	broad := action != "updated" || len(fields) == 0

	var out []string
	add := func(names ...string) {
		for _, n := range names {
			seen := false
			for _, o := range out {
				if o == n {
					seen = true
					break
				}
			}
			if !seen {
				out = append(out, n)
			}
		}
	}

	switch entityType {
	case "transaction":
		if broad || hasAny(fields, moneyMovingTxnFields) {
			add(figAccountBalance, figNetWorth, figBudgets, figReports, figSafeToSpend, figHealthScore)
		}
		if broad || fields["categoryId"] || fields["splits"] {
			add(figBudgets, figCategoryRollup, figReports)
		}
		if fields["tags"] {
			add(figReports)
		}
		// Toggling report exclusion re-buckets totals without moving balances.
		if fields["excludeFromReports"] {
			add(figReports, figBudgets, figSafeToSpend)
		}
		if broad || fields["payee"] || fields["desc"] {
			add(figSubscriptions)
		}
		if fields["memberId"] || fields["payerId"] {
			add(figMemberSplit)
		}
	case "account":
		add(figNetWorth, figSafeToSpend, figHealthScore)
		if broad || fields["balance"] || fields["balanceAsOf"] {
			add(figAccountBalance)
		}
		if broad || fields["creditLimit"] || fields["minPayment"] {
			add(figUtilization)
		}
	case "budget":
		add(figBudgets, figHealthScore, figSafeToSpend)
	case "goal":
		add(figGoalPace, figForecast)
	case "category":
		add(figCategoryRollup, figBudgets, figReports)
	case "recurring item":
		add(figUpcomingBills, figForecast, figSafeToSpend)
	case "rule":
		add(figAutoCategory)
	case "member":
		add(figMemberSplit)
	case "settings":
		// Only a repricing change moves figures; a generic settings write
		// (theme, preferences, KV state) must not claim it repriced anything.
		if fields["fxRates"] || fields["baseCurrency"] {
			add(figConversions)
		}
	case "earmark":
		add(figSafeToSpend)
	}
	return out
}

// hasAny reports whether fields contains any key marked true in want.
func hasAny(fields, want map[string]bool) bool {
	for f := range fields {
		if want[f] {
			return true
		}
	}
	return false
}

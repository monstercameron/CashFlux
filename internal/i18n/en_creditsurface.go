// SPDX-License-Identifier: MIT

package i18n

// creditSurfaceKeys holds the English strings for the redesigned /credit bento
// surface (hero + formula identity, the three factor tiles, the empty states).
// Merged via init so this file does not touch en.go.
var creditSurfaceKeys = Catalog{
	"credit.pageTitle":    "Credit health",
	"credit.metricsShow":  "Credit metrics",
	"credit.metricsHide":  "Hide metrics",
	"credit.metricsTitle": "Open the proxy's variables in the formula builder",
	"credit.formulaNote":  "Every piece is a variable you can use anywhere, and you can even re-weight the estimate by editing credit_proxy under Formulas.",
	"credit.formulaHint":  "The estimate and its three factors are live credit_* engine variables — drop any of them into a formula or a dashboard widget.",

	// Utilization (the dominant factor).
	"credit.utilTitle":    "Card utilization",
	"credit.utilTarget":   "under 30% of your combined limit",
	"credit.f.util.why":   "How much of your total card limit is in use — the single biggest input to a credit score, and the one you can move fastest by paying balances down.",
	"credit.f.util.curve": "Scored 100 at 10% used or less, 70 at 30%, sliding to 0 at 80%. Counts most (over half) of the estimate.",

	// On-time payments.
	"credit.ontimeTitle":    "On-time payments",
	"credit.f.ontime.why":   "Whether card payments land by their due day — estimated from your cards' due days and the payments in your ledger over the last three months.",
	"credit.f.ontime.curve": "The share of recent due dates with a payment on time, as a 0–100 score. An honest proxy — it only sees payments recorded here.",
	"credit.f.ontime.good":  "✓ Payments are landing on time",
	"credit.f.ontime.room":  "Some due dates look unpaid — worth a check",
	"credit.na.ontime":      "Not enough data yet — set a due day on a card and record its payments to score this.",

	// Account age.
	"credit.ageTitle":    "Account age",
	"credit.f.age.why":   "How long your cards have been open — a longer track record reads as stability. This grows on its own; the main way to hurt it is closing your oldest card.",
	"credit.f.age.curve": "Scored from each card's age on record, saturating at 100 once the average history is several years long.",
	"credit.f.age.good":  "✓ A long track record",
	"credit.f.age.room":  "A young history — this improves by itself with time",
	"credit.na.age":      "Not enough data yet — set a balance date on a card to score this.",

	// Empty states for the down/up pair.
	"credit.demeritsEmpty": "Nothing is dragging the estimate down right now.",
	"credit.adviceEmpty":   "No urgent moves — keep balances low and payments on time.",

	// Review-pass copy: units, the collapsed limit editor, projected advice scores.
	"credit.outOf100":  "out of 100",
	"credit.editLimit": "Edit limit",
	"credit.ptsUpTo":   "+%d pts → %d",
}

func init() {
	for k, v := range creditSurfaceKeys {
		english[k] = v
	}
}

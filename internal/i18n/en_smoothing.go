// SPDX-License-Identifier: MIT

package i18n

// smoothingKeys holds English strings for annual-bill smoothing (XC3) and the
// committed-vs-free budget split (XC4). Kept in a separate file (not en.go) so
// this change does not touch the concurrent-WIP catalog file. Registered into the
// catalog at init time.
var smoothingKeys = Catalog{
	// XC4: quiet caption under a budget's meter splitting the remaining money into the
	// part already claimed by recurring commitments and the part truly free to spend.
	// %s = committed amount, %s = free amount.
	"budgets.committedCaption": "committed %s · free %s",

	// XC3: plain-English explainer naming a smoothed bill's monthly set-aside folded into
	// the committed figure. %s = set-aside amount, %s = bill name, %s = landing month.
	"budgets.setAsideNote": "includes %s set-aside for %s (%s)",

	// XC3: toggle on the recurring edit form that opts an annual/quarterly bill into
	// smoothing.
	"recurring.smoothLabel": "Smooth this bill into my budgets",

	// XC3: helper text under the toggle explaining what smoothing does, in plain English.
	"recurring.smoothExplainer": "Set aside a little each month so a big yearly or quarterly bill doesn't blow one month's budget. We'll keep a \"Set aside for this\" fund for you and use it when the bill lands.",

	// XC3: note shown when the toggle is offered only for annual/quarterly bills.
	"recurring.smoothOnlyAnnual": "Only yearly and quarterly bills can be smoothed.",

	// XC3: landing-month explainer — the posted bill was covered by its accrued
	// set-aside, which is why the meter reads on-pace. %s = amount, %s = bill name.
	"budgets.landedNote": "covered by %s set aside for %s",
}

func init() {
	for k, v := range smoothingKeys {
		english[k] = v
	}
}

// SPDX-License-Identifier: MIT

package i18n

// lane5Keys holds English strings added by the 2026-07-17 goal/budget/household
// refinement lane (#51 slider accessibility, #70 budgets historical wording,
// #71 compact goal cards, #64 month close, #65 goals refinement, #66 household
// clarity). Merged via init so this file never touches the shared en.go.
var lane5Keys = Catalog{
	// #51 — the contribution planner's direct numeric entry.
	"goals.planAmountLabel": "Monthly contribution amount",
}

func init() {
	for k, v := range lane5Keys {
		english[k] = v
	}
}

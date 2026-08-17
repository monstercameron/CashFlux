// SPDX-License-Identifier: MIT

// Package wfpreset is the catalog of ready-made automations (C405).
//
// The workflow composer can express a great deal, and that was the problem: a
// blank composer asking for a trigger, a condition in a formula language, and a
// list of actions is a wall, and the two templates the app shipped were not
// enough of a runway to get over it. A preset is a complete, working workflow
// with the reasoning already done — the user picks one, sees exactly what it
// will do in plain English, and edits it afterwards like any other.
//
// Presets are DATA, not constructors: the catalog is a plain slice with no
// clock, no ID generator, and no store, which is what makes it testable and what
// lets the same list drive a picker, a search index, and the assistant. Turning
// one into a saved workflow is Instantiate's job, and the caller supplies the id.
package wfpreset

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// Preset is one ready-made automation. NameKey/DescKey are catalog keys rather
// than English: a preset gallery is exactly the kind of copy that has to
// translate, and the workflow it produces carries a translated Name.
type Preset struct {
	// ID is the stable catalog id (not the id of any workflow made from it).
	ID string
	// NameKey names the preset, and also names the workflow it creates.
	NameKey string
	// DescKey is one sentence on what it does and when it fires.
	DescKey string
	// Trigger, Condition and Actions are the workflow body, complete.
	Trigger   workflow.Trigger
	Condition string
	Actions   []workflow.Action
}

// PriceHikeRatio is the multiple of a merchant's typical charge that counts as a
// price change worth a to-do. Fifteen percent is above the noise of a variable
// bill and below the level where the unusual-charge alert already shouts.
const PriceHikeRatio = "1.15"

// All returns the preset catalog in the order a gallery should show it: the
// event-driven ones first (they need no scheduling decision), then the periodic
// rituals.
//
// The list is rebuilt per call so a caller mutating a returned Action cannot
// corrupt the catalog for everyone else.
func All() []Preset {
	return []Preset{
		{
			ID:      "subscription-price-change",
			NameKey: "wfpreset.priceChange.name",
			DescKey: "wfpreset.priceChange.desc",
			Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded},
			// txn_vs_typical is 0 when the merchant has too little history, so the
			// comparison is false for a first-ever charge — a new subscription is
			// not a price hike. See appstate.txnContext.
			Condition: "txn_vs_typical > " + PriceHikeRatio,
			Actions: []workflow.Action{{
				Kind:  workflow.ActionCreateTask,
				Title: "Check the price change at {{txn_payee}}",
				Notes: "This charge was {{txn_abs}}; this merchant normally bills {{txn_payee_typical}}. " +
					"Worth confirming whether the plan changed, and whether it is still worth it.",
			}},
		},
		{
			ID:      "overdue-bill-task",
			NameKey: "wfpreset.overdueBill.name",
			DescKey: "wfpreset.overdueBill.desc",
			Trigger: workflow.Trigger{Kind: workflow.TriggerBillDue},
			Actions: []workflow.Action{{
				Kind:  workflow.ActionCreateTask,
				Title: "Pay the bill that is due",
				Notes: "A bill reached its due date. Pay it, then mark this done.",
			}},
		},
		{
			ID:        "big-charge-review",
			NameKey:   "wfpreset.bigCharge.name",
			DescKey:   "wfpreset.bigCharge.desc",
			Trigger:   workflow.Trigger{Kind: workflow.TriggerTxnAdded},
			Condition: "txn_abs > 500",
			Actions: []workflow.Action{
				{Kind: workflow.ActionFlagReview},
				{Kind: workflow.ActionNotify, Message: "Flagged a {{txn_abs}} charge from {{txn_payee}} for review."},
			},
		},
		{
			ID:      "monthly-reconciliation",
			NameKey: "wfpreset.monthlyRecon.name",
			DescKey: "wfpreset.monthlyRecon.desc",
			Trigger: workflow.Trigger{Kind: workflow.TriggerScheduled, Cadence: domain.CadenceMonthly},
			Actions: []workflow.Action{
				{Kind: workflow.ActionApplyRules},
				{
					Kind:  workflow.ActionCreateTask,
					Title: "Reconcile last month",
					Notes: "Match every account's balance to its statement, clear what has settled, " +
						"and check that nothing is left uncategorized.",
				},
			},
		},
		{
			ID:      "quarterly-account-update",
			NameKey: "wfpreset.quarterlyAccounts.name",
			DescKey: "wfpreset.quarterlyAccounts.desc",
			Trigger: workflow.Trigger{Kind: workflow.TriggerScheduled, Cadence: domain.CadenceQuarterly},
			Actions: []workflow.Action{{
				Kind:  workflow.ActionCreateTask,
				Title: "Update the accounts that do not sync",
				Notes: "Refresh balances for anything the app cannot see automatically — retirement, " +
					"property, loans — so net worth is telling the truth.",
			}},
		},
		{
			ID:      "budget-over-task",
			NameKey: "wfpreset.budgetOver.name",
			DescKey: "wfpreset.budgetOver.desc",
			Trigger: workflow.Trigger{Kind: workflow.TriggerBudgetExceeded},
			Actions: []workflow.Action{{Kind: workflow.ActionFlagBudgetOver}},
		},
	}
}

// Find returns the preset with the given catalog id.
func Find(id string) (Preset, bool) {
	for _, p := range All() {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// Instantiate turns a preset into a saveable workflow with the given id and
// display name. It arrives ENABLED: a preset the user picked from a gallery and
// confirmed is one they want running, and shipping it switched off would make
// "why did nothing happen?" the most common outcome of using the feature.
//
// Actions are deep-copied so editing the saved workflow cannot reach back into
// the catalog.
func (p Preset) Instantiate(id, name string) workflow.Workflow {
	actions := make([]workflow.Action, len(p.Actions))
	copy(actions, p.Actions)
	return workflow.Workflow{
		ID:        id,
		Name:      name,
		Enabled:   true,
		Trigger:   p.Trigger,
		Condition: p.Condition,
		Actions:   actions,
	}
}

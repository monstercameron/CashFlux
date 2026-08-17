// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// smart_rowinsights.go renders an entity's OWN findings inline on its row, as the
// sentence-plus-action they already are — the surface SM-5, SM-11 and SM-16 ask
// for.
//
// The detectors behind those three (SMART-A1 balance anomaly, A8 low-balance
// forecast, A9/A2 fee-bleed and dormancy) have existed and been tested for a
// while, and every one of them already produces a title, a plain-English reason,
// and an executable one-tap action. What the account row offered was a severity
// DOT with the title as a tooltip, plus a popover that only appears at the
// "Everywhere" density. A dot tells you something is true about a row; it cannot
// tell you what, or let you act on it, which is the entire point of these three
// features. So this renders the finding itself.
//
// It deliberately reuses smartInsightCard in Compact mode rather than
// re-rendering the fields: that component already executes every ActionKind, owns
// the dismiss control, and keys off the insight's stable Key. A second renderer
// would be a second place for the action semantics to drift.
//
// Density: gated at AffordanceTooltip (Standard and up), NOT AffordanceBadge. At
// Minimal the row keeps the quiet dot it has always had; the sentence is the
// step up, not the floor.

// smartRowInsightsProps carries the findings to render on one row.
type smartRowInsightsProps struct {
	// ID is the entity id, for the test hook.
	ID string
	// Insights are the entity's findings, already filtered to the wanted features.
	Insights []smart.Insight
}

// smartRowInsights renders the findings as compact rows under an entity.
func smartRowInsights(props smartRowInsightsProps) ui.Node {
	if len(props.Insights) == 0 {
		return Fragment()
	}
	return Div(css.Class("smart-rowinsights", tw.Flex, tw.FlexCol),
		Attr("data-testid", "smart-rowinsights-"+props.ID),
		MapKeyed(props.Insights,
			func(i smart.Insight) any { return i.Key },
			func(i smart.Insight) ui.Node {
				return Fragment(
					ui.CreateElement(smartInsightCard, smartCardProps{Ins: i, Compact: true}),
					// SM-5: a finding that has a paired Smart+ explainer offers it
					// right under the finding, where the question it answers is
					// being asked — not on a hub the reader would have to go find.
					smartInsightExplain(i),
				)
			},
		),
	)
}

// smartExplainPairs maps a deterministic finding to the Smart+ feature that
// explains it in context. Only findings whose "why did this happen" is genuinely
// open belong here: an overdraft forecast already carries its own arithmetic, and
// a model restating it would be paid noise.
var smartExplainPairs = map[string]string{
	"SMART-A1": "SMART-A12", // an unusual balance move → what most likely explains it (SM-5)
}

// smartInsightExplain renders the paired Smart+ explainer for a finding, or
// nothing when the finding has no pair (or the feature is off / unconfigured,
// which smartExplainBlock decides for itself).
func smartInsightExplain(ins smart.Insight) ui.Node {
	code, ok := smartExplainPairs[ins.Feature]
	if !ok {
		return Fragment()
	}
	return ui.CreateElement(smartExplainBlock, smartExplainProps{
		ID:    "ins-" + ins.Key,
		Code:  code,
		Kind:  explainBalance,
		Title: ins.Title,
		Body:  ins.Detail,
	})
}

// smartRowInsightsFor renders an entity's findings inline, restricted to the given
// feature codes and subject to every gate: the density permits a sentence-level
// affordance, and each finding's own feature is enabled and unmuted
// (ShowsAffordance applies both). Returns Fragment when nothing qualifies.
//
// codes is what scopes a row to the features that belong on it — the account row
// asks for the balance-anomaly, overdraft-forecast and fee-bleed findings, not for
// every account-page finding there is. A nil codes means "any".
func smartRowInsightsFor(settings smart.Settings, byEntity map[string][]smart.Insight, relatedID string, codes []string) ui.Node {
	if relatedID == "" || !settings.DensityOrDefault().Shows(smart.AffordanceTooltip) {
		return Fragment()
	}
	want := map[string]bool{}
	for _, c := range codes {
		want[c] = true
	}
	var qualifying []smart.Insight
	for _, ins := range byEntity[relatedID] {
		if len(want) > 0 && !want[ins.Feature] {
			continue
		}
		if !settings.ShowsAffordance(ins.Feature, smart.AffordanceTooltip) {
			continue
		}
		qualifying = append(qualifying, ins)
	}
	if len(qualifying) == 0 {
		return Fragment()
	}
	// Cap at two. A row is not a feed: three stacked findings under one account
	// push the next account off the screen, and the /smart hub is where the full
	// set belongs.
	if len(qualifying) > 2 {
		qualifying = qualifying[:2]
	}
	return ui.CreateElement(smartRowInsights, smartRowInsightsProps{ID: relatedID, Insights: qualifying})
}

// subRowSmartCodes is the set of findings that belong ON a subscription row. T7
// ("this hasn't posted yet") is catalogued under Transactions, so it never
// appeared in the subscriptions page's own RunPage sweep — see mergeEntityInsights.
var subRowSmartCodes = []string{"SMART-T7"}

// mergeEntityInsights folds a second page's findings into an existing entity
// index, keeping only the given feature codes.
//
// It exists because a finding's PAGE (where it is catalogued and where the hub
// groups it) is not always the surface its subject lives on. The
// missing-transaction detector is a transactions feature that is about a
// subscription, and the subscriptions page indexes only PageSubscriptions
// findings — so the one row that could act on it was the one row that never saw
// it. Merging is the narrow fix; re-homing the feature would move it on the hub
// too, where it belongs exactly where it is.
func mergeEntityInsights(dst map[string][]smart.Insight, extra []smart.Insight, codes []string) map[string][]smart.Insight {
	if dst == nil {
		dst = map[string][]smart.Insight{}
	}
	want := map[string]bool{}
	for _, c := range codes {
		want[c] = true
	}
	for _, ins := range extra {
		if len(want) > 0 && !want[ins.Feature] {
			continue
		}
		if ins.Action == nil || ins.Action.RelatedID == "" {
			continue
		}
		dst[ins.Action.RelatedID] = append(dst[ins.Action.RelatedID], ins)
	}
	return dst
}

// accountRowSmartCodes is the set of findings that belong ON an account row: the
// unusual balance move (SM-5), the overdraft forecast (SM-11), and the fee draining
// an idle account (SM-16 — A9 prices the fee, A2 spots the dormancy).
var accountRowSmartCodes = []string{"SMART-A1", "SMART-A8", "SMART-A9", "SMART-A2"}

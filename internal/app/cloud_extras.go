// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func cloudAISettings() uic.Node {
	atom := uistate.UsePrefs()
	notice := uistate.UseNotice()
	keySet := uic.UseState(lsGet(cloudAIKeySetKey) != "")
	busy := uic.UseState(false)
	onError := func(msg string) { busy.Set(false); notice.Set(notice.Get().With(msg, true)) }
	upload := func() {
		if busy.Get() {
			return
		}
		busy.Set(true)
		key := ""
		if a := appstate.Default; a != nil {
			key = a.Settings().OpenAIKey
		}
		p := atom.Get()
		uploadOpenAIKeyToBackend(p.ServerURL, effectiveServerToken(p), key, func() {
			lsSet(cloudAIKeySetKey, "1")
			keySet.Set(true)
			busy.Set(false)
		}, onError)
	}
	remove := func() {
		if busy.Get() {
			return
		}
		busy.Set(true)
		p := atom.Get()
		removeOpenAIKeyFromBackend(p.ServerURL, effectiveServerToken(p), func() {
			lsSet(cloudAIKeySetKey, "")
			keySet.Set(false)
			busy.Set(false)
		}, onError)
	}
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
		H3(css.Class(tw.Text15, tw.FontSemibold), uistate.T("cloud1.aiTitle")),
		P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.aiHint")),
		If(keySet.Get(), Fragment(P(uistate.T("settings.serverKeySet")),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("settings.removeKey"), TestID: "cloud-ai-remove", OnClick: remove, Disabled: busy.Get()}))),
		If(!keySet.Get(), uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("settings.uploadKey"), TestID: "cloud-ai-upload", OnClick: upload, Disabled: busy.Get()})),
	)
}

type cloudBillingProps struct{ SignedIn bool }

func cloudBillingView(p cloudBillingProps) uic.Node {
	atom := uistate.UsePrefs()
	notice := uistate.UseNotice()
	interval := uic.UseState("annual")
	provider := uic.UseState("stripe")
	onError := func(msg string) { notice.Set(notice.Get().With(msg, true)) }
	checkout := func() {
		pr := atom.Get()
		startBillingCheckout(pr.ServerURL, effectiveServerToken(pr), interval.Get(), provider.Get(), onError)
	}
	portal := func() { pr := atom.Get(); openBillingPortal(pr.ServerURL, effectiveServerToken(pr), onError) }
	price := uistate.T("settings.cloudPriceAnnual")
	if interval.Get() == "monthly" {
		price = uistate.T("settings.cloudPriceMonthly")
	}
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap3),
		H3(css.Class(tw.Text15, tw.FontSemibold), uistate.T("settings.cloudPlanTitle")),
		If(p.SignedIn, uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("settings.manageSub"), TestID: "manage-subscription", OnClick: portal})),
		If(!p.SignedIn, Fragment(
			P(css.Class(tw.Text14), price), P(css.Class(tw.Text14, tw.TextDim), uistate.T("settings.cloudTrialNote")),
			uiw.Segmented(uiw.SegmentedProps{Label: uistate.T("settings.cloudPlanBilling"), Selected: interval.Get(), OnSelect: interval.Set,
				Options: []uiw.SegOption{{Value: "annual", Label: uistate.T("settings.cloudPlanAnnual")}, {Value: "monthly", Label: uistate.T("settings.cloudPlanMonthly")}}}),
			uiw.Segmented(uiw.SegmentedProps{Label: uistate.T("settings.cloudPayWith"), Selected: provider.Get(), OnSelect: provider.Set,
				Options: []uiw.SegOption{{Value: "stripe", Label: uistate.T("settings.cloudPayStripe")}, {Value: "paypal", Label: uistate.T("settings.cloudPayPayPal")}}}),
			uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("settings.cloudSubscribe"), TestID: "cloud-subscribe", OnClick: checkout}),
		)),
	)
}

type cloudConflictEntryProps struct{ OnReview func() }

func cloudConflictEntry(p cloudConflictEntryProps) uic.Node {
	return Section(css.Class(cloudSectionRules()), Attr("data-testid", "sync-conflict-restore"),
		H3(css.Class(tw.Text15, tw.FontSemibold), uistate.T("cloud1.copiesTitle")),
		P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.conflictHint")),
		uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.review"), TestID: "cloud-conflicts-entry", OnClick: p.OnReview}),
	)
}

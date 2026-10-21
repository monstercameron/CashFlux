// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"

	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

func serverAIKeySettings() uic.Node {
	atom := uistate.UsePrefs()
	notice := uistate.UseNotice()
	keyDraft := uic.UseState("")
	result := uic.UseState("")
	busy := uic.UseState(false)
	onError := func(msg string) { busy.Set(false); result.Set(""); notice.Set(notice.Get().With(msg, true)) }
	upload := func() {
		if busy.Get() || strings.TrimSpace(keyDraft.Get()) == "" {
			return
		}
		busy.Set(true)
		p := atom.Get()
		uploadOpenAIKeyToBackend(p.ServerURL, effectiveServerToken(p), keyDraft.Get(), func() {
			keyDraft.Set("")
			result.Set(uistate.T("settings.serverKeyStored"))
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
			keyDraft.Set("")
			result.Set(uistate.T("settings.serverKeyRemoved"))
			busy.Set(false)
		}, onError)
	}
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
		P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.aiHint")),
		uiw.TextInput(uiw.TextFieldProps{Class: "set-input", Value: keyDraft.Get(), OnInput: func(v string) { keyDraft.Set(v); result.Set("") },
			AriaLabel: uistate.T("settings.aiServerKeyPlaceholder"), Placeholder: uistate.T("settings.aiServerKeyPlaceholder"),
			Extra: []any{Type("password"), Attr("autocomplete", "off"), Attr("data-testid", "settings-ai-server-key")}}),
		uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("settings.uploadKey"), TestID: "settings-ai-server-save", OnClick: upload, Disabled: busy.Get() || strings.TrimSpace(keyDraft.Get()) == "", Kind: cloudActionPrimary}),
		uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("settings.removeKey"), TestID: "settings-ai-server-remove", OnClick: remove, Disabled: busy.Get()}),
		If(result.Get() != "", P(css.Class(tw.Text14, tw.TextDim), Attr("role", "status"), result.Get())),
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

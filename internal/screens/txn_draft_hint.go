// SPDX-License-Identifier: MIT

//go:build js && wasm

// Typing a transaction in words (SM-15): "spent 40 at whole foods yesterday" →
// a filled quick-add draft.
//
// The free tier is two existing parsers doing the halves each already does well —
// internal/rapidcapture for the amount and the merchant words, internal/duedate
// for "yesterday" / "friday" / "on the 3rd". Neither knew about the other, and
// between them they cover the sentence people actually type. Smart+ handles the
// rest and can additionally pick a category, which nothing local attempts here.
//
// Like SM-14, it fills on request rather than as you type. And like every Smart+
// surface in this series, a category the model names is resolved against the
// household's real list and dropped when it does not match.
package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/smartcatview"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// TxnDraftFill contains optional fields to merge into a quick-add draft.
type TxnDraftFill = smartcatview.DraftFill

// TxnDraftHintProps carries the callback that fills the quick-add form.
type TxnDraftHintProps struct{ OnApply func(TxnDraftFill) }

// TxnDraftHint hosts the sentence state and optional AI request.
func TxnDraftHint(props TxnDraftHintProps) ui.Node {
	pr := uistate.UsePrefs().Get()
	app := appstate.Default
	sentence := ui.UseState("")
	aiFill := ui.UseState(TxnDraftFill{})
	loading := ui.UseState(false)
	errText := ui.UseState("")
	settings := uistate.LoadSmartSettings()
	provider := app != nil && aiProviderConfigured(app, uistate.BackendConnectionActive(pr))
	local := parseTxnSentence(sentence.Get())
	view := smartcatview.PresentDraft(sentence.Get(), local, aiFill.Get(), provider, loading.Get(), settings)
	var conn smartAIConn
	if app != nil {
		conn = resolveAIConn(app, uistate.BackendConnectionActive(pr), pr.ServerURL, pr.ServerToken)
	}
	apply := func() {
		offer := smartcatview.PresentDraft(sentence.Get(), local, aiFill.Get(), provider, loading.Get(), settings).Offer
		if props.OnApply != nil && !offer.Empty() {
			props.OnApply(offer)
		}
	}
	ask := func() {
		a := appstate.Default
		if a == nil || loading.Get() || view.Raw == "" {
			return
		}
		loading.Set(true)
		errText.Set("")
		catalog := smartcatview.Catalog(a.Categories())
		runSmartAI(conn, smartai.TxnDraftRequest(view.Raw, catalog.Prompt(), time.Now()), func(text string) {
			loading.Set(false)
			fill, ok := smartcatview.ParseAIDraft(text, catalog, time.Now())
			if !ok {
				errText.Set(uistate.T("sm15.aiNoAnswer"))
				return
			}
			aiFill.Set(fill)
		}, func(message string) { loading.Set(false); errText.Set(message) })
	}
	if !view.Visible {
		return Fragment()
	}
	return Div(css.Class("sm15"), Attr("data-testid", "sm15-draft"),
		Div(css.Class("sm15-row", tw.Flex, tw.ItemsCenter, tw.Gap2),
			ui.CreateElement(txnDraftSentenceInput, txnDraftSentenceInputProps{
				Sentence: sentence.Get(), OnInput: func(value string) { sentence.Set(value); aiFill.Set(TxnDraftFill{}); errText.Set("") },
				OnApply: apply,
			}),
			ui.CreateElement(txnDraftAIAction, txnDraftAIActionProps{Visible: view.ShowAsk, Loading: loading.Get(), LabelKey: view.AskKey, OnAsk: ask}),
		),
		ui.CreateElement(txnDraftOffer, txnDraftOfferProps{View: view, Settings: settings, FormatDate: pr.FormatDate, OnApply: apply}),
		If(errText.Get() != "", P(css.Class("err", tw.Text12), Attr("role", "alert"), errText.Get())),
	)
}

type txnDraftSentenceInputProps struct {
	Sentence string
	OnInput  func(string)
	OnApply  func()
}

func txnDraftSentenceInput(p txnDraftSentenceInputProps) ui.Node {
	input := ui.UseEvent(p.OnInput)
	keyDown := ui.UseEvent(func(e ui.KeyboardEvent) {
		if e.GetKey() != "Enter" {
			return
		}
		e.PreventDefault()
		e.StopPropagation()
		p.OnApply()
	})
	return Input(css.Class("field field-wide"), Type("text"),
		Attr("id", "sm15-input"), Attr("data-testid", "sm15-input"), Attr("aria-label", uistate.T("sm15.label")),
		Placeholder(uistate.T("sm15.placeholder")), OnInput(input), OnKeyDown(keyDown), uiw.FieldValue(p.Sentence))
}

type txnDraftAIActionProps struct {
	Visible, Loading bool
	LabelKey         string
	OnAsk            func()
}

func txnDraftAIAction(p txnDraftAIActionProps) ui.Node {
	ask := ui.UseEvent(Prevent(p.OnAsk))
	click := OnClick(ask)
	if !p.Visible {
		return Fragment()
	}
	return Button(css.Class("btn btn-sm btn-ghost", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "sm15-ai-btn"), Attr("aria-disabled", ariaBool(p.Loading)),
		click, smartGlyph(true, tw.Fold(tw.W3, tw.H3)), Span(uistate.T(p.LabelKey)))
}

type txnDraftOfferProps struct {
	View       smartcatview.DraftView
	Settings   smart.Settings
	FormatDate func(time.Time) string
	OnApply    func()
}

func txnDraftOffer(p txnDraftOfferProps) ui.Node {
	if p.View.Offer.Empty() {
		return Fragment()
	}
	return smartRowHintFor(p.Settings, p.View.Code, smart.AffordanceFieldAssist, smartRowHintProps{
		Key: "sm15:" + p.View.Raw, TestID: "sm15", Text: txnDraftSummary(p.View.Offer, p.FormatDate),
		Detail: uistate.T(p.View.DetailKey), ActionLabel: uistate.T("sm15.action"), OnAction: p.OnApply,
	})
}

// parseTxnSentence forwards the legacy private entry point to the pure parser.
func parseTxnSentence(raw string) TxnDraftFill { return smartcatview.ParseSentence(raw, time.Now()) }

// txnDraftSummary adapts the user's translation and date preferences.
func txnDraftSummary(f TxnDraftFill, format func(time.Time) string) string {
	return smartcatview.DraftSummary(f, format, uistate.T)
}

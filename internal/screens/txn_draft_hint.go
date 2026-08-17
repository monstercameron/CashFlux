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
	"strings"
	"time"

	uiw "github.com/monstercameron/CashFlux/internal/ui"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/duedate"
	"github.com/monstercameron/CashFlux/internal/rapidcapture"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// The free and paid halves of SM-15.
const (
	txnDraftFreeCode = "SMART-T22F"
	txnDraftAICode   = "SMART-T22"
)

// TxnDraftFill is what a parse hands back to the form. Every field is optional:
// the form leaves any zero field exactly as the user left it, so a partial parse
// never wipes something already typed.
type TxnDraftFill struct {
	// AmountMajor is the amount as a major-unit decimal string, unsigned.
	AmountMajor string
	// Income is true when the sentence described money coming IN.
	Income bool
	Payee  string
	// Date is zero when the sentence named none.
	Date time.Time
	// CategoryID is set only by the Smart+ path, and only when the model named a
	// category the household actually has.
	CategoryID string
}

// Empty reports whether the parse found nothing worth applying.
func (f TxnDraftFill) Empty() bool {
	return f.AmountMajor == "" && f.Payee == "" && f.Date.IsZero() && f.CategoryID == ""
}

// TxnDraftHintProps carries the callback that fills the quick-add form.
type TxnDraftHintProps struct {
	OnApply func(TxnDraftFill)
}

// TxnDraftHint renders the sentence field and the fill affordance. Its own
// component so its state and request hooks sit at stable positions inside the
// quick-add panel.
func TxnDraftHint(props TxnDraftHintProps) ui.Node {
	pr := uistate.UsePrefs().Get()
	app := appstate.Default

	sentence := ui.UseState("")
	aiFill := ui.UseState(TxnDraftFill{})
	loading := ui.UseState(false)
	errText := ui.UseState("")

	settings := uistate.LoadSmartSettings()
	raw := strings.TrimSpace(sentence.Get())

	backendAI := app != nil && aiProviderConfigured(app, pr.Normalize().BackendActive())
	aiEnabled := backendAI && settings.IsEnabled(txnDraftAICode) && !settings.IsMuted(txnDraftAICode)
	freeEnabled := settings.IsEnabled(txnDraftFreeCode) && !settings.IsMuted(txnDraftFreeCode)
	aiConn := smartAIConn{}
	if app != nil {
		aiConn = resolveAIConn(app, pr.Normalize().BackendActive(), pr.ServerURL, pr.ServerToken)
	}

	local := parseTxnSentence(raw)

	onInput := ui.UseEvent(func(v string) {
		sentence.Set(v)
		aiFill.Set(TxnDraftFill{})
		errText.Set("")
	})
	applyLocal := func() {
		if props.OnApply != nil && !local.Empty() {
			props.OnApply(local)
		}
	}
	applyAI := func() {
		if props.OnApply != nil && !aiFill.Get().Empty() {
			props.OnApply(aiFill.Get())
		}
	}
	ask := ui.UseEvent(Prevent(func() {
		a := appstate.Default
		if a == nil || loading.Get() || raw == "" {
			return
		}
		loading.Set(true)
		errText.Set("")
		catalog := smartCatalog(a.Categories())
		runSmartAI(aiConn, smartai.TxnDraftRequest(raw, catalog.Prompt(), time.Now()),
			func(text string) {
				loading.Set(false)
				d, ok := smartai.ParseTxnDraft(strings.TrimSpace(text), catalog, time.Now())
				if !ok {
					errText.Set(uistate.T("sm15.aiNoAnswer"))
					return
				}
				aiFill.Set(TxnDraftFill{
					AmountMajor: d.AmountMajor,
					Income:      !d.Negative,
					Payee:       d.Payee,
					Date:        d.Date,
					CategoryID:  d.CategoryID,
				})
			},
			func(e string) { loading.Set(false); errText.Set(e) })
	}))

	if !freeEnabled && !aiEnabled {
		return Fragment()
	}
	if !settings.DensityOrDefault().Shows(smart.AffordanceFieldAssist) {
		return Fragment()
	}

	// The AI answer, when there is one, replaces the local offer: the user asked
	// for it explicitly and it is strictly the better-informed parse.
	offer := local
	fromAI := false
	if f := aiFill.Get(); !f.Empty() {
		offer, fromAI = f, true
	}

	var hint ui.Node = Fragment()
	if !offer.Empty() {
		detail := uistate.T("sm15.readLocal")
		code := txnDraftFreeCode
		if fromAI {
			detail = uistate.T("catSuggest.aiSource")
			code = txnDraftAICode
		}
		hint = smartRowHintFor(settings, code, smart.AffordanceFieldAssist, smartRowHintProps{
			Key:         "sm15:" + raw,
			TestID:      "sm15",
			Text:        txnDraftSummary(offer, pr.FormatDate),
			Detail:      detail,
			ActionLabel: uistate.T("sm15.action"),
			OnAction: func() {
				if fromAI {
					applyAI()
					return
				}
				applyLocal()
			},
		})
	}

	var askBtn ui.Node = Fragment()
	// The model is offered when it could add something the local parsers cannot:
	// a sentence they read nothing from, or one they read but could not categorize.
	if aiEnabled && raw != "" && !fromAI {
		label := uistate.T("sm15.aiAction")
		if loading.Get() {
			label = uistate.T("sm15.aiPending")
		}
		askBtn = Button(css.Class("btn btn-sm btn-ghost", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "sm15-ai-btn"), Attr("aria-disabled", ariaBool(loading.Get())),
			OnClick(ask), smartGlyph(true, tw.Fold(tw.W3, tw.H3)), Span(label))
	}

	return Div(css.Class("sm15"), Attr("data-testid", "sm15-draft"),
		Div(css.Class("sm15-row", tw.Flex, tw.ItemsCenter, tw.Gap2),
			uiw.Field(sentence.Get(), css.Class("field field-wide"), Type("text"),
				Attr("data-testid", "sm15-input"),
				Attr("aria-label", uistate.T("sm15.label")),
				Placeholder(uistate.T("sm15.placeholder")), OnInput(onInput)),
			askBtn,
		),
		hint,
		If(errText.Get() != "", P(css.Class("err", tw.Text12), Attr("role", "alert"), errText.Get())),
	)
}

// parseTxnSentence is the FREE parse: rapidcapture for the amount and the
// merchant words, duedate for the date word. Neither package knew about the
// other, and between them they read the sentence people actually type.
//
// The sign convention follows the app's: a bare "40 at whole foods" is money out.
// Only an explicit income word flips it, because guessing income from a bare
// number is the mistake that quietly corrupts the income/expense split.
func parseTxnSentence(raw string) TxnDraftFill {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return TxnDraftFill{}
	}
	out := TxnDraftFill{}
	// The date first, so its words are lifted out before the merchant label is read
	// and "yesterday" cannot end up as part of the payee.
	d := duedate.Parse(raw, time.Now())
	if !d.Due.IsZero() {
		out.Date = d.Due
	}
	rest := d.Title
	if rest == "" {
		rest = raw
	}
	drafts := rapidcapture.Parse(rest)
	if len(drafts) == 0 {
		return out
	}
	first := drafts[0]
	out.AmountMajor = first.MajorString
	out.Payee = cleanDraftLabel(first.Label)
	out.Income = txnSentenceIsIncome(rest)
	return out
}

// incomeWords are the only signals that flip a sentence to money IN.
var incomeWords = []string{"got paid", "received", "refund", "paid me", "earned", "income", "deposit"}

// txnSentenceIsIncome reports whether the sentence describes money coming in.
func txnSentenceIsIncome(s string) bool {
	low := strings.ToLower(s)
	for _, w := range incomeWords {
		if strings.Contains(low, w) {
			return true
		}
	}
	return false
}

// leadWords are the verbs and prepositions that glue a capture sentence together
// and are never part of a merchant's name.
var leadWords = map[string]bool{
	"spent": true, "paid": true, "bought": true, "at": true, "on": true, "for": true,
	"from": true, "to": true, "in": true, "a": true, "an": true, "the": true, "got": true,
}

// cleanDraftLabel strips the sentence's connective words from the merchant label,
// leaving "whole foods" rather than "spent at whole foods". Words are removed only
// from the ENDS: a merchant genuinely called "The Home Depot" keeps its "The"
// because the stripping stops at the first word that isn't connective.
func cleanDraftLabel(s string) string {
	words := strings.Fields(s)
	for len(words) > 0 && leadWords[strings.ToLower(words[0])] {
		words = words[1:]
	}
	for len(words) > 0 && leadWords[strings.ToLower(words[len(words)-1])] {
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}

// txnDraftSummary renders the parse as the one line the hint shows.
func txnDraftSummary(f TxnDraftFill, fmtDate func(time.Time) string) string {
	parts := []string{}
	if f.Payee != "" {
		parts = append(parts, f.Payee)
	}
	if f.AmountMajor != "" {
		if f.Income {
			parts = append(parts, uistate.T("sm15.amountIn", f.AmountMajor))
		} else {
			parts = append(parts, uistate.T("sm15.amountOut", f.AmountMajor))
		}
	}
	if !f.Date.IsZero() && fmtDate != nil {
		parts = append(parts, fmtDate(f.Date))
	}
	return strings.Join(parts, " · ")
}

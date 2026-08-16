// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aiprovider"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/toolperm"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type keyExplainerProps struct {
	// BaseURL is the configured endpoint, used to identify which provider's
	// prices and key page to quote. Empty means the default (OpenAI).
	BaseURL string
}

// KeyExplainer is the shared explanation shown wherever a feature is waiting for an
// API key (C247).
//
// The old gate said "Add your OpenAI key in Settings" and stopped there, which
// leaves the three questions that actually decide the matter unanswered: what will
// it cost, where do I get one, and what leaves my device. Someone weighing those up
// with no figures in front of them will assume the worst of all three, and the
// assistant stays switched off for reasons that were never true.
//
// So the card answers them in that order — cost first, because it is the one people
// ask first — and every answer is specific: a real ballpark from the provider's own
// pricing, a real link, and a plain statement of what is sent. Where a price is not
// known the line is omitted rather than guessed; an invented number here would be
// the one thing worse than silence.
func KeyExplainer(p keyExplainerProps) ui.Node {
	open := ui.UseEvent(Prevent(func() { uistate.OpenGlobalSettingsAt("ai") }))

	// An unrecognised endpoint (a local model, a proxy, something hand-entered)
	// gets the explanation without the figures: quoting OpenAI's prices for a
	// service that isn't OpenAI would be worse than quoting none.
	provider, known := aiprovider.ForBaseURL(p.BaseURL)

	costLine := Fragment()
	if cost, priced := provider.CheapestTypicalQuestionUSD(); priced && known {
		costLine = keyExplainerPoint(icon.Receipt,
			uistate.T("assistant.keyCost", ai.FormatCostUSD(cost), provider.Label))
	}
	whereLine := Fragment()
	if provider.KeyURL != "" && known {
		whereLine = Li(css.Class("asst-key-point"),
			uiw.Icon(icon.Landmark, css.Class("asst-key-icon", tw.ShrinkO, tw.W35, tw.H35)),
			Span(uistate.T("assistant.keyWhere")+" "),
			A(Href(provider.KeyURL), Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
				Attr("data-testid", "assistant-key-link"), provider.KeyURL),
		)
	}

	return Div(css.Class("asst-key-callout"), Attr("data-testid", "assistant-key-callout"),
		Attr("role", "group"), Attr("aria-label", uistate.T("assistant.keyGateAria")),
		P(css.Class("asst-key-lead"), uistate.T("assistant.keyCallout")),
		Ul(css.Class("asst-key-points"),
			costLine,
			whereLine,
			keyExplainerPoint(icon.Lock, uistate.T("assistant.keyPrivacy")),
		),
		capabilitySheet(),
		Button(css.Class("btn btn-sm btn-primary"), Type("button"),
			Attr("data-testid", "assistant-key-settings"), OnClick(open), uistate.T("assistant.keyAdd")),
	)
}

// capabilitySheet lists everything the assistant is able to CHANGE (PS7).
//
// Piggy's "the AI can only ever save your birthday and a support ticket" is a good
// trust pattern precisely because it is a short, checkable list. Ours is longer, so
// the honest version leads with the part that actually matters — the two things it
// can do that cannot be undone — and puts the full list behind a disclosure.
//
// It is generated from the same table the approval cards read. A hand-written "what
// the AI can do" page is a promise that quietly stops being true the first time
// somebody adds a tool; this one cannot drift, because adding a tool changes it.
func capabilitySheet() ui.Node {
	all := toolperm.Capabilities()
	permanent := toolperm.PermanentCapabilities()
	if len(all) == 0 {
		return Fragment()
	}
	rows := make([]any, 0, len(all))
	for _, c := range all {
		for _, w := range c.Writes {
			line := w
			if !c.Reversible {
				line += " " + uistate.T("assistant.capPermanentMark")
			}
			rows = append(rows, Li(css.Class("asst-cap-row"), line))
		}
	}
	list := append([]any{css.Class("asst-cap-list")}, rows...)
	return Details(css.Class("asst-cap"), Attr("data-testid", "assistant-capabilities"),
		Summary(css.Class("asst-cap-summary"), uistate.T("assistant.capSummary", len(all), len(permanent))),
		P(css.Class("asst-cap-lead"), uistate.T("assistant.capLead")),
		Ul(list...),
	)
}

// keyExplainerPoint renders one labelled point of the explanation. It takes no
// handlers, so it is safe to build inline rather than as its own component.
func keyExplainerPoint(name icon.Name, text string) ui.Node {
	return Li(css.Class("asst-key-point"),
		uiw.Icon(name, css.Class("asst-key-icon", tw.ShrinkO, tw.W35, tw.H35)),
		Span(text),
	)
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// sharePreviewProps carries what the NEXT assistant message would send — shown
// BEFORE anything leaves the device (the agent receipt is post-hoc; this is the
// pre-send half of the disclosure).
type sharePreviewProps struct {
	Tier         aicontext.ConversationTier
	CtxLine      string // the aggregates line injected as live context
	Categories   int    // category names shared for grounding
	CustomFields int    // custom field defs summarized
	MemoryOn     bool   // durable agent memory rides every turn
	Turns        int    // conversation turns re-sent with each message
	EstTokens    int    // rough size of the standing context + history (chars/4)
}

// sharePreview is the "What's shared?" disclosure in the assistant control
// cell: a chip that expands into a plain-English list of exactly what the next
// message will carry — the aggregates line, category/custom-field names, memory,
// the conversation so far — plus a rough token estimate, so the cost/privacy
// story is visible before sending, not just in the post-hoc receipt.
func sharePreview(props sharePreviewProps) ui.Node {
	return ui.CreateElement(sharePreviewComp, props)
}

func sharePreviewComp(props sharePreviewProps) ui.Node {
	open := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))

	tierLine := uistate.T("insights.shareTierFull")
	if props.Tier == aicontext.TierAggregatesOnly {
		tierLine = uistate.T("insights.shareTierAgg")
	}

	var panel ui.Node = Fragment()
	if open.Get() {
		line := func(text string) ui.Node { return Div(css.Class("t-caption"), text) }
		panel = Div(css.Class("ask-share-panel"), Attr("data-testid", "assistant-share-panel"),
			Style(map[string]string{"flex": "1 1 100%", "padding": "0.5rem 0.65rem", "margin-top": "0.35rem",
				"border": "1px dashed var(--border)", "border-radius": "8px", "display": "grid", "gap": "0.2rem"}),
			Div(css.Class("t-caption", tw.TextDim), uistate.T("insights.shareHeading")),
			line(tierLine),
			line(uistate.T("insights.shareAggregates", props.CtxLine)),
			line(uistate.T("insights.shareNames", props.Categories, props.CustomFields)),
			If(props.MemoryOn, line(uistate.T("insights.shareMemory"))),
			If(props.Turns > 0, line(uistate.T("insights.shareTurns", props.Turns))),
			Div(css.Class("t-caption", tw.TextDim), Attr("data-testid", "assistant-share-tokens"),
				uistate.T("insights.shareTokens", strconv.Itoa(props.EstTokens))),
		)
	}
	return Fragment(
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "assistant-share-chip"),
			Attr("aria-expanded", ariaBool(open.Get())), Title(uistate.T("insights.shareChipTitle")),
			OnClick(toggle), uistate.T("insights.shareChip")),
		panel,
	)
}

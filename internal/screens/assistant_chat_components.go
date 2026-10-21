// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/chatpolish"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// assistantAskHeaderProps are the already-resolved presentation and actions for
// the Ask header. The component owns only its event hooks; state stays with the
// Insights controller.
type assistantAskHeaderProps struct {
	StatusClass      string
	StatusKey        string
	ControlsOpen     bool
	AsideOpen        bool
	OnNewChat        func()
	OnToggleControls func()
	OnToggleAside    func()
}

// assistantAskHeader renders the title, status and three Ask-surface actions.
func assistantAskHeader(p assistantAskHeaderProps) ui.Node {
	newChat := ui.UseEvent(Prevent(func() {
		if p.OnNewChat != nil {
			p.OnNewChat()
		}
	}))
	toggleControls := ui.UseEvent(Prevent(func() {
		if p.OnToggleControls != nil {
			p.OnToggleControls()
		}
	}))
	toggleAside := ui.UseEvent(Prevent(func() {
		if p.OnToggleAside != nil {
			p.OnToggleAside()
		}
	}))

	return Div(css.Class("ask-head"),
		Div(css.Class("ask-head-row"),
			Div(css.Class("ask-head-id"),
				Span(ClassStr(p.StatusClass), Attr("aria-hidden", "true")),
				H2(css.Class("ask-title"), uistate.T("assistant.agentTitle")),
			),
			Div(css.Class("ask-head-actions"),
				Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "assistant-new-chat"), OnClick(newChat),
					uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("insights.newChat"))),
				Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "assistant-settings-toggle"),
					Attr("aria-expanded", ariaBool(p.ControlsOpen)), Attr("aria-controls", "ask-settings-drawer"),
					OnClick(toggleControls),
					uiw.Icon(icon.Settings, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("assistant.chatSettings"))),
				Button(css.Class("btn btn-tool ask-aside-toggle"), Type("button"), Attr("data-testid", "assistant-aside-toggle"),
					Attr("aria-expanded", ariaBool(p.AsideOpen)), OnClick(toggleAside),
					uiw.Icon(icon.MessageCircle, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("assistant.notesDrawer"))),
			),
		),
		Span(css.Class("ask-status"), uistate.T(p.StatusKey)),
	)
}

// assistantChatThreadProps contains prepared turns and the controller's actions.
// The thread does not read app state or perform conversation mutations.
type assistantChatThreadProps struct {
	Turns      []chatTurn
	Model      string
	Loading    bool
	Streaming  string
	OnDelete   func(string)
	OnRetry    func()
	OnEdit     func(id, text string)
	OnPin      func(string) bool
	OnFeedback func(id string, feedback chatpolish.Feedback)
}

// assistantChatThread renders keyed turns and the in-flight response. Existing
// bubble components own their row hooks; retries remain available only on the
// latest turn while the request is idle.
func assistantChatThread(p assistantChatThreadProps) ui.Node {
	lastID := ""
	if n := len(p.Turns); n > 0 {
		lastID = p.Turns[n-1].ID
	}
	retryFor := func(tid string) func() {
		if tid == lastID && !p.Loading {
			return p.OnRetry
		}
		return nil
	}

	return Div(Attr("id", "cf-chat-thread"), css.Class("chat-thread", tw.Flex, tw.FlexCol, tw.Gap4),
		MapKeyed(p.Turns,
			func(t chatTurn) any { return t.ID },
			func(t chatTurn) ui.Node {
				if t.Role == "user" {
					return ui.CreateElement(UserBubble, userBubbleProps{
						ID: t.ID, Text: t.Text, OnDelete: p.OnDelete, OnRetry: retryFor(t.ID), OnEdit: p.OnEdit,
					})
				}
				if t.Role == "afford" {
					return ui.CreateElement(AffordResultBubble, affordResultBubbleProps{ID: t.ID, HTML: t.Text, OnDelete: p.OnDelete})
				}
				return ui.CreateElement(AssistantBubble, asstBubbleProps{
					ID: t.ID, Text: t.Text, Usage: t.Usage, Model: p.Model, Sources: t.Sources, Feedback: t.Feedback,
					OnPin: p.OnPin, OnDelete: p.OnDelete, OnRetry: retryFor(t.ID), OnFeedback: p.OnFeedback,
				})
			},
		),
		If(p.Loading && strings.TrimSpace(p.Streaming) == "", Div(css.Class(tw.Flex, tw.JustifyStart),
			Div(css.Class("chat-row-agent"),
				Div(css.Class("chat-avatar"), Attr("aria-hidden", "true"), "✦"),
				Div(css.Class("insights-thinking chat-thinking", tw.Text13, tw.TextFaint), uistate.T("insights.thinking")),
			),
		)),
		If(p.Loading && strings.TrimSpace(p.Streaming) != "",
			ui.CreateElement(StreamingBubble, streamingBubbleProps{Text: p.Streaming})),
	)
}

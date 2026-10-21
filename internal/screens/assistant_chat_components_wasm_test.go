// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestAssistantAskHeaderPreservesActionsAndExpandedState(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	var newChats, controlToggles, asideToggles int
	props := assistantAskHeaderProps{
		StatusClass: "chat-status-dot is-local", StatusKey: "assistant.statusLocal",
		ControlsOpen: true, AsideOpen: false,
		OnNewChat:        func() { newChats++ },
		OnToggleControls: func() { controlToggles++ },
		OnToggleAside:    func() { asideToggles++ },
	}
	f.Render(ui.CreateElement(assistantAskHeader, props))
	f.Stabilize()

	if got := byTestID(f, "button", "assistant-settings-toggle"); got == nil || got.Attr("aria-expanded") != "true" || got.Attr("aria-controls") != "ask-settings-drawer" {
		t.Fatalf("settings control lost its expanded state/target: %#v", got)
	}
	if got := byTestID(f, "button", "assistant-aside-toggle"); got == nil || got.Attr("aria-expanded") != "false" {
		t.Fatalf("aside control expanded state = %v, want false", got)
	}
	statusFound := false
	for _, node := range f.AllByTag("span") {
		if node.Attr("class") == props.StatusClass {
			statusFound = true
		}
	}
	if !statusFound || !strings.Contains(f.Text(), uistate.T(props.StatusKey)) {
		t.Fatal("header status indicator or translated subtitle was not rendered")
	}
	if byTestID(f, "button", "assistant-new-chat") == nil {
		t.Fatal("New chat action missing")
	}
	byTestID(f, "button", "assistant-new-chat").Click()
	byTestID(f, "button", "assistant-settings-toggle").Click()
	byTestID(f, "button", "assistant-aside-toggle").Click()
	f.Stabilize()
	if newChats != 1 || controlToggles != 1 || asideToggles != 1 {
		t.Fatalf("callbacks fired new=%d controls=%d aside=%d, want one each", newChats, controlToggles, asideToggles)
	}
}

func TestAssistantChatThreadKeepsEditingStateWithTurnIDWhenOrderChanges(t *testing.T) {
	stubBudgetRowBrowser(t)
	t.Cleanup(func() { time.Sleep(650 * time.Millisecond) })
	f := render.New(t)
	t.Cleanup(f.Cleanup)
	props := assistantChatThreadProps{Turns: []chatTurn{
		{ID: "turn-a", Role: "user", Text: "Prompt A"},
		{ID: "turn-b", Role: "user", Text: "Prompt B"},
	}, OnEdit: func(string, string) {}}
	f.Render(ui.CreateElement(assistantChatThread, props))
	f.Stabilize()
	edits := f.AllByTag("button")
	for _, button := range edits {
		if button.Attr("data-testid") == "assistant-edit-msg" {
			button.Click()
			break
		}
	}
	f.Stabilize()
	if byTestID(f, "div", "assistant-edit-box") == nil {
		t.Fatal("first turn did not enter its edit state")
	}

	props.Turns = []chatTurn{props.Turns[1], props.Turns[0]}
	f.Render(ui.CreateElement(assistantChatThread, props))
	f.Stabilize()
	input := byTestID(f, "textarea", "assistant-edit-input")
	// FieldValue is carried on data-cf-value and synchronized to the live DOM by
	// the shared field bridge; the mock renderer's generic value property is not
	// the source of truth for these controlled fields.
	if input == nil || input.Attr("data-cf-value") != "Prompt A" {
		t.Fatal("editing draft did not stay with its keyed turn after reordering")
	}
	normalB := false
	for _, node := range f.AllByTag("div") {
		if strings.Contains(node.Attr("class"), "group") && strings.Contains(node.Text(), "Prompt B") {
			normalB = true
			break
		}
	}
	if !normalB {
		t.Fatal("the non-editing row no longer belongs to turn B after reordering")
	}
}

func TestAssistantChatThreadKeepsKeyedTurnActionsAndLoadingStates(t *testing.T) {
	stubBudgetRowBrowser(t)
	t.Cleanup(func() { time.Sleep(650 * time.Millisecond) })
	f := render.New(t)
	t.Cleanup(f.Cleanup)
	var deleted string
	var retried int
	props := assistantChatThreadProps{
		Turns:    []chatTurn{{ID: "latest-user", Role: "user", Text: "Review groceries"}},
		OnDelete: func(id string) { deleted = id },
		OnRetry:  func() { retried++ },
	}
	f.Render(ui.CreateElement(assistantChatThread, props))
	f.Stabilize()
	threadFound := false
	for _, node := range f.AllByTag("div") {
		if node.Attr("id") == "cf-chat-thread" && node.Attr("class") != "" {
			threadFound = true
			break
		}
	}
	if !threadFound {
		t.Fatal("thread root id/class contract was not rendered")
	}
	var retry *render.QueryNode
	for _, button := range f.AllByTag("button") {
		if button.Attr("aria-label") == uistate.T("insights.retry") {
			retry = button
			break
		}
	}
	if retry == nil {
		t.Fatal("idle latest user turn has no Retry action")
	}
	retry.Click()
	for _, button := range f.AllByTag("button") {
		if button.Attr("aria-label") == uistate.T("insights.deleteMsg") {
			button.Click()
			break
		}
	}
	f.Stabilize()
	if retried != 1 || deleted != "latest-user" {
		t.Fatalf("thread actions retry=%d deleted=%q; want retry once and delete latest-user", retried, deleted)
	}

	props.Loading = true
	props.OnRetry = func() { retried++ }
	f.Render(ui.CreateElement(assistantChatThread, props))
	f.Stabilize()
	for _, button := range f.AllByTag("button") {
		if button.Attr("aria-label") == uistate.T("insights.retry") {
			t.Fatal("retry remained available while a request was loading")
		}
	}
	if !strings.Contains(f.Text(), uistate.T("insights.thinking")) || byTestID(f, "div", "assistant-streaming") != nil {
		t.Fatal("blank in-flight response should show the thinking line only")
	}

	props.Streaming = "partial answer"
	f.Render(ui.CreateElement(assistantChatThread, props))
	f.Stabilize()
	if byTestID(f, "div", "assistant-streaming") == nil || strings.Contains(f.Text(), uistate.T("insights.thinking")) {
		t.Fatal("streaming response should replace the thinking line")
	}
}

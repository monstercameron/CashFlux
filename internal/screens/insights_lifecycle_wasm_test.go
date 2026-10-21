// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
	"syscall/js"
)

type insightsLifecycleCallbacks struct {
	delta       func(string)
	result      func(ai.Message, ai.Usage)
	err         func(string)
	cancelCount int
	model       string
	messages    []ai.Message
	tools       []ai.Tool
}

func setupInsightsLifecycle(t *testing.T, conversations ...domain.Conversation) (*appstate.App, *render.Fixture, js.Value, *[]insightsLifecycleCallbacks) {
	t.Helper()
	// The Node WASM runner has no browser globals. Install the package's minimal
	// browser surface before prefs or render code touches document/local storage.
	stubBudgetRowBrowser(t)
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	oldDefault := appstate.Default
	appstate.Default = app
	t.Cleanup(func() {
		appstate.Default = oldDefault
		_ = app.Store().Close()
	})
	for _, conversation := range conversations {
		if err := app.PutConversation(conversation); err != nil {
			t.Fatal(err)
		}
	}
	settings := app.Settings()
	settings.OpenAIKey = ""
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	oldBaseline, oldReceipt := auditview.CaptureAssistantBaseline, auditview.CaptureAssistantReceipt
	oldBaselineChecked, oldReceiptChecked := auditview.CaptureAssistantBaselineChecked, auditview.CaptureAssistantReceiptChecked
	auditview.CaptureAssistantBaseline = func() bool { return true }
	auditview.CaptureAssistantReceipt = func() (string, bool) { return "mock-undo", true }
	auditview.CaptureAssistantBaselineChecked = func() error { return nil }
	auditview.CaptureAssistantReceiptChecked = func() (string, bool, error) { return "mock-undo", true, nil }
	t.Cleanup(func() {
		auditview.CaptureAssistantBaseline, auditview.CaptureAssistantReceipt = oldBaseline, oldReceipt
		auditview.CaptureAssistantBaselineChecked, auditview.CaptureAssistantReceiptChecked = oldBaselineChecked, oldReceiptChecked
	})
	oldPrefs := uistate.CurrentPrefs()
	if err := uistate.SetPrefs(prefs.Prefs{ServerURL: "http://mock.invalid", ServerToken: "test-token"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = uistate.SetPrefs(oldPrefs) })

	transport := assistantChatTransport
	t.Cleanup(func() { assistantChatTransport = transport })
	callbacks := []insightsLifecycleCallbacks{}
	assistantChatTransport = func(
		_ bool, _, _, _, _, model string, messages []ai.Message, _ float64, _ string, tools []ai.Tool,
		delta func(string), result func(ai.Message, ai.Usage), onErr func(string),
	) func() {
		callbacks = append(callbacks, insightsLifecycleCallbacks{
			delta: delta, result: result, err: onErr, model: model,
			messages: append([]ai.Message(nil), messages...), tools: append([]ai.Tool(nil), tools...),
		})
		idx := len(callbacks) - 1
		return func() { callbacks[idx].cancelCount++ }
	}

	doc := js.Global().Get("document")
	previousGet := doc.Get("getElementById")
	composer := js.Global().Get("Object").New()
	composer.Set("value", "")
	markdown := map[string]js.Value{}
	get := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].String() == "cf-chat-input" {
			return composer
		}
		if len(args) > 0 && strings.HasPrefix(args[0].String(), "cf-md-") {
			key := args[0].String()
			el, ok := markdown[key]
			if !ok {
				el = js.Global().Get("Object").New()
				el.Set("innerHTML", "")
				el.Set("dataset", js.Global().Get("Object").New())
				markdown[key] = el
			}
			return el
		}
		return js.Null()
	})
	doc.Set("getElementById", get)
	t.Cleanup(func() { doc.Set("getElementById", previousGet); get.Release() })
	// Insights schedules deferred rail and scroll callbacks. Let them drain after
	// the fixture unmounts but before the minimal document globals are restored.
	t.Cleanup(func() { time.Sleep(350 * time.Millisecond) })

	f := render.New(t)
	f.Render(ui.CreateElement(Insights))
	f.Stabilize()
	t.Cleanup(f.Cleanup)
	return app, f, composer, &callbacks
}

func submitInsightsLifecycleTurn(t *testing.T, f *render.Fixture, composer js.Value, text string, callbacks *[]insightsLifecycleCallbacks) int {
	t.Helper()
	start := len(*callbacks)
	composer.Set("value", text)
	byTestID(f, "button", "assistant-send").Click()
	deadline := time.Now().Add(2 * time.Second)
	for len(*callbacks) == start && time.Now().Before(deadline) {
		f.Stabilize()
		time.Sleep(time.Millisecond)
	}
	if len(*callbacks) == start {
		t.Fatal("mock provider was not called")
	}
	return len(*callbacks) - 1
}

func waitForInsightsCallbackCount(t *testing.T, f *render.Fixture, callbacks *[]insightsLifecycleCallbacks, count int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for len(*callbacks) < count && time.Now().Before(deadline) {
		f.Stabilize()
		time.Sleep(time.Millisecond)
	}
	if len(*callbacks) < count {
		t.Fatalf("provider callback count = %d, want at least %d", len(*callbacks), count)
	}
}

func waitForInsightsConversation(t *testing.T, f *render.Fixture, app *appstate.App) domain.Conversation {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if conversations := app.Conversations(); len(conversations) > 0 {
			return conversations[0]
		}
		f.Stabilize()
		time.Sleep(time.Millisecond)
	}
	t.Fatal("conversation was not persisted before the provider response")
	return domain.Conversation{}
}

func TestInsightsSessionRejectsLateCallbacksAfterNewChatAndUnmount(t *testing.T) {
	app, f, composer, callbacks := setupInsightsLifecycle(t)
	first := submitInsightsLifecycleTurn(t, f, composer, "first request", callbacks)
	deadline := time.Now().Add(2 * time.Second)
	for len(app.Conversations()) != 1 && time.Now().Before(deadline) {
		f.Stabilize()
		time.Sleep(time.Millisecond)
	}
	if len(app.Conversations()) != 1 {
		t.Fatalf("saved conversations after first user turn = %d, want 1", len(app.Conversations()))
	}
	conversationID := app.Conversations()[0].ID
	byTestID(f, "button", "assistant-new-chat").Click()
	f.Stabilize()
	if (*callbacks)[first].cancelCount == 0 {
		t.Fatal("New chat did not cancel the outstanding provider request")
	}
	(*callbacks)[first].delta("late text")
	(*callbacks)[first].result(ai.Message{Role: ai.RoleAssistant, Content: "late answer"}, ai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15})
	(*callbacks)[first].result(ai.Message{Role: ai.RoleAssistant, Content: "duplicate late answer"}, ai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15})
	f.Stabilize()
	if summary := uistate.AgentReceiptSummary(conversationID); summary == "" {
		t.Fatal("late provider usage was not recorded against its original conversation")
	}
	if got := uistate.AgentTokensUsed(conversationID); got != 15 {
		t.Fatalf("late callback usage was recorded %d tokens, want one result's 15", got)
	}
	if got := app.AISpendThisMonth().Tokens; got != 15 {
		t.Fatalf("late callback household usage = %d tokens, want 15", got)
	}
	for _, message := range app.Conversations()[0].Messages {
		if message.Role == "assistant" && strings.Contains(message.Text, "late answer") {
			t.Fatal("late callback persisted an answer after New chat")
		}
	}

	second := submitInsightsLifecycleTurn(t, f, composer, "unmount request", callbacks)
	deadline = time.Now().Add(2 * time.Second)
	secondConversationID := ""
	for time.Now().Before(deadline) {
		for _, conversation := range app.Conversations() {
			if conversation.ID != conversationID {
				secondConversationID = conversation.ID
			}
		}
		if secondConversationID != "" {
			break
		}
		f.Stabilize()
		time.Sleep(time.Millisecond)
	}
	if secondConversationID == "" {
		t.Fatal("new chat did not get its own conversation id before unmount")
	}
	f.Render(ui.CreateElement(func() ui.Node { return Div() }))
	f.Stabilize()
	if (*callbacks)[second].cancelCount == 0 {
		t.Fatal("unmount did not cancel the outstanding provider request")
	}
	(*callbacks)[second].delta("after unmount")
	(*callbacks)[second].result(ai.Message{Role: ai.RoleAssistant, Content: "after unmount"}, ai.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2})
	if got := uistate.AgentTokensUsed(conversationID); got != 15 {
		t.Fatalf("unmounted callback changed prior conversation usage to %d tokens, want 15", got)
	}
	if got := uistate.AgentTokensUsed(secondConversationID); got != 2 {
		t.Fatalf("unmounted callback usage = %d tokens on its original conversation, want 2", got)
	}
	if got := app.AISpendThisMonth().Tokens; got != 17 {
		t.Fatalf("household usage after both late callbacks = %d tokens, want 17", got)
	}
}

func TestInsightsSessionCancelsWhenSwitchingConversation(t *testing.T) {
	older := domain.Conversation{ID: "older-" + id.New(), Title: "Older chat", CreatedAt: time.Now().Add(-time.Hour), UpdatedAt: time.Now().Add(-time.Hour)}
	active := domain.Conversation{ID: "active-" + id.New(), Title: "Active chat", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	app, f, composer, callbacks := setupInsightsLifecycle(t, older, active)
	request := submitInsightsLifecycleTurn(t, f, composer, "request in active chat", callbacks)
	byTestID(f, "button", "assistant-note-convs").Click()
	f.Stabilize()
	switched := false
	for _, button := range f.AllByTag("button") {
		if button.Text() == older.Title {
			button.Click()
			switched = true
			break
		}
	}
	if !switched {
		t.Fatalf("saved conversation %q was not rendered: %s", older.Title, f.Text())
	}
	f.Stabilize()
	if (*callbacks)[request].cancelCount == 0 {
		t.Fatal("switching conversations did not cancel the active provider request")
	}
	(*callbacks)[request].result(ai.Message{Role: ai.RoleAssistant, Content: "stale switched answer"}, ai.Usage{PromptTokens: 3, CompletionTokens: 3, TotalTokens: 6})
	f.Stabilize()
	if summary := uistate.AgentReceiptSummary(active.ID); summary == "" {
		t.Fatal("late provider usage was not recorded against the original conversation")
	}
	if got := uistate.AgentTokensUsed(active.ID); got != 6 {
		t.Fatalf("late switched callback usage = %d tokens on original conversation, want 6", got)
	}
	if got := uistate.AgentTokensUsed(older.ID); got != 0 {
		t.Fatalf("late switched callback leaked %d tokens into selected conversation", got)
	}
	for _, conversation := range app.Conversations() {
		if conversation.ID != active.ID {
			continue
		}
		for _, message := range conversation.Messages {
			if message.Role == "assistant" && strings.Contains(message.Text, "stale switched answer") {
				t.Fatal("late result persisted into the switched-away conversation")
			}
		}
	}
}

func TestInsightsPrivacyChangeCancelsFullTierToolResponse(t *testing.T) {
	// Seed the active conversation from the user's saved default before Insights
	// mounts. Reading/changing the privacy atom here would call a UI hook outside
	// its render context and would bypass the actual control under test.
	stubBudgetRowBrowser(t)
	previousDefault := uistate.DefaultPrivacyTier()
	uistate.PersistDefaultPrivacyTier(aicontext.TierFull)
	t.Cleanup(func() { uistate.PersistDefaultPrivacyTier(previousDefault) })
	app, f, composer, callbacks := setupInsightsLifecycle(t)
	f.Stabilize()
	request := submitInsightsLifecycleTurn(t, f, composer, "privacy switch during request", callbacks)
	waitForInsightsCallbackCount(t, f, callbacks, request+1)
	conversationID := waitForInsightsConversation(t, f, app).ID
	privacyBadge := byTestID(f, "button", "assistant-privacy-badge")
	if privacyBadge == nil || !strings.Contains(privacyBadge.Text(), "Full detail") {
		t.Fatalf("privacy badge before toggle = %v, want full detail", privacyBadge)
	}
	byTestID(f, "button", "assistant-privacy-badge").Click()
	f.Stabilize()
	privacyBadge = byTestID(f, "button", "assistant-privacy-badge")
	if privacyBadge == nil || !strings.Contains(privacyBadge.Text(), "Aggregates only") {
		t.Fatalf("privacy badge after toggle = %v, want aggregates only", privacyBadge)
	}
	if (*callbacks)[request].cancelCount == 0 {
		t.Fatal("lowering the privacy tier did not cancel the full-tier request")
	}
	(*callbacks)[request].result(ai.Message{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{
		ID: "late-private-write", Type: "function", Function: ai.FunctionCall{Name: "add_account", Arguments: `{"name":"Late Privacy Account","class":"asset"}`},
	}}}, ai.Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7})
	f.Stabilize()
	for _, account := range app.Accounts() {
		if account.Name == "Late Privacy Account" {
			t.Fatal("late full-tier tool response mutated data after privacy reduction")
		}
	}
	if byTestID(f, "div", "assistant-approval") != nil {
		t.Fatal("late full-tier tool response opened an approval after privacy reduction")
	}
	if got := uistate.AgentTokensUsed(conversationID); got != 7 {
		t.Fatalf("late provider usage = %d tokens on original conversation, want 7", got)
	}
	if got := uistate.AgentReceiptActionCount(conversationID); got != 0 {
		t.Fatalf("late full-tier tool response recorded %d actions, want none", got)
	}
}

func TestInsightsUsageIsKeptWhenLaterProviderStepErrors(t *testing.T) {
	app, f, composer, callbacks := setupInsightsLifecycle(t)
	first := submitInsightsLifecycleTurn(t, f, composer, "use a read tool", callbacks)
	conversationID := waitForInsightsConversation(t, f, app).ID
	(*callbacks)[first].result(ai.Message{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{
		ID: "categories", Type: "function", Function: ai.FunctionCall{Name: "list_categories", Arguments: `{}`},
	}}}, ai.Usage{PromptTokens: 4, CompletionTokens: 2, TotalTokens: 6})
	waitForInsightsCallbackCount(t, f, callbacks, 2)
	(*callbacks)[1].err("provider failed on the follow-up")
	f.Stabilize()
	if got := uistate.AgentTokensUsed(conversationID); got != 6 {
		t.Fatalf("usage retained after the follow-up error = %d tokens, want 6", got)
	}
	if got := app.AISpendThisMonth().Tokens; got != 6 {
		t.Fatalf("household usage retained after the follow-up error = %d tokens, want 6", got)
	}
}

func TestInsightsBudgetStopsBeforeAnotherToolLoopRequest(t *testing.T) {
	app, f, composer, callbacks := setupInsightsLifecycle(t)
	byTestID(f, "button", "assistant-settings-toggle").Click()
	byTestID(f, "select", "assistant-budget").Change("25000")
	f.Stabilize()
	first := submitInsightsLifecycleTurn(t, f, composer, "use a read tool", callbacks)
	conversationID := waitForInsightsConversation(t, f, app).ID
	(*callbacks)[first].result(ai.Message{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{
		ID: "categories-at-cap", Type: "function", Function: ai.FunctionCall{Name: "list_categories", Arguments: `{}`},
	}}}, ai.Usage{PromptTokens: 20000, CompletionTokens: 5000, TotalTokens: 25000})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f.Stabilize()
		if strings.Contains(strings.ToLower(f.Text()), "token cap") {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(*callbacks) != 1 {
		t.Fatalf("provider calls after reaching the per-chat cap = %d, want 1", len(*callbacks))
	}
	if got := uistate.AgentTokensUsed(conversationID); got != 25000 {
		t.Fatalf("usage at per-chat cap = %d tokens, want 25000", got)
	}
	if got := app.AISpendThisMonth().Tokens; got != 25000 {
		t.Fatalf("household usage at per-chat cap = %d tokens, want 25000", got)
	}
}

func TestInsightsRefreshesToolHandlersAfterApprovedWriteInSameResponse(t *testing.T) {
	app, f, composer, callbacks := setupInsightsLifecycle(t)
	first := submitInsightsLifecycleTurn(t, f, composer, "create and list a category", callbacks)
	conversationID := waitForInsightsConversation(t, f, app).ID
	(*callbacks)[first].result(ai.Message{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{
		{ID: "create-category", Type: "function", Function: ai.FunctionCall{Name: "create_category", Arguments: `{"name":"Fresh Handler Category","kind":"expense"}`}},
		{ID: "list-categories", Type: "function", Function: ai.FunctionCall{Name: "list_categories", Arguments: `{}`}},
	}}, ai.Usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5})
	waitForInsightsApproval(t, f)
	byTestID(f, "button", "assistant-approve").Click()
	waitForInsightsCallbackCount(t, f, callbacks, 2)
	foundFreshCategory := false
	for _, message := range (*callbacks)[1].messages {
		if message.Role == ai.RoleTool && message.Name == "list_categories" && strings.Contains(message.Content, "Fresh Handler Category") {
			foundFreshCategory = true
		}
	}
	if !foundFreshCategory {
		t.Fatalf("second tool result did not see the approved category write: %#v", (*callbacks)[1].messages)
	}
	(*callbacks)[1].result(ai.Message{Role: ai.RoleAssistant, Content: "category created"}, ai.Usage{PromptTokens: 2, CompletionTokens: 2, TotalTokens: 4})
	deadline := time.Now().Add(2 * time.Second)
	for uistate.AgentReceiptActionCount(conversationID) == 0 && time.Now().Before(deadline) {
		f.Stabilize()
		time.Sleep(time.Millisecond)
	}
	if got := uistate.AgentReceiptActionCount(conversationID); got != 1 {
		t.Fatalf("approved category action count = %d, want 1", got)
	}
}

func waitForInsightsApproval(t *testing.T, f *render.Fixture) *render.QueryNode {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f.Stabilize()
		if approval := byTestID(f, "div", "assistant-approval"); approval != nil {
			return approval
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("mutating proposal did not wait for approval: %s", f.Text())
	return nil
}

func TestInsightsCanceledApprovalCannotMutate(t *testing.T) {
	app, f, composer, callbacks := setupInsightsLifecycle(t)
	request := submitInsightsLifecycleTurn(t, f, composer, "propose an account", callbacks)
	(*callbacks)[request].result(ai.Message{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{
		ID: "call-account", Type: "function", Function: ai.FunctionCall{Name: "add_account", Arguments: `{"name":"Canceled Lifecycle Account","class":"asset"}`},
	}}}, ai.Usage{})
	waitForInsightsApproval(t, f)
	staleApprove := byTestID(f, "button", "assistant-approve")
	byTestID(f, "button", "assistant-new-chat").Click()
	f.Stabilize()
	// The retained callback is equivalent to a click queued just before the card
	// disappeared. The canceled approval wait must win and skip the tool.
	staleApprove.Click()
	f.Stabilize()
	for _, account := range app.Accounts() {
		if account.Name == "Canceled Lifecycle Account" {
			t.Fatal("approval delivered after cancellation ran a mutating tool")
		}
	}
}

func TestInsightsApprovalCannotMutateAfterAppReplacement(t *testing.T) {
	app, f, composer, callbacks := setupInsightsLifecycle(t)
	request := submitInsightsLifecycleTurn(t, f, composer, "propose an account", callbacks)
	(*callbacks)[request].result(ai.Message{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{
		ID: "call-replaced-account", Type: "function", Function: ai.FunctionCall{Name: "add_account", Arguments: `{"name":"Replaced Dataset Account","class":"asset"}`},
	}}}, ai.Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7})
	waitForInsightsApproval(t, f)
	staleApprove := byTestID(f, "button", "assistant-approve")
	conversationID := waitForInsightsConversation(t, f, app).ID

	replacement, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	appstate.Default = replacement
	t.Cleanup(func() {
		appstate.Default = app
		_ = replacement.Store().Close()
	})

	// This retained event belongs to the old page render. It must not execute
	// against the captured old app or create state in the replacement app.
	staleApprove.Click()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		f.Stabilize()
		if byTestID(f, "div", "assistant-approval") == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if byTestID(f, "div", "assistant-approval") != nil {
		t.Fatal("approval remained visible after the captured app was replaced")
	}
	for _, dataset := range []*appstate.App{app, replacement} {
		for _, account := range dataset.Accounts() {
			if account.Name == "Replaced Dataset Account" {
				t.Fatalf("stale approval mutated dataset %p", dataset)
			}
		}
	}
	if got := uistate.AgentTokensUsed(conversationID); got != 7 {
		t.Fatalf("provider usage recorded against original conversation = %d tokens, want 7", got)
	}
	if got := replacement.AISpendThisMonth().Tokens; got != 0 {
		t.Fatalf("replacement dataset received %d provider usage tokens, want 0", got)
	}
}

func TestInsightsDuplicateApprovalDecisionIsSafe(t *testing.T) {
	app, f, composer, callbacks := setupInsightsLifecycle(t)
	t.Cleanup(func() { uistate.DropLastAssistantActionFor("mock-undo") })
	first := submitInsightsLifecycleTurn(t, f, composer, "propose an account", callbacks)
	(*callbacks)[first].result(ai.Message{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{
		ID: "call-account", Type: "function", Function: ai.FunctionCall{Name: "add_account", Arguments: `{"name":"Lifecycle Test Account","class":"asset"}`},
	}}}, ai.Usage{PromptTokens: 2, CompletionTokens: 2, TotalTokens: 4})
	waitForInsightsApproval(t, f)
	approve := byTestID(f, "button", "assistant-approve")
	approve.Click()
	// A second delivery from a stale rendered button must be harmless and must not
	// block the browser event loop after the one-shot decision has been accepted.
	approve.Click()
	f.Stabilize()
	accountCount := 0
	for _, account := range app.Accounts() {
		if account.Name == "Lifecycle Test Account" {
			accountCount++
		}
	}
	if accountCount != 1 {
		t.Fatalf("duplicate approval executions created %d matching accounts", accountCount)
	}
	deadline := time.Now().Add(2 * time.Second)
	for uistate.AgentReceiptActionCount(app.Conversations()[0].ID) == 0 && time.Now().Before(deadline) {
		f.Stabilize()
		time.Sleep(time.Millisecond)
	}
	conversationID := app.Conversations()[0].ID
	if got := uistate.AgentReceiptActionCount(conversationID); got != 1 {
		t.Fatalf("approved action receipt count = %d, want exactly one for its conversation", got)
	}
	byTestID(f, "button", "assistant-new-chat").Click()
	f.Stabilize()
}

func TestInsightsBlankChatPreferencesCarryIntoFirstRequestAndBudgetBlocksAtCap(t *testing.T) {
	app, f, composer, callbacks := setupInsightsLifecycle(t)
	byTestID(f, "button", "assistant-settings-toggle").Click()
	modelSelect := byTestID(f, "select", "assistant-model")
	if modelSelect == nil {
		t.Fatal("model picker select not found")
	}
	modelSelect.Change("gpt-5.5")
	byTestID(f, "select", "assistant-budget").Change("25000")
	f.Stabilize()
	if got := app.Settings().OpenAIModel; got != "gpt-5.5" {
		t.Fatalf("model picker change saved %q, want selected blank-chat model", got)
	}
	request := submitInsightsLifecycleTurn(t, f, composer, "measure one answer", callbacks)
	deadline := time.Now().Add(2 * time.Second)
	for len(app.Conversations()) == 0 && time.Now().Before(deadline) {
		f.Stabilize()
		time.Sleep(time.Millisecond)
	}
	if len(app.Conversations()) == 0 {
		t.Fatal("conversation was not created before the first provider request")
	}
	conversationID := app.Conversations()[0].ID
	if conversationID == "" {
		t.Fatal("first provider request had no conversation id")
	}
	if got := (*callbacks)[request].model; got != "gpt-5.5" {
		t.Fatalf("first provider request model = %q, want selected blank-chat model", got)
	}
	if got := uistate.AgentModel(conversationID); got != "gpt-5.5" {
		t.Fatalf("new conversation model = %q, want selected blank-chat model (saved conversations: %#v)", got, app.Conversations())
	}
	if got := uistate.AgentBudget(conversationID); got != 25000 {
		t.Fatalf("new conversation budget = %d, want selected blank-chat cap", got)
	}
	(*callbacks)[request].result(ai.Message{Role: ai.RoleAssistant, Content: "answer"}, ai.Usage{PromptTokens: 20000, CompletionTokens: 5000, TotalTokens: 25000})
	deadline = time.Now().Add(2 * time.Second)
	for uistate.AgentReceiptSummary(conversationID) == "" && time.Now().Before(deadline) {
		f.Stabilize()
		time.Sleep(time.Millisecond)
	}
	if summary := uistate.AgentReceiptSummary(conversationID); summary == "" {
		t.Fatal("completed model usage was not charged to the conversation captured before request start")
	}
	if _, known := uistate.AgentCostSoFar(conversationID); !known {
		t.Fatal("known mocked usage did not produce a per-conversation cost receipt")
	}
	if got := uistate.AgentTokensUsed(conversationID); got != 25000 {
		t.Fatalf("conversation token usage = %d, want cap 25000", got)
	}
	count := len(*callbacks)
	composer.Set("value", "this should be blocked")
	byTestID(f, "button", "assistant-send").Click()
	f.Stabilize()
	if len(*callbacks) != count {
		t.Fatalf("spent conversation budget allowed another provider request (count %d -> %d)", count, len(*callbacks))
	}
	// Drain the final deferred scroll callback while the test document stub is
	// still installed; the WASM test host has no browser document after cleanup.
	f.Render(ui.CreateElement(func() ui.Node { return Div() }))
	f.Stabilize()
	time.Sleep(150 * time.Millisecond)
}

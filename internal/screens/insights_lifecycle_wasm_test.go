// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
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
	auditview.CaptureAssistantBaseline = func() bool { return true }
	auditview.CaptureAssistantReceipt = func() (string, bool) { return "mock-undo", true }
	t.Cleanup(func() {
		auditview.CaptureAssistantBaseline, auditview.CaptureAssistantReceipt = oldBaseline, oldReceipt
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
		_ bool, _, _, _, _, model string, _ []ai.Message, _ float64, _ string, _ []ai.Tool,
		delta func(string), result func(ai.Message, ai.Usage), onErr func(string),
	) func() {
		callbacks = append(callbacks, insightsLifecycleCallbacks{delta: delta, result: result, err: onErr, model: model})
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
	f.Stabilize()
	if summary := uistate.AgentReceiptSummary(conversationID); summary != "" {
		t.Fatalf("late callback recorded a conversation receipt: %q", summary)
	}
	for _, message := range app.Conversations()[0].Messages {
		if message.Role == "assistant" && strings.Contains(message.Text, "late answer") {
			t.Fatal("late callback persisted an answer after New chat")
		}
	}

	second := submitInsightsLifecycleTurn(t, f, composer, "unmount request", callbacks)
	f.Render(ui.CreateElement(func() ui.Node { return Div() }))
	f.Stabilize()
	if (*callbacks)[second].cancelCount == 0 {
		t.Fatal("unmount did not cancel the outstanding provider request")
	}
	(*callbacks)[second].delta("after unmount")
	(*callbacks)[second].result(ai.Message{Role: ai.RoleAssistant, Content: "after unmount"}, ai.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2})
	if summary := uistate.AgentReceiptSummary(conversationID); summary != "" {
		t.Fatalf("unmounted callback recorded a conversation receipt: %q", summary)
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
	if summary := uistate.AgentReceiptSummary(active.ID); summary != "" {
		t.Fatalf("late result changed the switched-away conversation receipt: %q", summary)
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

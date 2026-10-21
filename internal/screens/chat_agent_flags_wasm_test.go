// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func assistantFlagsApp(t *testing.T) *appstate.App {
	t.Helper()
	app := primaryTodoNotificationsApp(t)
	if err := app.PutAccount(domain.Account{ID: "flag-cash", Name: "Flag account", Currency: "USD",
		Class: domain.ClassAsset, Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		BalanceAsOf: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return app
}

func assistantFlagPair(t *testing.T, app *appstate.App, prefix, merchant string) {
	t.Helper()
	for i := range 2 {
		if err := app.PutTransaction(domain.Transaction{ID: fmt.Sprintf("%s-%d", prefix, i), AccountID: "flag-cash",
			Payee: merchant, Desc: merchant, Date: time.Now(), Amount: money.New(-4200, "USD")}); err != nil {
			t.Fatal(err)
		}
	}
}

func assistantDismissTool(app *appstate.App) chatTool {
	return agToolsFlags(app, "USD", currency.Rates{Base: "USD"})[0]
}

func TestAssistantFlagsRegisteredOnceWithStrictApproval(t *testing.T) {
	app := assistantFlagsApp(t)
	count := 0
	for _, tool := range buildChatTools(app, "USD", currency.Rates{Base: "USD"}, aicontext.TierFull) {
		if tool.spec.Function.Name == "dismiss_flagged_activity" {
			count++
			if !tool.mutates || tool.preview == nil || !strings.Contains(string(tool.spec.Function.Parameters), `"additionalProperties":false`) {
				t.Fatal("flag tool lost approval or strict argument schema")
			}
		}
	}
	if count != 1 {
		t.Fatalf("flag tool registrations = %d, want 1", count)
	}
}

func TestAssistantFlagsRejectMalformedAndUnknownArgumentsWithoutDismissing(t *testing.T) {
	app := assistantFlagsApp(t)
	assistantFlagPair(t, app, "coffee", "Coffee shop")
	if len(assistantFlagEffects(app, "")) == 0 {
		t.Fatal("fixture needs actual detected flags")
	}
	for _, input := range []string{`{"match":`, `null`, `[]`, `{"match":42}`, `{"match":true}`, `{"match":null}`,
		`{"match":[]}`, `{"match":{}}`, `{"Match":"Coffee"}`, `{"match":"Coffee","unknown":true}`, `{} {}`} {
		t.Run(input, func(t *testing.T) {
			tool := assistantDismissTool(app)
			before, rev := uistate.LoadSmartSettings(), app.Rev()
			raw := json.RawMessage(input)
			if out := tool.preview(raw); !strings.HasPrefix(out, "Can't dismiss") {
				t.Fatalf("invalid args offered an actionable preview: %q", out)
			}
			if out := tool.run(raw); !strings.HasPrefix(out, "Can't dismiss") || app.Rev() != rev || !reflect.DeepEqual(uistate.LoadSmartSettings(), before) {
				t.Fatalf("invalid args swept flags or hid error: %q", out)
			}
		})
	}
}

func TestAssistantFlagsPreviewListsEveryExactEffectAndLeavesTransactions(t *testing.T) {
	app := assistantFlagsApp(t)
	assistantFlagPair(t, app, "coffee", "Coffee shop")
	assistantFlagPair(t, app, "market", "Market")
	tool := assistantDismissTool(app)
	raw := json.RawMessage(`{"match":"COFFEE"}`)
	expected := assistantFlagEffects(app, "coffee")
	if len(expected) == 0 {
		t.Fatal("fixture has no coffee flag")
	}
	before := app.Transactions()
	preview := tool.preview(raw)
	for _, effect := range expected {
		for _, want := range []string{effect.Key, effect.Title, effect.Detail} {
			if !strings.Contains(preview, want) {
				t.Fatalf("preview omitted exact dismissal effect %q: %s", want, preview)
			}
		}
	}
	if out := tool.run(raw); !strings.HasPrefix(out, "Dismissed exactly") {
		t.Fatal(out)
	}
	settings := uistate.LoadSmartSettings()
	for _, effect := range expected {
		if !settings.IsDismissed(effect.Key) {
			t.Fatalf("approved flag %s not dismissed", effect.Key)
		}
	}
	if len(assistantFlagEffects(app, "market")) == 0 || !reflect.DeepEqual(app.Transactions(), before) {
		t.Fatal("selective dismissal hid unrelated flags or changed transactions")
	}
	// A retry of the original approval must never sweep a new matching flag.
	assistantFlagPair(t, app, "new-coffee", "Coffee shop on Oak Street")
	newFlags := assistantFlagEffects(app, "coffee")
	if len(newFlags) == 0 {
		t.Fatal("new matching flag fixture missing")
	}
	rev := app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "already dismissed") || app.Rev() != rev || !reflect.DeepEqual(assistantFlagEffects(app, "coffee"), newFlags) {
		t.Fatalf("retry changed newly detected flags: %q", out)
	}
}

func TestAssistantFlagsChangedApprovalSetRefusesThenCanBeReviewedAgain(t *testing.T) {
	app := assistantFlagsApp(t)
	assistantFlagPair(t, app, "coffee", "Coffee shop")
	tool := assistantDismissTool(app)
	raw := json.RawMessage(`{}`)
	tool.preview(raw)
	assistantFlagPair(t, app, "market", "Market")
	before, rev := uistate.LoadSmartSettings(), app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "changed after the preview") || app.Rev() != rev || !reflect.DeepEqual(uistate.LoadSmartSettings(), before) {
		t.Fatalf("approval swept a newly detected flag: %q", out)
	}
	tool.preview(raw)
	if out := tool.run(raw); !strings.HasPrefix(out, "Dismissed exactly") || len(assistantFlagEffects(app, "")) != 0 {
		t.Fatalf("freshly reviewed exact set was not dismissed: %q", out)
	}
}

func TestAssistantFlagsEmptyPreviewCannotDismissLaterFlags(t *testing.T) {
	app := primaryTodoNotificationsApp(t)
	tool := assistantDismissTool(app)
	raw := json.RawMessage(`{}`)
	if out := tool.preview(raw); out != "No flagged activities to dismiss." {
		t.Fatal(out)
	}
	if err := app.PutAccount(domain.Account{ID: "flag-cash", Name: "Cash", Currency: "USD", Class: domain.ClassAsset,
		Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, BalanceAsOf: time.Now()}); err != nil {
		t.Fatal(err)
	}
	assistantFlagPair(t, app, "coffee", "Coffee shop")
	rev := app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "changed after the preview") || app.Rev() != rev || len(assistantFlagEffects(app, "")) == 0 {
		t.Fatalf("empty approval dismissed a later flag: %q", out)
	}
}

func TestAssistantFlagsDatasetSwitchCannotApplyOldApproval(t *testing.T) {
	app := assistantFlagsApp(t)
	assistantFlagPair(t, app, "coffee", "Coffee shop")
	tool := assistantDismissTool(app)
	raw := json.RawMessage(`{}`)
	tool.preview(raw)
	appstate.Default = nil
	t.Cleanup(func() { appstate.Default = app })
	rev := app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "active dataset changed") || app.Rev() != rev {
		t.Fatalf("old approval crossed dataset identity: %q", out)
	}
}

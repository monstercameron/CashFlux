// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
)

func trustMemoryTestApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	return app
}

func trustMemoryTools(app *appstate.App) (remember, list chatTool) {
	tools := agToolsTrust(app, "USD", currency.Rates{Base: "USD"})
	return tools[0], tools[1]
}

func readTrustMemoryRaw(t *testing.T, app *appstate.App) string {
	t.Helper()
	raw, _, err := app.Store().GetSettingKV("cashflux:agent-memory")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestAssistantTrustMemoryUsesBoundDatasetAndFreshApprovedSnapshot(t *testing.T) {
	app := trustMemoryTestApp(t)
	other := trustMemoryTestApp(t)
	oldDefault := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = oldDefault })
	if err := app.SetSettingKV("cashflux:agent-memory", `{"facts":["Primary fact"]}`); err != nil {
		t.Fatal(err)
	}
	if err := other.SetSettingKV("cashflux:agent-memory", `{"facts":["Other dataset fact"]}`); err != nil {
		t.Fatal(err)
	}
	remember, list := trustMemoryTools(app)
	if got := list.run(json.RawMessage(`{}`)); !strings.Contains(got, "Primary fact") || strings.Contains(got, "Other dataset fact") {
		t.Fatalf("bound list result = %q", got)
	}

	args := json.RawMessage(`{"fact":"  Paid biweekly  "}`)
	preview := remember.preview(args)
	if !strings.Contains(preview, "Paid biweekly") {
		t.Fatalf("preview = %q", preview)
	}
	if got := remember.run(args); !strings.Contains(got, "Remembered") {
		t.Fatalf("remember result = %q", got)
	}
	if got := readTrustMemoryRaw(t, app); !strings.Contains(got, "Paid biweekly") || !strings.Contains(got, "Primary fact") {
		t.Fatalf("bound memory after write = %q", got)
	}
	if got := readTrustMemoryRaw(t, other); got != `{"facts":["Other dataset fact"]}` {
		t.Fatalf("other dataset changed to %q", got)
	}
	if got := remember.run(args); !strings.Contains(got, "does not match its preview") {
		t.Fatalf("reused approval result = %q, want stale preview refusal", got)
	}
}

func TestAssistantTrustMemoryRejectsChangedPreviewAndStrictArguments(t *testing.T) {
	app := trustMemoryTestApp(t)
	oldDefault := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = oldDefault })
	if err := app.SetSettingKV("cashflux:agent-memory", `{"facts":["Original fact"]}`); err != nil {
		t.Fatal(err)
	}
	remember, list := trustMemoryTools(app)
	args := json.RawMessage(`{"fact":"New fact"}`)
	remember.preview(args)
	if err := app.SetSettingKV("cashflux:agent-memory", `{"facts":["Edited in Settings"]}`); err != nil {
		t.Fatal(err)
	}
	if got := remember.run(args); !strings.Contains(got, "changed after the preview") {
		t.Fatalf("stale preview result = %q", got)
	}
	if got := readTrustMemoryRaw(t, app); got != `{"facts":["Edited in Settings"]}` {
		t.Fatalf("stale approval overwrote the newer memory with %q", got)
	}

	for _, invalid := range []json.RawMessage{
		json.RawMessage(`{"fact":"valid","extra":true}`),
		json.RawMessage(`{"fact":"valid"} {}`),
		json.RawMessage(`{"fact":"  "}`),
	} {
		if got := remember.preview(invalid); !strings.Contains(got, "Can't prepare") {
			t.Errorf("invalid preview %s = %q", invalid, got)
		}
	}
	if got := list.run(json.RawMessage(`{"extra":true}`)); !strings.Contains(got, "takes no arguments") {
		t.Fatalf("invalid list arguments = %q", got)
	}
}

func TestAssistantTrustMemoryRejectsDatasetReplacementAndMalformedStoredValue(t *testing.T) {
	app := trustMemoryTestApp(t)
	other := trustMemoryTestApp(t)
	oldDefault := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = oldDefault })
	if err := app.SetSettingKV("cashflux:agent-memory", `{"facts":["Original fact"]}`); err != nil {
		t.Fatal(err)
	}
	if err := other.SetSettingKV("cashflux:agent-memory", `{"facts":["Other fact"]}`); err != nil {
		t.Fatal(err)
	}
	remember, list := trustMemoryTools(app)
	args := json.RawMessage(`{"fact":"Replacement must not receive this"}`)
	remember.preview(args)
	appstate.Default = other
	if got := remember.run(args); !strings.Contains(got, "active dataset changed") {
		t.Fatalf("replacement result = %q", got)
	}
	if got := readTrustMemoryRaw(t, app); got != `{"facts":["Original fact"]}` {
		t.Fatalf("old dataset changed to %q", got)
	}
	if got := readTrustMemoryRaw(t, other); got != `{"facts":["Other fact"]}` {
		t.Fatalf("replacement dataset changed to %q", got)
	}
	appstate.Default = app
	if err := app.SetSettingKV("cashflux:agent-memory", `not-json`); err != nil {
		t.Fatal(err)
	}
	if got := remember.preview(args); !strings.Contains(got, "malformed") {
		t.Fatalf("malformed memory preview = %q", got)
	}
	if got := list.run(json.RawMessage(`{}`)); !strings.Contains(got, "malformed") {
		t.Fatalf("malformed memory list result = %q", got)
	}
	if got := readTrustMemoryRaw(t, app); got != `not-json` {
		t.Fatalf("malformed memory was overwritten with %q", got)
	}
}

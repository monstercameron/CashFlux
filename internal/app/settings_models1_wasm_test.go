// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"testing"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestAIModelPickerShowsAllSearchesAcceptsCustomAndKeepsReloadSelection(t *testing.T) {
	fixture := render.New(t)
	defer fixture.Cleanup()
	fixture.Render(Body(ID("models1-body"), uic.CreateElement(settingsModelPickerTestHost)))
	if findAppTestID(fixture, "select", "settings-ai-model") == nil {
		t.Fatal("model picker is missing")
	}
	for _, id := range []string{"text-embedding-3-small", "gpt-4o", "whisper-1", "o4-mini"} {
		if !containsText(fixture.Text(), id) {
			t.Errorf("model id %q is missing from the picker", id)
		}
	}
	if !containsText(fixture.Text(), "may not work for chat") {
		t.Fatal("picker does not label model types that may not support chat")
	}

	findAppTestID(fixture, "input", "settings-ai-model-search").Input("EMBED")
	fixture.Stabilize()
	if !containsText(fixture.Text(), "text-embedding-3-small") || containsText(fixture.Text(), "whisper-1") || strings.Count(fixture.Text(), "gpt-4o") != 1 {
		t.Fatalf("search did not filter models by substring: %q", fixture.Text())
	}

	findAppTestID(fixture, "input", "settings-ai-model-search").Input("custom-provider/model-x")
	fixture.Stabilize()
	findAppTestID(fixture, "button", "settings-ai-model-use-custom").Click()
	fixture.Stabilize()
	if !containsText(fixture.Text(), "custom-provider/model-x") {
		t.Fatal("custom selection is not visible in the picker")
	}

	findAppTestID(fixture, "input", "settings-ai-model-search").Input("")
	fixture.Stabilize()
	findAppTestID(fixture, "button", "settings-ai-model-reload").Click()
	fixture.Stabilize()
	if !containsText(fixture.Text(), "custom-provider/model-x") || !containsText(fixture.Text(), "not in the latest list") {
		t.Fatalf("selected model was not retained and marked after reload: %q", fixture.Text())
	}
}

func settingsModelPickerTestHost() uic.Node {
	models := uic.UseState([]string{"text-embedding-3-small", "gpt-4o", "whisper-1", "o4-mini"})
	selected := uic.UseState("gpt-4o")
	query := uic.UseState("")
	loaded := uic.UseState(true)
	onModel := uic.UseEvent(func(e uic.Event) { selected.Set(e.GetValue()) })
	onReload := uic.UseEvent(func() { models.Set([]string{"gpt-4o"}); loaded.Set(true) })
	onUseCustom := uic.UseEvent(func() {
		if model := strings.TrimSpace(query.Get()); model != "" {
			selected.Set(model)
		}
	})
	return settingsAIPane(settingsRightProps{
		AiKey: "test-key", CurModel: selected.Get(), Models: models.Get(), ModelsLoaded: loaded.Get(), ModelQuery: query.Get(),
		OnModel: onModel, OnModelQuery: func(value string) { query.Set(value) },
		OnReloadModels: onReload, OnUseCustomModel: onUseCustom,
	})
}

func containsText(text, fragment string) bool {
	return strings.Contains(text, fragment)
}

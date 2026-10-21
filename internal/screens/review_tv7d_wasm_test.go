// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestReviewTV7DBulkActionsStartDisabledAndShortcutsAreDisclosure(t *testing.T) {
	body, footer := reviewBulkView(nil, reviewIndex{}, nil, nil, nil,
		func(reviewRow) string { return "" }, nil, nil, nil,
		nil, nil, nil, nil, nil, "", prefs.Prefs{}, 0,
		ui.Fragment(), ui.Fragment(), 0)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.Fragment(body, footer))

	for _, id := range []string{"review-apply", "review-clear"} {
		button := findByTestID(f, "button", id)
		if !isDisabled(button) {
			t.Errorf("%s must be disabled when no merchants are selected", id)
		}
	}
	f.Render(reviewKeyboardDisclosure(uistate.T("review.keyboardHintBulkPlain")))
	disclosure := findByTestID(f, "details", "review-kbd")
	if disclosure == nil {
		t.Fatal("Review keyboard shortcuts are not inside a disclosure")
	}
	var hasSummary bool
	for _, summary := range f.AllByTag("summary") {
		hasSummary = hasSummary || summary.Text() == "Keyboard shortcuts"
	}
	if !hasSummary {
		t.Fatal("shortcut disclosure should be labelled Keyboard shortcuts")
	}
	hint := findByTestID(f, "span", "review-kbd-hint")
	if hint == nil || hint.Text() == "" || containsMiddleDot(hint.Text()) {
		t.Fatalf("shortcut hint should be readable plain copy: %#v", hint)
	}
}

func containsMiddleDot(value string) bool {
	for _, r := range value {
		if r == '·' {
			return true
		}
	}
	return false
}

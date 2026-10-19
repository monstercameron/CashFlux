// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func reviewInboxOpenerTestComponent(_ struct{}) ui.Node {
	UseReviewInbox()
	return nil
}

func TestOpenReviewInboxStartsOnTheCountedWholeQueue(t *testing.T) {
	f := render.New(t)
	f.Render(ui.CreateElement(reviewInboxOpenerTestComponent, struct{}{}))
	OpenReviewInbox()
	session := capturedReviewOpenSession.Get().Normalized()
	if !session.ScopeChosen || session.Scope != reviewscope.ScopeEntireQueue {
		t.Fatalf("opened review scope = (%v, %q), want explicitly chosen entire queue", session.ScopeChosen, session.Scope)
	}
	if session.Cursor != 0 || session.CursorKey != "" {
		t.Fatalf("opened review cursor = (%d, %q), want reset", session.Cursor, session.CursorKey)
	}
}

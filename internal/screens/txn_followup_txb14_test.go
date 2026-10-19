// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestToggleFollowUpTaskReturnsWriteFailure(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	old := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = old })
	task := domain.Task{ID: "follow-up", Title: "Call the merchant", Status: domain.StatusOpen, Priority: domain.PriorityMedium}
	if err := app.PutTask(task); err != nil {
		t.Fatal(err)
	}
	app.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
	if err := toggleFollowUpTask(task.ID, true); !errors.Is(err, appstate.ErrReadOnly) {
		t.Fatalf("toggle error = %v, want read-only write error", err)
	}
}

func TestToggleFollowUpTaskReturnsMissingTaskError(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	old := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = old })
	if err := toggleFollowUpTask("missing", true); err == nil {
		t.Fatal("toggle of a missing task returned no error")
	}
}

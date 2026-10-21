// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestBC6N8AutoBudgetErrorDoesNotExposeInternalError(t *testing.T) {
	f := render.New(t)
	f.Render(autoBudgetErrorNode(errors.New("private implementation detail"), "autobudget-test-error"))
	if got := strings.Join(strings.Fields(f.Text()), " "); got != "Auto budget could not be calculated. Try again." {
		t.Fatalf("auto-budget error copy = %q", got)
	}
}

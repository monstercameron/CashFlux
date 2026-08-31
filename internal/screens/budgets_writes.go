// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"log/slog"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// budgets_writes.go is the one place a failed write on the budgets surface is
// reported.
//
// Writes here had settled into two shapes. Most did the right thing — report the
// error and stop. Five did not: they discarded the error and carried straight on
// to BumpDataRevision and a success toast, so the page asserted a change the store
// had rejected. That is worse than a visible failure, because the user is told the
// opposite of what happened and has no reason to look again.
//
// One of the five was not merely misleading. The funding path reduces a SOURCE
// budget's cap and then raises the DESTINATION; discarding the source write while
// completing the destination one does not fail to move money, it INVENTS money —
// the household's plan grows by the amount of the transfer. The permanent branch
// of that same function already returned on error. The two branches disagreed
// about whether a rejected write mattered.
//
// This also decides what the user is allowed to see. Validation issues are written
// for people ("Name is required") and are shown as they are. A failure from the
// storage layer is not: "sql: database is closed" arrives at precisely the moment
// someone most needs to understand what happened, and tells them nothing they can
// act on. Those get a sentence, and the original goes to the log.

// budgetWriteFailed reports err and returns true when the caller must stop.
//
// Call it in place of discarding a write's error:
//
//	if budgetWriteFailed("save budget", app.PutBudget(b)) {
//	    return
//	}
//
// A nil error returns false and says nothing, so it reads as a guard rather than
// as error handling bolted onto the end.
func budgetWriteFailed(op string, err error) bool {
	if err == nil {
		return false
	}
	slog.Error("budgets: write rejected", "op", op, "err", err)
	uistate.PostNotice(budgetErrorText(err), true)
	return true
}

// budgetErrorText turns a write error into something worth putting on screen.
//
// The distinction is who the message was written for, not how severe it is. A
// validate.Issues names the field and what it needs, and read-only means the
// household has this identity in a viewing role — both are answers. Anything else
// reached us from a layer that describes machines, so the user gets a plain
// sentence and the detail stays in the log.
func budgetErrorText(err error) string {
	var issues validate.Issues
	if errors.As(err, &issues) && !issues.OK() {
		return issues.Error()
	}
	if errors.Is(err, appstate.ErrReadOnly) {
		return uistate.T("budgets.writeReadOnly")
	}
	return uistate.T("budgets.writeFailed")
}

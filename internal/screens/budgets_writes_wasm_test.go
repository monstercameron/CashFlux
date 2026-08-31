// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// budgetErrorText decides what a failed write is allowed to put on screen. The
// property under test is the SPLIT, not the wording: an error written for a person
// must survive intact, and an error written for a machine must not reach the page
// at all. Asserting only that "something is shown" would pass on the defect this
// exists to prevent — a raw storage error rendered to the user verbatim.
func TestBudgetErrorTextShowsValidationVerbatim(t *testing.T) {
	// A near-empty budget: validation rejects it on several fields at once, and its
	// message ("name is required; ...") is exactly the kind that must survive to
	// the screen intact.
	issues := validate.ValidateBudget(domain.Budget{ID: "b1"})
	if issues.OK() {
		t.Fatal("expected a nameless budget to fail validation; the fixture is wrong")
	}
	got := budgetErrorText(issues)
	if got != issues.Error() {
		t.Errorf("validation text was rewritten:\n got  %q\n want %q", got, issues.Error())
	}
}

func TestBudgetErrorTextHidesStorageDetail(t *testing.T) {
	// The shapes a storage layer actually produces, including a wrapped one — the
	// classifier uses errors.As/Is, so a wrapped storage error must not accidentally
	// match the validation branch and leak through.
	for _, err := range []error{
		errors.New("sql: database is closed"),
		errors.New("disk I/O error"),
		fmt.Errorf("put budget: %w", errors.New("UNIQUE constraint failed: budgets.id")),
	} {
		got := budgetErrorText(err)
		if strings.Contains(got, err.Error()) {
			t.Errorf("storage detail reached the screen for %v: %q", err, got)
		}
		for _, leak := range []string{"sql", "constraint", "I/O", "budgets."} {
			if strings.Contains(got, leak) {
				t.Errorf("%q leaked into the user-facing text %q", leak, got)
			}
		}
		if strings.TrimSpace(got) == "" {
			t.Errorf("a failed write said nothing at all for %v", err)
		}
	}
}

func TestBudgetErrorTextNamesTheReadOnlyCase(t *testing.T) {
	// Read-only is not a fault, it is an answer: the household is open in a viewing
	// role. Folding it into the generic "try again" would tell someone to retry
	// something that cannot succeed however many times they try it.
	generic := budgetErrorText(errors.New("sql: database is closed"))
	got := budgetErrorText(appstate.ErrReadOnly)
	if got == generic {
		t.Errorf("read-only is reported as a generic failure: %q", got)
	}
	if strings.Contains(got, "appstate") {
		t.Errorf("the sentinel's own text leaked: %q", got)
	}
	// And through a wrap, since callers return it from several layers down.
	if w := budgetErrorText(fmt.Errorf("put settings: %w", appstate.ErrReadOnly)); w != got {
		t.Errorf("a wrapped read-only error was classified differently:\n got  %q\n want %q", w, got)
	}
}

// budgetWriteFailed is the guard callers branch on, so its BOOLEAN is the part
// that must be right: a nil error has to be silent and false, or every write on
// the surface would report a failure that did not happen.
func TestBudgetWriteFailedIsSilentOnSuccess(t *testing.T) {
	if budgetWriteFailed("save budget", nil) {
		t.Error("a successful write was reported as a failure")
	}
	if !budgetWriteFailed("save budget", errors.New("sql: database is closed")) {
		t.Error("a rejected write was reported as success — the caller would carry on")
	}
}

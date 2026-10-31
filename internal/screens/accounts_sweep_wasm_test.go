// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// TestSweepRulesFormAddsARuleFromItsPickers checks the add form end to end: the source and
// destination pickers, the keep amount and the cadence are read, and the saved rule carries them.
func TestSweepRulesFormAddsARuleFromItsPickers(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	checking := checkingFixture("checking", "Checking", 500000)
	savings := checkingFixture("savings", "Savings", 0)
	savings.Type = domain.TypeSavings
	for _, a := range []domain.Account{checking, savings} {
		if err := app.PutAccount(a); err != nil {
			t.Fatal(err)
		}
	}

	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(SweepRulesForm, SweepRulesFormProps{}))
	f.Stabilize()

	byTestID(f, "select", "sweep-cfg-source").Dispatch("onchange", render.Event{Value: checking.ID})
	f.Stabilize()
	byTestID(f, "input", "sweep-cfg-keep").Input("3000")
	f.Stabilize()
	byTestID(f, "select", "sweep-cfg-dest").Dispatch("onchange", render.Event{Value: savings.ID})
	f.Stabilize()
	byTestID(f, "select", "sweep-cfg-cadence").Dispatch("onchange", render.Event{Value: string(domain.SweepWeekly)})
	f.Stabilize()
	byTestID(f, "form", "sweep-rules-form").Submit()
	f.Stabilize()

	rules := app.SweepRules()
	if len(rules) != 1 {
		t.Fatalf("sweep rules = %d, want 1: %s", len(rules), f.Text())
	}
	r := rules[0]
	if r.SourceAccountID != checking.ID || r.DestAccountID != savings.ID || r.KeepMinor != 300000 || r.Cadence != domain.SweepWeekly {
		t.Fatalf("saved rule = %+v, want checking to savings keeping 300000 weekly", r)
	}
	if byTestID(f, "div", "sweep-rule-row-"+r.ID) == nil {
		t.Fatalf("the saved rule is not listed: %s", f.Text())
	}
}

// TestSweepRulesFormRefusesTheSameAccountTwice checks that a rule needs two different accounts.
func TestSweepRulesFormRefusesTheSameAccountTwice(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	checking := checkingFixture("checking", "Checking", 500000)
	if err := app.PutAccount(checking); err != nil {
		t.Fatal(err)
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(SweepRulesForm, SweepRulesFormProps{}))
	f.Stabilize()

	byTestID(f, "form", "sweep-rules-form").Submit()
	f.Stabilize()

	if len(app.SweepRules()) != 0 {
		t.Fatal("a rule with no accounts was saved")
	}
	if got := f.Text(); !strings.Contains(got, uistate.T("acctSweepCfg.pickTwo")) {
		t.Fatalf("the pick-two message is missing: %s", got)
	}
}

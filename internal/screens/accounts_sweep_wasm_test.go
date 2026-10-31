// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/sweep"
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

// TestAccountsSweepCardIsALiveCalloutWithItsButtonsOutside checks that the proposal text is a
// polite live region and that the approve and dismiss buttons sit in the group but outside it,
// and that dismissing removes the card.
func TestAccountsSweepCardIsALiveCalloutWithItsButtonsOutside(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	checking := checkingFixture("checking", "Checking", 500000)
	savings := checkingFixture("savings", "Savings", 0)
	savings.Type = domain.TypeSavings
	for _, a := range []domain.Account{checking, savings} {
		if err := app.PutAccount(a); err != nil {
			t.Fatal(err)
		}
	}
	rule, err := app.PutSweepRule(domain.SweepRule{SourceAccountID: checking.ID, DestAccountID: savings.ID,
		KeepMinor: 300000, Cadence: domain.SweepMonthly, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(accountsSweepCard, accountsSweepCardProps{
		Proposal: sweep.Proposal{RuleID: rule.ID, SourceAccountID: checking.ID, DestAccountID: savings.ID,
			AmountMinor: 200000, Currency: checking.Currency, KeepMinor: 300000, BalanceMinor: 500000},
		FromName: "Checking", ToName: "Savings",
	}))
	f.Stabilize()

	group := byTestID(f, "div", "acct-sweep-card-"+rule.ID)
	if group == nil || group.Attr("role") != "group" {
		t.Fatalf("the sweep card is not a labelled group: %s", f.Text())
	}
	callout := byTestID(f, "div", "acct-sweep-callout-"+rule.ID)
	if callout == nil || callout.Attr("role") != "status" {
		t.Fatalf("the proposal text is not a status callout: %s", f.Text())
	}
	if byTestID(f, "p", "acct-sweep-breakdown-"+rule.ID) == nil {
		t.Fatal("the breakdown line is missing")
	}
	approve := byTestID(f, "button", "acct-sweep-approve-"+rule.ID)
	dismiss := byTestID(f, "button", "acct-sweep-dismiss-"+rule.ID)
	if approve == nil || dismiss == nil {
		t.Fatalf("the approve or dismiss button is missing: %s", f.Text())
	}
	if strings.Contains(callout.Text(), approve.Text()) || strings.Contains(callout.Text(), dismiss.Text()) {
		t.Fatalf("the buttons are inside the live region: %q", callout.Text())
	}

	dismiss.Click()
	f.Stabilize()
	if byTestID(f, "div", "acct-sweep-card-"+rule.ID) != nil {
		t.Fatal("the card is still shown after it was dismissed")
	}
}

// TestSweepRulesFormListsARuleAsAnEntityRowWithARemoveButton checks that a saved rule shows its
// sentence in a row with a remove button named after that sentence.
func TestSweepRulesFormListsARuleAsAnEntityRowWithARemoveButton(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	checking := checkingFixture("checking", "Checking", 500000)
	savings := checkingFixture("savings", "Savings", 0)
	savings.Type = domain.TypeSavings
	for _, a := range []domain.Account{checking, savings} {
		if err := app.PutAccount(a); err != nil {
			t.Fatal(err)
		}
	}
	rule, err := app.PutSweepRule(domain.SweepRule{SourceAccountID: checking.ID, DestAccountID: savings.ID,
		KeepMinor: 300000, Cadence: domain.SweepMonthly, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(SweepRulesForm, SweepRulesFormProps{}))
	f.Stabilize()

	row := byTestID(f, "div", "sweep-rule-row-"+rule.ID)
	if row == nil || !strings.Contains(row.Text(), "Checking") || !strings.Contains(row.Text(), "Savings") {
		t.Fatalf("the saved rule's sentence is not in its row: %s", f.Text())
	}
	del := byTestID(f, "button", "sweep-rule-del-"+rule.ID)
	if del == nil || !strings.Contains(del.Attr("aria-label"), "Checking") {
		t.Fatalf("the remove button is missing or unnamed: %s", f.Text())
	}
}

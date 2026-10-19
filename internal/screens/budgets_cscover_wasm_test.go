// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/coverplan"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func cscoverModel() coverplan.Model {
	return coverplan.Prepare([]coverAllOver{
		{ID: "food", Name: "Food", Currency: "USD", Shortfall: money.New(3730, "USD")},
		{ID: "travel", Name: "Travel", Currency: "USD", Shortfall: money.New(8000, "USD")},
	}, []coverAllSource{
		{ID: "dining", Name: "Dining", Currency: "USD", Avail: money.New(20000, "USD"), Monthly: money.New(20000, "USD")},
	}, "USD", "")
}

func TestCSCOVERBodyPreservesEmptyAndPopulatedMarkup(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	model := coverplan.Prepare(nil, nil, "USD", "")
	f.Render(ui.CreateElement(coverAllBody, coverAllBodyProps{Model: model}))
	if n := byTestID(f, "p", "cover-all-empty"); n == nil || n.Text() != uistate.T("coverAll.none") || n.Attr("class") != "empty cover-all-empty" {
		t.Fatalf("empty state = %v", n)
	}
	if byTestID(f, "p", "cover-all-intro") != nil || len(f.AllByTag("select")) != 0 {
		t.Fatal("empty body contains populated controls")
	}
	model = cscoverModel()
	model.CurrencyError = true
	chosen := map[string]string{"food": coverAllNextSource}
	display := coverplan.Present(model, chosen, false, uistate.T, fmtMoney)
	f.Render(ui.CreateElement(coverAllBody, coverAllBodyProps{Model: model, Display: display, Chosen: chosen, Error: "Try again", OnChoice: func(string, string) {}}))
	if n := byTestID(f, "p", "cover-all-intro"); n == nil || n.Text() != display.Intro {
		t.Fatalf("intro=%v", n)
	}
	if n := byTestID(f, "p", "cover-all-impact"); n == nil || n.Text() != uistate.T("budgets.impactReachThisAndNext") {
		t.Fatalf("impact=%v", n)
	}
	if n := byTestID(f, "p", "cover-all-currency-error"); n == nil || n.Attr("role") != "alert" || n.Text() != uistate.T("coverAll.currencyError") {
		t.Fatalf("currency alert=%v", n)
	}
	var hint, inlineError bool
	for _, n := range f.AllByTag("p") {
		hint = hint || strings.Contains(n.Attr("class"), "cover-all-hint")
		inlineError = inlineError || n.Attr("role") == "alert" && n.Text() == "Try again"
	}
	if !hint || !inlineError || len(f.AllByTag("select")) != 2 {
		t.Fatal("body lost hints, errors or rows")
	}
}

func TestCSCOVERKeyedListKeepsCallbacksWithTheirRows(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	model := cscoverModel()
	var calls []string
	onChoice := func(id, value string) { calls = append(calls, id+":"+value) }
	f.Render(ui.CreateElement(coverAllList, coverAllListProps{Model: model, Chosen: map[string]string{"food": "dining"}, OnChoice: onChoice}))
	byTestID(f, "select", "cover-all-src-food").Dispatch("onchange", render.Event{Value: coverAllNextSource})
	model.Overs[0], model.Overs[1] = model.Overs[1], model.Overs[0]
	f.Render(ui.CreateElement(coverAllList, coverAllListProps{Model: model, OnChoice: onChoice}))
	byTestID(f, "select", "cover-all-src-travel").Dispatch("onchange", render.Event{Value: "dining"})
	model.Overs = model.Overs[:1]
	f.Render(ui.CreateElement(coverAllList, coverAllListProps{Model: model, OnChoice: onChoice}))
	if byTestID(f, "select", "cover-all-src-food") != nil {
		t.Fatal("removed row remains")
	}
	byTestID(f, "select", "cover-all-src-travel").Dispatch("onchange", render.Event{Value: ""})
	model.Overs = nil
	f.Render(ui.CreateElement(coverAllList, coverAllListProps{Model: model, OnChoice: onChoice}))
	if len(f.AllByTag("select")) != 0 || !reflect.DeepEqual(calls, []string{"food:__next__", "travel:dining", "travel:"}) {
		t.Fatalf("callbacks=%v", calls)
	}
}

func TestCSCOVERRowKeepsLabelsCurrencyFilterAndRolloverOptions(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	m := cscoverModel()
	foreign := coverAllSource{ID: "eur", Name: "Euro", Currency: "EUR", Avail: money.New(100, "EUR")}
	var selected string
	p := coverAllRowProps{Over: m.Overs[0], Sources: append(m.Sources, foreign), Selected: "dining", OnChange: func(value string) { selected = value }}
	f.Render(ui.CreateElement(coverAllRow, p))
	selectNode := byTestID(f, "select", "cover-all-src-food")
	if selectNode == nil || selectNode.Attr("aria-label") != uistate.T("coverAll.sourceLabel") || len(f.AllByTag("option")) != 3 {
		t.Fatal("row select contract")
	}
	var donorLabel string
	for _, option := range f.AllByTag("option") {
		if option.Property("value") == "dining" {
			donorLabel = option.Text()
		}
	}
	if donorLabel != uistate.T("bv7g.cover.sourceBudget", "Dining", fmtMoney(m.Sources[0].Avail), fmtMoney(m.Sources[0].Monthly)) {
		t.Fatalf("donor=%q", donorLabel)
	}
	selectNode.Dispatch("onchange", render.Event{Value: "dining"})
	if selected != "dining" {
		t.Fatal("row callback")
	}
	p.Over.Rollover = true
	f.Render(ui.CreateElement(coverAllRow, p))
	if len(f.AllByTag("option")) != 2 {
		t.Fatal("rollover offers next month")
	}
}

func TestCSCOVERFooterPreservesActionsAcrossStates(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	calls := 0
	for _, tc := range []struct {
		name                    string
		empty, saving, disabled bool
	}{
		{"empty", true, false, true}, {"unselected", false, false, true}, {"selected", false, false, false}, {"saving", false, true, true}, {"empty again", true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.Render(ui.CreateElement(coverAllFooter, coverAllFooterProps{Empty: tc.empty, Saving: tc.saving, Disabled: tc.disabled, ApplyLabel: "Cover $37.30", OnDone: func() { calls++ }}))
			if tc.empty {
				close := byTestID(f, "button", "cover-all-close")
				if close == nil || close.Text() != uistate.T("action.close") || byTestID(f, "button", "cover-all-apply") != nil {
					t.Fatal("close action")
				}
				close.Dispatch("onclick", render.Event{})
			} else {
				apply := byTestID(f, "button", "cover-all-apply")
				if apply == nil || apply.Attr("type") != "submit" {
					t.Fatal("apply action")
				}
				// The fixture retains a removed boolean attribute; the updated
				// DOM property is authoritative when the button becomes enabled.
				if tc.disabled && !isDisabled(apply) || !tc.disabled && apply.Property("disabled") == true {
					t.Fatalf("disabled=%v attr=%q want %v", apply.Property("disabled"), apply.Attr("disabled"), tc.disabled)
				}
				want := "Cover $37.30"
				if tc.saving {
					want = uistate.T("enc34.saving")
				}
				if apply.Text() != want {
					t.Fatalf("label=%q", apply.Text())
				}
				byTestID(f, "button", "cover-all-cancel").Dispatch("onclick", render.Event{})
			}
		})
	}
	if calls != 5 {
		t.Fatalf("cancel callbacks=%d", calls)
	}
	f.Render(ui.CreateElement(coverAllFooter, coverAllFooterProps{Empty: true}))
	byTestID(f, "button", "cover-all-close").Dispatch("onclick", render.Event{})
}

func cscoverSeedApp(t *testing.T) period.Window {
	t.Helper()
	a := guardTestApp(t)
	pr := prefs.Default()
	if err := uistate.PersistPrefs(pr); err != nil {
		t.Fatal(err)
	}
	now := dateutil.WallClockUTC(time.Now())
	for _, row := range []struct {
		id           string
		limit, spent int64
	}{{"food", 10000, 13730}, {"travel", 10000, 18000}, {"dining", 30000, 1000}} {
		b := domain.Budget{ID: row.id, Name: row.id, CategoryID: row.id, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(row.limit, "USD")}
		if err := a.PutCategory(domain.Category{ID: row.id, Name: row.id, Kind: domain.KindExpense}); err != nil {
			t.Fatal(err)
		}
		if err := a.PutBudgetAt(b, now); err != nil {
			t.Fatal(err)
		}
		if err := a.Store().PutTransaction(domain.Transaction{ID: row.id, CategoryID: row.id, Date: now, Amount: money.New(-row.spent, "USD")}); err != nil {
			t.Fatal(err)
		}
	}
	return period.NewWindow(period.Month, now, time.Sunday)
}

func cscoverFormHarness(props coverAllFormProps) ui.Node {
	uistate.UseActiveMember().Set("")
	uistate.UsePeriod().Set(period.NewWindow(period.Month, budgetNow(), time.Sunday))
	uistate.UseBudgetsLastMonth().Set(false)
	return ui.CreateElement(coverAllForm, props)
}

func TestCSCOVERFormTracksSelectionsAndReseedsChangedTarget(t *testing.T) {
	cscoverSeedApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(cscoverFormHarness, coverAllFormProps{}))
	f.Stabilize()
	if !isDisabled(byTestID(f, "button", "cover-all-apply")) || len(f.AllByTag("select")) != 2 {
		t.Fatal("form must require an explicit choice")
	}
	byTestID(f, "select", "cover-all-src-food").Dispatch("onchange", render.Event{Value: "dining"})
	f.Stabilize()
	if apply := byTestID(f, "button", "cover-all-apply"); apply == nil || apply.Property("disabled") == true || apply.Text() != uistate.T("bv7g.cover.apply", fmtMoney(money.New(3730, "USD"))) {
		t.Fatalf("selected apply=%v", apply)
	}
	f.Render(ui.CreateElement(cscoverFormHarness, coverAllFormProps{TargetID: "travel"}))
	f.Stabilize()
	if len(f.AllByTag("select")) != 1 || !isDisabled(byTestID(f, "button", "cover-all-apply")) {
		t.Fatal("new target retained old selections")
	}
	f.Render(ui.CreateElement(cscoverFormHarness, coverAllFormProps{TargetID: "missing"}))
	f.Stabilize()
	if byTestID(f, "button", "cover-all-close") == nil || len(f.AllByTag("select")) != 0 {
		t.Fatal("missing target should close")
	}
}

func TestCSCOVERApplyWaitsForCheckpointAndRejectsFailedCheckpoint(t *testing.T) {
	vw := cscoverSeedApp(t)
	a := appstate.Default
	overs, sources, base := coverAllData(a, "", vw, prefs.Default(), false)
	m := coverplan.Prepare(overs, sources, base, "")
	chosen := map[string]string{"food": "dining"}
	var finish func(error)
	commits := 0
	restore := uistate.SetCheckpointCommitForTest(func(_, _ map[string]string, done func(error)) { commits++; finish = done })
	defer restore()
	saving, done, errText := false, false, ""
	actions := coverAllActions{IsSaving: func() bool { return saving }, SetSaving: func(v bool) { saving = v }, SetError: func(v string) { errText = v }, OnDone: func() { done = true }}
	before := a.Budgets()
	applyCoverAll(a, m, chosen, vw, actions)
	if !saving || finish == nil || done || !reflect.DeepEqual(before, a.Budgets()) {
		t.Fatal("cover ran before its checkpoint committed")
	}
	applyCoverAll(a, m, chosen, vw, actions)
	if commits != 1 {
		t.Fatal("duplicate apply while checkpoint is saving")
	}
	finish(errors.New("disk full"))
	if saving || done || !reflect.DeepEqual(before, a.Budgets()) {
		t.Fatal("failed checkpoint changed budgets")
	}
	applyCoverAll(a, m, chosen, vw, actions)
	finish(nil)
	if saving || !done || errText != "" {
		t.Fatalf("successful checkpoint: saving=%v done=%v error=%q", saving, done, errText)
	}
	var total int64
	for _, b := range a.Budgets() {
		for _, v := range b.PeriodBoosts {
			total += v
		}
	}
	if total != 0 {
		t.Fatalf("unbalanced cover=%d", total)
	}
}

func TestCSCOVERApplyValidationAndWriteFailureKeepModalOpen(t *testing.T) {
	vw := cscoverSeedApp(t)
	a := appstate.Default
	m := cscoverModel()
	var message string
	done, saving := false, false
	actions := coverAllActions{IsSaving: func() bool { return saving }, SetSaving: func(v bool) { saving = v }, SetError: func(v string) { message = v }, OnDone: func() { done = true }}
	applyCoverAll(a, m, map[string]string{"food": "missing"}, vw, actions)
	if message != uistate.T("coverAll.currencyError") || done || saving {
		t.Fatal("invalid donor started checkpoint")
	}
	completeCoverAll(a, m, nil, nil, vw, actions)
	if message != uistate.T("coverAll.doneNone") || done {
		t.Fatal("empty cover completed")
	}
	completeCoverAll(a, m, nil, []budgeting.CoverRequest{{DestinationID: "missing"}}, vw, actions)
	if message != uistate.T("budgets.writeFailed") || done {
		t.Fatal("rejected write completed")
	}
	message = ""
	applyCoverAll(nil, m, nil, vw, actions)
	m.CurrencyError = true
	applyCoverAll(a, m, nil, vw, actions)
	if message != "" || saving || done {
		t.Fatal("guard changed state")
	}
}

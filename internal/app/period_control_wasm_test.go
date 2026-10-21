// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestPeriodControlTracksBodyAfterStepsAndExternalWrites(t *testing.T) {
	f := render.New(t)
	defer func() {
		f.Render(uic.Fragment())
		f.Stabilize()
		f.Cleanup()
	}()
	var atom state.Atom[period.Window]
	f.Render(uic.CreateElement(func() uic.Node {
		atom = uistate.UsePeriod()
		viewed := atom.Get()
		return Div(
			uic.CreateElement(shellResolutionSlot, shellResolutionSlotProps{PeriodAware: true, Window: viewed}),
			uic.CreateElement(periodControlBodyTest),
		)
	}))
	f.Stabilize()
	initial := atom.Get()
	t.Cleanup(func() {
		atom.Set(initial)
		uistate.PersistPeriodWindow(initial)
	})
	july := period.NewWindow(period.Month, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC), time.Monday)
	atom.Set(july)
	f.Stabilize()
	assertPeriodControlAndBody(t, f, "Jul 2026")

	previous := periodStepButton(t, f, "resolution.prevPeriod")
	previous.Click()
	f.Stabilize()
	if got := atom.Get(); got != july.Shift(-1) {
		t.Fatalf("previous period set %+v, want %+v", got, july.Shift(-1))
	}
	assertPeriodControlAndBody(t, f, "Jun 2026")

	// Both clicks can reach a handler from the same render before reconciliation.
	previous = periodStepButton(t, f, "resolution.prevPeriod")
	previous.Click()
	previous.Click()
	f.Stabilize()
	if got := atom.Get(); got != july.Shift(-3) {
		t.Fatalf("two rapid clicks set %+v, want %+v", got, july.Shift(-3))
	}
	assertPeriodControlAndBody(t, f, "Apr 2026")

	atom.Set(july.Shift(1))
	f.Stabilize()
	assertPeriodControlAndBody(t, f, "Aug 2026")
}

func periodControlBodyTest() uic.Node {
	return Span(Attr("data-testid", "period-control-body-test"), uistate.UsePeriod().Get().Label())
}

func periodStepButton(t *testing.T, f *render.Fixture, labelKey string) *render.QueryNode {
	t.Helper()
	for _, node := range f.AllByTag("button") {
		if node.Attr("aria-label") == uistate.T(labelKey) {
			return node
		}
	}
	t.Fatalf("period button %q missing", labelKey)
	return nil
}

func assertPeriodControlAndBody(t *testing.T, f *render.Fixture, want string) {
	t.Helper()
	var pill, body *render.QueryNode
	for _, node := range f.AllByTag("button") {
		if node.Attr("data-testid") == "period-pill" {
			pill = node
			break
		}
	}
	for _, node := range f.AllByTag("span") {
		if node.Attr("data-testid") == "period-control-body-test" {
			body = node
			break
		}
	}
	if pill == nil || body == nil || !strings.Contains(pill.Text(), want) || body.Text() != want {
		t.Fatalf("period header/body mismatch: pill=%v body=%v, want %q", queryText(pill), queryText(body), want)
	}
}

func queryText(node *render.QueryNode) string {
	if node == nil {
		return "<missing>"
	}
	return node.Text()
}

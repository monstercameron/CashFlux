// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestTCS2QuickAddDurabilityAndRetry(t *testing.T) {
	for _, another := range []bool{false, true} {
		t.Run(map[bool]string{false: "save", true: "save another"}[another], func(t *testing.T) {
			a, err := appstate.New(&bytes.Buffer{}, false)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Store().Close()
			if err := a.PutAccount(domain.Account{ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
				t.Fatal(err)
			}
			old, oldCapture := appstate.Default, auditview.CaptureNow
			appstate.Default = a
			auditview.CaptureNow = func() {}
			defer func() { uistate.SetQuickAdd(false); appstate.Default = old; auditview.CaptureNow = oldCapture }()
			f := render.New(t)
			defer f.Cleanup()
			var open state.Atom[bool]
			f.Render(ui.CreateElement(func() ui.Node { open = uistate.UseQuickAdd(); return Body(ui.CreateElement(QuickAddHost)) }))
			uistate.SetQuickAdd(true)
			f.Stabilize()
			f.ByID("txn-add-amount").Input("12.50")
			f.ByID("txn-add-payee").Input("Market")
			var complete func(appstate.DurableResult)
			var rev uint64
			requests := 0
			a.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { requests++; rev, complete = r, done })
			click := func() {
				if another {
					findAppTestID(f, "button", "txn-add-another").Click()
				} else {
					f.ByID("txn-add-save").Click()
				}
			}
			click()
			if complete == nil || !open.Get() || len(a.Transactions()) != 1 {
				t.Fatal("quick add closed/reset before durability or omitted write")
			}
			if f.ByID("txn-add-amount").Attr("data-cf-value") != "12.50" {
				t.Fatal("draft cleared before durability")
			}
			click()
			if requests != 1 || len(a.Transactions()) != 1 {
				t.Fatal("pending click duplicated a write")
			}
			complete(appstate.DurableResult{Revision: rev, Err: errors.New("quota exhausted")})
			if !open.Get() || f.ByID("txn-add-amount").Attr("data-cf-value") != "12.50" {
				t.Fatal("failed save discarded draft")
			}
			click()
			if requests != 2 || len(a.Transactions()) != 1 {
				t.Fatal("retry created a second transaction")
			}
			complete(appstate.DurableResult{Revision: rev})
			if another {
				if !open.Get() || f.ByID("txn-add-amount").Attr("data-cf-value") != "" {
					t.Fatalf("durable rapid entry did not reset: open=%v amount=%q", open.Get(), f.ByID("txn-add-amount").Attr("data-cf-value"))
				}
			} else if open.Get() {
				t.Fatal("durable save did not close")
			}
		})
	}
}

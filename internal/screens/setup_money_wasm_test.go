//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestSetupMoneyInputsRejectInvalidWithoutWritingOrAdvancing(t *testing.T) {
	a := guardTestApp(t)
	if err := uistate.PersistPrefs(prefs.Default()); err != nil {
		t.Fatal(err)
	}
	var finish func(appstate.DurableResult)
	var revision uint64
	a.SetDurableWriter(func(rev uint64, done func(appstate.DurableResult)) {
		revision, finish = rev, done
	})
	f := render.New(t)
	f.Render(ui.CreateElement(SetupWizard))
	byTestID(f, "form", "setup-currency-form").Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	if finish == nil {
		t.Fatal("currency step did not await durability")
	}
	finish(appstate.DurableResult{Revision: revision})
	f.Stabilize()

	income := byTestID(f, "input", "setup-income")
	income.Dispatch("oninput", render.Event{Value: "1.001"})
	byTestID(f, "form", "setup-income-form").Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	if byTestID(f, "input", "setup-income") == nil {
		t.Fatal("invalid income advanced setup")
	}
	if got := uistate.CurrentPrefs().MonthlyIncomeMinor; got != 0 {
		t.Fatalf("invalid income partially wrote %d minor units", got)
	}
	if !strings.Contains(f.Text(), uistate.T("setup.amountInvalid")) {
		t.Fatal("invalid income error is not visible")
	}

	byTestID(f, "input", "setup-income").Dispatch("oninput", render.Event{Value: "12.34"})
	byTestID(f, "form", "setup-income-form").Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	if got := uistate.CurrentPrefs().MonthlyIncomeMinor; got != 1234 {
		t.Fatalf("income minor units = %d, want 1234", got)
	}
	if byTestID(f, "input", "setup-acct-name") == nil {
		t.Fatal("valid income did not advance to account step")
	}

	byTestID(f, "input", "setup-acct-name").Dispatch("oninput", render.Event{Value: "Card"})
	byTestID(f, "select", "setup-acct-type").Dispatch("onchange", render.Event{Value: string(domain.TypeCreditCard)})
	byTestID(f, "input", "setup-acct-bal").Dispatch("oninput", render.Event{Value: "2.001"})
	byTestID(f, "form", "setup-account-form").Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	if len(a.Accounts()) != 0 || byTestID(f, "input", "setup-acct-bal") == nil {
		t.Fatal("invalid account balance wrote an account or advanced setup")
	}

	byTestID(f, "input", "setup-acct-bal").Dispatch("oninput", render.Event{Value: "2.01"})
	byTestID(f, "form", "setup-account-form").Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	accounts := a.Accounts()
	if len(accounts) != 1 || accounts[0].OpeningBalance.Amount != 201 || accounts[0].Class != domain.ClassLiability {
		t.Fatalf("valid liability setup result = %+v", accounts)
	}
}

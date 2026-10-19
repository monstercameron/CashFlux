// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/txnlink"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestCSLINKSummary(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnLinkSummary, txnlink.Summary{Description: "Merchant", AccountLine: "Date and account", Amount: "($6.75)", Tone: "text-down"}))
	summary := findByTestID(f, "div", "txnlink-summary")
	if summary == nil || summary.Attr("class") != "txnlink-summary" || !strings.Contains(summary.Text(), "Merchant") || !strings.Contains(summary.Text(), "Date and account") || !strings.Contains(summary.Text(), "($6.75)") {
		t.Fatalf("summary = %#v; class=%q text=%q", summary, summary.Attr("class"), summary.Text())
	}
	if len(summary.Children()) != 2 {
		t.Fatal("summary markup changed")
	}
}

func TestCSLINKMode(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	selected := ""
	props := txnLinkModeProps{Selected: uistate.TxnLinkModeBill, Change: func(v string) { selected = v }}
	f.Render(ui.CreateElement(txnLinkMode, props))
	bill := findByTestID(f, "button", "txnlink-mode-bill")
	sub := findByTestID(f, "button", "txnlink-mode-sub")
	if bill == nil || sub == nil || bill.Attr("aria-checked") != "true" || sub.Attr("aria-checked") != "false" {
		t.Fatal("mode semantics changed")
	}
	sub.Click()
	if selected != uistate.TxnLinkModeSub {
		t.Fatal("mode callback missing")
	}
	props.Selected = selected
	f.Render(ui.CreateElement(txnLinkMode, props))
	if findByTestID(f, "button", "txnlink-mode-sub").Attr("aria-checked") != "true" {
		t.Fatal("new selection not rendered")
	}
	csLinkNoHookErrors(t, f)
}

func TestCSLINKSubscriptionPicker(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	selected := ""
	props := txnLinkSubscriptionPickerProps{Change: func(v string) { selected = v }}
	f.Render(ui.CreateElement(txnLinkSubscriptionPicker, props))
	if findByTestID(f, "p", "txnlink-no-subs") == nil {
		t.Fatal("empty state missing")
	}
	props.Options = []txnlink.Option{{Value: "A", Label: "A"}, {Value: "B", Label: "B"}}
	f.Render(ui.CreateElement(txnLinkSubscriptionPicker, props))
	selectNode := findByTestID(f, "select", "txnlink-sub-select")
	if selectNode == nil || selectNode.Attr("aria-label") != uistate.T("txnlink.subLabel") {
		t.Fatal("subscription selector missing its name")
	}
	options := f.AllByTag("option")
	if len(options) != 3 || options[0].Text() != uistate.T("txnlink.noneSub") || options[1].Text() != "A" || options[2].Text() != "B" {
		t.Fatal("option order changed")
	}
	selectNode.Change("B")
	if selected != "B" {
		t.Fatal("subscription callback missing")
	}
	props.Options = nil
	f.Render(ui.CreateElement(txnLinkSubscriptionPicker, props))
	csLinkNoHookErrors(t, f)
}

func TestCSLINKBillPicker(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	selected := ""
	toggles := 0
	props := txnLinkBillPickerProps{Merchant: "Corner store", Change: func(v string) { selected = v }, ToggleAutoLink: func() { toggles++ }}
	f.Render(ui.CreateElement(txnLinkBillPicker, props))
	if findByTestID(f, "p", "txnlink-no-debts") == nil {
		t.Fatal("empty state missing")
	}
	props.Options = []txnlink.Option{{Value: "debt", Label: "Card"}}
	f.Render(ui.CreateElement(txnLinkBillPicker, props))
	selectNode := findByTestID(f, "select", "txnlink-bill-select")
	if selectNode == nil || selectNode.Attr("aria-label") != uistate.T("txnlink.billLabel") {
		t.Fatal("bill selector missing its name")
	}
	if findByTestID(f, "input", "txnlink-autolink") != nil {
		t.Fatal("remember offered without a bill")
	}
	selectNode.Change("debt")
	if selected != "debt" {
		t.Fatal("bill callback missing")
	}
	props.Selected = "debt"
	props.ShowAutoLink = true
	props.AutoLink = true
	f.Render(ui.CreateElement(txnLinkBillPicker, props))
	check := findByTestID(f, "input", "txnlink-autolink")
	if check == nil || check.Attr("type") != "checkbox" || check.Property("checked") != true {
		t.Fatal("remember checkbox changed")
	}
	check.Change("")
	if toggles != 1 {
		t.Fatal("remember callback missing")
	}
	props.Options = nil
	f.Render(ui.CreateElement(txnLinkBillPicker, props))
	csLinkNoHookErrors(t, f)
}

func TestCSLINKPreview(t *testing.T) {
	for _, tt := range []struct {
		name  string
		draft txnlink.Draft
		want  []string
	}{
		{name: "none"},
		{name: "bill", draft: txnlink.Draft{BillAccountID: "debt"}, want: []string{uistate.T("txnlink.chipBill", "Card")}},
		{name: "sub", draft: txnlink.Draft{SubscriptionName: "Club"}, want: []string{uistate.T("txnlink.chipSub", "Club")}},
		{name: "both", draft: txnlink.Draft{BillAccountID: "debt", SubscriptionName: "Club"}, want: []string{uistate.T("txnlink.chipBill", "Card"), uistate.T("txnlink.chipSub", "Club")}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := render.New(t)
			defer f.Cleanup()
			f.Render(ui.CreateElement(txnLinkPreview, txnLinkPreviewProps{Decision: tt.draft.Evaluate(domain.Transaction{}), AccountNames: map[string]string{"debt": "Card"}}))
			preview := findByTestID(f, "div", "txnlink-preview")
			if len(tt.want) == 0 {
				if preview != nil {
					t.Fatal("empty preview visible")
				}
				return
			}
			if preview == nil {
				t.Fatal("preview missing")
			}
			if len(preview.Children()) != len(tt.want)+1 {
				t.Fatal("chip count changed")
			}
			for i, want := range tt.want {
				if preview.Children()[i+1].Text() != want {
					t.Fatalf("chip %d = %q", i, preview.Children()[i+1].Text())
				}
			}
		})
	}
}

func TestCSLINKFooter(t *testing.T) {
	for _, tt := range []struct {
		name        string
		txn         domain.Transaction
		draft       txnlink.Draft
		hint, label string
		disabled    bool
	}{
		{name: "unchanged", hint: "txnlink-nothing", label: "txnlink.save", disabled: true},
		{name: "link", draft: txnlink.Draft{BillAccountID: "debt"}, label: "txnlink.save"},
		{name: "remove", txn: domain.Transaction{BillAccountID: "debt"}, hint: "txnlink-clearhint", label: "txnlink.saveRemove"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := render.New(t)
			defer f.Cleanup()
			saved, cancelled := 0, 0
			f.Render(ui.CreateElement(txnLinkFooter, txnLinkFooterProps{Decision: tt.draft.Evaluate(tt.txn), Save: func() { saved++ }, Cancel: func() { cancelled++ }}))
			save := findByTestID(f, "button", "txnlink-save")
			if save == nil || save.Text() != uistate.T(tt.label) || save.Attr("type") != "button" {
				t.Fatal("save control changed")
			}
			if (save.Attr("aria-disabled") == "true") != tt.disabled {
				t.Fatal("save enablement changed")
			}
			if tt.hint != "" && findByTestID(f, "p", tt.hint) == nil {
				t.Fatal("save hint missing")
			}
			findByTestID(f, "button", "txnlink-cancel").Click()
			if cancelled != 1 {
				t.Fatal("cancel callback missing")
			}
			if !tt.disabled {
				save.Click()
				if saved != 1 {
					t.Fatal("save callback missing")
				}
			}
			csLinkNoHookErrors(t, f)
		})
	}
}

func TestCSLINKMissing(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	closed := 0
	f.Render(ui.CreateElement(txnLinkMissing, txnLinkMissingProps{Close: func() { closed++ }}))
	if findByTestID(f, "p", "txnlink-missing") == nil {
		t.Fatal("missing-payment message missing")
	}
	buttons := f.AllByTag("button")
	if len(buttons) != 1 || buttons[0].Text() != uistate.T("action.close") {
		t.Fatal("missing-payment footer changed")
	}
	buttons[0].Click()
	if closed != 1 {
		t.Fatal("close callback missing")
	}
}

type csLinkModal struct {
	fixture *render.Fixture
	root    func() ui.Node
	target  state.Atom[uistate.TxnLinkTarget]
	notice  state.Atom[uistate.Notice]
}

func csLinkMount(t *testing.T, app *appstate.App, txnID, mode string) *csLinkModal {
	t.Helper()
	old := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = old })
	m := &csLinkModal{fixture: render.New(t)}
	t.Cleanup(m.fixture.Cleanup)
	show := false
	host := func(visible bool) ui.Node {
		m.target = uistate.UseTxnLinkTarget()
		m.notice = uistate.UseNotice()
		if !visible {
			return Div()
		}
		return ui.CreateElement(TxnLinkBody, struct{}{})
	}
	m.root = func() ui.Node { return ui.CreateElement(host, show) }
	m.fixture.Render(m.root())
	m.target.Set(uistate.TxnLinkTarget{TxnID: txnID, Mode: mode})
	m.notice.Set(uistate.Notice{})
	show = true
	m.fixture.Render(m.root())
	return m
}

func TestCSLINKDialogPreservesDraftAcrossTabsAndCommitsBothLinks(t *testing.T) {
	app := sol1EditApp(t)
	if err := app.PutAccount(domain.Account{ID: "cslink-debt", Name: "Card", Currency: "USD", Class: domain.ClassLiability, Type: domain.TypeCreditCard, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	txn, _ := app.TransactionByID("sol1-record")
	txn.SubscriptionName = "Existing subscription"
	if err := app.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	m := csLinkMount(t, app, txn.ID, uistate.TxnLinkModeBill)
	f := m.fixture
	if findByTestID(f, "div", "txnlink-summary") == nil {
		t.Fatal("summary missing")
	}
	findByTestID(f, "select", "txnlink-bill-select").Change("cslink-debt")
	f.Render(m.root())
	findByTestID(f, "input", "txnlink-autolink").Change("")
	f.Render(m.root())
	findByTestID(f, "button", "txnlink-mode-sub").Click()
	f.Render(m.root())
	selectNode := findByTestID(f, "select", "txnlink-sub-select")
	if selectNode == nil {
		t.Fatal("existing subscription is not editable")
	}
	selectNode.Change("")
	f.Render(m.root())
	findByTestID(f, "button", "txnlink-mode-bill").Click()
	f.Render(m.root())
	if findByTestID(f, "input", "txnlink-autolink").Property("checked") != true {
		t.Fatal("tab switch lost auto-link draft")
	}
	before, _ := app.TransactionByID(txn.ID)
	if before.BillAccountID != "" || before.SubscriptionName != txn.SubscriptionName {
		t.Fatal("picker wrote before Save")
	}
	findByTestID(f, "button", "txnlink-save").Click()
	f.Render(m.root())
	after, _ := app.TransactionByID(txn.ID)
	if after.BillAccountID != "cslink-debt" || after.SubscriptionName != "" {
		t.Fatalf("saved transaction: %#v", after)
	}
	if len(app.Rules()) != 1 || app.Rules()[0].Match != txn.Payee {
		t.Fatalf("rules = %#v", app.Rules())
	}
	if m.target.Get().TxnID != "" {
		t.Fatal("save did not close")
	}
	if m.notice.Get().Text != uistate.T("txnlink.savedWithRule") {
		t.Fatalf("notice = %#v", m.notice.Get())
	}
	csLinkNoHookErrors(t, f)
}

func TestCSLINKDialogNoopCancelAndDisappearance(t *testing.T) {
	for _, mode := range []string{"noop", "cancel", "deleted", "nil app"} {
		t.Run(mode, func(t *testing.T) {
			app := sol1EditApp(t)
			if mode == "nil app" {
				app = nil
			}
			m := csLinkMount(t, app, "sol1-record", uistate.TxnLinkModeBill)
			f := m.fixture
			switch mode {
			case "noop":
				closed := false
				txn, _ := app.TransactionByID("sol1-record")
				saveTxnLink(app, txn, txnlink.Draft{}, func() { closed = true })
				if closed || !m.notice.Get().Err || m.notice.Get().Text != uistate.T("txnlink.nothingToSave") {
					t.Fatal("no-op changed behavior")
				}
				if findByTestID(f, "button", "txnlink-save").Attr("aria-disabled") != "true" {
					t.Fatal("no-op Save enabled")
				}
			case "cancel":
				findByTestID(f, "button", "txnlink-cancel").Click()
				if m.target.Get().TxnID != "" {
					t.Fatal("Cancel did not close")
				}
			case "deleted":
				if err := app.DeleteTransaction("sol1-record"); err != nil {
					t.Fatal(err)
				}
				uistate.BumpDataRevision()
				f.Render(m.root())
				if findByTestID(f, "p", "txnlink-missing") == nil {
					t.Fatal("deleted transaction not handled")
				}
				f.AllByTag("button")[0].Click()
				if m.target.Get().TxnID != "" {
					t.Fatal("missing Close did not close")
				}
			case "nil app":
				if findByTestID(f, "p", "txnlink-missing") == nil {
					t.Fatal("nil app not handled")
				}
			}
			csLinkNoHookErrors(t, f)
		})
	}
}

func csLinkNoHookErrors(t *testing.T, f *render.Fixture) {
	t.Helper()
	for _, d := range f.BuildDiagnostics() {
		message := strings.ToLower(d.Message)
		if strings.Contains(message, "panic") || strings.Contains(message, "hook") {
			t.Fatalf("component diagnostic: %#v", d)
		}
	}
}

func TestCSLINKSaveFailureKeepsDialogAndDraft(t *testing.T) {
	app := sol1EditApp(t)
	m := csLinkMount(t, app, "sol1-record", uistate.TxnLinkModeBill)
	txn, _ := app.TransactionByID("sol1-record")
	txn.Amount.Currency = ""
	closed := false
	saveTxnLink(app, txn, txnlink.Draft{SubscriptionName: "Club"}, func() { closed = true })
	if closed || m.target.Get().TxnID != "sol1-record" || !m.notice.Get().Err {
		t.Fatal("failed write closed or claimed success")
	}
	stored, _ := app.TransactionByID(txn.ID)
	if stored.SubscriptionName != "" {
		t.Fatal("failed save changed stored link")
	}
}

func TestCSLINKSaveClearsLinksAndDeduplicatesRules(t *testing.T) {
	for _, mode := range []string{"clear", "duplicate rule"} {
		t.Run(mode, func(t *testing.T) {
			app := sol1EditApp(t)
			txn, _ := app.TransactionByID("sol1-record")
			txn.BillAccountID = "debt"
			if err := app.PutTransaction(txn); err != nil {
				t.Fatal(err)
			}
			draft := txnlink.Draft{}
			wantNotice := "txnlink.cleared"
			if mode == "duplicate rule" {
				draft = txnlink.Draft{BillAccountID: "debt", AutoLink: true}
				rule, _ := txnlink.BillRule(nil, txn, draft, "existing", 1)
				if err := app.PutRule(rule); err != nil {
					t.Fatal(err)
				}
				wantNotice = "txnlink.saved"
			}
			m := csLinkMount(t, app, txn.ID, uistate.TxnLinkModeBill)
			saveTxnLink(app, txn, draft, func() { m.target.Set(uistate.TxnLinkTarget{}) })
			if m.notice.Get().Text != uistate.T(wantNotice) || m.target.Get().TxnID != "" {
				t.Fatalf("save = %#v", m.notice.Get())
			}
			if mode == "duplicate rule" && len(app.Rules()) != 1 {
				t.Fatal("duplicate auto-link rule")
			}
			if mode == "clear" {
				got, _ := app.TransactionByID(txn.ID)
				if got.BillAccountID != "" || got.SubscriptionName != "" {
					t.Fatal("links were not removed")
				}
			}
		})
	}
}

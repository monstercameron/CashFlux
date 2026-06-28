// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens — recurring.go holds the /recurring route and the
// RecurringManagerPanel component extracted from Planning() (FEATURE_MAP §5.7a).
// Moving the panel here gives /recurring a real scoped screen and lets Planning
// embed the same component without duplicating logic or hooks.
package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// RecurringManagerPanelProps holds configuration for RecurringManagerPanel.
// Currently the panel reads all state from appstate.Default, so no props are
// needed; the struct exists so call sites pass RecurringManagerPanelProps{} and
// future props can be added without altering existing callers.
type RecurringManagerPanelProps struct{}

// RecurringManagerPanel is the self-contained recurring cash-flow manager: the
// add-form (label/amount/cadence/account/category/first-due/autopost/autopay),
// auto-detected charges section, monthly-total note, per-row inline-edit list,
// and "Post due" action. It owns all its own hooks so it can be embedded at
// multiple call sites (Planning and /recurring) with isolated state per mount.
func RecurringManagerPanel(p RecurringManagerPanelProps) ui.Node {
	app := appstate.Default
	base := "USD"
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}

	// === Hooks — all unconditional (GWC rule) ===

	// Revision counter: any write operation increments this, causing the panel to
	// re-render and re-read app.Recurring() so the list stays in sync.
	rev := ui.UseState(0)
	rLabel := ui.UseState("")
	rAmount := ui.UseState("")
	rCadence := ui.UseState(string(domain.CadenceMonthly))
	rAccount := ui.UseState("")
	rCategory := ui.UseState("")
	rAutopost := ui.UseState(false)
	rAutopay := ui.UseState(false) // C157
	rNextDue := ui.UseState("")    // C149: first due date (blank = today)
	rErr := ui.UseState("")
	postMsg := ui.UseState("")
	onRLabel := ui.UseEvent(func(v string) { rLabel.Set(v) })
	onRAmount := ui.UseEvent(func(v string) { rAmount.Set(v) })
	onRNextDue := ui.UseEvent(func(v string) { rNextDue.Set(v) })
	onRCadence := ui.UseEvent(func(e ui.Event) { rCadence.Set(e.GetValue()) })
	onRAccount := ui.UseEvent(func(e ui.Event) { rAccount.Set(e.GetValue()) })
	onRCategory := ui.UseEvent(func(e ui.Event) { rCategory.Set(e.GetValue()) })
	addRecurring := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		label := strings.TrimSpace(rLabel.Get())
		if label == "" {
			rErr.Set(uistate.T("recurring.labelRequired"))
			return
		}
		amt, err := money.ParseMinor(strings.TrimSpace(rAmount.Get()), currency.Decimals(base))
		if err != nil || amt == 0 {
			rErr.Set(uistate.T("recurring.amountRequired"))
			return
		}
		// C149: honor the chosen first-due date; blank/invalid falls back to today.
		nextDue := time.Now()
		if s := strings.TrimSpace(rNextDue.Get()); s != "" {
			if d, derr := dateutil.ParseDate(s); derr == nil {
				nextDue = d
			}
		}
		r := domain.Recurring{
			ID: id.New(), Label: label, Amount: money.New(amt, base),
			Cadence: domain.RecurringCadence(rCadence.Get()), NextDue: nextDue,
			AccountID: rAccount.Get(), CategoryID: rCategory.Get(), Autopost: rAutopost.Get(), Autopay: rAutopay.Get(),
		}
		if err := app.PutRecurring(r); err != nil {
			rErr.Set(err.Error())
			return
		}
		rLabel.Set("")
		rAmount.Set("")
		rAutopost.Set(false)
		rAutopay.Set(false)
		rNextDue.Set("")
		rErr.Set("")
		rev.Set(rev.Get() + 1)
	}))
	deleteRecurring := func(rid string) {
		if app != nil {
			_ = app.DeleteRecurring(rid)
			rev.Set(rev.Get() + 1)
		}
	}

	// C147: one-click "add to plan" for an auto-detected recurring charge. Builds a
	// domain.Recurring from the detected subscription (charges are expenses → stored
	// negative, matching the sign convention) and persists it, then refreshes.
	addDetected := func(s subscriptions.Subscription) {
		if app == nil {
			return
		}
		nextDue := s.NextRenewal
		if nextDue.IsZero() {
			nextDue = time.Now()
		}
		r := domain.Recurring{
			ID: id.New(), Label: s.Name, Amount: money.New(-s.Amount, base),
			Cadence: domain.RecurringCadence(string(s.Cadence)), NextDue: nextDue,
		}
		if err := app.PutRecurring(r); err == nil {
			rev.Set(rev.Get() + 1)
		}
	}
	// C153: inline-edit a recurring. The row builds the edited Recurring (preserving
	// ID/NextDue/Autopost) and hands it here to persist.
	editRecurring := func(r domain.Recurring) {
		if app == nil {
			return
		}
		if err := app.PutRecurring(r); err != nil {
			return
		}
		rev.Set(rev.Get() + 1)
	}
	postDue := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		n, err := app.PostDueRecurring(time.Now())
		if err != nil {
			postMsg.Set(err.Error())
			return
		}
		postMsg.Set(uistate.T("recurring.posted", plural(n, "transaction")))
		rev.Set(rev.Get() + 1)
	}))

	// === Rendering (nil-guarded) ===

	if app == nil {
		return Fragment()
	}

	cadenceOpts := []ui.Node{
		Option(Value(string(domain.CadenceWeekly)), SelectedIf(rCadence.Get() == string(domain.CadenceWeekly)), uistate.T("recurring.cadenceWeekly")),
		Option(Value(string(domain.CadenceBiweekly)), SelectedIf(rCadence.Get() == string(domain.CadenceBiweekly)), uistate.T("recurring.cadenceBiweekly")),
		Option(Value(string(domain.CadenceMonthly)), SelectedIf(rCadence.Get() == string(domain.CadenceMonthly)), uistate.T("recurring.cadenceMonthly")),
		Option(Value(string(domain.CadenceSemimonthly)), SelectedIf(rCadence.Get() == string(domain.CadenceSemimonthly)), uistate.T("recurring.cadenceSemimonthly")),
		Option(Value(string(domain.CadenceQuarterly)), SelectedIf(rCadence.Get() == string(domain.CadenceQuarterly)), uistate.T("recurring.cadenceQuarterly")),
		Option(Value(string(domain.CadenceYearly)), SelectedIf(rCadence.Get() == string(domain.CadenceYearly)), uistate.T("recurring.cadenceYearly")),
	}
	acctOpts := []ui.Node{Option(Value(""), SelectedIf(rAccount.Get() == ""), uistate.T("recurring.noAccount"))}
	for _, ac := range app.Accounts() {
		acctOpts = append(acctOpts, Option(Value(ac.ID), SelectedIf(rAccount.Get() == ac.ID), ac.Name))
	}
	catOpts := []ui.Node{Option(Value(""), SelectedIf(rCategory.Get() == ""), uistate.T("recurring.noCategory"))}
	for _, c := range app.Categories() {
		catOpts = append(catOpts, Option(Value(c.ID), SelectedIf(rCategory.Get() == c.ID), c.Name))
	}
	recs := app.Recurring()
	var monthlyTotal int64
	for _, r := range recs {
		monthlyTotal += r.MonthlyEquivalent()
	}

	// C147: surface auto-detected recurring charges that aren't in the plan yet,
	// ungated (the SMART-P1 insight only fires when Smart is enabled, off by
	// default — so detection never reached most users). Each detected charge gets
	// a one-click "Add to plan". Already-planned labels and liability payments
	// (loan/card autopay — would double-count) are filtered out.
	detRates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	existingLabels := map[string]bool{}
	for _, r := range recs {
		existingLabels[strings.ToLower(strings.TrimSpace(r.Label))] = true
	}
	detected, _ := subscriptions.Detect(app.Transactions(), detRates, 3)
	var detectedRows []ui.Node
	for _, s := range detected {
		if existingLabels[strings.ToLower(strings.TrimSpace(s.Name))] {
			continue
		}
		if subscriptions.IsLiabilityPayment(s, app.Transactions(), app.Accounts()) {
			continue
		}
		sub := s // capture per-iteration value for the row's OnAdd closure
		detectedRows = append(detectedRows, ui.CreateElement(detectedRecurringRow, detectedRecurringRowProps{
			Name:    sub.Name,
			Monthly: uistate.T("recurring.detectedMonthly", fmtMoney(money.New(sub.MonthlyAmount(), base)), cadenceLabel(domain.RecurringCadence(string(sub.Cadence)))),
			OnAdd:   func() { addDetected(sub) },
		}))
	}
	detectedSection := Fragment()
	if len(detectedRows) > 0 {
		detectedSection = Div(css.Class("detected-recurring", tw.Mt2), Attr("data-testid", "detected-recurring"),
			P(css.Class("row-desc"), uistate.T("recurring.detectedTitle", plural(len(detectedRows), "charge"))),
			P(css.Class("muted"), uistate.T("recurring.detectedHint")),
			Div(css.Class("rows"), detectedRows),
		)
	}
	totalNote := Fragment()
	if len(recs) > 0 {
		totalNote = P(css.Class("muted"), uistate.T("recurring.monthlyTotal", fmtMoney(money.New(monthlyTotal, base))))
	}
	list := IfElse(len(recs) == 0,
		ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("recurring.empty"), CTALabel: uistate.T("recurring.add"), FocusID: "recurring-add"}),
		Div(css.Class("rows"), MapKeyed(recs,
			func(r domain.Recurring) any { return r.ID },
			func(r domain.Recurring) ui.Node {
				return ui.CreateElement(RecurringRow, recurringRowProps{Recurring: r, Accounts: app.Accounts(), Categories: app.Categories(), Base: base, OnDelete: deleteRecurring, OnSave: editRecurring})
			},
		)),
	)
	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("recurring.title"),
		// C156: HTML id anchor so /recurring route and /planning#recurring are directly linkable.
		Attrs: []any{Attr("id", "recurring")},
		Body: Fragment(
			P(css.Class("muted"), uistate.T("recurring.hint")),
			Form(css.Class("form-grid"), OnSubmit(addRecurring),
				Input(append([]any{css.Class("field"), Attr("id", "recurring-add"), Type("text"), Placeholder(uistate.T("recurring.labelPlaceholder")), Value(rLabel.Get()), OnInput(onRLabel)}, errAttrs("refi-err", rErr.Get())...)...),
				labeledField(uistate.T("recurring.amountPlaceholder", base), Input(css.Class("field"), Type("number"), Value(rAmount.Get()), Step("0.01"), OnInput(onRAmount))),
				Select(css.Class("field"), Attr("aria-label", uistate.T("recurring.cadence")), Title(uistate.T("recurring.cadence")), OnChange(onRCadence), cadenceOpts),
				Select(css.Class("field"), Attr("aria-label", uistate.T("recurring.account")), Title(uistate.T("recurring.account")), OnChange(onRAccount), acctOpts),
				Select(css.Class("field"), Attr("aria-label", uistate.T("recurring.category")), Title(uistate.T("recurring.category")), OnChange(onRCategory), catOpts),
				// C149: first-due date (blank = today).
				labeledField(uistate.T("recurring.nextDueLabel"), Input(css.Class("field"), Type("date"), Attr("data-testid", "recurring-nextdue"), Attr("aria-label", uistate.T("recurring.nextDueLabel")), Value(rNextDue.Get()), OnInput(onRNextDue))),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("recurring.add")),
			),
			uiw.ToggleRow(uiw.ToggleRowProps{Label: uistate.T("recurring.autopost"), On: rAutopost.Get(), OnChange: func(v bool) { rAutopost.Set(v) }}),
			uiw.ToggleRow(uiw.ToggleRowProps{Label: uistate.T("recurring.autopay"), On: rAutopay.Get(), OnChange: func(v bool) { rAutopay.Set(v) }}), // C157
			errText("refi-err", rErr.Get()),
			totalNote,
			detectedSection,
			list,
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
				Button(css.Class("btn"), Type("button"), Title(uistate.T("recurring.postDueTitle")), OnClick(postDue), uistate.T("recurring.postDue")),
				If(postMsg.Get() != "", Span(css.Class("muted"), postMsg.Get())),
			),
		),
	})
}

// RecurringHubProps holds configuration for RecurringHub. The struct exists so
// call sites pass RecurringHubProps{} and future props can be added without
// altering existing callers.
type RecurringHubProps struct{}

// RecurringHub is a registered component that owns the three-tab /recurring hub
// (FEATURE_MAP §5.3): Scheduled (RecurringManagerPanel), Bills (BillsPanel), and
// Subscriptions (SubscriptionsPanel). Each tab body is a separate ui.CreateElement
// call — hooks are isolated inside each child component, so tab switching is
// hook-safe regardless of which panels have been rendered previously.
func RecurringHub(p RecurringHubProps) ui.Node {
	activeTab := ui.UseState("scheduled")

	tab := activeTab.Get()
	var content ui.Node
	switch tab {
	case "bills":
		content = ui.CreateElement(BillsPanel, BillsPanelProps{})
	case "subscriptions":
		content = ui.CreateElement(SubscriptionsPanel, SubscriptionsPanelProps{})
	default:
		content = ui.CreateElement(RecurringManagerPanel, RecurringManagerPanelProps{})
	}

	return Div(
		Div(css.Class(tw.Mb2),
			uiw.Segmented(uiw.SegmentedProps{
				Label:    "Recurring view",
				Selected: tab,
				OnSelect: func(v string) { activeTab.Set(v) },
				Options: []uiw.SegOption{
					{Value: "scheduled", Label: uistate.T("recurring.tabScheduled")},
					{Value: "bills", Label: uistate.T("recurring.tabBills")},
					{Value: "subscriptions", Label: uistate.T("recurring.tabSubscriptions")},
				},
			}),
		),
		content,
	)
}

// Recurring is the /recurring route — the dedicated "Money that repeats" page.
// It renders RecurringHub, which provides a three-tab view: Scheduled (the
// cash-flow manager), Bills, and Subscriptions (FEATURE_MAP §5.3/§5.7b).
// The shell provides the heading and subtitle from the route registry
// (nav.recurring / screen.recurringSub); RecurringHub owns all content.
func Recurring() ui.Node {
	return ui.CreateElement(RecurringHub, RecurringHubProps{})
}

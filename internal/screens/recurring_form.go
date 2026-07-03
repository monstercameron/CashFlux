// SPDX-License-Identifier: MIT

//go:build js && wasm

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
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// RecurringFormProps configures the add/edit-recurring flip-modal body.
type RecurringFormProps struct {
	ID     string // "new" (or "") to create, else the recurring ID to edit
	OnDone func() // called to close the modal
}

// RecurringForm is the add/edit-recurring flip-modal body: label, direction
// (money in/out) + amount, cadence, optional account/category links, the first
// due date, and the autopost/autopay toggles. Editing seeds every field from the
// existing flow; saving persists and closes via OnDone. Its own component so its
// many input hooks sit at stable positions.
func RecurringForm(props RecurringFormProps) ui.Node {
	app := appstate.Default
	base := "USD"
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}
	dec := currency.Decimals(base)

	isNew := props.ID == "" || props.ID == "new"
	var existing domain.Recurring
	if !isNew && app != nil {
		for _, r := range app.Recurring() {
			if r.ID == props.ID {
				existing = r
				break
			}
		}
	}

	seedAmount, seedDir, seedDue := "", "out", ""
	if !isNew {
		seedAmount = money.FormatMinor(existing.Amount.Abs().Amount, currency.Decimals(existing.Amount.Currency))
		if !existing.Amount.IsNegative() {
			seedDir = "in"
		}
		seedDue = existing.NextDue.Format("2006-01-02")
	}

	labelS := ui.UseState(existing.Label)
	amountS := ui.UseState(seedAmount)
	dirS := ui.UseState(seedDir)
	cadenceS := ui.UseState(recurSeedCadence(existing))
	accountS := ui.UseState(existing.AccountID)
	categoryS := ui.UseState(existing.CategoryID)
	autopostS := ui.UseState(existing.Autopost)
	autopayS := ui.UseState(existing.Autopay)
	dueS := ui.UseState(seedDue)
	errS := ui.UseState("")

	onLabel := ui.UseEvent(func(v string) { labelS.Set(v) })
	onAmount := ui.UseEvent(func(v string) { amountS.Set(v) })
	onCadence := ui.UseEvent(func(e ui.Event) { cadenceS.Set(e.GetValue()) })
	onAccount := ui.UseEvent(func(e ui.Event) { accountS.Set(e.GetValue()) })
	onCategory := ui.UseEvent(func(e ui.Event) { categoryS.Set(e.GetValue()) })
	onDue := ui.UseEvent(func(v string) { dueS.Set(v) })

	save := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		label := strings.TrimSpace(labelS.Get())
		if label == "" {
			errS.Set(uistate.T("recurring.labelRequired"))
			return
		}
		amt, err := money.ParseMinor(strings.TrimSpace(amountS.Get()), dec)
		if err != nil || amt == 0 {
			errS.Set(uistate.T("recurring.amountRequired"))
			return
		}
		if amt < 0 {
			amt = -amt // magnitude field; the direction toggle owns the sign
		}
		if dirS.Get() == "out" {
			amt = -amt
		}
		nextDue := time.Now()
		if !isNew {
			nextDue = existing.NextDue
		}
		if s := strings.TrimSpace(dueS.Get()); s != "" {
			if d, derr := dateutil.ParseDate(s); derr == nil {
				nextDue = d
			}
		}
		rid := props.ID
		if isNew {
			rid = id.New()
		}
		// Auto-post needs an account to post into; without one the toggle is inert
		// (and shown disabled), so never persist it on.
		autopost := autopostS.Get() && accountS.Get() != ""
		r := domain.Recurring{
			ID: rid, Label: label, Amount: money.New(amt, base),
			Cadence: domain.RecurringCadence(cadenceS.Get()), NextDue: nextDue,
			AccountID: accountS.Get(), CategoryID: categoryS.Get(),
			Autopost: autopost, Autopay: autopayS.Get(),
		}
		if perr := app.PutRecurring(r); perr != nil {
			errS.Set(perr.Error())
			return
		}
		errS.Set("")
		uistate.BumpDataRevision()
		if props.OnDone != nil {
			props.OnDone()
		}
	}))
	cancel := ui.UseEvent(Prevent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	cadenceOpts := []ui.Node{}
	for _, c := range []domain.RecurringCadence{
		domain.CadenceWeekly, domain.CadenceBiweekly, domain.CadenceSemimonthly,
		domain.CadenceMonthly, domain.CadenceQuarterly, domain.CadenceYearly,
	} {
		cadenceOpts = append(cadenceOpts, Option(Value(string(c)), SelectedIf(cadenceS.Get() == string(c)), recurCadence(c)))
	}
	acctOpts := []ui.Node{Option(Value(""), SelectedIf(accountS.Get() == ""), uistate.T("recurring.noAccount"))}
	catOpts := []ui.Node{Option(Value(""), SelectedIf(categoryS.Get() == ""), uistate.T("recurring.noCategory"))}
	if app != nil {
		for _, a := range app.Accounts() {
			if a.Archived {
				continue
			}
			acctOpts = append(acctOpts, Option(Value(a.ID), SelectedIf(accountS.Get() == a.ID), a.Name))
		}
		for _, c := range app.Categories() {
			catOpts = append(catOpts, Option(Value(c.ID), SelectedIf(categoryS.Get() == c.ID), c.Name))
		}
	}

	saveLabel := uistate.T("recurring.add")
	if !isNew {
		saveLabel = uistate.T("recurring.saveFlow")
	}

	return Div(css.Class("rec-modal"), Attr("data-testid", "recurring-form"),
		Form(css.Class("form-grid rec-modal-form"), OnSubmit(save),
			labeledField(uistate.T("recurring.labelPlaceholder"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "rec-label"), Attr("autofocus", "true"),
					Placeholder(uistate.T("recurring.labelPlaceholder")), Value(labelS.Get()), OnInput(onLabel))),
			labeledField(uistate.T("recurring.directionLabel"),
				uiw.Segmented(uiw.SegmentedProps{
					Label:    uistate.T("recurring.directionLabel"),
					Selected: dirS.Get(),
					OnSelect: func(v string) { dirS.Set(v) },
					Options: []uiw.SegOption{
						{Value: "out", Label: uistate.T("recurring.dirOut"), TestID: "rec-dir-out"},
						{Value: "in", Label: uistate.T("recurring.dirIn"), TestID: "rec-dir-in"},
					},
				})),
			labeledField(uistate.T("recurring.amountLabel", base),
				Input(css.Class("field"), Type("number"), Attr("min", "0"), Step("0.01"), Attr("data-testid", "rec-amount"),
					Value(amountS.Get()), OnInput(onAmount))),
			labeledField(uistate.T("recurring.cadence"),
				Select(css.Class("field"), Attr("aria-label", uistate.T("recurring.cadence")), Attr("data-testid", "rec-cadence"),
					OnChange(onCadence), cadenceOpts)),
			labeledField(uistate.T("recurring.accountOptional"),
				Select(css.Class("field"), Attr("aria-label", uistate.T("recurring.account")), Attr("data-testid", "rec-account"),
					OnChange(onAccount), acctOpts)),
			labeledField(uistate.T("recurring.categoryOptional"),
				Select(css.Class("field"), Attr("aria-label", uistate.T("recurring.category")), Attr("data-testid", "rec-category"),
					OnChange(onCategory), catOpts)),
			Div(css.Class("rec-modal-wide"),
				labeledField(uistate.T("recurring.nextDueLabel"),
					Input(css.Class("field"), Type("date"), Attr("data-testid", "recurring-nextdue"),
						Attr("aria-label", uistate.T("recurring.nextDueLabel")), Value(dueS.Get()), OnInput(onDue)))),
			Div(css.Class("rec-modal-toggles"),
				// Auto-post is inert without an account — dim it and say why, instead of
				// letting the dependency fail silently at post time.
				Div(ClassStr(recurAutopostWrapClass(accountS.Get())),
					uiw.ToggleRow(uiw.ToggleRowProps{Label: uistate.T("recurring.autopost"), On: autopostS.Get() && accountS.Get() != "", OnChange: func(v bool) { autopostS.Set(v) }}),
				),
				If(accountS.Get() == "", P(css.Class("muted rec-autopost-hint"), uistate.T("recurring.autopostNeedsAccount"))),
				uiw.ToggleRow(uiw.ToggleRowProps{Label: uistate.T("recurring.autopay"), On: autopayS.Get(), OnChange: func(v bool) { autopayS.Set(v) }}),
			),
			If(errS.Get() != "", P(css.Class("err"), Attr("role", "alert"), errS.Get())),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
				Button(css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "rec-save"), saveLabel),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "rec-cancel"), OnClick(cancel), uistate.T("action.cancel")),
			),
		),
	)
}

// recurSeedCadence returns the cadence to seed the form's select with: the
// existing flow's cadence when editing, else monthly.
func recurSeedCadence(existing domain.Recurring) string {
	if existing.ID != "" {
		return string(existing.Cadence)
	}
	return string(domain.CadenceMonthly)
}

// recurAutopostWrapClass dims the auto-post toggle while no account is linked (the
// toggle would be inert — auto-post needs an account to post into).
func recurAutopostWrapClass(accountID string) string {
	if accountID == "" {
		return "rec-toggle-disabled"
	}
	return ""
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/accountedit"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// SweepRulesFormProps configures the sweep-rules manager. OnDone closes the host modal.
type SweepRulesFormProps struct {
	OnDone func()
}

// SweepRulesForm lists the household's surplus-sweep rules and offers an add form:
// keep $X in a source account, and move the excess to a destination account on a
// cadence. A rule never runs by itself. It proposes a transfer through the card on
// /accounts. This component owns the add-form hooks, and each rule row is a child
// component so no On* hook sits inside the rules loop.
func SweepRulesForm(props SweepRulesFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()

	src := ui.UseState("")
	dst := ui.UseState("")
	keep := ui.UseState("")
	cadence := ui.UseState(string(domain.SweepMonthly))
	errMsg := ui.UseState("")

	onKeep := ui.UseEvent(func(v string) { keep.Set(v) })

	accounts := app.Accounts()
	names := map[string]string{}
	curOf := map[string]string{}
	byID := map[string]domain.Account{}
	for _, a := range accounts {
		names[a.ID] = a.Name
		curOf[a.ID] = a.Currency
		byID[a.ID] = a
	}
	// nameOf is the account's name, or a localized "Unknown account" for a rule whose account no
	// longer exists, so its row never renders blank.
	nameOf := func(accountID string) string {
		if n := names[accountID]; n != "" {
			return n
		}
		return uistate.T("acctSweepCfg.unknownAccount")
	}
	// keepCurrency is the currency a rule's keep amount is shown in: its source account's, or the
	// base currency when that account no longer exists.
	keepCurrency := func(accountID string) string {
		if c := curOf[accountID]; c != "" {
			return c
		}
		return accountedit.BaseCurrencyOrDefault(app.Settings().BaseCurrency)
	}

	add := ui.UseEvent(Prevent(func() {
		source, dest := src.Get(), dst.Get()
		if source == "" || dest == "" || source == dest {
			errMsg.Set(uistate.T("acctSweepCfg.pickTwo"))
			return
		}
		keepMinor, err := money.ParseMinor(keep.Get(), currency.Decimals(curOf[source]))
		if err != nil || keepMinor < 0 {
			errMsg.Set(uistate.T("acctSweepCfg.keepInvalid"))
			return
		}
		if _, err := app.PutSweepRule(domain.SweepRule{
			SourceAccountID: source, DestAccountID: dest, KeepMinor: keepMinor,
			Cadence: domain.SweepCadence(cadence.Get()), Enabled: true,
		}); err != nil {
			slog.Error("sweep rules: save rule", "error", err)
			errMsg.Set(acctWriteErrorText(err))
			return
		}
		src.Set("")
		dst.Set("")
		keep.Set("")
		errMsg.Set("")
		uistate.RequestPersist()
		uistate.BumpDataRevision()
	}))

	rules := app.SweepRules()
	var ruleNodes []ui.Node
	for _, r := range rules {
		ruleNodes = append(ruleNodes, WithKey(ui.CreateElement(sweepRuleRow, sweepRuleRowProps{
			Rule: r, FromName: nameOf(r.SourceAccountID), ToName: nameOf(r.DestAccountID),
			KeepStr: fmtMoney(money.New(r.KeepMinor, keepCurrency(r.SourceAccountID))),
		}), r.ID))
	}

	// The pickers list only accounts that can take part in a sweep. A sweep is a cash
	// transfer, so it uses the transfer eligibility rule (acctTransferOptions): liquid
	// cash sources only, no valuation-only destinations, liabilities labelled as
	// payments, and each side excludes the other's selection.
	transferFrom, transferTo := acctTransferOptions(accounts, src.Get(), dst.Get())
	sweepFromOpts, sweepToOpts := sweepPairOptions(sweepAccountOptions(transferFrom), sweepAccountOptions(transferTo),
		byID, src.Get(), dst.Get())
	cadenceOpts := sweepCadenceOptions()

	// Called unconditionally so the hook keeps a stable position. The Done button
	// only renders when OnDone is set.
	onDone := ui.UseEvent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	})

	// The intro, the rules and the add-form fields scroll inside .modal-scroll. The
	// primary Add action stays pinned in .modal-foot so it is always reachable in the
	// small modal. Submitting adds a rule and keeps the modal open, so several rules
	// can be added in a row. Done closes the modal.
	return Form(css.Class("acct-edit-form"), Attr("id", "sweep-rules-form"),
		Attr("data-testid", "sweep-rules-form"), OnSubmit(add),
		Div(css.Class("modal-scroll", tw.Flex, tw.FlexCol, tw.Gap3),
			P(css.Class("t-caption", tw.TextDim), uistate.T("acctSweepCfg.intro")),
			If(len(ruleNodes) > 0, Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2), ruleNodes)),
			If(len(ruleNodes) == 0, P(css.Class("t-caption", tw.TextDim), uistate.T("acctSweepCfg.empty"))),
			// The add form: one field per line (label over control) so the picks read in the
			// order the sentence does: keep money in X, hold $Y, sweep the rest to Z, this often.
			Div(css.Class("sweep-add"),
				labeledField(uistate.T("accountsRedesign.sweepKeepIn"),
					uiw.SelectInput(uiw.SelectInputProps{Options: sweepFromOpts, Selected: src.Get(),
						OnChange: func(v string) { src.Set(v) }, AriaLabel: uistate.T("acctSweepCfg.source"), TestID: "sweep-cfg-source"})),
				labeledField(uistate.T("acctSweepCfg.keepAmount"),
					Input(css.Class("field"), Type("text"), Attr("inputmode", "decimal"),
						AriaLabel(uistate.T("acctSweepCfg.keepAmount")), Placeholder(sweepKeepExample(keepCurrency(src.Get()))),
						Attr("data-testid", "sweep-cfg-keep"), OnInput(onKeep), uiw.FieldValue(keep.Get()))),
				labeledField(uistate.T("accountsRedesign.sweepMoveTo"),
					uiw.SelectInput(uiw.SelectInputProps{Options: sweepToOpts, Selected: dst.Get(),
						OnChange: func(v string) { dst.Set(v) }, AriaLabel: uistate.T("acctSweepCfg.dest"), TestID: "sweep-cfg-dest"})),
				labeledField(uistate.T("acctSweepCfg.cadence"),
					uiw.SelectInput(uiw.SelectInputProps{Options: cadenceOpts, Selected: cadence.Get(),
						OnChange: func(v string) { cadence.Set(v) }, AriaLabel: uistate.T("acctSweepCfg.cadence"), TestID: "sweep-cfg-cadence"})),
			),
			If(errMsg.Get() != "", P(css.Class("t-caption", tw.TextDown), Role("alert"), errMsg.Get())),
		),
		Div(css.Class("modal-foot", tw.Flex, tw.ItemsCenter, tw.Gap2),
			If(props.OnDone != nil, Button(css.Class("btn"), Type("button"),
				Attr("data-testid", "sweep-cfg-close"), OnClick(onDone), uistate.T("action.done"))),
			Button(css.Class("btn btn-primary"), Type("submit"),
				Attr("data-testid", "sweep-cfg-add"), uistate.T("acctSweepCfg.add")),
		),
	)
}

// sweepKeepExample is the sample keep amount shown in the amount field, written in the currency the
// amount will be read in.
func sweepKeepExample(ccy string) string {
	dec := currency.Decimals(ccy)
	minor := int64(3000)
	for range dec {
		minor *= 10
	}
	return money.FormatMinor(minor, dec)
}

// sweepCadenceLabel is the plain-English name of a sweep cadence, such as "Monthly".
func sweepCadenceLabel(c domain.SweepCadence) string {
	return uistate.T("acctSweepCfg.cadence" + strconv.Itoa(c.CadenceDays()))
}

// sweepCadenceOptions are the entries of the cadence picker, from the most to the least frequent.
func sweepCadenceOptions() []uiw.SelectOption {
	cadences := []domain.SweepCadence{domain.SweepWeekly, domain.SweepBiweekly, domain.SweepMonthly, domain.SweepQuarterly}
	opts := make([]uiw.SelectOption, 0, len(cadences))
	for _, c := range cadences {
		opts = append(opts, uiw.SelectOption{Value: string(c), Label: sweepCadenceLabel(c)})
	}
	return opts
}

// sweepAccountOptions turns the transfer picker's options into the sweep picker's: the same
// accounts, with the sweep's own "Choose an account" prompt in place of the transfer placeholder.
func sweepAccountOptions(transfer []uiw.SelectOption) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("acctSweepCfg.pickAccount")}}
	for _, o := range transfer {
		if o.Value != "" {
			opts = append(opts, o)
		}
	}
	return opts
}

// sweepPairOptions narrows the sweep pickers to pairs a rule can hold, using the rule PutSweepRule
// applies (appstate.SweepPairError): distinct, active accounts in one currency, with a source that
// is not a liability. With a source chosen, the destinations are the accounts it can sweep into.
// With a destination chosen, the sources are the accounts that can sweep into it. With neither
// chosen, a source that has no valid destination among the destination options is not offered.
// byID maps an account id to its account.
func sweepPairOptions(from, to []uiw.SelectOption, byID map[string]domain.Account, srcID, dstID string) (fromOpts, toOpts []uiw.SelectOption) {
	pairOK := func(srcAcct, dstAcct string) bool {
		s, sok := byID[srcAcct]
		d, dok := byID[dstAcct]
		return sok && dok && appstate.SweepPairError(s, d) == nil
	}
	hasPartner := func(id string) bool {
		for _, o := range to {
			if o.Value != "" && pairOK(id, o.Value) {
				return true
			}
		}
		return false
	}
	for _, o := range from {
		switch {
		case o.Value == "":
		case dstID != "" && !pairOK(o.Value, dstID):
			continue
		case dstID == "" && !hasPartner(o.Value):
			continue
		}
		fromOpts = append(fromOpts, o)
	}
	for _, o := range to {
		if o.Value != "" && srcID != "" && !pairOK(srcID, o.Value) {
			continue
		}
		toOpts = append(toOpts, o)
	}
	return fromOpts, toOpts
}

type sweepRuleRowProps struct {
	Rule     domain.SweepRule
	FromName string
	ToName   string
	KeepStr  string
}

// sweepRuleRow renders one saved sweep rule with a delete button. It is its own
// component so the delete hook stays stable across the rules loop.
func sweepRuleRow(props sweepRuleRowProps) ui.Node {
	app := appstate.Default
	ruleText := uistate.T("acctSweepCfg.ruleLine", props.KeepStr, props.FromName, props.ToName,
		sweepCadenceLabel(props.Rule.Cadence))
	del := ui.UseEvent(Prevent(func() {
		uistate.ConfirmModal(uistate.T("acctSweepCfg.removeConfirm", ruleText), true, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeleteSweepRule(props.Rule.ID); err != nil {
				postAcctWriteError("delete sweep rule", err)
				return
			}
			uistate.RequestPersist()
			uistate.BumpDataRevision()
		})
	}))
	return Div(css.Class("row", tw.Flex, tw.ItemsCenter, tw.Gap2),
		Attr("data-testid", "sweep-rule-row-"+props.Rule.ID),
		Span(css.Class("t-body", tw.Flex1), ruleText),
		Button(css.Class("btn btn-ghost btn-sm btn-del"), Type("button"),
			Attr("data-testid", "sweep-rule-del-"+props.Rule.ID), OnClick(del),
			AriaLabel(uistate.T("acctSweepCfg.removeAria", ruleText)),
			uistate.T("acctSweepCfg.remove")),
	)
}

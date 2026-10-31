// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

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

	onSrc := ui.UseEvent(func(v string) { src.Set(v) })
	onDst := ui.UseEvent(func(v string) { dst.Set(v) })
	onKeep := ui.UseEvent(func(v string) { keep.Set(v) })
	onCadence := ui.UseEvent(func(v string) { cadence.Set(v) })

	accounts := app.Accounts()
	nameOf := map[string]string{}
	curOf := map[string]string{}
	for _, a := range accounts {
		nameOf[a.ID] = a.Name
		curOf[a.ID] = a.Currency
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
			errMsg.Set(err.Error())
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
		ruleNodes = append(ruleNodes, ui.CreateElement(sweepRuleRow, sweepRuleRowProps{
			Rule: r, FromName: nameOf[r.SourceAccountID], ToName: nameOf[r.DestAccountID],
			KeepStr: fmtMoney(money.New(r.KeepMinor, curOf[r.SourceAccountID])),
		}))
	}

	// The pickers list only accounts that can take part in a sweep. A sweep is a cash
	// transfer, so it uses the transfer eligibility rule (acctTransferOptions): liquid
	// cash sources only, no valuation-only destinations, liabilities labelled as
	// payments, and each side excludes the other's selection.
	sweepFromOpts, sweepToOpts := acctTransferOptions(accounts, src.Get(), dst.Get())
	optionNodes := func(opts []uiw.SelectOption, sel string) []ui.Node {
		nodes := []ui.Node{Option(Value(""), uistate.T("acctSweepCfg.pickAccount"))}
		for _, o := range opts {
			if o.Value == "" {
				continue // the transfer placeholder is replaced by the sweep one above
			}
			args := []any{Value(o.Value), o.Label}
			if o.Value == sel {
				args = append(args, Attr("selected", "selected"))
			}
			nodes = append(nodes, Option(args...))
		}
		return nodes
	}

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
					Select(css.Class("field"), AriaLabel(uistate.T("acctSweepCfg.source")),
						Attr("data-testid", "sweep-cfg-source"), OnInput(onSrc), optionNodes(sweepFromOpts, src.Get()))),
				labeledField(uistate.T("acctSweepCfg.keepAmount"),
					Input(css.Class("field"), Type("text"), Attr("inputmode", "decimal"),
						AriaLabel(uistate.T("acctSweepCfg.keepAmount")), Placeholder("3000"),
						Attr("data-testid", "sweep-cfg-keep"), OnInput(onKeep), uiw.FieldValue(keep.Get()))),
				labeledField(uistate.T("accountsRedesign.sweepMoveTo"),
					Select(css.Class("field"), AriaLabel(uistate.T("acctSweepCfg.dest")),
						Attr("data-testid", "sweep-cfg-dest"), OnInput(onDst), optionNodes(sweepToOpts, dst.Get()))),
				labeledField(uistate.T("acctSweepCfg.cadence"),
					Select(css.Class("field"), AriaLabel(uistate.T("acctSweepCfg.cadence")),
						Attr("data-testid", "sweep-cfg-cadence"), OnInput(onCadence),
						sweepCadenceOption(domain.SweepWeekly, cadence.Get()),
						sweepCadenceOption(domain.SweepBiweekly, cadence.Get()),
						sweepCadenceOption(domain.SweepMonthly, cadence.Get()),
						sweepCadenceOption(domain.SweepQuarterly, cadence.Get()))),
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

// sweepCadenceLabel is the plain-English name of a sweep cadence, such as "Monthly".
func sweepCadenceLabel(c domain.SweepCadence) string {
	return uistate.T("acctSweepCfg.cadence" + strconv.Itoa(c.CadenceDays()))
}

// sweepCadenceOption is one entry in the cadence picker, selected when sel matches it.
func sweepCadenceOption(c domain.SweepCadence, sel string) ui.Node {
	args := []any{Value(string(c)), sweepCadenceLabel(c)}
	if string(c) == sel {
		args = append(args, Attr("selected", "selected"))
	}
	return Option(args...)
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
	del := ui.UseEvent(Prevent(func() {
		if err := appstate.Default.DeleteSweepRule(props.Rule.ID); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
	}))
	return Div(css.Class("row", tw.Flex, tw.ItemsCenter, tw.Gap2),
		Attr("data-testid", "sweep-rule-row-"+props.Rule.ID),
		Span(css.Class("t-body", tw.Flex1),
			uistate.T("acctSweepCfg.ruleLine", props.FromName, props.KeepStr, props.ToName,
				sweepCadenceLabel(props.Rule.Cadence))),
		Button(css.Class("btn btn-ghost btn-sm btn-del"), Type("button"),
			Attr("data-testid", "sweep-rule-del-"+props.Rule.ID), OnClick(del),
			uistate.T("acctSweepCfg.remove")),
	)
}

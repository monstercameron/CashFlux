// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// SweepRulesFormProps configures the AC7 sweep-rules manager. OnDone closes the
// host modal.
type SweepRulesFormProps struct {
	OnDone func()
}

// SweepRulesForm lists the household's surplus-sweep rules and offers an add
// form: keep $X in a source account, move the excess to a destination account on
// a cadence. Each rule proposes (never auto-runs) a transfer via the card on
// /accounts. Its own component so the add-form hooks sit at stable positions;
// each rule row is a child component (no On* in a loop).
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
	msg := ui.UseState("")

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
		s, d := src.Get(), dst.Get()
		if s == "" || d == "" || s == d {
			msg.Set(uistate.T("acctSweepCfg.pickTwo"))
			return
		}
		keepMinor, _ := money.ParseMinor(keep.Get(), currency.Decimals(curOf[s]))
		if keepMinor < 0 {
			keepMinor = 0
		}
		if _, err := app.PutSweepRule(domain.SweepRule{
			SourceAccountID: s, DestAccountID: d, KeepMinor: keepMinor,
			Cadence: domain.SweepCadence(cadence.Get()), Enabled: true,
		}); err != nil {
			msg.Set(err.Error())
			return
		}
		src.Set("")
		dst.Set("")
		keep.Set("")
		msg.Set("")
		uistate.RequestPersist()
		uistate.BumpDataRevision()
	}))

	rules := app.SweepRules()
	var ruleNodes []ui.Node
	for _, r := range rules {
		r := r
		ruleNodes = append(ruleNodes, ui.CreateElement(sweepRuleRow, sweepRuleRowProps{
			Rule: r, FromName: nameOf[r.SourceAccountID], ToName: nameOf[r.DestAccountID],
			KeepStr: fmtMoney(money.New(r.KeepMinor, curOf[r.SourceAccountID])),
		}))
	}

	acctOpt := func(sel string) []ui.Node {
		opts := []ui.Node{Option(Value(""), uistate.T("acctSweepCfg.pickAccount"))}
		for _, a := range accounts {
			if a.Archived {
				continue
			}
			args := []any{Value(a.ID), a.Name}
			if a.ID == sel {
				args = append(args, Attr("selected", "selected"))
			}
			opts = append(opts, Option(args...))
		}
		return opts
	}

	var onDone ui.Handler
	if props.OnDone != nil {
		onDone = ui.UseEvent(func() { props.OnDone() })
	}
	// FlushBody form: the intro, existing rules, and the add-form fields scroll in
	// .modal-scroll, while the primary "Add sweep rule" action stays pinned in the
	// .modal-foot so it is always reachable (the small modal would otherwise push it
	// below the fold). A submit there fires the form's OnSubmit — adding a rule without
	// closing — so several rules can be added in a row; Done closes the modal.
	return Form(css.Class("acct-edit-form"), Attr("id", "sweep-rules-form"),
		Attr("data-testid", "sweep-rules-form"), OnSubmit(add),
		Div(css.Class("modal-scroll", tw.Flex, tw.FlexCol, tw.Gap3),
			P(css.Class("t-caption", tw.TextDim), uistate.T("acctSweepCfg.intro")),
			If(len(ruleNodes) > 0, Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2), ruleNodes)),
			If(len(ruleNodes) == 0, P(css.Class("t-caption", tw.TextDim), uistate.T("acctSweepCfg.empty"))),
			// The add form: one field per line (label over control) so the picks read in the
			// order the sentence does — keep money in X, hold $Y, sweep the rest to Z, this often.
			Div(css.Class("sweep-add"),
				labeledField(uistate.T("accountsRedesign.sweepKeepIn"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("acctSweepCfg.source")),
						Attr("data-testid", "sweep-cfg-source"), OnInput(onSrc), acctOpt(src.Get()))),
				labeledField(uistate.T("acctSweepCfg.keepAmount"),
					Input(css.Class("field"), Type("text"), Attr("inputmode", "decimal"),
						Attr("aria-label", uistate.T("acctSweepCfg.keepAmount")), Placeholder("3,000"),
						Attr("data-testid", "sweep-cfg-keep"), Value(keep.Get()), OnInput(onKeep))),
				labeledField(uistate.T("accountsRedesign.sweepMoveTo"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("acctSweepCfg.dest")),
						Attr("data-testid", "sweep-cfg-dest"), OnInput(onDst), acctOpt(dst.Get()))),
				labeledField(uistate.T("acctSweepCfg.cadence"),
					Select(css.Class("field"), Attr("aria-label", uistate.T("acctSweepCfg.cadence")),
						Attr("data-testid", "sweep-cfg-cadence"), OnInput(onCadence),
						sweepCadenceOption(domain.SweepWeekly, cadence.Get()),
						sweepCadenceOption(domain.SweepBiweekly, cadence.Get()),
						sweepCadenceOption(domain.SweepMonthly, cadence.Get()),
						sweepCadenceOption(domain.SweepQuarterly, cadence.Get()))),
			),
			If(msg.Get() != "", P(css.Class("t-caption", tw.TextDown), Attr("role", "alert"), msg.Get())),
		),
		Div(css.Class("modal-foot", tw.Flex, tw.ItemsCenter, tw.Gap2),
			If(props.OnDone != nil, Button(css.Class("btn"), Type("button"),
				Attr("data-testid", "sweep-cfg-close"), OnClick(onDone), uistate.T("action.done"))),
			Button(css.Class("btn btn-primary"), Type("submit"),
				Attr("data-testid", "sweep-cfg-add"), uistate.T("acctSweepCfg.add")),
		),
	)
}

func sweepCadenceOption(c domain.SweepCadence, sel string) ui.Node {
	label := uistate.T("acctSweepCfg.cadence" + strconv.Itoa(c.CadenceDays()))
	args := []any{Value(string(c)), label}
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

// sweepRuleRow renders one saved sweep rule with a delete button — its own
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
				uistate.T("acctSweepCfg.cadence"+strconv.Itoa(props.Rule.Cadence.CadenceDays())))),
		Button(css.Class("btn btn-ghost btn-sm btn-del"), Type("button"),
			Attr("data-testid", "sweep-rule-del-"+props.Rule.ID), OnClick(del),
			uistate.T("acctSweepCfg.remove")),
	)
}

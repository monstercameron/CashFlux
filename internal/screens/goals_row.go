//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// GoalRow renders one goal's progress toward its target, with contribute and
// (inline) edit actions. All hooks are declared unconditionally so the edit
// toggle never reorders them.
func GoalRow(props goalRowProps) ui.Node {
	g := props.Goal
	targetMajor := money.FormatMinor(g.TargetAmount.Amount, currency.Decimals(g.TargetAmount.Currency))
	dateISO := ""
	if !g.TargetDate.IsZero() {
		dateISO = dateutil.FormatDate(g.TargetDate)
	}

	del := ui.UseEvent(Prevent(func() { props.OnDelete(g.ID) }))
	doArchive := ui.UseEvent(Prevent(func() {
		if props.OnArchive != nil {
			props.OnArchive(g.ID, true)
		}
	}))
	doUnarchive := ui.UseEvent(Prevent(func() {
		if props.OnArchive != nil {
			props.OnArchive(g.ID, false)
		}
	}))
	drillAcct := ui.UseEvent(Prevent(func() {
		if props.OnDrillAccount != nil {
			props.OnDrillAccount(g.AccountID)
		}
	}))
	pr := uistate.UsePrefs().Get()
	editing := ui.UseState(false)
	contributing := ui.UseState(false)
	contribAmtS := ui.UseState("")
	postLedgerS := ui.UseState(false)
	contribute := ui.UseEvent(Prevent(func() {
		contribAmtS.Set("")
		postLedgerS.Set(false)
		contributing.Set(true)
	}))
	onContribAmt := ui.UseEvent(func(v string) { contribAmtS.Set(v) })
	onPostLedger := ui.UseEvent(func(e ui.Event) { postLedgerS.Set(e.IsChecked()) })
	doContribute := ui.UseEvent(Prevent(func() {
		if v := strings.TrimSpace(contribAmtS.Get()); v != "" {
			props.OnContribute(g, v, postLedgerS.Get())
		}
		contributing.Set(false)
	}))
	cancelContribute := ui.UseEvent(Prevent(func() { contributing.Set(false) }))
	nameS := ui.UseState(g.Name)
	targetS := ui.UseState(targetMajor)
	dateS := ui.UseState(dateISO)
	acctS := ui.UseState(g.AccountID)
	ownerS := ui.UseState(g.OwnerID)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onTarget := ui.UseEvent(func(v string) { targetS.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	// onAcct/onOwner hooks kept for stable hook ordering; SelectInput owns the
	// change event internally so these handlers are no longer wired to DOM.
	ui.UseEvent(func(e ui.Event) { acctS.Set(e.GetValue()) })
	ui.UseEvent(func(e ui.Event) { ownerS.Set(e.GetValue()) })
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(g.Name)
		targetS.Set(targetMajor)
		dateS.Set(dateISO)
		acctS.Set(g.AccountID)
		ownerS.Set(g.OwnerID)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(g.ID, nameS.Get(), targetS.Get(), dateS.Get(), acctS.Get(), ownerS.Get())
		editing.Set(false)
	}))

	// Land the cursor in the first field when an inline editor opens (§6.7).
	ui.UseEffect(func() func() {
		switch {
		case contributing.Get():
			focusByID("goal-contrib-" + g.ID)
		case editing.Get():
			focusByID("goal-edit-" + g.ID)
		}
		return nil
	}, fmt.Sprintf("%t-%t", editing.Get(), contributing.Get()))

	if contributing.Get() {
		linkedAcctName := accountName(props.Accounts, g.AccountID)
		var ledgerRow ui.Node = Fragment()
		if linkedAcctName != "" {
			cbArgs := []any{Type("checkbox"), Attr("id", "goal-contrib-ledger-"+g.ID), OnChange(onPostLedger)}
			if postLedgerS.Get() {
				cbArgs = append(cbArgs, Attr("checked", ""))
			}
			ledgerRow = labeledField(
				uistate.T("goals.contributePostLedger", linkedAcctName),
				Input(cbArgs...),
			)
		}
		return Div(css.Class("budget"),
			Div(css.Class("budget-head"), Span(css.Class("row-desc"), g.Name)),
			Form(css.Class("form-grid"), OnSubmit(doContribute),
				labeledField(uistate.T("goals.contributeAmount"),
					Input(css.Class("field"), Attr("id", "goal-contrib-"+g.ID), Type("number"), Placeholder(uistate.T("goals.contributeAmount")), Value(contribAmtS.Get()), Step("0.01"), OnInput(onContribAmt))),
				ledgerRow,
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("goals.contribute")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelContribute), uistate.T("action.cancel")),
			),
		)
	}
	if editing.Get() {
		return Div(css.Class("budget"),
			Form(css.Class("form-grid"), OnSubmit(saveEdit),
				labeledField(uistate.T("common.name"),
					Input(css.Class("field"), Attr("id", "goal-edit-"+g.ID), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName))),
				labeledField(uistate.T("goals.targetLabel"),
					Input(css.Class("field"), Type("number"), Placeholder(uistate.T("goals.targetLabel")), Value(targetS.Get()), Step("0.01"), OnInput(onTarget))),
				labeledField(uistate.T("goals.dateLabel"),
					Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("goals.dateLabel")), Value(dateS.Get()), OnInput(onDate))),
				labeledField(uistate.T("goals.owner"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   ownerSelectOptions(props.Members, ownerS.Get()),
						Selected:  ownerS.Get(),
						OnChange:  func(v string) { ownerS.Set(v) },
						AriaLabel: uistate.T("goals.owner"),
					})),
				labeledField(uistate.T("goals.linked"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   goalAccountOptions(props.Accounts, acctS.Get()),
						Selected:  acctS.Get(),
						OnChange:  func(v string) { acctS.Set(v) },
						AriaLabel: uistate.T("goals.linked"),
					})),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	pct := goalsvc.Percent(g)
	rem, _ := goalsvc.Remaining(g)
	complete, _ := goalsvc.IsComplete(g)
	overfund, _ := goalsvc.Overfund(g)
	pace := goalsvc.ClassifyPace(g, time.Now())

	// Sub-line: split into primary (actionable: remaining + deadline + monthly needed)
	// and secondary (confirmatory: % complete). The secondary is rendered in a dimmer
	// tone so Aaliyah can scan the right-side actionable figures without equal-weight
	// noise competing with them (G5/C50 "text-busy" fix).
	var subPrimary, subSecondary string
	if complete {
		subPrimary = uistate.T("goals.complete")
	} else {
		subPrimary = fmtMoney(rem) + " to go"
		subSecondary = fmt.Sprintf("%d%%", pct)
		if !g.TargetDate.IsZero() {
			subPrimary += uistate.T("goals.bySuffix", pr.FormatDate(g.TargetDate))
			if per, ok, _ := goalsvc.MonthlyNeeded(g, time.Now()); ok {
				subPrimary += uistate.T("goals.saveSuffix", fmtMoney(per))
			}
		}
	}

	redirect := ui.UseEvent(Prevent(func() {
		if props.OnRedirect != nil {
			props.OnRedirect()
		}
	}))

	// "What next" prompt: a completed (not-yet-archived) goal frees up whatever was
	// going into it — offer a calm, dismissible jump to Allocate to redirect it
	// toward another goal (L20). No nagging: it's a single low-key line.
	var whatNext ui.Node = Fragment()
	if complete && !g.Archived {
		whatNext = Div(css.Class("budget-sub"), Attr("data-testid", "goal-whatnext-"+g.ID),
			Span(uistate.T("goals.whatNext")+" "),
			Button(css.Class("budget-drill"), Type("button"), Attr("aria-label", uistate.T("goals.whatNextAction")),
				Attr("data-testid", "goal-redirect-"+g.ID), OnClick(redirect), uistate.T("goals.whatNextAction")),
		)
	}

	// Over-funding note: shown whenever the current amount exceeds the target.
	// We compute the real (un-clamped) percentage so e.g. a goal funded to 120%
	// reads "Funded 120% — $X over" rather than a bare surplus dollar amount (L59).
	var overfundNote ui.Node = Fragment()
	if overfund.IsPositive() {
		realPct := 0
		if g.TargetAmount.Amount > 0 {
			realPct = int(g.CurrentAmount.Amount * 100 / g.TargetAmount.Amount)
		}
		overfundNote = Span(
			css.Class("budget-sub"),
			Attr("data-testid", "goal-overfund-"+g.ID),
			Style(map[string]string{"color": "var(--up)"}),
			fmt.Sprintf("Funded %d%% — %s", realPct, uistate.T("goals.overTarget", fmtMoney(overfund))),
		)
	}

	// The linked account is split out of the run-on sub-line into its own clickable
	// element that drills to that account's transactions (C51).
	linkedName := accountName(props.Accounts, g.AccountID)
	var linkedLine ui.Node = Fragment()
	if linkedName != "" {
		linkedLine = Span(css.Class("budget-sub"),
			Button(css.Class("budget-drill"), Type("button"), Title(uistate.T("nav.transactions")), OnClick(drillAcct),
				Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0", "font": "inherit", "color": "inherit", "cursor": "pointer", "text-decoration": "underline", "text-decoration-style": "dotted", "text-underline-offset": "3px"}),
				uistate.T("goals.linkedSuffix", linkedName)),
		)
	}

	// Archive button shown on complete active goals; Unarchive shown on archived goals.
	var archiveBtn ui.Node = Fragment()
	if g.Archived {
		archiveBtn = Button(
			css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15),
			Type("button"),
			Attr("aria-label", uistate.T("goals.unarchiveTitle")),
			Title(uistate.T("goals.unarchiveTitle")),
			Attr("data-testid", "goal-unarchive-"+g.ID),
			OnClick(doUnarchive),
			Span(uistate.T("goals.unarchive")),
		)
	} else if complete {
		archiveBtn = Button(
			css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15),
			Type("button"),
			Attr("aria-label", uistate.T("goals.archiveTitle")),
			Title(uistate.T("goals.archiveTitle")),
			Attr("data-testid", "goal-archive-"+g.ID),
			OnClick(doArchive),
			Span(uistate.T("goals.archive")),
		)
	}

	return Div(css.Class("budget"),
		Attr("data-testid", "goal-row-"+g.ID),
		Div(css.Class("budget-head"),
			Span(css.Class("row-desc"), g.Name),
			paceBadge(pace),
			Span(css.Class("budget-amount"), fmtMoney(g.CurrentAmount)+" / "+fmtMoney(g.TargetAmount)),
			If(!g.Archived, Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("goals.contributeTitle")), OnClick(contribute), uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.contribute")))),
			If(!g.Archived, Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("goals.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit")))),
			archiveBtn,
			Button(css.Class("btn-del", "btn-del-hover"), Type("button"), Attr("aria-label", uistate.T("goals.deleteTitle")), Title(uistate.T("goals.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
		Div(css.Class("bar"), Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(pct)), Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("goals.progressLabel")), Div(ClassStr("bar-fill "+paceBarClass(pace)), Attr("style", barFillStyle(pct)))),
		Div(css.Class("budget-sub goal-sub"),
			Span(subPrimary),
			If(subSecondary != "", Span(css.Class("goal-sub-dim"), " · "+subSecondary)),
		),
		overfundNote,
		whatNext,
		linkedLine,
	)
}

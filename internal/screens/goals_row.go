// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"time"

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

	del := ui.UseEvent(Prevent(func() {
		// Capture which row holds focus before the row is removed, so focus can
		// be restored to the next row after the delete (§6.7).
		captureRowDeleteFocus(".goal-list", "[data-testid^='goal-row-']")
		props.OnDelete(g.ID)
	}))
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
	// Edit + Contribute open the shell-root flip modal (GoalEditHost) — the goal card
	// lives under transformed tile ancestors, so an in-card modal would render off-centre.
	openEdit := ui.UseEvent(Prevent(func() {
		uistate.SetGoalEdit(uistate.GoalEdit{ID: g.ID, Mode: uistate.GoalEditModeEdit})
	}))
	openContribute := ui.UseEvent(Prevent(func() {
		uistate.SetGoalEdit(uistate.GoalEdit{ID: g.ID, Mode: uistate.GoalEditModeContribute})
	}))
	// The ⋯ actions menu (archive + the destructive delete), so the card footer stays
	// uncluttered and a misclick can't delete a goal.
	menuID := "goal-menu-" + g.ID
	menuOpen := ui.UseState(false)
	toggleMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(!menuOpen.Get()) }))
	closeMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(false) }))
	menuHidden := ""
	if !menuOpen.Get() {
		menuHidden = " hidden-menu"
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
		subPrimary = uistate.T("goals.remaining", fmtMoney(rem))
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
		realPct := goalsvc.RawPercent(g)
		overfundNote = Span(
			css.Class("budget-sub"),
			Attr("data-testid", "goal-overfund-"+g.ID),
			Style(map[string]string{"color": "var(--up)"}),
			uistate.T("goals.overfundFmt", realPct, uistate.T("goals.overTarget", fmtMoney(overfund))),
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

	// C189/C192: sinking-fund indicators — monthly set-aside chip and linked
	// category label. Both are pre-computed by the Goals screen to avoid calling
	// service functions inside a variable-length loop.
	fundSetAside := props.FundSetAside
	var fundChip ui.Node = Fragment()
	if g.IsSinkingFund && fundSetAside > 0 {
		fundAmt := money.New(fundSetAside, g.CurrentAmount.Currency)
		fundChip = Span(ClassStr("pace-badge pace-rate"),
			Attr("data-testid", "fund-setaside-"+g.ID),
			uistate.T("goals.monthlySetAside", fmtMoney(fundAmt)),
		)
	}
	var catLine ui.Node = Fragment()
	if g.IsSinkingFund && props.LinkedCategoryName != "" {
		catLine = Span(css.Class("budget-sub"),
			Attr("data-testid", "fund-category-"+g.ID),
			uistate.T("goals.fundLinkedCategory", props.LinkedCategoryName),
		)
	}

	// C178: monthly contribution rate chip shown next to the pace badge so the
	// user can see what they need to save per month without hunting through the
	// sub-line text.
	var monthlyChip ui.Node = Fragment()
	if !complete && !g.TargetDate.IsZero() {
		if per, ok, _ := goalsvc.MonthlyNeeded(g, time.Now()); ok {
			monthlyChip = Span(ClassStr("pace-badge pace-rate"),
				Attr("title", uistate.T("goals.paceNeededTitle")),
				uistate.T("goals.paceNeeded", fmtMoney(per)),
			)
		}
	}

	// Archive / Unarchive live in the ⋯ menu (archive on complete active goals).
	var archiveItem ui.Node = Fragment()
	if g.Archived {
		archiveItem = Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "goal-unarchive-"+g.ID), OnClick(doUnarchive), uistate.T("goals.unarchive"))
	} else if complete {
		archiveItem = Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
			Attr("data-testid", "goal-archive-"+g.ID), OnClick(doArchive), uistate.T("goals.archive"))
	}

	return Div(ClassStr("goal-card "+goalCardStateClass(pace, complete)),
		Attr("data-testid", "goal-row-"+g.ID),
		// Header: the goal name gets its own line, with pace / monthly / fund chips.
		Div(css.Class("goal-card-head"),
			Span(css.Class("goal-card-title"), g.Name),
			paceBadge(pace),
			monthlyChip,
			fundChip,
		),
		// The card's "loader": a saved-of-target progress bar with the amount (left) and
		// percent (right) rendered inside it, tinted by pace.
		Div(css.Class("goal-card-loader"), Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(pct)), Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"), Attr("aria-label", uistate.T("goals.progressLabel")),
			Div(ClassStr("bar-fill "+paceBarClass(pace)), Attr("style", barFillStyle(pct))),
			Div(css.Class("goal-card-loader-figs"),
				Span(css.Class("budget-amount"), Span(css.Class("budget-spent"), fmtMoney(g.CurrentAmount)), " / "+fmtMoney(g.TargetAmount)),
				Span(css.Class("budget-pct"), fmt.Sprintf("%d%%", pct)),
			),
		),
		Div(css.Class("budget-sub goal-sub"),
			Span(subPrimary),
			If(subSecondary != "", Span(css.Class("goal-sub-dim"), " · "+subSecondary)),
		),
		overfundNote,
		whatNext,
		linkedLine,
		catLine,
		// Footer: Contribute + Edit open the flip modal; the ⋯ menu holds archive + the
		// destructive delete.
		Div(css.Class("goal-card-actions"),
			If(!g.Archived, Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-contribute-"+g.ID), Attr("aria-label", uistate.T("goals.contributeTitle")), Title(uistate.T("goals.contributeTitle")), OnClick(openContribute), uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("goals.contribute")))),
			If(!g.Archived, Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "goal-edit-btn-"+g.ID), Attr("aria-label", uistate.T("goals.editTitle")), Title(uistate.T("goals.editTitle")), OnClick(openEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit")))),
			Div(css.Class("add-wrap"), Attr("id", menuID),
				Button(css.Class("btn"), Type("button"), Attr("title", uistate.T("goals.moreActions")), Attr("aria-label", uistate.T("goals.moreActions")), Attr("aria-haspopup", "menu"), Attr("aria-expanded", ariaBool(menuOpen.Get())), OnClick(toggleMenu), uiw.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
				Div(ClassStr("add-backdrop"+menuHidden), OnClick(closeMenu)),
				Div(ClassStr("add-menu"+menuHidden), Attr("role", "menu"),
					archiveItem,
					Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "goal-delete-btn-"+g.ID), Attr("aria-label", uistate.T("goals.deleteTitle")), Title(uistate.T("goals.deleteTitle")), OnClick(del), uistate.T("action.delete")),
				),
			),
		),
	)
}

// goalCardStateClass tints a goal card by its pace: a green stripe on-track, amber when
// due soon / in the final stretch, red when overdue, a calm "done" when complete.
func goalCardStateClass(p goalsvc.Pace, complete bool) string {
	if complete {
		return "is-done"
	}
	switch p {
	case goalsvc.PaceOverdue:
		return "is-overdue"
	case goalsvc.PaceDueSoon, goalsvc.PaceFinalStretch:
		return "is-soon"
	default:
		return "is-ontrack"
	}
}

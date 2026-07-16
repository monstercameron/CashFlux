// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// GoalEditFormProps drives the goal editor rendered inside the shell-root flip modal
// (see internal/app GoalEditHost). Mode selects edit vs contribute.
type GoalEditFormProps struct {
	GoalID string
	Mode   string // one of uistate.GoalEditMode*
	OnDone func() // clears the atom (closes the modal)
}

// GoalEditForm renders the goal editor (full edit, or contribute) as the body of the
// shell-root flip modal. It owns all its state and its own Save/Cancel and performs the
// mutation directly against the store, mirroring BudgetEditForm. Living at the shell
// root (outside transformed bento/tile ancestors) keeps the modal centred.
func GoalEditForm(props GoalEditFormProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	pr := uistate.UsePrefs().Get()
	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	var g domain.Goal
	found := false
	if app != nil {
		for _, gg := range app.Goals() {
			if gg.ID == props.GoalID {
				g, found = gg, true
				break
			}
		}
	}
	base := "USD"
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}
	cur := g.TargetAmount.Currency
	if cur == "" {
		cur = base
	}
	dec := currency.Decimals(cur)
	targetMajor := ""
	dateISO := ""
	monthlyMajor := ""
	if found {
		targetMajor = money.FormatMinor(g.TargetAmount.Amount, dec)
		if !g.TargetDate.IsZero() {
			dateISO = dateutil.FormatDate(g.TargetDate)
		}
		if g.MonthlyContribution.Amount > 0 {
			monthlyMajor = money.FormatMinor(g.MonthlyContribution.Amount, dec)
		}
	}

	// Kind + habit-field initial values (empty when not a habit).
	kindInit := string(g.EffectiveKind())
	cadenceInit := string(g.HabitCadence)
	if cadenceInit == "" {
		cadenceInit = string(domain.CadenceWeekly)
	}
	habitTargetInit := ""
	if g.HabitTarget > 0 {
		habitTargetInit = strconv.Itoa(g.HabitTarget)
	}

	// All hooks unconditionally at stable positions.
	nameS := ui.UseState(g.Name)
	targetS := ui.UseState(targetMajor)
	monthlyS := ui.UseState(monthlyMajor)
	dateS := ui.UseState(dateISO)
	acctSetS := ui.UseState(seedLinkSet(g.LinkedAccountIDs()))
	budgetSetS := ui.UseState(seedLinkSet(g.BudgetIDs))
	reviewCadS := ui.UseState(string(g.ReviewCadence))
	ownerS := ui.UseState(g.OwnerID)
	contribS := ui.UseState("")
	postLedgerS := ui.UseState(false)
	// Which linked account a ledger-posted contribution debits (a goal linked to
	// several accounts gets a picker; defaults to the primary).
	ledgerAcctS := ui.UseState(g.AccountID)
	notesS := ui.UseState(g.Notes)
	onNotes := ui.UseEvent(func(v string) { notesS.Set(v) })
	errS := ui.UseState("")
	kindS := ui.UseState(kindInit)
	cadenceS := ui.UseState(cadenceInit)
	habitTargetS := ui.UseState(habitTargetInit)

	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onTarget := ui.UseEvent(func(v string) { targetS.Set(v) })
	onMonthly := ui.UseEvent(func(v string) { monthlyS.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	onHabitTarget := ui.UseEvent(func(v string) { habitTargetS.Set(v) })
	onContrib := ui.UseEvent(func(v string) { contribS.Set(v) })
	onPostLedger := ui.UseEvent(func(e ui.Event) { postLedgerS.Set(e.IsChecked()) })
	cancel := ui.UseEvent(Prevent(func() { done() }))
	// GL6: attach / replace / remove the goal's vision image via the shared picker.
	attachPhoto := ui.UseEvent(Prevent(func() { attachGoalImage(props.GoalID, func(m string) { errS.Set(m) }) }))
	removePhoto := ui.UseEvent(Prevent(func() { removeGoalImage(props.GoalID) }))
	// Plain closures (not hooks) — passed down to each hook-owning goalLinkRow, so no On*
	// hook is registered inside the checklist loop.
	onToggleAcct := func(idv string) { acctSetS.Set(toggleInSet(acctSetS.Get(), idv)) }
	onToggleBudget := func(idv string) { budgetSetS.Set(toggleInSet(budgetSetS.Get(), idv)) }
	// Contribute quick-fill: set the amount to exactly what's left to reach the target.
	fillRemaining := ui.UseEvent(Prevent(func() {
		rem := g.TargetAmount.Amount - g.CurrentAmount.Amount
		if rem < 0 {
			rem = 0
		}
		contribS.Set(money.FormatMinor(rem, dec))
	}))

	saveEdit := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		for _, gg := range app.Goals() {
			if gg.ID != props.GoalID {
				continue
			}
			if n := strings.TrimSpace(nameS.Get()); n != "" {
				gg.Name = n
			}
			gg.OwnerID = ownerS.Get()
			gg.Notes = strings.TrimSpace(notesS.Get())
			if ownerS.Get() == domain.GroupOwnerID {
				gg.Scope = domain.ScopeShared
			} else {
				gg.Scope = domain.ScopeIndividual
			}
			kind := domain.GoalKind(kindS.Get())
			gg.Kind = kind
			c := gg.TargetAmount.Currency
			if c == "" {
				c = base
			}
			switch kind {
			case domain.GoalKindFinancial:
				amt, err := money.ParseMinor(strings.TrimSpace(targetS.Get()), currency.Decimals(c))
				if err != nil || amt <= 0 {
					errS.Set(uistate.T("goals.targetRequired"))
					return
				}
				gg.TargetAmount = money.New(amt, c)
				// Multi-link (0..N): store the full account + budget sets. AccountID keeps a
				// "primary" the contribute ledger-post path + the "linked to" line use — chosen
				// as the first selected account in the USER'S account order (not an arbitrary
				// UUID sort), so it's the one they'd expect.
				gg.AccountIDs = sortedSetKeys(acctSetS.Get())
				gg.BudgetIDs = sortedSetKeys(budgetSetS.Get())
				gg.AccountID = ""
				for _, a := range app.Accounts() {
					if acctSetS.Get()[a.ID] {
						gg.AccountID = a.ID
						break
					}
				}
				// Optional explicit monthly assignment for zero-based budgeting.
				if mc := strings.TrimSpace(monthlyS.Get()); mc != "" {
					if m, mErr := money.ParseMinor(mc, currency.Decimals(c)); mErr == nil && m >= 0 {
						gg.MonthlyContribution = money.New(m, c)
					}
				} else {
					gg.MonthlyContribution = money.New(0, c)
				}
			case domain.GoalKindHabit:
				n, err := strconv.Atoi(strings.TrimSpace(habitTargetS.Get()))
				if err != nil || n <= 0 {
					errS.Set(uistate.T("goals.habitTargetRequired"))
					return
				}
				gg.HabitCadence = domain.RecurringCadence(cadenceS.Get())
				gg.HabitTarget = n
				// Non-financial: keep a valid zeroed amount + drop the account link.
				gg.TargetAmount = money.New(0, c)
				gg.AccountID = ""
			default: // checklist / milestone
				gg.TargetAmount = money.New(0, c)
				gg.AccountID = ""
			}
			if ds := strings.TrimSpace(dateS.Get()); ds != "" {
				d, derr := dateutil.ParseDate(ds)
				if derr != nil {
					errS.Set(uistate.T("goals.invalidDate"))
					return
				}
				gg.TargetDate = d
			} else {
				gg.TargetDate = time.Time{}
			}
			// The review cadence (any kind). Only stamp LastReviewedAt when the cadence was
			// just set or changed — so configuring the reminder starts its clock fresh, but a
			// plain edit (fixing a name or date) does NOT silently clear a due review the user
			// never actually looked at. Explicit "Mark reviewed" and Contribute do the stamping.
			oldCad := gg.ReviewCadence
			gg.ReviewCadence = domain.RecurringCadence(reviewCadS.Get())
			if gg.ReviewCadence != oldCad {
				gg.LastReviewedAt = time.Now()
			}
			if err := app.PutGoal(gg); err != nil {
				errS.Set(err.Error())
				return
			}
			break
		}
		uistate.BumpDataRevision()
		// Same reversibility as the inline target editor — the full edit changes the
		// same fields, so both write paths offer one-click Undo.
		uistate.PostUndoable(uistate.T("goals.editSavedToast", strings.TrimSpace(nameS.Get())))
		done()
	}))

	submitContribute := ui.UseEvent(Prevent(func() {
		if app == nil {
			done()
			return
		}
		c := g.CurrentAmount.Currency
		if c == "" {
			c = base
		}
		amt, err := money.ParseMinor(strings.TrimSpace(contribS.Get()), currency.Decimals(c))
		if err != nil || amt <= 0 {
			errS.Set(uistate.T("goals.targetRequired"))
			return
		}
		beforePct := goalsvc.Percent(g)
		after := g
		after.CurrentAmount = money.New(g.CurrentAmount.Amount+amt, c)
		afterPct := goalsvc.Percent(after)
		// Debit the account the user picked (multi-linked goals choose; defaults to the
		// first ELIGIBLE link) — without altering the goal's stored link. Resolved through
		// the same liquid-only filter the picker uses, so an ineligible stored primary
		// (e.g. a grandfathered 401(k) link) can never become a ledger-debit source.
		fromAcct := ""
		for _, a := range goalEligibleDebitAccounts(app.Accounts(), g) {
			if fromAcct == "" || a.ID == ledgerAcctS.Get() {
				fromAcct = a.ID
			}
		}
		res, cerr := app.ContributeToGoalFrom(g, money.New(amt, c), postLedgerS.Get(), fromAcct)
		if cerr != nil {
			errS.Set(cerr.Error())
			return
		}
		uistate.BumpDataRevision()
		notice := uistate.T("goals.contributedToast", fmtMoney(money.New(amt, c)))
		if postLedgerS.Get() && res.TransactionID != "" {
			notice += " " + uistate.T("goals.contributedLedger")
		}
		uistate.PostUndoable(notice) // money-state change → one-click reversible
		if m := goalsvc.MilestoneCrossed(beforePct, afterPct); m > 0 {
			uistate.PostNotice(uistate.T(fmt.Sprintf("goals.milestone%d", m)), false)
		}
		if res.BecameComplete {
			uistate.PostNotice(uistate.T("goals.completionPrompt"), false)
		}
		done()
	}))

	if app == nil || !found {
		return Div(css.Class("acct-edit-form"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	var errLine ui.Node = Fragment()
	if errS.Get() != "" {
		errLine = P(css.Class("err"), Attr("role", "alert"), errS.Get())
	}

	// --- Contribute: a single amount toward the goal (optionally posted to the ledger). ---
	if props.Mode == uistate.GoalEditModeContribute {
		// Ledger-post source: ONLY liquid, unarchived linked accounts may be debited (C7:
		// a manual expense against a 401(k)/mortgage is a data-integrity hazard, the same
		// reasoning as the earmark picker). The effective account is the picker's choice
		// when it's still eligible, else the first eligible link; none → memo-only (no row).
		debitable := goalEligibleDebitAccounts(app.Accounts(), g)
		effLedgerAcct := ""
		for _, a := range debitable {
			if a.ID == ledgerAcctS.Get() {
				effLedgerAcct = a.ID
				break
			}
		}
		if effLedgerAcct == "" && len(debitable) > 0 {
			effLedgerAcct = debitable[0].ID
		}
		linkedName := accountName(app.Accounts(), effLedgerAcct)
		var ledgerRow ui.Node = Fragment()
		if linkedName != "" {
			// G7: a goal linked to several (eligible) accounts picks WHICH one the debit
			// posts against, instead of being locked to the primary.
			var acctPicker ui.Node = Fragment()
			if len(debitable) > 1 {
				opts := make([]uiw.SelectOption, 0, len(debitable))
				for _, a := range debitable {
					opts = append(opts, uiw.SelectOption{Value: a.ID, Label: a.Name})
				}
				acctPicker = Div(css.Class("goal-contrib-acct"),
					labeledField(uistate.T("goals.contribFromLabel"),
						uiw.SelectInput(uiw.SelectInputProps{
							Options: opts, Selected: effLedgerAcct, TestID: "goal-contrib-acct",
							OnChange: func(v string) { ledgerAcctS.Set(v) }, AriaLabel: uistate.T("goals.contribFromLabel"),
						})))
			}
			ledgerRow = Fragment(
				Label(css.Class("goal-check-row"), Attr("data-testid", "goal-contrib-ledger-row"),
					Input(Type("checkbox"), Attr("id", "goal-contrib-ledger-"+g.ID), OnChange(onPostLedger), Checked(postLedgerS.Get())),
					Span(uistate.T("goals.contributePostLedger", linkedName)),
				),
				If(postLedgerS.Get(), acctPicker),
			)
		}
		// Progress context: a compact bar (saved of target) and the amount left to go.
		cpct := goalsvc.Percent(g)
		rem := g.TargetAmount.Amount - g.CurrentAmount.Amount
		if rem < 0 {
			rem = 0
		}
		var finishChip ui.Node = Fragment()
		if rem > 0 {
			finishChip = Button(css.Class("contrib-chip"), Type("button"), Attr("data-testid", "goal-contrib-finish"),
				OnClick(fillRemaining), uistate.T("goals.contribFinish"))
		}
		// Earmark note here must match the card: suppressed once complete (rem == 0), and if
		// the reservation no longer fits the live balance, warn instead of claiming coverage —
		// so Contribute never contradicts the card's overbooked warning.
		var earmarkNote ui.Node = Fragment()
		if g.AllocatedMinor() > 0 && rem > 0 {
			if overbookedGoals(app)[g.ID] {
				earmarkNote = P(css.Class("t-caption", tw.TextWarn), Style(map[string]string{"margin": "0"}),
					uistate.T("goals.earmarkOverbooked", fmtMoney(goalsvc.AllocatedTotal(g))))
			} else {
				earmarkNote = P(css.Class("t-caption"), Style(map[string]string{"margin": "0", "color": "var(--up)"}),
					uistate.T("goals.contribEarmark", fmtMoney(goalsvc.AllocatedTotal(g)), goalsvc.CoveragePercent(g)))
			}
		}
		return Form(css.Class("acct-edit-form", "goal-contribute"), OnSubmit(submitContribute),
			Div(css.Class("modal-scroll"),
				Div(css.Class("contrib-loader"),
					Div(ClassStr("bar-fill"), Attr("style", barFillStyle(cpct))),
					Div(css.Class("contrib-loader-figs"),
						Span(css.Class("contrib-saved"), Span(css.Class("budget-spent"), fmtMoney(g.CurrentAmount)), " / "+fmtMoney(g.TargetAmount)),
						Span(css.Class("contrib-pct"), fmt.Sprintf("%d%%", cpct)),
					),
				),
				If(rem > 0, P(css.Class("t-caption", "muted"), Style(map[string]string{"margin": "0"}),
					uistate.T("goals.contribRemaining", fmtMoney(money.New(rem, cur)), fmtMoney(g.TargetAmount)))),
				// Keep the earmark picture visible here too (overbooked-aware, complete-gated),
				// so the Contribute view never contradicts what the card shows.
				earmarkNote,
				labeledField(uistate.T("goals.contributeAmount"),
					Div(css.Class("contrib-amount-row"),
						Input(css.Class("field"), Attr("id", "goal-contrib-"+g.ID), Attr("autofocus", ""), Type("number"),
							Placeholder(uistate.T("goals.contributeAmount")), Value(contribS.Get()), Step("0.01"), OnInput(onContrib)),
						finishChip,
					)),
				ledgerRow,
				errLine,
			),
			Div(css.Class("modal-foot"),
				Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("goals.contribute")),
			),
		)
	}

	// --- Full edit. ---
	_ = pr
	kind := domain.GoalKind(kindS.Get())
	financial := kind.IsFinancial()
	hasPhoto := g.GoalImageArtifactID != ""
	photoBtnLabel := uistate.T("goals.addPhoto")
	if hasPhoto {
		photoBtnLabel = uistate.T("goals.changePhoto")
	}
	var removePhotoBtn ui.Node = Fragment()
	if hasPhoto {
		removePhotoBtn = Button(css.Class("btn"), Type("button"), Attr("data-testid", "goal-remove-photo-"+g.ID), OnClick(removePhoto),
			uistate.T("goals.removePhoto"))
	}
	return Form(css.Class("acct-edit-form"), OnSubmit(saveEdit),
		Div(css.Class("modal-scroll"),
			labeledField(uistate.T("common.name"),
				Input(css.Class("field"), Attr("id", "goal-edit-"+g.ID), Attr("autofocus", ""), Type("text"),
					Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName))),
			labeledField(uistate.T("goals.kindLabel"),
				Div(
					uiw.SelectInput(uiw.SelectInputProps{
						Options: goalKindOptions(), Selected: kindS.Get(), TestID: "goal-edit-kind",
						OnChange: func(v string) { kindS.Set(v) }, AriaLabel: uistate.T("goals.kindLabel"),
					}),
					Span(css.Class("budget-sub"), goalKindHint(kind)),
				)),
			If(financial, labeledField(uistate.T("goals.targetLabel"),
				Input(css.Class("field"), Type("number"), Placeholder(uistate.T("goals.targetLabel")), Value(targetS.Get()), Step("0.01"), OnInput(onTarget)))),
			If(financial, labeledField(uistate.T("goals.monthlyContribLabel"),
				Input(css.Class("field"), Type("number"), Attr("data-testid", "goal-edit-monthly"), Attr("min", "0"),
					Placeholder(uistate.T("goals.monthlyContribPlaceholder")), Value(monthlyS.Get()), Step("0.01"), OnInput(onMonthly)))),
			If(kind == domain.GoalKindHabit, labeledField(uistate.T("goals.habitCadenceLabel"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options: habitCadenceOptions(), Selected: cadenceS.Get(), TestID: "goal-edit-cadence",
					OnChange: func(v string) { cadenceS.Set(v) }, AriaLabel: uistate.T("goals.habitCadenceLabel"),
				}))),
			If(kind == domain.GoalKindHabit, labeledField(uistate.T("goals.habitTargetLabel"),
				Input(css.Class("field"), Type("number"), Attr("data-testid", "goal-edit-habit-target"), Placeholder(uistate.T("goals.habitTargetPlaceholder")), Value(habitTargetS.Get()), Step("1"), OnInput(onHabitTarget)))),
			labeledField(uistate.T("goals.dateLabel"),
				Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("goals.dateLabel")), Value(dateS.Get()), OnInput(onDate))),
			labeledField(uistate.T("goals.owner"),
				uiw.SelectInput(uiw.SelectInputProps{
					Options: ownerSelectOptions(app.Members(), ownerS.Get()), Selected: ownerS.Get(),
					OnChange: func(v string) { ownerS.Set(v) }, AriaLabel: uistate.T("goals.owner"),
				})),
			// A free-text note (why this goal exists, the plan) — shown on the goal card.
			labeledField(uistate.T("goals.notesLabel"),
				uiw.TextAreaInput(uiw.TextFieldProps{Value: notesS.Get(), Placeholder: uistate.T("goals.notesPlaceholder"),
					AriaLabel: uistate.T("goals.notesLabel"), OnInput: onNotes})),
			// GL6: vision image. The photo saves to the stored goal immediately (its own
			// artifact join), so it shows a live preview + change/remove; the button label
			// reflects whether an image is already attached.
			labeledField(uistate.T("goals.addPhoto"),
				Div(
					Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
						goalImageBanner(app, g),
						Button(css.Class("btn"), Type("button"), Attr("data-testid", "goal-add-photo-"+g.ID), OnClick(attachPhoto), photoBtnLabel),
						removePhotoBtn,
					),
					Span(css.Class("budget-sub"), uistate.T("goals.photoHint")),
				)),
			// Review reminder (any kind) — how often to revisit this goal.
			labeledField(uistate.T("goals.reviewCadenceLabel"),
				Div(
					uiw.SelectInput(uiw.SelectInputProps{
						Options: reviewCadenceOptions(), Selected: reviewCadS.Get(), TestID: "goal-edit-review",
						OnChange: func(v string) { reviewCadS.Set(v) }, AriaLabel: uistate.T("goals.reviewCadenceLabel"),
					}),
					Span(css.Class("budget-sub"), uistate.T("goals.reviewCadenceHint")),
				)),
			// Linked accounts (0..N) — the accounts this financial goal draws on. C7: only
			// LIQUID cash accounts are offered (same eligibility as the earmark picker —
			// a goal's money doesn't live in a 401(k) or a mortgage). An ineligible account
			// that's ALREADY linked stays listed so it can be unchecked, never silently lost.
			If(financial, labeledField(uistate.T("goals.linkedAccounts"),
				Div(css.Class("goal-link-list"), Attr("data-testid", "goal-link-accts"),
					If(len(app.Accounts()) == 0, P(css.Class("budget-sub"), uistate.T("goals.noAccountsToLink"))),
					MapKeyed(goalLinkableAccounts(app.Accounts(), acctSetS.Get()), func(a domain.Account) any { return a.ID }, func(a domain.Account) ui.Node {
						return ui.CreateElement(goalLinkRow, goalLinkRowProps{
							ID: a.ID, Label: a.Name, Selected: acctSetS.Get()[a.ID],
							TestPrefix: "goal-link-acct", OnToggle: onToggleAcct,
						})
					}),
					Span(css.Class("budget-sub"), uistate.T("goals.linkedAccountsHint")),
				))),
			// Linked budgets (0..N) — the budget lines that feed this goal.
			If(financial, labeledField(uistate.T("goals.linkedBudgets"),
				Div(css.Class("goal-link-list"), Attr("data-testid", "goal-link-budgets"),
					If(len(app.Budgets()) == 0, P(css.Class("budget-sub"), uistate.T("goals.noBudgetsToLink"))),
					MapKeyed(app.Budgets(), func(b domain.Budget) any { return b.ID }, func(b domain.Budget) ui.Node {
						return ui.CreateElement(goalLinkRow, goalLinkRowProps{
							ID: b.ID, Label: b.Name, Selected: budgetSetS.Get()[b.ID],
							TestPrefix: "goal-link-budget", OnToggle: onToggleBudget,
						})
					}),
					Span(css.Class("budget-sub"), uistate.T("goals.linkedBudgetsHint")),
				))),
			errLine,
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
		),
	)
}

// goalLinkableAccounts returns the accounts the goal-edit "Linked accounts" checklist
// offers (C7): liquid, unarchived cash accounts — the same eligibility as the earmark
// picker — PLUS any account already in the linked set (even if ineligible), so an
// existing link can be unchecked but a 401(k)/mortgage can never be newly linked.
func goalLinkableAccounts(accounts []domain.Account, linked map[string]bool) []domain.Account {
	out := make([]domain.Account, 0, len(accounts))
	for _, a := range accounts {
		if linked[a.ID] || (!a.Archived && earmarkEligibleType(a.Type)) {
			out = append(out, a)
		}
	}
	return out
}

// goalEligibleDebitAccounts returns the goal's linked accounts that may serve as a
// ledger-debit source for a contribution: liquid, unarchived cash only. A grandfathered
// ineligible link (e.g. an old 401(k)) is excluded — posting a manual expense against a
// retirement account is a data-integrity hazard, the same reasoning as earmark eligibility.
func goalEligibleDebitAccounts(accounts []domain.Account, g domain.Goal) []domain.Account {
	var out []domain.Account
	for _, aid := range g.LinkedAccountIDs() {
		if a, ok := domain.AccountByID(accounts, aid); ok && !a.Archived && earmarkEligibleType(a.Type) {
			out = append(out, a)
		}
	}
	return out
}

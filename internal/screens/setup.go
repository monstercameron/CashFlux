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
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/setup"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// setupKVCurrencyConfirmed is the SettingKV key that records whether the user
// has explicitly confirmed a base currency in the setup wizard.
const setupKVCurrencyConfirmed = "cashflux:setup:currencyConfirmed"

// setupKVWizardDone is the SettingKV key that records the first open of the
// setup wizard (so the badge/prompt can clear).
const setupKVWizardDone = "cashflux:setup:wizardDone"

// SetupWizard is the guided /setup screen. It walks the user through four
// steps — currency & week-start, monthly income, first account, and household
// members — then shows a completion CTA to the dashboard. Each step writes its
// data live so navigating away mid-wizard does not lose progress. Hook
// positions are unconditionally stable; step visibility is controlled by a
// single integer and If(cond, node) guards.
func SetupWizard() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	// Mark the wizard as opened so the getting-started badge can clear.
	if uistate.SettingKVGet(setupKVWizardDone) == "" {
		uistate.SettingKVSet(setupKVWizardDone, "1")
	}

	// ── Step state ──────────────────────────────────────────────────────────
	stepS := ui.UseState(0) // 0–4; 4 = completion screen

	// ── Step 1 — Currency & week-start ──────────────────────────────────────
	s := app.Settings()
	initialCur := s.BaseCurrency
	if initialCur == "" {
		initialCur = "USD"
	}
	curS := ui.UseState(initialCur)
	curConfirmedS := ui.UseState(uistate.SettingKVGet(setupKVCurrencyConfirmed) == "1")

	cp := uistate.UsePrefs().Get()
	ws := string(cp.WeekStart)
	if ws == "" {
		ws = string(prefs.WeekSunday)
	}
	weekS := ui.UseState(ws)

	// ── Step 2 — Income ─────────────────────────────────────────────────────
	existingIncome := uistate.CurrentPrefs().MonthlyIncomeMinor
	incomeStr := ui.UseState("")
	if existingIncome > 0 && incomeStr.Get() == "" {
		dec := currency.Decimals(curS.Get())
		div := int64(1)
		for i := 0; i < dec; i++ {
			div *= 10
		}
		whole := existingIncome / div
		frac := existingIncome % div
		incomeStr.Set(fmt.Sprintf("%d.%0*d", whole, dec, frac))
	}

	// ── Step 3 — Account ────────────────────────────────────────────────────
	acctNameS := ui.UseState("")
	acctTypeS := ui.UseState(string(domain.TypeChecking))
	acctBalS := ui.UseState("")
	acctErrS := ui.UseState("")

	// ── Step 4 — Members ────────────────────────────────────────────────────
	memberNameS := ui.UseState("")
	memberErrS := ui.UseState("")

	// ── Event handlers — stable, unconditional hook positions ────────────────
	onCur := ui.UseEvent(func(e ui.Event) { curS.Set(e.GetValue()) })
	onWeek := ui.UseEvent(func(e ui.Event) { weekS.Set(e.GetValue()) })
	onIncomeStr := ui.UseEvent(func(e ui.Event) { incomeStr.Set(e.GetValue()) })
	onAcctName := ui.UseEvent(func(e ui.Event) { acctNameS.Set(e.GetValue()) })
	onAcctType := ui.UseEvent(func(e ui.Event) { acctTypeS.Set(e.GetValue()) })
	onAcctBal := ui.UseEvent(func(e ui.Event) { acctBalS.Set(e.GetValue()) })
	onMemberName := ui.UseEvent(func(e ui.Event) { memberNameS.Set(e.GetValue()) })

	nav := router.UseNavigate()

	// confirmCurrency saves currency + week-start and advances to step 1.
	confirmCurrency := ui.UseEvent(Prevent(func() {
		s2 := app.Settings()
		s2.BaseCurrency = curS.Get()
		_ = app.PutSettings(s2)
		uistate.SettingKVSet(setupKVCurrencyConfirmed, "1")
		curConfirmedS.Set(true)

		pr := uistate.CurrentPrefs()
		pr.WeekStart = prefs.WeekStart(weekS.Get())
		uistate.SetPrefs(pr)
		stepS.Set(1)
	}))

	// confirmIncome saves monthly income and advances to step 2.
	confirmIncome := ui.UseEvent(Prevent(func() {
		raw := strings.TrimSpace(incomeStr.Get())
		if raw != "" {
			f, err := strconv.ParseFloat(raw, 64)
			if err == nil && f >= 0 {
				dec := currency.Decimals(curS.Get())
				mult := int64(1)
				for i := 0; i < dec; i++ {
					mult *= 10
				}
				pr := uistate.CurrentPrefs()
				pr.MonthlyIncomeMinor = int64(f * float64(mult))
				uistate.SetPrefs(pr)
			}
		}
		stepS.Set(2)
	}))

	// confirmAccount creates the first account and advances to step 3.
	confirmAccount := ui.UseEvent(Prevent(func() {
		name := strings.TrimSpace(acctNameS.Get())
		if name == "" {
			acctErrS.Set(uistate.T("setup.acctNameRequired"))
			return
		}
		raw := strings.TrimSpace(acctBalS.Get())
		var balMinor int64
		if raw != "" {
			f, err := strconv.ParseFloat(raw, 64)
			if err == nil && f >= 0 {
				dec := currency.Decimals(curS.Get())
				mult := int64(1)
				for i := 0; i < dec; i++ {
					mult *= 10
				}
				balMinor = int64(f * float64(mult))
			}
		}
		acctType := domain.AccountType(acctTypeS.Get())
		acctClass := domain.ClassAsset
		switch acctType {
		case domain.TypeCreditCard, domain.TypeLineOfCredit,
			domain.TypeLoan, domain.TypePersonalLoan, domain.TypeMortgage:
			acctClass = domain.ClassLiability
		}
		a := domain.Account{
			ID:             id.New(),
			Name:           name,
			OwnerID:        domain.GroupOwnerID,
			Scope:          domain.ScopeShared,
			Class:          acctClass,
			Type:           acctType,
			Currency:       curS.Get(),
			OpeningBalance: money.New(balMinor, curS.Get()),
			BalanceAsOf:    time.Now(),
		}
		if err := app.PutAccount(a); err != nil {
			acctErrS.Set(err.Error())
			return
		}
		acctErrS.Set("")
		uistate.BumpDataRevision()
		stepS.Set(3)
	}))

	// skipAccount advances past the account step without creating one.
	skipAccount := ui.UseEvent(Prevent(func() { stepS.Set(3) }))

	// addMember creates an additional household member.
	addMember := ui.UseEvent(Prevent(func() {
		name := strings.TrimSpace(memberNameS.Get())
		if name == "" {
			memberErrS.Set(uistate.T("setup.memberNameRequired"))
			return
		}
		m := domain.Member{
			ID:    id.New(),
			Name:  name,
			Color: "#7c83ff",
		}
		if err := app.PutMember(m); err != nil {
			memberErrS.Set(err.Error())
			return
		}
		memberErrS.Set("")
		memberNameS.Set("")
		uistate.BumpDataRevision()
	}))

	// skipMembers advances to the completion screen.
	skipMembers := ui.UseEvent(Prevent(func() { stepS.Set(4) }))

	// goDashboard navigates to the main dashboard.
	goDashboard := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/")) })

	// ── Derive live progress indicators ─────────────────────────────────────
	curConfirmed := curConfirmedS.Get()
	pr2 := uistate.CurrentPrefs()
	progress := setup.Compute(curConfirmed, pr2.MonthlyIncomeMinor, app.Accounts(), app.Members())
	step := stepS.Get()

	// ── Pre-build step panels unconditionally (stable hook positions) ─────────
	progressBar := setupStepDots(progress, step)

	// Step 0: currency + week-start
	curVal := curS.Get()
	weekVal := weekS.Get()
	step0 := uiw.Card(uiw.CardProps{
		Title: uistate.T("setup.step1Title"),
		Body: Form(css.Class("form-grid"), OnSubmit(confirmCurrency),
			P(css.Class("t-caption", tw.TextDim), uistate.T("setup.step1Hint")),
			labeledField(uistate.T("setup.currencyLabel"),
				Select(css.Class("field"),
					Attr("aria-label", uistate.T("setup.currencyLabel")),
					Attr("data-testid", "setup-currency"),
					OnChange(onCur),
					MapKeyed(currency.Codes(),
						func(c string) any { return c },
						func(c string) ui.Node {
							sym := currency.Symbol(c)
							label := fmt.Sprintf("%s — %s", c, sym)
							if c == curVal {
								return Option(Value(c), Attr("selected", ""), label)
							}
							return Option(Value(c), label)
						},
					),
				),
			),
			labeledField(uistate.T("setup.weekStartLabel"),
				Select(css.Class("field"),
					Attr("aria-label", uistate.T("setup.weekStartLabel")),
					Attr("data-testid", "setup-weekstart"),
					OnChange(onWeek),
					weekOption(prefs.WeekSunday, weekVal, uistate.T("setup.weekSunday")),
					weekOption(prefs.WeekMonday, weekVal, uistate.T("setup.weekMonday")),
					weekOption(prefs.WeekSaturday, weekVal, uistate.T("setup.weekSaturday")),
				),
			),
			Div(css.Class(tw.Flex, tw.Gap2, tw.Mt2),
				Button(css.Class("btn btn-primary"), Type("submit"),
					uistate.T("setup.confirmCurrency")),
			),
		),
	})

	// Step 1: monthly income
	curSym := currency.Symbol(curVal)
	incomeVal := incomeStr.Get()
	step1 := uiw.Card(uiw.CardProps{
		Title: uistate.T("setup.step2Title"),
		Body: Form(css.Class("form-grid"), OnSubmit(confirmIncome),
			P(css.Class("t-caption", tw.TextDim), uistate.T("setup.step2Hint")),
			labeledField(fmt.Sprintf("%s (%s)", uistate.T("setup.incomeLabel"), curSym),
				Input(css.Class("field"),
					Type("number"),
					Attr("min", "0"),
					Attr("step", "any"),
					Attr("placeholder", uistate.T("setup.incomePlaceholder")),
					Attr("aria-label", uistate.T("setup.incomeLabel")),
					Attr("data-testid", "setup-income"),
					Value(incomeVal),
					OnInput(onIncomeStr),
				),
			),
			Div(css.Class(tw.Flex, tw.Gap2, tw.Mt2),
				Button(css.Class("btn btn-primary"), Type("submit"),
					uistate.T("setup.confirmIncome")),
				Button(css.Class("btn btn-ghost"), Type("submit"),
					uistate.T("setup.skipIncome")),
			),
		),
	})

	// Step 2: first account
	acctName := acctNameS.Get()
	acctType := acctTypeS.Get()
	acctBal := acctBalS.Get()
	acctErr := acctErrS.Get()
	acctCount := len(app.Accounts())
	acctDoneNote := If(acctCount > 0,
		P(css.Class("t-caption text-up"),
			fmt.Sprintf(uistate.T("setup.acctAlreadyHave"), acctCount),
		),
	)
	step2 := uiw.Card(uiw.CardProps{
		Title: uistate.T("setup.step3Title"),
		Body: Form(css.Class("form-grid"), OnSubmit(confirmAccount),
			P(css.Class("t-caption", tw.TextDim), uistate.T("setup.step3Hint")),
			acctDoneNote,
			labeledField(uistate.T("setup.acctNameLabel"),
				Input(css.Class("field"),
					Type("text"),
					Attr("placeholder", uistate.T("setup.acctNamePlaceholder")),
					Attr("aria-label", uistate.T("setup.acctNameLabel")),
					Attr("data-testid", "setup-acct-name"),
					Value(acctName),
					OnInput(onAcctName),
				),
			),
			labeledField(uistate.T("setup.acctTypeLabel"),
				Select(css.Class("field"),
					Attr("aria-label", uistate.T("setup.acctTypeLabel")),
					Attr("data-testid", "setup-acct-type"),
					OnChange(onAcctType),
					acctTypeOption(domain.TypeChecking, acctType, uistate.T("acctType.checking")),
					acctTypeOption(domain.TypeSavings, acctType, uistate.T("acctType.savings")),
					acctTypeOption(domain.TypeCash, acctType, uistate.T("acctType.cash")),
					acctTypeOption(domain.TypeCreditCard, acctType, uistate.T("acctType.credit_card")),
					acctTypeOption(domain.TypeInvestment, acctType, uistate.T("acctType.investment")),
					acctTypeOption(domain.TypeOther, acctType, uistate.T("acctType.other")),
				),
			),
			labeledField(uistate.T("setup.acctBalLabel"),
				Input(css.Class("field"),
					Type("number"),
					Attr("min", "0"),
					Attr("step", "any"),
					Attr("placeholder", uistate.T("setup.acctBalPlaceholder")),
					Attr("aria-label", uistate.T("setup.acctBalLabel")),
					Attr("data-testid", "setup-acct-bal"),
					Value(acctBal),
					OnInput(onAcctBal),
				),
			),
			If(acctErr != "", P(css.Class("err", tw.Mt1), acctErr)),
			Div(css.Class(tw.Flex, tw.Gap2, tw.Mt2),
				Button(css.Class("btn btn-primary"), Type("submit"),
					uistate.T("setup.addAccount")),
				Button(css.Class("btn btn-ghost"), Type("button"), OnClick(skipAccount),
					uistate.T("setup.skipAccount")),
			),
		),
	})

	// Step 3: household members
	memberName := memberNameS.Get()
	memberErr := memberErrS.Get()
	members := app.Members()
	membersNote := If(len(members) > 0,
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1, tw.Mt2),
			P(css.Class("t-caption", tw.TextDim),
				fmt.Sprintf(uistate.T("setup.membersAlready"), len(members)),
			),
			MapKeyed(members,
				func(m domain.Member) any { return m.ID },
				func(m domain.Member) ui.Node {
					return Span(css.Class("t-caption", tw.FontMedium), "• "+m.Name)
				},
			),
		),
	)
	step3 := uiw.Card(uiw.CardProps{
		Title: uistate.T("setup.step4Title"),
		Body: Div(css.Class("form-grid"),
			P(css.Class("t-caption", tw.TextDim), uistate.T("setup.step4Hint")),
			membersNote,
			Form(css.Class(tw.Flex, tw.Gap2, tw.Mt2), OnSubmit(addMember),
				Input(css.Class("field", tw.Flex1),
					Type("text"),
					Attr("placeholder", uistate.T("setup.memberNamePlaceholder")),
					Attr("aria-label", uistate.T("setup.memberNameLabel")),
					Attr("data-testid", "setup-member-name"),
					Value(memberName),
					OnInput(onMemberName),
				),
				Button(css.Class("btn btn-primary"), Type("submit"),
					uistate.T("setup.addMember")),
			),
			If(memberErr != "", P(css.Class("err", tw.Mt1), memberErr)),
			Div(css.Class(tw.Flex, tw.Gap2, tw.Mt3),
				Button(css.Class("btn btn-ghost"), Type("button"), OnClick(skipMembers),
					uistate.T("setup.skipMembers")),
			),
		),
	})

	// Step 4: completion
	allDone := setup.AllRequired(progress)
	doneMsg := uistate.T("setup.doneBody")
	var doneP ui.Node
	if allDone {
		doneP = P(css.Class("t-body", "text-up"), doneMsg)
	} else {
		doneP = P(css.Class("t-body", tw.TextDim), uistate.T("setup.doneBodyPartial"))
	}
	step4 := uiw.Card(uiw.CardProps{
		Title: uistate.T("setup.doneTitle"),
		Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap4),
			doneP,
			Button(css.Class("btn btn-primary"),
				Type("button"),
				Attr("data-testid", "setup-go-dashboard"),
				OnClick(goDashboard),
				uistate.T("setup.goDashboard"),
			),
		),
	})

	// ── Compose the full page ─────────────────────────────────────────────────
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap5),
		uiw.Card(uiw.CardProps{
			Title: uistate.T("setup.welcomeTitle"),
			Body:  P(css.Class("t-body", tw.TextDim), uistate.T("setup.welcomeBody")),
		}),
		progressBar,
		If(step == 0, step0),
		If(step == 1, step1),
		If(step == 2, step2),
		If(step == 3, step3),
		If(step >= 4, step4),
	)
}

// setupStepDots renders the four-step breadcrumb progress bar.
func setupStepDots(p setup.Progress, activeStep int) ui.Node {
	labels := []string{
		uistate.T("setup.step1Label"),
		uistate.T("setup.step2Label"),
		uistate.T("setup.step3Label"),
		uistate.T("setup.step4Label"),
	}
	done := []bool{p.CurrencyDone, p.IncomeDone, p.AccountDone, p.MembersDone}

	items := []any{
		css.Class(tw.Flex, tw.ItemsCenter, tw.Gap3),
		Style(map[string]string{"flex-wrap": "wrap"}),
	}
	for i, label := range labels {
		tick := "○"
		var spanNode ui.Node
		if done[i] {
			tick = "✓"
			spanNode = Span(css.Class("t-caption", "text-up"),
				Span(Style(map[string]string{"margin-right": "4px"}), tick),
				label,
			)
		} else if i == activeStep {
			spanNode = Span(css.Class("t-caption", tw.FontMedium),
				Span(Style(map[string]string{"margin-right": "4px"}), tick),
				label,
			)
		} else {
			spanNode = Span(css.Class("t-caption", tw.TextFaint),
				Span(Style(map[string]string{"margin-right": "4px"}), tick),
				label,
			)
		}
		items = append(items, spanNode)
		if i < len(labels)-1 {
			items = append(items, Span(css.Class("t-caption", tw.TextFaint), "›"))
		}
	}
	return Div(items...)
}

// weekOption renders a single <option> for the week-start selector.
func weekOption(v prefs.WeekStart, selected, label string) ui.Node {
	if string(v) == selected {
		return Option(Value(string(v)), Attr("selected", ""), label)
	}
	return Option(Value(string(v)), label)
}

// acctTypeOption renders a single <option> for the account-type selector.
func acctTypeOption(v domain.AccountType, selected, label string) ui.Node {
	if string(v) == selected {
		return Option(Value(string(v)), Attr("selected", ""), label)
	}
	return Option(Value(string(v)), label)
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/actionpreview"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/payoff"
	"github.com/monstercameron/CashFlux/internal/trust"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// isInstallmentLoan reports whether an account type is a fixed-term installment
// loan (as distinct from revolving credit handled by /credit). Includes TypeLoan,
// TypePersonalLoan, and TypeMortgage.
func isInstallmentLoan(t domain.AccountType) bool {
	switch t {
	case domain.TypeLoan, domain.TypePersonalLoan, domain.TypeMortgage:
		return true
	default:
		return false
	}
}

// defaultTermMonths returns a sensible default repayment term for a loan type.
func defaultTermMonths(t domain.AccountType) int {
	if t == domain.TypeMortgage {
		return 360 // 30-year
	}
	return 60 // 5-year for personal loans and generic loans
}

// loanCardProps is the props bag for a single per-loan amortization card.
type loanCardProps struct {
	// OnTerm persists a changed term on the account (FP-T2a). Nil leaves the term
	// session-only.
	OnTerm func(a domain.Account, months int)

	Account domain.Account
	Balance int64  // balance in minor units (positive = principal owed)
	BaseCur string // household base currency
}

// loanCard is a standalone component (one per installment loan) so that each
// card's UseState and UseEvent hooks occupy stable positions — never called
// inside the variable-length loop in LoansScreen. Each card owns:
//   - a term (months) input controlling the amortization schedule (C204)
//   - an extra monthly payment input for payoff acceleration simulation (C205)
func loanCard(props loanCardProps) ui.Node {
	a := props.Account
	balance := props.Balance
	cur := a.Currency
	if cur == "" {
		cur = props.BaseCur
	}
	dec := currency.Decimals(cur)
	sym := currency.Symbol(cur)

	// Default terms: 360 months for mortgages, 60 for other loans.
	defaultTerm := defaultTermMonths(a.Type)

	// Per-card hook state. All UseState/UseEvent calls are at unconditional,
	// stable positions because loanCard is its own component (not inlined in a
	// loop body).
	// FP-T2a: the term is seeded from the ACCOUNT when it knows one. It used to be
	// session-only state, so the payoff date and the modeled payment reset to a
	// guessed default on every reload — figures a household plans around, quietly
	// reverting to something nobody entered.
	seedTerm := strconv.Itoa(defaultTerm)
	if a.TermMonths > 0 {
		seedTerm = strconv.Itoa(a.TermMonths)
	}
	termS := ui.UseState(seedTerm)
	extraS := ui.UseState("")
	scheduleOpen := uistate.UseLoanScheduleOpen()

	onTerm := ui.UseEvent(func(v string) {
		termS.Set(v)
		// Persist only a term that parses. Half-typed input arrives on every
		// keystroke, and writing "1" on the way to "180" would store a loan that
		// pays off next month.
		if t, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && t > 0 && t <= 1200 && props.OnTerm != nil {
			props.OnTerm(a, t)
		}
	})
	toggleSchedule := ui.UseEvent(Prevent(func() {
		if scheduleOpen.Get() == a.ID {
			scheduleOpen.Set("")
			return
		}
		scheduleOpen.Set(a.ID)
	}))
	onExtra := ui.UseEvent(func(v string) { extraS.Set(v) })

	// Parse the user-editable inputs; fall back to defaults on bad input.
	term := defaultTerm
	if t, err := strconv.Atoi(termS.Get()); err == nil && t > 0 && t <= 1200 {
		term = t
	}

	extraMinor := int64(0)
	if raw := extraS.Get(); raw != "" {
		if f, err := strconv.ParseFloat(raw, 64); err == nil && f > 0 {
			// Convert major-unit input to minor units.
			mul := int64(1)
			for i := 0; i < dec; i++ {
				mul *= 10
			}
			extraMinor = int64(f * float64(mul))
		}
	}

	apr, aprKnown := a.RateAPR()
	now := time.Now()

	// --- C204: base amortization schedule ---
	baseRows := payoff.AmortizeFixed(balance, apr, term, now)
	var baseMonthlyPayment int64
	var baseTotalInterest, baseTotalPaid int64
	var basePayoffDate time.Time
	if len(baseRows) > 0 {
		baseMonthlyPayment = baseRows[0].PaymentMinor
		baseTotalInterest, baseTotalPaid, basePayoffDate = payoff.AmortSummary(baseRows)
	}

	// --- C205: accelerated schedule with extra payment ---
	var extraRows []payoff.AmortRow
	var extraTotalInterest int64
	var extraPayoffDate time.Time
	var monthsSaved int
	var interestSaved int64
	hasExtra := extraMinor > 0 && len(baseRows) > 0
	if hasExtra {
		extraRows = payoff.AmortizeWithExtra(balance, apr, term, extraMinor, now)
		var extraTotalPaid int64
		extraTotalInterest, extraTotalPaid, extraPayoffDate = payoff.AmortSummary(extraRows)
		_ = extraTotalPaid
		monthsSaved = len(baseRows) - len(extraRows)
		if monthsSaved < 0 {
			monthsSaved = 0
		}
		interestSaved = baseTotalInterest - extraTotalInterest
		if interestSaved < 0 {
			interestSaved = 0
		}
	}

	// Format helpers.
	fmtMoney := func(minor int64) string {
		return sym + fmtMinorAmount(minor, dec)
	}
	fmtMonthYear := func(t time.Time) string {
		if t.IsZero() {
			return "—"
		}
		return fmt.Sprintf("%s %d", t.Month().String()[:3], t.Year())
	}

	// Loan type badge text.
	var typeBadge string
	switch a.Type {
	case domain.TypeMortgage:
		typeBadge = uistate.T("loans.typeMortgage")
	case domain.TypePersonalLoan:
		typeBadge = uistate.T("loans.typePersonalLoan")
	default:
		typeBadge = uistate.T("loans.typeLoan")
	}

	// APR display — self-contained (includes the "APR" word) so the "0% APR" no-rate
	// label doesn't get a second "APR" appended below.
	// "No APR recorded" and "0.00% APR" are different statements, and only the
	// first is a gap (WF4-b). A family loan at no interest is a real loan with a
	// real payoff date; saying its rate is missing sends somebody looking for a
	// number that does not exist.
	aprLabel := fmt.Sprintf("%.2f%% APR", apr)
	if !aprKnown {
		aprLabel = uistate.T("loans.noApr")
	}

	// --- Render ---

	// Header: loan name + type badge + balance + APR.
	header := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Mb3),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
			Div(ClassStr("t-body "+tw.Fold(tw.FontMedium)), a.Name),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
				Span(css.Class("badge", "t-caption"), typeBadge),
				Span(css.Class("t-caption", tw.TextDim), aprLabel),
			),
		),
		Div(ClassStr("t-figure "+tw.Fold(tw.FontDisplay)+" text-down"), fmtMoney(balance)),
	)

	// Term input row.
	termRow := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap3, tw.Mb3),
		Label(css.Class("t-caption", tw.TextDim),
			Attr("for", "loan-term-"+a.ID),
			Style(map[string]string{"white-space": "nowrap"}),
			uistate.T("loans.termLabel")),
		Input(css.Class("field"),
			Attr("id", "loan-term-"+a.ID),
			Type("number"), Attr("min", "1"), Attr("max", "1200"),
			Style(map[string]string{"width": "6rem"}),
			Placeholder(uistate.T("loans.termPlaceholder")),
			OnInput(onTerm),
			Attr("aria-label", uistate.T("debt.loanTermAria", a.Name)), uiw.FieldValue(termS.Get())),
		Span(css.Class("t-caption", tw.TextDim), uistate.T("loans.termMonthsSuffix")),
	)

	// Summary tiles: monthly payment, total interest, total paid, payoff date.
	var summaryNode ui.Node = Fragment()
	if len(baseRows) > 0 {
		summaryNode = Div(css.Class(tw.Grid, tw.GridCols2, tw.Gap3, tw.Mb3),
			Div(css.Class("stat"),
				// This is a MODELED payment (from the balance, APR, and the term set
				// above) — not necessarily the contractual minimum. Label it as such and
				// show the recorded minimum beside it so the two aren't confused (review).
				Div(css.Class("stat-label"), uistate.T("debt.loanModeledPayment")),
				Div(ClassStr("stat-value text-up"), fmtMoney(baseMonthlyPayment)),
				If(a.MinPayment.Amount > 0 && a.MinPayment.Amount != baseMonthlyPayment,
					Div(css.Class("t-caption", tw.TextDim), uistate.T("debt.loanRecordedMin", fmtMoney(a.MinPayment.Amount)))),
			),
			Div(css.Class("stat"),
				Div(css.Class("stat-label"), uistate.T("loans.totalInterest")),
				Div(ClassStr("stat-value text-down"), fmtMoney(baseTotalInterest)),
			),
			Div(css.Class("stat"),
				Div(css.Class("stat-label"), uistate.T("loans.totalPaid")),
				Div(css.Class("stat-value"), fmtMoney(baseTotalPaid)),
			),
			Div(css.Class("stat"),
				Div(css.Class("stat-label"), uistate.T("loans.payoffDate")),
				Div(css.Class("stat-value"), fmtMonthYear(basePayoffDate)),
			),
		)
	} else {
		summaryNode = P(css.Class("t-caption", tw.TextDim), uistate.T("loans.noSchedule"))
	}

	// WF4: say how far these figures can be trusted, and exactly why. A payoff
	// date computed from a real APR and one computed from a blank APR render
	// identically, and the second is a guess wearing the first one's clothes.
	//
	// A reason, never a bare score: "62% confident" cannot be acted on, argued
	// with, or improved. Naming the field makes the next step obvious.
	assessment := trust.Assess([]trust.Input{
		{Name: uistate.T("trust.inLoanApr"), Required: true, Missing: !aprKnown},
		{Name: uistate.T("trust.inLoanTerm"), Required: true, Missing: term <= 0,
			Assumed: a.TermMonths <= 0},
		{Name: uistate.T("trust.inLoanBalance"), Required: true,
			Missing: balance <= 0, AgeDays: accountAgeDays(a, now)},
	})
	var trustNode ui.Node = Fragment()
	if assessment.Level != trust.LevelSolid {
		key := "trust.qualified"
		tone := tw.TextDim
		if assessment.Level == trust.LevelUnreliable {
			key, tone = "trust.unreliable", tw.TextWarn
		}
		trustNode = P(css.Class("t-caption", tone), Attr("data-testid", "loan-trust-"+a.ID),
			uistate.T(key, strings.Join(assessment.Reasons(), ", ")))
	}

	// Extra-payment simulation section (C205).
	extraRow := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap3, tw.Mt3, tw.Mb2),
		Label(css.Class("t-caption", tw.TextDim),
			Attr("for", "loan-extra-"+a.ID),
			Style(map[string]string{"white-space": "nowrap"}),
			uistate.T("loans.extraLabel")),
		Input(css.Class("field"),
			Attr("id", "loan-extra-"+a.ID),
			Type("number"), Attr("min", "0"), Attr("step", "any"),
			Style(map[string]string{"width": "9.5rem"}),
			Placeholder(fmt.Sprintf(uistate.T("loans.extraPlaceholder"), sym)),
			OnInput(onExtra),
			Attr("aria-label", uistate.T("debt.loanExtraAria", a.Name)), uiw.FieldValue(extraS.Get())),
		Span(css.Class("t-caption", tw.TextDim), uistate.T("loans.extraPerMonth")),
	)

	// WF6: the extra payment as a before-and-after across every figure it moves,
	// including the ones it does not. The savings tiles below say what improves;
	// this says what the action DOES, costs included, which is the difference
	// between a decision aid and an advertisement.
	var previewNode ui.Node = Fragment()
	if hasExtra && len(extraRows) > 0 {
		pv := actionpreview.Build([]actionpreview.Metric{
			{Name: uistate.T("preview.mInterest"), Before: baseTotalInterest,
				After: extraTotalInterest, Goodness: actionpreview.LowerBetter,
				DisplayBefore: fmtMoney(baseTotalInterest), DisplayAfter: fmtMoney(extraTotalInterest)},
			{Name: uistate.T("preview.mMonths"), Before: int64(len(baseRows)),
				After: int64(len(extraRows)), Goodness: actionpreview.LowerBetter},
			// The cost. A preview that omits what leaves the account each month is
			// the advertisement, not the decision aid.
			{Name: uistate.T("preview.mMonthly"), Before: baseMonthlyPayment,
				After: baseMonthlyPayment + extraMinor, Goodness: actionpreview.LowerBetter,
				DisplayBefore: fmtMoney(baseMonthlyPayment),
				DisplayAfter: fmtMoney(baseMonthlyPayment + extraMinor)},
			// Stated as unchanged rather than omitted: its absence would be
			// indistinguishable from nobody having checked.
			{Name: uistate.T("preview.mBalance"), Before: balance, After: balance,
				Goodness: actionpreview.LowerBetter},
		})
		rows := make([]ui.Node, 0, len(pv.Changed))
		for _, c := range pv.Changed {
			before, after := c.Metric.DisplayBefore, c.Metric.DisplayAfter
			if before == "" {
				before, after = strconv.FormatInt(c.Metric.Before, 10), strconv.FormatInt(c.Metric.After, 10)
			}
			tone := tw.TextDim
			switch c.Direction {
			case actionpreview.DirectionWorse:
				tone = tw.TextWarn
			case actionpreview.DirectionBetter:
				tone = tw.TextUp
			}
			rows = append(rows, P(css.Class("t-caption", tone),
				Attr("data-testid", "loan-preview-row-"+a.ID),
				uistate.T("preview.row", c.Metric.Name, before, after)))
		}
		if len(pv.Unchanged) > 0 {
			rows = append(rows, P(css.Class("t-caption", tw.TextFaint),
				Attr("data-testid", "loan-preview-unchanged-"+a.ID),
				uistate.T("preview.unchanged", strings.Join(pv.Unchanged, ", "))))
		}
		previewNode = Div(css.Class("card-inset", tw.Mt2), Attr("data-testid", "loan-preview-"+a.ID),
			Div(css.Class("t-caption", tw.TextDim), uistate.T("preview.title")),
			Fragment(rows))
	}

	var savingsNode ui.Node = Fragment()
	if hasExtra && len(extraRows) > 0 {
		savingsNode = Div(css.Class("card-inset", tw.Flex, tw.FlexCol, tw.Gap2, tw.Mt2),
			Div(css.Class("t-caption", tw.TextDim), uistate.T("loans.savingsTitle")),
			Div(css.Class(tw.Grid, tw.GridCols2, tw.Gap3),
				Div(css.Class("stat"),
					Div(css.Class("stat-label"), uistate.T("loans.monthsSaved")),
					Div(ClassStr("stat-value text-up"),
						fmt.Sprintf("%d", monthsSaved)),
				),
				Div(css.Class("stat"),
					Div(css.Class("stat-label"), uistate.T("loans.interestSaved")),
					Div(ClassStr("stat-value text-up"),
						fmtMoney(interestSaved)),
				),
				Div(css.Class("stat"),
					Div(css.Class("stat-label"), uistate.T("loans.newPayoffDate")),
					Div(css.Class("stat-value"), fmtMonthYear(extraPayoffDate)),
				),
				Div(css.Class("stat"),
					Div(css.Class("stat-label"), uistate.T("loans.paymentsLeft")),
					Div(css.Class("stat-value"),
						fmt.Sprintf("%d", len(extraRows))),
				),
			),
		)
	}

	// --- FP-T2a: the schedule itself ---
	//
	// The summary tiles say what the loan costs; the schedule says WHERE the money
	// goes, which is the thing that surprises people. On a 30-year mortgage the
	// first payment is most interest and almost no principal, and no amount of
	// "total interest $X" conveys that as well as one row of it does.
	//
	// Rendered only while expanded, and only one loan at a time: 360 rows is a
	// real cost to pay for a panel nobody is reading.
	var scheduleNode ui.Node = Fragment()
	if len(baseRows) > 0 {
		rows := baseRows
		note := uistate.T("loans.scheduleNoteBase")
		if hasExtra && len(extraRows) > 0 {
			// When an extra payment is set, show the schedule the reader is actually
			// asking about — the accelerated one. Showing the base schedule beneath an
			// "18 months saved" figure would contradict it row by row.
			rows = extraRows
			note = uistate.T("loans.scheduleNoteExtra")
		}
		open := scheduleOpen.Get() == a.ID
		label := uistate.T("loans.scheduleShow", len(rows))
		if open {
			label = uistate.T("loans.scheduleHide")
		}
		var table ui.Node = Fragment()
		if open {
			table = Div(css.Class("loan-sched-wrap"),
				P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "loan-sched-note-"+a.ID), note),
				Table(css.Class("loan-sched"), Attr("data-testid", "loan-sched-"+a.ID),
					Thead(Tr(
						Th(uistate.T("loans.colNo")),
						Th(uistate.T("loans.colDate")),
						Th(uistate.T("loans.colPayment")),
						Th(uistate.T("loans.colPrincipal")),
						Th(uistate.T("loans.colInterest")),
						Th(uistate.T("loans.colBalance")),
					)),
					Tbody(MapKeyed(rows,
						func(r payoff.AmortRow) any { return r.PaymentNo },
						func(r payoff.AmortRow) ui.Node {
							return Tr(
								Td(strconv.Itoa(r.PaymentNo)),
								Td(fmtMonthYear(r.Date)),
								Td(fmtMoney(r.PaymentMinor)),
								Td(fmtMoney(r.PrincipalMinor)),
								Td(fmtMoney(r.InterestMinor)),
								Td(fmtMoney(r.BalanceMinor)),
							)
						})),
				))
		}
		scheduleNode = Div(css.Class(tw.Mt3),
			Button(css.Class("btn", "btn-quiet"), Type("button"),
				Attr("data-testid", "loan-sched-toggle-"+a.ID),
				Attr("aria-expanded", boolAttr(open)),
				OnClick(toggleSchedule), label),
			table)
	}

	return uiw.Card(uiw.CardProps{
		Body: Div(css.Class(tw.FlexCol),
			header,
			termRow,
			summaryNode,
			trustNode,
			extraRow,
			previewNode,
			savingsNode,
			scheduleNode,
		),
	})
}

// LoansPanelProps configures LoansPanel. No external props are required;
// the panel reads appstate.Default directly.
type LoansPanelProps struct{}

// LoansPanel renders a per-account amortization summary for each installment
// loan (TypeLoan / TypePersonalLoan / TypeMortgage) with an extra-payment
// simulation (C204 + C205, F27) as a registered component. It owns its
// UseDataRevision hook so it can be embedded at two call sites (/loans and
// /debt) without duplicating state or violating GWC hook rules.
//
// Each loan card is its own component so hooks stay at stable, unconditional
// positions — never inside a variable-length loop.
func LoansPanel(props LoansPanelProps) ui.Node {
	// Hook declared unconditionally before any conditional return (GWC rule).
	_ = uistate.UseDataRevision().Get()

	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	// FP-T2a: persist a changed term on the account. The schedule, the payoff date
	// and the modeled payment are all derived from it, and a figure that resets to
	// a guessed default on reload is worse than no figure — it looks entered.
	onTerm := func(a domain.Account, months int) {
		if a.TermMonths == months {
			return
		}
		a.TermMonths = months
		if err := app.PutAccount(a); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.RequestPersist()
	}

	settings := app.Settings()
	baseCur := settings.BaseCurrency
	if baseCur == "" {
		baseCur = "USD"
	}

	accounts := app.Accounts()
	txns := app.Transactions()

	// Filter to active installment-loan accounts.
	var loans []domain.Account
	for _, a := range accounts {
		if a.Archived || !isInstallmentLoan(a.Type) {
			continue
		}
		loans = append(loans, a)
	}

	if len(loans) == 0 {
		return uiw.Card(uiw.CardProps{
			Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap3),
				P(ClassStr("t-body "+tw.Fold(tw.FontMedium)), uistate.T("loans.emptyTitle")),
				P(css.Class("t-caption", tw.TextDim), uistate.T("loans.emptyBody")),
			),
		})
	}

	// Build a card per loan. Each card is created via ui.CreateElement so its
	// hook slots are stable — never inline-expanded in the loop.
	cards := make([]any, 0, len(loans))
	for _, a := range loans {
		bal, err := ledger.Balance(a, txns)
		var balMinor int64
		if err == nil {
			balMinor = bal.Amount
			// Liabilities carry negative balances in the ledger; amortization
			// takes a positive principal.
			if balMinor < 0 {
				balMinor = -balMinor
			}
		}
		p := loanCardProps{
			Account: a,
			Balance: balMinor,
			BaseCur: baseCur,
			OnTerm:  onTerm,
		}
		cards = append(cards, ui.CreateElement(loanCard, p))
	}

	return Div(append([]any{css.Class(tw.Flex, tw.FlexCol, tw.Gap5)}, cards...)...)
}

// LoansScreen is the /loans route — a thin shell rendering LoansPanel.
// The panel owns all hooks and state so it can also be embedded in /debt.
func LoansScreen() ui.Node {
	return ui.CreateElement(LoansPanel, LoansPanelProps{})
}

// accountAgeDays is how long since the account's balance was stated, or zero
// when that is unknown.
//
// Unknown reports zero rather than a large number: an account nobody has ever
// reconciled is a different problem from one reconciled last year, and inventing
// an age would report the wrong one.
func accountAgeDays(a domain.Account, now time.Time) int {
	if a.BalanceAsOf.IsZero() {
		return 0
	}
	d := int(now.Sub(a.BalanceAsOf).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

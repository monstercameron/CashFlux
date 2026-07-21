// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/payoff"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
	termS := ui.UseState(strconv.Itoa(defaultTerm))
	extraS := ui.UseState("")

	onTerm := ui.UseEvent(func(v string) { termS.Set(v) })
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

	apr := a.InterestRateAPR
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
	aprLabel := fmt.Sprintf("%.2f%% APR", apr)
	if apr == 0 {
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
			Value(termS.Get()),
			OnInput(onTerm),
			Attr("aria-label", uistate.T("loans.termLabel")),
		),
		Span(css.Class("t-caption", tw.TextDim), uistate.T("loans.termMonthsSuffix")),
	)

	// Summary tiles: monthly payment, total interest, total paid, payoff date.
	var summaryNode ui.Node = Fragment()
	if len(baseRows) > 0 {
		summaryNode = Div(css.Class(tw.Grid, tw.GridCols2, tw.Gap3, tw.Mb3),
			Div(css.Class("stat"),
				Div(css.Class("stat-label"), uistate.T("loans.monthlyPayment")),
				Div(ClassStr("stat-value text-up"), fmtMoney(baseMonthlyPayment)),
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
			Value(extraS.Get()),
			OnInput(onExtra),
			Attr("aria-label", uistate.T("loans.extraLabel")),
		),
		Span(css.Class("t-caption", tw.TextDim), uistate.T("loans.extraPerMonth")),
	)

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

	return uiw.Card(uiw.CardProps{
		Body: Div(css.Class(tw.FlexCol),
			header,
			termRow,
			summaryNode,
			extraRow,
			savingsNode,
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

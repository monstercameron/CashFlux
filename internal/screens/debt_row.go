// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// debtRowProps carries one liability's render data + the plain callbacks the list tile
// wires. The row owns its own On* hooks (wrapping the plain funcs), so the parent can
// MapKeyed over a variable-length debt slice without calling hooks in a loop.
type debtRowProps struct {
	Account     domain.Account
	Owed        money.Money        // absolute amount owed, in the account's currency
	Rank        int                // 1-based position in the payoff order; 0 = not in the plan
	Utilization float64            // credit utilization %, or -1 when N/A (not a line of credit)
	Available   money.Money        // remaining credit (credit lines only)
	Band        string             // "good" | "warn" | "high" — from DebtConfig.UtilizationBand
	InPayoff    bool               // whether this debt is included in the payoff plan
	Defs        []customfields.Def // account custom-field definitions (for the value summary)
	OnEdit      func(string)       // open the account editor
	OnView      func(string)       // drill to this account's transactions
	OnTogglePay func(domain.Account, bool)
	// BillPayment is this account's actual recurring payment, read from the most
	// recent transaction the user marked as a bill payment toward it (distinct from
	// the minimum). OnViewBills drills to those linked payments.
	BillPayment ledger.BillPaymentInfo
	OnViewBills func(string)
}

// DebtRow renders one liability as a card in the payoff ladder: a payoff-rank medallion
// and an APR/utilization-banded left rail (hotter = worse), the debt's name + type +
// lender, an APR chip, a utilization meter for revolving credit (banded by the config, not
// a hardcoded cutoff), the minimum-payment / due-day meta, any custom-field values, and the
// amount owed on the right in the display serif. Actions: view transactions, edit, and an
// include-in-payoff toggle.
func DebtRow(props debtRowProps) ui.Node {
	a := props.Account
	band := props.Band
	if band == "" {
		band = "good"
	}

	view := ui.UseEvent(Prevent(func() {
		if props.OnView != nil {
			props.OnView(a.ID)
		}
	}))
	edit := ui.UseEvent(Prevent(func() {
		if props.OnEdit != nil {
			props.OnEdit(a.ID)
		}
	}))
	togglePay := ui.UseEvent(Prevent(func() {
		if props.OnTogglePay != nil {
			props.OnTogglePay(a, !props.InPayoff)
		}
	}))
	viewBills := ui.UseEvent(Prevent(func() {
		if props.OnViewBills != nil {
			props.OnViewBills(a.ID)
		}
	}))

	cardCls := "debt-card debt-band-" + band
	if !props.InPayoff {
		cardCls += " is-excluded"
	}
	// The #1 debt in the payoff order is the one to attack first — give the whole card a
	// focus treatment so the order reads at a glance.
	focus := props.InPayoff && props.Rank == 1
	if focus {
		cardCls += " is-focus"
	}

	// Payoff-rank medallion — the ladder position (avalanche/snowball order). Excluded
	// debts show a dash instead of a number so the ladder reads only the debts in the plan.
	rankText := "—"
	if props.InPayoff && props.Rank > 0 {
		rankText = strconv.Itoa(props.Rank)
	}

	// APR chip.
	var aprChip ui.Node = Fragment()
	if a.InterestRateAPR > 0 {
		aprChip = Span(css.Class("debt-chip debt-apr"), fmt.Sprintf("%.2f%% APR", a.InterestRateAPR))
	}

	// Lender / institution note.
	lender := strings.TrimSpace(a.Lender)
	if lender == "" {
		lender = strings.TrimSpace(a.Institution)
	}

	// Utilization meter (revolving credit only). Fill width + band color come from the
	// config-classified band, so the "how full is this card" read has no baked-in cutoff.
	var utilMeter ui.Node = Fragment()
	if props.Utilization >= 0 {
		w := props.Utilization
		if w > 100 {
			w = 100
		}
		utilMeter = Div(css.Class("debt-util"),
			Div(css.Class("debt-util-track"),
				Attr("role", "progressbar"), Attr("aria-valuenow", fmt.Sprintf("%.0f", props.Utilization)),
				Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"),
				Attr("aria-label", uistate.T("debt.utilizationAria", a.Name)),
				Div(ClassStr("debt-util-fill debt-util-"+band), Attr("style", fmt.Sprintf("width:%.0f%%", w))),
			),
			Span(css.Class("debt-util-label", tw.TextDim),
				uistate.T("debt.utilizationLabel", fmt.Sprintf("%.0f", props.Utilization), fmtMoney(props.Available))),
		)
	}

	// Meta line: minimum payment + due day, middot-separated.
	var metaParts []string
	if a.MinPayment.Amount > 0 {
		metaParts = append(metaParts, uistate.T("debt.minPaymentMeta", fmtMoney(a.MinPayment)))
	}
	if a.DueDayOfMonth > 0 {
		metaParts = append(metaParts, uistate.T("debt.dueDayMeta", ordinalDay(a.DueDayOfMonth)))
	}
	// AC4: carrying cost — the monthly interest to hold this debt (owed × APR/100 ÷ 12),
	// a concrete dollar figure that competes with the discretionary spend it finances.
	if a.InterestRateAPR > 0 && props.Owed.Amount > 0 {
		carryMinor := int64(float64(props.Owed.Amount) * a.InterestRateAPR / 100 / 12)
		if carryMinor > 0 {
			metaParts = append(metaParts, uistate.T("accountsstmt.carryingCost", fmtMoney(money.New(carryMinor, props.Owed.Currency))))
		}
	}
	var metaNode ui.Node = Fragment()
	if len(metaParts) > 0 {
		metaNode = Span(css.Class("debt-meta", tw.TextDim), strings.Join(metaParts, " · "))
	}

	// Bill-payment line: the account's actual recurring payment (from the most
	// recent linked transaction), shown distinct from the minimum, with a link to
	// the payments that prove it.
	var billNode ui.Node = Fragment()
	if props.BillPayment.HasAny {
		billNode = Span(css.Class("debt-meta"), Attr("data-testid", "debt-bill-"+a.ID),
			uistate.T("debt.billPaymentMeta", fmtMoney(props.BillPayment.Latest)),
			Button(css.Class("btn-link", tw.Ml1), Type("button"), Attr("data-testid", "debt-bill-link-"+a.ID),
				Title(uistate.T("debt.billPaymentLinkTitle")), OnClick(viewBills),
				uistate.T("debt.billPaymentCount", plural(props.BillPayment.Count, "payment"))))
	}

	// Custom-field values (reuses the shared account custom-field summary).
	var customNode ui.Node = Fragment()
	if s := customSummary(props.Defs, a.Custom); s != "" {
		customNode = Span(css.Class("debt-meta", tw.TextDim), Attr("data-testid", "debt-custom-"+a.ID), s)
	}

	payoffLabel := uistate.T("debt.excludeFromPlan")
	if !props.InPayoff {
		payoffLabel = uistate.T("debt.includeInPlan")
	}

	return Div(ClassStr(cardCls), Attr("data-testid", "debt-card-"+a.ID), Attr("role", "listitem"),
		Div(css.Class("debt-rail"), Attr("aria-hidden", "true")),
		Div(css.Class("debt-rank"), Attr("aria-hidden", "true"), rankText),
		Div(css.Class("debt-body"),
			Div(css.Class("debt-head"),
				Span(css.Class("acct-type-icon", tw.TextDim), Attr("aria-hidden", "true"),
					uiw.Icon(accountTypeIcon(a.Type), css.Class(tw.ShrinkO, tw.W4, tw.H4))),
				Span(css.Class("debt-name"), a.Name),
				If(focus, Span(css.Class("debt-focus-tag"), uistate.T("debt.payFirst"))),
				Span(css.Class("debt-chip debt-type"), uistate.T("acctType."+string(a.Type))),
				aprChip,
				If(lender != "", Span(css.Class("debt-meta", tw.TextDim), lender)),
			),
			utilMeter,
			If(len(metaParts) > 0, metaNode),
			billNode,
			customNode,
		),
		Div(css.Class("debt-side"),
			Span(css.Class("debt-owed", tw.FontDisplay), Attr("aria-label", uistate.T("debt.owedAria", a.Name)), fmtMoney(props.Owed)),
			Div(css.Class("debt-actions"),
				Button(css.Class("btn btn-sm", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
					Attr("data-testid", "debt-view-"+a.ID), Title(uistate.T("accounts.viewTitle")), OnClick(view),
					uiw.Icon(icon.List, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("nav.transactions"))),
				Button(css.Class("btn btn-sm", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
					Attr("data-testid", "debt-edit-"+a.ID), Title(uistate.T("accounts.editTitle")), OnClick(edit),
					uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
				Button(css.Class("btn btn-sm btn-ghost"), Type("button"), Attr("aria-pressed", ariaBool(props.InPayoff)),
					Attr("data-testid", "debt-payoff-toggle-"+a.ID), Title(payoffLabel), OnClick(togglePay), Text(payoffLabel)),
			),
		),
	)
}

// ordinalDay renders a day-of-month with its English ordinal suffix (1st, 2nd, 3rd, 4th…).
func ordinalDay(d int) string {
	suffix := "th"
	if d < 11 || d > 13 {
		switch d % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(d) + suffix
}

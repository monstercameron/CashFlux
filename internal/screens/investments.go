// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/portfolio"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// isInvestmentAccount reports whether an account type holds investment positions
// (brokerage, retirement, or crypto). These are the accounts shown on
// /investments.
func isInvestmentAccount(t domain.AccountType) bool {
	switch t {
	case domain.TypeInvestment, domain.TypeRetirement, domain.TypeCrypto:
		return true
	default:
		return false
	}
}

// investmentAccountTypeBadge returns a short badge label for the account type.
func investmentAccountTypeBadge(t domain.AccountType) string {
	switch t {
	case domain.TypeRetirement:
		return uistate.T("investments.typeRetirement")
	case domain.TypeCrypto:
		return uistate.T("investments.typeCrypto")
	default:
		return uistate.T("investments.typeInvestment")
	}
}

// fmtShares formats a float64 shares value trimming unnecessary trailing zeros
// up to 6 decimal places.
func fmtShares(v float64) string {
	s := fmt.Sprintf("%.6f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// fmtSignedMoney formats a signed minor-unit amount into a display string with
// symbol. Negative amounts get a "−" prefix.
func fmtSignedMoney(minor int64, sym string, dec int) string {
	abs := minor
	if abs < 0 {
		abs = -abs
	}
	s := sym + fmtMinorAmount(abs, dec)
	if minor < 0 {
		return "−" + s
	}
	return s
}

// gainToneClass returns the semantic tone class for a gain/loss amount.
func gainToneClass(gainMinor int64) string {
	if gainMinor < 0 {
		return "text-down"
	}
	return "text-up"
}

// --------------------------------------------------------------------------
// holdingRowProps is the props bag for a single interactive holding row.
// Each row is its own component so hooks occupy stable positions — never inside
// the variable-length holdings loop.
// --------------------------------------------------------------------------

type holdingRowProps struct {
	H     domain.Holding
	Sym   string // currency symbol
	Dec   int    // currency decimal places
	OnDel func() // callback when user confirms delete
}

// holdingRow renders one holding: name/ticker, shares, current value, cost basis,
// and gain/loss. Delete is inline; no inline edit (the add form is the edit path).
func holdingRow(props holdingRowProps) ui.Node {
	h := props.H
	ph := portfolio.FromDomain(h)

	valueMinor := portfolio.HoldingValueMinor(ph)
	gainMinor := portfolio.UnrealizedGainMinor(ph)
	retPct := portfolio.ReturnPct(ph)
	tone := gainToneClass(gainMinor)

	label := h.Ticker
	if label == "" {
		label = h.Name
	}

	delHandler := ui.UseEvent(func(_ string) {
		if props.OnDel != nil {
			props.OnDel()
		}
	})

	nameMeta := h.Name
	if h.Ticker != "" && h.Ticker != h.Name {
		nameMeta = h.Name + " (" + h.Ticker + ")"
	}

	assetTag := If(h.AssetClass != "",
		Span(css.Class("t-caption", tw.TextFaint), " · "+h.AssetClass),
	)

	nameCol := Div(css.Class(tw.FlexCol, tw.Gap1),
		Div(ClassStr("t-body "+tw.Fold(tw.FontMedium)), nameMeta),
		Div(css.Class("t-caption", tw.TextDim),
			label, assetTag),
	)

	sharesStr := fmtShares(h.Shares) + " " + uistate.T("investments.shares")
	costStr := uistate.T("investments.costLabel") + " " + fmtSignedMoney(h.CostBasisMinor, props.Sym, props.Dec)

	valueBlock := Div(css.Class(tw.FlexCol, tw.ItemsEnd, tw.Gap1),
		Div(ClassStr("t-body "+tw.Fold(tw.FontMedium)),
			fmtSignedMoney(valueMinor, props.Sym, props.Dec)),
		Div(css.Class("t-caption", tw.TextDim),
			sharesStr+" · "+costStr),
	)

	gainBlock := Div(css.Class(tw.FlexCol, tw.ItemsEnd),
		Div(ClassStr("t-body "+tw.ColorClass(tone)),
			fmtSignedMoney(gainMinor, props.Sym, props.Dec)),
		Div(ClassStr("t-caption "+tw.ColorClass(tone)),
			fmt.Sprintf("%.2f%%", retPct)),
	)

	delBtn := Button(
		css.Class("btn-ghost", "t-caption", tw.TextDim),
		Attr("aria-label", fmt.Sprintf(uistate.T("investments.deleteHoldingAria"), h.Name)),
		OnClick(delHandler),
		uistate.T("investments.deleteHolding"),
	)

	return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap4, tw.Py2),
		Style(map[string]string{"border-bottom": "1px solid var(--line)"}),
		Div(css.Class(tw.Flex1), nameCol),
		valueBlock,
		gainBlock,
		delBtn,
	)
}

// --------------------------------------------------------------------------
// addHoldingFormProps is the props for the "Add holding" form, one per account.
// --------------------------------------------------------------------------

type addHoldingFormProps struct {
	AccountID string
	Sym       string
	Dec       int
}

// addHoldingForm renders the form for adding a new holding to one account.
// All UseState/UseEvent calls are at top level of this component for stable hooks.
func addHoldingForm(props addHoldingFormProps) ui.Node {
	tickerS := ui.UseState("")
	nameS := ui.UseState("")
	sharesS := ui.UseState("")
	costS := ui.UseState("")
	priceS := ui.UseState("")
	classS := ui.UseState("")
	errS := ui.UseState("")

	onTicker := ui.UseEvent(func(v string) { tickerS.Set(v) })
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onShares := ui.UseEvent(func(v string) { sharesS.Set(v) })
	onCost := ui.UseEvent(func(v string) { costS.Set(v) })
	onPrice := ui.UseEvent(func(v string) { priceS.Set(v) })
	onClass := ui.UseEvent(func(v string) { classS.Set(v) })

	onSave := ui.UseEvent(func(_ string) {
		app := appstate.Default
		if app == nil {
			return
		}
		name := strings.TrimSpace(nameS.Get())
		if name == "" {
			errS.Set(uistate.T("investments.nameRequired"))
			return
		}
		sharesF, err := strconv.ParseFloat(strings.TrimSpace(sharesS.Get()), 64)
		if err != nil || sharesF <= 0 {
			errS.Set(uistate.T("investments.sharesRequired"))
			return
		}

		// Convert major-unit inputs to minor units.
		mul := int64(1)
		for i := 0; i < props.Dec; i++ {
			mul *= 10
		}
		costF, err2 := strconv.ParseFloat(strings.TrimSpace(costS.Get()), 64)
		if err2 != nil || costF < 0 {
			errS.Set(uistate.T("investments.costRequired"))
			return
		}
		priceF, err3 := strconv.ParseFloat(strings.TrimSpace(priceS.Get()), 64)
		if err3 != nil || priceF < 0 {
			errS.Set(uistate.T("investments.priceRequired"))
			return
		}

		h := domain.Holding{
			ID:                        id.NewWithPrefix("hld"),
			AccountID:                 props.AccountID,
			Ticker:                    strings.TrimSpace(tickerS.Get()),
			Name:                      name,
			Shares:                    sharesF,
			CostBasisMinor:            int64(costF * float64(mul)),
			CurrentPriceMinorPerShare: int64(priceF * float64(mul)),
			AssetClass:                strings.TrimSpace(classS.Get()),
		}
		if saveErr := app.PutHolding(h); saveErr != nil {
			errS.Set(saveErr.Error())
			return
		}
		// Reset form.
		tickerS.Set("")
		nameS.Set("")
		sharesS.Set("")
		costS.Set("")
		priceS.Set("")
		classS.Set("")
		errS.Set("")
	})

	sym := props.Sym
	aid := props.AccountID

	fields := Div(css.Class(tw.Grid, tw.GridCols2, tw.Gap3, tw.Mt3),
		Div(css.Class(tw.FlexCol, tw.Gap1),
			Label(css.Class("t-caption", tw.TextDim),
				Attr("for", "hld-ticker-"+aid),
				uistate.T("investments.tickerLabel")),
			Input(css.Class("field"),
				Attr("id", "hld-ticker-"+aid),
				Type("text"),
				Placeholder(uistate.T("investments.tickerPlaceholder")),
				Value(tickerS.Get()), OnInput(onTicker),
				Attr("aria-label", uistate.T("investments.tickerLabel"))),
		),
		Div(css.Class(tw.FlexCol, tw.Gap1),
			Label(css.Class("t-caption", tw.TextDim),
				Attr("for", "hld-name-"+aid),
				uistate.T("investments.nameLabel")),
			Input(css.Class("field"),
				Attr("id", "hld-name-"+aid),
				Type("text"),
				Placeholder(uistate.T("investments.namePlaceholder")),
				Value(nameS.Get()), OnInput(onName),
				Attr("aria-label", uistate.T("investments.nameLabel"))),
		),
		Div(css.Class(tw.FlexCol, tw.Gap1),
			Label(css.Class("t-caption", tw.TextDim),
				Attr("for", "hld-shares-"+aid),
				uistate.T("investments.sharesLabel")),
			Input(css.Class("field"),
				Attr("id", "hld-shares-"+aid),
				Type("number"), Attr("min", "0"), Attr("step", "any"),
				Placeholder(uistate.T("investments.sharesPlaceholder")),
				Value(sharesS.Get()), OnInput(onShares),
				Attr("aria-label", uistate.T("investments.sharesLabel"))),
		),
		Div(css.Class(tw.FlexCol, tw.Gap1),
			Label(css.Class("t-caption", tw.TextDim),
				Attr("for", "hld-cost-"+aid),
				fmt.Sprintf(uistate.T("investments.costBasisLabel"), sym)),
			Input(css.Class("field"),
				Attr("id", "hld-cost-"+aid),
				Type("number"), Attr("min", "0"), Attr("step", "any"),
				Placeholder(fmt.Sprintf(uistate.T("investments.costBasisPlaceholder"), sym)),
				Value(costS.Get()), OnInput(onCost),
				Attr("aria-label", fmt.Sprintf(uistate.T("investments.costBasisLabel"), sym))),
		),
		Div(css.Class(tw.FlexCol, tw.Gap1),
			Label(css.Class("t-caption", tw.TextDim),
				Attr("for", "hld-price-"+aid),
				fmt.Sprintf(uistate.T("investments.priceLabel"), sym)),
			Input(css.Class("field"),
				Attr("id", "hld-price-"+aid),
				Type("number"), Attr("min", "0"), Attr("step", "any"),
				Placeholder(fmt.Sprintf(uistate.T("investments.pricePlaceholder"), sym)),
				Value(priceS.Get()), OnInput(onPrice),
				Attr("aria-label", fmt.Sprintf(uistate.T("investments.priceLabel"), sym))),
		),
		Div(css.Class(tw.FlexCol, tw.Gap1),
			Label(css.Class("t-caption", tw.TextDim),
				Attr("for", "hld-class-"+aid),
				uistate.T("investments.assetClassLabel")),
			Input(css.Class("field"),
				Attr("id", "hld-class-"+aid),
				Type("text"),
				Placeholder(uistate.T("investments.assetClassPlaceholder")),
				Value(classS.Get()), OnInput(onClass),
				Attr("aria-label", uistate.T("investments.assetClassLabel"))),
		),
	)

	saveBtn := Button(css.Class("btn-primary", tw.Mt3),
		OnClick(onSave),
		uistate.T("investments.addHolding"),
	)

	var errNode ui.Node = Fragment()
	if msg := errS.Get(); msg != "" {
		errNode = P(css.Class("t-caption", tw.TextDown, tw.Mt1), msg)
	}

	return Div(css.Class("card-inset", tw.Mt3, tw.FlexCol),
		Div(ClassStr("t-caption "+tw.Fold(tw.FontMedium, tw.TextDim)),
			uistate.T("investments.addHoldingTitle")),
		fields,
		saveBtn,
		errNode,
	)
}

// --------------------------------------------------------------------------
// investmentAccountCardProps — one investment account card with its holdings.
// --------------------------------------------------------------------------

type investmentAccountCardProps struct {
	Account  domain.Account
	Holdings []domain.Holding
	Sym      string
	Dec      int
}

// investmentAccountCard renders one full account section: header, performance
// summary (C220), asset-class allocation bars (C221), and holdings list with
// add form (C219). Each is its own component for stable hooks.
func investmentAccountCard(props investmentAccountCardProps) ui.Node {
	a := props.Account
	holdings := props.Holdings
	sym := props.Sym
	dec := props.Dec

	phSlice := portfolio.FromDomainSlice(holdings)
	summary := portfolio.PortfolioSummary(phSlice)
	allocation := portfolio.AllocationByAssetClass(phSlice)

	gainTone := gainToneClass(summary.TotalGainMinor)

	// C220 — performance summary tiles (2×2 grid)
	summaryNode := Div(css.Class(tw.Grid, tw.GridCols2, tw.Gap3, tw.Mb3),
		Div(css.Class("stat"),
			Div(css.Class("stat-label"), uistate.T("investments.totalValue")),
			Div(ClassStr("stat-value"), fmtSignedMoney(summary.TotalValueMinor, sym, dec)),
		),
		Div(css.Class("stat"),
			Div(css.Class("stat-label"), uistate.T("investments.totalCost")),
			Div(ClassStr("stat-value text-dim"), fmtSignedMoney(summary.TotalCostMinor, sym, dec)),
		),
		Div(css.Class("stat"),
			Div(css.Class("stat-label"), uistate.T("investments.totalGain")),
			Div(ClassStr("stat-value "+gainTone),
				fmtSignedMoney(summary.TotalGainMinor, sym, dec)),
		),
		Div(css.Class("stat"),
			Div(css.Class("stat-label"), uistate.T("investments.returnPct")),
			Div(ClassStr("stat-value "+gainTone),
				fmt.Sprintf("%.2f%%", summary.ReturnPct)),
		),
	)

	// C221 — asset-class allocation bar list
	var allocationNode ui.Node = Fragment()
	if len(allocation) > 0 {
		rows := []any{css.Class(tw.FlexCol, tw.Gap2, tw.Mb3)}
		for _, w := range allocation {
			pctStr := fmt.Sprintf("%.1f%%", w.Pct)
			barPct := w.Pct
			if barPct > 100 {
				barPct = 100
			}
			label := w.Label
			if label == "" {
				label = uistate.T("investments.assetClassOther")
			}
			rows = append(rows,
				Div(css.Class(tw.FlexCol, tw.Gap1),
					Div(css.Class(tw.Flex, tw.JustifyBetween),
						Span(css.Class("t-caption", tw.TextDim), label),
						Span(ClassStr("t-caption "+tw.Fold(tw.FontMedium)+" text-dim"),
							pctStr+" · "+fmtSignedMoney(w.ValueMinor, sym, dec)),
					),
					Div(Style(map[string]string{
						"background": "var(--line)", "border-radius": "3px", "height": "6px",
					}),
						Div(Style(map[string]string{
							"background":    "var(--up)",
							"border-radius": "3px",
							"height":        "6px",
							"width":         fmt.Sprintf("%.1f%%", barPct),
						})),
					),
				),
			)
		}
		allocationTitle := Div(ClassStr("t-caption "+tw.Fold(tw.FontMedium)+" text-dim "+tw.Fold(tw.Mb2)),
			uistate.T("investments.allocationTitle"))
		allocationNode = Div(css.Class(tw.FlexCol), allocationTitle, Div(rows...))
	}

	// C219 — per-holding rows (each is its own component via ui.CreateElement)
	var holdingsContent ui.Node
	if len(holdings) == 0 {
		holdingsContent = P(css.Class("t-caption", tw.TextDim, tw.Py2),
			uistate.T("investments.emptyHoldings"))
	} else {
		rows := []any{css.Class(tw.FlexCol)}
		for _, h := range holdings {
			hCopy := h
			app := appstate.Default
			p := holdingRowProps{
				H:   hCopy,
				Sym: sym,
				Dec: dec,
				OnDel: func() {
					if app != nil {
						_ = app.DeleteHolding(hCopy.ID)
					}
				},
			}
			rows = append(rows, ui.CreateElement(holdingRow, p))
		}
		holdingsContent = Div(rows...)
	}

	// Add form — stable component position per account card.
	formProps := addHoldingFormProps{AccountID: a.ID, Sym: sym, Dec: dec}

	badge := investmentAccountTypeBadge(a.Type)

	// Show total value in the header only if there are holdings.
	var headerRight ui.Node = Fragment()
	if len(holdings) > 0 {
		headerRight = Div(ClassStr("t-figure "+tw.Fold(tw.FontDisplay)),
			fmtSignedMoney(summary.TotalValueMinor, sym, dec))
	}

	header := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Mb3),
		Div(css.Class(tw.FlexCol, tw.Gap1),
			Div(ClassStr("t-body "+tw.Fold(tw.FontMedium)), a.Name),
			Span(css.Class("badge", "t-caption"), badge),
		),
		headerRight,
	)

	return uiw.Card(uiw.CardProps{
		Body: Div(css.Class(tw.FlexCol),
			header,
			If(len(holdings) > 0, summaryNode),
			If(len(holdings) > 0, allocationNode),
			holdingsContent,
			ui.CreateElement(addHoldingForm, formProps),
		),
	})
}

// --------------------------------------------------------------------------
// InvestmentsScreen is the /investments page.
//
// C219 — holdings list + add/delete per account
// C220 — performance summary (value, cost, gain/loss, return%)
// C221 — asset-class allocation breakdown
// --------------------------------------------------------------------------

// InvestmentsScreen renders the /investments page: per-account holdings list
// with performance summary and asset-class allocation bars (C219/C220/C221, F30).
func InvestmentsScreen() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()

	settings := app.Settings()
	baseCur := settings.BaseCurrency
	if baseCur == "" {
		baseCur = "USD"
	}
	sym := currency.Symbol(baseCur)
	dec := currency.Decimals(baseCur)

	accounts := app.Accounts()
	allHoldings := app.Holdings()

	// Filter to active investment accounts.
	var investAccounts []domain.Account
	for _, a := range accounts {
		if !a.Archived && isInvestmentAccount(a.Type) {
			investAccounts = append(investAccounts, a)
		}
	}

	// Empty state: no investment accounts at all.
	if len(investAccounts) == 0 {
		return uiw.Card(uiw.CardProps{
			Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap3),
				P(ClassStr("t-body "+tw.Fold(tw.FontMedium)), uistate.T("investments.noAccountsTitle")),
				P(css.Class("t-caption", tw.TextDim), uistate.T("investments.noAccountsBody")),
			),
		})
	}

	// Build a lookup: accountID → holdings.
	byAccount := make(map[string][]domain.Holding)
	for _, h := range allHoldings {
		byAccount[h.AccountID] = append(byAccount[h.AccountID], h)
	}

	// Overall portfolio summary across all investment accounts.
	var allPH []portfolio.Holding
	for _, a := range investAccounts {
		allPH = append(allPH, portfolio.FromDomainSlice(byAccount[a.ID])...)
	}
	overallSummary := portfolio.PortfolioSummary(allPH)
	overallTone := gainToneClass(overallSummary.TotalGainMinor)

	// Overall summary card — only shown when there is at least one holding.
	var overallCard ui.Node = Fragment()
	if len(allPH) > 0 {
		overallCard = uiw.Card(uiw.CardProps{
			Title: uistate.T("investments.overallTitle"),
			Body: Div(css.Class(tw.Grid, tw.GridCols2, tw.Gap3),
				Div(css.Class("stat"),
					Div(css.Class("stat-label"), uistate.T("investments.totalValue")),
					Div(ClassStr("stat-value"),
						fmtSignedMoney(overallSummary.TotalValueMinor, sym, dec)),
				),
				Div(css.Class("stat"),
					Div(css.Class("stat-label"), uistate.T("investments.totalCost")),
					Div(ClassStr("stat-value text-dim"),
						fmtSignedMoney(overallSummary.TotalCostMinor, sym, dec)),
				),
				Div(css.Class("stat"),
					Div(css.Class("stat-label"), uistate.T("investments.totalGain")),
					Div(ClassStr("stat-value "+overallTone),
						fmtSignedMoney(overallSummary.TotalGainMinor, sym, dec)),
				),
				Div(css.Class("stat"),
					Div(css.Class("stat-label"), uistate.T("investments.returnPct")),
					Div(ClassStr("stat-value "+overallTone),
						fmt.Sprintf("%.2f%%", overallSummary.ReturnPct)),
				),
			),
		})
	}

	// Per-account cards — each is its own component for stable hooks.
	cards := make([]any, 0, len(investAccounts)+2)
	cards = append(cards, css.Class(tw.Flex, tw.FlexCol, tw.Gap5))
	cards = append(cards, overallCard)
	for _, a := range investAccounts {
		p := investmentAccountCardProps{
			Account:  a,
			Holdings: byAccount[a.ID],
			Sym:      sym,
			Dec:      dec,
		}
		cards = append(cards, ui.CreateElement(investmentAccountCard, p))
	}

	return Div(cards...)
}

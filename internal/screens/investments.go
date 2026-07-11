// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/portfolio"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// isInvestmentAccount reports whether an account type holds investment positions
// (brokerage, retirement, or crypto). These are the accounts shown on /investments.
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

// securityTypeLabel is the display label for a security type (empty → other).
func securityTypeLabel(s domain.SecurityType) string {
	switch s.Normalized() {
	case domain.SecurityStock:
		return uistate.T("investments.secStock")
	case domain.SecurityETF:
		return uistate.T("investments.secETF")
	case domain.SecurityMutualFund:
		return uistate.T("investments.secMutualFund")
	case domain.SecurityBond:
		return uistate.T("investments.secBond")
	case domain.SecurityCrypto:
		return uistate.T("investments.secCrypto")
	case domain.SecurityCash:
		return uistate.T("investments.secCash")
	default:
		return uistate.T("investments.secOther")
	}
}

// securityTypeOptions builds the security-type picker options.
func securityTypeOptions() []uiw.SelectOption {
	out := make([]uiw.SelectOption, 0, len(domain.AllSecurityTypes))
	for _, s := range domain.AllSecurityTypes {
		out = append(out, uiw.SelectOption{Value: string(s), Label: securityTypeLabel(s)})
	}
	return out
}

// fmtShares formats a share count, trimming trailing zeros up to 6 decimals.
func fmtShares(v float64) string {
	s := fmt.Sprintf("%.6f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// fmtSignedMoney formats a signed minor-unit amount with symbol; negatives get "−".
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

// --- shared view -----------------------------------------------------------------

// investView is the derived render model the investment tiles share: the securities
// (per-ticker holdings) and their portfolio summary, the "traditional" balance-tracked
// investment accounts, and the combined total — so every tile reads one consistent model.
// An account is EITHER securities-tracked (has holdings) OR balance-tracked (traditional);
// it is never counted both ways, so the total can't double-count.
type investView struct {
	Base            string
	Sym             string
	Dec             int
	Securities      []domain.Holding // holdings on active investment accounts, value desc
	Traditional     []domain.Account // investment accounts with no holdings (balance-tracked)
	BalByID         map[string]int64 // traditional account balances (base currency)
	SecSummary      portfolio.Summary
	TradValueMinor  int64
	TotalValueMinor int64
	AllocClass      []portfolio.Weight
	AllocType       []portfolio.Weight
	HasAny          bool // any investment account exists at all
}

// investViewCache memoizes computeInvestView by store revision — the investments
// surface has ~6 tiles that each call it once per render.
var investViewCache = map[string]investView{}

// computeInvestView returns the shared investments model, memoized on the store
// revision so a multi-tile render aggregates the ledger once, not once per tile.
func computeInvestView(app *appstate.App) investView {
	return memoByRev(investViewCache, revKey(app), func() investView { return computeInvestViewRaw(app) })
}

// computeInvestViewRaw builds the shared model over the live store. Pure (no hooks).
func computeInvestViewRaw(app *appstate.App) investView {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	v := investView{Base: base, Sym: currency.Symbol(base), Dec: currency.Decimals(base), BalByID: map[string]int64{}}
	txns := app.Transactions()

	var accts []domain.Account
	for _, a := range app.Accounts() {
		if !a.Archived && isInvestmentAccount(a.Type) {
			accts = append(accts, a)
		}
	}
	v.HasAny = len(accts) > 0

	byAccount := map[string][]domain.Holding{}
	for _, h := range app.Holdings() {
		byAccount[h.AccountID] = append(byAccount[h.AccountID], h)
	}

	acctSet := map[string]bool{}
	for _, a := range accts {
		acctSet[a.ID] = true
		if len(byAccount[a.ID]) == 0 {
			// Traditional: valued by the account's own balance (FX-converted to base).
			bal, _ := ledger.Balance(a, txns)
			cv := bal.Amount
			if c, err := rates.Convert(bal, base); err == nil {
				cv = c.Amount
			}
			v.BalByID[a.ID] = cv
			v.TradValueMinor += cv
			v.Traditional = append(v.Traditional, a)
		}
	}
	// Securities: every holding on an active investment account.
	for _, h := range app.Holdings() {
		if acctSet[h.AccountID] {
			v.Securities = append(v.Securities, h)
		}
	}

	ph := portfolio.FromDomainSlice(v.Securities)
	v.SecSummary = portfolio.PortfolioSummary(ph)
	v.AllocClass = portfolio.AllocationByAssetClass(ph)
	v.AllocType = portfolio.AllocationBySecurityType(ph)
	v.TotalValueMinor = v.SecSummary.TotalValueMinor + v.TradValueMinor

	sort.SliceStable(v.Securities, func(i, j int) bool {
		return portfolio.HoldingValueMinor(portfolio.FromDomain(v.Securities[i])) >
			portfolio.HoldingValueMinor(portfolio.FromDomain(v.Securities[j]))
	})
	sort.SliceStable(v.Traditional, func(i, j int) bool {
		return v.BalByID[v.Traditional[i].ID] > v.BalByID[v.Traditional[j].ID]
	})
	return v
}

// investOwnerLink is a section's link to the page that owns its data (accounts).
func investOwnerLink(route, label string) ui.Node {
	return A(css.Class("debt-owner-link"), Href(uistate.RoutePath(route)),
		Attr("data-testid", "invest-link-"+route),
		Span(label),
		uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W3, tw.H3)),
	)
}

// investSection wraps a tile body with a serif section title + optional owning-page link,
// reusing the debt-section chrome so /investments matches the rest of the app.
func investSection(id, title string, action, body ui.Node) ui.Node {
	args := []any{css.Class("debt-section")}
	if id != "" {
		args = append(args, Attr("id", id))
	}
	if title != "" {
		args = append(args, Div(css.Class("debt-section-head"),
			H2(css.Class("debt-section-title"), title),
			If(action != nil, action),
		))
	}
	args = append(args, body)
	return Div(args...)
}

// InvestmentsScreen is the widgetized /investments surface — a thin SURFACE HOST like
// /debt and /accounts. It builds one engine RenderCtx over the live store and renders a
// fixed set of native tiles:
//
//   - invest-summary     (Native): the portfolio-value hero + gain/return + securities/traditional split
//   - invest-toolbar     (Native): Add holding, Manage accounts, Portfolio-metrics toggle
//   - invest-securities  (Native): the per-ticker holdings (with the add-holding form)
//   - invest-traditional (Native): balance-tracked investment accounts (only when any exist)
//   - invest-allocation  (Native): asset-class + security-type allocation (only when holdings exist)
//   - invest-formula     (Native): the opt-in Portfolio-metrics FormulaBuilder (toolbar toggle)
func InvestmentsScreen() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()
	formulasAtom := uistate.UseInvestShowFormulas()

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	accounts := app.Accounts()
	txns := app.Transactions()
	rctx := widgetrender.RenderCtx{
		App: app, Accounts: accounts, Txns: txns,
		ScopedAccounts: accounts, ScopedTxns: txns,
		Rates: rates, Base: base,
		Start: time.Time{}, End: time.Now(),
	}

	v := computeInvestView(app)

	specs := []domain.WidgetSpec{
		investNativeSpec("invest-summary"),
		investNativeSpec("invest-growth"),
		investNativeSpec("invest-toolbar"),
		investNativeSpec("invest-securities"),
	}
	if len(v.Securities) > 0 {
		specs = append(specs, investNativeSpec("invest-allocation"))
	}
	// The accounts tile (per-account growth cards + pool grouping) is the single account
	// list — it covers every investment account, so there is no separate "traditional" list.
	specs = append(specs, investNativeSpec("invest-pools"))
	if formulasAtom.Get() {
		specs = append(specs, investNativeSpec("invest-formula"))
	}

	return Div(css.Class("bento bento-invest"),
		MapKeyed(specs,
			func(sp domain.WidgetSpec) any { return sp.ID },
			func(sp domain.WidgetSpec) ui.Node {
				c := rctx
				c.Spec = sp
				if node, ok := safeRenderSpec(sp, c); ok {
					return node
				}
				return Fragment()
			},
		),
	)
}

// init registers the investments-surface widget bodies with the engine render registry.
func init() {
	R := widgetrender.Register
	R("invest-summary", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(investSummaryWidget, investPanelProps{App: c.App})
	})
	R("invest-growth", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(investGrowthWidget, investPanelProps{App: c.App})
	})
	R("invest-toolbar", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(investToolbarWidget, investPanelProps{App: c.App})
	})
	R("invest-securities", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(investSecuritiesWidget, investPanelProps{App: c.App})
	})
	R("invest-allocation", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(investAllocationWidget, investPanelProps{App: c.App})
	})
	R("invest-pools", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(investPoolsWidget, investPanelProps{App: c.App})
	})
	R("invest-formula", func(c widgetrender.RenderCtx) ui.Node {
		return ui.CreateElement(investFormulaWidget, investPanelProps{App: c.App})
	})
}

// investNativeSpec builds the seed spec for a Native investments tile (fixed surface).
func investNativeSpec(id string) domain.WidgetSpec {
	return domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: id, Kind: domain.KindNative, NativeID: id}
}

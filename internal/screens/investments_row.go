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
	"github.com/monstercameron/CashFlux/internal/portfolio"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// --- holding (security) card -----------------------------------------------------

type holdingRowProps struct {
	H         domain.Holding
	Sym       string
	Dec       int
	WeightPct float64      // this holding's share of total securities value
	OnClose   func(string) // mark the position sold (confirm + remove)
	OnDelete  func(string) // remove an entered-in-error record
	// OnPrice records a new price and the date it was true (FP-T2c). Nil leaves
	// the row read-only.
	OnPrice func(h domain.Holding, priceMinor int64, asOf time.Time)
}

// holdingRow renders one security position as a card: a security-type badge + name/ticker,
// the market value in the display serif, the gain/loss (toned) with return %, a shares ·
// cost meta line, and a portfolio-weight bar. Its own component (stable per-row hooks).
func holdingRow(props holdingRowProps) ui.Node {
	h := props.H

	// FP-T2c: the price editor. Until now a price change meant deleting the
	// position and re-entering it, which loses the record and is enough friction
	// that nobody does it — so a portfolio was stale by design and said nothing
	// about it.
	pricing := ui.UseState(false)
	priceS := ui.UseState("")
	asOfS := ui.UseState("")
	openPrice := func() {
		priceS.Set(strconv.FormatFloat(
			float64(h.CurrentPriceMinorPerShare)/float64(minorMul(props.Dec)), 'f', -1, 64))
		asOfS.Set(time.Now().Format("2006-01-02"))
		pricing.Set(true)
	}
	onPriceIn := ui.UseEvent(func(v string) { priceS.Set(v) })
	onAsOfIn := ui.UseEvent(func(v string) { asOfS.Set(v) })
	cancelPrice := ui.UseEvent(Prevent(func() { pricing.Set(false) }))
	savePrice := ui.UseEvent(Prevent(func() {
		f, err := strconv.ParseFloat(strings.TrimSpace(priceS.Get()), 64)
		if err != nil || f < 0 {
			uistate.PostNotice(uistate.T("investments.priceRequired"), true)
			return
		}
		// The as-of date defaults to today but is EDITABLE, because the price you
		// are entering is often last night's close or a statement from last week,
		// and stamping it "now" would quietly make stale data look fresh.
		asOf := time.Now()
		if d, derr := time.Parse("2006-01-02", strings.TrimSpace(asOfS.Get())); derr == nil {
			asOf = d
		}
		if props.OnPrice != nil {
			props.OnPrice(h, int64(f*float64(minorMul(props.Dec))), asOf)
		}
		pricing.Set(false)
	}))
	ph := portfolio.FromDomain(h)
	valueMinor := portfolio.HoldingValueMinor(ph)
	gainMinor := portfolio.UnrealizedGainMinor(ph)
	retPct := portfolio.ReturnPct(ph)
	tone := gainToneClass(gainMinor)

	name := h.Name
	if name == "" {
		name = h.Ticker
	}

	var tickerChip ui.Node = Fragment()
	if h.Ticker != "" {
		tickerChip = Span(css.Class("inv-ticker"), h.Ticker)
	}
	var classChip ui.Node = Fragment()
	if strings.TrimSpace(h.AssetClass) != "" {
		classChip = Span(css.Class("inv-chip inv-class"), h.AssetClass)
	}

	w := props.WeightPct
	if w > 100 {
		w = 100
	}
	if w < 0 {
		w = 0
	}

	return Div(css.Class("inv-card"), Attr("data-testid", "holding-"+h.ID), Attr("role", "listitem"),
		Div(css.Class("inv-card-body"),
			Div(css.Class("inv-head"),
				Span(css.Class("inv-sec-badge inv-sec-"+string(h.SecurityType.Normalized())), securityTypeLabel(h.SecurityType)),
				Span(css.Class("inv-name"), name),
				tickerChip,
				classChip,
			),
			Div(css.Class("inv-meta", tw.TextDim),
				uistate.T("investments.sharesAt", fmtShares(h.Shares), fmtSignedMoney(h.CurrentPriceMinorPerShare, props.Sym, props.Dec)),
				Span(css.Class("inv-sep"), " · "),
				uistate.T("investments.costMeta", fmtSignedMoney(h.CostBasisMinor, props.Sym, props.Dec)),
				Span(css.Class("inv-sep"), " · "),
				holdingPriceAge(h),
			),
			If(pricing.Get(), Form(css.Class("inv-price-form"), Attr("data-testid", "holding-price-form-"+h.ID),
				OnSubmit(savePrice),
				labeledField(uistate.T("investments.priceLabelPer", props.Sym),
					Input(css.Class("field"), Type("number"), Step("0.01"), Attr("min", "0"),
						Attr("data-testid", "holding-price-input-"+h.ID),
						Value(priceS.Get()), OnInput(onPriceIn))),
				labeledField(uistate.T("investments.priceAsOfLabel"),
					Input(css.Class("field"), Type("date"),
						Attr("data-testid", "holding-price-date-"+h.ID),
						Value(asOfS.Get()), OnInput(onAsOfIn))),
				Div(css.Class("inv-price-actions"),
					Button(css.Class("btn", "btn-primary"), Type("submit"),
						Attr("data-testid", "holding-price-save-"+h.ID), uistate.T("action.save")),
					Button(css.Class("btn"), Type("button"),
						Attr("data-testid", "holding-price-cancel-"+h.ID),
						OnClick(cancelPrice), uistate.T("action.cancel"))),
			)),
			Div(css.Class("inv-weight"),
				Div(css.Class("inv-weight-track"),
					Div(css.Class("inv-weight-fill"), Attr("style", fmt.Sprintf("width:%.1f%%", w)))),
				Span(css.Class("inv-weight-label", tw.TextDim), fmt.Sprintf("%.1f%%", props.WeightPct)),
			),
		),
		Div(css.Class("inv-side"),
			Span(css.Class("inv-value", tw.FontDisplay), fmtSignedMoney(valueMinor, props.Sym, props.Dec)),
			Div(ClassStr("inv-gain "+tw.ColorClass(tone)),
				Span(fmtSignedMoney(gainMinor, props.Sym, props.Dec)),
				Span(css.Class("inv-gain-pct"), fmt.Sprintf(" (%.2f%%)", retPct)),
			),
			// The app-standard ⋯ menu (was an instant-delete ×): closing marks the
			// position sold; deleting removes an entered-in-error record. Both
			// confirm before touching anything.
			uiw.KebabMenu(uiw.KebabMenuProps{
				ID:           "holding-menu-" + h.ID,
				AriaLabel:    uistate.T("investments.holdingMenuAria"),
				ToggleTestID: "holding-menu-btn-" + h.ID,
				Items: []ui.Node{
					Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
						Attr("data-testid", "holding-price-"+h.ID),
						Title(uistate.T("investments.updatePrice")),
						OnClick(openPrice),
						uistate.T("investments.updatePrice")),
					Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
						Attr("data-testid", "holding-close-"+h.ID),
						Title(uistate.T("investments.closePosition")),
						OnClick(func() {
							if props.OnClose != nil {
								props.OnClose(h.ID)
							}
						}),
						uistate.T("investments.closePosition")),
					Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
						Attr("data-testid", "holding-del-"+h.ID),
						Attr("aria-label", fmt.Sprintf(uistate.T("investments.deleteHoldingAria"), name)),
						Title(uistate.T("investments.deleteHoldingItem")),
						OnClick(func() {
							if props.OnDelete != nil {
								props.OnDelete(h.ID)
							}
						}),
						uistate.T("investments.deleteHoldingItem")),
				},
			}),
		),
	)
}

// --- add-holding form ------------------------------------------------------------

// InvestAddFormProps configures the add-security modal form.
type InvestAddFormProps struct {
	OnDone func() // called to close the modal (Cancel / backdrop / after nothing)
}

// InvestAddForm is the "Add a security" form shown inside the shell-root flip modal
// (InvestAddHost): pick the investment account + security type, then enter ticker / name /
// shares / cost / price. Saving adds the holding and clears the fields so several can be
// entered in a row (with a brief confirmation); Cancel calls OnDone to close. Its own
// component so its many input hooks sit at stable positions, and it reads the accounts +
// base currency itself so the host stays a thin wrapper.
func InvestAddForm(props InvestAddFormProps) ui.Node {
	app := appstate.Default
	base := "USD"
	var accounts []domain.Account
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
		accounts = investAccountsOf(app)
	}
	sym := currency.Symbol(base)
	dec := currency.Decimals(base)

	acctS := ui.UseState("")
	typeS := ui.UseState(string(domain.SecurityStock))
	tickerS := ui.UseState("")
	nameS := ui.UseState("")
	sharesS := ui.UseState("")
	costS := ui.UseState("")
	priceS := ui.UseState("")
	classS := ui.UseState("")
	errS := ui.UseState("")
	savedS := ui.UseState("")

	onTicker := ui.UseEvent(func(v string) { tickerS.Set(v) })
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onShares := ui.UseEvent(func(v string) { sharesS.Set(v) })
	onCost := ui.UseEvent(func(v string) { costS.Set(v) })
	onPrice := ui.UseEvent(func(v string) { priceS.Set(v) })
	onClass := ui.UseEvent(func(v string) { classS.Set(v) })

	// Default the account to the first one when unset.
	acct := acctS.Get()
	if acct == "" && len(accounts) > 0 {
		acct = accounts[0].ID
	}

	onSave := ui.UseEvent(Prevent(func() {
		app := appstate.Default
		if app == nil {
			return
		}
		aid := acct
		if aid == "" {
			errS.Set(uistate.T("investments.accountRequired"))
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
		mul := int64(1)
		for range dec {
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
			AccountID:                 aid,
			Ticker:                    strings.TrimSpace(tickerS.Get()),
			Name:                      name,
			Shares:                    sharesF,
			CostBasisMinor:            int64(costF * float64(mul)),
			CurrentPriceMinorPerShare: int64(priceF * float64(mul)),
			AssetClass:                strings.TrimSpace(classS.Get()),
			SecurityType:              domain.SecurityType(typeS.Get()),
		}
		if saveErr := app.PutHolding(h); saveErr != nil {
			errS.Set(saveErr.Error())
			return
		}
		uistate.BumpDataRevision()
		tickerS.Set("")
		nameS.Set("")
		sharesS.Set("")
		costS.Set("")
		priceS.Set("")
		classS.Set("")
		errS.Set("")
		savedS.Set(fmt.Sprintf(uistate.T("investments.addedFlash"), name))
	}))
	cancel := ui.UseEvent(Prevent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	acctOpts := make([]uiw.SelectOption, 0, len(accounts))
	for _, a := range accounts {
		acctOpts = append(acctOpts, uiw.SelectOption{Value: a.ID, Label: a.Name})
	}

	form := Form(css.Class("inv-add-form"), OnSubmit(onSave),
		Div(css.Class("form-grid"),
			labeledField(uistate.T("investments.accountLabel"),
				uiw.SelectInput(uiw.SelectInputProps{Options: acctOpts, Selected: acct,
					OnChange: func(v string) { acctS.Set(v) }, AriaLabel: uistate.T("investments.accountLabel"),
					TestID: "hld-account"})),
			labeledField(uistate.T("investments.securityTypeLabel"),
				uiw.SelectInput(uiw.SelectInputProps{Options: securityTypeOptions(), Selected: typeS.Get(),
					OnChange: func(v string) { typeS.Set(v) }, AriaLabel: uistate.T("investments.securityTypeLabel"),
					TestID: "hld-type"})),
			labeledField(uistate.T("investments.tickerLabel"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "hld-ticker"),
					Placeholder(uistate.T("investments.tickerPlaceholder")), OnInput(onTicker), uiw.FieldValue(tickerS.Get()))),
			labeledField(uistate.T("investments.nameLabel"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "hld-name"),
					Placeholder(uistate.T("investments.namePlaceholder")), OnInput(onName), uiw.FieldValue(nameS.Get()))),
			labeledField(uistate.T("investments.sharesLabel"),
				Input(css.Class("field"), Type("number"), Attr("min", "0"), Step("any"), Attr("data-testid", "hld-shares"),
					Placeholder(uistate.T("investments.sharesPlaceholder")), OnInput(onShares), uiw.FieldValue(sharesS.Get()))),
			labeledField(fmt.Sprintf(uistate.T("investments.costBasisLabel"), sym),
				Input(css.Class("field"), Type("number"), Attr("min", "0"), Step("any"), Attr("data-testid", "hld-cost"),
					Placeholder(fmt.Sprintf(uistate.T("investments.costBasisPlaceholder"), sym)), OnInput(onCost), uiw.FieldValue(costS.Get()))),
			labeledField(fmt.Sprintf(uistate.T("investments.priceLabel"), sym),
				Input(css.Class("field"), Type("number"), Attr("min", "0"), Step("any"), Attr("data-testid", "hld-price"),
					Placeholder(fmt.Sprintf(uistate.T("investments.pricePlaceholder"), sym)), OnInput(onPrice), uiw.FieldValue(priceS.Get()))),
			labeledField(uistate.T("investments.assetClassLabel"),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "hld-class"),
					Placeholder(uistate.T("investments.assetClassPlaceholder")), OnInput(onClass), uiw.FieldValue(classS.Get()))),
		),
		If(errS.Get() != "", P(css.Class("err"), Attr("role", "alert"), errS.Get())),
		If(errS.Get() == "" && savedS.Get() != "", P(ClassStr("t-caption "+tw.ColorClass("text-up")), Attr("role", "status"), savedS.Get())),
		Div(css.Class("modal-sticky-foot"),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "hld-cancel"), OnClick(cancel), uistate.T("investments.doneAdding")),
			Button(css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "hld-save"), uistate.T("investments.addHolding")),
		),
	)
	return Div(css.Class("inv-add-modal"), Attr("data-testid", "invest-add-form"), form)
}

// minorMul is 10^dec — the multiplier between a major-unit price and minor units.
func minorMul(dec int) int64 {
	mul := int64(1)
	for range dec {
		mul *= 10
	}
	return mul
}

// holdingPriceAge states when the holding's price was last true (FP-T2c).
//
// An unrecorded date reads as "price date not recorded", not as a blank and
// certainly not as today. A price with an unknown age and a price from this
// morning are different claims about the same number, and every value, weight
// and return on this page is computed from it — a portfolio priced two years ago
// is wrong in a way nothing downstream can detect.
//
// Past a threshold the line raises its voice, because the failure mode here is
// silent: a stale portfolio looks exactly like a fresh one.
func holdingPriceAge(h domain.Holding) ui.Node {
	if h.PriceAsOf.IsZero() {
		return Span(css.Class("inv-price-age"), Attr("data-testid", "holding-price-age-"+h.ID),
			uistate.T("investments.priceNoDate"))
	}
	days := int(time.Since(h.PriceAsOf).Hours() / 24)
	cls := "inv-price-age"
	if days > holdingPriceStaleDays {
		cls = "inv-price-age is-stale"
	}
	return Span(css.Class(cls), Attr("data-testid", "holding-price-age-"+h.ID),
		uistate.T("investments.pricedOn", h.PriceAsOf.Format("2 Jan 2006")))
}

// holdingPriceStaleDays is when a price stops being worth trusting silently.
//
// Thirty days: long enough that a household pricing monthly from a statement is
// not nagged, short enough that a quarter-old price cannot pass as current.
const holdingPriceStaleDays = 30

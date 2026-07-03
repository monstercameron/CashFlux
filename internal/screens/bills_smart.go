// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/billsched"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// billItemID is the stable identity a bill occurrence carries through the
// scheduler (must match how the engine builds billsched items).
func billItemID(b bills.Bill) string {
	return b.AccountID + "|" + b.DueDate.Format("2006-01-02") + "|" + b.Name
}

// billsSmartPlan bundles everything the bills views need from the scheduler.
type billsSmartPlan struct {
	Cfg       uistate.BillsSmartConfig
	HasAnchor bool
	Res       billsched.Result
	Base      string
}

// computeBillsSmart runs the billsched optimizer over every bill OCCURRENCE in
// [now, until] (each repeat of each bill in the window — paying next month's
// occurrence on this month's payday is the whole feature) against the
// configured pay cycle — shared by the bills tab (rows + calendar) and the
// smart-schedule modal, so both always agree. The standard window is
// engineenv.BillsSmartHorizonDays; the bills tab extends `until` to cover
// whatever month the calendar is paged to.
func computeBillsSmart(app *appstate.App, now, until time.Time) billsSmartPlan {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	cfg := uistate.BillsSmartConfigGet()
	horizonDays := int(until.Sub(now).Hours() / 24)
	if minDays := engineenv.BillsSmartHorizonDays; horizonDays < minDays {
		horizonDays, until = minDays, now.AddDate(0, 0, minDays)
	}
	in := liveBillsSmartHorizon(app, horizonDays)

	occurrences := bills.OccurrencesWithin(app.Accounts(), app.Recurring(), now, until)
	items := make([]billsched.Item, 0, len(occurrences))
	for _, b := range occurrences {
		amt, err := rates.Convert(b.Amount, base)
		if err != nil {
			amt = money.New(b.Amount.Amount, base)
		}
		items = append(items, billsched.Item{
			ID: billItemID(b), Name: b.Name, Amount: amt.Amount, Due: b.DueDate, Movable: !b.Autopay,
		})
	}
	liquid, _ := ledger.LiquidBalance(app.Accounts(), app.Transactions(), rates)
	return billsSmartPlan{
		Cfg:       cfg,
		HasAnchor: len(in.Paydays) > 0,
		Res:       billsched.Optimize(liquid.Amount, items, in.Paydays, in.IncomePerPayday, now, horizonDays, in.MinKeepMinor),
		Base:      base,
	}
}

// billsSmartSummaryProps feeds the compact on-page tile.
type billsSmartSummaryProps struct {
	Plan billsSmartPlan
}

// billsSmartSummaryTile is the bills tab's compact smart-schedule presence: one
// status line, one button into the flip modal where everything is configured,
// and — once the plan is on — the raw/smart view toggle. All the inputs live in
// the modal so the page stays scannable.
func billsSmartSummaryTile(props billsSmartSummaryProps) ui.Node {
	plan := props.Plan
	open := uistate.UseBillsSmartOpen()
	openModal := ui.UseEvent(Prevent(func() { open.Set(true) }))
	onView := func(v string) {
		c := uistate.BillsSmartConfigGet()
		c.ViewSmart = v == "smart"
		uistate.SetBillsSmartConfig(c)
		uistate.BumpDataRevision()
	}

	status := uistate.T("bills.smartStatusOff")
	btnLabel := uistate.T("bills.smartSetUp")
	if plan.Cfg.Enabled && plan.HasAnchor {
		btnLabel = uistate.T("bills.smartAdjust")
		switch {
		case len(plan.Res.Moves) > 0 && plan.Res.EvenGainMinor > 0:
			status = uistate.T("bills.smartStatusOn", len(plan.Res.Moves), fmtMoney(money.New(plan.Res.EvenGainMinor, plan.Base)))
		case len(plan.Res.Moves) > 0:
			status = uistate.T("bills.smartStatusOnEven", len(plan.Res.Moves))
		default:
			status = uistate.T("bills.smartAlreadyEven")
		}
	}

	strip := Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", "bills-smart-open"),
				Title(uistate.T("bills.smartEnableTitle")), OnClick(openModal), btnLabel),
			Span(css.Class("muted"), Attr("data-testid", "bills-smart-status"), status),
		),
		If(plan.Cfg.Enabled && plan.HasAnchor, uiw.Segmented(uiw.SegmentedProps{
			Label:    uistate.T("bills.smartViewLabel"),
			Selected: viewValue(plan.Cfg.ViewSmart),
			OnSelect: onView,
			Options: []uiw.SegOption{
				{Value: "raw", Label: uistate.T("bills.viewRaw"), TestID: "bills-view-raw"},
				{Value: "smart", Label: uistate.T("bills.viewSmart"), TestID: "bills-view-smart"},
			},
		})),
	)
	return recurTile("bills-smart", recurSection("sec-bills-smart", uistate.T("bills.smartTitle"), nil, strip))
}

// BillsSmartFormProps configures the smart-schedule flip-modal body.
type BillsSmartFormProps struct {
	OnDone func()
}

// BillsSmartForm is the smart-pay-schedule flip modal: two questions (when's a
// payday, how often are you paid), a live plan preview computed by the
// deterministic billsched engine (pay-ahead moves + biller-side suggestions +
// the schedule's engine variables), an optional keep-floor under Advanced, an
// opt-in AI read, and one decision — Use this plan / Turn off.
func BillsSmartForm(props BillsSmartFormProps) ui.Node {
	app := appstate.Default
	// The setup inputs save live and bump the data revision; subscribing here is
	// what re-renders the plan preview as the answers change.
	_ = uistate.UseDataRevision().Get()

	onAnchor := ui.UseEvent(func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		pr := uistate.CurrentPrefs()
		pr.PayCycleAnchor = v
		uistate.SetPrefs(pr)
		uistate.BumpDataRevision()
	})
	onFreq := func(v string) {
		c := uistate.BillsSmartConfigGet()
		c.PayFrequency = v
		uistate.SetBillsSmartConfig(c)
		uistate.BumpDataRevision()
	}
	advOpen := ui.UseState(false)
	toggleAdv := ui.UseEvent(Prevent(func() { advOpen.Set(!advOpen.Get()) }))
	usePlan := ui.UseEvent(Prevent(func() {
		c := uistate.BillsSmartConfigGet()
		c.Enabled = true
		c.ViewSmart = true
		uistate.SetBillsSmartConfig(c)
		uistate.BumpDataRevision()
		if props.OnDone != nil {
			props.OnDone()
		}
	}))
	turnOff := ui.UseEvent(Prevent(func() {
		c := uistate.BillsSmartConfigGet()
		c.Enabled = false
		c.ViewSmart = false
		uistate.SetBillsSmartConfig(c)
		uistate.BumpDataRevision()
		if props.OnDone != nil {
			props.OnDone()
		}
	}))
	cancel := ui.UseEvent(Prevent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	// AI state (optional read; the engine's figures are the ground truth).
	aiResult := ui.UseState("")
	aiLoading := ui.UseState(false)
	aiErr := ui.UseState("")

	if app == nil {
		return Fragment()
	}
	now := time.Now()
	plan := computeBillsSmart(app, now, now.AddDate(0, 0, engineenv.BillsSmartHorizonDays))
	base := plan.Base
	dec := currency.Decimals(base)
	res := plan.Res

	onKeep := ui.UseEvent(func(v string) {
		c := uistate.BillsSmartConfigGet()
		c.MinKeepMinor, _ = money.ParseMinor(strings.TrimSpace(v), dec)
		if c.MinKeepMinor < 0 {
			c.MinKeepMinor = 0
		}
		uistate.SetBillsSmartConfig(c)
		uistate.BumpDataRevision()
	})
	explain := ui.UseEvent(func() {
		settings := app.Settings()
		pr := uistate.LoadPrefs().Normalize()
		useBackend := pr.BackendActive()
		if settings.OpenAIKey == "" && !useBackend {
			aiErr.Set(uistate.T("allocate.needKey"))
			return
		}
		aiLoading.Set(true)
		aiErr.Set("")
		aiResult.Set("")
		var b strings.Builder
		fmt.Fprintf(&b, "Heaviest paycheck: %s raw vs %s smart. Bill payments paid ahead: %d. Projected 60-day low: %s.\n",
			fmtMoney(money.New(maxPeriodLoad(res.Raw.Loads), base)), fmtMoney(money.New(maxPeriodLoad(res.Smart.Loads), base)),
			len(res.Moves), fmtMoney(money.New(res.Raw.Low, base)))
		for i, mv := range res.Moves {
			if i >= 6 {
				break
			}
			fmt.Fprintf(&b, "- Pay %s on %s (due %s)\n", mv.Item.Name, mv.PayOn.Format("Jan 2"), mv.Item.Due.Format("Jan 2"))
		}
		for i, sg := range res.Suggestions {
			if i >= 4 {
				break
			}
			fmt.Fprintf(&b, "- Ask the biller to move %s to %s (low point +%s)\n", sg.Item.Name, sg.NewDue.Format("Jan 2"), fmtMoney(money.New(sg.LowGainMinor, base)))
		}
		model := settings.OpenAIModel
		if model == "" {
			model = "gpt-5.4-mini"
		}
		messages := []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a concise, friendly personal-finance assistant. In 2-3 sentences, explain this bill-payment plan's benefit and the single most valuable next step. Plain English; never invent numbers."},
			{Role: ai.RoleUser, Content: b.String()},
		}
		done := func(c string, _ ai.Usage) { aiLoading.Set(false); aiResult.Set(c) }
		fail := func(e string) { aiLoading.Set(false); aiErr.Set(e) }
		if useBackend {
			ai.SendProxyChat(pr.ServerURL, pr.ServerToken, model, messages, 0.5, done, fail)
		} else {
			ai.SendChat(settings.OpenAIKey, ai.DefaultBaseURL, model, messages, 0.5, done, fail)
		}
	})

	// --- the two setup questions ---
	anchorVal := uistate.LoadPrefs().PayCycleAnchor
	setup := Div(css.Class("bills-smart-setup"),
		labeledField(uistate.T("bills.smartAnchorLabel"),
			Input(css.Class("field"), Type("date"), Attr("data-testid", "bills-smart-anchor"),
				Attr("aria-label", uistate.T("bills.smartAnchorLabel")), Value(anchorVal), OnInput(onAnchor))),
		labeledField(uistate.T("bills.smartFreq"),
			uiw.Segmented(uiw.SegmentedProps{
				Label:    uistate.T("bills.smartFreq"),
				Selected: plan.Cfg.PayFrequency,
				OnSelect: onFreq,
				Options: []uiw.SegOption{
					{Value: "weekly", Label: uistate.T("bills.freqWeekly"), TestID: "bills-freq-weekly"},
					{Value: "biweekly", Label: uistate.T("bills.freqBiweekly"), TestID: "bills-freq-biweekly"},
					{Value: "semimonthly", Label: uistate.T("bills.freqSemimonthly"), TestID: "bills-freq-semimonthly"},
					{Value: "monthly", Label: uistate.T("bills.freqMonthly"), TestID: "bills-freq-monthly"},
				},
			})),
	)

	// --- the live plan preview ---
	var preview ui.Node
	if !plan.HasAnchor {
		preview = P(css.Class("muted"), Attr("data-testid", "bills-smart-noanchor"), uistate.T("bills.smartNoAnchor"))
	} else {
		gainTone := ""
		if res.EvenGainMinor > 0 {
			gainTone = " " + tw.ColorClass("text-up")
		}
		// The plan chip carries its own improvement delta so the win is readable
		// without subtracting the two figures (the value span stays a bare amount).
		planChip := Div(css.Class("debt-stat"),
			Div(css.Class("debt-stat-label", tw.TextDim), uistate.T("bills.smartChipLoadSmart")),
			Div(ClassStr("debt-stat-value "+tw.Fold(tw.FontDisplay)+gainTone), fmtMoney(money.New(maxPeriodLoad(res.Smart.Loads), base))),
			If(res.EvenGainMinor > 0, Div(ClassStr("bills-smart-delta "+tw.ColorClass("text-up")),
				uistate.T("bills.smartChipDelta", fmtMoney(money.New(res.EvenGainMinor, base))))),
		)
		chips := Div(css.Class("debt-chips"),
			recurStatChip(uistate.T("bills.smartChipLoadRaw"), fmtMoney(money.New(maxPeriodLoad(res.Raw.Loads), base)), ""),
			planChip,
			recurStatChip(uistate.T("bills.smartChipMoves"), fmt.Sprintf("%d", len(res.AheadByID)), ""),
			recurStatChip(uistate.T("bills.smartChipLow"), fmtMoney(money.New(res.Raw.Low, base)), lowTone(res.Raw.Low)),
		)
		lowNote := P(css.Class("muted"), Attr("data-testid", "bills-smart-lownote"),
			uistate.T("bills.smartLowNote", fmtMoney(money.New(res.Raw.Low, base))))

		var planBody ui.Node
		if len(res.Moves) == 0 {
			planBody = P(css.Class("muted"), Attr("data-testid", "bills-smart-even"), uistate.T("bills.smartAlreadyEven"))
		} else {
			// The plan reads as PAYDAY BUCKETS — "pay these on the 1st, these on
			// the 15th" — because that's the product: scattered due dates
			// consolidated onto paydays with balanced totals. Moves arrive sorted
			// by pay-on date, so grouping is a linear walk.
			rows := []any{css.Class("bills-smart-moves"), Attr("data-testid", "bills-smart-moves")}
			for i := 0; i < len(res.Moves); {
				j, total := i, int64(0)
				for ; j < len(res.Moves) && res.Moves[j].PayOn.Equal(res.Moves[i].PayOn); j++ {
					total += res.Moves[j].Item.Amount
				}
				rows = append(rows, Div(css.Class("bills-smart-bucket-head"),
					uistate.T("bills.smartBucketHead", res.Moves[i].PayOn.Format("Mon, Jan 2"), j-i, fmtMoney(money.New(total, base)))))
				for ; i < j; i++ {
					mv := res.Moves[i]
					moveCls := "bills-smart-move"
					var tag ui.Node = Fragment()
					if mv.CycleAhead {
						moveCls += " is-ahead"
						tag = Span(css.Class("rec-tag"), Title(uistate.T("bills.payAheadHint")), uistate.T("bills.smartPayAhead"))
					}
					rows = append(rows, Div(ClassStr(moveCls),
						tag,
						Span(css.Class("bills-smart-move-text"),
							uistate.T("bills.smartMoveLine", mv.Item.Name, mv.PayOn.Format("Jan 2"), mv.Item.Due.Format("Jan 2"))),
						Span(css.Class("bills-smart-move-amt", tw.TextDim), fmtMoney(money.New(mv.Item.Amount, base))),
					))
				}
			}
			movesHint := uistate.T("bills.smartMovesHint", fmtMoney(money.New(res.EvenGainMinor, base)))
			if res.EvenGainMinor == 0 {
				// Real consolidation with no headline gain: the heaviest paycheck
				// is an immovable stack, but the buckets still get organized.
				movesHint = uistate.T("bills.smartMovesEven")
			}
			planBody = Fragment(
				P(css.Class("muted"), movesHint),
				Div(rows...),
			)
		}

		// The intro promises biller-side suggestions too, so their absence gets an
		// explicit empty state instead of silently vanishing.
		var suggestBody ui.Node = P(css.Class("muted", tw.Mt2), Attr("data-testid", "bills-smart-nosuggest"),
			uistate.T("bills.smartSuggestNone"))
		if len(res.Suggestions) > 0 {
			rows := []any{css.Class("bills-smart-suggests"), Attr("data-testid", "bills-smart-suggests")}
			for _, sg := range res.Suggestions {
				rows = append(rows, Div(css.Class("bills-smart-move is-suggest"),
					Span(css.Class("rec-tag rec-tag-suggest"), uistate.T("bills.smartAskBiller")),
					Span(css.Class("bills-smart-move-text"),
						uistate.T("bills.smartSuggestLine", sg.Item.Name, sg.NewDue.Format("Jan 2"))),
					Span(ClassStr("bills-smart-move-amt "+tw.ColorClass("text-up")), "+"+fmtMoney(money.New(sg.LowGainMinor, base))),
				))
			}
			suggestBody = Fragment(
				P(css.Class("muted", tw.Mt2), uistate.T("bills.smartSuggestHint")),
				Div(rows...),
			)
		}

		var aiBody ui.Node = Fragment()
		switch {
		case aiLoading.Get():
			aiBody = P(css.Class("muted"), uistate.T("allocate.aiLoading"))
		case aiErr.Get() != "":
			aiBody = P(css.Class("err"), Attr("role", "alert"), aiErr.Get())
		case aiResult.Get() != "":
			aiBody = P(css.Class("alloc-ai-result"), Attr("data-testid", "bills-smart-ai"), aiResult.Get())
		}

		preview = Fragment(
			chips,
			lowNote,
			planBody,
			suggestBody,
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "bills-smart-explain"), OnClick(explain), uistate.T("allocate.aiExplain")),
			),
			aiBody,
		)
	}

	// Advanced: the one optional knob plus the schedule's formula variables —
	// power-user material, tucked out of the primary read.
	advanced := Fragment(
		Button(css.Class("btn btn-sm disclosure-toggle", tw.Mt2), Type("button"), Attr("aria-expanded", ariaBool(advOpen.Get())),
			Attr("data-testid", "bills-smart-adv"), OnClick(toggleAdv), Text(uistate.T("bills.smartAdvanced"))),
		If(advOpen.Get(), Fragment(
			labeledField(uistate.T("bills.smartKeep", base),
				Input(css.Class("field bills-smart-keep"), Type("number"), Attr("min", "0"), Step("0.01"),
					Attr("aria-label", uistate.T("bills.smartKeep", base)), Attr("data-testid", "bills-smart-keep"),
					Value(minorInput(plan.Cfg.MinKeepMinor, dec)), OnInput(onKeep))),
			labeledField(uistate.T("bills.smartVarsLabel"),
				Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2), Attr("data-testid", "bills-smart-vars"),
					Span(css.Class("rec-flow-var"), Title(uistate.T("bills.smartVarHint")), "bills_even_gain"),
					Span(css.Class("rec-flow-var"), Title(uistate.T("bills.smartVarHint")), "bills_suggest_gain"),
				)),
		)),
	)

	return Div(css.Class("bills-smart-modal"), Attr("data-testid", "bills-smart-form"),
		P(css.Class("muted"), uistate.T("bills.smartHint")),
		setup,
		preview,
		advanced,
		If(plan.HasAnchor, P(css.Class("muted", tw.Mt3), Attr("data-testid", "bills-smart-usehint"),
			uistate.T("bills.smartUseHint"))),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt3),
			If(plan.HasAnchor, Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "bills-smart-use"),
				OnClick(usePlan), uistate.T("bills.smartUsePlan"))),
			If(plan.Cfg.Enabled, Button(css.Class("btn"), Type("button"), Attr("data-testid", "bills-smart-off"),
				OnClick(turnOff), uistate.T("bills.smartTurnOff"))),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "bills-smart-cancel"), OnClick(cancel), uistate.T("action.cancel")),
		),
	)
}

// viewValue maps the persisted flag onto the Segmented value.
func viewValue(smart bool) string {
	if smart {
		return "smart"
	}
	return "raw"
}

// lowTone colors a projected low: danger when negative.
func lowTone(v int64) string {
	if v < 0 {
		return " " + tw.ColorClass("text-down")
	}
	return ""
}

// maxPeriodLoad returns the heaviest pay period's billed total.
func maxPeriodLoad(loads []billsched.PeriodLoad) int64 {
	var m int64
	for _, l := range loads {
		if l.Total > m {
			m = l.Total
		}
	}
	return m
}

// minorInput renders a minor-unit amount for a number input ("" when zero).
func minorInput(minor int64, dec int) string {
	if minor <= 0 {
		return ""
	}
	return money.FormatMinor(minor, dec)
}

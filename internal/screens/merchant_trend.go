// SPDX-License-Identifier: MIT

//go:build js && wasm

// Merchant spending-trend affordance for the transactions table (TX6b). The rich
// merchant "story" (recent-charge sparkline, this-vs-typical, visit frequency, month
// vs. a typical month) used to live only inside the edit modal — too hidden. This
// surfaces it from the row itself: a subtle trend chip appears on rows whose merchant
// has enough history; clicking it opens an anchored popover that shows a brief spinner
// (the per-merchant stats compute lazily) and then the trend. The heavy work is not
// done per row on render — only a cheap per-page charge count decides which rows get
// the chip; the full stats compute on demand when a chip is opened.
package screens

import (
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/merchantstats"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// merchantChargeCounts returns, keyed by lowercased resolved-merchant name, how many
// qualifying expense charges each merchant has across the whole ledger. Cheap O(txns)
// pass computed once per list render so a row can decide (O(1)) whether to show the
// trend chip without each row scanning the ledger itself.
func merchantChargeCounts(app *appstate.App) map[string]int {
	resolver := app.PayeeResolver()
	counts := map[string]int{}
	for _, t := range app.Transactions() {
		if t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(resolver.Resolve(firstNonEmpty(t.Payee, t.Desc))))
		if key == "" {
			continue
		}
		counts[key]++
	}
	return counts
}

// merchant-charge-count memo: the count map depends only on the full ledger (+ payee
// aliases), which change together with the data revision — so recompute it only when
// the revision moves, not on every sort / select / pagination re-render of the table.
var (
	mccCache map[string]int
	mccRev   = -1
)

// merchantChargeCountsMemo returns the per-merchant charge counts, recomputing only
// when the data revision has changed since the last call. Safe to share (read-only).
func merchantChargeCountsMemo(app *appstate.App, dataRev int) map[string]int {
	if dataRev == mccRev && mccCache != nil {
		return mccCache
	}
	mccCache = merchantChargeCounts(app)
	mccRev = dataRev
	return mccCache
}

// toBaseMag converts a money amount to a positive magnitude in the base currency.
func toBaseMag(app *appstate.App, m money.Money, base string) (int64, bool) {
	if m.Currency == "" || m.Currency == base {
		return absMinor(m.Amount), true
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	v, err := currency.ConvertBetween(m.Amount, m.Currency, base, rates)
	if err != nil {
		return 0, false
	}
	return absMinor(v), true
}

// computeMerchantStats gathers this merchant's expense charges (FX-converted to base)
// and computes its merchantstats. Returns (stats, base, ok) where ok is false for a
// one-off merchant with too little history to be worth showing.
func computeMerchantStats(app *appstate.App, merchant string) (merchantstats.Stats, string, bool) {
	merchant = strings.TrimSpace(merchant)
	if merchant == "" {
		return merchantstats.Stats{}, "", false
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	resolver := app.PayeeResolver()
	target := strings.ToLower(merchant)

	toBase := func(m money.Money) (int64, bool) {
		if m.Currency == "" || m.Currency == base {
			return absMinor(m.Amount), true
		}
		v, err := currency.ConvertBetween(m.Amount, m.Currency, base, rates)
		if err != nil {
			return 0, false
		}
		return absMinor(v), true
	}

	var charges []merchantstats.Charge
	for _, t := range app.Transactions() {
		if t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		if strings.ToLower(strings.TrimSpace(resolver.Resolve(firstNonEmpty(t.Payee, t.Desc)))) != target {
			continue
		}
		mag, ok := toBase(t.Amount)
		if !ok {
			continue
		}
		charges = append(charges, merchantstats.Charge{Date: t.Date, Minor: mag})
	}
	stats := merchantstats.Compute(charges, time.Now(), uistate.LoadPrefs().WeekStartWeekday())
	return stats, base, stats.Enough
}

// merchantStoryNodes renders the shared merchant "story" content — the title + this-vs-
// typical delta, visit frequency, month-vs-typical line, and the recent-charge
// sparkline — used by BOTH the edit-modal panel and the row trend popover. thisMag is
// this transaction's magnitude in base (for the delta); hasThis is false when there is
// no single charge to compare (the popover always has one, the panel usually does).
func merchantStoryNodes(stats merchantstats.Stats, merchant, base string, thisMag int64, hasThis bool) []ui.Node {
	// The delta gets its own colour-coded treatment (a real figure, not a tag word), so
	// it never borrows the uppercase/letter-spaced `.badge` pill built for SPLIT/SMART.
	var deltaLine ui.Node = Fragment()
	if hasThis {
		delta := stats.DeltaVsTypical(thisMag)
		switch {
		case delta > 0:
			deltaLine = Span(css.Class("mtrend-delta is-up"), uistate.T("merchantPanel.aboveUsual", "+"+fmtMoney(money.New(delta, base))))
		case delta < 0:
			deltaLine = Span(css.Class("mtrend-delta is-down"), uistate.T("merchantPanel.belowUsual", "-"+fmtMoney(money.New(-delta, base))))
		default:
			deltaLine = Span(css.Class("mtrend-delta is-flat"), uistate.T("merchantPanel.atUsual"))
		}
	}

	// Each line is a block (Div), so the layout never depends on the container being a
	// flex column — the visit and month lines can't run together onto one line.
	var visitLine ui.Node = Fragment()
	if stats.VisitsThisWeek > 0 {
		freq := uistate.T("merchantPanel.visitThisWeek", ordinalDay(stats.VisitsThisWeek))
		if stats.VisitsThisMonth > 0 {
			freq += " · " + uistate.T("merchantPanel.visitsThisMonth", stats.VisitsThisMonth)
		}
		visitLine = Div(css.Class("muted", tw.Text12), freq)
	} else if stats.VisitsThisMonth > 0 {
		visitLine = Div(css.Class("muted", tw.Text12), uistate.T("merchantPanel.visitsThisMonth", stats.VisitsThisMonth))
	}

	var monthLine string
	switch {
	case stats.VisitsThisMonth == 0 && stats.TypicalMonth > 0:
		// No charge in the current calendar month yet. A bare "$0.00 this month" reads
		// as alarming for a recurring merchant that simply hasn't posted this month —
		// say so plainly, and anchor it to the typical monthly figure.
		monthLine = uistate.T("merchantPanel.noneThisMonth", fmtMoney(money.New(stats.TypicalMonth, base)))
	case stats.TypicalMonth > 0:
		monthLine = uistate.T("merchantPanel.monthVsTypical",
			fmtMoney(money.New(stats.SpentThisMonth, base)), fmtMoney(money.New(stats.TypicalMonth, base)))
	default:
		monthLine = uistate.T("merchantPanel.monthSpentOnly", fmtMoney(money.New(stats.SpentThisMonth, base)))
	}

	return []ui.Node{
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2),
			Strong(uistate.T("merchantPanel.title", merchant)),
			deltaLine,
		),
		visitLine,
		Div(css.Class("muted", tw.Text12), monthLine),
		merchantSparkline(stats.Last12, stats.Last12Dates, base),
	}
}

// merchantSparkline wraps the bare recent-charge polyline with the scale metadata a
// reader needs to make sense of it: a y-axis legend (the high + low charge), an x-axis
// time span (oldest → newest charge date), and a caption with the count + latest
// charge. Value text lives in HTML around the SVG (the polyline stretches with
// preserveAspectRatio="none", which would distort any text inside it).
func merchantSparkline(mags []int64, dates []time.Time, base string) ui.Node {
	if len(mags) < 2 || len(dates) != len(mags) {
		return sparklineSVG(mags) // fall back to the bare line if dates are unavailable
	}
	lo, hi := mags[0], mags[0]
	for _, v := range mags {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	latest := mags[len(mags)-1]
	oldest, newest := dates[0], dates[len(dates)-1]
	return Div(css.Class("mtrend-spark"),
		Div(css.Class("mtrend-spark-row"),
			sparklineSVG(mags),
			// y-axis legend: highest charge at the top, lowest at the bottom.
			Div(css.Class("mtrend-spark-yaxis"),
				Span(css.Class("mtrend-spark-hi"), fmtMoney(money.New(hi, base))),
				Span(css.Class("mtrend-spark-lo"), fmtMoney(money.New(lo, base))),
			),
		),
		// x-axis: the span the line covers (oldest → newest charge).
		Div(css.Class("mtrend-spark-xaxis"),
			Span(oldest.Format("Jan '06")),
			Span(newest.Format("Jan '06")),
		),
		Div(css.Class("mtrend-spark-meta"),
			uistate.T("merchantTrend.sparkMeta", len(mags), fmtMoney(money.New(latest, base)))),
	)
}

// merchantTrendChipProps configures a row's trend affordance.
type merchantTrendChipProps struct {
	Merchant string      // resolved merchant display name (row already knows it has history)
	TxnID    string      // for stable ids/testids
	Amount   money.Money // this row's amount, for the this-vs-typical delta
}

// mtrendState is the whole per-chip state in one struct, so a closed chip costs a
// single UseState hook (not six) — 17 chips render on a table page, so the per-row
// hook footprint matters for /transactions render cost.
type mtrendState struct {
	Open, Ready bool
	Stats       merchantstats.Stats
	Base        string
	Mag         int64
	HasMag      bool
}

// mtrendCached is a merchant's computed story, cached across opens so re-opening the
// same merchant is instant (no spinner) — the spinner is only paid on a genuine first
// compute. Session-scoped (cleared on reload); slight staleness on a repeat open is a
// fine trade for a read-only trend viz.
type mtrendCached struct {
	stats merchantstats.Stats
	base  string
	ok    bool
}

var mtrendStatsCache = map[string]mtrendCached{}

// minTrendChipCharges gates which rows show the trend chip. It's deliberately higher
// than merchantstats.MinCharges (the edit-modal panel's bar): the chip is ambient on
// the ledger, so it should stay selective — only merchants you transact with often
// enough to have a real pattern — rather than appearing on nearly every row.
const minTrendChipCharges = 5

// merchantTrendChip is the per-row trend affordance: a small history glyph that toggles
// an anchored popover. On the FIRST open it shows a brief spinner while this merchant's
// stats compute lazily; repeat opens read the cache and show instantly. Its own
// component so its hooks sit at stable positions, never inside the row's map loop.
func merchantTrendChip(props merchantTrendChipProps) ui.Node {
	st := ui.UseState(mtrendState{})
	hovering := ui.UseRef(false) // pointer over the chip OR the popover (one hover region)
	wrapID := "mtrend-" + props.TxnID

	uiw.DismissPopover(st.Get().Open, wrapID, func() { st.Set(mtrendState{}) })
	uiw.AnchorPopover(st.Get().Open, wrapID)

	// openWith builds the open state from a (cached or freshly computed) result, adding
	// this row's this-vs-typical delta (a cheap single FX convert, so not cached).
	openWith := func(c mtrendCached) mtrendState {
		ns := mtrendState{Open: true, Ready: true, Base: c.base}
		if c.ok {
			ns.Stats = c.stats
			if app := appstate.Default; app != nil {
				ns.Mag, ns.HasMag = toBaseMag(app, props.Amount, c.base)
			}
		}
		return ns
	}

	// ensureOpen opens the popover, computing this merchant's stats lazily on the first
	// open (brief spinner) and reading the cache thereafter. Idempotent — a no-op when
	// already open, so hover-open and click-open (touch) share one path.
	ensureOpen := func() {
		if st.Get().Open {
			return
		}
		if c, ok := mtrendStatsCache[props.Merchant]; ok {
			st.Set(openWith(c)) // cached — instant, no spinner
			return
		}
		st.Set(mtrendState{Open: true}) // first compute — show the spinner beat
		var cb js.Func
		cb = js.FuncOf(func(js.Value, []js.Value) any {
			c := mtrendCached{}
			if app := appstate.Default; app != nil {
				if stats, base, ok := computeMerchantStats(app, props.Merchant); ok {
					c = mtrendCached{stats: stats, base: base, ok: true}
				}
			}
			mtrendStatsCache[props.Merchant] = c
			// Only reveal the computed content if the popover is still open — the pointer
			// may have left during the 400ms compute (the grace-close would have shut it).
			if st.Get().Open {
				st.Set(openWith(c))
			}
			cb.Release()
			return nil
		})
		js.Global().Call("setTimeout", cb, 400)
	}

	// Hover-driven open/close, mirroring the transaction follow-up chip. `hovering` is the
	// pointer being over the chip OR the popover, so the same handlers wire to both and the
	// mouse can bridge the small gap between them without the popover despawning.
	//
	// enter: open only after 500ms of continuous hover (a pointer merely passing over the
	// row never flashes it), and cancel any pending grace-close.
	enter := func() {
		hovering.Set(true)
		if st.Get().Open {
			return
		}
		var cb js.Func
		cb = js.FuncOf(func(js.Value, []js.Value) any {
			if hovering.Get() {
				ensureOpen()
			}
			cb.Release()
			return nil
		})
		js.Global().Call("setTimeout", cb, 500)
	}
	// leave: don't despawn instantly — wait a short grace period so the pointer can bridge
	// the chip→popover gap; the callback re-reads the live flag, so re-entering keeps it open.
	leave := func() {
		hovering.Set(false)
		var cb js.Func
		cb = js.FuncOf(func(js.Value, []js.Value) any {
			if !hovering.Get() {
				st.Set(mtrendState{})
			}
			cb.Release()
			return nil
		})
		js.Global().Call("setTimeout", cb, 240)
	}
	onEnter := ui.UseEvent(func(e ui.Event) { enter() })
	onLeave := ui.UseEvent(func(e ui.Event) { leave() })
	// Click also opens (touch has no hover), and stops propagation so it never triggers the
	// row's open-edit handler. Dismissal is hover-out / click-outside / Escape — no ✕ button.
	onClick := ui.UseEvent(func(e ui.Event) {
		e.PreventDefault()
		e.StopPropagation()
		ensureOpen()
	})

	s := st.Get()
	var pop ui.Node = Fragment()
	if s.Open {
		var body ui.Node
		switch {
		case !s.Ready:
			body = Div(css.Class("mtrend-loading"),
				Div(css.Class("mtrend-spinner"), Attr("role", "status"), Attr("aria-label", uistate.T("merchantTrend.loading"))))
		case s.Base == "":
			body = Span(css.Class("muted"), uistate.T("merchantTrend.none"))
		default:
			body = Div(css.Class("mtrend-card"),
				merchantStoryNodes(s.Stats, props.Merchant, s.Base, s.Mag, s.HasMag))
		}
		pop = Div(ClassStr("add-menu mtrend-pop"), Attr("role", "dialog"),
			Attr("data-testid", "mtrend-pop-"+props.TxnID),
			// Hovering the popover keeps it open (cancels the grace-close), so the pointer
			// can move off the chip and read the trend.
			OnMouseEnter(onEnter), OnMouseLeave(onLeave),
			body)
	}

	return Span(ClassStr("mtrend-wrap add-wrap"), Attr("id", wrapID),
		OnMouseEnter(onEnter), OnMouseLeave(onLeave),
		Button(css.Class("mtrend-chip"), Type("button"),
			Attr("data-testid", "mtrend-chip-"+props.TxnID),
			Attr("aria-haspopup", "dialog"), Attr("aria-expanded", ariaBool(s.Open)),
			Attr("aria-label", uistate.T("merchantTrend.label", props.Merchant)),
			Title(uistate.T("merchantTrend.label", props.Merchant)),
			OnClick(onClick),
			// A neutral history glyph — direction (up/down/flat) isn't known until the
			// lazy compute, so the resting chip must not imply "spending increased".
			uiw.Icon(icon.History, css.Class(tw.ShrinkO, tw.W35, tw.H35))),
		pop,
	)
}

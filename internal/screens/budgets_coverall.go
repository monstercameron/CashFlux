// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// coverAllFeatureCode is the SMART-B14 catalog code that gates the cover-overages
// feature (Free, on by default, opt-out under the Smart settings).
const coverAllFeatureCode = "SMART-B14"

// coverAllNextSource is the sentinel select value meaning "borrow from next month's
// same budget" rather than moving slack from another budget.
const coverAllNextSource = "__next__"

// coverAllOver is one over-budget the modal offers to cover: the shortfall to clear
// and the period boundaries used to apply the coverage as a per-period boost.
type coverAllOver struct {
	ID        string
	Name      string
	Shortfall money.Money // positive amount over the limit this period
	ThisStart time.Time   // current period start (the boost target)
	NextStart time.Time   // next period start (for the next-month borrow)
}

// coverAllSource is a budget with slack this period that can fund another's overage.
type coverAllSource struct {
	ID        string
	Name      string
	Avail     money.Money // positive remaining this period
	ThisStart time.Time
}

// coverAllData reuses the SAME evaluated statuses the budget grid shows (via
// computeBudgetView — rollup, smoothing, and member scope all applied) so the modal's
// over/source list matches the banner exactly, then derives each budget's period
// bounds the identical way computeBudgetView anchors them (so the per-period boost
// lands on the period whose overage it's covering). Pure read model.
func coverAllData(app *appstate.App, member string, vw period.Window, pr prefs.Prefs, showLastMonth bool) (overs []coverAllOver, sources []coverAllSource, base string) {
	v := computeBudgetView(app, member, vw, pr, showLastMonth)
	base = v.Base
	if base == "" {
		base = "USD"
	}
	weekStart := pr.WeekStartWeekday()
	var payAnchor time.Time
	if pr.PayCycleAnchor != "" {
		if t, err := time.Parse("2006-01-02", pr.PayCycleAnchor); err == nil {
			payAnchor = t
		}
	}
	// Anchor exactly like computeBudgetView: today when today is inside the viewed
	// window, otherwise the window's start.
	viewFrom, viewTo := vw.Range()
	anchor := viewFrom
	now := time.Now()
	if !now.Before(viewFrom) && now.Before(viewTo) {
		anchor = now
	}
	for _, st := range v.Statuses {
		b := st.Budget
		var bs, be time.Time
		if showLastMonth {
			bs, be = budgeting.PreviousPeriodRange(b.Period, anchor, weekStart)
		} else {
			bs, be = budgeting.PeriodRangeAnchored(b.Period, anchor, weekStart, payAnchor)
		}
		switch {
		case st.State == budgeting.StateOver:
			overs = append(overs, coverAllOver{ID: b.ID, Name: b.Name, Shortfall: budgeting.CoverAmount(st), ThisStart: bs, NextStart: be})
		case st.Remaining.Amount > 0:
			sources = append(sources, coverAllSource{ID: b.ID, Name: b.Name, Avail: st.Remaining, ThisStart: bs})
		}
	}
	sort.Slice(overs, func(i, j int) bool { return overs[i].Name < overs[j].Name })
	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
	return overs, sources, base
}

// coverAllButtonProps carries nothing — the button reads shared state itself.
type coverAllButtonProps struct{}

// coverAllBannerButton is the over-banner entry point that opens the cover-overages
// modal. Its own component so its click hook stays at a stable position; the CALLER
// gates it on SMART-B14 + an over-count so this only mounts when there's work to do.
func coverAllBannerButton(_ coverAllButtonProps) ui.Node {
	open := uistate.UseCoverAllOpen()
	onOpen := ui.UseEvent(Prevent(func() { open.Set(true) }))
	return Button(css.Class("btn btn-tool cover-all-open", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "budgets-cover-all"), Title(uistate.T("coverAll.title")), OnClick(onOpen),
		uiw.Icon(icon.Scale, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("coverAll.button")))
}

// budgetsCoverAllModal renders the cover-overages flip modal when its atom is set,
// as a shell-root sibling of the bento (no tile transform clips it). Matches the
// Add-budget footer standard (NoFooter + FlushBody + the form's pinned .modal-foot).
func budgetsCoverAllModal() ui.Node {
	open := uistate.UseCoverAllOpen()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("coverAll.title"),
		Width:     uiw.FlipMediumW,
		Height:    uiw.FlipMediumH,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   func() { open.Set(false) },
		Back:      ui.CreateElement(coverAllForm, coverAllFormProps{OnDone: func() { open.Set(false) }}),
	})
}

type coverAllFormProps struct{ OnDone func() }

// coverAllForm is the modal body: an intro, one row per over-budget with a per-budget
// coverage-source picker (leave / next month / another budget's slack), and a pinned
// Cancel + "Cover all" footer that applies every chosen coverage in one pass.
func coverAllForm(props coverAllFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()
	member := uistate.UseActiveMember().Get()
	pr := uistate.UsePrefs().Get()
	vw := uistate.UsePeriod().Get()
	showLM := uistate.UseBudgetsLastMonth().Get()
	overs, sources, base := coverAllData(app, member, vw, pr, showLM)
	dec := currency.Decimals(base)

	// choices: overBudgetID → source value ("" leave, "__next__", or a source budget id).
	// Seed each over to "next month" the first time this over-set is shown (a stable
	// signature so reopening with the same overs keeps the user's picks).
	choices := ui.UseState(map[string]string{})
	errText := ui.UseState("")
	sig := ""
	for _, o := range overs {
		sig += o.ID + ";"
	}
	seededSig := ui.UseState("")
	if seededSig.Get() != sig {
		seed := map[string]string{}
		for _, o := range overs {
			seed[o.ID] = coverAllNextSource
		}
		choices.Set(seed)
		errText.Set("")
		seededSig.Set(sig)
	}

	setChoice := func(overID, v string) {
		m := choices.Get()
		next := make(map[string]string, len(m))
		for k, val := range m {
			next[k] = val
		}
		next[overID] = v
		choices.Set(next)
	}

	srcByID := make(map[string]coverAllSource, len(sources))
	for _, s := range sources {
		srcByID[s.ID] = s
	}

	apply := ui.UseEvent(Prevent(func() {
		sel := choices.Get()
		// Validate that no source budget is over-drawn across all rows that draw on it.
		drawn := map[string]int64{}
		for _, o := range overs {
			src := sel[o.ID]
			if src == "" || src == coverAllNextSource {
				continue
			}
			drawn[src] += o.Shortfall.Amount + 1 // matches the cover-one-past-limit apply
		}
		for id, amt := range drawn {
			s := srcByID[id]
			if amt > s.Avail.Amount {
				errText.Set(uistate.T("coverAll.sourceShort", s.Name, fmtMoney(money.New(amt-s.Avail.Amount, base))))
				return
			}
		}

		// Accumulate per-period boost deltas per budget so multiple rows drawing on the
		// same source (or the same over) net correctly, then write each budget once.
		type pboost struct {
			start time.Time
			delta int64
		}
		byBudget := map[string][]pboost{}
		add := func(id string, start time.Time, delta int64) {
			byBudget[id] = append(byBudget[id], pboost{start, delta})
		}
		applied := 0
		for _, o := range overs {
			src := sel[o.ID]
			if src == "" || o.Shortfall.Amount <= 0 {
				continue
			}
			// Cover one minor unit PAST the shortfall: a budget at exactly its limit is
			// still classified "over" (spent >= limit), so covering just the exact
			// shortfall would leave it flagged. One cent more clears it (spent < limit).
			cover := o.Shortfall.Amount + 1
			switch src {
			case coverAllNextSource:
				// Borrow from next month's same budget: raise this period, lower the next.
				add(o.ID, o.ThisStart, cover)
				add(o.ID, o.NextStart, -cover)
			default:
				s, ok := srcByID[src]
				if !ok {
					continue
				}
				// Move this period's slack from the source budget to the over budget.
				add(o.ID, o.ThisStart, cover)
				add(s.ID, s.ThisStart, -cover)
			}
			applied++
		}
		if applied == 0 {
			errText.Set(uistate.T("coverAll.doneNone"))
			return
		}
		for id, ops := range byBudget {
			var b domain.Budget
			found := false
			for _, bb := range app.Budgets() {
				if bb.ID == id {
					b, found = bb, true
					break
				}
			}
			if !found {
				continue
			}
			for _, op := range ops {
				b = b.WithPeriodBoost(op.start, op.delta)
			}
			if err := app.PutBudget(b); err != nil {
				errText.Set(err.Error())
				return
			}
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		msg := uistate.T("coverAll.doneMany", applied)
		if applied == 1 {
			msg = uistate.T("coverAll.doneOne")
		}
		uistate.PostNotice(msg, false)
		if props.OnDone != nil {
			props.OnDone()
		}
	}))
	onCancel := ui.UseEvent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	})

	if len(overs) == 0 {
		return Form(css.Class("acct-edit-form"), OnSubmit(onCancel),
			Div(css.Class("modal-scroll"),
				P(css.Class("empty cover-all-empty"), Attr("data-testid", "cover-all-empty"), uistate.T("coverAll.none"))),
			Div(css.Class("modal-foot"),
				Button(css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "cover-all-close"), uistate.T("action.close"))),
		)
	}

	var totalOver int64
	for _, o := range overs {
		totalOver += o.Shortfall.Amount
	}
	intro := uistate.T("coverAll.intro", len(overs), fmtMoney(money.New(totalOver, base)))
	if len(overs) == 1 {
		intro = uistate.T("coverAll.introOne", fmtMoney(money.New(overs[0].Shortfall.Amount, base)))
	}

	listArgs := []any{css.Class("cover-all-list")}
	for _, o := range overs {
		oid := o.ID
		listArgs = append(listArgs, ui.CreateElement(coverAllRow, coverAllRowProps{
			Over: o, Sources: sources, Dec: dec, Selected: choices.Get()[oid],
			OnChange: func(v string) { setChoice(oid, v) },
		}))
	}

	return Form(css.Class("acct-edit-form cover-all"), OnSubmit(apply),
		Div(css.Class("modal-scroll"),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
				Attr("data-testid", "cover-all-intro"), intro),
			Div(listArgs...),
			P(css.Class("t-caption", tw.TextDim, "cover-all-hint"), uistate.T("coverAll.nextMonthHint")),
			If(errText.Get() != "", P(css.Class("err"), Attr("role", "alert"), errText.Get())),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "cover-all-cancel"), OnClick(onCancel), uistate.T("coverAll.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "cover-all-apply"), uistate.T("coverAll.apply")),
		),
	)
}

type coverAllRowProps struct {
	Over     coverAllOver
	Sources  []coverAllSource
	Dec      int
	Selected string
	OnChange func(string)
}

// coverAllRow is one over-budget line: its name + overage, and a source picker. Own
// component so the select's change hook sits at a stable position (never in a loop).
func coverAllRow(props coverAllRowProps) ui.Node {
	onChange := ui.UseEvent(func(e ui.Event) { props.OnChange(e.GetValue()) })
	o := props.Over
	opts := []any{css.Class("field cover-all-src"), Attr("data-testid", "cover-all-src-"+o.ID),
		Attr("aria-label", uistate.T("coverAll.sourceLabel")), OnChange(onChange)}
	opts = append(opts,
		Option(Value(""), SelectedIf(props.Selected == ""), uistate.T("coverAll.sourceSkip")),
		Option(Value(coverAllNextSource), SelectedIf(props.Selected == coverAllNextSource), uistate.T("coverAll.sourceNextMonth")))
	for _, s := range props.Sources {
		opts = append(opts, Option(Value(s.ID), SelectedIf(props.Selected == s.ID),
			uistate.T("coverAll.sourceBudget", s.Name, fmtMoney(s.Avail))))
	}
	return Div(css.Class("cover-all-row"), Attr("data-testid", "cover-all-row"),
		Div(css.Class("cover-all-row-main"),
			Span(css.Class("cover-all-row-name"), o.Name),
			Span(css.Class("cover-all-row-over", tw.TextDown), uistate.T("coverAll.overBy", fmtMoney(o.Shortfall)))),
		Div(css.Class("cover-all-row-src"),
			Span(css.Class("cover-all-row-srclabel", tw.Text12, tw.TextDim), uistate.T("coverAll.sourceLabel")),
			Select(opts...)))
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	uiw "github.com/monstercameron/CashFlux/internal/ui"

	"github.com/monstercameron/CashFlux/internal/benchseries"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// benchmarkPanelProps carries the portfolio curve the benchmark is compared
// against: the dates the growth chart plots and the portfolio's value at each,
// in major units.
type benchmarkPanelProps struct {
	Dates  []time.Time
	Values []float64
}

// investBenchmarkPanel imports a comparison series and reports how the portfolio
// did against it (C380).
//
// The app has no market-data feed and is not getting one — that is the
// local-first position, not a gap. But "is my portfolio doing well" cannot be
// answered in isolation, and the user can get the answer themselves: every index
// provider hands out a date,value CSV. Importing one turns a line with no
// reference into a comparison.
//
// The reading is stated in words, not left to the chart. Two indexed lines
// require the reader to eyeball a gap; "you are 4.2 points ahead of S&P 500 over
// this window" is the sentence they came for.
func investBenchmarkPanel(props benchmarkPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	open := ui.UseState(false)
	name := ui.UseState("")
	raw := ui.UseState("")
	errMsg := ui.UseState("")

	saved := uistate.LoadBenchmark()

	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()); errMsg.Set("") }))
	onName := ui.UseEvent(func(v string) { name.Set(v) })
	onRaw := ui.UseEvent(func(v string) { raw.Set(v); errMsg.Set("") })

	importIt := ui.UseEvent(Prevent(func() {
		label := strings.TrimSpace(name.Get())
		if label == "" {
			// A nameless series has no legend, and "the benchmark" tells a reader
			// nothing six months later about what they compared against.
			errMsg.Set(uistate.T("bench.errName"))
			return
		}
		s, err := benchseries.Parse(label, raw.Get())
		if err != nil {
			errMsg.Set(uistate.T("bench.errParse"))
			return
		}
		uistate.SaveBenchmark(s)
		raw.Set("")
		open.Set(false)
		from, to := s.Span()
		uistate.PostNotice(uistate.T("bench.imported", s.Name, len(s.Points),
			from.Format("Jan 2006"), to.Format("Jan 2006")), false)
	}))
	remove := ui.UseEvent(Prevent(func() {
		uistate.SaveBenchmark(benchseries.Series{})
		uistate.PostNotice(uistate.T("bench.removed"), false)
	}))

	// The comparison, when there is one to make.
	var read ui.Node = Fragment()
	if !saved.Empty() {
		c := benchseries.Align(props.Dates, props.Values, saved)
		switch {
		case !c.Known():
			// Saying WHY there is no comparison beats an empty space: the usual
			// cause is a benchmark whose dates do not reach this window.
			read = P(css.Class("muted"), Attr("data-testid", "invest-bench-nooverlap"),
				uistate.T("bench.noOverlap", saved.Name))
		default:
			lead := c.LeadPct()
			cls := "invest-bench-behind"
			key := "bench.behind"
			if lead >= 0 {
				cls, key = "invest-bench-ahead", "bench.ahead"
			}
			read = Fragment(
				P(ClassStr("invest-bench-read "+cls), Attr("data-testid", "invest-bench-read"),
					uistate.T(key, fmtPct1(absF(lead)), saved.Name)),
				P(css.Class("muted"), Attr("data-testid", "invest-bench-detail"),
					uistate.T("bench.detail", fmtSignedPct1(c.PortfolioPct),
						saved.Name, fmtSignedPct1(c.BenchmarkPct), len(c.Dates))),
				// A short overlay is honest only if it says it is short.
				If(c.Skipped > 0, P(css.Class("muted"), Attr("data-testid", "invest-bench-skipped"),
					uistate.T("bench.skipped", c.Skipped))),
			)
		}
	}

	var form ui.Node = Fragment()
	if open.Get() {
		form = Div(css.Class("invest-bench-form"),
			Label(css.Class("invest-bench-ctrl"),
				Span(css.Class("t-caption"), uistate.T("bench.nameLabel")),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "invest-bench-name"),
					Attr("aria-label", uistate.T("bench.nameLabel")),
					Placeholder(uistate.T("bench.namePlaceholder")), OnInput(onName), uiw.FieldValue(name.Get()))),
			Textarea(css.Class("field invest-bench-text"), Attr("rows", "6"),
				Attr("data-testid", "invest-bench-paste"),
				Attr("aria-label", uistate.T("bench.pasteLabel")),
				Placeholder(uistate.T("bench.placeholder")), OnInput(onRaw), uiw.FieldValue(raw.Get())),
			If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"),
				Attr("data-testid", "invest-bench-error"), errMsg.Get())),
			// Say the constraint rather than letting someone wait for a live feed
			// that is never coming.
			P(css.Class("muted"), uistate.T("bench.localOnly")),
			Button(css.Class("btn btn-primary btn-sm"), Type("button"),
				Attr("data-testid", "invest-bench-import"), OnClick(importIt),
				uistate.T("bench.import")),
		)
	}

	return Div(css.Class("invest-bench"), Attr("data-testid", "invest-bench"),
		read,
		Div(css.Class("invest-bench-actions"),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "invest-bench-toggle"),
				OnClick(toggle), benchToggleLabel(saved, open.Get())),
			If(!saved.Empty(), Button(css.Class("btn btn-sm"), Type("button"),
				Attr("data-testid", "invest-bench-remove"), OnClick(remove),
				uistate.T("bench.remove")))),
		form,
	)
}

// benchToggleLabel names the entry point for what it will do: import the first
// benchmark, or replace the one already loaded.
func benchToggleLabel(s benchseries.Series, open bool) string {
	if open {
		return uistate.T("action.cancel")
	}
	if s.Empty() {
		return uistate.T("bench.add")
	}
	return uistate.T("bench.replace", s.Name)
}

// fmtPct1 renders a percentage to one decimal place.
func fmtPct1(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) + "%" }

// fmtSignedPct1 keeps the sign, because "-3.2%" and "3.2%" are opposite
// findings and a legend that drops the sign is worse than no legend.
func fmtSignedPct1(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	if v > 0 {
		s = "+" + s
	}
	return s + "%"
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

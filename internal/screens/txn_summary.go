// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txntoolbar"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// txnSummaryHead is the transactions summary, built on the shared PageHeadline the
// way the Budgets and Accounts headers are. The period's net leads, with its running
// total drawn beside it, then spent and income as facts.
func txnSummaryHead(props txnToolbarRenderProps) ui.Node {
	view := props.Derived.View
	base := props.Base
	first, last := txnTrendWindow(props.Filter, props.Derived.Rows)
	series := txntoolbar.RunningTotal(txnDayPoints(props.Derived.Rows, props.Rates, base), first, last)
	values := make([]float64, len(series))
	for i, v := range series {
		values[i] = float64(v)
	}
	return uiw.PageHeadline(uiw.PageHeadlineProps{
		TestID:   "txn-summary",
		Figure:   uiw.PageHeadlineFigure(uiw.PageHeadlineFigureProps{Text: fmtMoney(money.New(view.Net, base)), TestID: "txn-summary-net"}),
		Sentence: Span(uistate.T("txnsum.sentence", txnCountPeriod(props.Filter))),
		Visual:   uiw.PageHeadlineTrend(uiw.PageHeadlineTrendProps{Values: values, Label: uistate.T("txnsum.trendLabel"), GradientID: "txn-summary-trend"}),
		Footer:   txnToolbarFoot(ui.CreateElement(txnToolbarCount, props)),
	})
}

// txnTrendWindow is the period the running total covers: the filter's own dates where
// it has them, and otherwise the span of the rows shown. With no rows and no dates the
// window is empty, which draws no line.
func txnTrendWindow(filter txnfilter.Criteria, rows []domain.Transaction) (first, last time.Time) {
	if from, err := dateutil.ParseDate(filter.From); err == nil && !from.IsZero() {
		first = from
	}
	if to, err := dateutil.ParseDate(filter.To); err == nil && !to.IsZero() {
		last = to
	}
	if first.IsZero() || last.IsZero() {
		for _, row := range rows {
			if first.IsZero() || row.Date.Before(first) {
				first = row.Date
			}
			if last.IsZero() || row.Date.After(last) {
				last = row.Date
			}
		}
	}
	return first, last
}

// txnDayPoints converts each counted row to the base currency, keyed by its date.
// Excluded rows stay out, as they do in the period totals.
func txnDayPoints(rows []domain.Transaction, rates currency.Rates, base string) []txntoolbar.DayPoint {
	points := make([]txntoolbar.DayPoint, 0, len(rows))
	for _, row := range rows {
		if row.ExcludeFromReports {
			continue
		}
		converted, err := rates.Convert(row.Amount, base)
		if err != nil {
			continue
		}
		points = append(points, txntoolbar.DayPoint{Day: row.Date, Minor: converted.Amount})
	}
	return points
}

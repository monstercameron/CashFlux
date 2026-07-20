// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/balancesheet"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// TWO SIDES — signature graphic #2.
//
// Two boundary lines — what you own on top, what you owe beneath — with the
// space between them shaded. That space IS your net worth, so the graphic's
// whole subject is the GAP: whether it is widening, and whether that is because
// the top is rising or the bottom is falling.
//
// This is the second design. The first stacked both sides at full magnitude
// around a zero line, and on a real household it rendered as a solid block: a
// $304,000 property holding is 79% of the asset side and essentially constant,
// so it set the Y scale and flattened every series that actually moves. That is
// precisely the pathology the old page had — a chart restating a number instead
// of showing a shape — just wearing a different form. The fix is the one THE
// BRIDGE already uses: floor the axis to the band the data really occupies, and
// disclose the floor in words rather than smuggling it.
//
// Composition did not survive that truncation (you cannot stack from zero on an
// axis that starts at $231,000), and it should not have: a constant slab
// carries no information in a change-over-time graphic. So "how big and what
// shape" is separated from "how it moved" — composition moves to the strips
// below, where each side is shown at exact figures and exact shares, now. Each
// graphic then does one job well instead of two badly.
//
// Only the gap is toned, and it is toned by MEANING: the accent while you own
// more than you owe, the danger colour only if the lines cross and the
// household is genuinely underwater. Debt on its own is never painted as alarm.

// nwsBucketLabel names a composition bucket.
func nwsBucketLabel(b balancesheet.Bucket) string {
	switch b {
	case balancesheet.BucketCash:
		return uistate.T("nw.bucketCash")
	case balancesheet.BucketInvested:
		return uistate.T("nw.bucketInvested")
	case balancesheet.BucketProperty:
		return uistate.T("nw.bucketProperty")
	case balancesheet.BucketOtherAsset:
		return uistate.T("nw.bucketOther")
	case balancesheet.BucketCredit:
		return uistate.T("nw.bucketCredit")
	case balancesheet.BucketLoans:
		return uistate.T("nw.bucketLoans")
	}
	return uistate.T("nw.bucketMortgage")
}

// nwsToneClass is the shared swatch modifier for stacking position i of a side.
// The composition strip segments and their key dots both use it, so a key can
// never drift from the strip it explains.
func nwsToneClass(asset bool, i int) string {
	if asset {
		return fmt.Sprintf("is-a%d", i)
	}
	return fmt.Sprintf("is-l%d", i)
}

// nwsGapScale maps a figure into the 0..100 viewBox across the band the two
// boundaries actually occupy — NOT from zero. A shared scale is what keeps the
// gap readable as a real quantity; flooring it is what keeps the movement
// visible at all.
type nwsGapScale struct{ lo, hi float64 }

func newNwsGapScale(pts []balancesheet.Point) nwsGapScale {
	lo, hi := float64(pts[0].LiabilitiesMinor), float64(pts[0].AssetsMinor)
	for _, p := range pts {
		for _, v := range []float64{float64(p.AssetsMinor), float64(p.LiabilitiesMinor)} {
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	pad := (hi - lo) * 0.06
	if pad <= 0 {
		pad = absFloat(hi)*0.1 + 1
	}
	return nwsGapScale{lo: lo - pad, hi: hi + pad}
}

func (s nwsGapScale) y(v float64) float64 {
	span := s.hi - s.lo
	if span <= 0 {
		return 50
	}
	return 100 * (s.hi - v) / span
}

const nwsSidesW = 1000.0

// nwsSidesX is the horizontal position of point i of n.
func nwsSidesX(i, n int) float64 {
	if n <= 1 {
		return 0
	}
	return nwsSidesW * float64(i) / float64(n-1)
}

// nwsLinePoints builds a polyline point list for one boundary.
func nwsLinePoints(vals []int64, s nwsGapScale) string {
	out := make([]string, 0, len(vals))
	for i, v := range vals {
		out = append(out, fmt.Sprintf("%.2f,%.3f", nwsSidesX(i, len(vals)), s.y(float64(v))))
	}
	return strings.Join(out, " ")
}

// nwsGapPath closes the region between the two boundaries: the upper edge left
// to right, the lower edge back again.
func nwsGapPath(upper, lower []int64, s nwsGapScale) string {
	n := len(upper)
	if n < 2 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&b, "%s%.2f %.3f ", cmd, nwsSidesX(i, n), s.y(float64(upper[i])))
	}
	for i := n - 1; i >= 0; i-- {
		fmt.Fprintf(&b, "L%.2f %.3f ", nwsSidesX(i, n), s.y(float64(lower[i])))
	}
	b.WriteString("Z")
	return b.String()
}

// nwsSides renders the labelled gap chart — dollar axis, dated axis, the two
// regions named where they sit, and the net worth called out at the right end —
// plus the composition strips for both sides.
func nwsSides(v nwsView) ui.Node {
	pts := v.Points
	if len(pts) < 2 {
		return P(css.Class("empty"), Attr("data-testid", "nws-sides-empty"), uistate.T("nws.sidesEmpty"))
	}
	scale := newNwsGapScale(pts)
	assets := make([]int64, len(pts))
	liabs := make([]int64, len(pts))
	for i, p := range pts {
		assets[i], liabs[i] = p.AssetsMinor, p.LiabilitiesMinor
	}
	first, last := pts[0], pts[len(pts)-1]

	// The gap is toned by meaning, not by the presence of debt.
	gapCls := "nws-gap"
	if last.NetMinor < 0 {
		gapCls += " is-underwater"
	}

	// Gridlines at round dollar values. The axis SHOWS the floor it was given —
	// a truncated scale the reader can read off is honest; one they cannot is
	// the thing charts get accused of.
	ticks := balancesheet.AxisTicks(int64(scale.lo), int64(scale.hi), 4)
	kids := []any{
		css.Class("nws-sides-svg"),
		Attr("viewBox", fmt.Sprintf("0 0 %.0f 100", nwsSidesW)),
		Attr("preserveAspectRatio", "none"),
		Attr("role", "img"),
		Attr("data-testid", "nws-sides-svg"),
		Attr("aria-label", uistate.T("nws.sidesAria",
			fmtMoney(money.New(last.AssetsMinor, v.Base)),
			fmtMoney(money.New(last.LiabilitiesMinor, v.Base)),
			fmtMoney(money.New(first.NetMinor, v.Base)),
			fmtMoney(money.New(last.NetMinor, v.Base)))),
	}
	for _, t := range ticks {
		y := scale.y(float64(t))
		kids = append(kids, Line(css.Class("nws-grid"),
			Attr("vector-effect", "non-scaling-stroke"),
			Attr("x1", "0"), Attr("x2", fmt.Sprintf("%.0f", nwsSidesW)),
			Attr("y1", fmt.Sprintf("%.3f", y)), Attr("y2", fmt.Sprintf("%.3f", y))))
	}
	kids = append(kids,
		Path(ClassStr(gapCls), Attr("d", nwsGapPath(assets, liabs, scale))),
		Polyline(css.Class("nws-line-assets"), Attr("fill", "none"),
			Attr("vector-effect", "non-scaling-stroke"),
			Attr("points", nwsLinePoints(assets, scale))),
		Polyline(css.Class("nws-line-liab"), Attr("fill", "none"),
			Attr("vector-effect", "non-scaling-stroke"),
			Attr("points", nwsLinePoints(liabs, scale))),
	)

	// Y axis: the tick values as real HTML in the gutter, so they stay crisp
	// under the stretched viewBox and are readable by assistive tech.
	yaxis := []any{css.Class("nws-yaxis"), Attr("data-testid", "nws-yaxis"), Attr("aria-hidden", "true")}
	for _, t := range ticks {
		yaxis = append(yaxis, Span(css.Class("nws-ytick"),
			Style(map[string]string{"top": fmt.Sprintf("%.3f%%", scale.y(float64(t)))}),
			fmtMoneyCompact(money.New(t, v.Base))))
	}
	// The floor gets its own tick at the bottom edge. The gridlines above it are
	// round numbers because those are what a person reads off an axis, but the
	// scale's actual starting point must be ON the scale — a chart that begins
	// somewhere other than zero has to say where, in the place the reader is
	// already looking, not only in a note underneath.
	yaxis = append(yaxis, Span(css.Class("nws-ytick", "is-floor"),
		Style(map[string]string{"top": "100%"}),
		fmtMoneyCompact(money.New(int64(scale.lo), v.Base))))

	// The two regions NAMED where they sit, each with its current figure, and
	// the net worth called out between them: the reader should not have to
	// decode a caption to learn which half is which.
	tops := nwsSpreadLabels([]float64{
		scale.y(float64(last.AssetsMinor)),
		(scale.y(float64(last.AssetsMinor)) + scale.y(float64(last.LiabilitiesMinor))) / 2,
		scale.y(float64(last.LiabilitiesMinor)),
	})
	anno := Div(css.Class("nws-annos"), Attr("data-testid", "nws-annos"),
		nwsAnno("is-assets", tops[0], uistate.T("nws.stripOwn"),
			fmtMoney(money.New(last.AssetsMinor, v.Base))),
		nwsAnno("is-gap", tops[1], uistate.T("nws.annoNet"),
			fmtMoney(money.New(last.NetMinor, v.Base))),
		nwsAnno("is-liab", tops[2], uistate.T("nws.stripOwe"),
			fmtMoney(money.New(last.LiabilitiesMinor, v.Base))),
	)

	// X axis: every point dated, not just the two ends.
	xaxis := []any{css.Class("nws-xaxis"), Attr("data-testid", "nws-xaxis")}
	for i := range pts {
		label := ""
		if i < len(v.Labels) {
			label = v.Labels[i]
		}
		if label == "" {
			label = pts[i].At.Format("Jan")
		}
		xaxis = append(xaxis, Span(css.Class("nws-xtick"), label))
	}

	// The gap MEASURED at both ends, so the story survives even where the wedge
	// is subtle: this is what it was, this is what it is now.
	ends := Div(css.Class("nws-sides-ends"), Attr("data-testid", "nws-gap-ends"),
		Span(css.Class("nws-gap-value"), uistate.T("nws.gapWas",
			fmtMoney(money.New(first.NetMinor, v.Base)))),
		Span(css.Class("nws-gap-value"), uistate.T("nws.gapNow",
			fmtMoney(money.New(last.NetMinor, v.Base)))),
	)

	// The chart answers "how it moved"; the strips answer "what shape it is".
	// They sit SIDE BY SIDE at full width — two questions, two panels, one
	// screen — and stack only when the pane is too narrow to hold both.
	return Div(css.Class("nws-sides"),
		Div(css.Class("nws-sides-plot"),
			Div(css.Class("nws-plot"),
				Svg(kids...),
				Div(yaxis...),
				anno,
			),
			Div(xaxis...),
			ends,
			// A truncated axis is disclosed in words as well as shown on the
			// scale — the same rule THE BRIDGE follows.
			P(css.Class("nws-sec-note"), Style(map[string]string{"margin": "0"}),
				Attr("data-testid", "nws-sides-floor"),
				uistate.T("nws.sidesFloor", fmtMoney(money.New(int64(scale.lo), v.Base)))),
		),
		Div(css.Class("nws-strips"),
			nwsStrip(true, uistate.T("nws.stripOwn"), balancesheet.AssetBuckets,
				last.Assets, last.AssetsMinor, v.Base),
			nwsStrip(false, uistate.T("nws.stripOwe"), balancesheet.LiabilityBuckets,
				last.Liabilities, last.LiabilitiesMinor, v.Base),
		),
	)
}

// nwsAnno renders one in-chart region label: what this part of the picture is,
// and what it currently amounts to.
func nwsAnno(mod string, topPct float64, label, value string) ui.Node {
	return Div(ClassStr("nws-anno "+mod), Style(map[string]string{"top": fmt.Sprintf("%.3f%%", topPct)}),
		Span(css.Class("nws-anno-label"), label),
		Span(css.Class("nws-anno-value"), value),
	)
}

// nwsSpreadLabels keeps the three in-chart labels from landing on top of each
// other when the two boundaries run close together (a household near breakeven),
// preserving their order and staying inside the plot. Positions are in percent.
func nwsSpreadLabels(tops []float64) []float64 {
	const minGap, lo, hi = 17.0, 4.0, 96.0
	out := append([]float64(nil), tops...)
	for i := range out {
		if out[i] < lo {
			out[i] = lo
		}
		if i > 0 && out[i] < out[i-1]+minGap {
			out[i] = out[i-1] + minGap
		}
	}
	// Pushing down may have run the last one off the bottom; walk back up.
	for i := len(out) - 1; i >= 0; i-- {
		if out[i] > hi {
			out[i] = hi
		}
		if i < len(out)-1 && out[i] > out[i+1]-minGap {
			out[i] = out[i+1] - minGap
		}
	}
	return out
}

// nwsStrip renders one side's composition as a 100% bar with its key beneath:
// what the side is made of, right now, at exact figures. Shares are of THIS
// side only — which is the direct fix for a $304k property holding reducing
// every other bar on the page to a stub.
func nwsStrip(asset bool, title string, order []balancesheet.Bucket, amounts map[balancesheet.Bucket]int64, total int64, base string) ui.Node {
	segs := []any{css.Class("nws-strip-bar")}
	keys := []any{css.Class("nws-strip-key")}
	for i, b := range order {
		amt := amounts[b]
		if amt == 0 {
			continue
		}
		share := int64(0)
		if total > 0 {
			share = amt * 100 / total
		}
		tone := nwsToneClass(asset, i)
		segs = append(segs, Div(ClassStr("nws-strip-seg "+tone),
			Attr("title", fmt.Sprintf("%s · %s", nwsBucketLabel(b), fmtMoney(money.New(amt, base)))),
			Style(map[string]string{"width": fmt.Sprintf("%d%%", share)})))
		keys = append(keys, Span(css.Class("nws-legend-item"),
			Span(ClassStr("nws-legend-dot "+tone)),
			Span(uistate.T("nws.legendEntry", nwsBucketLabel(b), fmtMoney(money.New(amt, base)), share)),
		))
	}
	if total == 0 {
		return Fragment()
	}
	return Div(css.Class("nws-strip"), Attr("data-testid", "nws-strip"),
		Div(css.Class("nws-strip-head"),
			Span(ClassStr("nws-strip-swatch "+nwsToneClass(asset, 0))),
			Span(css.Class("nws-strip-title"), title),
			Span(css.Class("nws-strip-total"), fmtMoney(money.New(total, base))),
		),
		Div(segs...),
		Div(keys...),
	)
}

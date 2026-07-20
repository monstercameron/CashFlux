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
// A mirrored area chart: assets stacked UPWARD from the zero line, liabilities
// stacked DOWNWARD, and the net-worth line running through the middle. It
// replaces a smooth trend line that only restated the headline figure. The
// story a household actually has is the shape of the GAP — whether it widens
// because the top is rising or because the bottom is shrinking, and which part
// of each side is doing it.
//
// Both sides are drawn in ONE hue each, stepped by alpha: the theme accent for
// what you own, a neutral for what you owe. Composition therefore reads as two
// sides rather than seven unrelated series, and — deliberately — the liability
// side is NOT red. A mortgage is structure, not an emergency; red stays
// reserved for a genuinely negative net worth and alarm-band ratios.

// nwsBandClass returns the fill class for a bucket at stacking position i.
func nwsBandClass(asset bool, i int) string {
	if asset {
		return fmt.Sprintf("nws-band-a%d", i)
	}
	return fmt.Sprintf("nws-band-l%d", i)
}

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

// nwsSidesScale maps a signed figure into the 0..100 viewBox with the zero line
// wherever the two sides' extents put it — the asset side gets the room it
// needs above, the liability side the room it needs below, so neither is
// squeezed to a sliver by the other.
type nwsSidesScale struct{ hi, lo float64 } // hi = headroom above 0, lo = below

func newNwsSidesScale(pts []balancesheet.Point) nwsSidesScale {
	var maxA, maxL int64
	for _, p := range pts {
		if p.AssetsMinor > maxA {
			maxA = p.AssetsMinor
		}
		if p.LiabilitiesMinor > maxL {
			maxL = p.LiabilitiesMinor
		}
	}
	hi, lo := float64(maxA)*1.08, float64(maxL)*1.08
	if hi <= 0 {
		hi = 1
	}
	if lo <= 0 {
		// A debt-free household still needs a sliver below the line, or the zero
		// axis would sit flush on the floor and read as a cropped chart.
		lo = hi * 0.08
	}
	return nwsSidesScale{hi: hi, lo: lo}
}

// y maps a signed minor amount (positive = asset side) to the viewBox.
func (s nwsSidesScale) y(v float64) float64 {
	span := s.hi + s.lo
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

// nwsBandPath builds one stacked band's closed polygon: the upper cumulative
// edge left-to-right, then the lower cumulative edge back again.
func nwsBandPath(lower, upper []float64, s nwsSidesScale) string {
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
		fmt.Fprintf(&b, "%s%.2f %.3f ", cmd, nwsSidesX(i, n), s.y(upper[i]))
	}
	for i := n - 1; i >= 0; i-- {
		fmt.Fprintf(&b, "L%.2f %.3f ", nwsSidesX(i, n), s.y(lower[i]))
	}
	b.WriteString("Z")
	return b.String()
}

// nwsSides renders the mirrored composition chart plus its legend, its month
// axis, and the text equivalent beneath (every band's current figure and share
// of its own side).
func nwsSides(v nwsView) ui.Node {
	pts := v.Points
	if len(pts) < 2 {
		return P(css.Class("empty"), Attr("data-testid", "nws-sides-empty"), uistate.T("nws.sidesEmpty"))
	}
	scale := newNwsSidesScale(pts)
	n := len(pts)

	kids := []any{
		css.Class("nws-sides-svg"),
		Attr("viewBox", fmt.Sprintf("0 0 %.0f 100", nwsSidesW)),
		Attr("preserveAspectRatio", "none"),
		Attr("role", "img"),
		Attr("data-testid", "nws-sides-svg"),
		Attr("aria-label", uistate.T("nws.sidesAria",
			fmtMoney(money.New(v.Latest().AssetsMinor, v.Base)),
			fmtMoney(money.New(v.Latest().LiabilitiesMinor, v.Base)),
			fmtMoney(money.New(v.Latest().NetMinor, v.Base)))),
	}

	// Asset bands stack upward from zero, most liquid first (so cash sits on the
	// line and the least spendable holdings sit furthest out).
	cum := make([]float64, n)
	for bi, bucket := range balancesheet.AssetBuckets {
		lower := append([]float64(nil), cum...)
		empty := true
		for i, p := range pts {
			if amt := p.Assets[bucket]; amt != 0 {
				empty = false
				cum[i] += float64(amt)
			}
		}
		if empty {
			continue
		}
		if d := nwsBandPath(lower, cum, scale); d != "" {
			kids = append(kids, Path(css.Class(nwsBandClass(true, bi)), Attr("d", d)))
		}
	}
	// Liability bands stack downward, most urgent first.
	for i := range cum {
		cum[i] = 0
	}
	for bi, bucket := range balancesheet.LiabilityBuckets {
		lower := append([]float64(nil), cum...)
		empty := true
		for i, p := range pts {
			if amt := p.Liabilities[bucket]; amt != 0 {
				empty = false
				cum[i] -= float64(amt)
			}
		}
		if empty {
			continue
		}
		if d := nwsBandPath(lower, cum, scale); d != "" {
			kids = append(kids, Path(css.Class(nwsBandClass(false, bi)), Attr("d", d)))
		}
	}

	// The zero line, then the net-worth line on top — the figure the page is
	// named after runs straight through the middle of what makes it.
	zeroY := scale.y(0)
	kids = append(kids, Line(css.Class("nws-sides-zero"),
		Attr("vector-effect", "non-scaling-stroke"),
		Attr("x1", "0"), Attr("x2", fmt.Sprintf("%.0f", nwsSidesW)),
		Attr("y1", fmt.Sprintf("%.3f", zeroY)), Attr("y2", fmt.Sprintf("%.3f", zeroY)),
	))
	netPts := make([]string, 0, n)
	for i, p := range pts {
		netPts = append(netPts, fmt.Sprintf("%.2f,%.3f", nwsSidesX(i, n), scale.y(float64(p.NetMinor))))
	}
	kids = append(kids, Polyline(css.Class("nws-sides-net"),
		Attr("vector-effect", "non-scaling-stroke"),
		Attr("points", strings.Join(netPts, " ")), Attr("fill", "none")))

	// Month axis: the first caption and the last, so the window is stated without
	// crowding the chart with ticks it does not need.
	axis := Fragment()
	if len(v.Labels) >= 2 {
		axis = Div(css.Class("nws-sides-axis"),
			Span(v.Labels[0]),
			Span(v.Labels[len(v.Labels)-1]),
		)
	}

	latest := v.Latest()
	legend := []any{css.Class("nws-sides-legend"), Attr("data-testid", "nws-sides-legend")}
	legend = append(legend, nwsLegendItems(balancesheet.AssetBuckets, latest.Assets, latest.AssetsMinor, true, v.Base)...)
	legend = append(legend, nwsLegendItems(balancesheet.LiabilityBuckets, latest.Liabilities, latest.LiabilitiesMinor, false, v.Base)...)
	legend = append(legend, Span(css.Class("nws-legend-item"),
		Span(css.Class("nws-legend-dot"), Style(map[string]string{"background": "var(--text)"})),
		Span(uistate.T("nws.legendNet"))))

	return Div(css.Class("nws-sides"),
		Svg(kids...),
		axis,
		Div(legend...),
	)
}

// nwsLegendItems builds one side's legend entries, each carrying its current
// figure and its share of ITS OWN side — which is the whole point of
// normalizing within a side: a $304k condo must not reduce every other holding
// to an unreadable stub.
func nwsLegendItems(order []balancesheet.Bucket, amounts map[balancesheet.Bucket]int64, sideTotal int64, asset bool, base string) []any {
	out := make([]any, 0, len(order))
	for i, b := range order {
		amt := amounts[b]
		if amt == 0 {
			continue
		}
		share := int64(0)
		if sideTotal > 0 {
			share = amt * 100 / sideTotal
		}
		out = append(out, Span(css.Class("nws-legend-item"),
			Span(ClassStr("nws-legend-dot is-"+nwsLegendTone(asset, i))),
			Span(uistate.T("nws.legendEntry", nwsBucketLabel(b), fmtMoney(money.New(amt, base)), share)),
		))
	}
	return out
}

// nwsLegendTone maps a stacking position to its legend swatch modifier, mirroring
// the band fills exactly.
func nwsLegendTone(asset bool, i int) string {
	if asset {
		return fmt.Sprintf("a%d", i)
	}
	return fmt.Sprintf("l%d", i)
}

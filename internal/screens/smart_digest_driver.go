// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens — SmartDigestDriver and SmartDigestSection implement the
// SMART proactive digest: a cadence-driven summary posted to the notification
// feed. The driver is a headless component mounted ONCE in the app Shell so
// its UseEffect hook is always at a constant depth (On*-hooks-in-loops rule).
package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartdigest"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// digestCode is the stable catalog code for the proactive digest feature.
const digestCode = "SMART-DIGEST"

// SmartDigestDriver is a headless component (renders nothing) that fires the
// proactive digest when it is due. Mounting it once in the Shell ensures the
// hook tree depth is constant.
//
// Guard conditions (ALL must hold to post a digest):
//  1. The feature is enabled in SMART settings.
//  2. The density dial is not Off (Off silences all proactive surfaces).
//  3. The feature's cadence is Due(lastRun, now, dataChanged=false, appOpen=true).
//  4. There are active insights to summarise (empty → nothing to say).
//  5. The period key has not already been delivered (DeliveredLog dedup).
//
// The driver stamps LastRun BEFORE building the digest so the effect-key
// (which embeds LastRun) changes immediately, preventing re-entry within the
// same cadence window — at most one digest per due window.
func SmartDigestDriver(_ struct{}) ui.Node {
	_ = uistate.UseDataRevision().Get() // re-render on data/settings change

	settings := uistate.LoadSmartSettings()
	cad := settings.CadenceFor(digestCode)
	last := settings.LastRunAt(digestCode)

	// Effect key: changes when LastRun changes (after the stamp), so the effect
	// cannot re-enter within the same cadence window.
	effectKey := "smart-digest|" + string(cad) + "|" + strconv.FormatInt(last.Unix(), 10)

	ui.UseEffect(func() func() {
		// Re-read inside the effect — render-time reads can be stale.
		s := uistate.LoadSmartSettings()
		if !s.IsEnabled(digestCode) {
			return nil
		}
		if s.DensityOrDefault() == smart.DensityOff {
			return nil
		}

		cadInner := s.CadenceFor(digestCode)
		lastInner := s.LastRunAt(digestCode)
		now := time.Now()

		// Due check — appOpen=true so on_open cadence fires on each mount.
		if !cadInner.Due(lastInner, now, false, true) {
			return nil
		}

		app := appstate.Default
		if app == nil {
			return nil
		}

		// Stamp BEFORE building — changes the effect key so we cannot re-enter.
		uistate.MarkSmartRun(digestCode, now)

		// Gather active insights from all enabled engines (all pages).
		weekStart := uistate.CurrentPrefs().WeekStartWeekday()
		in := buildSmartInput(app, weekStart)
		allInsights := smartengine.Run(in, s)

		// Load the delivered log from persisted storage.
		deliveredLog := uistate.LoadDigestDeliveredLog()

		// Build the digest. Returns ok=false when empty or already delivered.
		item, ok := smartdigest.Build(allInsights, now, cadInner, deliveredLog)
		if !ok {
			return nil
		}

		// Persist the updated delivered log (marks this period as delivered).
		uistate.SaveDigestDeliveredLog(deliveredLog)

		// Post to the notification feed. PrependNotifyFeed's ID-based dedup
		// provides a second line of defence against double-posting.
		uistate.PrependNotifyFeed([]uistate.FeedItem{
			{
				ID:    item.ID,
				Title: item.Title,
				Body:  item.Body,
				At:    item.At,
			},
		})
		// Bump the data revision so the notification bell badge updates.
		uistate.BumpDataRevision()

		return nil
	}, effectKey)

	return Fragment()
}

// --- /smart hub UI -----------------------------------------------------------

// digestRowProps carries the opt-in and cadence state for the digest control row.
type digestRowProps struct {
	On  bool
	Cad smart.Cadence
}

// smartDigestRow is the interactive row for the digest: toggle on the right,
// cadence picker in between when enabled. Its own component so the On* hooks
// (UseEvent for cadence) sit at stable positions.
func smartDigestRow(props digestRowProps) ui.Node {
	rev := uistate.UseDataRevision()

	onChange := func(on bool) {
		uistate.SetSmartFeatureEnabled(digestCode, on)
		rev.Set(rev.Get() + 1)
	}
	onCadence := ui.UseEvent(func(v string) {
		uistate.SetSmartCadence(digestCode, smart.Cadence(v))
		rev.Set(rev.Get() + 1)
	})

	// Digest-appropriate cadences: On app open, Daily, Weekly, Monthly.
	// Live/Manual are excluded — Live would post on every render (too noisy);
	// Manual never auto-fires (defeating the purpose of a proactive digest).
	digestCadences := []smart.Cadence{
		smart.CadenceOnOpen,
		smart.CadenceDaily,
		smart.CadenceWeekly,
		smart.CadenceMonthly,
	}

	var cadencePicker ui.Node = Fragment()
	if props.On {
		cadencePicker = Select(ClassStr("field "+tw.Fold(tw.Text12)),
			Attr("data-testid", "smart-digest-cadence"),
			Attr("aria-label", "How often to post a digest"),
			OnChange(onCadence),
			MapKeyed(digestCadences,
				func(c smart.Cadence) any { return string(c) },
				func(c smart.Cadence) ui.Node {
					return Option(Value(string(c)), SelectedIf(c == props.Cad), c.Label())
				},
			),
		)
	}

	return Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap3, tw.Py2, tw.BorderB, tw.BorderLine)),
		Attr("data-testid", "smart-feature-"+digestCode),
		Div(ClassStr(tw.Fold(tw.FlexCol, tw.Gap1, tw.MinW0)),
			Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap2)),
				Span(ClassStr(tw.Fold(tw.Text14, tw.FontMedium)), "Proactive money digest"),
				Span(ClassStr(tw.Fold(tw.Text11, tw.FontMedium, tw.TextUp, tw.BgUp, tw.Px1, tw.Py05, tw.Rounded)),
					"Free",
				),
			),
			Span(ClassStr(tw.Fold(tw.Text12, tw.TextDim)),
				"Post a brief summary of your top active insights to the notification feed.",
			),
		),
		Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap3)),
			cadencePicker,
			uiw.Toggle(uiw.ToggleProps{
				On:       props.On,
				OnChange: onChange,
				Label:    "Proactive money digest",
			}),
		),
	)
}

// SmartDigestSection renders the opt-in card for the proactive digest on the
// /smart hub, placed near the manage controls. It uses the same visual style as
// the manage section but is separate since SMART-DIGEST is a cross-app
// meta-feature (PageHub) not tied to any single data page.
func SmartDigestSection(settings smart.Settings) ui.Node {
	on := settings.IsEnabled(digestCode)
	cad := settings.CadenceFor(digestCode)
	// The tier default for a Free feature is Live — not appropriate for a
	// notification digest. Display Weekly as the sensible default so the picker
	// is pre-set on first enable.
	if cad == smart.CadenceLive || cad == smart.CadenceManual {
		cad = smart.CadenceWeekly
	}

	return uiw.Card(uiw.CardProps{
		Header: smartBrandHeader("Digest", false, nil),
		TestID: "smart-digest-section",
		Body: Div(ClassStr(tw.Fold(tw.FlexCol, tw.Gap2)),
			P(ClassStr(tw.Fold(tw.Text13, tw.TextDim)),
				"Get a brief summary of your top money insights posted to your notification feed, "+
					"on a schedule you choose. Strictly opt-in — nothing posts until you enable it.",
			),
			ui.CreateElement(smartDigestRow, digestRowProps{On: on, Cad: cad}),
		),
	})
}

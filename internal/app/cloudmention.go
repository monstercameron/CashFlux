// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// cloudMentionSnoozedKey is the browserstore key holding the Unix timestamp (seconds)
// of the most recent snooze. An empty value means the banner has never been snoozed.
// Legacy values of "1" (from the old permanent-dismiss scheme) parse as 1 — the epoch
// of 1970-01-01 — which is far more than cloudMentionSnoozeDays ago, so the banner
// re-surfaces correctly on upgrade without any migration code.
const (
	cloudMentionSnoozedKey = "cashflux:cloud-mention-snoozed"
	cloudMentionSnoozeDays = 30 // re-surface the banner after this many days
)

// cloudMentionSnoozed returns true when the stored snooze timestamp is within the
// last cloudMentionSnoozeDays days. An empty or unparseable value is treated as
// "never snoozed" (returns false). The legacy "1" value (old permanent dismiss)
// parses as Unix epoch 1970-01-01, which is always older than 30 days → false.
func cloudMentionSnoozed() bool {
	raw := lsGet(cloudMentionSnoozedKey)
	if raw == "" {
		return false
	}
	ts, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return false // unrecognised value: treat as not snoozed
	}
	snoozedAt := time.Unix(ts, 0)
	return time.Since(snoozedAt) < cloudMentionSnoozeDays*24*time.Hour
}

// CloudMention introduces the optional CashFlux Cloud tier (sync + backup + AI
// proxy) without nagging (§7.11). Per the 2026-07-17 visual audit it is a single
// compact rail row — icon + label + a small ✕ — never a multi-line promo card, so
// it can't stack against the primary navigation at short viewport heights or
// collapse into a one-word-per-line slab on the narrow mobile rail (the CSS hides
// it entirely there). The row navigates to the /plans comparison surface and
// snoozes itself; the ✕ snoozes for 30 days without navigating. After the snooze
// window expires the row re-surfaces automatically — dismissing is a snooze, not a
// permanent opt-out. Its own component so the snooze + atom hooks stay at a stable
// render position (§7.11, R31-reengage).
func CloudMention() uic.Node {
	snoozed := uic.UseState(cloudMentionSnoozed())

	// Hide while snoozed, or if Cloud sync is already in use (any sync status set).
	if snoozed.Get() || loadSyncStatus().State != "" {
		return Fragment()
	}

	snooze := func() {
		lsSet(cloudMentionSnoozedKey, strconv.FormatInt(time.Now().Unix(), 10))
		snoozed.Set(true)
	}

	// R31-reengage: following the row to /plans also snoozes it so it doesn't
	// re-appear immediately; navigation is handled by the anchor href.
	onLearn := uic.UseEvent(func() { snooze() })
	onDismiss := uic.UseEvent(func() { snooze() })

	return Div(css.Class("cloud-mention", tw.Flex, tw.ItemsCenter, tw.Gap15),
		Attr("role", "note"),
		Attr("data-testid", "cloud-mention"),
		A(css.Class("cloud-mention-link", tw.Flex, tw.ItemsCenter, tw.Gap15, tw.Flex1, tw.MinW0),
			Attr("href", uistate.RoutePath("/plans")),
			Attr("title", uistate.T("cloud.rowTitle")),
			OnClick(onLearn),
			ui.Icon(icon.Cloud, css.Class(tw.W4, tw.H4, tw.ShrinkO)),
			Span(css.Class("cloud-mention-label", tw.Truncate), uistate.T("cloud.rowLabel")),
		),
		Button(css.Class("cloud-mention-x"), Type("button"),
			Attr("title", uistate.T("cloud.mentionDismiss")),
			Attr("aria-label", uistate.T("cloud.mentionDismiss")),
			Attr("data-testid", "cloud-mention-dismiss"),
			OnClick(onDismiss),
			ui.Icon(icon.Close, css.Class(tw.W35, tw.H35)),
		),
	)
}

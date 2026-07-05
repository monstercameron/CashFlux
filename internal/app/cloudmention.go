// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strconv"
	"time"

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

// CloudMention is a calm, dismissible banner that introduces the optional CashFlux
// Cloud tier (sync + backup + AI proxy) without nagging (§7.11). It shows only
// when the user hasn't snoozed it within the last 30 days and isn't already
// syncing. "Learn more" navigates to the Plans comparison surface (/plans);
// "Not now" snoozes for 30 days.
// After the snooze window expires the banner re-surfaces automatically — dismissing
// it is a snooze, not a permanent opt-out, so the upgrade path is never permanently
// buried. Its own component so the snooze + atom hooks stay at a stable render
// position (§7.11, R31-reengage).
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

	onDismiss := uic.UseEvent(func() { snooze() })
	// R31-reengage: snooze the banner when the user clicks "Learn more" so it
	// doesn't re-appear immediately; navigation to /plans is handled by the
	// anchor href — no imperative router call needed.
	onLearn := uic.UseEvent(func() { snooze() })

	return Div(css.Class("cloud-mention", tw.Flex, tw.FlexCol, tw.Gap1),
		Attr("role", "note"),
		P(css.Class("cloud-mention-title"), uistate.T("cloud.mentionTitle")),
		P(css.Class("cloud-mention-body", tw.Text12, tw.TextDim), uistate.T("cloud.mentionBody")),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1),
			// R31-reengage: "Learn more" navigates to the permanent /plans
			// comparison surface (replacing the one-shot ShowUpgradeSheet call),
			// so full pricing is always one step away without consuming a modal.
			// ShowUpgradeSheet still fires from gated Cloud actions (§7.11).
			A(css.Class("btn", "btn-sm"), Attr("href", uistate.RoutePath("/plans")),
				OnClick(onLearn), uistate.T("cloud.mentionLearn")),
			Button(css.Class("btn", "btn-sm", tw.TextFaint), Type("button"), OnClick(onDismiss), uistate.T("cloud.mentionDismiss")),
		),
	)
}

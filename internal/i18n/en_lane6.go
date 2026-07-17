// SPDX-License-Identifier: MIT

package i18n

// lane6Keys holds English strings added by the 2026-07-17 lane-6 remediation
// batch (subscriptions honesty, notifications polish, assistant restructure,
// detection confidence, a11y). Merged via init so the shared en.go is never
// touched by this concurrent lane.
var lane6Keys = Catalog{
	// #49 — local-first "How to cancel": the button files the on-device
	// checklist; the external search is a small secondary link.
	"subs.guideBtnTitle": "File a step-by-step cancellation checklist as a to-do",
	"subs.guideTitle":    "Cancel %s — step-by-step checklist",
	"subs.guideAdded":    "Added a cancellation checklist for %s to your to-dos.",
	"subs.webSearch":     "Web search ↗",

	// #75 — notifications polish: human due-dates, a guarded Clear all, and the
	// labeled mobile action overflow.
	"notify.billBodyToday":          "Due today.",
	"notify.billBodyTomorrow":       "Due tomorrow.",
	"notifications.clearAllConfirm": "Clear all %d notifications? This removes read, unread, and snoozed alerts.",
	"notifications.clearedNotice":   "All notifications cleared.",
	"notifications.moreActions":     "More actions",
	"notifications.alertSettings":   "Alert settings",

	// #74 (UX-10) — the to-do suggestion counts the SAME population as the
	// transactions page's Review inbox and says so with the same name.
	"todo.suggestReviewInbox": "Review %d transactions in the Review inbox",

	// #73 (UX-09) — assistant restructure: settings drawer, aside drawer,
	// composer-adjacent privacy + scope, honest cost.
	"assistant.chatSettings":    "Chat settings",
	"assistant.notesDrawer":     "Notes & chats",
	"assistant.nextScope":       "Next message sends %s from this device · ~%d tokens",
	"insights.usageCostUnknown": "Used %d tokens · cost unavailable",

	// #52 — detection confidence tiers + review inbox.
	"subs.confConfirmed":    "Confirmed",
	"subs.confLikely":       "Likely",
	"subs.confReview":       "Needs review",
	"subs.confAria":         "Detection confidence: %s. Why: %s",
	"subs.confExcluded":     "Excludes %d detections awaiting your review below.",
	"subs.reviewInboxTitle": "Review detections",
	"subs.reviewInboxDesc":  "These patterns look like subscriptions, but the evidence is thin. Confirm the real ones; reject the rest and they stay out of your totals.",
	"subs.confirmBtn":       "Confirm",
	"subs.confirmTitle":     "Confirm this is a real subscription",
	"subs.confirmedNotice":  "Confirmed %s as a subscription.",

	// #68 — shared five-state visual vocabulary + first-class density preference.
	"state.healthy":               "Healthy",
	"state.watch":                 "Watch",
	"state.action":                "Action needed",
	"state.blocked":               "Blocked",
	"state.unconfirmed":           "Unconfirmed",
	"settings.densityLabel":       "Density",
	"settings.densityComfortable": "Comfortable",
	"settings.densityCompact":     "Compact",
	"settings.densityHint":        "Compact tightens rows, cards, and controls across the whole app — the same setting the theme editor and the budgets list use.",
}

func init() {
	for k, v := range lane6Keys {
		english[k] = v
	}
}

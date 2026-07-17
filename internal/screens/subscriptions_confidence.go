// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// confirmedSubsKey is the settings-KV slot holding the user-confirmed
// subscription names (task #52): a JSON array of subscriptions.ConfirmKey'd
// names. Confirmations ride the dataset persistence like every other setting.
const confirmedSubsKey = "cashflux:subs:confirmed"

// loadConfirmedSubs reads the persisted confirmed-name set.
func loadConfirmedSubs() map[string]bool {
	out := map[string]bool{}
	raw := uistate.KVGet(confirmedSubsKey)
	if raw == "" {
		return out
	}
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		return out
	}
	for _, n := range names {
		out[n] = true
	}
	return out
}

// addConfirmedSub persists one more confirmed name (idempotent) and requests
// an immediate dataset persist so the confirmation survives a reload.
func addConfirmedSub(name string) {
	set := loadConfirmedSubs()
	set[subscriptions.ConfirmKey(name)] = true
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	if b, err := json.Marshal(names); err == nil {
		uistate.KVSet(confirmedSubsKey, string(b))
		uistate.RequestPersist()
	}
}

// subConfidenceLabel is the visible tier wording.
func subConfidenceLabel(level subscriptions.Confidence) string {
	switch level {
	case subscriptions.ConfidenceConfirmed:
		return uistate.T("subs.confConfirmed")
	case subscriptions.ConfidenceLikely:
		return uistate.T("subs.confLikely")
	default:
		return uistate.T("subs.confReview")
	}
}

// subConfidenceChip renders a detection's tier as a small labeled chip whose
// tooltip/aria carry the WHY (the concrete evidence line) — the tier is always
// explainable in place. Pure node builder, no hooks.
func subConfidenceChip(a subscriptions.Assessment, slug string) ui.Node {
	label := subConfidenceLabel(a.Level)
	why := a.ReasonLine()
	return Span(ClassStr("conf-chip conf-"+string(a.Level)),
		Attr("data-testid", "sub-conf-"+slug),
		Title(why),
		Attr("aria-label", uistate.T("subs.confAria", label, why)),
		label)
}

// subsReviewInboxProps drives the "Review detections" surface: every detection
// the assessor graded Review, with Confirm / Not-a-subscription resolutions.
type subsReviewInboxProps struct {
	Subs      []subscriptions.Subscription
	Assess    func(subscriptions.Subscription) subscriptions.Assessment
	Base      string
	OnConfirm func(name string)
	OnReject  func(name string)
}

// SubsReviewInbox lists the needs-review detections with their evidence and
// one-click resolutions. Rows are their own components (hook rule).
func SubsReviewInbox(props subsReviewInboxProps) ui.Node {
	rows := MapKeyed(props.Subs,
		func(s subscriptions.Subscription) any { return "conf-review|" + s.Name },
		func(s subscriptions.Subscription) ui.Node {
			return ui.CreateElement(subsReviewInboxRow, subsReviewRowProps{
				Sub: s, Why: props.Assess(s).ReasonLine(), Base: props.Base,
				OnConfirm: props.OnConfirm, OnReject: props.OnReject,
			})
		},
	)
	return Fragment(
		P(css.Class("row-meta"), uistate.T("subs.reviewInboxDesc")),
		Div(css.Class("rows rec-cardrows"), Attr("data-testid", "subs-review-inbox"), rows),
	)
}

type subsReviewRowProps struct {
	Sub       subscriptions.Subscription
	Why       string
	Base      string
	OnConfirm func(string)
	OnReject  func(string)
}

func subsReviewInboxRow(props subsReviewRowProps) ui.Node {
	s := props.Sub
	slug := nameSlug(s.Name)
	confirm := ui.UseEvent(Prevent(func() {
		if props.OnConfirm != nil {
			props.OnConfirm(s.Name)
		}
	}))
	reject := ui.UseEvent(Prevent(func() {
		if props.OnReject != nil {
			props.OnReject(s.Name)
		}
	}))
	return Div(css.Class("row"), Attr("data-testid", "subs-review-row-"+slug),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), s.Name),
			Span(css.Class("row-meta"), props.Why),
		),
		Span(css.Class("budget-amount"), fmtMoney(money.New(s.Amount, props.Base))),
		Div(css.Class("sub-actions"),
			Button(css.Class("btn btn-sm btn-primary"), Type("button"),
				Attr("data-testid", "subs-review-confirm-"+slug),
				Title(uistate.T("subs.confirmTitle")),
				Attr("aria-label", uistate.T("subs.confirmTitle")+" "+s.Name),
				OnClick(confirm), uistate.T("subs.confirmBtn")),
			Button(css.Class("btn btn-sm"), Type("button"),
				Attr("data-testid", "subs-review-reject-"+slug),
				Title(uistate.T("subs.ignoreTitle")),
				Attr("aria-label", uistate.T("subs.ignoreTitle")+" "+s.Name),
				OnClick(reject), uistate.T("subs.ignore")),
		),
	)
}

// subsConfExcludedNote is the headline-honesty caption (task #52): shown under
// the hero whenever needs-review detections are excluded from the totals.
func subsConfExcludedNote(excluded int) ui.Node {
	if excluded == 0 {
		return Fragment()
	}
	return P(css.Class("row-meta "+tw.Fold(tw.Text12)), Attr("data-testid", "subs-conf-excluded"),
		uistate.T("subs.confExcluded", excluded))
}

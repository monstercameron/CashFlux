// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// notifMemberLensProps drives the Notification Center's member filter (C407).
type notifMemberLensProps struct {
	Members  []domain.Member
	Selected string
	OnPick   func(string)
}

// notifMemberLens is the "whose alerts" filter. Its own component because it
// owns a change hook and sits inside a summary tile that returns early.
//
// Picking a member does NOT hide household-wide alerts — see
// FeedItem.MatchesMemberFilter. A lens that hid unassigned work would let an
// overdue bill nobody claimed go permanently unseen in a household where each
// person uses their own view.
func notifMemberLens(props notifMemberLensProps) ui.Node {
	onChange := ui.UseEvent(func(e ui.Event) {
		if props.OnPick != nil {
			props.OnPick(e.GetValue())
		}
	})
	opts := []ui.Node{Option(Value(""), SelectedIf(props.Selected == ""),
		uistate.T("notifications.memberAll"))}
	for _, m := range props.Members {
		opts = append(opts, Option(Value(m.ID), SelectedIf(props.Selected == m.ID), m.Name))
	}
	return Select(css.Class("field notif-member-lens"), Attr("data-testid", "notif-member-lens"),
		Attr("aria-label", uistate.T("notifications.memberLensLabel")), OnChange(onChange), opts)
}

// notifMemberChip names the member an alert belongs to, on the row (C407).
// Household-wide alerts get no chip: "everyone" is the default, and a chip on
// every row would be noise that makes the assigned ones harder to spot.
func notifMemberChip(memberID string, members []domain.Member) ui.Node {
	if memberID == "" {
		return Fragment()
	}
	for _, m := range members {
		if m.ID == memberID {
			return Span(css.Class("notif-member-chip"), Attr("data-testid", "notif-member-"+memberID),
				Attr("title", uistate.T("notifications.memberChipTitle", m.Name)), m.Name)
		}
	}
	// A member who has been deleted still leaves alerts behind. Saying so beats
	// showing a stale name or silently dropping the routing.
	return Span(css.Class("notif-member-chip"), Attr("data-testid", "notif-member-unknown"),
		uistate.T("notifications.memberGone"))
}

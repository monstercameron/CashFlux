// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/memberimpact"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// members_lane5.go holds the #66 household-clarity pieces: the plain-English
// roles-and-ownership explainer, the read-only "what changes when this member
// leaves" preview, and small shared helpers other surfaces use for ownership
// wording.

// ownerDisplayName resolves an owner id to its display name: a member's name,
// or "Group (shared)" for the shared owner / an unset id.
func ownerDisplayName(members []domain.Member, id string) string {
	if id == "" || id == domain.GroupOwnerID {
		return uistate.T("owner.group")
	}
	for _, m := range members {
		if m.ID == id {
			return m.Name
		}
	}
	return uistate.T("owner.group")
}

// memberRoleExplainKey maps a role value to its plain-English explainer key.
func memberRoleExplainKey(role string) string {
	switch domain.MemberRole(role) {
	case domain.RoleOwner:
		return "members.roleExplainOwner"
	case domain.RoleViewer:
		return "members.roleExplainViewer"
	default:
		return "members.roleExplainAdmin"
	}
}

// membersRolesExplainer is the collapsed "How roles & ownership work" note on
// the household page: what each role can see and change, and how account
// ownership, shared ownership, and the transaction member differ. Disclosure so
// the page stays calm for households who already know.
func membersRolesExplainer(_ struct{}) ui.Node {
	open := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	head := Button(css.Class("goal-plan-toggle"), Type("button"),
		Attr("data-testid", "members-roles-explain-toggle"),
		Attr("aria-expanded", ariaBool(open.Get())), OnClick(toggle),
		uiw.Icon(icon.Users, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
		Span(uistate.T("members.rolesExplainToggle")))
	if !open.Get() {
		return Div(css.Class(tw.Mb2), head)
	}
	roleLine := func(titleKey, bodyKey string) ui.Node {
		return Li(css.Class(tw.Mb1),
			Strong(uistate.T(titleKey)), Span(" — "), Span(uistate.T(bodyKey)))
	}
	return Div(css.Class(tw.Mb2), head,
		Div(css.Class("rpt-headsup", tw.Mt2), Attr("data-testid", "members-roles-explain"),
			Ul(css.Class(tw.Mt1),
				roleLine("members.roleOwner", "members.roleExplainOwner"),
				roleLine("members.roleAdmin", "members.roleExplainAdmin"),
				roleLine("members.roleViewer", "members.roleExplainViewer"),
			),
			P(css.Class("muted", tw.Text12, tw.Mt2), Attr("data-testid", "members-ownership-explain"),
				uistate.T("members.ownershipExplain")),
		),
	)
}

// membersLeavePreview is the read-only "What changes when a member leaves?"
// panel (#66): pick a member and see exactly which accounts, shares, budgets,
// goals, and tagged transactions would need reassignment — the same facts the
// reassign-before-delete flow acts on, previewed without touching anything.
func membersLeavePreview(_ struct{}) ui.Node {
	_ = uistate.UseDataRevision().Get()
	sel := ui.UseState("")
	onSel := ui.UseEvent(func(e ui.Event) { sel.Set(e.GetValue()) })
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	members := app.Members()
	if len(members) == 0 {
		return Fragment()
	}
	opts := []ui.Node{Option(Value(""), SelectedIf(sel.Get() == ""), uistate.T("members.leavePickPlaceholder"))}
	for _, m := range members {
		opts = append(opts, Option(Value(m.ID), SelectedIf(sel.Get() == m.ID), m.Name))
	}
	var detail ui.Node = Fragment()
	if id := sel.Get(); id != "" {
		b := memberimpact.Compute(id, app.Accounts(), app.Budgets(), app.Goals(), app.Transactions())
		name := ownerDisplayName(members, id)
		line := func(testid, key string, names []string) ui.Node {
			if len(names) == 0 {
				return Fragment()
			}
			return Li(Attr("data-testid", testid),
				Span(uistate.T(key, len(names))), Span(css.Class(tw.TextDim), " — "+strings.Join(names, ", ")))
		}
		var body ui.Node
		if b.Empty() {
			body = P(css.Class("muted"), Attr("data-testid", "member-leave-empty"),
				uistate.T("members.leaveNothing", name))
		} else {
			body = Fragment(
				P(Attr("data-testid", "member-leave-total"), uistate.T("members.leaveIntro", name, b.Total())),
				Ul(css.Class(tw.Mt1),
					line("member-leave-accounts", "members.leaveAccounts", b.AccountsOwned),
					line("member-leave-shares", "members.leaveShares", b.AccountShares),
					line("member-leave-budgets", "members.leaveBudgets", b.Budgets),
					line("member-leave-goals", "members.leaveGoals", b.Goals),
					If(b.TxnCount > 0, Li(Attr("data-testid", "member-leave-txns"), uistate.T("members.leaveTxns", b.TxnCount))),
				),
				P(css.Class("muted", tw.Text12, tw.Mt2), uistate.T("members.leaveNote")),
			)
		}
		detail = Div(css.Class("rpt-headsup", tw.Mt2), Attr("data-testid", "member-leave-preview"), body)
	}
	return Div(css.Class(tw.Mt3),
		Label(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Span(css.Class("muted", tw.Text12), uistate.T("members.leaveTitle")),
			Select(css.Class("field"), Attr("data-testid", "member-leave-select"),
				Attr("aria-label", uistate.T("members.leaveTitle")), OnChange(onSel), opts),
		),
		detail,
	)
}

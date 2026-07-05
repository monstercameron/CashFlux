// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/memberrole"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Members is the standalone /members route: the person roster plus the
// per-person analytics sections (worth / spending / income split). The
// /household hub renders the same roster via membersBody and keeps the
// analytics on its own "By person" tab.
func Members() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	roster := rptSection("sec-people", uistate.T("members.listTitle"), nil, membersBody())
	f := hhFiguresNow(app, app.Members())
	args := []any{roster}
	if len(app.Members()) > 0 {
		args = append(args, byPersonSections(f)...)
	}
	return Div(args...)
}

// membersBody renders the person-roster body: the orientation copy, the
// reassign-before-delete panel, and one hh-person ledger row per member (with
// inline edit, PIN management, and the overflow menu). It registers hooks, so
// callers must invoke it at a stable render position.
func membersBody() ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	rev := state.UseAtom("rev:members", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	errMsg := ui.UseState("")
	reassignID := ui.UseState("") // member awaiting reassignment before delete
	reassignTo := ui.UseState(domain.GroupOwnerID)

	onReassignTo := ui.UseEvent(func(e ui.Event) { reassignTo.Set(e.GetValue()) })
	onAddMember := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("member") }))

	ownedCount := func(memberID string) int {
		owned := 0
		for _, a := range app.Accounts() {
			if a.OwnerID == memberID {
				owned++
			} else if _, ok := a.OwnershipShares[memberID]; ok {
				// Ghost-member guard: also count membership as a fractional-ownership
				// share holder. Without this, a deleted member leaves dangling keys in
				// OwnershipShares on all accounts where it was a co-owner (C279 delta).
				owned++
			}
		}
		for _, b := range app.Budgets() {
			if b.OwnerID == memberID {
				owned++
			}
		}
		for _, g := range app.Goals() {
			if g.OwnerID == memberID {
				owned++
			}
		}
		// Transactions can be assigned to a member directly (MemberID), independent
		// of account ownership. Count them too, so a member used only as a
		// transaction tag still routes through reassign-before-delete (which clears
		// those MemberIDs) instead of being deleted out from under them.
		for _, t := range app.Transactions() {
			if t.MemberID == memberID {
				owned++
			}
		}
		return owned
	}

	deleteMember := func(memberID string) {
		// If the member owns entities, open the reassign panel; otherwise delete now.
		if ownedCount(memberID) > 0 {
			reassignID.Set(memberID)
			reassignTo.Set(domain.GroupOwnerID)
			errMsg.Set("")
			return
		}
		if err := app.DeleteMember(memberID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	cancelReassign := ui.UseEvent(Prevent(func() { reassignID.Set("") }))
	confirmReassign := ui.UseEvent(Prevent(func() {
		from := reassignID.Get()
		to := reassignTo.Get()
		if _, err := app.DeleteMemberAfterReassign(from, to); err != nil {
			errMsg.Set(err.Error())
			return
		}
		reassignID.Set("")
		errMsg.Set("")
		bump()
	}))

	setDefault := func(memberID string) {
		if err := app.SetDefaultMember(memberID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		bump()
	}

	memberDefs := app.CustomFieldDefsFor("member")

	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	viewTransactions := func(memberID string) {
		f := uistate.TxFilter{Member: memberID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	members := app.Members()
	f := hhFiguresNow(app, members)

	// The share denominator: the largest absolute per-owner worth, so the bars
	// rank people against the household's biggest holder.
	var maxAbs int64
	for _, m := range members {
		if a := f.ownerWorth(m.ID).Amount; a > maxAbs {
			maxAbs = a
		} else if -a > maxAbs {
			maxAbs = -a
		}
	}

	renderRow := func(m domain.Member) ui.Node {
		mID := m.ID // capture for closures
		worth := f.ownerWorth(mID)
		abs := worth.Amount
		if abs < 0 {
			abs = -abs
		}
		pct := 0
		if maxAbs > 0 {
			pct = int(abs * 100 / maxAbs)
		}
		return ui.CreateElement(MemberRow, memberRowProps{
			Member:       m,
			Worth:        worth,
			Spent:        money.New(f.SpendByID[mID], f.Base),
			SharePct:     pct,
			CustomLine:   customSummary(memberDefs, m.Custom),
			OnDelete:     deleteMember,
			OnSetDefault: setDefault,
			OnView:       viewTransactions,
			// C274: per-member PIN management (set/change opens the flip modal).
			MemberHasPIN: app.MemberHasPIN(mID),
			OnClearPIN: func(id string) {
				app.ClearMemberPIN(id)
				bump()
			},
		})
	}
	keyOf := func(m domain.Member) any { return m.ID }

	// When the reassign panel opens, move focus to its target select so a
	// keyboard user lands on the choice they must make (L-quickhit #47).
	ui.UseEffect(func() func() {
		if reassignID.Get() != "" {
			focusByID("member-reassign")
		}
		return nil
	}, reassignID.Get())

	// Reassign-before-delete panel, shown when a member who owns entities is deleted.
	reassignPanel := Fragment()
	if rid := reassignID.Get(); rid != "" {
		var targetName string
		opts := []ui.Node{Option(Value(domain.GroupOwnerID), SelectedIf(reassignTo.Get() == domain.GroupOwnerID), uistate.T("owner.group"))}
		for _, m := range members {
			if m.ID == rid {
				targetName = m.Name
				continue
			}
			opts = append(opts, Option(Value(m.ID), SelectedIf(reassignTo.Get() == m.ID), m.Name))
		}
		reassignPanel = Div(css.Class("rpt-headsup", tw.Mb2),
			H3(css.Class(tw.Mb1), uistate.T("members.reassignTitle")),
			P(css.Class("muted"), uistate.T("members.reassignDesc", targetName, ownedCount(rid))),
			Form(css.Class("form-grid"), OnSubmit(confirmReassign),
				Select(css.Class("field"), Attr("id", "member-reassign"), Attr("aria-label", uistate.T("members.reassignTitle")), OnChange(onReassignTo), opts),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("members.moveAndDelete")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelReassign), uistate.T("action.cancel")),
			),
		)
	}

	return Fragment(
		reassignPanel,
		If(errMsg.Get() != "", P(css.Class("notice-danger"), errMsg.Get())),
		P(css.Class("muted"), uistate.T("members.desc")),
		IfElse(len(members) == 0,
			ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("members.empty"), CTALabel: uistate.T("members.addFirst"), AddTarget: "member"}),
			hhRowsList(MapKeyed(members, keyOf, renderRow)),
		),
		Div(css.Class(tw.Mt2),
			Button(css.Class("btn"), Type("button"), OnClick(onAddMember), uistate.T("members.add")),
		),
		// C274: single-device disclosure — clarifies that roles are labels on a
		// shared local dataset, not per-member logins.
		P(css.Class("muted", tw.Text12, tw.Mt2), Attr("data-testid", "members-single-device-note"), uistate.T("members.singleDeviceNote")),
	)
}

// memberAvatar is a colored initial avatar (the member's first letter on a
// disc tinted with their color), for scannability and a touch of personality (C62).
// Decorative — the member name follows as text — so it's aria-hidden. Inline-styled
// base look (size is overridden by the roster's .hh-person rules); falls back to
// the accent when no color is set.
func memberAvatar(name, color string) ui.Node {
	initial := "?"
	if t := strings.TrimSpace(name); t != "" {
		initial = strings.ToUpper(string([]rune(t)[0]))
	}
	// Pick the initial's color for contrast: white hard-coded on any member color failed on
	// light fills (white on green-400 = 1.74:1, on pink-400 = 2.65:1). readableTextOn flips to
	// dark text on light avatars. No-color members get a branded accent disc (white always passes).
	bg, text := color, "#ffffff"
	if strings.TrimSpace(bg) == "" {
		bg = "var(--accent)"
	} else if strings.HasPrefix(strings.TrimSpace(bg), "#") {
		text = readableTextOn(bg)
	}
	return Span(css.Class("member-avatar"), Attr("aria-hidden", "true"),
		Style(map[string]string{
			"display": "inline-flex", "align-items": "center", "justify-content": "center",
			"width": "1.5rem", "height": "1.5rem", "border-radius": "50%",
			"background": bg, "color": text, "font-weight": "700",
			"vertical-align": "middle", "flex-shrink": "0",
		}),
		initial,
	)
}

// readableTextOn returns "#1c1c1e" or "#ffffff" — whichever has more contrast against the
// given hex fill (WCAG relative-luminance threshold ~0.179) — so a colored avatar/badge
// initial stays legible on any user-chosen color. Unparseable input falls back to white.
func readableTextOn(hex string) string {
	h := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return "#ffffff"
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return "#ffffff"
	}
	chanLin := func(c uint64) float64 {
		s := float64(c) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	lum := 0.2126*chanLin((v>>16)&0xff) + 0.7152*chanLin((v>>8)&0xff) + 0.0722*chanLin(v&0xff)
	if lum > 0.179 {
		return "#1c1c1e"
	}
	return "#ffffff"
}

// memberDateStyleOptions are the per-member date-style choices: a leading
// "Inherit" (empty value = use the household default) then the concrete styles.
func memberDateStyleOptions() []uiw.SelectOption {
	return []uiw.SelectOption{
		{Value: "", Label: uistate.T("members.prefInherit")},
		{Value: string(prefs.DateISO), Label: "2006-01-02"},
		{Value: string(prefs.DateUS), Label: "01/02/2006"},
		{Value: string(prefs.DateEU), Label: "02/01/2006"},
		{Value: string(prefs.DateLong), Label: "Jan 2, 2006"},
	}
}

// memberRoleOptions returns the three role choices — owner / admin / viewer —
// labelled via memberrole.Label so the select matches the canonical display names.
func memberRoleOptions() []uiw.SelectOption {
	return []uiw.SelectOption{
		{Value: string(domain.RoleOwner), Label: memberrole.Label(domain.RoleOwner)},
		{Value: string(domain.RoleAdmin), Label: memberrole.Label(domain.RoleAdmin)},
		{Value: string(domain.RoleViewer), Label: memberrole.Label(domain.RoleViewer)},
	}
}

// memberDefaultAccountOptions lists "Inherit" (no per-member default) then every
// account by name, for the per-member default-account preference.
func memberDefaultAccountOptions() []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("members.prefInherit")}}
	if app := appstate.Default; app != nil {
		for _, a := range app.Accounts() {
			opts = append(opts, uiw.SelectOption{Value: a.ID, Label: a.Name})
		}
	}
	return opts
}

type memberRowProps struct {
	Member domain.Member
	// Worth/Spent/SharePct are the person's ledger figures: net worth, spending
	// this period, and their share of the household's largest holding (0–100).
	Worth    money.Money
	Spent    money.Money
	SharePct int
	// CustomLine is the pre-built "Label: value · …" summary of the member's
	// custom-field values.
	CustomLine   string
	OnDelete     func(string)
	OnSetDefault func(string)
	OnView       func(string)
	// C274: per-member PIN management (device-level access control). Set/change
	// opens the shell-root flip modal; remove clears immediately.
	MemberHasPIN bool
	OnClearPIN   func(id string) // nil → PIN management unavailable
}

// MemberRow is one person's ledger row: the oversized avatar + name + role
// chips on the left, the worth/spent figure column on the right, their share
// of household worth as a bar underneath, and the actions behind an Edit
// button plus a ⋯ menu (transactions, default, PIN, delete). Edit and PIN
// set/change open the shell-root flip modal (MemberEditHost).
func MemberRow(props memberRowProps) ui.Node {
	m := props.Member
	color := m.Color
	if color == "" {
		color = "#7c83ff"
	}

	del := ui.UseEvent(Prevent(func() { props.OnDelete(m.ID) }))
	mkDefault := ui.UseEvent(Prevent(func() { props.OnSetDefault(m.ID) }))
	view := ui.UseEvent(Prevent(func() { props.OnView(m.ID) }))
	// Edit and PIN set/change open the shell-root flip modal (MemberEditHost) —
	// an inline row form sat under transformed tile ancestors (see BudgetEditHost).
	startEdit := ui.UseEvent(Prevent(func() {
		uistate.SetMemberEdit(uistate.MemberEdit{ID: m.ID, Mode: uistate.MemberEditModeEdit})
	}))
	onShowPINForm := ui.UseEvent(Prevent(func() {
		uistate.SetMemberEdit(uistate.MemberEdit{ID: m.ID, Mode: uistate.MemberEditModePIN})
	}))
	onRemovePIN := ui.UseEvent(Prevent(func() {
		if props.OnClearPIN != nil {
			props.OnClearPIN(m.ID)
		}
	}))

	// C276: show the member's real role (Owner/Admin/Viewer) as a chip,
	// with the "default" quick-add-seed chip kept separately and visually distinct.
	roleLabel := memberrole.Label(memberrole.Resolve(m))

	// The ⋯ overflow menu: transactions, make-default, PIN management, delete.
	menuItem := func(testID, label string, on ui.Handler, extra ...any) ui.Node {
		args := []any{css.Class("add-item"), Type("button"), Attr("role", "menuitem"), OnClick(on)}
		if testID != "" {
			args = append(args, Attr("data-testid", testID))
		}
		args = append(args, extra...)
		args = append(args, label)
		return Button(args...)
	}
	items := []ui.Node{
		menuItem("member-view-"+m.ID, uistate.T("nav.transactions"), view, Title(uistate.T("members.viewTitle"))),
	}
	if !m.IsDefault {
		items = append(items, menuItem("member-make-default-"+m.ID, uistate.T("members.makeDefault"), mkDefault, Title(uistate.T("members.makeDefaultTitle"))))
	}
	if props.MemberHasPIN {
		items = append(items,
			menuItem("member-change-pin-"+m.ID, uistate.T("profileSwitch.changePIN"), onShowPINForm),
			menuItem("member-remove-pin-"+m.ID, uistate.T("profileSwitch.removePIN"), onRemovePIN),
		)
	} else {
		items = append(items, menuItem("member-set-pin-"+m.ID, uistate.T("profileSwitch.setPIN"), onShowPINForm))
	}
	items = append(items, menuItem("member-delete-"+m.ID, uistate.T("members.deleteTitle"), del,
		Attr("aria-label", uistate.T("members.deleteTitle"))))

	chips := []any{css.Class("hh-person-chips"),
		Span(css.Class("badge"), Attr("data-testid", "member-role-badge-"+m.ID), roleLabel),
	}
	if m.IsDefault {
		chips = append(chips, Span(css.Class("badge badge-muted"), Attr("data-testid", "member-default-chip-"+m.ID), uistate.T("members.defaultBadge")))
	}
	if props.MemberHasPIN {
		chips = append(chips, Span(css.Class("badge badge-muted"), uistate.T("members.pinBadge")))
	}

	// A negative worth reads as a drag on the household, not a share of it: the
	// bar takes the down tone and the label leads with a minus (L7 gate note).
	shareTitle := uistate.T("members.shareOfWorth")
	sharePctLabel := fmt.Sprintf("%d%%", props.SharePct)
	if props.Worth.IsNegative() {
		shareTitle = uistate.T("members.shareOfWorthNeg")
		sharePctLabel = fmt.Sprintf("−%d%%", props.SharePct)
	}
	shareBar := Div(css.Class("hh-person-share"),
		Div(css.Class("share-bar", "share-bar-thin"), Attr("title", shareTitle),
			Div(ClassStr(shareFillCls(props.Worth)), Style(map[string]string{"width": fmt.Sprintf("%d%%", props.SharePct)}))),
		Span(css.Class("hh-person-share-pct"), Attr("title", shareTitle), sharePctLabel),
	)

	return Div(css.Class("row hh-person"),
		Div(css.Class("hh-person-main"),
			Div(css.Class("hh-person-id"),
				memberAvatar(m.Name, color),
				Div(
					Div(css.Class("hh-person-name", tw.FontDisplay), m.Name),
					Div(chips...),
				),
			),
			Div(css.Class("hh-person-figures"),
				Span(ClassStr("hh-person-worth amount "+accentFor(props.Worth)), fmtMoney(props.Worth)),
				Span(css.Class("hh-person-sub"), uistate.T("members.spentSub", fmtMoney(props.Spent))),
			),
			Div(css.Class("hh-person-actions"),
				Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("members.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
				uiw.KebabMenu(uiw.KebabMenuProps{
					ID:           "member-menu-" + m.ID,
					AriaLabel:    uistate.T("members.menuAria"),
					ToggleTestID: "member-menu-btn-" + m.ID,
					Items:        items,
				}),
			),
		),
		shareBar,
		If(props.CustomLine != "", Div(css.Class("hh-person-custom"), props.CustomLine)),
	)
}

// shareFillCls tones the person's worth share bar: accent for positive,
// money-down for negative (matching the /networth liability bars).
func shareFillCls(v money.Money) string {
	if v.IsNegative() {
		return "share-bar-fill nw-bar-down"
	}
	return "share-bar-fill"
}

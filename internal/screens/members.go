// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/customfields"
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
	saveMember := func(id, newName, newColor, dateStyle, defAccountID, newRole string, custom map[string]string) {
		for _, m := range app.Members() {
			if m.ID != id {
				continue
			}
			if n := strings.TrimSpace(newName); n != "" {
				m.Name = n
			}
			m.Color = strings.TrimSpace(newColor)
			// Per-member preferences (§1.19): empty = inherit the household default.
			m.Prefs.DateStyle = strings.TrimSpace(dateStyle)
			m.Prefs.DefaultAccountID = strings.TrimSpace(defAccountID)
			// Role: accept the form value; fall back to the resolved role if invalid.
			if r, err := memberrole.ParseRole(strings.TrimSpace(newRole)); err == nil {
				m.Role = r
			}
			m.Custom = customValuesToMap(memberDefs, custom)
			if err := app.PutMember(m); err != nil {
				errMsg.Set(err.Error())
				return
			}
			break
		}
		errMsg.Set("")
		bump()
	}

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
			Defs:         memberDefs,
			OnDelete:     deleteMember,
			OnSetDefault: setDefault,
			OnSave:       saveMember,
			OnView:       viewTransactions,
			// C274: per-member PIN management.
			MemberHasPIN: app.MemberHasPIN(mID),
			OnSetPIN: func(id, pin string) error {
				err := app.SetMemberPIN(id, pin)
				if err == nil {
					bump() // re-render so MemberHasPIN reflects the new state
				}
				return err
			},
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
	// custom-field values; Defs drive the inline edit form's custom inputs.
	CustomLine   string
	Defs         []customfields.Def
	OnDelete     func(string)
	OnSetDefault func(string)
	OnSave       func(id, name, color, dateStyle, defAccountID, role string, custom map[string]string)
	OnView       func(string)
	// C274: per-member PIN management (device-level access control).
	MemberHasPIN bool
	OnSetPIN     func(id, pin string) error // nil → PIN management unavailable
	OnClearPIN   func(id string)            // nil → PIN management unavailable
}

// MemberRow is one person's ledger row: the oversized avatar + name + role
// chips on the left, the worth/spent figure column on the right, their share
// of household worth as a bar underneath, and the actions behind an Edit
// button plus a ⋯ menu (transactions, default, PIN, delete). It can be edited
// inline (name, color, role, preferences, custom fields). All hooks are
// declared unconditionally so the edit toggle never reorders them.
func MemberRow(props memberRowProps) ui.Node {
	m := props.Member
	color := m.Color
	if color == "" {
		color = "#7c83ff"
	}

	del := ui.UseEvent(Prevent(func() { props.OnDelete(m.ID) }))
	mkDefault := ui.UseEvent(Prevent(func() { props.OnSetDefault(m.ID) }))
	view := ui.UseEvent(Prevent(func() { props.OnView(m.ID) }))
	editing := ui.UseState(false)
	nameS := ui.UseState(m.Name)
	colorS := ui.UseState(color)
	dateStyleS := ui.UseState(m.Prefs.DateStyle)
	defAcctS := ui.UseState(m.Prefs.DefaultAccountID)
	roleS := ui.UseState(string(memberrole.Resolve(m)))
	customS := ui.UseState(map[string]string{})
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onColor := ui.UseEvent(func(v string) { colorS.Set(v) })
	setCustom := func(key, value string) {
		cur := customS.Get()
		next := make(map[string]string, len(cur)+1)
		for k, v := range cur {
			next[k] = v
		}
		next[key] = value
		customS.Set(next)
	}
	startEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(m.Name)
		colorS.Set(color)
		dateStyleS.Set(m.Prefs.DateStyle)
		defAcctS.Set(m.Prefs.DefaultAccountID)
		roleS.Set(string(memberrole.Resolve(m)))
		customS.Set(customMapToStrings(m.Custom))
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(m.ID, nameS.Get(), colorS.Get(), dateStyleS.Get(), defAcctS.Get(), roleS.Get(), customS.Get())
		editing.Set(false)
	}))

	// Land the cursor in the first field when the inline editor opens (§6.7).
	editKey := "closed"
	if editing.Get() {
		editKey = "open"
	}
	ui.UseEffect(func() func() {
		if editing.Get() {
			focusByID("member-edit-" + m.ID)
		}
		return nil
	}, editKey)

	// C274: per-member PIN management hooks — declared unconditionally so
	// hook depth is stable regardless of the editing / showPINForm state.
	showPINForm := ui.UseState(false)
	pinInputS := ui.UseState("")
	pinErrS := ui.UseState("")
	onPINInput := ui.UseEvent(func(v string) { pinInputS.Set(v) })
	onShowPINForm := ui.UseEvent(Prevent(func() {
		showPINForm.Set(true)
		pinInputS.Set("")
		pinErrS.Set("")
	}))
	onCancelPINForm := ui.UseEvent(Prevent(func() {
		showPINForm.Set(false)
		pinInputS.Set("")
		pinErrS.Set("")
	}))
	onSubmitPIN := ui.UseEvent(Prevent(func() {
		if props.OnSetPIN == nil {
			return
		}
		if err := props.OnSetPIN(m.ID, pinInputS.Get()); err != nil {
			pinErrS.Set(uistate.T("profileSwitch.pinTooWeak"))
		} else {
			pinErrS.Set("")
			showPINForm.Set(false)
			pinInputS.Set("")
		}
	}))
	onRemovePIN := ui.UseEvent(Prevent(func() {
		if props.OnClearPIN != nil {
			props.OnClearPIN(m.ID)
		}
	}))

	if editing.Get() {
		// Custom-field inputs (member-scoped defs), rendered as keyed components so
		// each owns its event hook.
		customInputs := MapKeyed(props.Defs,
			func(d customfields.Def) any { return d.Key },
			func(d customfields.Def) ui.Node {
				return labeledField(d.Label, ui.CreateElement(CustomFieldInput, customFieldInputProps{
					Def: d, Value: customS.Get()[d.Key], OnChange: setCustom,
				}))
			})
		return Div(css.Class("row hh-person"),
			Form(css.Class("form-grid hh-person-form"), OnSubmit(saveEdit),
				labeledField(uistate.T("members.name"),
					Input(css.Class("field"), Attr("id", "member-edit-"+m.ID), Type("text"), Attr("aria-label", uistate.T("members.name")), Placeholder(uistate.T("members.name")), Value(nameS.Get()), OnInput(onName))),
				labeledField(uistate.T("members.color"),
					Input(css.Class("color-input"), Type("color"), Attr("title", uistate.T("members.color")), Attr("aria-label", uistate.T("members.color")), Value(colorS.Get()), OnInput(onColor))),
				// Per-member preferences (§1.19): an optional personal date style and a
				// default account that seeds this member's quick-add. "Inherit" = use the
				// household default.
				labeledField(uistate.T("members.prefDateStyle"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   memberDateStyleOptions(),
						Selected:  dateStyleS.Get(),
						OnChange:  func(v string) { dateStyleS.Set(v) },
						AriaLabel: uistate.T("members.prefDateStyle"),
					})),
				labeledField(uistate.T("members.prefDefaultAccount"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   memberDefaultAccountOptions(),
						Selected:  defAcctS.Get(),
						OnChange:  func(v string) { defAcctS.Set(v) },
						AriaLabel: uistate.T("members.prefDefaultAccount"),
					})),
				labeledField(uistate.T("members.roleLabel"),
					uiw.SelectInput(uiw.SelectInputProps{
						Options:   memberRoleOptions(),
						Selected:  roleS.Get(),
						OnChange:  func(v string) { roleS.Set(v) },
						AriaLabel: "Role",
						TestID:    "member-edit-role-" + m.ID,
					})),
				customInputs,
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	// C276: show the member's real role (Owner/Admin/Viewer) as a chip,
	// with the "default" quick-add-seed chip kept separately and visually distinct.
	roleLabel := memberrole.Label(memberrole.Resolve(m))

	// C274: the PIN set/change form, opened from the ⋯ menu.
	pinForm := Fragment()
	if props.OnSetPIN != nil && showPINForm.Get() {
		pinLbl := uistate.T("profileSwitch.setPIN")
		if props.MemberHasPIN {
			pinLbl = uistate.T("profileSwitch.changePIN")
		}
		pinForm = Form(css.Class("form-grid hh-person-form"),
			Attr("data-testid", "member-pin-form-"+m.ID),
			OnSubmit(onSubmitPIN),
			uiw.FormField(uistate.T("profileSwitch.pinNew"),
				Input(css.Class("field"), Type("password"),
					Attr("autocomplete", "off"),
					Attr("data-testid", "member-pin-input-"+m.ID),
					Value(pinInputS.Get()),
					OnInput(onPINInput),
				),
			),
			If(pinErrS.Get() != "", P(css.Class("notice-danger"), pinErrS.Get())),
			Button(css.Class("btn btn-primary"), Type("submit"), pinLbl),
			Button(css.Class("btn"), Type("button"), OnClick(onCancelPINForm),
				uistate.T("profileSwitch.pinFormCancel")),
		)
	}

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
	if props.OnSetPIN != nil {
		if props.MemberHasPIN {
			items = append(items,
				menuItem("member-change-pin-"+m.ID, uistate.T("profileSwitch.changePIN"), onShowPINForm),
				menuItem("member-remove-pin-"+m.ID, uistate.T("profileSwitch.removePIN"), onRemovePIN),
			)
		} else {
			items = append(items, menuItem("member-set-pin-"+m.ID, uistate.T("profileSwitch.setPIN"), onShowPINForm))
		}
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

	shareBar := Div(css.Class("hh-person-share"),
		Div(css.Class("share-bar", "share-bar-thin"), Attr("title", uistate.T("members.shareOfWorth")),
			Div(ClassStr(shareFillCls(props.Worth)), Style(map[string]string{"width": fmt.Sprintf("%d%%", props.SharePct)}))),
		Span(css.Class("hh-person-share-pct"), fmt.Sprintf("%d%%", props.SharePct)),
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
		pinForm,
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

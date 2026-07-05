// SPDX-License-Identifier: MIT

//go:build js && wasm

// profileswitch.go — C274 "Who's using CashFlux?" device profile-switch modal.
//
// SCOPE BOUNDARY: this is device-level profile gating + scope-switch auth only.
// It does NOT provide cryptographic per-member data isolation — all members share
// the same local dataset. Roles and PINs are a shared-device convenience layer,
// not per-member logins.
//
// Owner override: a member with RoleOwner may switch to ANY profile without
// entering the target member's PIN. Owners manage the household device and must
// not be blocked by another member's access PIN.
package app

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/memberrole"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/state"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// psHandle is captured by ProfileSwitchHost on each mount so openProfileSwitch
// can open the modal from outside any component (e.g. from MemberSwitcher's
// button click handler). Nil before the first mount — no-op in that window.
var psHandle interface {
	Get() bool
	Set(bool)
}

// openProfileSwitch opens the "Who's using CashFlux?" profile-switch modal.
// Safe to call before the host is mounted (no-op in that case).
func openProfileSwitch() {
	if psHandle != nil {
		psHandle.Set(true)
	}
}

// profileCardItemProps configures one selectable profile card in the modal.
// OnSelect is a plain func (not an On* hook) so the caller can build it in a
// loop without violating the GWC On*-hooks-in-loops rule.
type profileCardItemProps struct {
	MemberID    string // "" means Everyone (household view)
	MemberName  string
	MemberColor string
	IsActive    bool
	HasPIN      bool
	OnSelect    func()
}

// profileCardItem is a stable per-card sub-component. Each instance owns its
// UseEvent hook at a fixed depth (never inside a loop), mirroring the
// scopeChip / OwnerShareRow pattern used elsewhere.
func profileCardItem(props profileCardItemProps) uic.Node {
	click := uic.UseEvent(func() { props.OnSelect() })
	cls := "ps-card"
	if props.IsActive {
		cls += " ps-card-active"
	}
	pinBadge := uic.Node(Fragment())
	if props.HasPIN {
		pinBadge = Span(css.Class("badge", "ps-pin-badge"), "🔒")
	}
	return Button(
		ClassStr(cls),
		Type("button"),
		Attr("aria-label", props.MemberName),
		Attr("aria-pressed", fmt.Sprintf("%t", props.IsActive)),
		Attr("data-testid", "ps-card-"+props.MemberID),
		OnClick(click),
		Span(css.Class("ps-card-name"), props.MemberName),
		pinBadge,
	)
}

// ProfileSwitchHost is the singleton "Who's using CashFlux?" profile-switcher
// modal host. Mount it once in Shell alongside SettingsHost / DialogHost so
// its hook depth is always constant regardless of which screen is active.
//
// Two-step flow:
//  1. Member-picker: lists Everyone + each household member (with 🔒 badge if
//     PIN-protected). Tapping a card with no PIN (or as an Owner) switches
//     immediately. Tapping a PIN-protected card advances to step 2.
//  2. PIN challenge: prompts for the target member's PIN. Correct → switch;
//     incorrect → error message, clear input, stay on challenge step.
//
// All hooks are declared unconditionally before any early return so the hook
// call order never changes between open and closed renders.
func ProfileSwitchHost() uic.Node {
	// ── hooks — always called, always in this order ───────────────────────

	// hook 1: is the modal currently open?
	open := state.UseAtom("cf:ps-open", false)
	psHandle = open // capture for openProfileSwitch()

	// hook 2: target member awaiting PIN ("" = pick phase, not PIN phase)
	pendingID := state.UseAtom("cf:ps-pending", "")

	// hook 3: PIN input value
	pinInput := state.UseAtom("cf:ps-pin", "")

	// hook 4: PIN error message
	pinErr := state.UseAtom("cf:ps-err", "")

	// hook 5: current active scope (for the picker's highlight state)
	scopeAtom := uistate.UseActiveScope()

	// hook 6: PIN text-input handler
	onPinInput := uic.UseEvent(func(v string) { pinInput.Set(v) })

	// hook 7: close / cancel — resets all ephemeral state
	doClose := uic.UseEvent(func() {
		open.Set(false)
		pendingID.Set("")
		pinInput.Set("")
		pinErr.Set("")
	})

	// hook 8: PIN submit — verify then switch or show error
	onPinSubmit := uic.UseEvent(func() {
		target := pendingID.Get()
		if VerifyMemberPIN(target, pinInput.Get()) {
			switchToMember(target)
			open.Set(false)
			pendingID.Set("")
			pinInput.Set("")
			pinErr.Set("")
		} else {
			pinErr.Set(uistate.T("profileSwitch.pinWrong"))
			pinInput.Set("")
		}
	})

	// ── stable anchor when closed ─────────────────────────────────────────
	if !open.Get() {
		return Div(css.Class("cf-ps-root"))
	}

	// ── caller-is-owner check (owner override) ────────────────────────────
	app := appstate.Default
	callerIsOwner := false
	if app != nil {
		callerID := uistate.ActiveIdentityID()
		for _, m := range app.Members() {
			if m.ID == callerID && memberrole.Resolve(m) == domain.RoleOwner {
				callerIsOwner = true
				break
			}
		}
	}

	// ── PIN challenge step ────────────────────────────────────────────────
	if pid := pendingID.Get(); pid != "" {
		targetName := pid
		if app != nil {
			for _, m := range app.Members() {
				if m.ID == pid {
					targetName = m.Name
					break
				}
			}
		}
		prompt := fmt.Sprintf(uistate.T("profileSwitch.pinPrompt"), targetName)
		return Div(css.Class("cf-ps-root"),
			Div(css.Class("cf-dialog-backdrop"),
				Attr("role", "dialog"),
				Attr("aria-modal", "true"),
				Attr("aria-label", uistate.T("profileSwitch.title")),
				Div(css.Class("cf-dialog-scrim"), OnClick(doClose)),
				Div(css.Class("cf-ps-panel"),
					H3(css.Class("cf-ps-title"), uistate.T("profileSwitch.title")),
					P(css.Class("muted"), prompt),
					Input(css.Class("set-input"),
						Type("password"),
						Attr("id", "cf-ps-pin"),
						Attr("autocomplete", "off"),
						Attr("aria-label", uistate.T("profileSwitch.pinLabel")),
						Placeholder(uistate.T("profileSwitch.pinLabel")),
						Value(pinInput.Get()),
						OnInput(onPinInput),
					),
					If(pinErr.Get() != "", P(css.Class("notice-danger"), pinErr.Get())),
					Div(css.Class("cf-ps-actions"),
						Button(css.Class("btn"), Type("button"), OnClick(doClose),
							uistate.T("profileSwitch.cancel")),
						Button(css.Class("btn btn-primary"), Type("button"), OnClick(onPinSubmit),
							uistate.T("profileSwitch.pinBtn")),
					),
				),
			),
		)
	}

	// ── member-picker step ────────────────────────────────────────────────
	cur := scopeAtom.Get()
	curMemberID := ""
	if len(cur.Owners) == 1 {
		curMemberID = cur.Owners[0]
	}

	// makeSelectHandler returns the card's click handler. Builds the handler
	// as a plain func (not a UseEvent) so it can be called in a loop without
	// registering a new hook each iteration (the sub-component profileCardItem
	// owns the UseEvent that wraps this func).
	makeSelectHandler := func(targetID string) func() {
		return func() {
			// Owner override or no PIN: switch immediately.
			if targetID == "" || callerIsOwner || !MemberHasPIN(targetID) {
				switchToMember(targetID)
				open.Set(false)
				pendingID.Set("")
				pinInput.Set("")
				pinErr.Set("")
				return
			}
			// Advance to PIN challenge.
			pendingID.Set(targetID)
			pinInput.Set("")
			pinErr.Set("")
		}
	}

	// Seed cards with the CSS class first so the slice can be spread directly
	// into Div(cards...) — Go doesn't allow mixing Div(css.Class(...), slice...).
	cards := []any{css.Class("cf-ps-cards")}
	// "Everyone (household)" card — always first in the list.
	cards = append(cards, uic.CreateElement(profileCardItem, profileCardItemProps{
		MemberID:   "everyone",
		MemberName: uistate.T("profileSwitch.everyone"),
		IsActive:   curMemberID == "",
		HasPIN:     false,
		OnSelect:   makeSelectHandler(""),
	}))
	if app != nil {
		for _, m := range app.Members() {
			m := m // capture loop var
			cards = append(cards, uic.CreateElement(profileCardItem, profileCardItemProps{
				MemberID:    m.ID,
				MemberName:  m.Name,
				MemberColor: m.Color,
				IsActive:    curMemberID == m.ID,
				HasPIN:      MemberHasPIN(m.ID),
				OnSelect:    makeSelectHandler(m.ID),
			}))
		}
	}
	// Owner override disclosure — shown only to the current owner.
	if callerIsOwner {
		cards = append(cards, P(css.Class("muted", "ps-owner-note"), uistate.T("profileSwitch.ownerNote")))
	}

	return Div(css.Class("cf-ps-root"),
		Div(css.Class("cf-dialog-backdrop"),
			Attr("role", "dialog"),
			Attr("aria-modal", "true"),
			Attr("aria-label", uistate.T("profileSwitch.title")),
			Div(css.Class("cf-dialog-scrim"), OnClick(doClose)),
			Div(css.Class("cf-ps-panel"),
				H3(css.Class("cf-ps-title"), uistate.T("profileSwitch.title")),
				Div(cards...),
				Div(css.Class("cf-ps-actions"),
					Button(css.Class("btn"), Type("button"), OnClick(doClose),
						uistate.T("profileSwitch.cancel")),
				),
			),
		),
	)
}

// switchToMember changes the active scope to the given member (or the Everyone
// household view when memberID is ""). Sets only the Owners dimension so other
// scope dimensions (Institutions, Types, AccountIDs) are reset to "all" —
// profile-switching is an identity operation, not an incremental scope filter.
func switchToMember(memberID string) {
	var owners []string
	if memberID != "" {
		owners = []string{memberID}
	}
	uistate.SetActiveScope(scope.ReportScope{Owners: owners})
}

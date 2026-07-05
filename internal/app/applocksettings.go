// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// appLockSection renders the Settings → App lock controls, adapting to whether a
// passcode is currently set. onChange re-renders the settings panel after a
// change (the setup form calls it on success via showAppLockSetup's callback).
func appLockSection(onChange func()) uic.Node {
	c := loadAppLock()
	if c.Enabled {
		active := !c.Suspended
		status := uistate.T("applock.statusSuspended")
		if active {
			status = uistate.T("applock.statusOn")
			if c.AutoLockMinutes > 0 {
				status = fmt.Sprintf(uistate.T("applock.statusOnAuto"), c.AutoLockMinutes)
			}
		}
		actions := []any{css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1)}
		if active {
			actions = append(actions, dataBtn(uistate.T("applock.cmdLock"), false, showAppLockGate))
		}
		actions = append(actions,
			dataBtn(uistate.T("applock.cmdChange"), false, func() { showAppLockSetup(onChange) }),
			// C282: passkey manager button — opens the WebAuthn PRF setup modal.
			// Always shown when the lock is enabled so users can add/remove a passkey.
			dataBtn(uistate.T("webauthn.manageBtn"), false, showPasskeyManager),
			dataBtn(uistate.T("applock.cmdRemove"), true, func() {
				disableAppLock()
				if onChange != nil {
					onChange()
				}
			}),
		)
		return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
			P(css.Class("muted", tw.TextXs), status),
			ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("applock.toggleActive"), On: active, OnChange: func(v bool) {
				setLockSuspended(!v)
				if onChange != nil {
					onChange()
				}
			}}),
			Span(actions...),
			ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("applock.toggleMeta"), On: !c.HideMeta, OnChange: func(v bool) {
				setLockHideMeta(!v)
				if onChange != nil {
					onChange()
				}
			}}),
			ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("applock.toggleQuotes"), On: !c.HideQuotes, OnChange: func(v bool) {
				setLockHideQuotes(!v)
				if onChange != nil {
					onChange()
				}
			}}),
		)
	}
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
		P(css.Class("muted", tw.TextXs), uistate.T("applock.statusOff")),
		Span(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
			dataBtn(uistate.T("applock.cmdSet"), false, func() { showAppLockSetup(onChange) }),
		),
	)
}

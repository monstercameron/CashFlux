//go:build js && wasm

package app

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// appLockSection renders the Settings → App lock controls, adapting to whether a
// passcode is currently set. onChange re-renders the settings panel after a
// change (the setup form calls it on success via showAppLockSetup's callback).
func appLockSection(onChange func()) uic.Node {
	c := loadAppLock()
	if c.Enabled {
		status := uistate.T("applock.statusOn")
		if c.AutoLockMinutes > 0 {
			status = fmt.Sprintf(uistate.T("applock.statusOnAuto"), c.AutoLockMinutes)
		}
		return Div(Class("flex flex-col gap-1"),
			P(Class("muted text-xs"), status),
			Span(Class("flex flex-wrap gap-2 py-1"),
				dataBtn(uistate.T("applock.cmdLock"), false, showAppLockGate),
				dataBtn(uistate.T("applock.cmdChange"), false, func() { showAppLockSetup(onChange) }),
				dataBtn(uistate.T("applock.cmdRemove"), true, func() {
					disableAppLock()
					if onChange != nil {
						onChange()
					}
				}),
			),
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
	return Div(Class("flex flex-col gap-1"),
		P(Class("muted text-xs"), uistate.T("applock.statusOff")),
		Span(Class("flex flex-wrap gap-2 py-1"),
			dataBtn(uistate.T("applock.cmdSet"), false, func() { showAppLockSetup(onChange) }),
		),
	)
}

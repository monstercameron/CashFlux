// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

type shellResolutionSlotProps struct {
	PeriodAware bool
	YearEnding  bool
	Window      period.Window
}

// shellResolutionSlot keeps the top bar's child position stable across routes.
func shellResolutionSlot(props shellResolutionSlotProps) uic.Node {
	return If(props.PeriodAware,
		uic.CreateElement(ResolutionControl, resolutionControlProps{YearEnding: props.YearEnding, Window: props.Window}),
	)
}

func nextTheme(theme prefs.Theme) prefs.Theme {
	switch theme {
	case prefs.ThemeDark:
		return prefs.ThemeLight
	case prefs.ThemeLight:
		return prefs.ThemeSystem
	default:
		return prefs.ThemeDark
	}
}

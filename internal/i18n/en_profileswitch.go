// SPDX-License-Identifier: MIT

package i18n

// profileSwitchKeys holds the English strings for C274 local per-member
// profile + PIN switch (device user-switching). Merged via init so this
// file does not touch the concurrent-WIP en.go; mirrors the en_webauthn.go
// pattern.
var profileSwitchKeys = Catalog{
	// "Who's using CashFlux?" profile-switch modal.
	"profileSwitch.title":    "Who's using CashFlux?",
	"profileSwitch.everyone": "Everyone (household)",
	// %s is replaced with the target member's display name.
	"profileSwitch.pinPrompt":  "Enter PIN for %s",
	"profileSwitch.pinLabel":   "PIN",
	"profileSwitch.pinWrong":   "Incorrect PIN — try again.",
	"profileSwitch.pinBtn":     "Unlock",
	"profileSwitch.cancel":     "Cancel",
	"profileSwitch.switchBtn":  "Switch profile…",
	"profileSwitch.ownerNote":  "You are an Owner and can switch to any profile without entering that member’s PIN.",
	// Per-member PIN management controls on /members.
	"profileSwitch.setPIN":       "Set PIN",
	"profileSwitch.changePIN":    "Change PIN",
	"profileSwitch.removePIN":    "Remove PIN",
	"profileSwitch.pinNew":       "New PIN",
	"profileSwitch.pinSetOK":     "PIN set.",
	"profileSwitch.pinRemovedOK": "PIN removed.",
	"profileSwitch.pinTooWeak":   "PIN must be at least 4 characters and not too simple.",
	"profileSwitch.pinFormCancel": "Cancel",
}

func init() {
	for k, v := range profileSwitchKeys {
		english[k] = v
	}
}

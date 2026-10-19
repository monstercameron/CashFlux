// SPDX-License-Identifier: MIT

package app

// cloudStep describes the next setup task, never a successful upload.
type cloudStep string

const (
	cloudOff     cloudStep = "off"
	cloudWhere   cloudStep = "where"
	cloudSignIn  cloudStep = "signin"
	cloudProtect cloudStep = "protect"
	cloudFirst   cloudStep = "first"
	cloudDone    cloudStep = "done"
)

type cloudFacts struct {
	Disabled, SignedIn, Protected bool
	Setup                         string
	State                         string
	Pending                       int
}

// cloudNextStep derives progress from capabilities rather than trusting a saved
// step to authorize a write. Signing out or removing protection reopens setup.
func cloudNextStep(f cloudFacts) cloudStep {
	if f.Setup == string(cloudWhere) {
		return cloudWhere
	}
	if f.Disabled && f.Setup == "" {
		return cloudOff
	}
	if !f.SignedIn {
		return cloudSignIn
	}
	if !f.Protected {
		return cloudProtect
	}
	if f.Disabled || f.Setup != "" || f.State == "" || f.State == "local" {
		return cloudFirst
	}
	return cloudDone
}

// cloudFace is a presentation decision. It never chooses a data copy or alters
// sync state; unknown transport states deliberately cannot claim success.
func cloudFace(f cloudFacts, step cloudStep) (title, hint string) {
	if step == cloudOff {
		return "cloud1.off", "cloud1.offHint"
	}
	if !f.Protected && f.SignedIn {
		return "cloud1.attention", "cloud1.protectHint"
	}
	if f.State == "conflict" {
		return "cloud1.attention", "cloud1.conflictHint"
	}
	switch f.State {
	case "locked-different-passcode":
		return "cloud1.attention", "cloud1.differentPasscode"
	case "needs-rebind", "rebind":
		return "cloud1.attention", "cloud1.rebindHint"
	case "identity-unknown":
		return "cloud1.attention", "cloud1.identityHint"
	case "error":
		return "cloud1.attention", "cloud1.attentionHint"
	}
	if step != cloudDone {
		return "cloud1.settingUp", "cloud1.setupHint"
	}
	switch f.State {
	case "synced":
		if f.Pending == 0 {
			return "cloud1.synced", "cloud1.syncedHint"
		}
		return "cloud1.syncing", "cloud1.pendingHint"
	case "syncing":
		return "cloud1.syncing", "cloud1.syncingHint"
	case "offline":
		return "cloud1.paused", "cloud1.pausedHint"
	default:
		return "cloud1.attention", "cloud1.attentionHint"
	}
}

func cloudCanSync(f cloudFacts) bool { return !f.Disabled && f.SignedIn && f.Protected }

// cloudAuthDisablesSync prevents authentication from skipping protected setup.
func cloudAuthDisablesSync(alreadyDisabled, setupPending, encryptionReady bool) bool {
	return alreadyDisabled || setupPending || !encryptionReady
}

func cloudSetupFinished(saved string, disabled, busy bool, state string, pending int) bool {
	return saved == string(cloudFirst) && !disabled && !busy && state == "synced" && pending == 0
}

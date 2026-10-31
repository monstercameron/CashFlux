// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
	"syscall/js"
)

type cloudActionKind uint8

const (
	cloudActionQuiet cloudActionKind = iota
	cloudActionPrimary
	cloudActionDanger
)

type cloudActionProps struct {
	Label, TestID string
	OnClick       func()
	Disabled      bool
	Kind          cloudActionKind
}

// cloudAction owns its event hook, including when used in conditional slots.
func cloudAction(p cloudActionProps) uic.Node {
	class := "btn"
	if p.Kind == cloudActionPrimary {
		class += " btn-primary"
	}
	if p.Kind == cloudActionDanger {
		class += " btn-danger"
	}
	return Button(css.Class(class), Type("button"), Attr("data-testid", p.TestID),
		DisabledIf(p.Disabled), OnClick(func() {
			if !p.Disabled && p.OnClick != nil {
				p.OnClick()
			}
		}), p.Label)
}

type cloudStatusProps struct {
	Facts                   cloudFacts
	Step                    cloudStep
	LastSync, Device, Peers string
	Action                  uic.Node
}

func cloudStatusView(p cloudStatusProps) uic.Node {
	title, hint := cloudFace(p.Facts, p.Step)
	return Section(css.Class("cloud-status", cloudStatusRules()), Attr("data-testid", "sync-status-card"),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.ItemsCenter, tw.Gap3),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2, tw.MinW0), Attr("role", "status"), Attr("aria-live", "polite"), Attr("aria-atomic", "true"),
				H2(css.Class(tw.Text18, tw.FontSemibold), uistate.T(title)),
				P(css.Class(tw.Text14, tw.TextDim), uistate.T(hint)),
				If(p.Facts.Protected, uiw.Pill(uiw.PillProps{Label: uistate.T("cloud1.encrypted"), Tone: uiw.PillPositive})),
				If(p.LastSync != "", P(css.Class(tw.Text12, tw.TextDim), uistate.T("sync.lastSynced", p.LastSync))),
				If(p.Facts.Pending > 0, P(css.Class(tw.Text12), uistate.T("sync.pendingCount", p.Facts.Pending))),
			), p.Action,
		),
		P(css.Class(tw.Text12, tw.TextDim, tw.Mt3), uistate.T("cloud1.thisDevice", p.Device)),
		If(p.Peers != "", P(css.Class(tw.Text12, tw.TextDim), p.Peers)),
	)
}

type cloudSetupProps struct {
	Step                          cloudStep
	Where, SignIn, Protect, First uic.Node
}

func cloudSetupView(p cloudSetupProps) uic.Node {
	id := uic.UseId()
	uic.UseEffect(func() func() {
		doc := js.Global().Get("document")
		if doc.Truthy() && doc.Get("getElementById").Type() == js.TypeFunction {
			if section := doc.Call("getElementById", id); section.Truthy() && section.Get("focus").Type() == js.TypeFunction {
				section.Call("focus")
			}
		}
		return nil
	}, string(p.Step))
	steps := []cloudStep{cloudWhere, cloudSignIn, cloudProtect, cloudFirst}
	return Section(css.Class("cloud-setup", cloudSectionRules()), ID(id), Attr("tabindex", "-1"), Attr("aria-label", uistate.T("cloud1.step."+string(p.Step))),
		Ol(css.Class("cloud-steps", cloudStepsRules()), Attr("aria-label", uistate.T("cloud1.progress")),
			MapKeyed(steps, func(s cloudStep) any { return string(s) }, func(s cloudStep) uic.Node {
				return uic.CreateElement(cloudStepLabel, cloudStepLabelProps{Step: s, Current: p.Step})
			})),
		If(p.Step == cloudWhere, p.Where), If(p.Step == cloudSignIn, p.SignIn),
		If(p.Step == cloudProtect, p.Protect), If(p.Step == cloudFirst, p.First),
	)
}

type cloudStepLabelProps struct{ Step, Current cloudStep }

func cloudStepLabel(p cloudStepLabelProps) uic.Node {
	args := []any{css.Class("cloud-step", cloudStepRules()), uistate.T("cloud1.step." + string(p.Step))}
	if p.Step == p.Current {
		args = append(args, Attr("aria-current", "step"))
	}
	return Li(args...)
}

type cloudProtectionProps struct {
	Protected                      bool
	OnProtect, OnPasskey, OnBackup func()
}

func cloudProtectionView(p cloudProtectionProps) uic.Node {
	return Section(css.Class("cloud-protection", cloudSectionRules()), Attr("data-testid", "cloud-protection"),
		H3(css.Class(tw.Text15, tw.FontSemibold), uistate.T("cloud1.step.protect")),
		IfElse(p.Protected,
			uiw.Pill(uiw.PillProps{Label: uistate.T("cloud1.encrypted"), Tone: uiw.PillPositive}),
			P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.protectHint"))),
		P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.encryptExplanation")),
		If(!p.Protected, uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.protectAction"), TestID: "sync-enable-passcode", Kind: cloudActionPrimary, OnClick: p.OnProtect})),
		If(p.Protected, uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("webauthn.manageBtn"), TestID: "cloud-passkeys", OnClick: p.OnPasskey})),
		H4(css.Class(tw.Text14, tw.FontSemibold, tw.Mt3), uistate.T("cloud1.recoveryTitle")),
		P(css.Class(tw.Text14, tw.TextDim), uistate.T("cloud1.recoveryHint")),
		P(css.Class(tw.Text12, tw.TextDim), uistate.T("cloud1.recoveryLimit")),
		uic.CreateElement(cloudAction, cloudActionProps{Label: uistate.T("cloud1.backup"), TestID: "cloud-backup", OnClick: p.OnBackup}),
	)
}

type cloudDisclosureProps struct {
	Title, TestID string
	Body          uic.Node
}

func cloudDisclosure(p cloudDisclosureProps) uic.Node {
	return Details(css.Class("cloud-disclosure", cloudSectionRules()),
		Summary(css.Class(tw.Text14, tw.FontSemibold), Attr("data-testid", p.TestID), p.Title),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap3, tw.Mt3), p.Body))
}

// Component-local rules inherit the existing CashFlux theme and spacing scale.
func cloudStatusRules() css.Sheet {
	return css.New(css.Raw("border", "1px solid var(--border)"), css.Raw("border-inline-start", "4px solid var(--accent)"), css.Raw("background", "var(--bg-card)"), css.Raw("padding", "var(--space-4,1rem)"), css.Raw("border-radius", "var(--radius-sm)"))
}
func cloudSectionRules() css.Sheet {
	return css.New(css.Raw("border-top", "1px solid var(--border)"), css.Raw("padding-block", "var(--space-4,1rem)"), css.Raw("display", "flex"), css.Raw("flex-direction", "column"), css.Raw("gap", "var(--space-3,.75rem)"), css.Raw("min-width", "0"))
}
func cloudStepsRules() css.Sheet {
	return css.New(css.Raw("display", "flex"), css.Raw("flex-wrap", "wrap"), css.Raw("gap", "var(--space-4,1rem)"), css.Raw("padding-inline-start", "var(--space-4,1rem)"), css.Raw("color", "var(--text-dim)"))
}
func cloudStepRules() css.Sheet {
	return css.New(css.Raw("padding-inline-end", "var(--space-3,.75rem)"))
}

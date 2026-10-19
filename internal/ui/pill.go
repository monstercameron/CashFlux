// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"hash/fnv"
	"strings"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// PillTone is the semantic emphasis of a Pill.
type PillTone int

const (
	// PillNeutral is the default quiet tone.
	PillNeutral PillTone = iota
	// PillAttention highlights something that may need attention.
	PillAttention
	// PillPositive indicates a positive state.
	PillPositive
	// PillDanger marks a destructive or dangerous state.
	PillDanger
	// PillAccent uses the current accent color.
	PillAccent
	// PillDim deemphasizes secondary information.
	PillDim
)

// PillProps configures a Pill, Badge, or Chip.
type PillProps struct {
	Label, AriaLabel, Title string
	Tone                    PillTone
	Icon                    icon.Name
	OnClick                 func()
	OnRemove                func()
	TestID                  string
}

// Pill renders a status label, actionable suggestion, or removable chip.
func Pill(props PillProps) uic.Node {
	if props.OnClick != nil && props.OnRemove != nil {
		return uic.CreateElement(pillActionRemovable, props)
	}
	if props.OnClick != nil {
		return uic.CreateElement(pillAction, props)
	}
	if props.OnRemove != nil {
		return uic.CreateElement(pillRemovable, props)
	}
	return pillStatic(props)
}

func pillAttrs(props PillProps) []any {
	label := props.AriaLabel
	if label == "" {
		label = props.Label
	}
	attrs := []any{css.Class(pillBaseRules(), pillToneSheet(props.Tone), "cf-pill", pillToneClass(props.Tone))}
	if props.Title != "" {
		attrs = append(attrs, Title(props.Title))
	}
	if props.TestID != "" {
		attrs = append(attrs, Attr("data-testid", props.TestID))
	}
	return attrs
}

func pillContent(props PillProps) []any {
	content := []any{}
	if props.Icon != "" {
		content = append(content, Icon(props.Icon))
	}
	content = append(content, Span(props.Label))
	return content
}

func pillStatic(props PillProps) uic.Node {
	attrs := pillAttrs(props)
	label := props.AriaLabel
	if label == "" {
		label = props.Label
	}
	if label != "" {
		attrs = append(attrs, Attr("aria-label", label))
	}
	attrs = append(attrs, Attr("role", "status"))
	return Span(append(attrs, pillContent(props)...)...)
}

func pillAction(props PillProps) uic.Node {
	return uic.CreateElement(pillActionButton, props)
}

func pillActionButton(props PillProps) uic.Node {
	label := props.AriaLabel
	if label == "" {
		label = props.Label
	}
	args := append(pillAttrs(props), Type("button"), Attr("aria-label", label), OnClick(func() {
		if props.OnClick != nil {
			props.OnClick()
		}
	}))
	args = append(args, pillContent(props)...)
	return Button(args...)
}

func pillActionRemovable(props PillProps) uic.Node {
	attrs := append(pillAttrs(props), Attr("role", "group"), Attr("aria-label", props.Label))
	removeLabel := "Remove " + props.Label
	return Span(append(attrs,
		uic.CreateElement(pillActionButton, props),
		uic.CreateElement(pillRemoveButton, pillRemoveButtonProps{Label: removeLabel, OnRemove: props.OnRemove}),
	)...)
}

func pillRemovable(props PillProps) uic.Node {
	attrs := pillAttrs(props)
	attrs = append(attrs, Attr("role", "group"), Attr("aria-label", props.Label))
	removeLabel := "Remove " + props.Label
	return Span(append(attrs,
		pillContent(props),
		uic.CreateElement(pillRemoveButton, pillRemoveButtonProps{Label: removeLabel, OnRemove: props.OnRemove}),
	)...)
}

type pillRemoveButtonProps struct {
	Label    string
	OnRemove func()
}

func pillRemoveButton(props pillRemoveButtonProps) uic.Node {
	return Button(css.Class("cf-pill-remove"), Type("button"), Attr("aria-label", props.Label), OnClick(func() {
		if props.OnRemove != nil {
			props.OnRemove()
		}
	}), "×")
}

// Badge renders the same visual and behavior contract as Pill.
func Badge(props PillProps) uic.Node { return Pill(props) }

// Node renders a toolbar Chip as a removable Pill.
func (chip Chip) Node(onRemove func()) uic.Node {
	label := "Remove " + chip.Label
	if chip.RemoveLabel != "" {
		label = chip.RemoveLabel
	}
	return uic.CreateElement(chipPill, chipPillProps{
		Pill:        PillProps{Label: chip.Label, AriaLabel: chip.Label, TestID: chip.Key},
		RemoveLabel: label, OnRemove: onRemove,
	})
}

type chipPillProps struct {
	Pill        PillProps
	RemoveLabel string
	OnRemove    func()
}

func chipPill(props chipPillProps) uic.Node {
	attrs := append(pillAttrs(props.Pill), Attr("role", "group"), Attr("aria-label", props.Pill.Label))
	return Span(append(attrs,
		pillContent(props.Pill),
		uic.CreateElement(pillRemoveButton, pillRemoveButtonProps{Label: props.RemoveLabel, OnRemove: props.OnRemove}),
	)...)
}

func pillBaseRules() css.Sheet {
	return css.New(tw.InlineFlex, tw.ItemsCenter, tw.Gap1, tw.RoundedFull, tw.Px2, tw.Py1, tw.Text12, tw.FontMedium)
}

func pillToneSheet(tone PillTone) css.Sheet {
	switch tone {
	case PillAttention:
		rules := append([]css.Rule{}, tw.Border...)
		rules = append(rules, tw.BorderAmber50, tw.BgAmber10, tw.TextWarn)
		return css.New(rules...)
	case PillPositive:
		return css.New(tw.TextUp)
	case PillDanger:
		return css.New(tw.TextDanger)
	case PillAccent:
		return css.New(css.TextColor(css.Color("var(--accent, #2e8b57)")))
	case PillDim:
		return css.New(tw.TextFaint)
	default:
		return css.New(tw.TextDim)
	}
}

func pillToneClass(tone PillTone) string {
	switch tone {
	case PillAttention:
		return "cf-pill--attention"
	case PillPositive:
		return "cf-pill--positive"
	case PillDanger:
		return "cf-pill--danger"
	case PillAccent:
		return "cf-pill--accent"
	case PillDim:
		return "cf-pill--dim"
	default:
		return "cf-pill--neutral"
	}
}

// MemberAvatarProps configures a member avatar. Name is also its accessible name.
type MemberAvatarProps struct {
	Name   string
	Blank  bool
	TestID string
}

// MemberAvatar renders deterministic initials on a non-semantic member color.
// A blank name renders an empty, named avatar rather than inventing initials.
func MemberAvatar(props MemberAvatarProps) uic.Node {
	name := strings.TrimSpace(props.Name)
	attrs := []any{css.Class("cf-member-avatar", memberAvatarTone(name), memberAvatarBaseRules(), memberAvatarSheet(name)), Attr("role", "img"), Attr("aria-label", name)}
	if props.TestID != "" {
		attrs = append(attrs, Attr("data-testid", props.TestID))
	}
	if name == "" || props.Blank {
		return Span(attrs...)
	}
	return Span(append(attrs, memberInitials(name))...)
}

func memberAvatarTone(name string) string {
	const paletteSize = 6
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(name))))
	return "cf-member-avatar--" + string(rune('1'+h.Sum32()%paletteSize))
}

func memberAvatarSheet(name string) css.Sheet {
	const paletteSize = 6
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(name))))
	switch h.Sum32() % paletteSize {
	case 0:
		return css.New(css.Bg(css.Color("var(--member-avatar-bg-1, #dbeafe)")), css.TextColor(css.Color("var(--member-avatar-fg-1, #1e3a8a)")))
	case 1:
		return css.New(css.Bg(css.Color("var(--member-avatar-bg-2, #ede9fe)")), css.TextColor(css.Color("var(--member-avatar-fg-2, #4c1d95)")))
	case 2:
		return css.New(css.Bg(css.Color("var(--member-avatar-bg-3, #cffafe)")), css.TextColor(css.Color("var(--member-avatar-fg-3, #164e63)")))
	case 3:
		return css.New(css.Bg(css.Color("var(--member-avatar-bg-4, #fce7f3)")), css.TextColor(css.Color("var(--member-avatar-fg-4, #831843)")))
	case 4:
		return css.New(css.Bg(css.Color("var(--member-avatar-bg-5, #e2e8f0)")), css.TextColor(css.Color("var(--member-avatar-fg-5, #1e293b)")))
	default:
		return css.New(css.Bg(css.Color("var(--member-avatar-bg-6, #ffedd5)")), css.TextColor(css.Color("var(--member-avatar-fg-6, #7c2d12)")))
	}
}

func memberAvatarBaseRules() css.Sheet {
	return css.New(tw.InlineFlex, tw.ItemsCenter, tw.JustifyCenter, tw.RoundedFull, tw.W8, tw.H8, tw.Text12, tw.FontSemibold)
}

func memberInitials(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	if len(words) == 0 {
		return ""
	}
	first := []rune(words[0])
	initials := string(unicode.ToUpper(first[0]))
	if len(words) > 1 {
		last := []rune(words[len(words)-1])
		initials += string(unicode.ToUpper(last[0]))
	}
	return initials
}

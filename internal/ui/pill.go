// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"hash/fnv"
	"strconv"
	"strings"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
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
	Title  string
	Color  string
	Blank  bool
	TestID string
}

// MemberAvatar renders deterministic initials on a non-semantic member color.
// A blank name renders an empty, named avatar rather than inventing initials.
func MemberAvatar(props MemberAvatarProps) uic.Node {
	name := strings.TrimSpace(props.Name)
	attrs := []any{css.Class("cf-member-avatar", memberAvatarTone(name), memberAvatarBaseRules(), memberAvatarSheet(name, props.Color)), Attr("role", "img"), Attr("aria-label", name)}
	if props.Title != "" {
		attrs = append(attrs, Title(props.Title))
	}
	if props.TestID != "" {
		attrs = append(attrs, Attr("data-testid", props.TestID))
	}
	if name == "" || props.Blank {
		return Span(attrs...)
	}
	return Span(append(attrs, memberInitials(name))...)
}

// AvatarStackProps configures a compact, accessible stack of member avatars.
type AvatarStackProps struct {
	Members []MemberAvatarProps
	Label   string
	Title   string
	TestID  string
}

// AvatarStack shows up to three overlapping member avatars and a count chip for any remainder.
func AvatarStack(props AvatarStackProps) uic.Node {
	css.Inject("cf-member-avatar-stack", avatarStackCSS)
	label := strings.TrimSpace(props.Label)
	if label == "" {
		var names []string
		for _, member := range props.Members {
			if name := strings.TrimSpace(member.Name); name != "" {
				names = append(names, name)
			}
		}
		label = strings.Join(names, ", ")
	}
	args := []any{css.Class("cf-member-avatar-stack"), Attr("role", "img"), Attr("aria-label", label)}
	if props.Title != "" {
		args = append(args, Title(props.Title))
	}
	if props.TestID != "" {
		args = append(args, Attr("data-testid", props.TestID))
	}
	shown := len(props.Members)
	if shown > 3 {
		shown = 3
	}
	for i := 0; i < shown; i++ {
		member := props.Members[i]
		member.TestID = ""
		args = append(args, Span(css.Class("cf-member-avatar-stack-item"), Attr("aria-hidden", "true"), MemberAvatar(member)))
	}
	if extra := len(props.Members) - shown; extra > 0 {
		args = append(args, Span(css.Class("cf-member-avatar-stack-more"), Attr("aria-hidden", "true"), "+"+strconv.Itoa(extra)))
	}
	return Span(args...)
}

// MemberUnassigned renders the compact named marker used when a transaction has no owner.
func MemberUnassigned(label, testID string) uic.Node {
	css.Inject("cf-member-avatar-stack", avatarStackCSS)
	return Span(css.Class("cf-member-unassigned"), Attr("role", "img"), Attr("aria-label", label), Title(label), Attr("data-testid", testID), "—")
}

const avatarStackCSS = `
.cf-member-avatar-stack { display: inline-flex; align-items: center; white-space: nowrap; }
.cf-member-avatar-stack-item { display: inline-flex; margin-inline-start: calc(var(--space-3) * -1); border: 2px solid var(--txn-row-surface, var(--bg-card)); border-radius: 999px; }
.cf-member-avatar-stack-item:first-of-type { margin-inline-start: 0; }
.cf-member-avatar-stack-more { display: inline-flex; align-items: center; justify-content: center; min-width: var(--txn-avatar-size); height: var(--txn-avatar-size); margin-inline-start: calc(var(--space-3) * -1); border: 2px solid var(--txn-row-surface, var(--bg-card)); border-radius: 999px; background: var(--bg-elev); color: var(--text-dim); font-size: 10px; line-height: 1; }
.cf-member-unassigned { display: inline-flex; width: var(--txn-avatar-size); height: var(--txn-avatar-size); align-items: center; justify-content: center; border: 1px solid var(--border); border-radius: 999px; color: var(--text-dim); font-size: 10px; line-height: 1; }
`

func memberAvatarTone(name string) string {
	const paletteSize = 6
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(name))))
	return "cf-member-avatar--" + string(rune('1'+h.Sum32()%paletteSize))
}

func memberAvatarSheet(name, customColor string) css.Sheet {
	if background, foreground, ok := memberAvatarCustomColors(customColor); ok {
		return css.New(css.Bg(css.Color(background)), css.TextColor(css.Color(foreground)))
	}
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

func memberAvatarCustomColors(color string) (background, foreground string, ok bool) {
	color = strings.TrimSpace(color)
	if len(color) != 7 || color[0] != '#' {
		return "", "", false
	}
	rgb, err := strconv.ParseUint(color[1:], 16, 32)
	if err != nil {
		return "", "", false
	}
	r, g, b := float64((rgb>>16)&0xff), float64((rgb>>8)&0xff), float64(rgb&0xff)
	foreground = "#ffffff"
	if 0.299*r+0.587*g+0.114*b > 150 {
		foreground = "#172033"
	}
	return color, foreground, true
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

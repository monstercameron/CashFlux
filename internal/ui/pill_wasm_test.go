// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestPillToneClassesAndAccessibleBehavior(t *testing.T) {
	f := render.New(t)
	cases := []struct {
		name  string
		tone  PillTone
		class string
	}{
		{"neutral", PillNeutral, "cf-pill--neutral"},
		{"attention", PillAttention, "cf-pill--attention"},
		{"positive", PillPositive, "cf-pill--positive"},
		{"danger", PillDanger, "cf-pill--danger"},
		{"accent", PillAccent, "cf-pill--accent"},
		{"dim", PillDim, "cf-pill--dim"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f.Render(Pill(PillProps{Label: "Needs review", Tone: tc.tone}))
			pill := f.ByRole("status", "Needs review")
			if !pill.Exists() {
				t.Fatal("Pill is missing its accessible name")
			}
			if got := pill.Attr("class"); !containsClass(got, tc.class) {
				t.Fatalf("Pill tone class = %q, want %q", got, tc.class)
			}
		})
	}

	clicked := 0
	f.Render(Pill(PillProps{Label: "Dining?", AriaLabel: "Suggested category: Dining", OnClick: func() { clicked++ }}))
	action := f.ByRole("button", "Suggested category: Dining")
	if !action.Exists() {
		t.Fatal("actionable Pill is not a named button")
	}
	action.Click()
	if clicked != 1 {
		t.Fatalf("action callback count = %d, want 1", clicked)
	}
	removed := 0
	f.Render(Pill(PillProps{Label: "Groceries", OnRemove: func() { removed++ }}))
	remove := f.ByRole("button", "Remove Groceries")
	if !remove.Exists() {
		t.Fatal("removable Pill has no named remove button")
	}
	remove.Click()
	if removed != 1 {
		t.Fatalf("remove callback count = %d, want 1", removed)
	}
}

func containsClass(classList, want string) bool {
	for _, className := range strings.Fields(classList) {
		if className == want {
			return true
		}
	}
	return false
}

func TestMemberAvatarInitialsColorAndBlankName(t *testing.T) {
	f := render.New(t)
	f.Render(MemberAvatar(MemberAvatarProps{Name: "Ada Lovelace"}))
	avatar := f.ByRole("img", "Ada Lovelace")
	if !avatar.Exists() || avatar.Text() != "AL" {
		t.Fatalf("avatar initials = %q, exists=%t; want AL", avatar.Text(), avatar.Exists())
	}
	color := avatar.Attr("class")
	if !containsClass(color, memberAvatarTone("Ada Lovelace")) || memberAvatarTone("Ada Lovelace") == memberAvatarTone("Grace Hopper") {
		t.Fatalf("avatar palette is missing a stable distinct member tone: %q", color)
	}
	f.Render(MemberAvatar(MemberAvatarProps{Name: "Unassigned", Blank: true}))
	blank := f.ByRole("img", "Unassigned")
	if !blank.Exists() || blank.Text() != "" {
		t.Fatalf("blank avatar should retain its accessible name; text=%q exists=%t", blank.Text(), blank.Exists())
	}
}

func TestMemberAvatarUsesConfiguredColorAndAvatarStackCapsAtThree(t *testing.T) {
	background, foreground, ok := memberAvatarCustomColors("#7357C8")
	if !ok || background != "#7357C8" || foreground != "#ffffff" {
		t.Fatalf("custom member color = %q/%q/%t", background, foreground, ok)
	}
	if _, _, ok := memberAvatarCustomColors("url(javascript:alert(1))"); ok {
		t.Fatal("avatar accepted a non-hex custom color")
	}
	f := render.New(t)
	f.Render(AvatarStack(AvatarStackProps{
		Members: []MemberAvatarProps{{Name: "Marcus Hartley"}, {Name: "Priya Hartley"}, {Name: "Nadia Hartley"}, {Name: "Alex Hartley"}, {Name: "Sam Hartley"}},
		Label:   "Shared by Marcus Hartley, Priya Hartley, Nadia Hartley, Alex Hartley, and Sam Hartley",
	}))
	stack := f.ByRole("img", "Shared by Marcus Hartley, Priya Hartley, Nadia Hartley, Alex Hartley, and Sam Hartley")
	if !stack.Exists() || !strings.Contains(stack.Text(), "+2") {
		t.Fatalf("stack text=%q exists=%t", stack.Text(), stack.Exists())
	}
}

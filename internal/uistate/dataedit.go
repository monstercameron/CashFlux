// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/state"

// Shell-root edit selectors for the Data & People pages (members, categories,
// rules, artifacts). Each row's Edit action sets its atom; the matching
// shell-mounted host reads it and renders the editor inside a FlipPanel modal
// — rather than an inline row form, which sat under transformed bento/tile
// ancestors and broke position:fixed centering (see BudgetEditHost).
//
// The captured-atom pattern mirrors UseBudgetEdit: Use* must be called during
// a render (the shell hosts do, every frame); Set*/Close* then work from click
// handlers without calling state.UseAtom outside a render (which panics).

// Member-editor modes (which form the member flip modal shows).
const (
	MemberEditModeEdit = "edit" // full edit form (name/color/prefs/role/custom fields)
	MemberEditModePIN  = "pin"  // set/change this member's PIN
)

// MemberEdit selects the member + editor a modal should show; zero = closed.
type MemberEdit struct {
	ID   string
	Mode string
}

var (
	capturedMemberEdit state.Atom[MemberEdit]
	memberEditCaptured bool
)

// UseMemberEdit returns the shared atom selecting which member editor is open.
func UseMemberEdit() state.Atom[MemberEdit] {
	a := state.UseAtom("members:edit", MemberEdit{})
	capturedMemberEdit = a
	memberEditCaptured = true
	return a
}

// SetMemberEdit opens the member editor modal. Safe from click handlers.
func SetMemberEdit(e MemberEdit) {
	if memberEditCaptured {
		capturedMemberEdit.Set(e)
	}
}

// CloseMemberEdit dismisses the member editor modal.
func CloseMemberEdit() { SetMemberEdit(MemberEdit{}) }

var (
	capturedCategoryEdit state.Atom[string]
	categoryEditCaptured bool
)

// UseCategoryEdit returns the shared atom holding the ID of the category being
// edited ("" = closed).
func UseCategoryEdit() state.Atom[string] {
	a := state.UseAtom("categories:edit", "")
	capturedCategoryEdit = a
	categoryEditCaptured = true
	return a
}

// SetCategoryEdit opens the category editor modal for the given category.
func SetCategoryEdit(id string) {
	if categoryEditCaptured {
		capturedCategoryEdit.Set(id)
	}
}

// CloseCategoryEdit dismisses the category editor modal.
func CloseCategoryEdit() { SetCategoryEdit("") }

var (
	capturedRuleEdit state.Atom[string]
	ruleEditCaptured bool
)

// UseRuleEdit returns the shared atom holding the ID of the rule being edited
// ("" = closed).
func UseRuleEdit() state.Atom[string] {
	a := state.UseAtom("rules:edit", "")
	capturedRuleEdit = a
	ruleEditCaptured = true
	return a
}

// SetRuleEdit opens the rule editor modal for the given rule.
func SetRuleEdit(id string) {
	if ruleEditCaptured {
		capturedRuleEdit.Set(id)
	}
}

// CloseRuleEdit dismisses the rule editor modal.
func CloseRuleEdit() { SetRuleEdit("") }

var (
	capturedArtifactEdit state.Atom[string]
	artifactEditCaptured bool
)

// UseArtifactEdit returns the shared atom holding the ID of the artifact being
// renamed ("" = closed).
func UseArtifactEdit() state.Atom[string] {
	a := state.UseAtom("artifacts:edit", "")
	capturedArtifactEdit = a
	artifactEditCaptured = true
	return a
}

// SetArtifactEdit opens the artifact rename modal for the given artifact.
func SetArtifactEdit(id string) {
	if artifactEditCaptured {
		capturedArtifactEdit.Set(id)
	}
}

// CloseArtifactEdit dismisses the artifact rename modal.
func CloseArtifactEdit() { SetArtifactEdit("") }

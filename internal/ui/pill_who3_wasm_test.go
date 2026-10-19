// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"
)

func TestWHO3UnassignedMarkerUsesAvatarStackSizeAndTypography(t *testing.T) {
	for _, want := range []string{
		".cf-member-unassigned { display: inline-flex; width: var(--txn-avatar-size); height: var(--txn-avatar-size);",
		"font-size: 10px; line-height: 1;",
		".cf-member-avatar-stack-more { display: inline-flex;",
		"height: var(--txn-avatar-size);",
	} {
		if !strings.Contains(avatarStackCSS, want) {
			t.Errorf("AvatarStack styles missing shared sizing rule %q", want)
		}
	}
	if strings.Contains(avatarStackCSS, ".cf-member-avatar-stack .cf-member-avatar { width:") || strings.Contains(avatarStackCSS, "width: var(--space-") || strings.Contains(avatarStackCSS, "height: var(--space-") {
		t.Fatal("AvatarStack styles must not size components with --space-* tokens")
	}
}

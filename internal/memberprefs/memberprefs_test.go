// SPDX-License-Identifier: MIT

package memberprefs_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/memberprefs"
)

func TestResolve(t *testing.T) {
	cases := []struct {
		name       string
		member     domain.Member
		household  string
		wantStyle  string
		wantAcct   string
		wantMember string
	}{
		{
			name:       "all inherit from household",
			member:     domain.Member{ID: "m1"},
			household:  "iso",
			wantStyle:  "iso",
			wantAcct:   "",
			wantMember: "m1", // defaults to own ID
		},
		{
			name:       "member overrides date style",
			member:     domain.Member{ID: "m1", Prefs: domain.MemberPrefs{DateStyle: "us"}},
			household:  "iso",
			wantStyle:  "us",
			wantMember: "m1",
		},
		{
			name:       "member sets default account + member",
			member:     domain.Member{ID: "m1", Prefs: domain.MemberPrefs{DefaultAccountID: "a9", DefaultMemberID: "m2"}},
			household:  "long",
			wantStyle:  "long", // still inherits date style
			wantAcct:   "a9",
			wantMember: "m2",
		},
		{
			name:       "empty member date style does not shadow household",
			member:     domain.Member{ID: "m1", Prefs: domain.MemberPrefs{DateStyle: ""}},
			household:  "eu",
			wantStyle:  "eu",
			wantMember: "m1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := memberprefs.Resolve(tc.member, tc.household)
			if got.DateStyle != tc.wantStyle {
				t.Errorf("DateStyle = %q, want %q", got.DateStyle, tc.wantStyle)
			}
			if got.DefaultAccountID != tc.wantAcct {
				t.Errorf("DefaultAccountID = %q, want %q", got.DefaultAccountID, tc.wantAcct)
			}
			if got.DefaultMemberID != tc.wantMember {
				t.Errorf("DefaultMemberID = %q, want %q", got.DefaultMemberID, tc.wantMember)
			}
		})
	}
}

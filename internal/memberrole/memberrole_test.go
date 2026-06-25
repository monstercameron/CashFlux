// SPDX-License-Identifier: MIT

package memberrole_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/memberrole"
)

func TestValid(t *testing.T) {
	cases := []struct {
		role domain.MemberRole
		want bool
	}{
		{domain.RoleOwner, true},
		{domain.RoleAdmin, true},
		{domain.RoleViewer, true},
		{"", false},
		{"superuser", false},
		{"OWNER", false},
	}
	for _, tc := range cases {
		got := memberrole.Valid(tc.role)
		if got != tc.want {
			t.Errorf("Valid(%q) = %v, want %v", tc.role, got, tc.want)
		}
	}
}

func TestParseRole(t *testing.T) {
	cases := []struct {
		input   string
		want    domain.MemberRole
		wantErr bool
	}{
		{"owner", domain.RoleOwner, false},
		{"admin", domain.RoleAdmin, false},
		{"viewer", domain.RoleViewer, false},
		{"", domain.MemberRole(""), true},
		{"ADMIN", domain.MemberRole(""), true},
		{"unknown", domain.MemberRole(""), true},
	}
	for _, tc := range cases {
		got, err := memberrole.ParseRole(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseRole(%q) err = %v, wantErr %v", tc.input, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("ParseRole(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestLabel(t *testing.T) {
	cases := []struct {
		role domain.MemberRole
		want string
	}{
		{domain.RoleOwner, "Owner"},
		{domain.RoleAdmin, "Admin"},
		{domain.RoleViewer, "Viewer"},
	}
	for _, tc := range cases {
		got := memberrole.Label(tc.role)
		if got != tc.want {
			t.Errorf("Label(%q) = %q, want %q", tc.role, got, tc.want)
		}
	}
}

func TestLabelPanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Label with invalid role should panic")
		}
	}()
	memberrole.Label("bogus")
}

func TestCanManageMembers(t *testing.T) {
	cases := []struct {
		role domain.MemberRole
		want bool
	}{
		{domain.RoleOwner, true},
		{domain.RoleAdmin, false},
		{domain.RoleViewer, false},
	}
	for _, tc := range cases {
		got := memberrole.CanManageMembers(tc.role)
		if got != tc.want {
			t.Errorf("CanManageMembers(%q) = %v, want %v", tc.role, got, tc.want)
		}
	}
}

func TestCanEditEntities(t *testing.T) {
	cases := []struct {
		role domain.MemberRole
		want bool
	}{
		{domain.RoleOwner, true},
		{domain.RoleAdmin, true},
		{domain.RoleViewer, false},
	}
	for _, tc := range cases {
		got := memberrole.CanEditEntities(tc.role)
		if got != tc.want {
			t.Errorf("CanEditEntities(%q) = %v, want %v", tc.role, got, tc.want)
		}
	}
}

func TestCanViewOnly(t *testing.T) {
	cases := []struct {
		role domain.MemberRole
		want bool
	}{
		{domain.RoleOwner, false},
		{domain.RoleAdmin, false},
		{domain.RoleViewer, true},
	}
	for _, tc := range cases {
		got := memberrole.CanViewOnly(tc.role)
		if got != tc.want {
			t.Errorf("CanViewOnly(%q) = %v, want %v", tc.role, got, tc.want)
		}
	}
}

func TestDefaultRole(t *testing.T) {
	if got := memberrole.DefaultRole(true); got != domain.RoleOwner {
		t.Errorf("DefaultRole(isDefault=true) = %q, want %q", got, domain.RoleOwner)
	}
	if got := memberrole.DefaultRole(false); got != domain.RoleAdmin {
		t.Errorf("DefaultRole(isDefault=false) = %q, want %q", got, domain.RoleAdmin)
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name   string
		member domain.Member
		want   domain.MemberRole
	}{
		{
			name:   "explicit owner",
			member: domain.Member{IsDefault: true, Role: domain.RoleOwner},
			want:   domain.RoleOwner,
		},
		{
			name:   "explicit admin",
			member: domain.Member{IsDefault: false, Role: domain.RoleAdmin},
			want:   domain.RoleAdmin,
		},
		{
			name:   "explicit viewer",
			member: domain.Member{IsDefault: false, Role: domain.RoleViewer},
			want:   domain.RoleViewer,
		},
		{
			// Legacy default member with no role field — should resolve to owner.
			name:   "migration default: isDefault=true, role empty",
			member: domain.Member{IsDefault: true, Role: ""},
			want:   domain.RoleOwner,
		},
		{
			// Legacy non-default member with no role field — should resolve to admin.
			name:   "migration default: isDefault=false, role empty",
			member: domain.Member{IsDefault: false, Role: ""},
			want:   domain.RoleAdmin,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := memberrole.Resolve(tc.member)
			if got != tc.want {
				t.Errorf("Resolve(%+v) = %q, want %q", tc.member, got, tc.want)
			}
		})
	}
}

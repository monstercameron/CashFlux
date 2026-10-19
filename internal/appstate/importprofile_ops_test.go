// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/importmap"
)

func TestImportProfilesPersistWithDatasetAndStayAppScoped(t *testing.T) {
	a := newApp(t, false)
	profile := importmap.SavedProfile{ID: "chase", Profile: importmap.Profile{Name: "Chase", DateCol: 0, DescCol: 1, AmountCol: 2}}
	if _, err := a.SaveImportProfile(profile, "workspace-a"); err != nil {
		t.Fatal(err)
	}
	b := newApp(t, false)
	if got := b.ImportProfiles("workspace-a"); len(got) != 0 {
		t.Fatalf("profiles leaked across datasets: %+v", got)
	}

	data, err := a.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	c := newApp(t, false)
	if err := c.ImportJSON(data); err != nil {
		t.Fatal(err)
	}
	got := c.ImportProfiles("workspace-a")
	if len(got) != 1 || got[0].ID != profile.ID || got[0].Profile.Name != profile.Profile.Name {
		t.Fatalf("profiles after dataset round trip = %+v", got)
	}
	if got := c.ImportProfiles("workspace-b"); len(got) != 0 {
		t.Fatalf("profile crossed workspace scope: %+v", got)
	}
	other := importmap.SavedProfile{ID: profile.ID, Profile: importmap.Profile{Name: "Other bank"}}
	if _, err := c.SaveImportProfile(other, "workspace-b"); err != nil {
		t.Fatal(err)
	}
	if got := c.ImportProfiles("workspace-a"); len(got) != 1 || got[0].Profile.Name != "Chase" {
		t.Fatalf("workspace-a profiles after workspace-b save = %+v", got)
	}
	if err := c.DeleteImportProfile(profile.ID, "workspace-a"); err != nil {
		t.Fatal(err)
	}
	if got := c.ImportProfiles("workspace-a"); len(got) != 0 {
		t.Fatalf("profiles after delete = %+v", got)
	}
	if got := c.ImportProfiles("workspace-b"); len(got) != 1 || got[0].Profile.Name != "Other bank" {
		t.Fatalf("deleting workspace-a profile affected workspace-b: %+v", got)
	}
}

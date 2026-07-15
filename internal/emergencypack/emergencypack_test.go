// SPDX-License-Identifier: MIT

package emergencypack

import (
	"strings"
	"testing"
	"time"
)

func samplePack() Pack {
	return Pack{
		GeneratedAt:     time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
		OwnerName:       "Cam",
		VaultHasEntries: true,
		Accounts: []AccountEntry{{
			Name:            "Everyday Checking",
			Institution:     "Chase",
			Type:            "Checking",
			Balance:         "$2,340.00",
			BeneficiaryNote: "TOD to Jane",
			Documents:       []DocEntry{{Label: "March statement", AttachedOn: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}},
		}},
		Institutions: []InstitutionEntry{{Name: "Chase", SupportPhone: "1-800-935-9935", Accounts: []string{"Everyday Checking"}}},
		Contacts:     []Contact{{Name: "Pat Lee", Role: "Attorney", Phone: "555-1000"}},
		ClosingNote:  "You've got this.",
	}
}

// The most important guarantee: no credential-ish content leaks into either render.
func TestNoCredentialsInRender(t *testing.T) {
	p := samplePack()
	for _, doc := range []string{RenderText(p), RenderHTML(p)} {
		low := strings.ToLower(doc)
		for _, bad := range []string{"password", "pin:", "login:", "passcode"} {
			if strings.Contains(low, bad) {
				t.Errorf("pack must never contain %q", bad)
			}
		}
		// It SHOULD point to the vault without exposing it.
		if !strings.Contains(low, "vault") {
			t.Errorf("pack should mention the encrypted vault")
		}
	}
}

func TestTextRenderIncludesKeyFacts(t *testing.T) {
	out := RenderText(samplePack())
	for _, want := range []string{"Everyday Checking", "Chase", "$2,340.00", "TOD to Jane", "1-800-935-9935", "Pat Lee", "You've got this."} {
		if !strings.Contains(out, want) {
			t.Errorf("text pack missing %q", want)
		}
	}
}

func TestHTMLEscapesUserContent(t *testing.T) {
	p := samplePack()
	p.Accounts[0].Notes = "<script>alert(1)</script>"
	out := RenderHTML(p)
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Errorf("user content must be HTML-escaped")
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("expected escaped script tag in output")
	}
}

func TestNoVaultLineWhenEmpty(t *testing.T) {
	p := samplePack()
	p.VaultHasEntries = false
	out := RenderText(p)
	if !strings.Contains(out, "No saved logins are stored") {
		t.Errorf("empty-vault pack should state no logins are stored")
	}
}

func TestSortAccountsForPack(t *testing.T) {
	entries := []AccountEntry{
		{Name: "Z", Institution: "Wells"},
		{Name: "A", Institution: "Chase"},
		{Name: "B", Institution: "Chase"},
	}
	SortAccountsForPack(entries)
	if entries[0].Name != "A" || entries[1].Name != "B" || entries[2].Name != "Z" {
		t.Errorf("sort order wrong: %+v", entries)
	}
}

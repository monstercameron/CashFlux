// SPDX-License-Identifier: MIT

package i18n

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestScreensKeyCoverage guards against user-visible raw i18n keys (C335): it
// scans the wasm UI sources (internal/screens, internal/app) for uistate.T("…")
// literals plus the screen-registry Label/Title/Subtitle fields, and fails when
// a referenced key is missing from the merged English catalog — because T falls
// back to returning the key itself, which is exactly how "nav.setup" and
// "setup.welcomeTitle" shipped to the rail and the setup wizard.
//
// Limitation (documented, deliberate): keys built dynamically (assigned to a
// variable before the T call, or concatenated) are not seen by this scan; those
// call sites should keep a nearby literal alias or their own test.
func TestScreensKeyCoverage(t *testing.T) {
	// The closing quote must be followed by `,` or `)` so prefix-concatenation
	// sites (uistate.T("acctType."+kind)) — dynamic keys — are not misread as
	// a literal key "acctType.".
	tLit := regexp.MustCompile(`uistate\.T\(\s*"([^"]+)"\s*[,)]`)
	regField := regexp.MustCompile(`(?:Label|Title|Subtitle):\s*"([a-z][a-zA-Z0-9_]*\.[a-zA-Z0-9_.-]+)"`)

	used := map[string][]string{} // key -> files
	for _, dir := range []string{"../screens", "../app"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			for _, m := range tLit.FindAllStringSubmatch(string(src), -1) {
				used[m[1]] = append(used[m[1]], path)
			}
			// The screen registry's Label/Title/Subtitle flow through uistate.T
			// in the shell — a missing key there paints the nav rail raw.
			if e.Name() == "screens.go" {
				for _, m := range regField.FindAllStringSubmatch(string(src), -1) {
					used[m[1]] = append(used[m[1]], path)
				}
			}
		}
	}
	if len(used) == 0 {
		t.Fatal("scan found no uistate.T literals — the source layout moved; update the scan paths")
	}

	var missing []string
	for key, files := range used {
		if _, ok := english[key]; !ok {
			missing = append(missing, key+" (used in "+files[0]+")")
		}
	}
	if len(missing) > 0 {
		t.Errorf("%d i18n key(s) referenced by the UI are missing from the English catalog — they would render as raw key names:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

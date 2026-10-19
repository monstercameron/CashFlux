// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

func TestImportCopyExplainsSmartAndSmartPlus(t *testing.T) {
	bundle := DefaultBundle()
	for key, wantParts := range map[string][]string{
		"documents.smartBranchDesc":     {"Runs on your device", "No account or key needed"},
		"documents.smartPlusBranchDesc": {"OpenAI", "Each action explains what it sends"},
		"documents.localFirstNote":      {"Smart imports run on this device", "Smart+", "OpenAI"},
		"impc.aiDisclosure":             {"image", "OpenAI", "your key", "after you choose", "charges"},
		"impc.pdfDisclosure":            {"PDF", "OpenAI", "your key", "after you choose", "charges"},
		"impc.extractDisclosure":        {"statement text", "OpenAI", "your key", "after you choose", "charges"},
		"impc.suggestDisclosure":        {"Transaction descriptions", "category names", "OpenAI", "your key", "after you choose", "charges"},
		"imb.suggestLocal":              {"categories with rules", "on your device"},
		"imb.suggestAI":                 {"categories with AI", "Smart+"},
		"imb.disclosureDirect":          {"selected file", "transaction descriptions", "OpenAI", "Model:", "charges", "AI spend"},
		"imb.disclosureProxy":           {"selected file", "transaction descriptions", "OpenAI through your server", "Model:", "AI spend"},
		"imb.consent":                   {"Allow Smart+ to send", "Nothing is imported until you review and confirm"},
	} {
		got := bundle.T(English, key)
		if got == key || got == "" {
			t.Errorf("English import copy %q is missing", key)
			continue
		}
		for _, part := range wantParts {
			if !strings.Contains(got, part) {
				t.Errorf("English import copy %q = %q; want it to mention %q", key, got, part)
			}
		}
	}
}

func TestImportCopyUsesClearCountAndActionWording(t *testing.T) {
	bundle := DefaultBundle()
	for key, want := range map[string]string{
		"documents.preflightCounts":               "Ready to import %s. Possible duplicates: %d.",
		"documents.preflightCountsNoDuplicates":   "Ready to import %s.",
		"documents.preflightCountsWithDuplicates": "Ready to import %s. Skip duplicate rows (%d).",
		"documents.preflightImportNow":            "Import rows (%d)",
		"impc.importAllRows":                      "Import rows (%d)",
		"impc.importOneToAccount":                 "Import 1 transaction to %s",
		"impc.importManyToAccount":                "Import %d transactions to %s",
		"impc.close":                              "Close",
		"documents.rollbackConfirm":               "Roll back this import? Your data returns to the moment before it ran — the imported rows and anything you changed since will be lost.",
		"documents.wizardOrAI":                    "Not sure? Map the columns below, or choose \"Extract with AI\" above.",
		"orderimport.parsed":                      "Orders found: %d. Matches appear below.",
		"orderimport.drift":                       "Amounts differ by %s. Could a gift card, promo, or partial charge explain the gap?",
		"documents.csvEmpty":                      "No transaction rows found. Add rows below the header or choose another file.",
		"documents.csvHeaderOnly":                 "Only a header row was found. Add transaction rows below it.",
		"documents.csvNotRecognized":              "These columns weren't recognized. Choose Statement text to map them.",
		"documents.stmtNoneFound":                 "No transactions found. Check that each row has a date, description, and amount.",
		"documents.typeCsvDesc":                   "Import a bank CSV with date, payee, and amount columns.",
		"documents.typeStmtDesc":                  "Paste bank text and map columns when needed.",
		"documents.typePdfDesc":                   "Send a PDF statement to OpenAI to extract transactions.",
		"documents.typeReceiptDesc":               "Send a receipt image to OpenAI to extract its items.",
		"documents.smartBranchTier":               "On your device",
		"documents.smartPlusBranchTier":           "Uses AI",
		"imb.directKeyRequired":                   "This Smart+ source currently needs your own OpenAI key in Settings. The on-device parser works without a key.",
		"documents.rollbackAction":                "Roll back",
		"documents.preflightRepeated":             "Repeated rows in this file: %d. Only the first occurrence is imported.",
		"documents.preflightUnusable":             "Rows that could not be read: %d. No transactions will be added for them.",
	} {
		if got := bundle.T(English, key); got != want {
			t.Errorf("English import copy %q = %q, want %q", key, got, want)
		}
	}

	for _, key := range []string{
		"documents.smartBranchTier", "documents.smartPlusBranchTier",
		"documents.historySkipped", "documents.historyDuplicates", "documents.historyFailed",
		"documents.historyRemoved", "documents.historyUntraceable",
		"imb.suggestLocal", "imb.suggestAI", "imb.profileMeta",
	} {
		if got := bundle.T(English, key); strings.Contains(got, "·") || strings.Contains(got, "(s)") {
			t.Errorf("English import copy %q contains disallowed compact or placeholder wording: %q", key, got)
		}
	}
}

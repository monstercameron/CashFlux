// SPDX-License-Identifier: MIT

package i18n

// accountsFilingKeys holds the English strings for the /accounts "filing cabinet"
// arc: the institution directory (AC10), the account documents drawer + renewal
// reminders (AC8/AC17), the estate emergency pack + beneficiary notes (AC16), the
// idle-cash benchmark line (AC15), and the revaluation-cadence override (AC5). Kept
// in their own file and merged via init so this does not touch the shared en.go.
var accountsFilingKeys = Catalog{
	// AC10 — institution directory.
	"accounts.institutionsAction":      "Institutions",
	"accounts.institutionsManageTitle": "Manage your institutions",
	"accounts.institutionsHint":        "Add each bank, brokerage, or lender once, then link your accounts to it — a colored tag helps you spot which accounts sit where.",
	"accounts.institutionsEmpty":       "You haven't added any institutions yet.",
	"accounts.addInstitution":          "Add institution",
	"accounts.editInstitution":         "Edit institution",
	// %s = number of accounts linked, e.g. "3 accounts".
	"accounts.institutionAccountCount": "%s linked",
	"accounts.institutionNameLabel":    "Name",
	"accounts.institutionNamePh":       "Chase, Fidelity, Ally…",
	"accounts.institutionNameRequired": "Give the institution a name.",
	"accounts.institutionColorLabel":   "Color tag",
	"accounts.institutionPhoneLabel":   "Support phone",
	"accounts.institutionPhonePh":      "(optional)",
	"accounts.institutionURLLabel":     "Support website",
	"accounts.institutionURLPh":        "https://…",
	"accounts.institutionNoteLabel":    "Note",
	"accounts.institutionNotePh":       "Branch, account manager, reminders…",
	"accounts.institutionSaved":        "Institution saved.",
	// %s = institution name.
	"accounts.institutionDeleted":        "Removed \"%s\". Its accounts now show no institution.",
	"accounts.institutionDeleteConfirm":  "Delete \"%s\"? Its accounts will show no institution — nothing else changes.",
	"accounts.deleteInstitution":         "Delete institution",
	"accounts.institutionChipAria":       "Institution: %s",
	"accounts.institutionNone":           "No institution",
	"accounts.institutionDirectoryLabel": "Institution",
	"accounts.manageInstitutionsLink":    "Manage institutions",

	// AC8 / AC17 — account documents drawer + renewal reminders.
	"accounts.documentsSection": "Documents",
	// %d = number of documents filed.
	"accounts.documentsToggleShow":   "Documents (%d)",
	"accounts.documentsToggleHide":   "Hide documents",
	"accounts.documentsEmpty":        "No documents filed yet.",
	"accounts.attachDocument":        "Attach a document",
	"accounts.documentLabelField":    "What is this? (optional)",
	"accounts.documentLabelPh":       "Auto insurance policy, March statement…",
	"accounts.documentExpiryField":   "Renews or expires on (optional)",
	"accounts.documentExpiryHint":    "PDF or image files. Add a renewal date and we'll remind you a little before it's due.",
	"accounts.documentAttached":      "Document attached.",
	"accounts.documentRemoved":       "Document removed.",
	"accounts.documentRemoveConfirm": "Remove this document? Any renewal reminder for it will clear too.",
	"accounts.documentOpen":          "Open",
	"accounts.documentRemove":        "Remove",
	// %s = date, e.g. "Filed Jul 14, 2026".
	"accounts.documentFiledOn": "Filed %s",
	// %s = date, e.g. "Renews Jul 14, 2027".
	"accounts.documentExpiresOn": "Renews %s",

	// AC16 — beneficiary notes + estate emergency pack.
	"accounts.beneficiaryNoteLabel": "Who inherits this account?",
	"accounts.beneficiaryNoteHint":  "A beneficiary or transfer-on-death note, in your own words — for your records only. Never put a password here.",
	"accounts.beneficiaryNotePh":    "e.g. \"Named my sister as TOD beneficiary at the bank, 2024.\"",
	"settings.emergencyPackTitle":   "Emergency pack",
	"settings.emergencyPackHint":    "A calm, plain-language document — generated on this device, never uploaded — for someone who needs to step in for you: a spouse, a family member, an executor. It lists your accounts, balances, institutions, and any beneficiary notes you've left. It NEVER includes passwords.",
	"settings.emergencyPackBtn":     "Create emergency pack",
	"settings.emergencyPackConfirm": "This creates a document listing your account names, balances, and institutions for someone who may need to step in for you. It stays on this device — nothing is uploaded, and it never includes passwords. Continue?",
	"settings.emergencyPackDone":    "Emergency pack saved to your downloads.",

	// AC15 — idle-cash benchmark.
	"settings.idleCashBenchmarkLabel": "Idle-cash benchmark rate (%)",
	"settings.idleCashBenchmarkHint":  "What a high-yield savings account or money-market fund could pay you — your own assumption, not a live rate. Leave blank to hide the idle-cash figure.",
	// %s (1) = idle cash amount, %s (2) = yearly amount that could be earned, %s (3) = the benchmark percent.
	"accounts.idleCashLine": "~%s sitting idle · could earn ~%s/yr at your %s%% benchmark",
	"accounts.idleCashLink": "See allocation options",

	// AC5 — revaluation cadence override.
	"accounts.revalueDaysLabel": "Re-estimate every (days)",
	"accounts.revalueDaysPh":    "Leave blank for the default",
	"accounts.revalueDaysHint":  "How often to refresh this estimate — leave blank to use the usual schedule for this account type.",
}

func init() {
	for k, v := range accountsFilingKeys {
		english[k] = v
	}
}

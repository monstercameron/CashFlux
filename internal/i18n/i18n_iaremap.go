// SPDX-License-Identifier: MIT

package i18n

// iaRemapKeys holds English strings introduced by the FEATURE_MAP §5.6
// IA-remap capstone: new hub routes (/assistant, /household, /studio) and the
// updated Tools rail sub-group labels. Registered at init time using the
// init-merge pattern so en.go (which may carry concurrent WIP) is never touched.
//
// Pre-conditions verified before adding each key:
//   - nav.assistant / screen.assistantSub  → defined in i18n_assistant.go (DO NOT redefine)
//   - rail.subBuild = "Build"              → defined in en.go (DO NOT redefine; reused as-is)
//   - rail.subBills / rail.subPlan / rail.subData → defined in en.go but carry old labels;
//     new nav.toolsPlan/nav.toolsData keys shadow them for the rail header display only.
var iaRemapKeys = Catalog{
	// /household hub — nav label and page subtitle.
	"nav.household":    "Household",
	"screen.householdSub": "Members, shared expenses, and per-person views",

	// /studio hub — nav label and page subtitle.
	"nav.studio":    "Studio",
	"screen.studioSub": "Build widgets, manage your dashboard, and custom pages",

	// Tools rail sub-group header labels (updated for the new grouping).
	// nav.assistant is already in i18n_assistant.go — not redefined here.
	"nav.toolsUnderstand": "Understand",
	"nav.toolsPlan":       "Plan & forecast",
	"nav.toolsData":       "Data & people",
}

func init() {
	for k, v := range iaRemapKeys {
		english[k] = v
	}
}

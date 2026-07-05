// SPDX-License-Identifier: MIT

package i18n

// settingsSurfaceKeys holds the English strings for the tabbed global settings
// panel and the redesigned system pages. Merged via init so this file does not
// touch en.go.
var settingsSurfaceKeys = Catalog{
	"settings.tabsAria":     "Settings sections",
	"settings.tabHousehold": "Household",
	"settings.tabPrefs":     "Preferences",
	"settings.tabAlerts":    "Alerts",
	"settings.tabAI":        "AI",
	"settings.tabCloud":     "Cloud",
	"settings.tabData":      "Data",
	"settings.tabAdvanced":  "Advanced",
	"topbar.settings":       "Settings",
	"screen.settingsSub":    "Everything configurable, in one place — household, preferences, alerts, AI, cloud, data, and advanced.",

	"about.eyebrow":         "local-first budgeting · built in the open",
	"about.chipStorage":     "Your data lives",
	"about.chipStorageVal":  "On this device",
	"about.chipTracking":    "Tracking",
	"about.chipTrackingVal": "None",
	"about.chipCloud":       "Cloud sync",
	"about.chipCloudVal":    "Off by default",

	"help.heroTitle":      "Help & getting started",
	"help.heroLabel":      "Setup steps done",
	"help.chipShortcut":   "Shortcut list",
	"help.chipPalette":    "Command palette",
	"help.chipOffline":    "Works offline",
	"help.chipOfflineVal": "Yes",
	"help.takeAllSet":     "You're all set up — nice work. Everything below is reference when you need it.",
	"help.takeRemaining":  "%d setup steps left — the checklist below links straight to each one.",

	"appearance.eyebrow":     "theme, motion, and accent — applied live, saved on this device",
	"appearance.heroTitle":   "Make it yours",
	"appearance.modeTitle":   "Mode, motion & accent",
	"appearance.editorTitle": "Theme editor",
	"appearance.takeaway":    "You're in %s mode with %s motion. Every change below applies instantly.",
}

func init() {
	for k, v := range settingsSurfaceKeys {
		english[k] = v
	}
}

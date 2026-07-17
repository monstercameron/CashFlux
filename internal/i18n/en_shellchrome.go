// SPDX-License-Identifier: MIT

package i18n

// shellChromeKeys are the compact shell-chrome strings from the 2026-07-17 visual
// audit (Cloud rail row, sample-data chip menu), kept separate from en.go
// (concurrent WIP) like the other feature key files.
var shellChromeKeys = Catalog{
	// One-line Cloud rail row: the whole row links to /plans; the ✕ snoozes.
	"cloud.rowLabel": "CashFlux Cloud",
	"cloud.rowTitle": "Sync, backup, and AI across your devices — the app stays free and local either way. Learn more on the Plans page.",
}

func init() {
	for k, v := range shellChromeKeys {
		english[k] = v
	}
}

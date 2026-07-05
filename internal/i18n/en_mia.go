// SPDX-License-Identifier: MIT

// Package i18n — MIA-extend additional keys (#445).
//
// Pattern: add new keys here only; init() merges them into the english
// catalog without touching the dirty en.go which is under concurrent WIP.
package i18n

var miaKeys = Catalog{
	// Dashboard scope (#445-8)
	"dashboard.householdTotal": "vs household total: %s",

	// Accounts — institution field (#445-10)
	"accounts.institution":     "Institution",
	"accounts.institutionHint": "Bank or institution",
	"accounts.setInstitution":  "Set institution",

	// Insights scope chip (#445-9)
	"insights.scopeChangeReports": "Change scope in Reports →",
	"insights.scopeNotice":        "Scoped view",
}

func init() {
	for k, v := range miaKeys {
		english[k] = v
	}
}

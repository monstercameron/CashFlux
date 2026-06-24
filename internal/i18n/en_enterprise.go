// SPDX-License-Identifier: MIT

package i18n

// adminKeys is the enterprise/admin set of English strings registered into the
// source catalog at init time — kept separate from en.go (user WIP) so EC3
// changes do not land in the user's working tree.
var adminKeys = Catalog{
	// Nav + screen chrome
	"nav.admin":        "Admin",
	"screen.adminSub":  "Operator overview and user management",

	// Overview card labels
	"admin.totalUsers":    "Total users",
	"admin.subsActive":    "Active",
	"admin.subsTrialing":  "Trialing",
	"admin.subsPastDue":   "Past due",
	"admin.subsCanceled":  "Canceled",
	"admin.estimatedMRR":  "Est. MRR",
	"admin.totalStorage":  "Total storage",
	"admin.todayRequests": "Requests today",
	"admin.todayTokens":   "Tokens today",
	"admin.dayLabel":      "Stats as of",

	// Users table headers
	"admin.colEmail":    "Email",
	"admin.colProvider": "Provider",
	"admin.colPlan":     "Plan",
	"admin.colStatus":   "Status",
	"admin.colCreated":  "Joined",

	// Empty / access states
	"admin.signInPrompt":    "Sign in to the cloud to view the admin console.",
	"admin.accessDenied":    "Admin access only. Your account does not have operator permissions.",
	"admin.noUsers":         "No users found.",

	// Loading / error
	"admin.loading":          "Loading admin data…",
	"admin.loadingOverview":  "Loading overview",
	"admin.loadingUsers":     "Loading users",
	"admin.errorOverview":    "Could not load overview.",
	"admin.errorUsers":       "Could not load users.",
	"admin.retry":            "Retry",

	// Section headings
	"admin.overviewTitle": "Platform overview",
	"admin.usersTitle":    "Users",
}

func init() {
	for k, v := range adminKeys {
		english[k] = v
	}
}

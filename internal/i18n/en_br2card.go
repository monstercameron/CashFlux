// SPDX-License-Identifier: MIT

package i18n

var br2CardKeys = Catalog{
	"budgets.goalFundedAvailableOne":  "%s from the %s goal is available here this month; this budget's limit remains %s.",
	"budgets.goalFundedAvailableMany": "%s from these goals (%s) is available here this month; this budget's limit remains %s.",
	"budgets.goalFundedUnnamed":       "saved goal",
}

func init() {
	for key, value := range br2CardKeys {
		english[key] = value
	}
}

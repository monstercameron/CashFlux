// SPDX-License-Identifier: MIT

package widgetcfg

// Built-in widget settings schemas. Each dashboard widget that exposes settings
// registers here; the flip panel renders these and the widget reads its values.
// Add a widget's feasible settings by appending a Schema in init.
func init() {
	// KPI tiles are formula-driven and programmable: each computes its figure by
	// evaluating a CONFIGURABLE expression over the engine variable surface
	// (internal/engineenv) — net_worth, income, safe_to_spend, savings_rate, and
	// cf_* custom fields, all derived from fundamental sources (transactions,
	// accounts, bills, goals). The seeded formula reproduces the built-in figure;
	// the user can rewrite it (and the display format) in the tile's settings.
	formatField := Field{Key: "format", Label: "Display as", Type: Select, Default: "currency", Options: []Option{
		{Value: "currency", Label: "Currency"},
		{Value: "percent", Label: "Percent"},
		{Value: "number", Label: "Number"},
	}}
	kpi := func(id, title, formula, format string) {
		ff := formatField
		ff.Default = format
		register(Schema{
			WidgetID: id, Title: title,
			Fields: []Field{
				{Key: "formula", Label: "Formula (over engine variables)", Type: Text, Default: formula},
				ff,
			},
		})
	}
	kpi("kpi-networth", "Net worth", "net_worth", "currency")
	kpi("kpi-assets", "Assets", "assets", "currency")
	kpi("kpi-liabilities", "Liabilities", "liabilities", "currency")
	kpi("kpi-income", "Income", "income", "currency")
	kpi("kpi-spending", "Spending", "expense", "currency")
	kpi("kpi-safetospend", "Safe to spend", "safe_to_spend", "currency")

	register(Schema{
		WidgetID: "savings",
		Title:    "Savings rate",
		Fields: []Field{
			// Programmable like the KPI tiles: the rate is a configurable formula over
			// the engine variables (default = (income−expense)/income, clamped).
			{Key: "formula", Label: "Formula (over engine variables)", Type: Text, Default: "clamp(safediv(income - expense, income, 0) * 100, -100, 100)"},
			{Key: "target", Label: "Target savings rate", Type: Number, Default: "20", Unit: "%", Min: 0, Max: 100},
			{Key: "showBar", Label: "Show progress bar", Type: Toggle, Default: "true"},
		},
	})
	// display controls a list tile's overflow behavior when there are more rows
	// than fit: cap (top N), scroll (overflow-scroll inside the tile), or page
	// (prev/next pager). The renderers and the data pipeline both honor it.
	displayOpts := []Option{
		{Value: "cap", Label: "Show the top few"},
		{Value: "scroll", Label: "Scroll inside the tile"},
	}
	displayPaged := append(append([]Option{}, displayOpts...), Option{Value: "page", Label: "Page through them"})

	register(Schema{
		WidgetID: "recent",
		Title:    "Recent transactions",
		Fields: []Field{
			{Key: "count", Label: "Rows per page", Type: Number, Default: "6", Min: 3, Max: 20},
			{Key: "display", Label: "When there are more", Type: Select, Default: "page", Options: displayPaged},
		},
	})
	register(Schema{
		WidgetID: "trend",
		Title:    "Net worth trend",
		Fields: []Field{
			{Key: "months", Label: "History window", Type: Number, Default: "6", Unit: "months", Min: 3, Max: 120},
			{Key: "showXAxis", Label: "Show time labels", Type: Toggle, Default: "true"},
		},
	})
	register(Schema{
		WidgetID: "breakdown",
		Title:    "Spending breakdown",
		Fields: []Field{
			{Key: "topN", Label: "Top categories (rest grouped as Other)", Type: Number, Default: "3", Min: 2, Max: 6},
		},
	})
	register(Schema{
		WidgetID: "todo",
		Title:    "To-do",
		Fields: []Field{
			{Key: "count", Label: "Tasks to show", Type: Number, Default: "3", Min: 1, Max: 10},
			{Key: "sort", Label: "Order", Type: Select, Default: "smart", Options: []Option{
				{Value: "smart", Label: "Smart"}, {Value: "priority", Label: "Priority"},
				{Value: "az", Label: "A–Z"}, {Value: "due", Label: "Due date"},
			}},
			{Key: "showCompleted", Label: "Show completed", Type: Toggle, Default: "false"},
		},
	})
	register(Schema{
		WidgetID: "accounts",
		Title:    "Accounts",
		Fields: []Field{
			{Key: "count", Label: "Accounts to show", Type: Number, Default: "6", Min: 3, Max: 12},
			{Key: "cleared", Label: "Show cleared balance only", Type: Toggle, Default: "false"},
			{Key: "display", Label: "When there are more", Type: Select, Default: "scroll", Options: displayOpts},
		},
	})
	register(Schema{
		WidgetID: "budgets",
		Title:    "Budgets",
		Fields: []Field{
			{Key: "count", Label: "Budgets to show", Type: Number, Default: "6", Min: 3, Max: 20},
			{Key: "atRisk", Label: "Show only near or over budget", Type: Toggle, Default: "false"},
			{Key: "display", Label: "When there are more", Type: Select, Default: "scroll", Options: displayOpts},
		},
	})
	register(Schema{
		WidgetID: "bills",
		Title:    "Upcoming bills",
		Fields: []Field{
			{Key: "display", Label: "When there are more", Type: Select, Default: "scroll", Options: displayOpts},
		},
	})
	register(Schema{
		WidgetID: "goals",
		Title:    "Goals",
		Fields: []Field{
			{Key: "byProgress", Label: "Feature the goal nearest completion", Type: Toggle, Default: "false"},
			{Key: "showDate", Label: "Show the target date", Type: Toggle, Default: "true"},
		},
	})
	register(Schema{
		WidgetID: "attention",
		Title:    "Needs attention",
		Fields: []Field{
			{Key: "bills", Label: "Bills due soon", Type: Toggle, Default: "true"},
			{Key: "budgets", Label: "Budget alerts (near or over)", Type: Toggle, Default: "true"},
			{Key: "stale", Label: "Stale account balances", Type: Toggle, Default: "true"},
			{Key: "tasks", Label: "Overdue & high-priority to-dos", Type: Toggle, Default: "true"},
			{Key: "spending", Label: "Biggest spending spike", Type: Toggle, Default: "true"},
			{Key: "billsDays", Label: "Flag bills due within", Type: Number, Default: "7", Unit: "days", Min: 1, Max: 60},
			{Key: "maxItems", Label: "Most you'll see at once", Type: Number, Default: "5", Min: 1, Max: 12},
			{Key: "minSeverity", Label: "Only show", Type: Select, Default: "all", Options: []Option{
				{Value: "all", Label: "Everything"},
				{Value: "warn", Label: "Warnings & critical"},
				{Value: "critical", Label: "Critical only"},
			}},
		},
	})
}

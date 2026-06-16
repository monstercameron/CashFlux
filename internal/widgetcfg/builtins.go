package widgetcfg

// Built-in widget settings schemas. Each dashboard widget that exposes settings
// registers here; the flip panel renders these and the widget reads its values.
// Add a widget's feasible settings by appending a Schema in init.
func init() {
	register(Schema{
		WidgetID: "savings",
		Title:    "Savings rate",
		Fields: []Field{
			{Key: "target", Label: "Target savings rate", Type: Number, Default: "20", Unit: "%", Min: 0, Max: 100},
			{Key: "showBar", Label: "Show progress bar", Type: Toggle, Default: "true"},
		},
	})
}

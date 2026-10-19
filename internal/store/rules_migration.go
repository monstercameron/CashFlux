package store

import "github.com/monstercameron/CashFlux/internal/rules"

const ruleSemanticsMigrationKey = "migration.rules.signed-amount.v1"
const legacyRuleSemanticsMigrationKey = "migration.rules.signed-amount-and-substring.v1"

// migrateLegacyRuleSemantics upgrades rules once and records the migration in
// the dataset so later loads and exports do not reinterpret edited conditions.
func migrateLegacyRuleSemantics(ds Dataset) Dataset {
	if ds.SchemaVersion == 0 || ds.SchemaVersion >= 2 {
		return ds
	}
	if ds.SettingsKV != nil && (ds.SettingsKV[ruleSemanticsMigrationKey] == "1" || ds.SettingsKV[legacyRuleSemanticsMigrationKey] == "1") {
		if ds.SettingsKV[ruleSemanticsMigrationKey] != "1" {
			ds.SettingsKV[ruleSemanticsMigrationKey] = "1"
		}
		return ds
	}
	ds.Rules, _ = rules.MigrateLegacyRuleSemantics(ds.Rules)
	if ds.SettingsKV == nil {
		ds.SettingsKV = make(map[string]string)
	}
	ds.SettingsKV[ruleSemanticsMigrationKey] = "1"
	return ds
}

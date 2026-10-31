// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

// debtConfigKey is the app-KV key holding the debt-page display/policy config.
const debtConfigKey = "cashflux:debt:config"

// DebtConfigGet returns the effective debt config: the persisted override layered over the
// defaults (so a partial/old stored config still gets sane values for any missing field),
// or the plain defaults when nothing is stored.
func DebtConfigGet() DebtConfig {
	cfg, err := ParseDebtConfig(kvGet(debtConfigKey))
	if err != nil {
		return defaultDebtConfig()
	}
	return cfg
}

// SetDebtConfig persists a debt config override. Passing the zero value clears the
// override (the next read falls back to the defaults). It requests an immediate
// dataset flush so a tuning change survives a reload that lands within the autosave
// tick — the debt tuner writes here on every click.
func SetDebtConfig(cfg DebtConfig) {
	if cfg == (DebtConfig{}) {
		kvSet(debtConfigKey, "")
		RequestPersist()
		return
	}
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(debtConfigKey, string(data))
		RequestPersist()
	}
}

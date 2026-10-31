// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "encoding/json"

const billsSmartConfigKey = "cashflux:bills:smartconfig"

// BillsSmartConfigGet returns the effective config: the persisted override
// layered over the defaults.
func BillsSmartConfigGet() BillsSmartConfig {
	cfg, err := ParseBillsSmartConfig(kvGet(billsSmartConfigKey))
	if err != nil {
		return defaultBillsSmartConfig()
	}
	return cfg
}

// SetBillsSmartConfig persists the smart-schedule config.
func SetBillsSmartConfig(cfg BillsSmartConfig) {
	if data, err := json.Marshal(cfg); err == nil {
		kvSet(billsSmartConfigKey, string(data))
	}
}

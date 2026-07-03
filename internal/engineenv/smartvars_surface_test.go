// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
)

// TestAddSmartVars: the smart_* posture counts flow through Vars and always exist.
func TestAddSmartVars(t *testing.T) {
	v := Vars(Data{Rates: currency.Rates{Base: "USD"}, Now: time.Now(),
		Smart: SmartCounts{FreeOn: 20, AIOn: 6}})
	if v["smart_free_on"] != 20 || v["smart_ai_on"] != 6 || v["smart_features_on"] != 26 {
		t.Errorf("smart vars = %v/%v/%v, want 20/6/26",
			v["smart_free_on"], v["smart_ai_on"], v["smart_features_on"])
	}
	empty := Vars(Data{Rates: currency.Rates{Base: "USD"}, Now: time.Now()})
	for _, k := range SmartVarNames {
		if _, ok := empty[k]; !ok {
			t.Errorf("%s should always be present", k)
		}
	}
}

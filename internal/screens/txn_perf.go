// SPDX-License-Identifier: MIT

package screens

import "time"

// txnPerfObserve is installed only by the wasm profiling harness. Production
// calls are no-ops, so collecting evidence does not require a separate app build.
var txnPerfObserve func(stage string, elapsed time.Duration)

func txnPerfStart(stage string) func() {
	observe := txnPerfObserve
	if observe == nil {
		return func() {}
	}
	start := time.Now()
	return func() { observe(stage, time.Since(start)) }
}

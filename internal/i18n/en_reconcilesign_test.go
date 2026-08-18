// SPDX-License-Identifier: MIT

package i18n

import "testing"

// The reconcile dialog picks its sign-hint key through a helper
// (screens.reconSignHintKey) rather than writing a literal at the call site, so
// TestScreensKeyCoverage cannot see these two. Its own doc names that limitation
// and asks such call sites to bring their own test. This is it: a missing key
// renders as the raw key string, which on this dialog would mean shipping
// "accounts.reconSignHintDebt" where the sign convention should be.
func TestReconcileSignHintKeysExist(t *testing.T) {
	for _, key := range []string{
		"accounts.reconSignHintAsset",
		"accounts.reconSignHintDebt",
		"accounts.reconCancelConfirm",
		"accounts.reconCancelDone",
		"accounts.reconCancelPartial",
	} {
		got, ok := english[key]
		if !ok {
			t.Errorf("%s is missing from the English catalog", key)
			continue
		}
		if got == "" || got == key {
			t.Errorf("%s = %q, which is not a sentence", key, got)
		}
	}
}

// The worksheet picks its residual key by sign at the call site, so the coverage
// scan cannot see either one (C690).
func TestWorksheetResidualKeysExist(t *testing.T) {
	for _, key := range []string{"worksheet.residualHigher", "worksheet.residualLower"} {
		got, ok := english[key]
		if !ok || got == "" || got == key {
			t.Errorf("%s = %q ok=%v, which is not a sentence", key, got, ok)
		}
	}
}

// SPDX-License-Identifier: MIT

package domain

// RegularSpendOverrides stores the household's corrections to the regular
// monthly spend baseline. These choices do not change reports or budgets.
type RegularSpendOverrides struct {
	// EssentialCategoryIDs are category roots counted as variable essentials.
	EssentialCategoryIDs []string `json:"essentialCategoryIds,omitempty"`
	// OneOffTransactionIDs are omitted from the baseline while remaining in reports.
	OneOffTransactionIDs []string `json:"oneOffTransactionIds,omitempty"`
	// TreatMerchantRegular contains normalized merchant keys accepted as regular.
	TreatMerchantRegular []string `json:"treatMerchantRegular,omitempty"`
}

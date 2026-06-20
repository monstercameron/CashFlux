package payoff

import "time"

// DebtFreeMonth turns a payoff month-count into the calendar month the final
// payment lands, so the UI can show "debt-free by <Month Year>" instead of a bare
// month count. It returns the first day of start's month advanced by months-1 (the
// month of the last payment); months<=0 (nothing owed) returns start's month.
func DebtFreeMonth(start time.Time, months int) time.Time {
	first := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
	if months <= 0 {
		return first
	}
	return first.AddDate(0, months-1, 0)
}

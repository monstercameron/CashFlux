// SPDX-License-Identifier: MIT

package appstate

// TransactionDurability identifies one pending or completed transaction save.
// Operation distinguishes overlapping requests for the same row. TransactionIDs
// is owned by the observer and may be retained without aliasing another observer.
type TransactionDurability struct {
	Operation      uint64
	TransactionIDs []string
	Pending        bool
	Result         DurableResult
}

// OnTransactionDurability observes pending/completed row saves. It returns a
// cancellation function so a UI scope cannot keep observing a previous App.
func (a *App) OnTransactionDurability(fn func(TransactionDurability)) func() {
	if fn == nil {
		return func() {}
	}
	index := len(a.transactionDurabilityObservers)
	a.transactionDurabilityObservers = append(a.transactionDurabilityObservers, fn)
	return func() { a.transactionDurabilityObservers[index] = nil }
}

func (a *App) publishTransactionDurability(event TransactionDurability) {
	for _, observer := range a.transactionDurabilityObservers {
		if observer != nil {
			copy := event
			copy.TransactionIDs = append([]string(nil), event.TransactionIDs...)
			observer(copy)
		}
	}
}

// RequestTransactionsDurable binds row pending-state to the same durable result
// as forms and undo. A pending event never acknowledges a write; completion is
// emitted only when the storage writer resolves the request.
func (a *App) RequestTransactionsDurable(ids []string, done func(DurableResult)) {
	a.transactionDurabilityOperation++
	event := TransactionDurability{Operation: a.transactionDurabilityOperation, TransactionIDs: append([]string(nil), ids...), Pending: true}
	a.publishTransactionDurability(event)
	a.RequestDurable(func(result DurableResult) {
		event.Pending, event.Result = false, result
		a.publishTransactionDurability(event)
		if done != nil {
			done(result)
		}
	})
}

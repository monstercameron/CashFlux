//go:build js && wasm

package screens

import "testing"

func TestBC6N5BudgetLimitSavesKeepLatestPendingAmount(t *testing.T) {
	queue := &budgetLimitSaveQueue{}
	first := budgetLimitSaveRequest{amount: 100}
	second := budgetLimitSaveRequest{amount: 200}
	latest := budgetLimitSaveRequest{amount: 300}

	if !queue.enqueue(first) {
		t.Fatal("first save did not start")
	}
	if queue.enqueue(second) {
		t.Fatal("second save started before the first completed")
	}
	if queue.enqueue(latest) {
		t.Fatal("newest pending save started before the first completed")
	}

	got, ok := queue.complete()
	if !ok || got.amount != latest.amount {
		t.Fatalf("first completion returned pending save %#v, %v; want amount %d", got, ok, latest.amount)
	}
	if !queue.active {
		t.Fatal("queue became inactive before the pending save completed")
	}

	if _, ok := queue.complete(); ok {
		t.Fatal("queue returned an unexpected extra pending save")
	}
	if queue.active {
		t.Fatal("queue stayed active after all saves completed")
	}
}

func TestBC6N5BudgetLimitSaveCanQueueAgainWhilePendingSaveRuns(t *testing.T) {
	queue := &budgetLimitSaveQueue{}
	queue.enqueue(budgetLimitSaveRequest{amount: 100})
	queue.enqueue(budgetLimitSaveRequest{amount: 200})

	firstPending, ok := queue.complete()
	if !ok || firstPending.amount != 200 {
		t.Fatalf("first completion returned %#v, %v; want amount 200", firstPending, ok)
	}

	queue.enqueue(budgetLimitSaveRequest{amount: 300})
	secondPending, ok := queue.complete()
	if !ok || secondPending.amount != 300 {
		t.Fatalf("second completion returned %#v, %v; want amount 300", secondPending, ok)
	}
}

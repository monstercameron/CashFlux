// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"
)

func TestUseAfterSettleTimerDoesNotRunAfterUnmount(t *testing.T) {
	called := make(chan struct{}, 1)
	cancel := afterSettleTimer(10*time.Millisecond, func() { called <- struct{}{} })
	cancel()
	time.Sleep(25 * time.Millisecond)
	select {
	case <-called:
		t.Fatal("settle callback ran after cleanup")
	default:
	}
}

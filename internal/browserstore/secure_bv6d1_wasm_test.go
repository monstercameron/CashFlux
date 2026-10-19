// SPDX-License-Identifier: MIT

//go:build js && wasm

package browserstore

import (
	"errors"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

func TestRecordChangedErrorsIdentifyKeyAndCASStage(t *testing.T) {
	for _, stage := range []string{"plaintext generation condition", "encrypted prior-record comparison"} {
		t.Run(stage, func(t *testing.T) {
			const key = "cashflux:dataset:gen"
			err := recordChangedAt(key, stage)
			if !errors.Is(err, atrest.ErrRecordChanged) {
				t.Fatalf("wrapped error lost ErrRecordChanged identity: %v", err)
			}
			if !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), stage) {
				t.Fatalf("error lacks key or comparison stage: %v", err)
			}
			if strings.Contains(err.Error(), "generation-value") || strings.Contains(err.Error(), "ciphertext") {
				t.Fatalf("error exposed record data: %v", err)
			}
		})
	}
}

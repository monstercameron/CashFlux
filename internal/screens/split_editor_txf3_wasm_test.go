// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestSplitEditorRemainderIncludesTransactionCurrency(t *testing.T) {
	for _, tc := range []struct {
		name      string
		split     int64
		wantMinor int64
	}{
		{name: "left", split: -600, wantMinor: 1400},
		{name: "over", split: -2500, wantMinor: 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			txn := domain.Transaction{
				Amount: money.New(-2000, "EUR"),
				Splits: []domain.CategorySplit{{
					CategoryID: "food",
					Amount:     money.New(tc.split, "EUR"),
				}},
			}
			f := render.New(t)
			f.Render(ui.CreateElement(SplitEditor, splitEditorProps{Txn: txn}))

			remainder := byTestID(f, "span", "split-remainder")
			if remainder == nil {
				t.Fatal("split editor did not render its remainder status")
			}
			want := fmtMoney(money.New(tc.wantMinor, "EUR"))
			if !strings.Contains(remainder.Text(), want) {
				t.Errorf("split remainder = %q, want it to include %q", remainder.Text(), want)
			}
		})
	}
}

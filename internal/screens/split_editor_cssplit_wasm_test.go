// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"reflect"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/splitdraft"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestCSSplitModeToggleComponent(t *testing.T) {
	for _, percent := range []bool{false, true} {
		t.Run(map[bool]string{false: "amounts", true: "percents"}[percent], func(t *testing.T) {
			var actions []splitdraft.ActionKind
			f := render.New(t)
			f.Render(ui.CreateElement(splitModeToggle, splitModeToggleProps{Percent: percent,
				Change: func(a splitdraft.Action) { actions = append(actions, a.Kind) },
			}))
			amount := byTestID(f, "button", "split-mode-amount")
			pct := byTestID(f, "button", "split-mode-percent")
			if amount.Attr("aria-pressed") != ariaBool(!percent) || pct.Attr("aria-pressed") != ariaBool(percent) {
				t.Fatal("mode buttons lost their accessible pressed state")
			}
			if amount.Attr("class") != splitdraft.ModeClass(!percent) || pct.Attr("class") != splitdraft.ModeClass(percent) {
				t.Fatal("mode button classes changed")
			}
			amount.Click()
			pct.Click()
			if !reflect.DeepEqual(actions, []splitdraft.ActionKind{splitdraft.ToAmounts, splitdraft.ToPercents}) {
				t.Fatalf("mode actions = %v", actions)
			}
		})
	}
}

func TestCSSplitDraftStatusComponent(t *testing.T) {
	for _, tc := range []struct{ tone, key, value, want string }{
		{"pos", "splitEditor.balanced", "", "Balanced"},
		{"split-remainder-neutral", "splitEditor.left", "$12.34", "Left to assign: $12.34"},
		{"neg", "splitEditor.pctOver", "12.34", "12.34% over 100%"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			added := false
			f := render.New(t)
			f.Render(ui.CreateElement(splitDraftStatus, splitDraftStatusProps{
				View:   splitdraft.View{Tone: tc.tone, Key: tc.key, Value: tc.value},
				Change: func(a splitdraft.Action) { added = a.Kind == splitdraft.Add },
			}))
			status := byTestID(f, "span", "split-remainder")
			if status.Text() != tc.want || status.Attr("role") != "status" || status.Attr("class") != "hero-stat-sub "+tc.tone {
				t.Fatalf("status = %q, class %q", status.Text(), status.Attr("class"))
			}
			byTestID(f, "button", "split-add").Click()
			if !added {
				t.Fatal("Add split did not dispatch its plain callback")
			}
		})
	}
}

func TestCSSplitContentComponent(t *testing.T) {
	previous := appstate.Default
	appstate.Default = nil
	t.Cleanup(func() { appstate.Default = previous })
	tx := domain.Transaction{ID: "content", Amount: money.New(-100, "USD")}
	draft := splitdraft.New(tx, 0)
	draft.ErrorKey = "splitEditor.needTwo"
	var actions []splitdraft.Action
	var saved, cleared, canceled, receipt bool
	for _, modal := range []bool{false, true} {
		t.Run(map[bool]string{false: "inline", true: "modal"}[modal], func(t *testing.T) {
			p := splitEditorProps{Txn: tx, Members: []domain.Member{{ID: "m", Name: "Cam"}},
				OnReceiptSplit: func(id string) { receipt = id == tx.ID },
			}
			if modal {
				p.FooterFormID = SplitModalFormID
			}
			f := render.New(t)
			f.Render(ui.CreateElement(splitEditorContent, splitEditorContentProps{Editor: p, Draft: draft,
				Change: func(a splitdraft.Action) { actions = append(actions, a) },
				Save:   func() { saved = true }, Clear: func() { cleared = true }, Cancel: func() { canceled = true },
			}))
			if !strings.Contains(f.Text(), "Add at least two categories") || byTestID(f, "p", "split-label-who") == nil {
				t.Fatal("content lost error/owner rendering")
			}
			byTestID(f, "select", "split-cat-0").Change("food")
			byTestID(f, "input", "split-amt-0").Input("0.50")
			byTestID(f, "select", "split-owner-0").Change("m")
			byTestID(f, "button", "split-remove-1").Click()
			byTestID(f, "button", "txn-receipt-split-open").Click()
			if modal {
				f.ByID(SplitModalFormID).Dispatch("onsubmit", render.Event{})
				byTestID(f, "button", "split-cancel").Click()
			} else {
				byTestID(f, "button", "split-save").Click()
			}
			want := []splitdraft.Action{
				{Kind: splitdraft.SetCategory, Index: 0, Value: "food"},
				{Kind: splitdraft.SetAmount, Index: 0, Value: "0.50"},
				{Kind: splitdraft.SetOwner, Index: 0, Value: "m"},
				{Kind: splitdraft.Remove, Index: 1},
			}
			if !reflect.DeepEqual(actions, want) || !saved || !receipt || cleared || (modal && !canceled) {
				t.Fatalf("callbacks = %v; saved %v receipt %v canceled %v cleared %v", actions, saved, receipt, canceled, cleared)
			}
			actions = nil
			saved, receipt, canceled = false, false, false
		})
	}
}

func TestCSSplitEditorModeRoundTripAndRowLifecycle(t *testing.T) {
	tx := domain.Transaction{ID: "tiny", Amount: money.New(-3, "USD"), Splits: []domain.CategorySplit{
		{CategoryID: "a", Amount: money.New(-1, "USD")},
		{CategoryID: "b", Amount: money.New(-1, "USD")},
		{CategoryID: "c", Amount: money.New(-1, "USD")},
	}}
	var saved domain.Transaction
	f := render.New(t)
	f.Render(ui.CreateElement(SplitEditor, splitEditorProps{Txn: tx, OnSave: func(tx domain.Transaction) { saved = tx }}))
	byTestID(f, "button", "split-mode-percent").Click()
	f.Stabilize()
	if byTestID(f, "span", "split-remainder").Text() != "Balanced" {
		t.Fatal("balanced draft lost money converting to percentages")
	}
	byTestID(f, "button", "split-mode-amount").Click()
	f.Stabilize()
	if byTestID(f, "span", "split-remainder").Text() != "Balanced" {
		t.Fatal("balanced draft lost money converting back to amounts")
	}
	byTestID(f, "button", "split-add").Click()
	f.Stabilize()
	if byTestID(f, "input", "split-amt-3") == nil {
		t.Fatal("adding a row did not mount its independent controls")
	}
	byTestID(f, "button", "split-remove-3").Click()
	f.Stabilize()
	byTestID(f, "button", "split-save").Click()
	if !reflect.DeepEqual(saved.Splits, tx.Splits) {
		t.Fatalf("save after mode/row transitions = %+v", saved.Splits)
	}
	byTestID(f, "button", "split-clear").Click()
	if saved.Splits != nil {
		t.Fatal("clear did not propose removal of the saved breakdown")
	}
}

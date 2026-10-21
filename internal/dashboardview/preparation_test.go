// SPDX-License-Identifier: MIT

package dashboardview

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/CashFlux/internal/widgetengine"
	"github.com/monstercameron/CashFlux/internal/widgetregistry"
	"github.com/monstercameron/CashFlux/internal/widgetvis"
)

func frameFixture() widgetengine.DataCtx {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "income", AccountID: "a", Desc: "Salary", Date: start.AddDate(0, 0, 1), Amount: money.New(50000, "USD")},
		{ID: "expense", AccountID: "a", Desc: "Groceries", CategoryID: "food", Date: start.AddDate(0, 0, 2), Amount: money.New(-10000, "USD")},
		{ID: "refund", AccountID: "a", Desc: "Refund", CategoryID: "food", Date: start.AddDate(0, 0, 3), Amount: money.New(3000, "USD")},
		{ID: "foreign", AccountID: "b", Desc: "Foreign groceries", CategoryID: "food", Date: start.AddDate(0, 0, 4), Amount: money.New(-1000, "EUR")},
	}
	return widgetengine.DataCtx{Base: "USD", Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}},
		Accounts:     []domain.Account{{ID: "a", Name: "Checking", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeChecking}, {ID: "b", Name: "Euro", Currency: "EUR", Class: domain.ClassAsset, Type: domain.TypeChecking}},
		Transactions: txns, Ledger: budgeting.NewNettedLedger(txns), Categories: []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}},
		Budgets: []domain.Budget{{ID: "food-budget", Name: "Food", CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(20000, "USD")}},
		Start:   start, End: start.AddDate(0, 1, 0), Now: start.AddDate(0, 0, 10), ChartAnchor: start.AddDate(0, 0, 10),
		MonthVars: func(time.Time, time.Time) map[string]float64 { return map[string]float64{"income": 1234} }}
}

func TestPreparedFramesMatchHydratorAndKeepResultsImmutable(t *testing.T) {
	context := frameFixture()
	contexts := NewFrameContexts(2)
	prepared := contexts.Prepare(FrameIdentity{}, func() widgetengine.DataCtx { return context })
	var pipes []domain.Pipeline
	for _, name := range []string{"accounts", "transactions", "transactions-full", "budgets", "bills", "spending-breakdown"} {
		pipes = append(pipes, domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: name}})
	}
	for _, metric := range []string{"networth", "cashflow", "formula", "flow"} {
		pipes = append(pipes, domain.Pipeline{Source: domain.Source{Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: metric, Months: 3, Expr: "income", Filter: "cat:food"}}})
	}
	pipes = append(pipes, domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "transactions"}, Transform: []domain.Transform{{Kind: domain.TransformSort, Arg: "-date"}, {Kind: domain.TransformLimit, N: 1}}})
	pipes = append(pipes, domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "unknown"}})
	for i, pipe := range pipes {
		want, wantErr := widgetengine.HydrateFrame(&pipe, context)
		for repeat := 0; repeat < 2; repeat++ {
			got, err := prepared.Hydrate(&pipe)
			if fmt.Sprint(err) != fmt.Sprint(wantErr) || !reflect.DeepEqual(got, want) {
				t.Fatalf("request %d repeat %d got %+v/%v want %+v/%v", i, repeat, got, err, want, wantErr)
			}
			if len(got.Fields) > 0 {
				got.Fields[0].Name = "changed"
				if len(got.Fields[0].Values) > 0 {
					got.Fields[0].Values[0] = "changed"
				}
			}
		}
	}
	if prepared.Hydrations() != uint64(len(pipes)) {
		t.Fatalf("hydrations=%d want %d", prepared.Hydrations(), len(pipes))
	}
}

func TestFrameContextsInvalidateCompleteIdentityAndBoundRetention(t *testing.T) {
	cache := NewFrameContexts(2)
	builds := 0
	get := func(key FrameIdentity) *PreparedFrames {
		return cache.Prepare(key, func() widgetengine.DataCtx { builds++; return frameFixture() })
	}
	key := FrameIdentity{Instance: 1, Revision: 1, Preferences: 1, Scope: "[A B]", Base: "USD", Rates: "EUR:2", Start: time.Unix(1, 0), End: time.Unix(2, 0), ClockMinute: 1, VariablesAt: time.Unix(3, 0)}
	original := get(key)
	if get(key) != original || builds != 1 {
		t.Fatal("unchanged context rebuilt")
	}
	mutations := []func(*FrameIdentity){func(k *FrameIdentity) { k.Instance++ }, func(k *FrameIdentity) { k.Revision++ }, func(k *FrameIdentity) { k.Preferences++ }, func(k *FrameIdentity) { k.Scope = "[A] [B]" }, func(k *FrameIdentity) { k.Base = "EUR" }, func(k *FrameIdentity) { k.Rates = "EUR:3" }, func(k *FrameIdentity) { k.Start = k.Start.Add(time.Second) }, func(k *FrameIdentity) { k.End = k.End.Add(time.Second) }, func(k *FrameIdentity) { k.ClockMinute++ }, func(k *FrameIdentity) { k.VariablesAt = k.VariablesAt.Add(time.Second) }}
	for i, mutate := range mutations {
		changed := key
		mutate(&changed)
		if get(changed) == original {
			t.Fatalf("identity field %d failed to invalidate", i)
		}
	}
	if cache.Len() != 2 {
		t.Fatalf("retained %d contexts", cache.Len())
	}
	if get(key) == original {
		t.Fatal("evicted financial snapshot was retained")
	}
}

func TestPreparedFramePipelineConfigurationInvalidatesAndBoundsRows(t *testing.T) {
	context := frameFixture()
	prepared := NewFrameContexts(1).Prepare(FrameIdentity{}, func() widgetengine.DataCtx { return context })
	pipe := domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "transactions"}, Transform: []domain.Transform{{Kind: domain.TransformLimit, N: 1}}}
	one, err := prepared.Hydrate(&pipe)
	if err != nil || one.Rows != 1 {
		t.Fatalf("first limit: %+v/%v", one, err)
	}
	pipe.Transform[0].N = 2
	two, err := prepared.Hydrate(&pipe)
	if err != nil || two.Rows != 2 {
		t.Fatalf("changed limit: %+v/%v", two, err)
	}
	for i := 3; i < 45; i++ {
		pipe.Transform[0].N = i
		_, _ = prepared.Hydrate(&pipe)
	}
	if prepared.Len() != 32 {
		t.Fatalf("unbounded pipeline cache: %d", prepared.Len())
	}
}

func layoutFixture() LayoutInput {
	defaults := map[string]domain.WidgetSpec{}
	for _, id := range []string{"hero", "recent", "accounts"} {
		defaults[id], _ = widgetregistry.DefaultSpec(id)
	}
	custom := domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "us:custom", Kind: domain.KindList, Title: "Custom", Pipeline: &domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "transactions"}}}
	return LayoutInput{Instance: 1, Ready: true, Items: []dashlayout.Item{{ID: "recent", ColSpan: 2, RowSpan: 2}, {ID: "wb:published", ColSpan: 1, RowSpan: 1}, {ID: "accounts", ColSpan: 2, RowSpan: 1}, {ID: "us:custom", ColSpan: 2, RowSpan: 1}},
		Defaults: defaults, Configs: map[string]widgetcfg.Config{"recent": {"count": "2"}}, Hidden: widgetvis.Set{"accounts": true},
		Stored: []domain.Placement{{ID: "recent", Spec: domain.WidgetSpec{ID: "recent", Title: "My recent", Kind: domain.KindNative, NativeID: "bogus", Style: domain.Style{Accent: "#123456"}}}, {ID: "us:custom", Spec: custom}}}
}

func TestPreparedLayoutPreservesCompositionAndReusesUnchangedInputs(t *testing.T) {
	var cache LayoutCache
	input := layoutFixture()
	got, hit := cache.Prepare(input)
	if hit || got.Err != nil || len(got.Entries) != 3 || len(got.Placements) != 4 || got.Hero == nil {
		t.Fatalf("prepared layout %+v hit=%t", got, hit)
	}
	if got.Entries[0].Placement.Spec.Kind != domain.KindList || got.Entries[0].Placement.Spec.Title != "My recent" || got.Entries[0].Placement.Spec.Settings["count"] != "2" || got.Entries[1].Published.ID != "wb:published" || got.Entries[2].Placement.ID != "us:custom" {
		t.Fatal("placement overlay/order changed")
	}
	if got.Placements[1].ID != "accounts" || !got.Placements[1].Hidden {
		t.Fatal("hidden placement dropped from persistence")
	}
	if again, hit := cache.Prepare(input); !hit || again != got {
		t.Fatal("unchanged layout rebuilt/serialized")
	}
	input.Ready = false
	first, _ := cache.Prepare(input)
	if len(first.Entries) != 0 || len(first.Placements) != 1 {
		t.Fatal("first paint built a partial ready grid")
	}
	input.Ready = true
	if again, hit := cache.Prepare(input); !hit || again != got {
		t.Fatal("ready transition lost warm layout")
	}
}

func TestLayoutCacheFreezesEveryNestedSpecReference(t *testing.T) {
	var cache LayoutCache
	input := layoutFixture()
	spec := input.Stored[1].Spec
	spec.Scalar = &domain.ScalarBind{Expr: "original"}
	spec.Settings = widgetcfg.Config{"sample": "original"}
	spec.Graph = []byte(`{"nodes":{"original":1}}`)
	spec.Content = domain.ContentLayout{Mode: domain.LayoutCustom, Blocks: []domain.Block{{Kind: domain.BlockText, Text: "original"}}}
	// A list cannot also bind a scalar/graph; the cache must still freeze invalid
	// inputs, since later edits can make them valid without changing their ID.
	input.Stored[1].Spec = spec
	first, _ := cache.Prepare(input)
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var frozen LayoutInput
	if err := json.Unmarshal(encoded, &frozen); err != nil {
		t.Fatal(err)
	}
	input.Stored[1].Spec.Scalar.Expr = "changed"
	input.Stored[1].Spec.Pipeline.Source.Collection = "accounts"
	input.Stored[1].Spec.Pipeline.Transform = append(input.Stored[1].Spec.Pipeline.Transform, domain.Transform{Kind: domain.TransformLimit, N: 1})
	input.Stored[1].Spec.Settings["sample"] = "changed"
	input.Stored[1].Spec.Graph[2] = 'X'
	input.Stored[1].Spec.Content.Blocks[0].Text = "changed"
	input.Configs["recent"]["count"] = "3"
	input.Items[0].ColSpan = 3
	input.Hidden["accounts"] = false
	if _, hit := cache.Prepare(input); hit {
		t.Fatal("mutated nested input reused frozen layout")
	}
	if old, hit := cache.Prepare(frozen); !hit || old != first {
		t.Fatal("caller mutations poisoned original cached input")
	}
	if first.Entries[0].Placement.Spec.Settings["count"] != "2" || first.Entries[0].Placement.Layout.ColSpan != 2 {
		t.Fatal("caller mutations changed prepared placements")
	}
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestKeyValueRendersADefinitionListOfFacts(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(KeyValue(KeyValueProps{
		Class: "acct-facts", TestID: "facts",
		Facts: []KeyValueFact{
			{Label: "Opened", Value: Span("Jan 2024")},
			{Label: "Balance", Value: Span("1,200.00"), Tone: "pos", TestID: "fact-balance"},
			{Label: "Owed", Value: Span("40.00"), Tone: "neg"},
		},
	}))
	dl := uiNodeByTestID(f, "facts")
	if dl == nil || dl.Tag() != "dl" {
		t.Fatal("KeyValue root is not a <dl>")
	}
	if !containsClass(dl.Attr("class"), "kv") || !containsClass(dl.Attr("class"), "acct-facts") {
		t.Fatalf("class = %q, want kv and acct-facts", dl.Attr("class"))
	}
	dts := f.AllByTag("dt")
	dds := f.AllByTag("dd")
	if len(dts) != 3 || len(dds) != 3 {
		t.Fatalf("dt/dd count = %d/%d, want 3/3", len(dts), len(dds))
	}
	if dts[0].Text() != "Opened" || !containsClass(dts[0].Attr("class"), "stat-label") {
		t.Fatalf("first label = %q class %q, want Opened sharing stat-label", dts[0].Text(), dts[0].Attr("class"))
	}
	bal := uiNodeByTestID(f, "fact-balance")
	if bal == nil || bal.Tag() != "dd" || bal.Text() != "1,200.00" {
		t.Fatal("fact value is not a <dd> carrying its test id and text")
	}
	if !containsClass(bal.Attr("class"), "kv-value") || !containsClass(bal.Attr("class"), "pos") {
		t.Fatalf("value class = %q, want kv-value and pos", bal.Attr("class"))
	}
	if !containsClass(dds[2].Attr("class"), "neg") || containsClass(dds[0].Attr("class"), "pos") {
		t.Fatal("tone classes leaked between facts")
	}
}

func TestKeyValueWithNoFactsRendersAnEmptyList(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(KeyValue(KeyValueProps{TestID: "empty-facts"}))
	dl := uiNodeByTestID(f, "empty-facts")
	if dl == nil || len(dl.Children()) != 0 {
		t.Fatal("an empty KeyValue must render a <dl> with no rows")
	}
}

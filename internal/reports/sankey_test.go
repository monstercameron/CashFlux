// SPDX-License-Identifier: MIT

package reports

import (
	"math"
	"testing"
)

func TestLayoutSankey_ColumnsAndOrder(t *testing.T) {
	flows := []Flow{
		{From: "Salary", To: "Income", Value: 6000},
		{From: "Side gig", To: "Income", Value: 2000},
		{From: "Income", To: "Rent", Value: 3000},
		{From: "Income", To: "Food", Value: 2000},
		{From: "Income", To: "Savings", Value: 3000},
	}
	l := LayoutSankey(flows, 1000, 400, 10, 8, 3)
	if len(l.Nodes) != 6 || len(l.Links) != 5 {
		t.Fatalf("nodes=%d links=%d, want 6/5", len(l.Nodes), len(l.Links))
	}
	byLabel := map[string]FlowNode{}
	for _, n := range l.Nodes {
		byLabel[n.Label] = n
	}
	for label, wantCol := range map[string]int{"Salary": 0, "Side gig": 0, "Income": 1, "Rent": 2, "Food": 2, "Savings": 2} {
		if byLabel[label].Col != wantCol {
			t.Errorf("%s col = %d, want %d", label, byLabel[label].Col, wantCol)
		}
	}
	// Column X positions: sources at 0, hub centered, sinks at right edge.
	if byLabel["Salary"].X != 0 || byLabel["Income"].X != 495 || byLabel["Rent"].X != 990 {
		t.Errorf("X positions = %v/%v/%v, want 0/495/990", byLabel["Salary"].X, byLabel["Income"].X, byLabel["Rent"].X)
	}
	// First-appearance stacking: Rent above Food above Savings.
	if !(byLabel["Rent"].Y < byLabel["Food"].Y && byLabel["Food"].Y < byLabel["Savings"].Y) {
		t.Errorf("sink stack out of order: %v / %v / %v", byLabel["Rent"].Y, byLabel["Food"].Y, byLabel["Savings"].Y)
	}
	// Value proportionality (above the min floor): Rent (3000) is taller than Food (2000).
	if byLabel["Rent"].H <= byLabel["Food"].H {
		t.Errorf("Rent.H=%v not > Food.H=%v", byLabel["Rent"].H, byLabel["Food"].H)
	}
	// The hub carries all 8000 and is the tallest node.
	if byLabel["Income"].Value != 8000 {
		t.Errorf("hub value = %d, want 8000", byLabel["Income"].Value)
	}
}

func TestLayoutSankey_RibbonOffsetsTileTheNode(t *testing.T) {
	flows := []Flow{
		{From: "A", To: "Hub", Value: 100},
		{From: "B", To: "Hub", Value: 300},
		{From: "Hub", To: "X", Value: 250},
		{From: "Hub", To: "Y", Value: 150},
	}
	l := LayoutSankey(flows, 900, 300, 12, 10, 2)
	nodes := map[string]int{}
	for i, n := range l.Nodes {
		nodes[n.Label] = i
	}
	hub := l.Nodes[nodes["Hub"]]
	// Incoming ribbons at the hub start at its top and tile downward without
	// overlap, ordered by the source's Y (A appears above B).
	var inLinks []FlowLink
	for _, lk := range l.Links {
		if lk.To == nodes["Hub"] {
			inLinks = append(inLinks, lk)
		}
	}
	if len(inLinks) != 2 {
		t.Fatalf("hub in-links = %d, want 2", len(inLinks))
	}
	firstTY := math.Min(inLinks[0].TY, inLinks[1].TY)
	if math.Abs(firstTY-hub.Y) > 1e-9 {
		t.Errorf("first in-ribbon TY=%v, want hub top %v", firstTY, hub.Y)
	}
	if got := inLinks[0].H + inLinks[1].H; got > hub.H+1e-9 {
		t.Errorf("in-ribbons total %v exceed hub height %v", got, hub.H)
	}
	// Outgoing ribbons at the hub likewise tile from the top.
	var outLinks []FlowLink
	for _, lk := range l.Links {
		if lk.From == nodes["Hub"] {
			outLinks = append(outLinks, lk)
		}
	}
	firstSY := math.Min(outLinks[0].SY, outLinks[1].SY)
	if math.Abs(firstSY-hub.Y) > 1e-9 {
		t.Errorf("first out-ribbon SY=%v, want hub top %v", firstSY, hub.Y)
	}
}

func TestLayoutSankey_MinHeightFloorAndDegenerate(t *testing.T) {
	flows := []Flow{
		{From: "Big", To: "Income", Value: 100000},
		{From: "Tiny", To: "Income", Value: 3},
		{From: "Income", To: "Out", Value: 100003},
	}
	l := LayoutSankey(flows, 1000, 400, 10, 8, 4)
	for _, n := range l.Nodes {
		if n.H < 4 {
			t.Errorf("%s height %v below the 4px floor", n.Label, n.H)
		}
	}
	if got := LayoutSankey(nil, 1000, 400, 10, 8, 4); len(got.Nodes) != 0 {
		t.Errorf("nil flows produced %d nodes", len(got.Nodes))
	}
	if got := LayoutSankey([]Flow{{From: "A", To: "B", Value: 0}}, 1000, 400, 10, 8, 4); len(got.Nodes) != 0 {
		t.Errorf("zero-value flow produced %d nodes", len(got.Nodes))
	}
	// Self-loops are dropped.
	if got := LayoutSankey([]Flow{{From: "A", To: "A", Value: 10}}, 1000, 400, 10, 8, 4); len(got.Nodes) != 0 {
		t.Errorf("self-loop produced %d nodes", len(got.Nodes))
	}
}

func TestLayoutSankey_TwoLevelGraph(t *testing.T) {
	// No hub at all (every node is source-only or sink-only) still lays out.
	flows := []Flow{
		{From: "A", To: "X", Value: 10},
		{From: "B", To: "Y", Value: 20},
	}
	l := LayoutSankey(flows, 600, 200, 8, 6, 2)
	if len(l.Nodes) != 4 {
		t.Fatalf("nodes = %d, want 4", len(l.Nodes))
	}
	for _, n := range l.Nodes {
		if n.Col == 1 {
			t.Errorf("%s landed in the hub column of a two-level graph", n.Label)
		}
	}
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

// uiNodeByTestID returns the first element carrying data-testid id, or nil.
func uiNodeByTestID(f *render.Fixture, id string) *render.QueryNode {
	for _, tag := range []string{"div", "span", "button", "dl", "dd", "dt", "section", "strong"} {
		for _, node := range f.AllByTag(tag) {
			if node.Attr("data-testid") == id {
				return node
			}
		}
	}
	return nil
}

// uiParentOf returns the element whose children include node, or nil.
func uiParentOf(f *render.Fixture, node *render.QueryNode) *render.QueryNode {
	for _, tag := range []string{"div", "span", "button", "dl", "section"} {
		for _, candidate := range f.AllByTag(tag) {
			for _, child := range candidate.Children() {
				if child.NodeID() == node.NodeID() {
					return candidate
				}
			}
		}
	}
	return nil
}

func TestDisclosureIsAButtonThatControlsItsBody(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Disclosure(DisclosureProps{
		ID: "acct-details", Summary: Span("Details"), Body: P("Opening balance"), Open: true,
		TestID: "details", Class: "acct-disclosure",
	}))
	btn := buttonByTestID(f, "details")
	if btn == nil {
		t.Fatal("Disclosure did not render a <button> carrying its test id")
	}
	if btn.Attr("type") != "button" {
		t.Fatalf("type = %q, want button", btn.Attr("type"))
	}
	if btn.Attr("aria-controls") != "acct-details" {
		t.Fatalf("aria-controls = %q, want acct-details", btn.Attr("aria-controls"))
	}
	if btn.Attr("aria-expanded") != "true" {
		t.Fatalf("aria-expanded = %q, want true for an initially open Disclosure", btn.Attr("aria-expanded"))
	}
	body := uiNodeByTestID(f, "details-body")
	if body == nil || body.Attr("id") != "acct-details" {
		t.Fatal("open Disclosure has no body region with the controlled id")
	}
	if !strings.Contains(body.Text(), "Opening balance") {
		t.Fatalf("body text = %q", body.Text())
	}
	if !containsClass(body.Attr("class"), "fade-panel") || containsClass(body.Attr("class"), "is-leaving") {
		t.Fatalf("open body class = %q, want fade-panel without is-leaving", body.Attr("class"))
	}
	root := uiParentOf(f, btn)
	if root == nil || !containsClass(root.Attr("class"), "disclosure") || !containsClass(root.Attr("class"), "acct-disclosure") {
		t.Fatal("Disclosure root is missing the base and caller classes")
	}
}

func TestDisclosureTogglesOnClick(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Disclosure(DisclosureProps{ID: "more", Summary: Span("More"), Body: P("Extra facts"), TestID: "more-toggle"}))
	if uiNodeByTestID(f, "more-toggle-body") != nil {
		t.Fatal("a Disclosure that starts closed must not mount its body")
	}
	btn := buttonByTestID(f, "more-toggle")
	if btn.Attr("aria-expanded") != "false" {
		t.Fatalf("closed aria-expanded = %q, want false", btn.Attr("aria-expanded"))
	}
	btn.Click()
	f.Stabilize()
	btn = buttonByTestID(f, "more-toggle")
	if btn.Attr("aria-expanded") != "true" {
		t.Fatalf("aria-expanded after click = %q, want true", btn.Attr("aria-expanded"))
	}
	body := uiNodeByTestID(f, "more-toggle-body")
	if body == nil || !strings.Contains(body.Text(), "Extra facts") {
		t.Fatal("clicking the toggle did not show the body")
	}
	btn.Click()
	f.Stabilize()
	btn = buttonByTestID(f, "more-toggle")
	if btn.Attr("aria-expanded") != "false" {
		t.Fatalf("aria-expanded after second click = %q, want false", btn.Attr("aria-expanded"))
	}
	// The body stays mounted through the fade-out, marked as leaving.
	if body := uiNodeByTestID(f, "more-toggle-body"); body != nil && !containsClass(body.Attr("class"), "is-leaving") {
		t.Fatalf("closing body class = %q, want is-leaving while it fades out", body.Attr("class"))
	}
}

func TestDisclosureFallsBackToAGeneratedID(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Disclosure(DisclosureProps{Summary: Span("Auto"), Body: P("Body"), Open: true, TestID: "auto"}))
	btn := buttonByTestID(f, "auto")
	body := uiNodeByTestID(f, "auto-body")
	if btn == nil || body == nil || btn.Attr("aria-controls") == "" || btn.Attr("aria-controls") != body.Attr("id") {
		t.Fatal("a Disclosure without an ID must still link aria-controls to its body id")
	}
}

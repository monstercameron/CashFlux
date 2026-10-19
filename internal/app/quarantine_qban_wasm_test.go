// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestQBANBannerFlowsBeforeAppAndHideKeepsQuarantineCopy(t *testing.T) {
	global := js.Global()
	oldDocument := global.Get("document")
	global.Set("document", newQuarantineTestDocument())
	t.Cleanup(func() { global.Set("document", oldDocument) })

	oldHidden := quarantineBannerHidden
	quarantineBannerHidden = false
	t.Cleanup(func() { quarantineBannerHidden = oldHidden })

	const recoveryKey = quarantinePrefix + "1720000002"
	const recoveryCopy = `{"unreadable":true}`
	browserstore.Set(recoveryKey, recoveryCopy)
	t.Cleanup(func() { browserstore.Remove(recoveryKey) })

	doc := global.Get("document")
	body := doc.Get("body")
	appRoot := doc.Call("createElement", "main")
	appRoot.Set("id", "app")
	body.Call("appendChild", appRoot)
	showQuarantineBanner()

	banner := doc.Call("getElementById", "cashflux-quarantine-banner")
	if !banner.Truthy() {
		t.Fatal("showQuarantineBanner did not mount the banner")
	}
	if body.Get("firstChild").Equal(appRoot) || !body.Get("firstChild").Equal(banner) {
		t.Fatal("banner is not the first in-flow body child ahead of the app root")
	}
	style := banner.Get("style").Get("cssText").String()
	if strings.Contains(style, "fixed") || strings.Contains(style, "position:") {
		t.Fatalf("banner can cover the app instead of taking page flow: style %q", style)
	}
	if banner.Get("children").Length() != 2 {
		t.Fatalf("banner children = %d, want message and actions", banner.Get("children").Length())
	}
	buttons := banner.Get("children").Index(1).Get("children")
	if buttons.Length() != 3 {
		t.Fatalf("banner actions = %d, want Download, Start fresh and Hide for now", buttons.Length())
	}
	if got := buttons.Index(2).Get("textContent").String(); got != uistate.T("datasetQuarantine.hideForNow") || got != "Hide for now" {
		t.Fatalf("Hide label = %q, want localized %q", got, uistate.T("datasetQuarantine.hideForNow"))
	}
	buttons.Index(2).Get("onfocus").Invoke()
	if got := buttons.Index(2).Get("style").Get("outline").String(); got == "" {
		t.Fatal("keyboard focus does not have a visible outline")
	}
	buttons.Index(2).Get("onclick").Invoke()
	if doc.Call("getElementById", "cashflux-quarantine-banner").Truthy() {
		t.Fatal("Hide for now left the banner mounted")
	}
	if got, ok := browserstore.Get(recoveryKey); !ok || got != recoveryCopy {
		t.Fatalf("Hide for now changed the saved recovery copy: value=%q present=%v", got, ok)
	}
	if !quarantineBannerHidden {
		t.Fatal("Hide for now did not suppress another banner mount for this page load")
	}
	showQuarantineBanner()
	if doc.Call("getElementById", "cashflux-quarantine-banner").Truthy() {
		t.Fatal("banner reappeared during the same page load after being hidden")
	}
}

func newQuarantineTestDocument() js.Value {
	return js.Global().Get("Function").New(`
		function element(tag) {
		var node = {
				tagName: tag.toUpperCase(), id: "", textContent: "", style: { cssText: "" },
				children: [], attributes: {}, parentNode: null,
				setAttribute: function(name, value) { this.attributes[name] = value; },
				appendChild: function(child) {
					child.parentNode = this;
					this.children.push(child);
					return child;
				},
				insertBefore: function(child, before) {
					child.parentNode = this;
					var index = this.children.indexOf(before);
					this.children.splice(index < 0 ? this.children.length : index, 0, child);
					return child;
				},
				remove: function() {
					if (!this.parentNode) return;
					var siblings = this.parentNode.children;
					var index = siblings.indexOf(this);
					if (index >= 0) siblings.splice(index, 1);
					this.parentNode = null;
				}
			};
		var style = { cssText: "", outline: "", outlineOffset: "" };
		Object.defineProperty(node, "style", {
			get: function() { return style; },
			set: function(value) { style.cssText = String(value); }
		});
		return node;
		}
		var body = element("body");
		Object.defineProperty(body, "firstChild", { get: function() { return this.children[0] || null; } });
		return {
			body: body,
			createElement: element,
			getElementById: function(id) {
				function find(node) {
					if (node.id === id) return node;
					for (var i = 0; i < node.children.length; i++) {
						var found = find(node.children[i]);
						if (found) return found;
					}
					return null;
				}
				return find(body);
			}
		};
	`).Invoke()
}

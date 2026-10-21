//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestCustomFieldMalformedBoolDoesNotWriteAndValidFalseDoes(t *testing.T) {
	app := guardTestApp(t)
	if err := app.PutCustomFieldDef(customfields.Def{
		ID: "member-enabled", EntityType: "member", Key: "enabled", Label: "Enabled", Type: customfields.TypeBool,
	}); err != nil {
		t.Fatal(err)
	}
	initialMembers := len(app.Members())
	f := render.New(t)
	f.Render(ui.CreateElement(MemberAddForm, MemberAddFormProps{}))
	f.InputByID("member-add", "Jordan")
	if got := f.ByID("member-add").Attr("data-cf-value"); got != "Jordan" {
		t.Fatalf("member name input value = %q, want Jordan", got)
	}
	var enabled *render.QueryNode
	for _, node := range f.AllByTag("select") {
		if node.Attr("title") == "Enabled" {
			enabled = node
			break
		}
	}
	if enabled == nil {
		t.Fatal("custom boolean field is missing")
	}
	enabled.Dispatch("onchange", render.Event{Value: "yes"})
	byTestID(f, "form", "member-add-form").Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	if len(app.Members()) != initialMembers || !strings.Contains(f.Text(), "Enabled must be true or false.") {
		t.Fatalf("malformed boolean was not rejected: members=%d text=%s", len(app.Members()), f.Text())
	}

	for _, node := range f.AllByTag("select") {
		if node.Attr("title") == "Enabled" {
			enabled = node
			break
		}
	}
	enabled.Dispatch("onchange", render.Event{Value: "false"})
	byTestID(f, "form", "member-add-form").Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	members := app.Members()
	if len(members) != initialMembers+1 || members[len(members)-1].Custom["enabled"] != false {
		t.Fatalf("valid false value was not preserved: %+v", members)
	}
}

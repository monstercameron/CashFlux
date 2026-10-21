// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func notifyInitialTestApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	previous, previousAtom, previousCaptured := appstate.Default, capturedNotifyFeed, notifyFeedCaptured
	appstate.Default, notifyFeedCaptured = app, false
	t.Cleanup(func() {
		appstate.Default, capturedNotifyFeed, notifyFeedCaptured = previous, previousAtom, previousCaptured
		_ = app.Store().Close()
	})
	return app
}

type notifyInitialReaderProps struct {
	Name string
	Tick int
}

func TestNotifyFeedInitialLoadRunsOncePerMountedHook(t *testing.T) {
	notifyInitialTestApp(t)
	PersistNotifyFeed([]FeedItem{{ID: "stored", Title: "Stored", Severity: "warning"}})
	loads := map[string]int{}
	atoms := map[string]state.Atom[[]FeedItem]{}
	reader := func(p notifyInitialReaderProps) ui.Node {
		atom := useNotifyFeedWithLoader(func() []FeedItem { loads[p.Name]++; return loadNotifyFeed() })
		atoms[p.Name] = atom
		items := atom.Get()
		title := "empty"
		read := false
		if len(items) > 0 {
			title, read = items[0].Title, items[0].Read
		}
		return P(Attr("id", p.Name), fmt.Sprintf("%s:%t", title, read))
	}
	f := render.New(t)
	defer f.Cleanup()
	var setTick func(int)
	var setSecond, setLate func(bool)
	f.Render(ui.CreateElement(func() ui.Node {
		tick := ui.UseState(0)
		second := ui.UseState(true)
		late := ui.UseState(false)
		setTick, setSecond, setLate = tick.Set, second.Set, late.Set
		var secondNode, lateNode ui.Node
		if second.Get() {
			secondNode = ui.CreateElement(reader, notifyInitialReaderProps{Name: "second", Tick: tick.Get()})
		}
		if late.Get() {
			lateNode = ui.CreateElement(reader, notifyInitialReaderProps{Name: "late", Tick: tick.Get()})
		}
		return Div(ui.CreateElement(reader, notifyInitialReaderProps{Name: "first", Tick: tick.Get()}), Div(secondNode), Div(lateNode))
	}))
	if loads["first"] != 1 || loads["second"] != 1 || f.ByID("first").Text() != "Stored:false" {
		t.Fatalf("initial loads=%v text=%s", loads, f.Text())
	}
	for tick := 1; tick <= 3; tick++ {
		setTick(tick)
		f.Stabilize()
	}
	if loads["first"] != 1 || loads["second"] != 1 {
		t.Fatalf("warm renders reread persisted JSON: %v", loads)
	}
	MarkFeedItemRead("stored", true)
	f.Stabilize()
	if f.ByID("first").Text() != "Stored:true" || f.ByID("second").Text() != "Stored:true" || loads["first"] != 1 || loads["second"] != 1 {
		t.Fatalf("read update reran defaults or was lost: %v %s", loads, f.Text())
	}
	// A newly mounted subscriber must preserve the atom, even if its persisted
	// default differs from a live, intentionally unpersisted write.
	atoms["first"].Set([]FeedItem{{ID: "live", Title: "Live only", Read: true}})
	f.Stabilize()
	setLate(true)
	f.Stabilize()
	if loads["late"] != 1 || f.ByID("late").Text() != "Live only:true" {
		t.Fatalf("late default overwrote the live feed: %v %s", loads, f.Text())
	}
	setSecond(false)
	f.Stabilize()
	setSecond(true)
	f.Stabilize()
	if loads["second"] != 2 || f.ByID("second").Text() != "Live only:true" || loads["first"] != 1 {
		t.Fatalf("remount reused an obsolete seed: %v %s", loads, f.Text())
	}
}

func TestNotifyFeedInitialLoadUsesFreshRuntimeAndAppWorkspace(t *testing.T) {
	first := notifyInitialTestApp(t)
	PersistNotifyFeed([]FeedItem{{ID: "first", Title: "First household", Severity: "warning"}})
	second, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Store().Close()
	appstate.Default = second
	PersistNotifyFeed([]FeedItem{{ID: "second", Title: "Second household", Severity: "critical"}})
	// The supported workspace switch reloads Go and its atom registry. Create
	// separate real host runtimes to model that boundary, including returning.
	for _, tc := range []struct {
		app   *appstate.App
		title string
	}{{first, "First household"}, {second, "Second household"}, {first, "First household"}} {
		appstate.Default = tc.app
		f := render.New(t)
		loads := 0
		f.Render(ui.CreateElement(func() ui.Node {
			atom := useNotifyFeedWithLoader(func() []FeedItem { loads++; return loadNotifyFeed() })
			items := atom.Get()
			if len(items) == 0 {
				return P("empty")
			}
			return P(items[0].Title)
		}))
		f.Stabilize()
		if loads != 1 || f.Text() != tc.title {
			f.Cleanup()
			t.Fatalf("workspace default leaked: loads=%d text=%q want=%q", loads, f.Text(), tc.title)
		}
		f.Cleanup()
	}
}

func TestNotifyFeedInitialLoadKeepsPreMountCatchUp(t *testing.T) {
	notifyInitialTestApp(t)
	PersistNotifyFeed([]FeedItem{{ID: "older", Title: "Older", Severity: "warning"}})
	// Boot may prepend before any subscriber captures the feed atom.
	PrependNotifyFeed([]FeedItem{{ID: "new", Title: "New catch-up", Severity: "critical"}})
	f := render.New(t)
	defer f.Cleanup()
	loads := 0
	f.Render(ui.CreateElement(func() ui.Node {
		atom := useNotifyFeedWithLoader(func() []FeedItem { loads++; return loadNotifyFeed() })
		var titles []string
		for _, item := range atom.Get() {
			titles = append(titles, item.Title)
		}
		return P(strings.Join(titles, "|"))
	}))
	if loads != 1 || f.Text() != "New catch-up|Older" {
		t.Fatalf("pre-mount catch-up lost: loads=%d text=%q", loads, f.Text())
	}
	PrependNotifyFeed([]FeedItem{{ID: "latest", Title: "Latest", Severity: "warning"}})
	f.Stabilize()
	if loads != 1 || f.Text() != "Latest|New catch-up|Older" {
		t.Fatalf("post-mount catch-up reran defaults: loads=%d text=%q", loads, f.Text())
	}
}

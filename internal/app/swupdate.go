// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"log/slog"
	"sync"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/buildinfo"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// Service-worker update flow (SV7a). index.html registers sw.js per build
// and reports a waiting worker; this file turns that into the user-visible
// "An update is ready — Reload" banner, keeps every open tab informed
// through BroadcastChannel("cashflux-build"), and tracks the server's
// active build in internal/buildinfo so buildinfo.IsStale() can tell the
// rest of the app when this tab has been replaced. Decisions live in the
// pure buildinfo package; everything here is DOM/JS glue.

const (
	swUpdateBannerID   = "cashflux-swupdate-banner"
	swBuildChannelName = "cashflux-build"
	// swBuildPollInterval bounds how long a tab can run a replaced build
	// without noticing, even if no other tab and no service-worker event
	// ever tells it. build.json is a few dozen bytes and never cached.
	swBuildPollInterval = 5 * time.Minute
)

// swUpdateState is the mutable glue state behind the banner. All fields
// are guarded by mu: JS callbacks and the poll goroutine both touch it.
var swUpdateState struct {
	mu            sync.Mutex
	running       string // this tab's build ID (buildinfo.Running at wire time)
	updateReady   bool   // a newer worker is waiting, here or in another tab
	dismissed     string // buildinfo.DismissalKey the user chose "Later" for
	shown         buildinfo.Banner
	reloadWanted  bool   // the user clicked Reload; controllerchange finishes it
	reloaded      bool   // reload already issued (never twice)
	fetching      bool   // a build.json fetch is in flight
	registeredFor string // last build handed to __cashfluxRegisterBuild
	channel       js.Value
	listeners     []js.Func // lifetime listeners, never released
}

func init() { wireSWUpdate() }

// wireSWUpdate attaches every listener the update flow needs. It runs from
// init so no call site has to be threaded through app boot, and it is a
// complete no-op for development builds: `gwc dev` live reload must never
// be interrupted by an update banner.
func wireSWUpdate() {
	if buildinfo.IsDev() {
		return
	}
	win := js.Global().Get("window")
	if !win.Truthy() {
		return
	}
	swUpdateState.mu.Lock()
	swUpdateState.running = buildinfo.Running()
	swUpdateState.mu.Unlock()

	// index.html may already know the active build / a waiting worker by
	// the time the wasm boots (registration ran on window load).
	if known := js.Global().Get("__cashfluxBuild"); known.Truthy() {
		adoptActiveBuild(known)
	}
	if js.Global().Get("__cashfluxUpdateReady").Truthy() {
		setSWUpdateReady()
	}

	listen := func(target js.Value, event string, fn func(js.Value)) {
		if !target.Truthy() {
			return
		}
		cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) > 0 {
				fn(args[0])
			}
			return nil
		})
		swUpdateState.listeners = append(swUpdateState.listeners, cb)
		target.Call("addEventListener", event, cb)
	}

	listen(win, "cashflux:build-known", func(e js.Value) { adoptActiveBuild(e.Get("detail")) })
	listen(win, "cashflux:update-ready", func(js.Value) { setSWUpdateReady() })

	if nav := js.Global().Get("navigator"); nav.Truthy() {
		if sw := nav.Get("serviceWorker"); sw.Truthy() {
			// The update applied: a new worker controls this page. Only
			// reload when the user asked for it — a first install claiming
			// the page must not refresh a tab out from under its user.
			listen(sw, "controllerchange", func(js.Value) {
				swUpdateState.mu.Lock()
				wanted := swUpdateState.reloadWanted
				swUpdateState.mu.Unlock()
				if wanted {
					reloadPageForUpdate()
				}
			})
			listen(sw, "message", func(e js.Value) {
				msgType, buildID := parseBuildMessage(e.Get("data"))
				if msgType == "SW_ACTIVATED" && buildID != "" && buildID != runningBuild() {
					// Another tab applied the update. Learn the new active
					// build so the banner (and IsStale) catch up here too.
					broadcastBuildMessage("activated", buildID)
					fetchActiveBuild()
				}
			})
		}
	}

	if doc := js.Global().Get("document"); doc.Truthy() {
		listen(doc, "visibilitychange", func(js.Value) {
			if doc.Get("visibilityState").String() == "visible" {
				fetchActiveBuild()
			}
		})
	}

	wireBuildChannel()
	fetchActiveBuild()
	go pollActiveBuild()
	slog.Debug("service-worker update flow wired", "build", runningBuild())
}

func runningBuild() string {
	swUpdateState.mu.Lock()
	defer swUpdateState.mu.Unlock()
	if swUpdateState.running != "" {
		return swUpdateState.running
	}
	return buildinfo.Running()
}

func pollActiveBuild() {
	ticker := time.NewTicker(swBuildPollInterval)
	defer ticker.Stop()
	for range ticker.C {
		fetchActiveBuild()
	}
}

// adoptActiveBuild records a build info object handed over by index.html
// ({buildId, schema}) as the server's active build.
func adoptActiveBuild(v js.Value) {
	if !v.Truthy() {
		return
	}
	info := buildinfo.Info{BuildID: jsString(v.Get("buildId"))}
	if s := v.Get("schema"); s.Type() == js.TypeNumber {
		info.Schema = s.Int()
	}
	if info.BuildID == "" {
		return
	}
	buildinfo.SetActive(info)
	maybeRegisterActiveBuild(info.BuildID)
	refreshSWUpdateBanner()
}

func setSWUpdateReady() {
	swUpdateState.mu.Lock()
	swUpdateState.updateReady = true
	swUpdateState.mu.Unlock()
	// Name the waiting build so the gentle banner can be dismissed per
	// build and the stale check has fresh data.
	fetchActiveBuild()
	refreshSWUpdateBanner()
}

// maybeRegisterActiveBuild asks index.html to register the service worker
// for a build this tab is not running, so the newer worker installs (and
// waits) without needing a page reload first. Once per build ID.
func maybeRegisterActiveBuild(activeBuild string) {
	if activeBuild == "" || activeBuild == buildinfo.DevBuild || activeBuild == runningBuild() {
		return
	}
	swUpdateState.mu.Lock()
	if swUpdateState.registeredFor == activeBuild {
		swUpdateState.mu.Unlock()
		return
	}
	swUpdateState.registeredFor = activeBuild
	swUpdateState.mu.Unlock()
	if register := js.Global().Get("__cashfluxRegisterBuild"); register.Type() == js.TypeFunction {
		register.Invoke(activeBuild)
	}
}

// fetchActiveBuild re-reads build.json (never cached, never intercepted by
// the service worker) and records the server's active build. Failures —
// offline, dev serving, an auth gate's HTML — leave state untouched: no
// news is not staleness.
func fetchActiveBuild() {
	swUpdateState.mu.Lock()
	if swUpdateState.fetching {
		swUpdateState.mu.Unlock()
		return
	}
	swUpdateState.fetching = true
	swUpdateState.mu.Unlock()
	done := func() {
		swUpdateState.mu.Lock()
		swUpdateState.fetching = false
		swUpdateState.mu.Unlock()
	}

	fetch := js.Global().Get("fetch")
	if fetch.Type() != js.TypeFunction {
		done()
		return
	}
	var onText, onResponse, onError js.Func
	release := func() {
		onText.Release()
		onResponse.Release()
		onError.Release()
		done()
	}
	onError = js.FuncOf(func(js.Value, []js.Value) any { release(); return nil })
	onText = js.FuncOf(func(_ js.Value, args []js.Value) any {
		defer release()
		if len(args) == 0 {
			return nil
		}
		info, err := buildinfo.ParseBuildJSON([]byte(args[0].String()))
		if err != nil {
			slog.Debug("build.json unreadable; keeping last known build", "error", err)
			return nil
		}
		buildinfo.SetActive(info)
		maybeRegisterActiveBuild(info.BuildID)
		refreshSWUpdateBanner()
		return nil
	})
	onResponse = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 || !args[0].Truthy() || !args[0].Get("ok").Truthy() {
			release()
			return nil
		}
		args[0].Call("text").Call("then", onText).Call("catch", onError)
		return nil
	})
	fetch.Invoke("./build.json", js.ValueOf(map[string]any{
		"cache":       "no-store",
		"credentials": "same-origin",
	})).Call("then", onResponse).Call("catch", onError)
}

// --- Cross-tab build channel -------------------------------------------------

// wireBuildChannel joins BroadcastChannel("cashflux-build"): every tab
// announces the build it runs, answers pings, and hears when another tab
// finds or applies an update. Tabs never decide staleness by comparing
// with each other — an announcement only triggers a fresh build.json
// read, and the server's active build decides (SV principle 6).
func wireBuildChannel() {
	ctor := js.Global().Get("BroadcastChannel")
	if ctor.Type() != js.TypeFunction {
		return
	}
	channel := ctor.New(swBuildChannelName)
	swUpdateState.mu.Lock()
	swUpdateState.channel = channel
	swUpdateState.mu.Unlock()
	onMessage := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 {
			handleBuildMessage(args[0].Get("data"))
		}
		return nil
	})
	swUpdateState.listeners = append(swUpdateState.listeners, onMessage)
	channel.Set("onmessage", onMessage)
	broadcastBuildMessage("announce", runningBuild())
}

func broadcastBuildMessage(msgType, buildID string) {
	swUpdateState.mu.Lock()
	channel := swUpdateState.channel
	swUpdateState.mu.Unlock()
	if !channel.Truthy() {
		return
	}
	channel.Call("postMessage", js.ValueOf(map[string]any{
		"type":    msgType,
		"buildId": buildID,
	}))
}

// handleBuildMessage reacts to one channel message from another tab.
func handleBuildMessage(data js.Value) {
	msgType, buildID := parseBuildMessage(data)
	switch msgType {
	case "ping":
		broadcastBuildMessage("announce", runningBuild())
	case "announce":
		if buildID != "" && buildID != runningBuild() {
			// Two builds are open at once. Whichever differs from the
			// server's active build is the stale one — go and look.
			fetchActiveBuild()
		}
	case "update-ready":
		setSWUpdateReady()
	case "activated", "applying":
		// Another tab moved (or is moving) to the new build. This tab is
		// about to be the old one: surface the banner and refresh the
		// active build so buildinfo.IsStale() catches up.
		swUpdateState.mu.Lock()
		swUpdateState.updateReady = true
		swUpdateState.mu.Unlock()
		fetchActiveBuild()
		refreshSWUpdateBanner()
	}
}

// parseBuildMessage extracts {type, buildId} from a channel or
// service-worker message, tolerating missing and non-string fields.
func parseBuildMessage(data js.Value) (msgType, buildID string) {
	if !data.Truthy() {
		return "", ""
	}
	if data.Type() == js.TypeString {
		return data.String(), ""
	}
	return jsString(data.Get("type")), jsString(data.Get("buildId"))
}

// --- Banner ------------------------------------------------------------------

// swuObservations snapshots the pure inputs the banner decision needs.
func swuObservations() buildinfo.Observations {
	active, known := buildinfo.Active()
	swUpdateState.mu.Lock()
	defer swUpdateState.mu.Unlock()
	running := swUpdateState.running
	if running == "" {
		running = buildinfo.Running()
	}
	return buildinfo.Observations{
		Running:        running,
		RunningSchema:  store.SchemaVersion,
		Active:         active,
		ActiveKnown:    known,
		UpdateReady:    swUpdateState.updateReady,
		DismissedBuild: swUpdateState.dismissed,
	}
}

// refreshSWUpdateBanner shows, updates or hides the banner to match the
// current decision. Safe to call on every state change.
func refreshSWUpdateBanner() {
	decision := buildinfo.DecideBanner(swuObservations())
	swUpdateState.mu.Lock()
	unchanged := swUpdateState.shown == decision
	swUpdateState.shown = decision
	swUpdateState.mu.Unlock()
	if unchanged && decision != buildinfo.BannerNone && swBannerElement().Truthy() {
		return
	}
	switch decision {
	case buildinfo.BannerNone:
		if el := swBannerElement(); el.Truthy() {
			el.Call("remove")
		}
	default:
		showSWUpdateBanner(decision)
	}
}

func swBannerElement() js.Value {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return js.Null()
	}
	return doc.Call("getElementById", swUpdateBannerID)
}

// showSWUpdateBanner renders the update bar by direct DOM work, the same
// boot-level pattern as the quarantine banner: it must appear regardless
// of router/screen state, above everything, and survive re-renders.
func showSWUpdateBanner(kind buildinfo.Banner) {
	doc := js.Global().Get("document")
	if !doc.Truthy() || doc.Get("body").IsNull() {
		return
	}
	if el := swBannerElement(); el.Truthy() {
		el.Call("remove") // re-render: kind or language may have changed
	}
	stale := kind == buildinfo.BannerStale

	banner := doc.Call("createElement", "section")
	banner.Set("id", swUpdateBannerID)
	banner.Call("setAttribute", "role", "alert")
	if stale {
		banner.Call("setAttribute", "aria-live", "assertive")
		banner.Set("style", "position:fixed;z-index:100000;top:0;left:0;right:0;padding:12px 20px;background:#fff4d6;color:#302400;border-bottom:1px solid #d9a400;box-shadow:0 2px 12px #0002;font:500 15px/1.5 system-ui,sans-serif;display:flex;gap:14px;align-items:center;flex-wrap:wrap")
	} else {
		banner.Call("setAttribute", "aria-live", "polite")
		banner.Set("style", "position:fixed;z-index:100000;top:0;left:0;right:0;padding:12px 20px;background:#1f2c24;color:#dff2e4;border-bottom:1px solid #356b50;box-shadow:0 2px 12px #0006;font:500 15px/1.5 system-ui,sans-serif;display:flex;gap:14px;align-items:center;flex-wrap:wrap")
	}

	text := doc.Call("createElement", "p")
	text.Set("style", "margin:0;flex:1;min-width:220px")
	if stale {
		text.Set("textContent", uistate.T("swupdate.staleMessage"))
	} else {
		text.Set("textContent", uistate.T("swupdate.updateMessage"))
	}
	banner.Call("appendChild", text)

	reload := swBannerButton(doc, uistate.T("swupdate.reload"), uistate.T("swupdate.reloadAria"), stale, true, applySWUpdate)
	reload.Set("id", swUpdateBannerID+"-reload")
	banner.Call("appendChild", reload)
	if !stale {
		// A stale tab gets no "Later": the banner is the last line of
		// defence against quietly writing old-format data, so it stays
		// until the reload happens.
		later := swBannerButton(doc, uistate.T("swupdate.later"), uistate.T("swupdate.dismissAria"), stale, false, dismissSWUpdate)
		later.Set("id", swUpdateBannerID+"-later")
		banner.Call("appendChild", later)
	}
	doc.Get("body").Call("appendChild", banner)
}

func swBannerButton(doc js.Value, label, aria string, stale, primary bool, action func()) js.Value {
	button := doc.Call("createElement", "button")
	button.Set("type", "button")
	button.Set("textContent", label)
	button.Call("setAttribute", "aria-label", aria)
	switch {
	case primary && stale:
		button.Set("style", "padding:7px 14px;border:1px solid #725600;border-radius:6px;background:#302400;color:#fff4d6;font:inherit;font-weight:600;cursor:pointer")
	case primary:
		button.Set("style", "padding:7px 14px;border:1px solid #7fd0a3;border-radius:6px;background:#2e8b57;color:#fff;font:inherit;font-weight:600;cursor:pointer")
	default:
		button.Set("style", "padding:7px 14px;border:1px solid #356b50;border-radius:6px;background:transparent;color:#7fd0a3;font:inherit;cursor:pointer")
	}
	cb := js.FuncOf(func(js.Value, []js.Value) any { action(); return nil })
	swUpdateState.listeners = append(swUpdateState.listeners, cb)
	button.Set("onclick", cb)
	return button
}

// applySWUpdate is the Reload click: ask the waiting worker to activate,
// then reload once it takes control. With no waiting worker (the banner
// came from build.json alone) a plain reload already lands on the new
// build. A timeout covers a worker that never answers, so the click can
// never strand the user on the old build.
func applySWUpdate() {
	swUpdateState.mu.Lock()
	swUpdateState.reloadWanted = true
	swUpdateState.mu.Unlock()
	if active, known := buildinfo.Active(); known {
		broadcastBuildMessage("applying", active.BuildID)
	}
	if el := swBannerElement(); el.Truthy() {
		el.Set("textContent", uistate.T("swupdate.applyingUpdate"))
	}
	posted := false
	if apply := js.Global().Get("__cashfluxApplyUpdate"); apply.Type() == js.TypeFunction {
		posted = apply.Invoke().Truthy()
	}
	if !posted {
		reloadPageForUpdate()
		return
	}
	fallback := js.FuncOf(func(js.Value, []js.Value) any {
		reloadPageForUpdate()
		return nil
	})
	swUpdateState.listeners = append(swUpdateState.listeners, fallback)
	js.Global().Call("setTimeout", fallback, 4000)
}

// dismissSWUpdate is the Later click. It records the offered build so the
// gentle banner stays down for that build only — a still newer build
// re-offers, and a tab that turns stale is never silenced.
func dismissSWUpdate() {
	obs := swuObservations()
	if buildinfo.DecideBanner(obs) == buildinfo.BannerStale {
		return
	}
	swUpdateState.mu.Lock()
	swUpdateState.dismissed = buildinfo.DismissalKey(obs)
	swUpdateState.mu.Unlock()
	refreshSWUpdateBanner()
}

func reloadPageForUpdate() {
	swUpdateState.mu.Lock()
	if swUpdateState.reloaded {
		swUpdateState.mu.Unlock()
		return
	}
	swUpdateState.reloaded = true
	swUpdateState.mu.Unlock()
	if loc := js.Global().Get("location"); loc.Truthy() {
		loc.Call("reload")
	}
}

// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/browser"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/theme"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// ThemeEditor is the appearance theme editor component: pick a built-in preset,
// tweak every design token (surface/text/accent colors, corner radius, font-size
// scale, UI/display fonts, density), see live validation warnings, and reset to
// the default. Every change applies and persists immediately (live theming), so
// the surrounding app is itself the preview. Mount with uic.CreateElement(ThemeEditor).
// All copy flows through the shared i18n bundle under the themeEd.* namespace
// (2026-07-19 sweep — the earlier inline-English decoupling is retired). Preset
// names, font family names, and the stored "Custom" theme name are identifiers,
// not copy, and stay literal.
func ThemeEditor() uic.Node {
	cur := uic.UseState(uistate.LoadTheme())
	importMsg := uic.UseState("")
	fonts := uic.UseState(uistate.LoadFonts())
	fontMsg := uic.UseState("")
	prefsAtom := uistate.UsePrefs()
	t := cur.Get()

	apply := func(next theme.Theme) {
		next.Name = "Custom"
		uistate.ApplyTheme(next)
		uistate.PersistTheme(next)
		// The theme owns density + display scale now; mirror them back into the
		// legacy prefs so a fresh migration and any prefs reader stay consistent
		// (one appearance system, two stores kept in lockstep).
		p := prefsAtom.Get()
		p.Compact = next.Density == theme.Compact
		p.Scale = int(next.Scale*100 + 0.5)
		// The theme also owns the shell skin — ApplyTheme derives data-theme from
		// the theme's luminance — so mirror that into prefs.Theme too. Without
		// this, applying a dark preset (Midnight) while prefs said "light" flips
		// the whole app dark while the Appearance hero and Mode control keep
		// reading "Light".
		if next.IsLight() {
			p.Theme = prefs.ThemeLight
		} else {
			p.Theme = prefs.ThemeDark
		}
		prefsAtom.Set(p)
		uistate.PersistPrefs(p)
		cur.Set(next)
		// A preset can change tokens (accent, fonts) without touching prefs at
		// all — bump the shared revision so readers of the applied look (the
		// Appearance hero's accent chip, charts) re-render.
		uistate.BumpDataRevision()
	}
	uploadFont := func() {
		browser.PickFileNamed(".woff2,.woff,.ttf,.otf", func(name, mime string, data []byte) {
			if mime == "" {
				mime = theme.FontMIMEForName(name)
			}
			if errs := theme.ValidateFontUpload(mime, len(data)); len(errs) > 0 {
				fontMsg.Set(strings.Join(errs, " "))
				return
			}
			family := themeFontFamilyFromName(name)
			fonts.Set(uistate.AddFont(theme.FontAsset{Family: family, MIME: mime, DataURL: artifacts.DataURL(mime, data)}))
			fontMsg.Set("")
			// Start using the uploaded font for the interface right away.
			next := t
			next.FontUI = family
			apply(next)
		})
	}
	removeFont := func(family string) {
		fonts.Set(uistate.RemoveFont(family))
		// If the active theme referenced the removed font, fall back to a curated
		// one so nothing points at a now-missing @font-face.
		next := t
		changed := false
		if next.FontUI == family {
			next.FontUI = "Inter"
			changed = true
		}
		if next.FontDisplay == family {
			next.FontDisplay = "Fraunces"
			changed = true
		}
		if changed {
			apply(next)
		}
	}
	var fontRows []uic.Node
	for _, f := range fonts.Get() {
		fontRows = append(fontRows, uic.CreateElement(themeFontRow, themeFontRowProps{Family: f.Family, OnRemove: removeFont}))
	}

	banner := uic.UseState(uistate.LoadBanner())
	bannerMsg := uic.UseState("")
	setBanner := func(b theme.Banner) {
		uistate.PersistBanner(b)
		uistate.ApplyBanner(b)
		banner.Set(b)
	}
	uploadBanner := func() {
		browser.PickFileNamed(".png,.jpg,.jpeg,.webp,.gif", func(name, mime string, data []byte) {
			if mime == "" {
				mime = theme.ImageMIMEForName(name)
			}
			if errs := theme.ValidateImageUpload(mime, len(data)); len(errs) > 0 {
				bannerMsg.Set(strings.Join(errs, " "))
				return
			}
			bannerMsg.Set("")
			setBanner(theme.ImageBanner(artifacts.DataURL(mime, data), name))
		})
	}
	var bannerBtns []uic.Node
	for _, p := range theme.BannerPresets() {
		p := p
		bannerBtns = append(bannerBtns, themeDataBtn(p.Name, false, func() {
			bannerMsg.Set("")
			setBanner(p)
		}))
	}
	setColor := func(field, hex string) {
		n := t
		switch field {
		case "bgBase":
			n.BgBase = hex
		case "bgCard":
			n.BgCard = hex
		case "border":
			n.Border = hex
		case "text":
			n.Text = hex
		case "textDim":
			n.TextDim = hex
		case "accent":
			n.Accent = hex
		case "up":
			n.Up = hex
		case "down":
			n.Down = hex
		}
		apply(n)
	}

	onRadius := uic.UseEvent(func(e uic.Event) {
		if n, err := strconv.Atoi(e.GetValue()); err == nil {
			nt := t
			nt.Radius = n
			apply(nt)
		}
	})
	onScale := uic.UseEvent(func(e uic.Event) {
		if pct, err := strconv.Atoi(e.GetValue()); err == nil {
			nt := t
			nt.Scale = float64(pct) / 100
			apply(nt)
		}
	})
	onFontUI := uic.UseEvent(func(e uic.Event) {
		nt := t
		nt.FontUI = e.GetValue()
		apply(nt)
	})
	onFontDisplay := uic.UseEvent(func(e uic.Event) {
		nt := t
		nt.FontDisplay = e.GetValue()
		apply(nt)
	})

	// Built-in presets, each its own button component.
	var presetBtns []uic.Node
	for _, p := range theme.Presets() {
		presetBtns = append(presetBtns, uic.CreateElement(themePresetBtn, themePresetBtnProps{Theme: p, OnPick: apply}))
	}

	// Color tokens, each its own field component (keeps the change hook stable).
	colorTokens := []struct{ label, field, val string }{
		{uistate.T("themeEd.colBgBase"), "bgBase", t.BgBase},
		{uistate.T("themeEd.colBgCard"), "bgCard", t.BgCard},
		{uistate.T("themeEd.colBorder"), "border", t.Border},
		{uistate.T("themeEd.colText"), "text", t.Text},
		{uistate.T("themeEd.colTextDim"), "textDim", t.TextDim},
		{uistate.T("themeEd.colAccent"), "accent", t.Accent},
		{uistate.T("themeEd.colUp"), "up", t.Up},
		{uistate.T("themeEd.colDown"), "down", t.Down},
	}
	var colorFields []uic.Node
	for _, c := range colorTokens {
		colorFields = append(colorFields, uic.CreateElement(themeColorField, themeColorFieldProps{
			Label: c.label, Field: c.field, Value: c.val, OnSet: setColor,
		}))
	}

	// Live validation: surface any token that would make the theme unreadable.
	var warnings []uic.Node
	for _, is := range t.Validate() {
		warnings = append(warnings, Li(is.Field+": "+is.Message))
	}
	var validationNode uic.Node
	if len(warnings) > 0 {
		validationNode = Div(css.Class(tw.Mt2),
			P(css.Class(tw.TextXs), Style(map[string]string{"color": "#d8716f"}), uistate.T("themeEd.warnHead")),
			Ul(css.Class("muted", tw.TextXs), Style(map[string]string{"margin": "0.25rem 0 0", "padding-left": "1.1rem"}), warnings),
		)
	} else {
		validationNode = P(css.Class("muted", tw.TextXs, tw.Mt2), uistate.T("themeEd.allGood"))
	}

	scalePct := strconv.Itoa(int(t.Scale*100 + 0.5))

	return Div(css.Class("theme-editor"),
		H4(css.Class("set-label"), uistate.T("themeEd.title")),
		P(css.Class("muted", tw.TextXs), uistate.T("themeEd.lede")),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1), presetBtns),

		Div(css.Class("set-label", tw.Mt2), uistate.T("themeEd.colors")),
		Div(css.Class(tw.Grid, tw.GridCols2, tw.Gap2), colorFields),

		Div(css.Class("set-label", tw.Mt2), uistate.T("themeEd.shapeType")),
		Div(css.Class("toggle-row"),
			Span(uistate.T("themeEd.radius")),
			Input(css.Class("set-input"), Style(map[string]string{"width": "5.5rem"}), Type("number"), Attr("min", "0"), Attr("max", "48"), Attr("step", "1"), Attr("aria-label", uistate.T("themeEd.radiusAria")), Value(strconv.Itoa(t.Radius)), OnChange(onRadius)),
		),
		Div(css.Class("toggle-row"),
			Span(uistate.T("themeEd.textSize")),
			Input(css.Class("set-input"), Style(map[string]string{"width": "5.5rem"}), Type("number"), Attr("min", "70"), Attr("max", "200"), Attr("step", "5"), Attr("aria-label", uistate.T("themeEd.textSizeAria")), Value(scalePct), OnChange(onScale)),
		),
		Div(css.Class("toggle-row"),
			Span(uistate.T("themeEd.fontUI")),
			Select(css.Class("set-input"), Attr("aria-label", uistate.T("themeEd.fontUI")), OnChange(onFontUI), themeFontOptions(t.FontUI, fonts.Get())),
		),
		Div(css.Class("toggle-row"),
			Span(uistate.T("themeEd.fontDisplay")),
			Select(css.Class("set-input"), Attr("aria-label", uistate.T("themeEd.fontDisplay")), OnChange(onFontDisplay), themeFontOptions(t.FontDisplay, fonts.Get())),
		),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.ItemsCenter, tw.Gap2, tw.Py1),
			themeDataBtn(uistate.T("themeEd.uploadFont"), false, uploadFont),
			Span(css.Class("muted", tw.TextXs), uistate.T("themeEd.fontFormats")),
		),
		If(fontMsg.Get() != "", P(css.Class(tw.TextXs), Style(map[string]string{"color": "#d8716f"}), fontMsg.Get())),
		If(len(fontRows) > 0, Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1, tw.Py1), fontRows)),
		ui.Segmented(ui.SegmentedProps{
			Label:    uistate.T("themeEd.density"), // C318: name the radiogroup
			Options:  []ui.SegOption{{Value: string(theme.Comfortable), Label: uistate.T("themeEd.densityComfortable")}, {Value: string(theme.Compact), Label: uistate.T("themeEd.densityCompact")}},
			Selected: string(t.Density),
			OnSelect: func(v string) {
				nt := t
				nt.Density = theme.Density(v)
				apply(nt)
			},
		}),
		Div(css.Class("toggle-row"),
			Span(uistate.T("themeEd.iconWeight")),
			ui.Segmented(ui.SegmentedProps{
				Label:    uistate.T("themeEd.iconWeight"), // C318: name the radiogroup
				Options:  []ui.SegOption{{Value: "1.2", Label: uistate.T("themeEd.iconThin")}, {Value: "1.6", Label: uistate.T("themeEd.iconRegular")}, {Value: "2.2", Label: uistate.T("themeEd.iconBold")}},
				Selected: strconv.FormatFloat(t.IconStroke, 'g', -1, 64),
				OnSelect: func(v string) {
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						nt := t
						nt.IconStroke = f
						apply(nt)
					}
				},
			}),
		),

		Div(css.Class("set-label", tw.Mt2), uistate.T("themeEd.banner")),
		P(css.Class("muted", tw.TextXs), uistate.T("themeEd.bannerLede")),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1), bannerBtns),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.ItemsCenter, tw.Gap2, tw.Py1),
			themeDataBtn(uistate.T("themeEd.uploadImage"), false, uploadBanner),
			themeDataBtn(uistate.T("themeEd.removeBanner"), false, func() {
				bannerMsg.Set("")
				setBanner(theme.Banner{})
			}),
			Span(css.Class("muted", tw.TextXs), uistate.T("themeEd.imageFormats")),
		),
		If(!banner.Get().None(), P(css.Class("muted", tw.TextXs), uistate.T("themeEd.showing", banner.Get().Name))),
		If(bannerMsg.Get() != "", P(css.Class(tw.TextXs), Style(map[string]string{"color": "#d8716f"}), bannerMsg.Get())),

		validationNode,

		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1, tw.Mt2),
			themeDataBtn(uistate.T("themeEd.export"), false, func() {
				if b, err := t.ToJSON(); err == nil {
					browser.DownloadBytes("cashflux-theme.json", "application/json", b)
				}
			}),
			themeDataBtn(uistate.T("themeEd.import"), false, func() {
				browser.PickFile(".json", func(data []byte) {
					next, err := theme.FromJSON(data)
					if err != nil {
						importMsg.Set(uistate.T("themeEd.badImport"))
						return
					}
					importMsg.Set("")
					apply(next)
				})
			}),
			themeDataBtn(uistate.T("themeEd.reset"), false, func() {
				importMsg.Set("")
				// Clear the pinned theme rather than apply()-persisting a snapshot
				// of the default: a snapshot stays pinned to THIS moment's mode
				// forever, leaving the Mode control writing prefs nothing reads.
				uistate.ClearTheme()
				def := uistate.DefaultTheme()
				uistate.ApplyTheme(def)
				cur.Set(def)
				uistate.BumpDataRevision()
			}),
		),
		If(importMsg.Get() != "", P(css.Class(tw.TextXs, tw.Mt1), Style(map[string]string{"color": "#d8716f"}), importMsg.Get())),
	)
}

// curatedFonts are the font families offered for the UI and display fonts.
// Proper family names (Inter, Fraunces) are identifiers; the generic labels
// carry i18n keys resolved at render time.
var curatedFonts = []struct{ value, labelKey string }{
	{"Inter", ""},
	{"Fraunces", ""},
	{"ui-sans-serif, system-ui, sans-serif", "themeEd.fontSystemSans"},
	{"ui-serif, Georgia, serif", "themeEd.fontSystemSerif"},
	{"ui-monospace, SFMono-Regular, monospace", "themeEd.fontMonospace"},
}

// themeFontOptions renders the curated font <option>s plus any uploaded custom
// families, with the current one selected.
func themeFontOptions(current string, uploaded []theme.FontAsset) []uic.Node {
	seen := map[string]bool{}
	var opts []uic.Node
	for _, f := range curatedFonts {
		seen[f.value] = true
		label := f.value
		if f.labelKey != "" {
			label = uistate.T(f.labelKey)
		}
		opts = append(opts, Option(Value(f.value), SelectedIf(f.value == current), label))
	}
	for _, f := range uploaded {
		if f.Family == "" || seen[f.Family] {
			continue
		}
		seen[f.Family] = true
		opts = append(opts, Option(Value(f.Family), SelectedIf(f.Family == current), uistate.T("themeEd.uploadedSuffix", f.Family)))
	}
	return opts
}

// themeFontFamilyFromName derives a CSS font-family name from an uploaded file's
// name by stripping any directory and extension.
func themeFontFamilyFromName(name string) string {
	base := name
	if i := strings.LastIndexAny(base, `/\`); i >= 0 {
		base = base[i+1:]
	}
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return "Custom font"
	}
	return base
}

// themePresetBtnProps configures a preset-apply button.
type themePresetBtnProps struct {
	Theme  theme.Theme
	OnPick func(theme.Theme)
}

// themePresetBtn applies a built-in preset when clicked. Own component so its
// click hook is stable even though presets render in a loop.
func themePresetBtn(props themePresetBtnProps) uic.Node {
	return Button(css.Class("btn"), Type("button"),
		Title(uistate.T("themeEd.usePreset", props.Theme.Name)),
		OnClick(func() {
			if props.OnPick != nil {
				props.OnPick(props.Theme)
			}
		}),
		props.Theme.Name,
	)
}

// themeFontRowProps configures one uploaded-font row with a remove control.
type themeFontRowProps struct {
	Family   string
	OnRemove func(family string)
}

// themeFontRow lists one uploaded custom font with a remove button. Own component
// so each remove hook stays stable across the list.
func themeFontRow(props themeFontRowProps) uic.Node {
	return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2, tw.TextXs),
		Span(css.Class("muted", tw.Truncate), props.Family),
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap1), Type("button"),
			Attr("aria-label", uistate.T("themeEd.removeFamily", props.Family)),
			Title(uistate.T("themeEd.removeFamily", props.Family)),
			OnClick(func() {
				if props.OnRemove != nil {
					props.OnRemove(props.Family)
				}
			}),
			ui.Icon(icon.Close, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
			Span(uistate.T("themeEd.remove")),
		),
	)
}

// themeColorFieldProps configures one color-token picker.
type themeColorFieldProps struct {
	Label string
	Field string
	Value string
	OnSet func(field, hex string)
}

// themeColorField renders a labeled native color picker for one design token.
// Own component so each input's change hook is stable across the field list.
func themeColorField(props themeColorFieldProps) uic.Node {
	on := uic.UseEvent(func(e uic.Event) {
		if props.OnSet != nil {
			props.OnSet(props.Field, e.GetValue())
		}
	})
	val := props.Value
	if val == "" {
		val = "#000000"
	}
	return Label(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.TextXs),
		Input(Type("color"), Style(map[string]string{"width": "2rem", "height": "1.6rem", "padding": "0", "border": "none", "background": "none"}), Attr("aria-label", props.Label), Value(val), OnChange(on)),
		Span(css.Class("muted"), props.Label),
	)
}

// themeDataBtnProps configures a data-action button used by the theme editor.
type themeDataBtnProps struct {
	Label   string
	Danger  bool
	OnClick func()
}

// themeDataBtn renders a data-action button. Own component so each click hook
// stays stable across the button list.
func themeDataBtn(label string, danger bool, onClick func()) uic.Node {
	return uic.CreateElement(themeDataButton, themeDataBtnProps{Label: label, Danger: danger, OnClick: onClick})
}

func themeDataButton(props themeDataBtnProps) uic.Node {
	cls := "data-btn"
	if props.Danger {
		cls += " data-btn-danger"
	}
	onClick := props.OnClick
	return Button(css.Class(cls), Type("button"),
		OnClick(func() {
			if onClick != nil {
				onClick()
			}
		}),
		props.Label,
	)
}

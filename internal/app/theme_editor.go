//go:build js && wasm

package app

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/theme"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// themeEditor is the Settings → Appearance theme editor: pick a built-in preset,
// tweak every design token (surface/text/accent colors, corner radius, font-size
// scale, UI/display fonts, density), see live validation warnings, and reset to
// the default migrated from your display preferences. Every change applies and
// persists immediately (live theming), so the surrounding app is itself the
// preview. It is a self-contained component so its hooks stay isolated; mount it
// with uic.CreateElement(themeEditor). English strings are inline here rather
// than going through the shared i18n bundle, to keep this new panel decoupled.
func themeEditor() uic.Node {
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
		prefsAtom.Set(p)
		uistate.PersistPrefs(p)
		cur.Set(next)
	}
	uploadFont := func() {
		pickFileNamed(".woff2,.woff,.ttf,.otf", func(name, mime string, data []byte) {
			if mime == "" {
				mime = theme.FontMIMEForName(name)
			}
			if errs := theme.ValidateFontUpload(mime, len(data)); len(errs) > 0 {
				fontMsg.Set(strings.Join(errs, " "))
				return
			}
			family := fontFamilyFromName(name)
			fonts.Set(uistate.AddFont(theme.FontAsset{Family: family, MIME: mime, DataURL: artifacts.DataURL(mime, data)}))
			fontMsg.Set("")
			// Start using the uploaded font for the interface right away.
			next := t
			next.FontUI = family
			apply(next)
		})
	}

	banner := uic.UseState(uistate.LoadBanner())
	bannerMsg := uic.UseState("")
	setBanner := func(b theme.Banner) {
		uistate.PersistBanner(b)
		uistate.ApplyBanner(b)
		banner.Set(b)
	}
	uploadBanner := func() {
		pickFileNamed(".png,.jpg,.jpeg,.webp,.gif", func(name, mime string, data []byte) {
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
		bannerBtns = append(bannerBtns, dataBtn(p.Name, false, func() {
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
		{"App background", "bgBase", t.BgBase},
		{"Card surface", "bgCard", t.BgCard},
		{"Borders", "border", t.Border},
		{"Text", "text", t.Text},
		{"Muted text", "textDim", t.TextDim},
		{"Accent", "accent", t.Accent},
		{"Positive / inflow", "up", t.Up},
		{"Negative / outflow", "down", t.Down},
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
		validationNode = Div(Class("mt-2"),
			P(Class("text-xs"), Style(map[string]string{"color": "#d8716f"}), "Some tokens may be hard to read:"),
			Ul(Class("text-xs muted"), Style(map[string]string{"margin": "0.25rem 0 0", "padding-left": "1.1rem"}), warnings),
		)
	} else {
		validationNode = P(Class("muted text-xs mt-2"), "Looks good — all text meets the contrast guideline.")
	}

	scalePct := strconv.Itoa(int(t.Scale*100 + 0.5))

	return Div(Class("theme-editor"),
		Div(Class("set-label"), "Theme"),
		P(Class("muted text-xs"), "Start from a preset, then fine-tune any color, the corner radius, the text size, and the fonts. Changes apply instantly."),
		Div(Class("flex flex-wrap gap-2 py-1"), presetBtns),

		Div(Class("set-label mt-2"), "Colors"),
		Div(Class("grid grid-cols-2 gap-2"), colorFields),

		Div(Class("set-label mt-2"), "Shape & type"),
		Div(Class("toggle-row"),
			Span("Corner radius"),
			Input(Class("set-input"), Style(map[string]string{"width": "5.5rem"}), Type("number"), Attr("min", "0"), Attr("max", "48"), Attr("step", "1"), Attr("aria-label", "Corner radius in pixels"), Value(strconv.Itoa(t.Radius)), OnChange(onRadius)),
		),
		Div(Class("toggle-row"),
			Span("Text size"),
			Input(Class("set-input"), Style(map[string]string{"width": "5.5rem"}), Type("number"), Attr("min", "70"), Attr("max", "200"), Attr("step", "5"), Attr("aria-label", "Text size percent"), Value(scalePct), OnChange(onScale)),
		),
		Div(Class("toggle-row"),
			Span("Interface font"),
			Select(Class("set-input"), Attr("aria-label", "Interface font"), OnChange(onFontUI), fontOptions(t.FontUI, fonts.Get())),
		),
		Div(Class("toggle-row"),
			Span("Heading font"),
			Select(Class("set-input"), Attr("aria-label", "Heading font"), OnChange(onFontDisplay), fontOptions(t.FontDisplay, fonts.Get())),
		),
		Div(Class("flex flex-wrap items-center gap-2 py-1"),
			dataBtn("Upload font…", false, uploadFont),
			Span(Class("muted text-xs"), "WOFF2, WOFF, TTF, or OTF · up to 1 MB"),
		),
		If(fontMsg.Get() != "", P(Class("text-xs"), Style(map[string]string{"color": "#d8716f"}), fontMsg.Get())),
		ui.Segmented(ui.SegmentedProps{
			Options:  []ui.SegOption{{Value: string(theme.Comfortable), Label: "Comfortable"}, {Value: string(theme.Compact), Label: "Compact"}},
			Selected: string(t.Density),
			OnSelect: func(v string) {
				nt := t
				nt.Density = theme.Density(v)
				apply(nt)
			},
		}),

		Div(Class("set-label mt-2"), "Dashboard banner"),
		P(Class("muted text-xs"), "A decorative band atop the dashboard. Choose a gradient or upload your own image."),
		Div(Class("flex flex-wrap gap-2 py-1"), bannerBtns),
		Div(Class("flex flex-wrap items-center gap-2 py-1"),
			dataBtn("Upload image…", false, uploadBanner),
			dataBtn("Remove banner", false, func() {
				bannerMsg.Set("")
				setBanner(theme.Banner{})
			}),
			Span(Class("muted text-xs"), "PNG, JPEG, WebP, or GIF · up to 2 MB"),
		),
		If(!banner.Get().None(), P(Class("muted text-xs"), "Showing: "+banner.Get().Name)),
		If(bannerMsg.Get() != "", P(Class("text-xs"), Style(map[string]string{"color": "#d8716f"}), bannerMsg.Get())),

		validationNode,

		Div(Class("flex flex-wrap gap-2 py-1 mt-2"),
			dataBtn("Export theme", false, func() {
				if b, err := t.ToJSON(); err == nil {
					downloadBytes("cashflux-theme.json", "application/json", b)
				}
			}),
			dataBtn("Import theme", false, func() {
				pickFile(".json", func(data []byte) {
					next, err := theme.FromJSON(data)
					if err != nil {
						importMsg.Set("That file isn't a valid theme.")
						return
					}
					importMsg.Set("")
					apply(next)
				})
			}),
			dataBtn("Reset to default", false, func() {
				importMsg.Set("")
				apply(uistate.DefaultTheme())
			}),
		),
		If(importMsg.Get() != "", P(Class("text-xs mt-1"), Style(map[string]string{"color": "#d8716f"}), importMsg.Get())),
	)
}

// curatedFonts are the font families offered for the UI and display fonts. Inter
// and Fraunces are already loaded; "system" falls back to the OS sans stack.
var curatedFonts = []struct{ value, label string }{
	{"Inter", "Inter"},
	{"Fraunces", "Fraunces"},
	{"ui-sans-serif, system-ui, sans-serif", "System sans"},
	{"ui-serif, Georgia, serif", "System serif"},
	{"ui-monospace, SFMono-Regular, monospace", "Monospace"},
}

// fontOptions renders the curated font <option>s plus any uploaded custom
// families, with the current one selected. Uploaded families that duplicate a
// curated value are skipped so the list stays clean.
func fontOptions(current string, uploaded []theme.FontAsset) []uic.Node {
	seen := map[string]bool{}
	var opts []uic.Node
	for _, f := range curatedFonts {
		seen[f.value] = true
		opts = append(opts, Option(Value(f.value), SelectedIf(f.value == current), f.label))
	}
	for _, f := range uploaded {
		if f.Family == "" || seen[f.Family] {
			continue
		}
		seen[f.Family] = true
		opts = append(opts, Option(Value(f.Family), SelectedIf(f.Family == current), f.Family+" (uploaded)"))
	}
	return opts
}

// fontFamilyFromName derives a CSS font-family name from an uploaded file's name
// by stripping any directory and extension. Spaces are kept (the family is quoted
// in the @font-face rule). A blank result falls back to a generic label.
func fontFamilyFromName(name string) string {
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
	return Button(Class("btn"), Type("button"),
		Title("Use the "+props.Theme.Name+" preset"),
		OnClick(func() {
			if props.OnPick != nil {
				props.OnPick(props.Theme)
			}
		}),
		props.Theme.Name,
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
	return Label(Class("flex items-center gap-2 text-xs"),
		Input(Type("color"), Style(map[string]string{"width": "2rem", "height": "1.6rem", "padding": "0", "border": "none", "background": "none"}), Attr("aria-label", props.Label), Value(val), OnChange(on)),
		Span(Class("muted"), props.Label),
	)
}

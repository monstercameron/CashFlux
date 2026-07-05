// SPDX-License-Identifier: MIT

package tw

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v4/css"
)

// emit folds a token (a css.Rule or []css.Rule) into a class and returns the CSS
// the registry emitted for it, so tests can assert exact declarations.
func emit(t *testing.T, token any) string {
	t.Helper()
	css.Reset()
	var rules []css.Rule
	switch v := token.(type) {
	case css.Rule:
		rules = []css.Rule{v}
	case []css.Rule:
		rules = v
	default:
		t.Fatalf("unsupported token type %T", token)
	}
	if css.New(rules...) == "" {
		t.Fatalf("token folded to an empty class")
	}
	return css.Harvest()
}

func TestExactTailwindValues(t *testing.T) {
	cases := []struct {
		name  string
		token any
		want  []string // substrings that must all appear in the emitted CSS
	}{
		{"Gap2", Gap2, []string{"gap:0.5rem"}},
		{"Gap15", Gap15, []string{"gap:0.375rem"}},
		{"Mt1", Mt1, []string{"margin-top:0.25rem"}},
		{"Mt045", Mt045, []string{"margin-top:0.45rem"}},
		{"Px3", Px3, []string{"padding-left:0.75rem", "padding-right:0.75rem"}},
		{"Py25", Py25, []string{"padding-top:0.625rem", "padding-bottom:0.625rem"}},
		{"W4", W4, []string{"width:1rem"}},
		{"H4", H4, []string{"height:1rem"}},
		{"W18px", W18px, []string{"width:18px"}},
		{"HScreen", HScreen, []string{"height:100vh"}},
		{"MinW0", MinW0, []string{"min-width:0"}},
		{"TextFaint", TextFaint, []string{"color:var(--text-faint,#7d7d85)"}}, // theme-aware (GX14)
		{"TextDown", TextDown, []string{"color:var(--down,#d8716f)"}},
		{"BgBase", BgBase, []string{"background-color:#0e0e0f"}},
		{"Rounded4", Rounded4, []string{"border-radius:4px"}},
		{"RoundedFull", RoundedFull, []string{"border-radius:9999px"}},
		{"Rounded", Rounded, []string{"border-radius:0.25rem"}},
		{"Text12", Text12, []string{"font-size:12px"}},
		{"TextXs", TextXs, []string{"font-size:0.75rem", "line-height:1rem"}},
		{"Border", Border, []string{"border-width:1px", "border-style:solid"}},
		{"BorderLine", BorderLine, []string{"border-color:#232325"}},
		{"ShadowLg", ShadowLg, []string{"box-shadow:0 10px 15px"}},
		{"Opacity60", Opacity60, []string{"opacity:0.6"}},
		{"Uppercase", Uppercase, []string{"text-transform:uppercase"}},
		{"FlexWrap", FlexWrap, []string{"flex-wrap:wrap"}},
		{"Flex1", Flex1, []string{"flex:1 1 0%"}},
		{"GridCols3", GridCols3, []string{"grid-template-columns:repeat(3, minmax(0, 1fr))"}},
		{"Truncate", Truncate, []string{"overflow:hidden", "text-overflow:ellipsis", "white-space:nowrap"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalize(emit(t, c.token))
			for _, w := range c.want {
				if !strings.Contains(got, normalize(w)) {
					t.Errorf("%s: emitted CSS %q missing %q", c.name, got, w)
				}
			}
		})
	}
}

func TestVariantsAndSelectors(t *testing.T) {
	// hover:bg-hover → a :hover rule with the hover background.
	if got := normalize(emit(t, HoverBgHover)); !strings.Contains(got, ":hover") || !strings.Contains(got, "background-color:#161617") {
		t.Errorf("HoverBgHover emitted %q, want :hover + background-color:#161617", got)
	}
	// space-y-4 → a child-combinator rule applying margin-top to non-first children.
	if got := normalize(emit(t, SpaceY4)); !strings.Contains(got, ">*+*") || !strings.Contains(got, "margin-top:1rem") {
		t.Errorf("SpaceY4 emitted %q, want '>*+*' + margin-top:1rem", got)
	}
	// group-hover:opacity-100 → a .group:hover descendant rule.
	if got := normalize(emit(t, GroupHoverOpacity100)); !strings.Contains(got, ".group:hover") || !strings.Contains(got, "opacity:1") {
		t.Errorf("GroupHoverOpacity100 emitted %q, want .group:hover + opacity:1", got)
	}
}

// normalize strips whitespace so substring checks are robust to formatting.
func normalize(s string) string {
	return strings.Join(strings.Fields(s), "")
}

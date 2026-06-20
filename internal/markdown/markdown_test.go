package markdown

import "testing"

func TestParseBlocks(t *testing.T) {
	t.Run("paragraph joins soft-wrapped lines", func(t *testing.T) {
		bs := Parse("hello\nworld")
		if len(bs) != 1 || bs[0].Kind != Paragraph {
			t.Fatalf("want 1 paragraph, got %+v", bs)
		}
		if got := plainText(bs[0].Inlines); got != "hello world" {
			t.Fatalf("paragraph text = %q", got)
		}
	})

	t.Run("blank line splits paragraphs", func(t *testing.T) {
		bs := Parse("one\n\ntwo")
		if len(bs) != 2 || bs[0].Kind != Paragraph || bs[1].Kind != Paragraph {
			t.Fatalf("want 2 paragraphs, got %+v", bs)
		}
	})

	t.Run("headings carry level", func(t *testing.T) {
		bs := Parse("# Title\n### Sub")
		if len(bs) != 2 {
			t.Fatalf("want 2 blocks, got %d", len(bs))
		}
		if bs[0].Kind != Heading || bs[0].Level != 1 || plainText(bs[0].Inlines) != "Title" {
			t.Fatalf("h1 wrong: %+v", bs[0])
		}
		if bs[1].Level != 3 {
			t.Fatalf("h3 level = %d", bs[1].Level)
		}
	})

	t.Run("seven hashes is not a heading", func(t *testing.T) {
		bs := Parse("####### nope")
		if bs[0].Kind != Paragraph {
			t.Fatalf("want paragraph, got %v", bs[0].Kind)
		}
	})

	t.Run("hash without space is not a heading", func(t *testing.T) {
		bs := Parse("#tag")
		if bs[0].Kind != Paragraph {
			t.Fatalf("want paragraph, got %v", bs[0].Kind)
		}
	})

	t.Run("unordered list", func(t *testing.T) {
		bs := Parse("- a\n- b\n* c")
		if len(bs) != 1 || bs[0].Kind != List || bs[0].Ordered {
			t.Fatalf("want one unordered list, got %+v", bs)
		}
		if len(bs[0].Items) != 3 {
			t.Fatalf("want 3 items, got %d", len(bs[0].Items))
		}
		if plainText(bs[0].Items[1]) != "b" {
			t.Fatalf("item[1] = %q", plainText(bs[0].Items[1]))
		}
	})

	t.Run("ordered list", func(t *testing.T) {
		bs := Parse("1. first\n2) second")
		if len(bs) != 1 || !bs[0].Ordered || len(bs[0].Items) != 2 {
			t.Fatalf("want ordered list of 2, got %+v", bs)
		}
	})

	t.Run("ordered and unordered do not merge", func(t *testing.T) {
		bs := Parse("- a\n1. b")
		if len(bs) != 2 {
			t.Fatalf("want 2 separate lists, got %d", len(bs))
		}
	})

	t.Run("fenced code preserves contents and lang", func(t *testing.T) {
		bs := Parse("```go\nx := 1\ny := 2\n```")
		if len(bs) != 1 || bs[0].Kind != Code {
			t.Fatalf("want code block, got %+v", bs)
		}
		if bs[0].Lang != "go" {
			t.Fatalf("lang = %q", bs[0].Lang)
		}
		if bs[0].Text != "x := 1\ny := 2" {
			t.Fatalf("code text = %q", bs[0].Text)
		}
	})

	t.Run("code block is not inline-parsed", func(t *testing.T) {
		bs := Parse("```\n**not bold**\n```")
		if bs[0].Kind != Code || bs[0].Text != "**not bold**" {
			t.Fatalf("code mangled: %+v", bs[0])
		}
	})

	t.Run("empty input", func(t *testing.T) {
		if bs := Parse(""); len(bs) != 0 {
			t.Fatalf("want no blocks, got %d", len(bs))
		}
	})
}

func TestParseInline(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Inline
	}{
		{"plain", "just text", []Inline{{Kind: Text, Content: "just text"}}},
		{"bold star", "a **b** c", []Inline{
			{Kind: Text, Content: "a "}, {Kind: Strong, Content: "b"}, {Kind: Text, Content: " c"},
		}},
		{"bold underscore", "__x__", []Inline{{Kind: Strong, Content: "x"}}},
		{"italic", "a *b* c", []Inline{
			{Kind: Text, Content: "a "}, {Kind: Emphasis, Content: "b"}, {Kind: Text, Content: " c"},
		}},
		{"code span", "use `go test`", []Inline{
			{Kind: Text, Content: "use "}, {Kind: CodeSpan, Content: "go test"},
		}},
		{"code not parsed inside", "`**x**`", []Inline{{Kind: CodeSpan, Content: "**x**"}}},
		{"link", "see [docs](https://x.dev)", []Inline{
			{Kind: Text, Content: "see "}, {Kind: Link, Content: "docs", URL: "https://x.dev"},
		}},
		{"unterminated bold is literal", "a **b", []Inline{{Kind: Text, Content: "a **b"}}},
		{"unterminated code is literal", "a `b", []Inline{{Kind: Text, Content: "a `b"}}},
		{"malformed link is literal", "[no paren]", []Inline{{Kind: Text, Content: "[no paren]"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseInline(tc.in)
			if !inlinesEqual(got, tc.want) {
				t.Fatalf("parseInline(%q)\n got %+v\nwant %+v", tc.in, got, tc.want)
			}
		})
	}
}

func plainText(ins []Inline) string {
	s := ""
	for _, in := range ins {
		s += in.Content
	}
	return s
}

func inlinesEqual(a, b []Inline) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

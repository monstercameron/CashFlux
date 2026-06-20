// Package markdown is a tiny, dependency-free Markdown parser that turns the
// lightweight Markdown the AI assistant emits (headings, bold/italic, inline
// code, fenced code, links, and bullet/number lists) into a structured block
// tree. It is pure Go with no syscall/js so it unit-tests natively; the UI layer
// walks the returned []Block to build framework nodes, never injecting raw HTML
// (the security boundary lives in the renderer, which only emits known elements).
//
// The supported subset is deliberately small and forgiving: anything it does not
// recognise degrades to plain paragraph text rather than erroring, so an
// unexpected token never breaks a rendered answer.
package markdown

import "strings"

// BlockKind enumerates the top-level block types the parser emits.
type BlockKind int

const (
	// Paragraph is a run of text rendered as a <p>.
	Paragraph BlockKind = iota
	// Heading is a # .. ###### heading; Level carries 1–6.
	Heading
	// List is a bullet or numbered list; Ordered distinguishes the two and
	// Items holds each item's parsed inline spans.
	List
	// Code is a fenced code block; Text holds its raw, unparsed contents and
	// Lang the optional info string after the opening fence.
	Code
)

// Block is one top-level element of a parsed document.
type Block struct {
	Kind    BlockKind
	Level   int        // Heading: 1–6
	Inlines []Inline   // Paragraph, Heading
	Items   [][]Inline // List: one inline run per item
	Ordered bool       // List: true for "1." lists, false for "-"/"*"
	Text    string     // Code: raw block contents
	Lang    string     // Code: optional language hint
}

// InlineKind enumerates inline span types within a block.
type InlineKind int

const (
	// Text is a plain, unstyled span.
	Text InlineKind = iota
	// Strong is bold (**…** or __…__).
	Strong
	// Emphasis is italic (*…* or _…_).
	Emphasis
	// CodeSpan is inline `code`.
	CodeSpan
	// Link is [label](url); Content holds the label, URL the destination.
	Link
)

// Inline is a styled span of text within a block.
type Inline struct {
	Kind    InlineKind
	Content string
	URL     string // Link only
}

// Parse converts Markdown source into a slice of blocks. It is total: any input
// produces a (possibly empty) block list and never panics.
func Parse(src string) []Block {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var blocks []Block
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Blank lines separate blocks.
		if trimmed == "" {
			continue
		}

		// Fenced code block: ``` optionally followed by a language hint, until
		// the next ``` (or end of input).
		if strings.HasPrefix(trimmed, "```") {
			lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			var body []string
			i++
			for ; i < len(lines); i++ {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					break
				}
				body = append(body, lines[i])
			}
			blocks = append(blocks, Block{Kind: Code, Text: strings.Join(body, "\n"), Lang: lang})
			continue
		}

		// Heading: 1–6 leading '#' then a space.
		if lvl := headingLevel(trimmed); lvl > 0 {
			text := strings.TrimSpace(trimmed[lvl:])
			blocks = append(blocks, Block{Kind: Heading, Level: lvl, Inlines: parseInline(text)})
			continue
		}

		// List: a run of consecutive bullet or ordered items. A list is
		// homogeneous — the first item's marker fixes ordered vs unordered.
		if ordered, content, ok := listItem(trimmed); ok {
			items := [][]Inline{parseInline(content)}
			for i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				o2, c2, ok2 := listItem(next)
				if !ok2 || o2 != ordered {
					break
				}
				items = append(items, parseInline(c2))
				i++
			}
			blocks = append(blocks, Block{Kind: List, Ordered: ordered, Items: items})
			continue
		}

		// Paragraph: gather consecutive non-blank, non-special lines and join
		// them with spaces (Markdown soft-wraps).
		para := []string{trimmed}
		for i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if next == "" || headingLevel(next) > 0 || strings.HasPrefix(next, "```") {
				break
			}
			if _, _, ok := listItem(next); ok {
				break
			}
			para = append(para, next)
			i++
		}
		blocks = append(blocks, Block{Kind: Paragraph, Inlines: parseInline(strings.Join(para, " "))})
	}
	return blocks
}

// headingLevel returns the heading level (1–6) for a "# …" line, or 0 if the
// line is not a heading (no marker, more than six '#', or no following space).
func headingLevel(s string) int {
	n := 0
	for n < len(s) && s[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n >= len(s) || s[n] != ' ' {
		return 0
	}
	return n
}

// listItem reports whether s is a list item and, if so, whether it is ordered
// ("1." / "2)") or unordered ("-" / "*" / "+") plus the item's content text.
func listItem(s string) (ordered bool, content string, ok bool) {
	if len(s) >= 2 && (s[0] == '-' || s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return false, strings.TrimSpace(s[2:]), true
	}
	// Ordered: one or more digits, then '.' or ')', then a space.
	d := 0
	for d < len(s) && s[d] >= '0' && s[d] <= '9' {
		d++
	}
	if d > 0 && d+1 < len(s) && (s[d] == '.' || s[d] == ')') && s[d+1] == ' ' {
		return true, strings.TrimSpace(s[d+2:]), true
	}
	return false, "", false
}

// parseInline scans a line into styled spans. It recognises inline code first
// (its contents are never re-parsed), then links, then bold, then italic, and
// emits the rest as plain text. Unterminated markers are treated literally so
// stray '*' or '`' never swallow the remainder of a line.
func parseInline(s string) []Inline {
	var out []Inline
	var plain strings.Builder
	flush := func() {
		if plain.Len() > 0 {
			out = append(out, Inline{Kind: Text, Content: plain.String()})
			plain.Reset()
		}
	}

	i := 0
	for i < len(s) {
		switch {
		case s[i] == '`':
			if j := strings.IndexByte(s[i+1:], '`'); j >= 0 {
				flush()
				out = append(out, Inline{Kind: CodeSpan, Content: s[i+1 : i+1+j]})
				i += j + 2
				continue
			}
		case s[i] == '[':
			if label, url, n, ok := parseLink(s[i:]); ok {
				flush()
				out = append(out, Inline{Kind: Link, Content: label, URL: url})
				i += n
				continue
			}
		case strings.HasPrefix(s[i:], "**") || strings.HasPrefix(s[i:], "__"):
			marker := s[i : i+2]
			if j := strings.Index(s[i+2:], marker); j >= 0 {
				flush()
				out = append(out, Inline{Kind: Strong, Content: s[i+2 : i+2+j]})
				i += j + 4
				continue
			}
		case s[i] == '*' || s[i] == '_':
			marker := s[i]
			if j := strings.IndexByte(s[i+1:], marker); j >= 0 {
				flush()
				out = append(out, Inline{Kind: Emphasis, Content: s[i+1 : i+1+j]})
				i += j + 2
				continue
			}
		}
		plain.WriteByte(s[i])
		i++
	}
	flush()
	return out
}

// parseLink parses a "[label](url)" starting at s[0]=='[', returning the label,
// URL, the number of bytes consumed, and whether a well-formed link was found.
func parseLink(s string) (label, url string, n int, ok bool) {
	close := strings.IndexByte(s, ']')
	if close < 0 || close+1 >= len(s) || s[close+1] != '(' {
		return "", "", 0, false
	}
	end := strings.IndexByte(s[close+2:], ')')
	if end < 0 {
		return "", "", 0, false
	}
	return s[1:close], strings.TrimSpace(s[close+2 : close+2+end]), close + 2 + end + 1, true
}

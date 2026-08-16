// SPDX-License-Identifier: MIT

// Package chatpolish holds the pure logic behind the assistant's conversation
// management (G2-C7): searching across saved chats, exporting one as Markdown,
// and the rules for renaming and for editing-and-resending a past message.
//
// These are the parts of "chat polish" that are decisions rather than DOM: what
// counts as a match, what a title becomes when someone clears it, where a
// conversation is truncated when a question is asked again differently. Keeping
// them here means each one is a table test rather than a browser click, and the
// screen stays a thin shell over answers that are already settled.
package chatpolish

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Match is one conversation that matched a search, with the reason it matched.
type Match struct {
	// Conversation is the matching chat.
	Conversation domain.Conversation
	// InTitle reports whether the query hit the conversation's name.
	InTitle bool
	// Hits is how many messages matched.
	Hits int
	// Excerpt is a short window around the first match inside a message, so a
	// result says WHY it matched instead of only that it did.
	Excerpt string
}

// Search finds conversations matching a free-text query, newest first.
//
// It searches titles and message text together, because a person looking for "the
// chat about the car insurance" does not know or care whether that phrase was the
// title or something they typed inside it. An empty or whitespace query matches
// nothing rather than everything: a search box that shows all results when empty
// looks identical to one that is broken.
func Search(convs []domain.Conversation, query string) []Match {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	out := make([]Match, 0, len(convs))
	for _, c := range convs {
		m := Match{Conversation: c, InTitle: strings.Contains(strings.ToLower(c.Title), q)}
		for _, msg := range c.Messages {
			idx := strings.Index(strings.ToLower(msg.Text), q)
			if idx < 0 {
				continue
			}
			m.Hits++
			if m.Excerpt == "" {
				m.Excerpt = excerpt(msg.Text, idx, len(q))
			}
		}
		if m.InTitle || m.Hits > 0 {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Conversation.UpdatedAt.After(out[j].Conversation.UpdatedAt)
	})
	return out
}

// excerptRadius is how much text surrounds a hit in the excerpt. Wide enough to
// carry a clause, short enough to stay on one line in the rail.
const excerptRadius = 40

// excerpt returns a window of text around a match, with ellipses where it was cut.
// It cuts on rune boundaries so a multi-byte character never splits into mojibake.
func excerpt(text string, idx, length int) string {
	runes := []rune(text)
	// idx is a byte offset; convert it to a rune offset so the window lands in the
	// right place in text that isn't pure ASCII.
	start := len([]rune(text[:idx]))
	end := start + len([]rune(text[idx:idx+length]))
	from := start - excerptRadius
	if from < 0 {
		from = 0
	}
	to := end + excerptRadius
	if to > len(runes) {
		to = len(runes)
	}
	out := strings.TrimSpace(string(runes[from:to]))
	if from > 0 {
		out = "…" + out
	}
	if to < len(runes) {
		out += "…"
	}
	return collapseSpace(out)
}

// collapseSpace flattens newlines and runs of whitespace so an excerpt stays on
// one line in a list.
func collapseSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// ExportMarkdown renders a conversation as a Markdown document: a title, when it
// happened, and each turn attributed.
//
// Markdown rather than JSON because the export is for a person — pasting into a
// note, sending to an accountant, keeping a record of what the assistant said
// before acting on it. The assistant's answers are already Markdown, so they pass
// through as themselves rather than being escaped into something less readable.
func ExportMarkdown(c domain.Conversation) string {
	var b strings.Builder
	title := strings.TrimSpace(c.Title)
	if title == "" {
		title = "Conversation"
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	if !c.CreatedAt.IsZero() {
		fmt.Fprintf(&b, "*%s*\n\n", c.CreatedAt.Format("2 January 2006, 15:04"))
	}
	for _, m := range c.Messages {
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", speaker(m.Role), strings.TrimSpace(m.Text))
		// The sources are the reason to keep the export at all — an answer with a
		// figure in it is only checkable if what it was computed from travels with
		// it.
		for _, s := range m.Sources {
			label := strings.TrimSpace(s.Label)
			if strings.TrimSpace(s.Scope) != "" {
				label += " · " + strings.TrimSpace(s.Scope)
			}
			fmt.Fprintf(&b, "> **Source:** %s\n", label)
			if ev := strings.TrimSpace(s.Evidence); ev != "" {
				for _, line := range strings.Split(ev, "\n") {
					fmt.Fprintf(&b, "> %s\n", line)
				}
			}
			b.WriteString(">\n")
		}
		if len(m.Sources) > 0 {
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// speaker names a role for the export in the second person, matching how the
// conversation reads on screen.
func speaker(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "user":
		return "You"
	case "assistant":
		return "Assistant"
	case "afford":
		return "Affordability check"
	default:
		return "Note"
	}
}

// ExportFilename builds a filesystem-safe name for an exported conversation,
// dated so several exports of the same chat do not overwrite each other in a
// downloads folder.
func ExportFilename(c domain.Conversation, at time.Time) string {
	base := slug(c.Title)
	if base == "" {
		base = "conversation"
	}
	return fmt.Sprintf("cashflux-%s-%s.md", base, at.Format("2006-01-02"))
}

// slug reduces a title to lowercase words joined by hyphens, capped so a long
// first question does not become a 200-character filename.
func slug(title string) string {
	var b strings.Builder
	lastHyphen := true
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 48 {
		out = strings.Trim(out[:48], "-")
	}
	return out
}

// CleanTitle normalises a renamed conversation's title: trimmed, collapsed, and
// capped. It returns ok=false when the result is empty, which the caller treats as
// "go back to deriving the title from the first message" rather than as an error —
// clearing a name is a legitimate thing to want, and it should not leave a chat
// called nothing.
func CleanTitle(title string) (string, bool) {
	out := collapseSpace(title)
	if out == "" {
		return "", false
	}
	if r := []rune(out); len(r) > 80 {
		out = strings.TrimSpace(string(r[:80]))
	}
	return out, true
}

// TruncateForResend returns the messages that should remain when a past message is
// edited and asked again: everything before it, plus the edited message itself.
//
// Everything after is dropped, because it was the assistant answering a question
// that is no longer the question. Keeping those turns would leave a thread whose
// replies do not follow from what is above them — and, worse, would feed that
// contradiction back to the model as context. ok is false when the id is not in
// the conversation or does not name a message the user wrote.
func TruncateForResend(msgs []domain.ChatMessage, id, newText string) (out []domain.ChatMessage, ok bool) {
	text := strings.TrimSpace(newText)
	if text == "" {
		return nil, false
	}
	for i, m := range msgs {
		if m.ID != id {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(m.Role), "user") {
			return nil, false
		}
		out = append(out, msgs[:i]...)
		edited := m
		edited.Text = text
		// The old turn's token accounting belonged to the old question.
		edited.Tokens, edited.PromptTokens, edited.CompletionTokens = 0, 0, 0
		edited.Sources = nil
		return append(out, edited), true
	}
	return nil, false
}

// Feedback is a reader's verdict on one assistant answer.
type Feedback string

const (
	// FeedbackNone is the absence of a verdict — the normal state.
	FeedbackNone Feedback = ""
	// FeedbackUp means the answer was useful.
	FeedbackUp Feedback = "up"
	// FeedbackDown means it was not.
	FeedbackDown Feedback = "down"
)

// ToggleFeedback returns the new verdict after clicking one of the two buttons,
// so clicking the same button again clears it. A rating that cannot be taken back
// is a rating people stop giving.
func ToggleFeedback(current, clicked Feedback) Feedback {
	if current == clicked {
		return FeedbackNone
	}
	return clicked
}

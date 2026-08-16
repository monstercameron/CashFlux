// SPDX-License-Identifier: MIT

package chatpolish

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func conv(id, title string, updated time.Time, msgs ...domain.ChatMessage) domain.Conversation {
	return domain.Conversation{ID: id, Title: title, UpdatedAt: updated, CreatedAt: updated, Messages: msgs}
}

func msg(id, role, text string) domain.ChatMessage {
	return domain.ChatMessage{ID: id, Role: role, Text: text}
}

func TestSearchMatchesTitlesAndMessageText(t *testing.T) {
	day := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	convs := []domain.Conversation{
		conv("a", "Car insurance", day, msg("m1", "user", "is my premium high?")),
		conv("b", "Groceries", day.Add(time.Hour), msg("m2", "user", "what did we spend on car insurance last year?")),
		conv("c", "Holidays", day, msg("m3", "user", "how much for flights?")),
	}
	got := Search(convs, "car insurance")
	if len(got) != 2 {
		t.Fatalf("matches = %d, want 2", len(got))
	}
	// Newest first: b was updated an hour later than a.
	if got[0].Conversation.ID != "b" || got[1].Conversation.ID != "a" {
		t.Fatalf("order = %s,%s — want newest first", got[0].Conversation.ID, got[1].Conversation.ID)
	}
	if !got[1].InTitle {
		t.Fatal("a matched on its title but InTitle is false")
	}
	if got[0].Hits != 1 || got[0].Excerpt == "" {
		t.Fatalf("b should report its message hit with an excerpt: %+v", got[0])
	}
}

func TestSearchIsCaseInsensitive(t *testing.T) {
	convs := []domain.Conversation{conv("a", "Car Insurance", time.Now())}
	if got := Search(convs, "CAR insurance"); len(got) != 1 {
		t.Fatalf("matches = %d, want 1", len(got))
	}
}

func TestAnEmptyQueryMatchesNothing(t *testing.T) {
	convs := []domain.Conversation{conv("a", "Anything", time.Now(), msg("m", "user", "hello"))}
	for _, q := range []string{"", "   ", "\t"} {
		if got := Search(convs, q); len(got) != 0 {
			t.Fatalf("query %q returned %d matches — an empty search that shows everything looks broken", q, len(got))
		}
	}
}

func TestExcerptShowsTheSurroundingClauseAndMarksTheCut(t *testing.T) {
	long := "We were talking about the weekly shop and then " +
		"the car insurance renewal came up, which is due in March next year sometime."
	convs := []domain.Conversation{conv("a", "Chat", time.Now(), msg("m", "user", long))}
	got := Search(convs, "car insurance")
	if len(got) != 1 {
		t.Fatalf("matches = %d", len(got))
	}
	ex := got[0].Excerpt
	if !strings.Contains(ex, "car insurance") {
		t.Fatalf("excerpt does not contain the match: %q", ex)
	}
	if !strings.HasPrefix(ex, "…") || !strings.HasSuffix(ex, "…") {
		t.Fatalf("excerpt does not mark that it was cut on both sides: %q", ex)
	}
	if strings.Contains(ex, "\n") {
		t.Fatalf("excerpt spans lines: %q", ex)
	}
}

func TestExcerptSurvivesNonASCIIText(t *testing.T) {
	// A byte-offset window would slice a multi-byte rune in half here.
	convs := []domain.Conversation{conv("a", "Chat", time.Now(),
		msg("m", "user", "Café — naïve — résumé — we spent £312 on café visits this month, apparently"))}
	got := Search(convs, "café visits")
	if len(got) != 1 {
		t.Fatalf("matches = %d", len(got))
	}
	if !strings.Contains(got[0].Excerpt, "café visits") {
		t.Fatalf("excerpt = %q", got[0].Excerpt)
	}
	if strings.Contains(got[0].Excerpt, "\uFFFD") {
		t.Fatalf("excerpt split a rune: %q", got[0].Excerpt)
	}
}

func TestExportMarkdownAttributesEachTurnAndCarriesSources(t *testing.T) {
	c := conv("a", "Car insurance", time.Date(2026, time.August, 16, 9, 30, 0, 0, time.UTC),
		msg("m1", "user", "is my premium high?"),
		domain.ChatMessage{ID: "m2", Role: "assistant", Text: "It's $312 a year.", Sources: []domain.ChatSource{
			{Tool: "list_transactions", Label: "Transactions", Scope: "insurance", Evidence: "3 rows\ntotal $312"},
		}},
	)
	out := ExportMarkdown(c)
	if !strings.HasPrefix(out, "# Car insurance\n") {
		t.Fatalf("export does not lead with the title:\n%s", out)
	}
	for _, want := range []string{"## You", "## Assistant", "is my premium high?", "It's $312 a year.",
		"> **Source:** Transactions · insurance", "> total $312", "16 August 2026"} {
		if !strings.Contains(out, want) {
			t.Errorf("export missing %q:\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n\n") {
		t.Fatalf("export does not end with exactly one trailing newline: %q", out[len(out)-5:])
	}
}

func TestExportOfAnUntitledConversationStillHasAHeading(t *testing.T) {
	out := ExportMarkdown(conv("a", "  ", time.Time{}, msg("m", "user", "hi")))
	if !strings.HasPrefix(out, "# Conversation\n") {
		t.Fatalf("export = %q", out)
	}
}

func TestExportFilenameIsSafeAndDated(t *testing.T) {
	at := time.Date(2026, time.August, 16, 0, 0, 0, 0, time.UTC)
	got := ExportFilename(conv("a", "Can we afford the baby?!", at), at)
	if got != "cashflux-can-we-afford-the-baby-2026-08-16.md" {
		t.Fatalf("filename = %q", got)
	}
	if strings.ContainsAny(got, `/\:*?"<>|`) {
		t.Fatalf("filename contains a character no filesystem wants: %q", got)
	}
}

func TestExportFilenameFallsBackWhenTheTitleHasNothingUsable(t *testing.T) {
	at := time.Date(2026, time.August, 16, 0, 0, 0, 0, time.UTC)
	got := ExportFilename(conv("a", "?!!—", at), at)
	if got != "cashflux-conversation-2026-08-16.md" {
		t.Fatalf("filename = %q", got)
	}
}

func TestExportFilenameIsBounded(t *testing.T) {
	at := time.Date(2026, time.August, 16, 0, 0, 0, 0, time.UTC)
	got := ExportFilename(conv("a", strings.Repeat("word ", 60), at), at)
	if len(got) > 80 {
		t.Fatalf("filename is %d chars: %q", len(got), got)
	}
	if strings.Contains(got, "--") {
		t.Fatalf("filename has a doubled hyphen from the cut: %q", got)
	}
}

func TestCleanTitleTrimsCollapsesAndCaps(t *testing.T) {
	got, ok := CleanTitle("  Car   insurance\n renewal  ")
	if !ok || got != "Car insurance renewal" {
		t.Fatalf("CleanTitle = %q/%v", got, ok)
	}
	long, ok := CleanTitle(strings.Repeat("a", 200))
	if !ok || len([]rune(long)) != 80 {
		t.Fatalf("long title = %d runes", len([]rune(long)))
	}
}

func TestClearingATitleIsAllowedAndReported(t *testing.T) {
	// ok=false means "derive the name again", not "reject the edit" — a chat
	// should never end up called nothing because someone emptied the box.
	if _, ok := CleanTitle("   "); ok {
		t.Fatal("an empty rename reported as a usable title")
	}
}

func TestTruncateForResendDropsWhatCameAfter(t *testing.T) {
	msgs := []domain.ChatMessage{
		msg("m1", "user", "what did we spend on food?"),
		domain.ChatMessage{ID: "m2", Role: "assistant", Text: "$400.", Tokens: 90},
		msg("m3", "user", "and on fuel?"),
		domain.ChatMessage{ID: "m4", Role: "assistant", Text: "$120."},
	}
	out, ok := TruncateForResend(msgs, "m1", "what did we spend on groceries?")
	if !ok {
		t.Fatal("TruncateForResend refused a valid edit")
	}
	if len(out) != 1 {
		t.Fatalf("kept %d messages, want just the edited one", len(out))
	}
	if out[0].Text != "what did we spend on groceries?" || out[0].ID != "m1" {
		t.Fatalf("edited message = %+v", out[0])
	}
}

func TestTruncateForResendClearsTheOldTurnsAccounting(t *testing.T) {
	msgs := []domain.ChatMessage{{
		ID: "m1", Role: "user", Text: "old", Tokens: 50, PromptTokens: 40, CompletionTokens: 10,
		Sources: []domain.ChatSource{{Tool: "list_transactions"}},
	}}
	out, ok := TruncateForResend(msgs, "m1", "new")
	if !ok {
		t.Fatal("refused")
	}
	if out[0].Tokens != 0 || out[0].PromptTokens != 0 || out[0].CompletionTokens != 0 {
		t.Fatalf("old token accounting survived the edit: %+v", out[0])
	}
	if out[0].Sources != nil {
		t.Fatalf("old sources survived an edit that changed the question: %+v", out[0].Sources)
	}
}

func TestTruncateForResendRefusesWhatItCannotDo(t *testing.T) {
	msgs := []domain.ChatMessage{
		msg("m1", "user", "q"),
		msg("m2", "assistant", "a"),
	}
	if _, ok := TruncateForResend(msgs, "m2", "edited"); ok {
		t.Fatal("allowed editing the assistant's own answer")
	}
	if _, ok := TruncateForResend(msgs, "nope", "edited"); ok {
		t.Fatal("allowed editing a message that isn't there")
	}
	if _, ok := TruncateForResend(msgs, "m1", "   "); ok {
		t.Fatal("allowed resending an empty question")
	}
}

func TestFeedbackTogglesOffWhenClickedTwice(t *testing.T) {
	if got := ToggleFeedback(FeedbackNone, FeedbackUp); got != FeedbackUp {
		t.Fatalf("first click = %q", got)
	}
	if got := ToggleFeedback(FeedbackUp, FeedbackUp); got != FeedbackNone {
		t.Fatalf("clicking the same verdict again should clear it, got %q", got)
	}
	if got := ToggleFeedback(FeedbackUp, FeedbackDown); got != FeedbackDown {
		t.Fatalf("switching verdict = %q", got)
	}
}

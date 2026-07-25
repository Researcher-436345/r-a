package llm

import (
	"strings"
	"testing"

	"assistant/internal/domain"
)

func TestBuildPrompt(t *testing.T) {
	article := domain.Article{
		ID:      "a1",
		Title:   "On Flow Policies",
		Authors: "Zhou et al.",
		Content: "Full article body about QGF.",
	}
	history := []domain.Message{
		{Role: domain.RoleUser, Content: "earlier question"},
		{Role: domain.RoleAssistant, Content: "earlier answer"},
		{Role: domain.RoleSystem, Content: "should be skipped"},
	}

	got := BuildPrompt(article, history, "What is QGF?")

	// single system message, 2 history turns (system skipped), question
	if len(got) != 4 {
		t.Fatalf("expected 4 messages, got %d: %+v", len(got), got)
	}

	// message 0: one system message combining instructions + the article.
	if got[0].Role != domain.RoleSystem {
		t.Errorf("message 0 should be a system message, got %+v", got[0])
	}
	if !strings.Contains(got[0].Content, SystemPrompt) {
		t.Errorf("system message should contain the instructions, got %q", got[0].Content)
	}
	if !strings.Contains(got[0].Content, article.Content) ||
		!strings.Contains(got[0].Content, article.Title) ||
		!strings.Contains(got[0].Content, article.Authors) {
		t.Errorf("system message should include article title/authors/content, got %q", got[0].Content)
	}
	if got[1].Role != domain.RoleUser || got[1].Content != "earlier question" {
		t.Errorf("message 1 should be the first history turn, got %+v", got[1])
	}
	if got[2].Role != domain.RoleAssistant || got[2].Content != "earlier answer" {
		t.Errorf("message 2 should be the assistant history turn, got %+v", got[2])
	}

	last := got[len(got)-1]
	if last.Role != domain.RoleUser || last.Content != "What is QGF?" {
		t.Errorf("last message should be the new question, got %+v", last)
	}
}

func TestBuildPromptNoHistory(t *testing.T) {
	got := BuildPrompt(domain.Article{Content: "body"}, nil, "q")
	if len(got) != 2 {
		t.Fatalf("expected 2 messages with no history, got %d", len(got))
	}
	if got[0].Role != domain.RoleSystem {
		t.Errorf("expected system message first, got %+v", got[0])
	}
	if got[1].Content != "q" {
		t.Errorf("expected question last, got %q", got[1].Content)
	}
}

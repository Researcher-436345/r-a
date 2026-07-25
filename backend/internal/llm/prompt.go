package llm

import (
	"fmt"

	"assistant/internal/domain"
)

// SystemPrompt instructs the model to act as an assistant grounded in the
// provided article.
const SystemPrompt = `You are a research assistant that answers questions about a single scientific article.
Rules:
- Answer strictly based on the article text provided below. Do not invent facts.
- If the answer is not contained in the article, say so plainly instead of guessing.
- When helpful, refer to the relevant section of the article.
- Reply in the same language the user used in their question.
- Be concise and precise.`

// PromptMessage is a provider-agnostic chat message used to build a request.
type PromptMessage struct {
	Role    domain.Role
	Content string
}

// BuildPrompt assembles the message list sent to the LLM: a single system
// message (instructions + the full article), prior conversation history, and
// the new question.
//
// The instructions and article are combined into one system message: some
// providers/gateways (e.g. litellm/DeepInfra) require a single system message at
// the very beginning of the conversation.
func BuildPrompt(article domain.Article, history []domain.Message, question string) []PromptMessage {
	msgs := make([]PromptMessage, 0, len(history)+2)

	msgs = append(msgs, PromptMessage{
		Role:    domain.RoleSystem,
		Content: SystemPrompt + "\n\n" + articleContext(article),
	})

	for _, m := range history {
		// Only user/assistant turns belong in the conversation; skip any stored
		// system messages so we don't duplicate or override our instructions.
		if m.Role == domain.RoleUser || m.Role == domain.RoleAssistant {
			msgs = append(msgs, PromptMessage{Role: m.Role, Content: m.Content})
		}
	}

	msgs = append(msgs, PromptMessage{Role: domain.RoleUser, Content: question})
	return msgs
}

func articleContext(a domain.Article) string {
	return fmt.Sprintf("ARTICLE\nTitle: %s\nAuthors: %s\n\n%s", a.Title, a.Authors, a.Content)
}

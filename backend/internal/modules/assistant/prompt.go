package assistant

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/google/uuid"
)

const chatSystemPrompt = `You are a helpful research assistant. Answer in the same language as the user. Ground answers in the supplied full paper text and conversation. Avoid inventing details that are not supported by the paper or chat context. When helpful, format with Markdown: short headings, bullet/numbered lists, and **bold** for key terms. Prefer ## / ### over giant titles.

The paper text may include page markers of the form <<<p=N>>> (PDF page N). When you refer to a specific place in the paper, cite it inline as [p.N «short quote»] using only page numbers that appear in those markers. The quote must be a short verbatim snippet (3–12 words) from that page. Plain [p.N] is allowed when a quote does not help. Do not invent page numbers or quotes. Prefer a few precise cites over many.`

func EstimateTokens(s string) int {
	n := utf8.RuneCountInString(s)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4
}

func truncateToTokens(s string, maxTokens int) string {
	if maxTokens < 1 || EstimateTokens(s) <= maxTokens {
		return s
	}
	// approx chars
	maxChars := maxTokens * 4
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	head := maxChars * 70 / 100
	tail := maxChars - head - 80
	if tail < 0 {
		tail = 0
	}
	msg := "\n\n[... paper truncated to fit model context ...]\n\n"
	return string(runes[:head]) + msg + string(runes[len(runes)-tail:])
}

type PromptBundle struct {
	System       string
	PaperText    string
	ThreadSummary string
	History      []ChatTurn
	Question     string
	ContextText  string
}

func (l LLM) ChatWithPaper(
	ctx context.Context,
	p catalog.PaperOut,
	fullPaper string,
	threadSummary string,
	history []ChatTurn,
	message, contextText string,
) (ChatResult, error) {
	return l.ChatWithPaperStream(ctx, p, fullPaper, threadSummary, history, message, contextText, nil)
}

func (l LLM) ChatWithPaperStream(
	ctx context.Context,
	p catalog.PaperOut,
	fullPaper string,
	threadSummary string,
	history []ChatTurn,
	message, contextText string,
	onDelta func(string) error,
) (ChatResult, error) {
	built, err := l.buildChatPrompt(ctx, p, fullPaper, threadSummary, history, message, contextText)
	if err != nil {
		return ChatResult{Usage: built.Usage}, err
	}
	out, err := l.requestStream(ctx, built.System, built.Turns, onDelta)
	return ChatResult{Reply: out, Summary: built.Summary, Usage: built.Usage}, err
}

type builtPrompt struct {
	System  string
	Turns   []ChatTurn
	Summary string
	Usage   ContextUsage
}

func (l LLM) buildChatPrompt(
	ctx context.Context,
	p catalog.PaperOut,
	fullPaper string,
	threadSummary string,
	history []ChatTurn,
	message, contextText string,
) (builtPrompt, error) {
	limit := l.Config.LLMContextTokens
	if limit < 4000 {
		limit = 120000
	}
	budget := limit - l.Config.LLMReplyReserve
	if budget < 2000 {
		budget = 2000
	}

	system := chatSystemPrompt
	meta := paperContext(p)

	remaining := budget - EstimateTokens(system) - EstimateTokens(meta) - EstimateTokens(message) - EstimateTokens(contextText) - 200
	if remaining < 500 {
		remaining = 500
	}

	paperBudget := remaining * 75 / 100
	historyBudget := remaining - paperBudget
	if historyBudget < 400 {
		historyBudget = 400
		paperBudget = remaining - historyBudget
	}

	rawPaper := strings.TrimSpace(fullPaper)
	paper := rawPaper
	if paper != "" {
		paper = truncateToTokens(paper, paperBudget)
	}

	keep := l.Config.ChatRecentKeep
	if keep < 2 {
		keep = 2
	}
	fittedHistory, fittedSummary, err := l.fitHistory(ctx, history, threadSummary, historyBudget, keep, false)
	if err != nil {
		return builtPrompt{}, err
	}

	userParts := []string{meta}
	if paper != "" {
		userParts = append(userParts, "Full paper text (sections marked <<<p=N>>> mean PDF page N):\n"+paper)
	} else {
		userParts = append(userParts, "Full paper text is not available yet; rely on title/abstract only.")
	}
	if strings.TrimSpace(fittedSummary) != "" {
		userParts = append(userParts, "Earlier conversation summary:\n"+strings.TrimSpace(fittedSummary))
	}
	if strings.TrimSpace(contextText) != "" {
		userParts = append(userParts, "Highlighted passages:\n"+strings.TrimSpace(contextText))
	}
	userParts = append(userParts, "User question:\n"+strings.TrimSpace(message))

	turns := make([]ChatTurn, 0, len(fittedHistory)+1)
	turns = append(turns, fittedHistory...)
	userContent := strings.Join(userParts, "\n\n")
	turns = append(turns, ChatTurn{Role: "user", Content: userContent})

	used := EstimateTokens(system) + EstimateTokens(userContent)
	for _, t := range fittedHistory {
		used += EstimateTokens(t.Content) + 4
	}
	paperTokens := EstimateTokens(paper)
	histTokens := used - paperTokens - EstimateTokens(system) - EstimateTokens(meta) - EstimateTokens(message) - EstimateTokens(contextText)
	if histTokens < 0 {
		histTokens = 0
	}
	percent := 0.0
	if limit > 0 {
		percent = float64(used) * 100 / float64(limit)
		if percent > 100 {
			percent = 100
		}
	}

	return builtPrompt{
		System:  system,
		Turns:   turns,
		Summary: fittedSummary,
		Usage: ContextUsage{
			UsedTokens:    used,
			LimitTokens:   limit,
			Percent:       percent,
			PaperTokens:   paperTokens,
			HistoryTokens: histTokens,
			HasFullPaper:  rawPaper != "",
			Model:         l.resolvedModel(),
		},
	}, nil
}

// EstimateChatContext estimates prompt fill without calling the LLM (dry history fit).
func (l LLM) EstimateChatContext(
	p catalog.PaperOut,
	fullPaper string,
	threadSummary string,
	history []ChatTurn,
) ContextUsage {
	limit := l.Config.LLMContextTokens
	if limit < 4000 {
		limit = 120000
	}
	budget := limit - l.Config.LLMReplyReserve
	if budget < 2000 {
		budget = 2000
	}
	system := chatSystemPrompt
	meta := paperContext(p)
	remaining := budget - EstimateTokens(system) - EstimateTokens(meta) - 200
	if remaining < 500 {
		remaining = 500
	}
	paperBudget := remaining * 75 / 100
	historyBudget := remaining - paperBudget
	if historyBudget < 400 {
		historyBudget = 400
		paperBudget = remaining - historyBudget
	}
	rawPaper := strings.TrimSpace(fullPaper)
	paper := rawPaper
	if paper != "" {
		paper = truncateToTokens(paper, paperBudget)
	}
	keep := l.Config.ChatRecentKeep
	if keep < 2 {
		keep = 2
	}
	fittedHistory, fittedSummary, _ := l.fitHistory(context.Background(), history, threadSummary, historyBudget, keep, true)

	userParts := []string{meta}
	if paper != "" {
		userParts = append(userParts, "Full paper text (sections marked <<<p=N>>> mean PDF page N):\n"+paper)
	} else {
		userParts = append(userParts, "Full paper text is not available yet; rely on title/abstract only.")
	}
	if strings.TrimSpace(fittedSummary) != "" {
		userParts = append(userParts, "Earlier conversation summary:\n"+strings.TrimSpace(fittedSummary))
	}
	userContent := strings.Join(userParts, "\n\n")
	used := EstimateTokens(system) + EstimateTokens(userContent)
	for _, t := range fittedHistory {
		used += EstimateTokens(t.Content) + 4
	}
	paperTokens := EstimateTokens(paper)
	histTokens := used - paperTokens - EstimateTokens(system) - EstimateTokens(meta)
	if histTokens < 0 {
		histTokens = 0
	}
	percent := 0.0
	if limit > 0 {
		percent = float64(used) * 100 / float64(limit)
		if percent > 100 {
			percent = 100
		}
	}
	return ContextUsage{
		UsedTokens:    used,
		LimitTokens:   limit,
		Percent:       percent,
		PaperTokens:   paperTokens,
		HistoryTokens: histTokens,
		HasFullPaper:  rawPaper != "",
		Model:         l.resolvedModel(),
	}
}

func (l LLM) fitHistory(
	ctx context.Context,
	history []ChatTurn,
	existingSummary string,
	budget int,
	keepRecent int,
	dry bool,
) ([]ChatTurn, string, error) {
	if len(history) == 0 {
		return nil, strings.TrimSpace(existingSummary), nil
	}

	sum := strings.TrimSpace(existingSummary)
	total := EstimateTokens(sum)
	for _, h := range history {
		total += EstimateTokens(h.Content) + 4
	}
	if total <= budget {
		return history, sum, nil
	}

	if keepRecent > len(history) {
		keepRecent = len(history)
	}
	recent := history[len(history)-keepRecent:]
	older := history[:len(history)-keepRecent]

	recentTokens := 0
	for _, h := range recent {
		recentTokens += EstimateTokens(h.Content) + 4
	}
	summaryBudget := budget - recentTokens
	if summaryBudget < 200 {
		for len(recent) > 2 && recentTokens > budget-200 {
			recent = recent[1:]
			recentTokens = 0
			for _, h := range recent {
				recentTokens += EstimateTokens(h.Content) + 4
			}
		}
		summaryBudget = budget - recentTokens
	}

	if dry {
		if sum != "" {
			return recent, truncateToTokens(sum, summaryBudget), nil
		}
		return recent, "", nil
	}

	toSummarize := older
	if sum != "" {
		toSummarize = append([]ChatTurn{{Role: "user", Content: "Previous summary:\n" + sum}}, older...)
	}
	if len(toSummarize) == 0 {
		return recent, sum, nil
	}

	newSummary, err := l.SummarizeHistory(ctx, toSummarize, summaryBudget)
	if err != nil {
		return recent, sum, nil
	}
	return recent, newSummary, nil
}

func (l LLM) SummarizeHistory(ctx context.Context, turns []ChatTurn, maxTokens int) (string, error) {
	if len(turns) == 0 {
		return "", nil
	}
	var b strings.Builder
	for _, t := range turns {
		role := t.Role
		if role == "" {
			role = "user"
		}
		b.WriteString(strings.ToUpper(role))
		b.WriteString(": ")
		b.WriteString(t.Content)
		b.WriteString("\n\n")
	}
	system := "Summarize the conversation for a research paper assistant. Preserve user goals, constraints, decisions, key facts about the paper discussed, and open questions. Be concise. Write in the same language as the conversation."
	prompt := fmt.Sprintf("Max length about %d tokens.\n\nConversation:\n%s", maxTokens, truncateToTokens(b.String(), maxTokens*3))
	return l.request(ctx, system, []ChatTurn{{Role: "user", Content: prompt}})
}

// LastCoveredID returns the id of the last message that was folded into summary (everything before recent).
func LastCoveredID(all []Message, recentKeep int) *uuid.UUID {
	if len(all) <= recentKeep {
		return nil
	}
	id := all[len(all)-recentKeep-1].ID
	return &id
}

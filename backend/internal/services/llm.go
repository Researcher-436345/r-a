package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/centraluniversity/researcher/internal/config"
	"github.com/centraluniversity/researcher/internal/models"
)

type LLM struct {
	Config config.Config
	HTTP   *http.Client
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type searchResult struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

const researchSystemPrompt = `You are an expert research assistant with access to live web search. Help the user discover and investigate scientific papers, research directions, methods, and competing approaches.

Research requirements:
- Determine the user's actual research question, important terminology, and relevant subtopics before answering.
- Search broadly enough to identify the main approaches, then verify important claims against primary sources.
- Prefer peer-reviewed papers, original preprints, publisher pages, official project pages, datasets, benchmarks, and documentation. Use secondary sources only when they add useful context.
- Prioritize recent work when the user asks about new, current, recent, or state-of-the-art research. State publication dates and clearly distinguish peer-reviewed work from preprints.
- Cross-check important or potentially controversial claims. Describe disagreements, limitations, negative results, and uncertainty instead of presenting weak evidence as settled fact.
- Never invent papers, authors, dates, metrics, quotations, DOI/arXiv identifiers, or URLs. If evidence is insufficient, say so explicitly.
- Include concrete scientific papers and research works that are directly relevant to the user's topic, rather than discussing the field only in general terms.
- For each recommended work, provide a concise overview: the title with a direct link to the original paper, authors and publication year when available, the central idea, the method or approach used, the main reported results, and why the work is relevant to the user's question. Clearly note important limitations and whether the work is peer-reviewed or a preprint.
- Answer in the same language as the user's latest message.
- Do not expose private chain-of-thought or hidden reasoning. Present only concise conclusions about the search process when useful.

Output requirements:
- Return only a polished Markdown answer.
- Use a clear hierarchy of headings (##, ###), short paragraphs, bullet or numbered lists, **bold emphasis**, blockquotes, and Markdown tables when they improve comprehension.
- Attach sources as clickable Markdown links with descriptive labels: [paper or source title](https://...).
- Cite factual, recent, and quantitative claims close to the relevant sentence. Do not output unverifiable bare URLs.
- Finish with a "Sources" section containing the most important unique links, unless the API-provided citations already produce an equivalent linked source list.
- Give a substantive synthesis, not merely a list of search results. Explain how approaches differ, when each is useful, and what the evidence does and does not establish.`

func (l LLM) Chat(ctx context.Context, p models.PaperOut, message, contextText string) string {
	return l.request(ctx, "You are a helpful research assistant. Answer in the same language as the user. Ground answers in supplied metadata and avoid inventing details.", paperContext(p)+"\n\nHighlighted passages:\n"+or(contextText, "None provided.")+"\n\nUser question:\n"+message)
}

func (l LLM) ResearchStream(ctx context.Context, history []LLMMessage, mode string, onDelta func(string) error) (string, error) {
	if l.Config.LLMAPIKey == "" {
		return "", fmt.Errorf("web search LLM is not configured: add LLM_API_KEY to .env")
	}

	system := researchSystemPrompt
	if mode == "deep" {
		system += "\n\nDeep research mode: perform a more exhaustive multi-step search, compare multiple independent sources, organize the answer as a detailed research report, and include concrete next-reading recommendations."
	} else {
		system += "\n\nWeb search mode: answer efficiently while still checking the most important claims and providing direct source links."
	}
	messages := make([]LLMMessage, 0, len(history)+1)
	messages = append(messages, LLMMessage{Role: "system", Content: system})
	messages = append(messages, history...)

	body, err := json.Marshal(map[string]any{
		"model":    l.Config.LLMModelWebSearch,
		"messages": messages,
		"stream":   true,
	})
	if err != nil {
		return "", fmt.Errorf("encode web search request: %w", err)
	}

	endpoint := strings.TrimRight(l.Config.LLMBaseURLWebSearch, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create web search request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+l.Config.LLMAPIKey)
	req.Header.Set("Content-Type", "application/json")
	if strings.Contains(l.Config.LLMBaseURLWebSearch, "openrouter.ai") {
		req.Header.Set("HTTP-Referer", l.Config.LLMHTTPReferer)
		req.Header.Set("X-Title", l.Config.LLMAppTitle)
	}

	client := l.HTTP
	if client == nil {
		client = &http.Client{Timeout: l.Config.LLMWebSearchTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("web search LLM request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		message := strings.TrimSpace(string(detail))
		if message == "" {
			message = resp.Status
		}
		return "", fmt.Errorf("web search LLM returned %s: %s", resp.Status, message)
	}

	var answer strings.Builder
	var citations []string
	var results []searchResult
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		if line == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Citations     []string       `json:"citations"`
			SearchResults []searchResult `json:"search_results"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		citations = appendUnique(citations, chunk.Citations...)
		for _, result := range chunk.SearchResults {
			if result.URL != "" && !hasSearchResult(results, result.URL) {
				results = append(results, result)
			}
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			full := chunk.Choices[0].Message.Content
			current := answer.String()
			if strings.HasPrefix(full, current) {
				delta = strings.TrimPrefix(full, current)
			}
		}
		if delta == "" {
			continue
		}
		answer.WriteString(delta)
		if err := onDelta(delta); err != nil {
			return answer.String(), err
		}
	}
	if err := scanner.Err(); err != nil {
		return answer.String(), fmt.Errorf("read web search stream: %w", err)
	}

	sources := ""
	if !hasLinkedSourcesSection(answer.String()) {
		sources = markdownSources(citations, results)
	}
	if sources != "" {
		answer.WriteString(sources)
		if err := onDelta(sources); err != nil {
			return answer.String(), err
		}
	}
	if strings.TrimSpace(answer.String()) == "" {
		return "", fmt.Errorf("web search LLM returned an empty response")
	}
	return answer.String(), nil
}

func hasLinkedSourcesSection(answer string) bool {
	lower := strings.ToLower(answer)
	hasHeading := strings.Contains(lower, "\n## sources") ||
		strings.Contains(lower, "\n## источники") ||
		strings.Contains(lower, "\n## источники и литература")
	return hasHeading && strings.Contains(answer, "](")
}

func appendUnique(target []string, values ...string) []string {
	for _, value := range values {
		if value == "" {
			continue
		}
		found := false
		for _, current := range target {
			if current == value {
				found = true
				break
			}
		}
		if !found {
			target = append(target, value)
		}
	}
	return target
}

func hasSearchResult(results []searchResult, targetURL string) bool {
	for _, result := range results {
		if result.URL == targetURL {
			return true
		}
	}
	return false
}

func markdownSources(citations []string, results []searchResult) string {
	urls := appendUnique(nil, citations...)
	for _, result := range results {
		urls = appendUnique(urls, result.URL)
	}
	if len(urls) == 0 {
		return ""
	}

	titles := make(map[string]string, len(results))
	for _, result := range results {
		titles[result.URL] = result.Title
	}
	var output strings.Builder
	output.WriteString("\n\n## Sources\n\n")
	for index, sourceURL := range urls {
		title := strings.TrimSpace(titles[sourceURL])
		if title == "" {
			if parsed, err := url.Parse(sourceURL); err == nil {
				title = parsed.Hostname()
			}
		}
		if title == "" {
			title = "Source"
		}
		title = strings.NewReplacer("[", "", "]", "").Replace(title)
		fmt.Fprintf(&output, "%d. [%s](%s)\n", index+1, title, sourceURL)
	}
	return output.String()
}
func (l LLM) Explain(ctx context.Context, p models.PaperOut, text, question string) string {
	if question == "" {
		question = "Explain this fragment simply and in context."
	}
	return l.request(ctx, "You are a helpful research assistant. Explain selected paper fragments clearly, briefly, and accurately. Answer in the same language as the user.", paperContext(p)+"\n\nFragment:\n"+text+"\n\nQuestion:\n"+question)
}
func (l LLM) request(ctx context.Context, system, user string) string {
	if l.Config.LLMProvider == "gemini" {
		return l.gemini(ctx, system, user)
	}
	return l.openAI(ctx, system, user)
}
func (l LLM) openAI(ctx context.Context, system, user string) string {
	if l.Config.LLMAPIKey == "" {
		return "LLM не настроен. Добавь `LLM_API_KEY` в `.env`. Из РФ: зарегистрируйся на aitunnel.ru, пополни баланс и вставь ключ AITunnel."
	}
	body, _ := json.Marshal(map[string]any{"model": l.Config.LLMModel, "messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}}, "temperature": 0.2})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(l.Config.LLMBaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+l.Config.LLMAPIKey)
	req.Header.Set("Content-Type", "application/json")
	if strings.Contains(l.Config.LLMBaseURL, "openrouter.ai") {
		req.Header.Set("HTTP-Referer", l.Config.LLMHTTPReferer)
		req.Header.Set("X-Title", l.Config.LLMAppTitle)
	}
	client := l.HTTP
	if client == nil {
		client = &http.Client{Timeout: l.Config.LLMTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "Ошибка LLM API: " + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "Ошибка LLM API: " + resp.Status
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil || len(out.Choices) == 0 {
		return "LLM API вернул пустой ответ."
	}
	if s := strings.TrimSpace(out.Choices[0].Message.Content); s != "" {
		return s
	}
	return "LLM API вернул ответ без текста."
}
func (l LLM) gemini(ctx context.Context, system, user string) string {
	if l.Config.LLMAPIKey == "" {
		return "Gemini не настроен. Добавь `LLM_API_KEY` из Google AI Studio в `.env`."
	}
	body, _ := json.Marshal(map[string]any{"system_instruction": map[string]any{"parts": []map[string]string{{"text": system}}}, "contents": []map[string]any{{"role": "user", "parts": []map[string]string{{"text": user}}}}, "generationConfig": map[string]float64{"temperature": 0.2}})
	u := fmt.Sprintf("%s/models/%s:generateContent?key=%s", strings.TrimRight(l.Config.LLMBaseURL, "/"), l.Config.LLMModel, l.Config.LLMAPIKey)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	client := l.HTTP
	if client == nil {
		client = &http.Client{Timeout: l.Config.LLMTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "Ошибка Gemini API: " + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "Ошибка Gemini API: " + resp.Status
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil || len(out.Candidates) == 0 {
		return "Gemini вернул пустой ответ."
	}
	var parts []string
	for _, p := range out.Candidates[0].Content.Parts {
		parts = append(parts, p.Text)
	}
	if s := strings.TrimSpace(strings.Join(parts, "")); s != "" {
		return s
	}
	return "Gemini вернул ответ без текста."
}
func paperContext(p models.PaperOut) string {
	names := make([]string, 0, len(p.Authors))
	for _, a := range p.Authors {
		names = append(names, a.Name)
	}
	year := "unknown"
	if p.Year != nil {
		year = fmt.Sprint(*p.Year)
	}
	abstract := "No abstract available."
	if p.Abstract != nil && strings.TrimSpace(*p.Abstract) != "" {
		abstract = *p.Abstract
	}
	return fmt.Sprintf("Title: %s\nAuthors: %s\nYear: %s\nAbstract:\n%s", p.Title, or(strings.Join(names, ", "), "unknown"), year, abstract)
}
func or(s, f string) string {
	if strings.TrimSpace(s) == "" {
		return f
	}
	return s
}

package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/centraluniversity/researcher/internal/config"
	"github.com/centraluniversity/researcher/internal/models"
)

type LLM struct {
	Config config.Config
	HTTP   *http.Client
}

func (l LLM) Chat(ctx context.Context, p models.PaperOut, message, contextText string) string {
	return l.request(ctx, "You are a helpful research assistant. Answer in the same language as the user. Ground answers in supplied metadata and avoid inventing details.", paperContext(p)+"\n\nHighlighted passages:\n"+or(contextText, "None provided.")+"\n\nUser question:\n"+message)
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

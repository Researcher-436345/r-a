package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/centraluniversity/researcher/internal/models"
)

func Translate(ctx context.Context, llm LLM, paper models.PaperOut, text, target string) (string, *string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if llm.Config.LLMAPIKey != "" {
		reply := llm.Explain(ctx, paper, text, "Translate the fragment into "+target+". Return only the translation, no quotes or commentary.")
		l := strings.ToLower(reply)
		if reply != "" && !strings.HasPrefix(l, "ошибка") && !strings.Contains(l, "не настроен") {
			return reply, nil
		}
	}
	u := "https://api.mymemory.translated.net/get?q=" + url.QueryEscape(truncate(text, 450)) + "&langpair=" + url.QueryEscape("autodetect|"+target)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "Не удалось перевести: " + err.Error(), nil
	}
	defer resp.Body.Close()
	var data struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
		Matches []struct {
			Source string `json:"source"`
		} `json:"matches"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil || strings.TrimSpace(data.ResponseData.TranslatedText) == "" {
		return "Не удалось перевести: empty translation from MyMemory", nil
	}
	var source *string
	if len(data.Matches) > 0 && data.Matches[0].Source != "" {
		source = &data.Matches[0].Source
	}
	return strings.TrimSpace(data.ResponseData.TranslatedText), source
}
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

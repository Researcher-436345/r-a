package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/centraluniversity/researcher/internal/platform/config"
)

func testSummaryConfig(contextTokens, replyReserve int) config.Config {
	return config.Config{LLMContextTokens: contextTokens, LLMReplyReserve: replyReserve}
}

func testPaper() catalog.PaperOut {
	abstract := "We study test-time guidance of flow policies."
	return catalog.PaperOut{Title: "Guided Flow Policies", Abstract: &abstract}
}

func TestNormalizeSummaryLang(t *testing.T) {
	cases := map[string]string{
		"":        "ru",
		"ru":      "ru",
		"RU":      "ru",
		"ru-RU":   "ru",
		"en":      "en",
		"EN-us":   "en",
		"en_US":   "en",
		"de":      "ru",
		"  en  ":  "en",
		"english": "ru",
	}
	for input, want := range cases {
		if got := NormalizeSummaryLang(input); got != want {
			t.Errorf("NormalizeSummaryLang(%q) = %q, want %q", input, got, want)
		}
	}
}

// The skeleton is machine-parsed by the client: the headings stay English in
// both languages and must appear in the same order in the system prompt and in
// the user turn.
func TestBuildSummaryPromptKeepsSectionOrder(t *testing.T) {
	for _, lang := range []string{"ru", "en"} {
		system, turns := BuildSummaryPrompt(testPaper(), "<<<p=1>>>\nbody", lang, 1000)
		if len(turns) != 1 {
			t.Fatalf("lang %s: expected a single user turn, got %d", lang, len(turns))
		}
		for name, content := range map[string]string{"system": system, "user": turns[0].Content} {
			cursor := 0
			for _, section := range summaryHeadings {
				heading := "## " + section
				idx := strings.Index(content[cursor:], heading)
				if idx < 0 {
					t.Fatalf("lang %s: heading %q missing or out of order in %s prompt", lang, heading, name)
				}
				cursor += idx + len(heading)
			}
		}
	}
}

func TestSummarySkeletonIsAlphaXivCardPlusDeepDive(t *testing.T) {
	want := []string{"TL;DR", "Problem", "Method", "Results", "Takeaways", "Limitations", "Deep dive"}
	got := SummaryHeadings()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("SummaryHeadings() = %v, want %v", got, want)
	}
}

func TestBuildSummaryPromptEmbedsPaperText(t *testing.T) {
	system, turns := BuildSummaryPrompt(testPaper(), "<<<p=7>>>\nthe body text", "ru", 1000)
	if !strings.Contains(system, "<<<p=N>>>") {
		t.Error("system prompt must explain page markers")
	}
	content := turns[0].Content
	if !strings.Contains(content, "<<<p=7>>>") || !strings.Contains(content, "the body text") {
		t.Error("paper text must be embedded in the user turn")
	}
	if !strings.Contains(content, "Guided Flow Policies") {
		t.Error("paper metadata must be embedded in the user turn")
	}
	if !strings.Contains(content, "Russian") {
		t.Error("ru prompt must request a Russian answer")
	}
	if !strings.Contains(content, "# <title in Russian>") {
		t.Error("ru prompt must ask for a translated title line")
	}
}

func TestHasPageStructure(t *testing.T) {
	cases := map[string]bool{
		"":                                  false,
		"plain text without markers":        false,
		"<<<p=1>>>\nonly one page":          false,
		"<<<p=1>>>\na\n\n<<<p=1>>>\nb":      false,
		"<<<p=1>>>\na\n\n<<<p=2>>>\nb":      true,
		"<<<p=3>>>\na\n\n<<<p=17>>>\nb\n\n": true,
	}
	for input, want := range cases {
		if got := HasPageStructure(input); got != want {
			t.Errorf("HasPageStructure(%q) = %v, want %v", input, got, want)
		}
	}
}

// Текст из TeX кладёт все чанки на страницу 1 — модель в этом случае
// придумывает номера страниц, поэтому цитаты надо запрещать целиком.
func TestBuildSummaryPromptDisablesCitesWithoutPageSplit(t *testing.T) {
	single := "<<<p=1>>>\n" + strings.Repeat("body text ", 50)
	_, turns := BuildSummaryPrompt(testPaper(), single, "ru", 1000)
	content := turns[0].Content
	if !strings.Contains(content, "NO reliable page structure") {
		t.Error("single-page text must switch the prompt into no-citation mode")
	}
	if !strings.Contains(content, "body text") {
		t.Error("paper text must still be supplied without page citations")
	}

	multi := "<<<p=1>>>\nintro\n\n<<<p=4>>>\nresults"
	_, turns = BuildSummaryPrompt(testPaper(), multi, "ru", 1000)
	if strings.Contains(turns[0].Content, "NO reliable page structure") {
		t.Error("multi-page text must keep citations enabled")
	}
}

func TestBuildSummaryPromptWithoutFullTextDisablesCitations(t *testing.T) {
	_, turns := BuildSummaryPrompt(testPaper(), "   ", "en", 1000)
	content := turns[0].Content
	if !strings.Contains(content, "do not use page citations") {
		t.Error("abstract-only prompt must forbid page citations")
	}
	if strings.Contains(content, "<<<p=") {
		t.Error("abstract-only prompt must not promise page markers")
	}
	if !strings.Contains(content, "English") {
		t.Error("en prompt must request an English answer")
	}
}

func TestBuildSummaryPromptTruncatesToBudget(t *testing.T) {
	long := strings.Repeat("word ", 5000)
	_, turns := BuildSummaryPrompt(testPaper(), long, "ru", 100)
	content := turns[0].Content
	if !strings.Contains(content, "paper truncated to fit model context") {
		t.Error("oversized paper text must be truncated with a marker")
	}
	if EstimateTokens(content) > 1000 {
		t.Errorf("truncated prompt is still too large: %d tokens", EstimateTokens(content))
	}
}

func TestSummaryPaperBudgetLeavesRoomForReply(t *testing.T) {
	llm := LLM{Config: testSummaryConfig(8000, 2000)}
	budget := llm.SummaryPaperBudget(testPaper())
	if budget <= 0 || budget >= 6000 {
		t.Errorf("budget %d must be positive and below limit minus reply reserve", budget)
	}

	// The overview is much longer than a chat reply: a small configured reserve
	// must be raised to the summary reserve, not applied as is.
	wide := LLM{Config: testSummaryConfig(120000, 2000)}
	if got := wide.SummaryPaperBudget(testPaper()); got > 120000-summaryReplyReserve {
		t.Errorf("budget %d ignores the summary reply reserve", got)
	}
}

func TestLooksComplete(t *testing.T) {
	cases := map[string]bool{
		"":                      false,
		"   ":                   false,
		"Полное предложение.":   true,
		"Ends with a question?": true,
		"Цитата в конце»":       true,
		"ссылка [p.3]":          true,
		"| a | b |":             true,
		"$$R = f(L)$$":          true,
		"```":                   true,
		"оборвано на середине сло":     false,
		"оборвано на дефисе слабой-к-": false,
		"list item without period\n- ": false,
	}
	for input, want := range cases {
		if got := looksComplete(input); got != want {
			t.Errorf("looksComplete(%q) = %v, want %v", input, got, want)
		}
	}
}

// sseServer plays back one scripted reply per request and records the
// messages each request carried.
type sseServer struct {
	mu       sync.Mutex
	replies  []string // content per call, in order
	finishes []string // finish_reason per call
	requests [][]map[string]string
}

func (s *sseServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req struct {
		Messages  []map[string]string `json:"messages"`
		MaxTokens int                 `json:"max_tokens"`
		Stream    bool                `json:"stream"`
	}
	_ = json.Unmarshal(body, &req)
	s.mu.Lock()
	idx := len(s.requests)
	s.requests = append(s.requests, req.Messages)
	s.mu.Unlock()
	if !req.Stream || req.MaxTokens != summaryMaxTokens {
		http.Error(w, "expected streaming request with the summary token cap", http.StatusBadRequest)
		return
	}
	if idx >= len(s.replies) {
		http.Error(w, "unexpected extra request", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	for _, piece := range strings.SplitAfter(s.replies[idx], " ") {
		if piece == "" {
			continue
		}
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q},\"finish_reason\":null}]}\n\n", piece)
	}
	fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":%q}]}\n\n", s.finishes[idx])
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func newSummaryLLM(t *testing.T, srv *sseServer) LLM {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	t.Cleanup(ts.Close)
	return LLM{Config: config.Config{
		LLMProvider:      "openai_compatible",
		LLMBaseURL:       ts.URL,
		LLMAPIKey:        "test",
		LLMModel:         "test-model",
		LLMContextTokens: 20000,
		LLMReplyReserve:  2000,
	}}
}

func TestGenerateSummaryReturnsCompleteReplyAsIs(t *testing.T) {
	srv := &sseServer{replies: []string{"# T\n## TL;DR\nВсё готово."}, finishes: []string{"stop"}}
	llm := newSummaryLLM(t, srv)
	var streamed strings.Builder
	out, err := llm.GenerateSummary(context.Background(), testPaper(), "<<<p=1>>>\na\n\n<<<p=2>>>\nb", "ru", func(d string) error {
		streamed.WriteString(d)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "# T\n## TL;DR\nВсё готово." || streamed.String() != out {
		t.Fatalf("out=%q streamed=%q", out, streamed.String())
	}
	if len(srv.requests) != 1 {
		t.Fatalf("complete reply must not trigger a continuation, got %d requests", len(srv.requests))
	}
}

// Провайдер иногда отдаёт обрывок с finish_reason=stop: тогда текст уходит
// обратно как ход ассистента и модель дописывает его с места обрыва.
func TestGenerateSummaryContinuesReplyCutMidSentence(t *testing.T) {
	srv := &sseServer{
		replies:  []string{"# T\n## TL;DR\nОборвано на середи", "не предложения. Конец."},
		finishes: []string{"stop", "stop"},
	}
	llm := newSummaryLLM(t, srv)
	var streamed strings.Builder
	out, err := llm.GenerateSummary(context.Background(), testPaper(), "", "ru", func(d string) error {
		streamed.WriteString(d)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "# T\n## TL;DR\nОборвано на середине предложения. Конец."
	if out != want {
		t.Fatalf("out = %q, want %q", out, want)
	}
	if streamed.String() != want {
		t.Fatalf("client must see one continuous reply, got %q", streamed.String())
	}
	if len(srv.requests) != 2 {
		t.Fatalf("expected a continuation request, got %d requests", len(srv.requests))
	}
	second := srv.requests[1]
	if len(second) != 4 || second[2]["role"] != "assistant" || !strings.Contains(second[2]["content"], "Оборвано на середи") {
		t.Fatalf("continuation must replay the partial text as an assistant turn: %v", second)
	}
	if second[3]["role"] != "user" || second[3]["content"] != summaryContinuePrompt {
		t.Fatalf("continuation must end with the continue prompt: %v", second[3])
	}
}

// Обрыв чаще всего — исчерпанное окно контекста, а не лимит вывода: у части
// провайдеров deepseek окно 32k, статья занимает ~29k. Поэтому продолжение
// несёт вдвое меньше текста статьи, иначе оно обрывается на первых же словах.
func TestGenerateSummaryContinuationShrinksPaperText(t *testing.T) {
	srv := &sseServer{
		replies:  []string{"# T\n## TL;DR\nРаз", " два", " три."},
		finishes: []string{"stop", "stop", "stop"},
	}
	llm := newSummaryLLM(t, srv)
	long := "<<<p=1>>>\n" + strings.Repeat("word ", 20000) + "\n\n<<<p=2>>>\n" + strings.Repeat("tail ", 20000)
	out, err := llm.GenerateSummary(context.Background(), testPaper(), long, "ru", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "# T\n## TL;DR\nРаз два три." {
		t.Fatalf("out = %q", out)
	}
	if len(srv.requests) != 3 {
		t.Fatalf("expected two continuation passes, got %d requests", len(srv.requests))
	}
	first := len(srv.requests[0][1]["content"])
	second := len(srv.requests[1][1]["content"])
	third := len(srv.requests[2][1]["content"])
	if !(second < first*3/4 && third < second*3/4) {
		t.Fatalf("paper text must shrink on every resume pass: %d → %d → %d chars", first, second, third)
	}
	for _, req := range srv.requests[1:] {
		if !strings.Contains(req[1]["content"], "paper truncated to fit model context") {
			t.Fatal("resume pass must carry a truncated paper, not the full text")
		}
	}
}

func TestGenerateSummaryGivesUpAfterMaxContinuations(t *testing.T) {
	// Every pass ends mid-sentence; after the last allowed resume the partial
	// text is saved as is rather than failing the whole overview.
	replies := []string{"# T\n## TL;DR\nРаз"}
	finishes := []string{"stop"}
	want := "# T\n## TL;DR\nРаз"
	for i := 0; i < summaryContinuations; i++ {
		piece := fmt.Sprintf(" ещё%d", i+1)
		replies = append(replies, piece)
		finishes = append(finishes, "stop")
		want += piece
	}
	srv := &sseServer{replies: replies, finishes: finishes}
	llm := newSummaryLLM(t, srv)
	out, err := llm.GenerateSummary(context.Background(), testPaper(), "", "ru", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != want || len(srv.requests) != 1+summaryContinuations {
		t.Fatalf("out=%q requests=%d (want %d)", out, len(srv.requests), 1+summaryContinuations)
	}
}

func TestGenerateSummaryContinuesAfterTokenCap(t *testing.T) {
	srv := &sseServer{
		replies:  []string{"# T\n## TL;DR\nПервая часть.", " Вторая часть."},
		finishes: []string{"length", "stop"},
	}
	llm := newSummaryLLM(t, srv)
	out, err := llm.GenerateSummary(context.Background(), testPaper(), "", "ru", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "# T\n## TL;DR\nПервая часть. Вторая часть." {
		t.Fatalf("out = %q", out)
	}
}

// Reasoning-модель может потратить весь лимит на размышления и не выдать
// ни слова: продолжать нечего, это ошибка, а не пустой «готовый» обзор.
func TestGenerateSummaryRejectsEmptyCutReply(t *testing.T) {
	srv := &sseServer{replies: []string{""}, finishes: []string{"length"}}
	llm := newSummaryLLM(t, srv)
	_, err := llm.GenerateSummary(context.Background(), testPaper(), "", "ru", nil)
	if err == nil {
		t.Fatal("empty cut reply must fail")
	}
	if len(srv.requests) != 1 {
		t.Fatalf("nothing to continue, got %d requests", len(srv.requests))
	}
}

func TestSummaryModelPrefersConfiguredDefault(t *testing.T) {
	api := API{Config: config.Config{
		LLMModel:        "chat-model",
		SummaryLLMModel: "summary-model",
		LLMModels:       []config.LLMModelOption{{ID: "chat-model", Label: "Chat"}, {ID: "other", Label: "Other"}},
	}}
	if got := api.summaryModel(""); got != "summary-model" {
		t.Errorf("default must be SUMMARY_LLM_MODEL, got %q", got)
	}
	if got := api.summaryModel("other"); got != "other" {
		t.Errorf("explicit picker choice must win, got %q", got)
	}
	if got := api.summaryModel("nope"); got != "" {
		t.Errorf("unknown explicit model must be rejected, got %q", got)
	}
	api.Config.SummaryLLMModel = ""
	if got := api.summaryModel(""); got != "chat-model" {
		t.Errorf("without SUMMARY_LLM_MODEL fall back to LLM_MODEL, got %q", got)
	}
}

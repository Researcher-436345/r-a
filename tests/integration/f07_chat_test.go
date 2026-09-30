//go:build integration

package integration

import (
	"strings"
	"testing"

	"github.com/centraluniversity/researcher/tests/client"
	"github.com/centraluniversity/researcher/tests/fixtures"
	"github.com/stretchr/testify/require"
)

// F07 — чат по статье. Ист.: STATUS EPIC-08, HANDOFF §4, PARSER.md, assistant/{http,prompt,llm}.go.

// parsedPaper opens a catalog paper and waits for its full text.
func parsedPaper(t *testing.T, s *client.Session, arxivID string) client.Paper {
	t.Helper()
	return s.ParsedArxiv(arxivID)
}

// F07-01 Обычный ответ: что вернулось пользователю и что ушло в модель.
func TestF07_ChatReply(t *testing.T) {
	u := user(t, "f07-reply")
	paper := parsedPaper(t, u.Session, "2301.00001")
	question := "F07-01 what is the hypothesis " + client.Nonce()
	stand.LLM().Script(t, client.Script{Match: question, Reply: "Гипотеза на третьей странице [p.3 «deterministic fixtures make tests honest»]."})
	t.Cleanup(func() { stand.LLM().Reset(t) })

	resp := u.Chat(paper.ID, map[string]any{"message": question})
	require.Equal(t, 200, resp.Status, resp)
	body := resp.Map(t)
	require.Contains(t, body["reply"], "Гипотеза на третьей странице")
	require.NotEmpty(t, body["message_id"])
	require.NotEmpty(t, body["user_message_id"])
	usage := body["context_usage"].(map[string]any)
	require.Equal(t, true, usage["has_full_paper"])
	require.Greater(t, usage["paper_tokens"].(float64), 0.0)

	call := onlyCall(t, question)
	require.Equal(t, "test-model", call.Model)
	system := call.Text("system")
	require.Contains(t, system, "<<<p=N>>>", "the system prompt explains page markers")
	require.Contains(t, system, "[p.N", "and asks for page cites")
	prompt := call.Text("user")
	require.Contains(t, prompt, "Title: Test Paper One")
	require.Contains(t, prompt, "Full paper text")
	require.Contains(t, prompt, "HYPOTHESIS-ON-PAGE-THREE")
	require.True(t, strings.HasSuffix(strings.TrimSpace(prompt), question), "the question closes the prompt")
}

// F07-02 Стрим: дельты, потом done с теми же полями, что и обычный ответ.
func TestF07_ChatStream(t *testing.T) {
	u := user(t, "f07-stream")
	paper := parsedPaper(t, u.Session, "2301.00001")
	question := "F07-02 stream " + client.Nonce()
	reply := "Ответ по частям, чтобы проверить склейку дельт."
	stand.LLM().Script(t, client.Script{Match: question, Reply: reply, ChunkSize: 5})
	t.Cleanup(func() { stand.LLM().Reset(t) })

	events := u.ChatStream(paper.ID, map[string]any{"message": question})
	require.NotEmpty(t, events)
	deltas := 0
	for _, e := range events {
		if e.Type() == "delta" {
			deltas++
		}
	}
	require.Greater(t, deltas, 1, "the reply arrives in several deltas")
	done := client.Last(events)
	require.Equal(t, "done", done.Type(), done.Data)
	require.Equal(t, reply, done.Field("reply"))
	require.Equal(t, reply, client.JoinDeltas(events))
	require.NotEmpty(t, done.Field("message_id"))
	require.NotEmpty(t, done.Field("user_message_id"))
	require.True(t, onlyCall(t, question).Stream, "the provider is asked to stream")
}

// F07-03 История сохраняется парами и уходит провайдеру со вторым вопросом.
func TestF07_ChatHistory(t *testing.T) {
	u := user(t, "f07-history")
	paper := parsedPaper(t, u.Session, "2301.00002")
	first := "F07-03 first " + client.Nonce()
	second := "F07-03 second " + client.Nonce()
	stand.LLM().Script(t, client.Script{Match: first, Reply: "первый ответ"})
	stand.LLM().Script(t, client.Script{Match: second, Reply: "второй ответ"})
	t.Cleanup(func() { stand.LLM().Reset(t) })

	require.Equal(t, 200, u.Chat(paper.ID, map[string]any{"message": first}).Status)
	require.Equal(t, 200, u.Chat(paper.ID, map[string]any{"message": second}).Status)

	msgs, resp := u.ChatMessages(paper.ID)
	require.Equal(t, 200, resp.Status, resp)
	require.Len(t, msgs, 4)
	require.Equal(t, []string{"user", "assistant", "user", "assistant"}, []string{msgs[0].Role, msgs[1].Role, msgs[2].Role, msgs[3].Role})
	require.Equal(t, first, msgs[0].Content)
	require.Equal(t, "первый ответ", msgs[1].Content)
	require.Equal(t, second, msgs[2].Content)

	call := onlyCall(t, second)
	require.Contains(t, call.Text(""), first, "the earlier question is in the history sent to the provider")
	require.Contains(t, call.Text("assistant"), "первый ответ")
}

// F07-04 + F07-05 Ошибка провайдера: понятный статус, история не растёт.
func TestF07_ProviderErrorsAreNotSaved(t *testing.T) {
	u := user(t, "f07-errors")
	paper := parsedPaper(t, u.Session, "2301.00002")
	t.Cleanup(func() { stand.LLM().Reset(t) })

	broken := "F07-04 broken " + client.Nonce()
	stand.LLM().Script(t, client.Script{Match: broken, Status: 502, Times: 2})
	resp := u.Chat(paper.ID, map[string]any{"message": broken})
	require.Equal(t, 502, resp.Status, resp)
	require.NotEmpty(t, resp.Detail())
	events := u.ChatStream(paper.ID, map[string]any{"message": broken})
	require.Equal(t, "error", client.Last(events).Type(), client.Last(events).Data)
	require.NotEmpty(t, client.Last(events).Field("detail"))

	balance := "F07-05 balance " + client.Nonce()
	stand.LLM().Script(t, client.Script{Match: balance, Status: 402, Body: `{"error":{"message":"insufficient balance"}}`})
	resp = u.Chat(paper.ID, map[string]any{"message": balance})
	require.Equal(t, 502, resp.Status, resp)
	require.Contains(t, resp.Detail(), "балансе")
	require.Contains(t, resp.Detail(), "test-model")

	msgs, _ := u.ChatMessages(paper.ID)
	require.Empty(t, msgs, "failed exchanges leave no messages")
}

// F07-06 Чужая история не видна.
func TestF07_HistoryIsPrivate(t *testing.T) {
	a := user(t, "f07-priv-a")
	b := user(t, "f07-priv-b")
	paper := parsedPaper(t, a.Session, "2301.00003")
	question := "F07-06 " + client.Nonce()
	require.Equal(t, 200, a.Chat(paper.ID, map[string]any{"message": question}).Status)
	mine, _ := a.ChatMessages(paper.ID)
	require.Len(t, mine, 2)
	theirs, resp := b.ChatMessages(paper.ID)
	require.Equal(t, 200, resp.Status, resp)
	require.Empty(t, theirs)
}

// F07-07 Список моделей и выбор модели для ответа.
func TestF07_Models(t *testing.T) {
	u := user(t, "f07-models")
	models := u.Models()
	require.Equal(t, 200, models.Status, models)
	body := models.Map(t)
	require.Equal(t, "test-model", body["default"])
	items := body["items"].([]any)
	require.Len(t, items, 2)
	require.Equal(t, "test-model-alt", items[1].(map[string]any)["id"])

	paper := parsedPaper(t, u.Session, "2301.00001")
	question := "F07-07 alt model " + client.Nonce()
	resp := u.Chat(paper.ID, map[string]any{"message": question, "model": "test-model-alt"})
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "test-model-alt", onlyCall(t, question).Model)
	require.Equal(t, "test-model-alt", resp.Map(t)["context_usage"].(map[string]any)["model"])

	resp = u.Chat(paper.ID, map[string]any{"message": "x", "model": "gpt-imaginary"})
	require.Equal(t, 400, resp.Status, resp)
}

// F07-08 Выделенный фрагмент попадает в промпт и сохраняется с вопросом.
func TestF07_ContextText(t *testing.T) {
	u := user(t, "f07-context")
	paper := parsedPaper(t, u.Session, "2301.00001")
	question := "F07-08 explain the highlight " + client.Nonce()
	highlight := "HIGHLIGHT-" + client.Nonce()
	resp := u.Chat(paper.ID, map[string]any{"message": question, "context_text": highlight})
	require.Equal(t, 200, resp.Status, resp)
	prompt := onlyCall(t, question).Text("user")
	require.Contains(t, prompt, "Highlighted passages:\n"+highlight)
	msgs, _ := u.ChatMessages(paper.ID)
	require.Len(t, msgs, 2)
	require.NotNil(t, msgs[0].ContextText)
	require.Equal(t, highlight, *msgs[0].ContextText)
	require.Nil(t, msgs[1].ContextText)
}

// F07-09 Explain из попапа выделения.
func TestF07_Explain(t *testing.T) {
	u := user(t, "f07-explain")
	paper := publicPaper(t, u.Session, "2301.00001")
	fragment := "F07-09 fragment " + client.Nonce()
	stand.LLM().Script(t, client.Script{Match: fragment, Reply: "Объяснение фрагмента."})
	t.Cleanup(func() { stand.LLM().Reset(t) })
	resp := u.Explain(paper.ID, map[string]any{"text": fragment, "question": "Почему это важно?"})
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "Объяснение фрагмента.", resp.Map(t)["reply"])
	prompt := onlyCall(t, fragment).Text("user")
	require.Contains(t, prompt, "Fragment:\n"+fragment)
	require.Contains(t, prompt, "Question:\nПочему это важно?")
	require.Contains(t, prompt, "Title: Test Paper One")

	require.Equal(t, 400, u.Explain(paper.ID, map[string]any{"text": "  "}).Status)
	require.Equal(t, 400, u.Explain(paper.ID, map[string]any{"text": "x", "model": "gpt-imaginary"}).Status)
}

// F07-10 Мусор в теле чата.
func TestF07_ChatValidation(t *testing.T) {
	u := user(t, "f07-valid")
	paper := publicPaper(t, u.Session, "2301.00001")
	require.Equal(t, 400, u.Chat(paper.ID, map[string]any{"message": "   "}).Status)
	require.Equal(t, 400, u.Chat(paper.ID, map[string]any{"message": "x", "temperature": 1}).Status)
	require.Equal(t, 400, u.Post("/papers/"+paper.ID+"/chat", "not json").Status)
}

// F07-11 + F07-12 Статья без текста и метр контекста.
func TestF07_ContextMeter(t *testing.T) {
	u := user(t, "f07-meter")
	parsed := parsedPaper(t, u.Session, "2301.00001")
	ctx := u.ChatContext(parsed.ID, "")
	require.Equal(t, 200, ctx.Status, ctx)
	body := ctx.Map(t)
	require.Greater(t, body["used_tokens"].(float64), 0.0)
	require.EqualValues(t, 120000, body["limit_tokens"])
	require.Greater(t, body["paper_tokens"].(float64), 0.0)
	require.Equal(t, true, body["has_full_paper"])
	require.Equal(t, "test-model", body["model"])
	require.Equal(t, "test-model-alt", u.ChatContext(parsed.ID, "model=test-model-alt").Map(t)["model"])
	require.Equal(t, 400, u.ChatContext(parsed.ID, "model=gpt-imaginary").Status)

	bare, resp := u.AddDOI(client.NewDOI(client.SyntheticDOIClosed))
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, false, u.ChatContext(bare.ID, "").Map(t)["has_full_paper"])
	question := "F07-11 no text " + client.Nonce()
	resp = u.Chat(bare.ID, map[string]any{"message": question})
	require.Equal(t, 200, resp.Status, resp)
	require.Contains(t, onlyCall(t, question).Text("user"), "Full paper text is not available yet")
}

// F07-13 Чат по чужой загрузке недоступен.
func TestF07_ChatOnForeignUpload(t *testing.T) {
	owner := user(t, "f07-fu-owner")
	private, resp := owner.UploadPDF("private.pdf", fixtures.PDF("private-1p.pdf"))
	require.Equal(t, 201, resp.Status, resp)
	other := user(t, "f07-fu-other")
	require.Equal(t, 404, other.Chat(private.ID, map[string]any{"message": "hi"}).Status)
	_, resp = other.ChatMessages(private.ID)
	require.Equal(t, 404, resp.Status)
	require.Equal(t, 404, other.Explain(private.ID, map[string]any{"text": "x"}).Status)
	require.Equal(t, 404, other.ChatContext(private.ID, "").Status)
}

//go:build integration

package integration

import (
	"strings"
	"sync"
	"testing"

	"github.com/centraluniversity/researcher/tests/client"
	"github.com/centraluniversity/researcher/tests/fixtures"
	"github.com/stretchr/testify/require"
)

// F06 — перевод и заметки. Ист.: backend/README «Translation», HANDOFF §4/§5, translation/*, annotations/http.go.

// F06-01 Все заявленные языки принимаются, неизвестный — нет, по умолчанию русский.
func TestF06_TranslateLanguages(t *testing.T) {
	u := user(t, "f06-langs")
	paper := publicPaper(t, u.Session, "2301.00001")
	for _, lang := range []string{"ru", "en", "de", "fr", "es", "it", "pt", "zh", "ja", "ko"} {
		resp := u.Translate(paper.ID, "F06-01 "+client.Nonce(), lang)
		require.Equal(t, 200, resp.Status, "%s → %s", lang, resp)
		require.Equal(t, lang, resp.Map(t)["target_lang"])
		require.NotEmpty(t, resp.Map(t)["translation"])
	}
	resp := u.Post("/papers/"+paper.ID+"/translate", map[string]string{"text": "no language given"})
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "ru", resp.Map(t)["target_lang"], "default language is Russian")
	resp = u.Translate(paper.ID, "hello", "xx")
	require.Equal(t, 400, resp.Status, resp)
	require.NotEmpty(t, resp.Detail())
}

// F06-02 Пустой текст → 400, 5000 символов проходят, 5001 → 413.
func TestF06_TranslateTextLimits(t *testing.T) {
	u := user(t, "f06-limits")
	paper := publicPaper(t, u.Session, "2301.00001")
	require.Equal(t, 400, u.Translate(paper.ID, "   ", "en").Status)
	require.Equal(t, 200, u.Translate(paper.ID, strings.Repeat("ж", 5000), "en").Status)
	resp := u.Translate(paper.ID, strings.Repeat("ж", 5001), "en")
	require.Equal(t, 413, resp.Status, resp)
	require.Contains(t, resp.Detail(), "5000")
}

// F06-03 Провайдер перевода упал: обычный запрос → 502, стрим → событие error.
func TestF06_TranslateProviderDown(t *testing.T) {
	u := user(t, "f06-down")
	paper := publicPaper(t, u.Session, "2301.00002")
	text := "F06-03 provider failure " + client.Nonce()
	stand.LLM().Script(t, client.Script{Match: text, Status: 500, Times: 2})
	t.Cleanup(func() { stand.LLM().Reset(t) })

	resp := u.Translate(paper.ID, text, "en")
	require.Equal(t, 502, resp.Status, resp)
	require.NotEmpty(t, resp.Detail())

	events := u.TranslateStream(paper.ID, text, "en")
	require.NotEmpty(t, events)
	require.Equal(t, "error", client.Last(events).Type(), "the stream ends with an error event: %s", client.Last(events).Data)
	require.NotEmpty(t, client.Last(events).Field("detail"))
	for _, e := range events {
		require.NotEqual(t, "done", e.Type(), "no done after a failure")
	}
}

// F06-04 Переводчик держит не больше 8 переводов сразу; лишние получают 429 и могут повторить позже.
func TestF06_TranslatorBusy(t *testing.T) {
	u := user(t, "f06-busy")
	paper := publicPaper(t, u.Session, "2301.00002")
	marker := "F06-04 slow " + client.Nonce()
	stand.LLM().Script(t, client.Script{Match: marker, Reply: "медленно", DelayMS: 3000})
	t.Cleanup(func() { stand.LLM().Reset(t) })

	const parallel = 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	statuses := map[int]int{}
	var busyDetail string
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := u.Clone().Translate(paper.ID, marker, "ru")
			mu.Lock()
			statuses[resp.Status]++
			if resp.Status == 429 {
				busyDetail = resp.Detail()
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	require.GreaterOrEqual(t, statuses[429], 1, "at least one request above the concurrency limit is refused: %v", statuses)
	require.GreaterOrEqual(t, statuses[200], 8, "the first eight are served: %v", statuses)
	require.Equal(t, parallel, statuses[200]+statuses[429], "only 200 and 429 are acceptable: %v", statuses)
	require.Contains(t, strings.ToLower(busyDetail), "busy")

	stand.LLM().Reset(t)
	require.Equal(t, 200, u.Translate(paper.ID, "after the burst", "ru").Status)
}

// F06-05 Чужую загрузку переводить нельзя.
func TestF06_TranslateForeignUpload(t *testing.T) {
	owner := user(t, "f06-owner")
	private, resp := owner.UploadPDF("private.pdf", fixtures.PDF("private-1p.pdf"))
	require.Equal(t, 201, resp.Status, resp)
	other := user(t, "f06-other")
	require.Equal(t, 404, other.Translate(private.ID, "text", "en").Status)
}

// F06-06 + F06-07 Заметка: создать, прочитать, изменить текст, удалить.
func TestF06_AnnotationCRUD(t *testing.T) {
	u := user(t, "f06-crud")
	paper := publicPaper(t, u.Session, "2301.00001")
	body := map[string]any{
		"page": 3, "rect": map[string]float64{"x": 0.1, "y": 0.2, "w": 0.5, "h": 0.05},
		"selected_text": "HYPOTHESIS-ON-PAGE-THREE", "note": "check this claim", "color": "#22c55e",
	}
	a, resp := u.CreateAnnotation(paper.ID, body)
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, 3, a.Page)
	require.Equal(t, "HYPOTHESIS-ON-PAGE-THREE", a.SelectedText)
	require.Equal(t, "check this claim", a.Note)
	require.Equal(t, "#22c55e", a.Color)
	require.EqualValues(t, 0.5, a.Rect["w"])
	require.Nil(t, a.SourceChatMessageID)

	list, resp := u.Annotations(paper.ID)
	require.Equal(t, 200, resp.Status, resp)
	require.Len(t, list, 1)
	require.Equal(t, a.ID, list[0].ID)

	edited, resp := u.PatchAnnotation(a.ID, map[string]any{"note": "  changed  "})
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "changed", edited.Note, "note is trimmed")
	require.Equal(t, a.Color, edited.Color)
	require.Equal(t, a.SelectedText, edited.SelectedText)

	_, resp = u.PatchAnnotation(a.ID, map[string]any{"note": "x", "color": "#000000"})
	require.Equal(t, 400, resp.Status, "only the note can change: %s", resp)
	_, resp = u.PatchAnnotation(a.ID, map[string]any{"page": 1})
	require.Equal(t, 400, resp.Status, resp)

	require.Equal(t, 204, u.DeleteAnnotation(a.ID).Status)
	require.Equal(t, 404, u.DeleteAnnotation(a.ID).Status)
	require.Equal(t, 404, u.DeleteAnnotation("not-a-uuid").Status)
	list, _ = u.Annotations(paper.ID)
	require.Empty(t, list)

	defaulted := u.Note(paper.ID, 1, "MARKER-PAGE-1", "")
	require.Equal(t, "#facc15", defaulted.Color, "default highlight colour")
}

// F06-08 + F06-09 Валидация заметки.
func TestF06_AnnotationValidation(t *testing.T) {
	u := user(t, "f06-valid")
	paper := publicPaper(t, u.Session, "2301.00001")
	cases := map[string]map[string]any{
		"empty selection":       {"page": 1, "selected_text": "   ", "note": "x"},
		"negative page":         {"page": -1, "selected_text": "text", "note": "x"},
		"unknown field":         {"page": 1, "selected_text": "text", "note": "x", "tag": "y"},
		"bad chat message id":   {"page": 1, "selected_text": "text", "note": "x", "source_chat_message_id": "abc"},
		"colour longer than 16": {"page": 1, "selected_text": "text", "note": "x", "color": "#not-a-real-colour-value"}, // D-03
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, resp := u.CreateAnnotation(paper.ID, body)
			require.Equal(t, 400, resp.Status, resp)
		})
	}
}

// F06-10 Заметки видны и изменяемы только их автору.
func TestF06_AnnotationsArePrivate(t *testing.T) {
	a := user(t, "f06-priv-a")
	b := user(t, "f06-priv-b")
	paper := publicPaper(t, a.Session, "2301.00002")
	note := a.Note(paper.ID, 1, "MARKER-PAGE-1", "mine")

	list, resp := b.Annotations(paper.ID)
	require.Equal(t, 200, resp.Status, resp)
	require.Empty(t, list, "another user does not see my notes")
	_, resp = b.PatchAnnotation(note.ID, map[string]any{"note": "hijack"})
	require.Equal(t, 404, resp.Status, resp)
	require.Equal(t, 404, b.DeleteAnnotation(note.ID).Status)
	mine, _ := a.Annotations(paper.ID)
	require.Len(t, mine, 1)
	require.Equal(t, "mine", mine[0].Note)
}

// F06-11 + F06-12 Заметка из ответа чата ссылается на сообщение; чужое сообщение — D-06.
func TestF06_AnnotationLinkedToChatMessage(t *testing.T) {
	a := user(t, "f06-link-a")
	paper := publicPaper(t, a.Session, "2301.00001")
	question := "F06-11 " + client.Nonce()
	reply := a.Chat(paper.ID, map[string]any{"message": question})
	require.Equal(t, 200, reply.Status, reply)
	messageID := reply.Map(t)["message_id"].(string)

	note, resp := a.CreateAnnotation(paper.ID, map[string]any{"page": 1, "selected_text": "from the reply", "note": "", "source_chat_message_id": messageID})
	require.Equal(t, 201, resp.Status, resp)
	require.NotNil(t, note.SourceChatMessageID)
	require.Equal(t, messageID, *note.SourceChatMessageID)
	list, _ := a.Annotations(paper.ID)
	require.Equal(t, messageID, *list[0].SourceChatMessageID)

	b := user(t, "f06-link-b")
	_, resp = b.CreateAnnotation(paper.ID, map[string]any{"page": 1, "selected_text": "x", "note": "", "source_chat_message_id": messageID})
	require.Contains(t, []int{400, 404}, resp.Status, "another user's chat message is not a valid source: %s", resp)
}

// F06-13 Заметки на чужой загрузке недоступны.
func TestF06_AnnotationsOnForeignUpload(t *testing.T) {
	owner := user(t, "f06-fu-owner")
	private, resp := owner.UploadPDF("private.pdf", fixtures.PDF("private-1p.pdf"))
	require.Equal(t, 201, resp.Status, resp)
	other := user(t, "f06-fu-other")
	_, resp = other.Annotations(private.ID)
	require.Equal(t, 404, resp.Status, resp)
	_, resp = other.CreateAnnotation(private.ID, map[string]any{"page": 1, "selected_text": "x", "note": ""})
	require.Equal(t, 404, resp.Status, resp)
}

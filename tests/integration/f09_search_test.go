//go:build integration

package integration

import (
	"strings"
	"testing"

	"github.com/centraluniversity/researcher/tests/client"
	"github.com/stretchr/testify/require"
)

// F09 — поиск и deep research. Ист.: SERVICES.md, services/websearch/README.md, searchapi/http.go.

// F09-01 + F09-02 + F09-03 Первый вопрос создаёт чат; в источниках только статьи; результат открывается в ридере.
func TestF09_FirstMessageCreatesChat(t *testing.T) {
	u := user(t, "f09-first")
	t.Cleanup(func() { stand.LLM().Reset(t) })
	question := "F09-01 recent work on deterministic fixtures " + client.Nonce()
	stand.LLM().Script(t, client.Script{
		Match: question,
		Reply: "Самая свежая работа — Test Paper One [1]. Обзоры на списках не в счёт [2].",
		Sources: []client.Source{
			{URL: "https://arxiv.org/abs/2301.00001", Title: "Test Paper One"},
			{URL: "https://arxiv.org/list/cs.AI/recent", Title: "cs.AI listing"},
		},
	})

	chatID := client.NewSearchChatID()
	events, resp := u.Ask(chatID, question, "web")
	require.Equal(t, 200, resp.Status, resp)
	require.NotEmpty(t, events)
	require.Equal(t, "meta", events[0].Name, events[0].Data)
	require.Equal(t, chatID, events[0].Field("chat_id"))
	require.NotEmpty(t, client.Named(events, "delta"))
	done := client.Last(events)
	require.Equal(t, "done", done.Name, done.Data)
	answer := done.Field("content")
	require.Contains(t, answer, "Test Paper One [1]")
	require.Contains(t, answer, "## Sources", "a cited answer gets a sources section")
	require.Contains(t, answer, "https://arxiv.org/abs/2301.00001")
	require.NotContains(t, answer[strings.Index(answer, "## Sources"):], "arxiv.org/list", "index pages are not sources")
	require.Equal(t, answer, client.JoinDeltas(events), "the saved answer is the concatenation of deltas")

	sources := client.Named(events, "sources")
	require.NotEmpty(t, sources)
	last := sources[len(sources)-1].Field("sources")
	require.Contains(t, last, "https://arxiv.org/abs/2301.00001")
	require.NotContains(t, last, "arxiv.org/list")

	chats, resp := u.SearchChats()
	require.Equal(t, 200, resp.Status, resp)
	require.Len(t, chats, 1)
	require.Equal(t, chatID, chats[0].ID)
	require.Equal(t, "web", chats[0].Mode)
	require.True(t, strings.HasPrefix(question, chats[0].Title) || chats[0].Title == question, "title comes from the question: %q", chats[0].Title)

	chat, resp := u.SearchChatByID(chatID)
	require.Equal(t, 200, resp.Status, resp)
	require.Len(t, chat.Messages, 2)
	require.Equal(t, "user", chat.Messages[0].Role)
	require.Equal(t, question, chat.Messages[0].Content)
	require.Equal(t, "assistant", chat.Messages[1].Role)
	require.Len(t, chat.Messages[1].Sources, 1, "only the paper link is stored")
	require.Equal(t, "https://arxiv.org/abs/2301.00001", chat.Messages[1].Sources[0].URL)
	require.Equal(t, "arxiv.org", chat.Messages[1].Sources[0].Domain)

	paper, resp := u.AddFromURL(chat.Messages[1].Sources[0].URL, chat.Messages[1].Sources[0].Title)
	require.Equal(t, 201, resp.Status, resp)
	require.NotNil(t, paper.ArxivID)
	require.Equal(t, "2301.00001", *paper.ArxivID)
}

// F09-04 + F09-05 История уходит провайдеру; deep-режим меняет модель.
func TestF09_HistoryAndDeepMode(t *testing.T) {
	u := user(t, "f09-deep")
	t.Cleanup(func() { stand.LLM().Reset(t) })
	first := "F09-04 first " + client.Nonce()
	second := "F09-04 second " + client.Nonce()
	stand.LLM().Script(t, client.Script{Match: first, Reply: "первый ответ поиска"})
	stand.LLM().Script(t, client.Script{Match: second, Reply: "второй ответ поиска"})

	chatID := client.NewSearchChatID()
	_, resp := u.Ask(chatID, first, "web")
	require.Equal(t, 200, resp.Status, resp)
	events, resp := u.Ask(chatID, second, "deep")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "done", client.Last(events).Name, client.Last(events).Data)

	call := onlyCall(t, second)
	require.Equal(t, "test-model-deep", call.Model)
	require.Contains(t, call.Text("user"), first)
	require.Contains(t, call.Text("assistant"), "первый ответ поиска")
	chat, _ := u.SearchChatByID(chatID)
	require.Equal(t, "deep", chat.Mode)
	require.Len(t, chat.Messages, 4)

	third := "F09-05 unknown mode " + client.Nonce()
	stand.LLM().Script(t, client.Script{Match: third, Reply: "ок"})
	_, resp = u.Ask(chatID, third, "turbo")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "test-model", onlyCall(t, third).Model, "an unknown mode is treated as web")
	chat, _ = u.SearchChatByID(chatID)
	require.Equal(t, "web", chat.Mode)
}

// F09-06 + F09-07 Список, удаление, чужие чаты.
func TestF09_ListDeleteAndOwnership(t *testing.T) {
	u := user(t, "f09-list")
	t.Cleanup(func() { stand.LLM().Reset(t) })
	older, newer := client.NewSearchChatID(), client.NewSearchChatID()
	for _, id := range []string{older, newer} {
		q := "F09-06 " + id
		stand.LLM().Script(t, client.Script{Match: q, Reply: "ответ"})
		_, resp := u.Ask(id, q, "web")
		require.Equal(t, 200, resp.Status, resp)
	}
	chats, _ := u.SearchChats()
	require.Len(t, chats, 2)
	require.Equal(t, newer, chats[0].ID, "newest first")

	stranger := user(t, "f09-stranger")
	_, resp := stranger.SearchChatByID(older)
	require.Equal(t, 404, resp.Status, resp)
	require.Equal(t, 404, stranger.DeleteSearchChat(older).Status)
	_, resp = stranger.Ask(older, "hijack", "web")
	require.Equal(t, 404, resp.Status, "another user's chat cannot be continued: %s", resp)

	require.Equal(t, 204, u.DeleteSearchChat(older).Status)
	_, resp = u.SearchChatByID(older)
	require.Equal(t, 404, resp.Status)
	require.Equal(t, 404, u.DeleteSearchChat(older).Status)
	chats, _ = u.SearchChats()
	require.Len(t, chats, 1)
}

// F09-08 Валидация сообщения и id чата.
func TestF09_Validation(t *testing.T) {
	u := user(t, "f09-valid")
	id := client.NewSearchChatID()
	_, resp := u.Ask(id, "   ", "web")
	require.Equal(t, 400, resp.Status, resp)
	_, resp = u.Ask(id, strings.Repeat("я", 20001), "web")
	require.Equal(t, 400, resp.Status, resp)
	_, resp = u.Ask("not-a-uuid", "hello", "web")
	require.Equal(t, 404, resp.Status, resp)
	require.Equal(t, 400, u.Post("/search/chats/"+id+"/messages", map[string]any{"message": "x", "temperature": 1}).Status)
	chats, _ := u.SearchChats()
	require.Empty(t, chats, "rejected messages create no chat")
}

// F09-09 Провайдер упал или промолчал: событие error, вопрос сохранён, ответа нет.
func TestF09_ProviderFailure(t *testing.T) {
	u := user(t, "f09-fail")
	t.Cleanup(func() { stand.LLM().Reset(t) })
	broken := "F09-09 broken " + client.Nonce()
	stand.LLM().Script(t, client.Script{Match: broken, Status: 500})
	chatID := client.NewSearchChatID()
	events, resp := u.Ask(chatID, broken, "web")
	require.Equal(t, 200, resp.Status, resp)
	last := client.Last(events)
	require.Equal(t, "error", last.Name, last.Data)
	require.NotEmpty(t, last.Field("detail"))
	require.Empty(t, client.Named(events, "done"))
	chat, _ := u.SearchChatByID(chatID)
	require.Len(t, chat.Messages, 1, "the question is kept, no answer is stored")

	silent := "F09-09 silent " + client.Nonce()
	stand.LLM().Script(t, client.Script{Match: silent, Reply: "   "})
	events, resp = u.Ask(chatID, silent, "web")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "error", client.Last(events).Name, client.Last(events).Data)
}

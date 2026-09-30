//go:build integration

package integration

import (
	"sync"
	"testing"
	"time"

	"github.com/centraluniversity/researcher/tests/client"
	"github.com/centraluniversity/researcher/tests/fixtures"
	"github.com/stretchr/testify/require"
)

// F08 — обзор статьи. Ист.: ITERATION_PUSH (paper overview), PARSER.md §5, assistant/summary_http.go, 008.

// overview is a model reply that follows the required skeleton and ends like finished prose.
func overview(tag string) string {
	return "# Обзор " + tag + "\n\n## TL;DR\nКоротко.\n\n## Problem\n- Задача.\n\n## Method\n- Метод.\n\n## Results\n- 42%.\n\n## Takeaways\n- Вывод.\n\n## Limitations\n- Ограничение.\n\n## Deep dive\nПодробный разбор " + tag + "."
}

// freshParsed mints a synthetic arXiv paper and waits for its text, so every
// run starts without a cached overview.
func freshParsed(t *testing.T, s *client.Session) client.Paper {
	t.Helper()
	return s.ParsedArxiv(client.NewArxivID(client.SyntheticArxivOK))
}

// scriptOverview makes fake-llm answer summary prompts for this paper.
func scriptOverview(t *testing.T, paper client.Paper, reply string, times int) {
	t.Helper()
	stand.LLM().Script(t, client.Script{Match: paper.Title, Reply: reply, Times: times})
}

// F08-01 + F08-02 Первое открытие вкладки генерирует обзор, дальше он берётся из общего кэша.
func TestF08_GenerateAndCache(t *testing.T) {
	u := user(t, "f08-gen")
	paper := freshParsed(t, u.Session)
	t.Cleanup(func() { stand.LLM().Reset(t) })
	_, resp := u.GetSummary(paper.ID, "")
	require.Equal(t, 404, resp.Status, resp)

	content := overview(client.Nonce())
	scriptOverview(t, paper, content, 1)
	summary, resp := u.CreateSummary(paper.ID, "")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "ready", summary.Status)
	require.Equal(t, "ru", summary.Lang)
	require.Equal(t, "test-model", summary.Model)
	require.Equal(t, content, summary.Content)
	require.False(t, summary.Stale)
	require.NotNil(t, summary.VersionID)

	call := onlyCall(t, paper.Title)
	prompt := call.Text("user")
	require.Contains(t, prompt, "Write the overview in Russian")
	for _, heading := range []string{"## TL;DR", "## Problem", "## Method", "## Results", "## Takeaways", "## Limitations", "## Deep dive"} {
		require.Contains(t, prompt, heading)
	}
	require.Contains(t, prompt, "HYPOTHESIS-ON-PAGE-THREE", "the full text goes to the model")

	again, resp := u.CreateSummary(paper.ID, "")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, content, again.Content)
	require.Len(t, stand.LLM().Requests(t, paper.Title), 1, "a cached overview costs no provider call")

	got, resp := u.GetSummary(paper.ID, "")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, content, got.Content)

	other := user(t, "f08-other")
	theirs, resp := other.GetSummary(paper.ID, "")
	require.Equal(t, 200, resp.Status, "the overview is shared: %s", resp)
	require.Equal(t, content, theirs.Content)
}

// F08-03 Стрим: свежая генерация идёт дельтами, кэш приходит одним done.
func TestF08_Stream(t *testing.T) {
	u := user(t, "f08-stream")
	paper := freshParsed(t, u.Session)
	t.Cleanup(func() { stand.LLM().Reset(t) })
	content := overview(client.Nonce())
	scriptOverview(t, paper, content, 1)

	events, resp := u.CreateSummaryStream(paper.ID, "")
	require.Equal(t, 200, resp.Status, resp)
	require.Greater(t, len(events), 2)
	require.Equal(t, content, client.JoinDeltas(events))
	done := client.Last(events)
	require.Equal(t, "done", done.Type(), done.Data)
	require.Contains(t, done.Field("summary"), `"status":"ready"`)

	cached, resp := u.CreateSummaryStream(paper.ID, "")
	require.Equal(t, 200, resp.Status, resp)
	require.Len(t, cached, 1, "a cached overview is one done event")
	require.Equal(t, "done", cached[0].Type())
}

// F08-04 Язык обзора: отдельный кэш на en, неизвестный язык → ru.
func TestF08_Language(t *testing.T) {
	u := user(t, "f08-lang")
	paper := freshParsed(t, u.Session)
	t.Cleanup(func() { stand.LLM().Reset(t) })
	ru := overview("ru " + client.Nonce())
	scriptOverview(t, paper, ru, 1)
	_, resp := u.CreateSummary(paper.ID, "lang=ru")
	require.Equal(t, 200, resp.Status, resp)

	en := overview("en " + client.Nonce())
	scriptOverview(t, paper, en, 1)
	summary, resp := u.CreateSummary(paper.ID, "lang=en")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "en", summary.Lang)
	require.Equal(t, en, summary.Content)
	calls := stand.LLM().Requests(t, paper.Title)
	require.Len(t, calls, 2)
	require.Contains(t, calls[1].Text("user"), "Write the overview in English")

	got, _ := u.GetSummary(paper.ID, "lang=en")
	require.Equal(t, en, got.Content)
	got, _ = u.GetSummary(paper.ID, "lang=de")
	require.Equal(t, "ru", got.Lang, "unsupported languages fall back to Russian")
	require.Equal(t, ru, got.Content)
}

// F08-05 Пока текст парсится — 425, клиент повторяет позже.
func TestF08_TooEarlyWhileParsing(t *testing.T) {
	u := user(t, "f08-early")
	paper := freshParsed(t, u.Session)
	t.Cleanup(func() { stand.LLM().Reset(t) })
	stand.SetDocumentStatus(t, paper.ID, "pending")
	_, resp := u.CreateSummary(paper.ID, "")
	require.Equal(t, 425, resp.Status, resp)
	require.NotEmpty(t, resp.Detail())
	_, resp = u.CreateSummaryStream(paper.ID, "")
	require.Equal(t, 425, resp.Status, resp)

	stand.SetDocumentStatus(t, paper.ID, "ready")
	scriptOverview(t, paper, overview(client.Nonce()), 1)
	_, resp = u.CreateSummary(paper.ID, "")
	require.Equal(t, 200, resp.Status, resp)
}

// F08-06 Вторая вкладка во время генерации получает 409, первая доводит до done.
func TestF08_ConcurrentGeneration(t *testing.T) {
	u := user(t, "f08-conc")
	paper := freshParsed(t, u.Session)
	t.Cleanup(func() { stand.LLM().Reset(t) })
	stand.LLM().Script(t, client.Script{Match: paper.Title, Reply: overview(client.Nonce()), DelayMS: 3000, Times: 1})

	var wg sync.WaitGroup
	var events []client.Event
	var first client.Response
	wg.Add(1)
	go func() {
		defer wg.Done()
		events, first = u.Clone().WithToken(u.Token).CreateSummaryStream(paper.ID, "")
	}()
	time.Sleep(800 * time.Millisecond)
	_, second := u.CreateSummary(paper.ID, "")
	require.Equal(t, 409, second.Status, second)
	require.NotEmpty(t, second.Detail())
	wg.Wait()
	require.Equal(t, 200, first.Status, first)
	require.Equal(t, "done", client.Last(events).Type(), client.Last(events).Data)
}

// F08-07 + F08-08 force пересобирает; смена версии текста делает кэш stale и пересобирает без force.
func TestF08_ForceAndStale(t *testing.T) {
	u := user(t, "f08-stale")
	paper := freshParsed(t, u.Session)
	t.Cleanup(func() { stand.LLM().Reset(t) })
	first := overview("first " + client.Nonce())
	scriptOverview(t, paper, first, 1)
	_, resp := u.CreateSummary(paper.ID, "")
	require.Equal(t, 200, resp.Status, resp)

	forced := overview("forced " + client.Nonce())
	scriptOverview(t, paper, forced, 1)
	summary, resp := u.CreateSummary(paper.ID, "force=1")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, forced, summary.Content)
	require.Len(t, stand.LLM().Requests(t, paper.Title), 2)

	stand.DetachSummaryVersion(t, paper.ID, "ru")
	got, resp := u.GetSummary(paper.ID, "")
	require.Equal(t, 200, resp.Status, resp)
	require.True(t, got.Stale, "an overview built from an older parse is flagged")
	require.Equal(t, forced, got.Content, "but is still served")

	fresh := overview("fresh " + client.Nonce())
	scriptOverview(t, paper, fresh, 1)
	summary, resp = u.CreateSummary(paper.ID, "")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, fresh, summary.Content, "a stale overview is regenerated without force")
	require.False(t, summary.Stale)
}

// F08-09 Обрезанный ответ дописывается продолжением.
func TestF08_TruncatedReplyIsContinued(t *testing.T) {
	u := user(t, "f08-cont")
	paper := freshParsed(t, u.Session)
	t.Cleanup(func() { stand.LLM().Reset(t) })
	head := "# Обзор\n\n## TL;DR\nНачало, которое оборвалось на полу"
	tail := "слове.\n\n## Problem\n- Всё остальное " + client.Nonce() + "."
	stand.LLM().Script(t, client.Script{Match: paper.Title, Reply: head, FinishReason: "length", Times: 1})
	stand.LLM().Script(t, client.Script{Match: "Continue it exactly", Reply: tail, Times: 1})

	summary, resp := u.CreateSummary(paper.ID, "")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "ready", summary.Status)
	require.Equal(t, head+tail, summary.Content)
	calls := stand.LLM().Requests(t, paper.Title)
	require.Len(t, calls, 2)
	require.Contains(t, calls[1].Text("assistant"), head, "the partial text goes back as the assistant turn")
}

// F08-10 Пустой обрезанный ответ — ошибка, не кэш.
func TestF08_EmptyTruncatedReplyIsNotCached(t *testing.T) {
	u := user(t, "f08-empty")
	paper := freshParsed(t, u.Session)
	t.Cleanup(func() { stand.LLM().Reset(t) })
	stand.LLM().Script(t, client.Script{Match: paper.Title, Empty: true, FinishReason: "length", Times: 1})
	_, resp := u.CreateSummary(paper.ID, "")
	require.Equal(t, 502, resp.Status, resp)
	require.NotEmpty(t, resp.Detail())

	got, resp := u.GetSummary(paper.ID, "")
	require.True(t, resp.Status == 404 || got.Status != "ready", "no ready overview after a failure: %s", resp)

	good := overview(client.Nonce())
	scriptOverview(t, paper, good, 1)
	summary, resp := u.CreateSummary(paper.ID, "")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, good, summary.Content)
}

// F08-11 Статья без текста: обзор по аннотации.
func TestF08_AbstractOnly(t *testing.T) {
	u := user(t, "f08-abstract")
	paper, resp := u.AddDOI(client.NewDOI(client.SyntheticDOIClosed))
	require.Equal(t, 201, resp.Status, resp)
	t.Cleanup(func() { stand.LLM().Reset(t) })
	content := overview(client.Nonce())
	scriptOverview(t, paper, content, 1)
	summary, resp := u.CreateSummary(paper.ID, "")
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "ready", summary.Status)
	require.Nil(t, summary.VersionID, "nothing was parsed")
	require.Contains(t, onlyCall(t, paper.Title).Text("user"), "rely on the title and abstract only")
}

// F08-12 Доступ: неизвестная модель, чужая загрузка, гость.
func TestF08_Access(t *testing.T) {
	u := user(t, "f08-access")
	paper := publicPaper(t, u.Session, "2301.00001")
	_, resp := u.CreateSummary(paper.ID, "model=gpt-imaginary")
	require.Equal(t, 400, resp.Status, resp)

	owner := user(t, "f08-owner")
	private, resp := owner.UploadPDF("private.pdf", fixtures.PDF("private-1p.pdf"))
	require.Equal(t, 201, resp.Status, resp)
	_, resp = u.GetSummary(private.ID, "")
	require.Equal(t, 404, resp.Status, resp)
	_, resp = u.CreateSummary(private.ID, "")
	require.Equal(t, 404, resp.Status, resp)

	_, resp = guest(t).GetSummary(paper.ID, "")
	require.Equal(t, 401, resp.Status, resp)
}

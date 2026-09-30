//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/centraluniversity/researcher/tests/client"
	"github.com/centraluniversity/researcher/tests/fixtures"
	"github.com/stretchr/testify/require"
)

// F01 — гость. Ист.: backend/README.md «Guest access», SERVER_RECOVERY.md,
// identity/public.go. Кейсы: docs/testing/flows/F01-guest.md.

// F01-01 Лента открывается без входа и содержит статьи с id и заголовком.
func TestF01_TrendingFeedWithoutLogin(t *testing.T) {
	g := guest(t)
	resp := g.Get("/feed/trending")
	require.Equal(t, 200, resp.Status, resp)
	var out struct {
		Items []struct {
			ArxivID string `json:"arxiv_id"`
			Title   string `json:"title"`
		} `json:"items"`
		Category string `json:"category"`
		Sort     string `json:"sort"`
	}
	resp.JSON(t, &out)
	require.Equal(t, "cs.AI", out.Category, "default category")
	require.Equal(t, "new", out.Sort, "default sort")
	require.NotEmpty(t, out.Items)
	for _, it := range out.Items {
		require.NotEmpty(t, it.ArxivID)
		require.NotEmpty(t, it.Title)
	}
}

// F01-02 Лента кешируется: повторный запрос помечен cached.
func TestF01_TrendingFeedIsCached(t *testing.T) {
	g := guest(t)
	first := g.Get("/feed/trending?sort=hot&limit=5")
	require.Equal(t, 200, first.Status, first)
	second := g.Get("/feed/trending?sort=hot&limit=5")
	require.Equal(t, 200, second.Status, second)
	require.Equal(t, true, second.Map(t)["cached"], "second call within the hour comes from cache")
}

// F01-03 Неверные параметры ленты → 400, а не пустой список.
func TestF01_TrendingFeedRejectsBadParams(t *testing.T) {
	g := guest(t)
	for _, q := range []string{"?sort=best", "?limit=51", "?limit=-1", "?category=x"} {
		resp := g.Get("/feed/trending" + q)
		require.Equalf(t, 400, resp.Status, "%s → %s", q, resp)
		require.NotEmpty(t, resp.Detail())
	}
}

// F01-04 Гость открывает статью из ленты и получает PDF, не имея библиотеки.
func TestF01_GuestReadsArxivPaper(t *testing.T) {
	g := guest(t)
	paper, resp := g.OpenArxiv("2301.00001")
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, "Test Paper One: Deterministic Fixtures for Integration Tests", paper.Title)
	require.NotNil(t, paper.ArxivID)
	require.Equal(t, "2301.00001", *paper.ArxivID)
	require.Len(t, paper.Authors, 2)
	require.NotNil(t, paper.LatestVersion)
	require.Equal(t, "arxiv", paper.LatestVersion.Source)

	// The reader polls pdf-url; while the worker downloads it answers 409 pdf_processing.
	ready := g.WaitForPdfReady(paper.ID, 90*time.Second)
	require.Equal(t, 200, ready.Status, ready)
	require.Equal(t, "ready", ready.Map(t)["status"])

	pdf := g.PDF(paper.ID)
	require.Equal(t, 200, pdf.Status)
	require.Contains(t, pdf.Header.Get("Content-Type"), "application/pdf")
	require.Equal(t, fixtures.PDF("paper-5p.pdf"), pdf.Body, "the stored file is byte-for-byte what arXiv served")

	card := g.MustGetPaper(paper.ID)
	require.Equal(t, "ready", card.LatestVersion.Status)
}

// F01-05 Повторное открытие той же статьи не создаёт дубликат.
func TestF01_OpenArxivTwiceReturnsSamePaper(t *testing.T) {
	g := guest(t)
	first, r1 := g.OpenArxiv("2301.00002")
	require.Equal(t, 201, r1.Status, r1)
	second, r2 := g.OpenArxiv("https://arxiv.org/abs/2301.00002v1")
	require.Contains(t, []int{200, 201}, r2.Status, r2)
	require.Equal(t, first.ID, second.ID, "same arXiv id (with or without version, as URL) is the same paper")
}

// F01-06 arxiv/open никогда не кладёт статью в библиотеку, даже если попросить.
func TestF01_OpenArxivNeverAddsToLibrary(t *testing.T) {
	u := user(t, "f01-open")
	resp := u.Post("/papers/arxiv/open", map[string]any{"arxiv_id": "2301.00003", "add_to_library": true})
	require.Equal(t, 201, resp.Status, resp)
	var p client.Paper
	resp.JSON(t, &p)
	require.False(t, u.HasInLibrary(p.ID), "open is read-only for the library")
}

// F01-07 Несуществующий и битый arXiv id дают понятную ошибку клиента, не 5xx.
func TestF01_OpenArxivBadIds(t *testing.T) {
	t.Run("malformed id", func(t *testing.T) {
		_, resp := guest(t).OpenArxiv("not-an-id")
		require.Equal(t, 400, resp.Status, resp)
	})
	t.Run("unknown id", func(t *testing.T) {
		_, resp := guest(t).OpenArxiv("2301.99999")
		// Ожидание: arXiv не знает такой статьи → ошибка клиента (404/422), не 5xx. Сейчас 500 — см. D-11.
		require.Contains(t, []int{404, 422}, resp.Status, "unknown arXiv id must not be a server error: %s", resp)
		require.NotEmpty(t, resp.Detail())
	})
}

// F01-08 Гость может переводить выделение публичной статьи (обычный ответ и стрим).
func TestF01_GuestTranslatesSelection(t *testing.T) {
	g := guest(t)
	paper, resp := g.OpenArxiv("2301.00001")
	require.Equal(t, 201, resp.Status, resp)

	source := "F01-08 unique source sentence " + client.UniqueEmail("x")
	stand.LLM().Script(t, client.Script{Match: source, Reply: "Уникальное исходное предложение F01-08", Times: 2})

	plain := g.Translate(paper.ID, source, "ru")
	require.Equal(t, 200, plain.Status, plain)
	require.Equal(t, "Уникальное исходное предложение F01-08", plain.Map(t)["translation"])
	require.Equal(t, "ru", plain.Map(t)["target_lang"])

	events := g.TranslateStream(paper.ID, source, "ru")
	require.NotEmpty(t, events)
	require.Equal(t, "done", client.Last(events).Type(), "stream ends with a done event")
	require.Equal(t, "Уникальное исходное предложение F01-08", client.JoinDeltas(events))

	calls := stand.LLM().Requests(t, source)
	require.Len(t, calls, 2, "both requests reached the provider with the selected text")
	require.Contains(t, calls[0].Text("system"), "translat", "the provider is asked to translate, not to chat")
}

// F01-09 Всё личное для гостя закрыто: 401, а не пустые данные.
func TestF01_GuestIsRefusedOnPersonalEndpoints(t *testing.T) {
	g := guest(t)
	paper, resp := g.OpenArxiv("2301.00001")
	require.Equal(t, 201, resp.Status, resp)
	pid := paper.ID
	cases := []struct {
		method, path string
		body         any
	}{
		{"GET", "/auth/me", nil},
		{"GET", "/library", nil},
		{"GET", "/library/folders", nil},
		{"POST", "/library/" + pid, map[string]any{}},
		{"POST", "/papers/arxiv", map[string]any{"arxiv_id": "2301.00001"}},
		{"POST", "/papers/doi", map[string]any{"doi": "10.1000/xyz"}},
		{"GET", "/papers/" + pid + "/annotations", nil},
		{"POST", "/papers/" + pid + "/annotations", map[string]any{"page": 1, "selected_text": "x", "note": "y"}},
		{"GET", "/papers/" + pid + "/chat/messages", nil},
		{"POST", "/papers/" + pid + "/chat", map[string]any{"message": "hi"}},
		{"POST", "/papers/" + pid + "/explain", map[string]any{"text": "hi"}},
		{"GET", "/papers/" + pid + "/summary", nil},
		{"POST", "/papers/" + pid + "/summary", nil},
		{"POST", "/papers/" + pid + "/retry-pdf", nil},
		{"POST", "/papers/" + pid + "/find-fulltext", nil},
		{"GET", "/search/chats", nil},
		{"POST", "/search/chats/00000000-0000-0000-0000-000000000001/messages", map[string]any{"message": "q", "mode": "web"}},
		{"GET", "/assistant/models", nil},
	}
	for _, c := range cases {
		resp := g.Do(c.method, c.path, c.body)
		require.Equalf(t, 401, resp.Status, "%s %s → %s", c.method, c.path, resp)
	}
	up := g.PostMultipart("/papers/upload", "file", "paper-1p.pdf", "application/pdf", fixtures.PDF("paper-1p.pdf"))
	require.Equal(t, 401, up.Status, up)
}

// F01-10 Загруженный пользователем PDF гостю недоступен (и карточка тоже).
func TestF01_GuestCannotSeeUploadedPaper(t *testing.T) {
	u := user(t, "f01-upload")
	paper, resp := u.UploadPDF("private-notes.pdf", fixtures.PDF("private-1p.pdf"))
	require.Equal(t, 201, resp.Status, resp)

	g := guest(t)
	_, card := g.GetPaper(paper.ID)
	require.Equal(t, 404, card.Status, card)
	pdfURL := g.Get("/papers/" + paper.ID + "/pdf-url")
	require.Equal(t, 404, pdfURL.Status, pdfURL)
	pdf := g.PDF(paper.ID)
	require.Equal(t, 404, pdf.Status, pdf)
	tr := g.Translate(paper.ID, "anything", "ru")
	require.Equal(t, 404, tr.Status, tr)
}

// F01-11 Подделать пользователя заголовком нельзя: gateway его отбрасывает.
func TestF01_GuestCannotImpersonateWithUserIdHeader(t *testing.T) {
	u := user(t, "f01-victim")
	me := u.Get("/auth/me").Map(t)["id"].(string)

	g := guest(t)
	resp := g.Get("/library", "X-User-Id", me)
	require.Equal(t, 401, resp.Status, resp)
}

// F01-12 Гость с просроченным/мусорным токеном получает 401 даже на публичном маршруте.
func TestF01_GarbageBearerTokenIsRejectedEvenOnPublicRoutes(t *testing.T) {
	g := guest(t)
	g.Token = "not-a-jwt"
	resp := g.Get("/feed/trending")
	require.Equal(t, 401, resp.Status, resp)
}

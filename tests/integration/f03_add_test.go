//go:build integration

package integration

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/centraluniversity/researcher/tests/client"
	"github.com/centraluniversity/researcher/tests/fixtures"
	"github.com/stretchr/testify/require"
)

// F03 — добавление статьи. Ист.: HANDOFF §5, iteration-1, STATUS EPIC-03/12, catalog/url.go.

// F03-01 arXiv → библиотека: карточка из arXiv, статья в «Хочу прочитать» как unread.
func TestF03_AddArxivToLibrary(t *testing.T) {
	u := user(t, "f03-arxiv")
	paper, resp := u.AddArxiv("2301.00001")
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, "Test Paper One: Deterministic Fixtures for Integration Tests", paper.Title)
	require.NotNil(t, paper.Abstract)
	require.Contains(t, *paper.Abstract, "hypothesis marker")
	require.Len(t, paper.Authors, 2)
	require.NotNil(t, paper.ArxivID)

	item, resp := u.LibraryItemOf(paper.ID)
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "unread", item.Status)
	require.NotNil(t, item.FolderID, "a new paper lands in a folder")
	require.Equal(t, u.SystemFolder("want_to_read").ID, *item.FolderID, "the default folder is «Хочу прочитать»")
}

// F03-02 Один arXiv id в любой записи — одна статья и одна строка библиотеки.
func TestF03_ArxivIsDeduplicated(t *testing.T) {
	u := user(t, "f03-dedup")
	first, resp := u.AddArxiv("2301.00002")
	require.Equal(t, 201, resp.Status, resp)
	for _, form := range []string{"2301.00002v2", "https://arxiv.org/abs/2301.00002", "https://arxiv.org/pdf/2301.00002v1", "arXiv:2301.00002", "2301.00002"} {
		again, resp := u.AddArxiv(form)
		require.Contains(t, []int{200, 201}, resp.Status, "%s → %s", form, resp)
		require.Equal(t, first.ID, again.ID, "%s must resolve to the same paper", form)
	}
	items, resp := u.Library()
	require.Equal(t, 200, resp.Status, resp)
	require.Len(t, items, 1, "one library row for one paper")
}

// F03-03 add_to_library:false добавляет статью в каталог, но не в библиотеку.
func TestF03_AddArxivWithoutLibrary(t *testing.T) {
	u := user(t, "f03-nolib")
	resp := u.Post("/papers/arxiv", map[string]any{"arxiv_id": "2301.00003", "add_to_library": false})
	require.Equal(t, 201, resp.Status, resp)
	require.False(t, u.HasInLibrary(resp.Map(t)["id"].(string)))
}

// F03-04 DOI без открытого доступа: карточка есть, файла нет, причина понятна.
func TestF03_AddDOIWithoutOpenAccess(t *testing.T) {
	u := user(t, "f03-doi")
	doi := client.NewDOI(client.SyntheticDOIClosed)
	paper, resp := u.AddDOI(doi)
	require.Equal(t, 201, resp.Status, resp)
	require.Contains(t, paper.Title, "Synthetic DOI Fixture")
	require.Len(t, paper.Authors, 2)
	require.NotNil(t, paper.DOI)
	require.Equal(t, doi, *paper.DOI)
	require.NotNil(t, paper.LatestVersion)
	require.Equal(t, "doi", paper.LatestVersion.Source)
	require.Equal(t, "ready", paper.LatestVersion.Status)
	require.Nil(t, paper.LatestVersion.PDFKey, "a DOI record has no file")
	require.False(t, paper.HasFullText)

	card := u.Get("/papers/" + paper.ID)
	require.Equal(t, 200, card.Status)
	body := card.Map(t)
	require.EqualValues(t, 2022, body["year"])
	require.Equal(t, "Journal of Deterministic Fixtures", body["venue"])

	pdf := u.Get("/papers/" + paper.ID + "/pdf-url")
	require.Equal(t, 422, pdf.Status, pdf)
	require.Equal(t, "pdf_unavailable", pdf.Code())
	require.Contains(t, strings.ToLower(pdf.Detail()), "open access")
	require.True(t, u.HasInLibrary(paper.ID))
}

// F03-05 DOI, для которого OpenAlex знает PDF издателя: файл скачивается.
func TestF03_AddDOIWithOpenAccessPDF(t *testing.T) {
	u := user(t, "f03-oa")
	paper, resp := u.AddDOI(client.NewDOI(client.SyntheticDOIOpen))
	require.Equal(t, 201, resp.Status, resp)
	require.NotNil(t, paper.LatestVersion)
	require.Equal(t, "web_pdf", paper.LatestVersion.Source)
	ready := u.WaitForPdfReady(paper.ID, 90*time.Second)
	require.Equal(t, 200, ready.Status, ready)
	pdf := u.PDF(paper.ID)
	require.Equal(t, 200, pdf.Status)
	require.Equal(t, fixtures.PDF("paper-oa-1p.pdf"), pdf.Body, "the publisher's PDF is stored byte-for-byte")
}

// F03-06 DOI, который OpenAlex связывает с arXiv, открывается как arXiv-статья.
func TestF03_AddDOIKnownAsArxiv(t *testing.T) {
	u := user(t, "f03-twin")
	paper, resp := u.AddDOI("10.5555/fixture.arxiv-twin")
	require.Equal(t, 201, resp.Status, resp)
	require.NotNil(t, paper.ArxivID)
	require.Equal(t, "2301.00003", *paper.ArxivID)
	require.Equal(t, "arxiv", paper.LatestVersion.Source)
	byID, resp := u.AddArxiv("2301.00003")
	require.Contains(t, []int{200, 201}, resp.Status, resp)
	require.Equal(t, byID.ID, paper.ID, "the same paper whichever identifier is used")
}

// F03-07 Формы записи DOI ведут к одной статье.
func TestF03_DOIIsDeduplicated(t *testing.T) {
	u := user(t, "f03-doiforms")
	doi := client.NewDOI(client.SyntheticDOIClosed)
	first, resp := u.AddDOI(doi)
	require.Equal(t, 201, resp.Status, resp)
	for _, form := range []string{"https://doi.org/" + doi, "doi:" + doi, strings.ToUpper(doi)} {
		again, resp := u.AddDOI(form)
		require.Contains(t, []int{200, 201}, resp.Status, "%s → %s", form, resp)
		require.Equal(t, first.ID, again.ID, "%s must resolve to the same paper", form)
	}
	items, resp := u.Library()
	require.Equal(t, 200, resp.Status, resp)
	require.Len(t, items, 1)
}

// F03-08 Ошибки DOI: мусор, неизвестный, журнал вместо статьи.
func TestF03_DOIErrors(t *testing.T) {
	u := user(t, "f03-doierr")
	t.Run("malformed", func(t *testing.T) {
		_, resp := u.AddDOI("not-a-doi")
		require.Equal(t, 400, resp.Status, resp)
		require.NotEmpty(t, resp.Detail())
	})
	t.Run("unknown", func(t *testing.T) {
		_, resp := u.AddDOI("10.5555/nope")
		require.Equal(t, 422, resp.Status, resp)
		require.Equal(t, "not_found", resp.Code())
	})
	t.Run("container", func(t *testing.T) {
		_, resp := u.AddDOI("10.5555/fixture.journal")
		require.Equal(t, 422, resp.Status, resp)
		require.Equal(t, "not_a_paper", resp.Code())
	})
}

// F03-09 Загрузка PDF: обработка в фоне, заголовок из имени файла.
func TestF03_UploadPDF(t *testing.T) {
	u := user(t, "f03-upload")
	paper, resp := u.UploadPDF("my_reading-notes.pdf", fixtures.PDF("private-1p.pdf"))
	require.Equal(t, 201, resp.Status, resp)
	require.NotNil(t, paper.LatestVersion)
	require.Equal(t, "upload", paper.LatestVersion.Source)
	require.Contains(t, []string{"processing", "ready"}, paper.LatestVersion.Status)
	require.Equal(t, "my reading notes", paper.Title, "no metadata in the PDF → title from the file name")
	require.True(t, u.HasInLibrary(paper.ID))

	ready := u.WaitForPdfReady(paper.ID, 90*time.Second)
	require.Equal(t, 200, ready.Status, ready)
	pdf := u.PDF(paper.ID)
	require.Equal(t, fixtures.PDF("private-1p.pdf"), pdf.Body)
}

// F03-10 Тот же файл у другого пользователя — та же статья (дедуп по SHA-256).
func TestF03_UploadIsDeduplicatedBySHA(t *testing.T) {
	a := user(t, "f03-sha-a")
	b := user(t, "f03-sha-b")
	pa, resp := a.UploadPDF("notes.pdf", fixtures.PDF("private-1p.pdf"))
	require.Equal(t, 201, resp.Status, resp)
	pb, resp := b.UploadPDF("something-else.pdf", fixtures.PDF("private-1p.pdf"))
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, pa.ID, pb.ID, "identical bytes are one paper")
	require.True(t, a.HasInLibrary(pa.ID))
	require.True(t, b.HasInLibrary(pa.ID))
}

// F03-11 Не PDF отклоняется сразу.
func TestF03_UploadRejectsNonPDF(t *testing.T) {
	u := user(t, "f03-notpdf")
	t.Run("text file", func(t *testing.T) {
		resp := u.PostMultipart("/papers/upload", "file", "not-a-pdf.txt", "text/plain", fixtures.PDF("not-a-pdf.txt"))
		require.Equal(t, 400, resp.Status, resp)
		require.Contains(t, resp.Detail(), "PDF")
	})
	t.Run("empty file", func(t *testing.T) {
		resp := u.PostMultipart("/papers/upload", "file", "empty.pdf", "application/pdf", nil)
		require.Equal(t, 400, resp.Status, resp)
	})
}

// F03-12 [assumption] HTML под именем .pdf не должен приниматься: без сигнатуры %PDF- ридер его не откроет.
func TestF03_UploadRejectsHTMLNamedPDF(t *testing.T) {
	u := user(t, "f03-html")
	resp := u.PostMultipart("/papers/upload", "file", "not-a-pdf.pdf", "application/pdf", fixtures.PDF("not-a-pdf.pdf"))
	require.Equal(t, 400, resp.Status, "a file without a PDF header must be refused: %s", resp)
}

// F03-13 Ссылка на arXiv (abs / pdf) и на doi.org ведёт к той же статье, что и id.
func TestF03_AddFromURL(t *testing.T) {
	u := user(t, "f03-url")
	byID, resp := u.AddArxiv("2301.00002")
	require.Contains(t, []int{200, 201}, resp.Status, resp)
	for _, link := range []string{"https://arxiv.org/abs/2301.00002", "https://arxiv.org/pdf/2301.00002v1", "http://export.arxiv.org/abs/2301.00002"} {
		p, resp := u.AddFromURL(link, "")
		require.Equal(t, 201, resp.Status, "%s → %s", link, resp)
		require.Equal(t, byID.ID, p.ID, link)
	}
	byDOI, resp := u.AddDOI("10.5555/fixture.metadata-only")
	require.Contains(t, []int{200, 201}, resp.Status, resp)
	p, resp := u.AddFromURL("https://doi.org/10.5555/fixture.metadata-only", "Closed Access Fixture")
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, byDOI.ID, p.ID)
}

// F03-14 Ссылка на список статей или мусор — понятная ошибка.
func TestF03_FromURLRejectsNonPapers(t *testing.T) {
	u := user(t, "f03-urlerr")
	t.Run("index page", func(t *testing.T) {
		_, resp := u.AddFromURL("https://arxiv.org/list/cs.AI/recent", "")
		require.Equal(t, 422, resp.Status, resp)
		require.Equal(t, "not_a_paper", resp.Code())
	})
	t.Run("not a url", func(t *testing.T) {
		_, resp := u.AddFromURL("not a url", "")
		require.Equal(t, 400, resp.Status, resp)
	})
	t.Run("ftp", func(t *testing.T) {
		_, resp := u.AddFromURL("ftp://arxiv.org/abs/2301.00001", "")
		require.Equal(t, 400, resp.Status, resp)
	})
}

// F03-15 Ссылка на статью, которой arXiv не знает — ошибка клиента, не 5xx (см. D-11).
func TestF03_FromURLUnknownArxiv(t *testing.T) {
	u := user(t, "f03-urlunknown")
	_, resp := u.AddFromURL("https://arxiv.org/abs/2301.99999", "")
	require.Contains(t, []int{404, 422}, resp.Status, "unknown arXiv id must not be a server error: %s", resp)
	require.NotEmpty(t, resp.Detail())
}

// F03-16 Лимит на добавление: 60 в минуту на пользователя, потом 429 с Retry-After.
func TestF03_ResolveThrottle(t *testing.T) {
	stand.ResetThrottle(t)
	u := user(t, "f03-throttle")
	stand.ResetThrottle(t)
	var mu sync.Mutex
	statuses := map[int]int{}
	var wg sync.WaitGroup
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := u.Post("/papers/arxiv", map[string]any{"arxiv_id": "not-an-id"})
			mu.Lock()
			statuses[resp.Status]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	require.Equal(t, 60, statuses[400], "the first 60 resolves are answered on their merits: %v", statuses)
	resp := u.Post("/papers/arxiv", map[string]any{"arxiv_id": "not-an-id"})
	require.Equal(t, 429, resp.Status, resp)
	require.Equal(t, "rate_limited", resp.Code())
	require.NotEmpty(t, resp.Header.Get("Retry-After"))
	stand.ResetThrottle(t)
}

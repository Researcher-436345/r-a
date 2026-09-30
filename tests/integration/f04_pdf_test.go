//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/centraluniversity/researcher/tests/client"
	"github.com/centraluniversity/researcher/tests/fixtures"
	"github.com/stretchr/testify/require"
)

// F04 — обработка PDF и полного текста. Ист.: PARSER.md, HANDOFF §4, catalog/http.go (B3), cmd/worker.

// F04-01 + F04-02 Свежая статья: 409 pdf_processing → ready → текст разобран и виден чату по страницам.
func TestF04_ProcessingToReadyToFullText(t *testing.T) {
	u := user(t, "f04-flow")
	id := client.NewArxivID(client.SyntheticArxivOK)
	paper, resp := u.AddArxiv(id)
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, "processing", paper.LatestVersion.Status, "a fresh paper starts processing")

	seen := map[int]bool{}
	deadline := time.Now().Add(90 * time.Second)
	for {
		r := u.Get("/papers/" + paper.ID + "/pdf-url")
		seen[r.Status] = true
		if r.Status == 200 {
			require.Equal(t, "ready", r.Map(t)["status"])
			break
		}
		require.Equal(t, 409, r.Status, "while downloading the reader gets 409 pdf_processing: %s", r)
		require.Equal(t, "pdf_processing", r.Code())
		require.False(t, time.Now().After(deadline), "PDF still processing after 90s")
		time.Sleep(300 * time.Millisecond)
	}
	require.Equal(t, fixtures.PDF("paper-5p.pdf"), u.PDF(paper.ID).Body)

	parsed := u.WaitForFullText(paper.ID, 3*time.Minute)
	require.True(t, parsed.HasFullText)
	ctx := u.ChatContext(paper.ID, "")
	require.Equal(t, 200, ctx.Status, ctx)
	require.Equal(t, true, ctx.Map(t)["has_full_paper"])
	require.Greater(t, ctx.Map(t)["paper_tokens"].(float64), 0.0)

	question := "F04-02 where is the hypothesis " + client.Nonce()
	reply := u.Chat(paper.ID, map[string]any{"message": question})
	require.Equal(t, 200, reply.Status, reply)
	prompt := onlyCall(t, question).Text("user")
	require.Contains(t, prompt, "<<<p=1>>>", "the prompt carries page markers (a five-page fixture fits one chunk)")
	require.Contains(t, prompt, "HYPOTHESIS-ON-PAGE-THREE", "the prompt carries the parsed text")
}

// F04-03 + F04-04 Издатель отвечает 503: версия failed с понятной причиной; после починки retry-pdf доводит до ready.
func TestF04_DownloadFailsThenRetrySucceeds(t *testing.T) {
	u := user(t, "f04-retry")
	id := client.NewArxivID(client.SyntheticArxivOK)
	stand.Scholar().Fail(t, client.Rule{PathPrefix: "/pdf/" + id, Status: 503, Body: "publisher down"})
	t.Cleanup(func() { stand.Scholar().Reset(t) })

	paper, resp := u.AddArxiv(id)
	require.Equal(t, 201, resp.Status, resp)
	failed := u.WaitForVersionStatus(paper.ID, "failed", 3*time.Minute)
	require.NotNil(t, failed.LatestVersion.ErrorMessage)
	require.Contains(t, *failed.LatestVersion.ErrorMessage, "arxiv.org", "the reason names the host that refused")

	pdfURL := u.Get("/papers/" + paper.ID + "/pdf-url")
	require.Equal(t, 422, pdfURL.Status, pdfURL)
	require.Equal(t, "pdf_unavailable", pdfURL.Code())
	require.Equal(t, *failed.LatestVersion.ErrorMessage, pdfURL.Detail())
	require.Equal(t, 422, u.PDF(paper.ID).Status)

	stand.Scholar().Reset(t)
	retried, resp := u.RetryPDF(paper.ID)
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "processing", retried.LatestVersion.Status)
	require.Nil(t, retried.LatestVersion.ErrorMessage)
	ready := u.WaitForPdfReady(paper.ID, 90*time.Second)
	require.Equal(t, 200, ready.Status, ready)
	require.Equal(t, fixtures.PDF("paper-5p.pdf"), u.PDF(paper.ID).Body)
	require.True(t, u.WaitForFullText(paper.ID, 3*time.Minute).HasFullText)
}

// F04-05 Издатель отдаёт HTML вместо PDF: failed, retry не помогает, find-fulltext честно говорит «не нашёл».
func TestF04_HTMLInsteadOfPDF(t *testing.T) {
	u := user(t, "f04-html")
	id := client.NewArxivID(client.SyntheticArxivNotPDF)
	paper, resp := u.AddArxiv(id)
	require.Equal(t, 201, resp.Status, resp)
	failed := u.WaitForVersionStatus(paper.ID, "failed", 3*time.Minute)
	require.NotNil(t, failed.LatestVersion.ErrorMessage)
	require.Contains(t, *failed.LatestVersion.ErrorMessage, "did not return a PDF")

	retried, resp := u.RetryPDF(paper.ID)
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "processing", retried.LatestVersion.Status)
	again := u.WaitForVersionStatus(paper.ID, "failed", 3*time.Minute)
	require.NotNil(t, again.LatestVersion.ErrorMessage)

	_, resp = u.FindFullText(paper.ID)
	require.Equal(t, 422, resp.Status, resp)
	require.Equal(t, "not_found", resp.Code())
}

// F04-06 retry на готовом PDF ничего не скачивает заново.
func TestF04_RetryOnReadyPDFIsNoop(t *testing.T) {
	u := user(t, "f04-noop")
	id := client.NewArxivID(client.SyntheticArxivOK)
	paper, resp := u.AddArxiv(id)
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, 200, u.WaitForPdfReady(paper.ID, 90*time.Second).Status)
	before := len(stand.Scholar().Requests(t, "/pdf/"+id))
	retried, resp := u.RetryPDF(paper.ID)
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "ready", retried.LatestVersion.Status)
	time.Sleep(2 * time.Second)
	require.Equal(t, before, len(stand.Scholar().Requests(t, "/pdf/"+id)), "no second download")
}

// F04-07 + F04-08 Статья по DOI: retry невозможен, find-fulltext ищет в открытых источниках.
func TestF04_DOIPaperRetryAndFindFullText(t *testing.T) {
	u := user(t, "f04-doi")
	closed, resp := u.AddDOI(client.NewDOI(client.SyntheticDOIClosed))
	require.Equal(t, 201, resp.Status, resp)
	_, resp = u.RetryPDF(closed.ID)
	require.Equal(t, 422, resp.Status, resp)
	require.Equal(t, "pdf_unavailable", resp.Code())
	require.Contains(t, resp.Detail(), "find-fulltext")
	_, resp = u.FindFullText(closed.ID)
	require.Equal(t, 422, resp.Status, resp)
	require.Equal(t, "not_found", resp.Code())

	open, resp := u.AddDOI(client.NewDOI(client.SyntheticDOIOpen))
	require.Equal(t, 201, resp.Status, resp)
	ready := u.WaitForPdfReady(open.ID, 90*time.Second)
	require.Equal(t, 200, ready.Status, ready)
	found, resp := u.FindFullText(open.ID)
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "ready", found.LatestVersion.Status)
	require.Equal(t, "web_pdf", found.LatestVersion.Source)
}

// F04-09 Загруженный файл проходит тот же путь: ready, те же байты, текст разобран.
func TestF04_UploadIsParsed(t *testing.T) {
	u := user(t, "f04-upload")
	paper, resp := u.UploadPDF("private-1p.pdf", fixtures.PDF("private-1p.pdf"))
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, 200, u.WaitForPdfReady(paper.ID, 90*time.Second).Status)
	require.Equal(t, fixtures.PDF("private-1p.pdf"), u.PDF(paper.ID).Body)
	parsed := u.WaitForFullText(paper.ID, 3*time.Minute)
	require.True(t, parsed.HasFullText)
	question := "F04-09 what is on the page " + client.Nonce()
	require.Equal(t, 200, u.Chat(paper.ID, map[string]any{"message": question}).Status)
	require.Contains(t, onlyCall(t, question).Text("user"), "PRIVATE-MARKER-PAGE-1")
}

// F04-10 Неизвестная статья и не-uuid на маршрутах PDF — 404.
func TestF04_PDFRoutesUnknownPaper(t *testing.T) {
	u := user(t, "f04-404")
	for _, id := range []string{"00000000-0000-0000-0000-000000000001", "not-a-uuid"} {
		require.Equal(t, 404, u.Get("/papers/"+id+"/pdf-url").Status, id)
		require.Equal(t, 404, u.Get("/papers/"+id+"/pdf").Status, id)
		require.Equal(t, 404, u.Post("/papers/"+id+"/retry-pdf", nil).Status, id)
		require.Equal(t, 404, u.Post("/papers/"+id+"/find-fulltext", nil).Status, id)
	}
}

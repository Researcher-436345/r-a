//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/centraluniversity/researcher/tests/client"
	"github.com/centraluniversity/researcher/tests/fixtures"
	"github.com/stretchr/testify/require"
)

// F12 — деградация. Ист.: HANDOFF §4 (fail-open), PARSER.md «Ops», platform/queue.go, cmd/worker/main.go.

// F12-01 arXiv отвечает 503: понятная ошибка, а не 500 и не зависание.
func TestF12_ArxivDown(t *testing.T) {
	u := user(t, "f12-arxiv")
	stand.Scholar().Fail(t, client.Rule{PathPrefix: "/api/query", Status: 503, Body: "arXiv is down"})
	t.Cleanup(func() { stand.Scholar().Reset(t) })

	started := time.Now()
	_, resp := u.OpenArxiv(client.NewArxivID(client.SyntheticArxivOK))
	require.Equal(t, 502, resp.Status, resp)
	require.NotEmpty(t, resp.Detail())
	require.Less(t, time.Since(started), 30*time.Second, "the failure is reported promptly")

	feed := guest(t).Get("/feed/trending?category=cs.LG&sort=new")
	require.Contains(t, []int{200, 502}, feed.Status, feed)
	if feed.Status == 502 {
		require.NotEmpty(t, feed.Detail())
	}
}

// F12-02 Без Redis вход работает: троттлинг открыт, сессии в базе.
func TestF12_RedisDown(t *testing.T) {
	u := user(t, "f12-redis")
	stand.StopService(t, "redis")
	login := u.Clone().Login(u.Email, u.Password)
	require.Equal(t, 200, login.Status, "throttle fails open without Redis: %s", login)
	require.Equal(t, 200, u.Get("/auth/me").Status)
	feed := guest(t).Get("/feed/trending?category=cs.CV")
	require.Contains(t, []int{200, 502}, feed.Status, feed)
}

// F12-03 Без хранилища загрузка и чтение PDF отвечают 502, а не зависают.
func TestF12_StorageDown(t *testing.T) {
	u := user(t, "f12-minio")
	paper := u.ReadyArxiv("2301.00001")
	stand.StopService(t, "minio")
	// Unique bytes: an already stored file would be deduplicated by SHA-256
	// and never touch the storage.
	unique := append(fixtures.PDF("private-1p.pdf"), []byte("\n% "+client.Nonce()+"\n")...)
	_, resp := u.UploadPDF("private.pdf", unique)
	require.Equal(t, 502, resp.Status, resp)
	require.NotEmpty(t, resp.Detail())
	pdf := u.PDF(paper.ID)
	require.Equal(t, 502, pdf.Status, pdf)
}

// F12-04 Парсер недоступен: документ помечается failed, ридер и чат живут (D-10).
func TestF12_ParserDown(t *testing.T) {
	u := user(t, "f12-parser")
	stand.StopService(t, "parser")
	paper, resp := u.AddArxiv(client.NewArxivID(client.SyntheticArxivOK))
	require.Equal(t, 201, resp.Status, resp)
	require.Equal(t, 200, u.WaitForPdfReady(paper.ID, 90*time.Second).Status, "the PDF itself is fine")

	deadline := time.Now().Add(90 * time.Second)
	var status, message string
	for {
		status, message = stand.DocumentStatus(t, paper.ID)
		if status == "failed" || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Second)
	}
	require.Equal(t, "failed", status, "the document must not stay pending when the parser is down (D-10); last error: %q", message)
	require.NotEmpty(t, message)

	ctx := u.ChatContext(paper.ID, "")
	require.Equal(t, 200, ctx.Status, ctx)
	require.Equal(t, false, ctx.Map(t)["has_full_paper"])
	question := "F12-04 " + client.Nonce()
	require.Equal(t, 200, u.Chat(paper.ID, map[string]any{"message": question}).Status)
	require.Contains(t, onlyCall(t, question).Text("user"), "Full paper text is not available yet")

	t.Cleanup(func() { stand.LLM().Reset(t) })
	stand.LLM().Script(t, client.Script{Match: paper.Title, Reply: overview(client.Nonce()), Times: 1})
	_, resp = u.CreateSummary(paper.ID, "")
	require.NotEqual(t, 425, resp.Status, "a failed parse must not look like «still parsing»: %s", resp)
	require.Equal(t, 200, resp.Status, resp)
}

// F12-05 Перезапуск воркера посреди загрузки: задача возвращается в очередь, статья доходит до ready.
func TestF12_WorkerRestartMidDownload(t *testing.T) {
	u := user(t, "f12-worker")
	id := client.NewArxivID(client.SyntheticArxivOK)
	stand.Scholar().Fail(t, client.Rule{PathPrefix: "/pdf/" + id, Status: 200, DelayMS: 8000, Body: "slow"})
	t.Cleanup(func() { stand.Scholar().Reset(t) })

	paper, resp := u.AddArxiv(id)
	require.Equal(t, 201, resp.Status, resp)
	time.Sleep(1500 * time.Millisecond)
	stand.RestartService(t, "worker")
	stand.Scholar().Reset(t)
	ready := u.WaitForPdfReady(paper.ID, 4*time.Minute)
	require.Equal(t, 200, ready.Status, ready)
	require.Equal(t, fixtures.PDF("paper-5p.pdf"), u.PDF(paper.ID).Body)
}

// F12-06 Повторный retry-pdf во время загрузки не запускает вторую загрузку.
func TestF12_DuplicateJobsAreDropped(t *testing.T) {
	u := user(t, "f12-dedupe")
	id := client.NewArxivID(client.SyntheticArxivOK)
	stand.Scholar().Fail(t, client.Rule{PathPrefix: "/pdf/" + id, Status: 200, DelayMS: 5000, Body: "slow", Times: 1})
	t.Cleanup(func() { stand.Scholar().Reset(t) })

	paper, resp := u.AddArxiv(id)
	require.Equal(t, 201, resp.Status, resp)
	time.Sleep(500 * time.Millisecond)
	retried, resp := u.RetryPDF(paper.ID)
	require.Equal(t, 200, resp.Status, resp)
	require.Equal(t, "processing", retried.LatestVersion.Status)
	require.Equal(t, 200, u.WaitForPdfReady(paper.ID, 3*time.Minute).Status)
	time.Sleep(3 * time.Second)
	downloads := 0
	for _, r := range stand.Scholar().Requests(t, "/pdf/"+id) {
		if r.Method == "GET" {
			downloads++
		}
	}
	require.LessOrEqual(t, downloads, 2, "one delayed attempt plus at most one real download, never a second job")
}

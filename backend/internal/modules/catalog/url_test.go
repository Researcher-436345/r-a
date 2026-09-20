package catalog

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/throttle"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestClassifyKnownArticleURL(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		kind  string
		value string
	}{
		{name: "arxiv abstract", url: "https://arxiv.org/abs/2608.12345v2", kind: "arxiv", value: "2608.12345"},
		{name: "arxiv pdf", url: "https://arxiv.org/pdf/2608.12345.pdf", kind: "arxiv", value: "2608.12345"},
		{name: "arxiv html", url: "https://arxiv.org/html/2509.22692v1", kind: "arxiv", value: "2509.22692"},
		{name: "www arxiv", url: "https://www.arxiv.org/abs/2412.09846", kind: "arxiv", value: "2412.09846"},
		{name: "ar5iv", url: "https://ar5iv.labs.arxiv.org/html/2201.09746", kind: "arxiv", value: "2201.09746"},
		{name: "ar5iv old id", url: "https://ar5iv.org/abs/cs/0112017", kind: "arxiv", value: "cs/0112017"},
		{name: "doi", url: "https://doi.org/10.1000/example", kind: "doi", value: "10.1000/example"},
		{name: "pdf", url: "https://papers.example.org/article.pdf?download=1", kind: "pdf", value: "https://papers.example.org/article.pdf?download=1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := classifyKnownArticleURL(test.url)
			if !ok {
				t.Fatal("URL was not classified")
			}
			if got.Kind != test.kind || got.Value != test.value {
				t.Fatalf("got kind=%q value=%q", got.Kind, got.Value)
			}
		})
	}
}

func TestNormalizeArticleURLRemovesTrackingAndFragment(t *testing.T) {
	got, err := normalizeArticleURL(" https://Example.org/paper?utm_source=chat&q=1#results ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://Example.org/paper?q=1" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeArticleURLRejectsPrivateTargets(t *testing.T) {
	for _, raw := range []string{
		"http://localhost/paper.pdf",
		"http://127.0.0.1/paper.pdf",
		"http://169.254.169.254/latest/meta-data",
		"file:///tmp/paper.pdf",
		"https://user:password@example.org/paper.pdf",
	} {
		if _, err := normalizeArticleURL(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

func TestParseArticleHTML(t *testing.T) {
	raw := []byte(`<!doctype html>
<html><head>
  <base href="https://publisher.example.org/articles/42/">
  <meta name="citation_title" content="A Useful Paper">
  <meta name="citation_author" content="Alice Smith">
  <meta name="citation_author" content="Bob Jones">
  <meta name="citation_doi" content="https://doi.org/10.1234/Example.42">
  <meta name="citation_pdf_url" content="paper.pdf">
  <meta name="citation_publication_date" content="2026-08-12">
  <meta name="citation_journal_title" content="Journal of Useful Results">
</head></html>`)

	metadata := parseArticleHTML(raw, "https://publisher.example.org/landing")
	if metadata.Title != "A Useful Paper" {
		t.Fatalf("title=%q", metadata.Title)
	}
	if metadata.DOI != "10.1234/example.42" {
		t.Fatalf("doi=%q", metadata.DOI)
	}
	if metadata.PDFURL != "https://publisher.example.org/articles/42/paper.pdf" {
		t.Fatalf("pdf_url=%q", metadata.PDFURL)
	}
	if len(metadata.Authors) != 2 || metadata.Authors[1] != "Bob Jones" {
		t.Fatalf("authors=%v", metadata.Authors)
	}
	if metadata.Year == nil || *metadata.Year != 2026 {
		t.Fatalf("year=%v", metadata.Year)
	}
}

func TestLinkNamesOtherPaper(t *testing.T) {
	cases := []struct {
		paperTitle, hint string
		want             bool
	}{
		{"Deep Reinforcement Learning: An Overview", "Reinforcement Learning: An Overview", false},
		{"Foundations and Models in Modern Computer Vision: Key Building Blocks", "Foundations and Models in Modern Computer Vision", false},
		{"RL$^2$: Fast Reinforcement Learning via Slow Reinforcement Learning", "RL$^2$: Fast Reinforcement Learning via Slow", false},
		{"Multi-frame image super-resolution with fast upscaling technique", "Attention-based Multi-Reference Learning for Image Super-Resolution", true},
		{"Some arXiv paper", "arXiv 2202.13124", false},
		{"Some arXiv paper", "", false},
		{"Конспект по обучению с подкреплением", "Конспект по обучению с подкреплением", false},
		{"", "Attention-based Multi-Reference Learning for Image Super-Resolution", false},
	}
	for _, c := range cases {
		if got := linkNamesOtherPaper(c.paperTitle, cleanTitleHint(c.hint)); got != c.want {
			t.Errorf("linkNamesOtherPaper(%q, %q) = %v, want %v", c.paperTitle, c.hint, got, c.want)
		}
	}
}

func TestWriteResolveErrorCodes(t *testing.T) {
	cases := []struct {
		err       error
		resolving bool
		status    int
		code      string
	}{
		{errNotAPaper, true, 422, `"code":"not_a_paper"`},
		{fmt.Errorf("%w: DOI names a proceedings", errNotAPaper), false, 422, `"code":"not_a_paper"`},
		{errUnsupportedArticleURL, true, 422, `"code":"not_found"`},
		{&sourceFetchError{message: "Failed to fetch Crossref metadata", err: fmt.Errorf("%w: DOI not found", errUnsupportedArticleURL)}, false, 422, `"code":"not_found"`},
		{&sourceFetchError{message: "Failed to fetch arXiv metadata", err: errors.New("timeout")}, false, 502, ""},
		{errors.New("dial tcp: i/o timeout"), true, 502, ""},
		{errInvalidArticleURL, true, 400, ""},
		{errors.New("database is down"), false, 500, ""},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		writeResolveError(rec, c.err, c.resolving)
		if rec.Code != c.status || (c.code != "" && !strings.Contains(rec.Body.String(), c.code)) {
			t.Errorf("%v: got %d %s, want %d %s", c.err, rec.Code, rec.Body.String(), c.status, c.code)
		}
	}
}

func TestAddFromURLRejectsIndexPagesBeforeAnyWork(t *testing.T) {
	failing := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Errorf("unexpected network call to %s", r.URL)
		return nil, errors.New("network disabled")
	})}
	prevArticle, prevPDF := articleHTTPClient, pdfHTTPClient
	articleHTTPClient, pdfHTTPClient = failing, failing
	t.Cleanup(func() { articleHTTPClient, pdfHTTPClient = prevArticle, prevPDF })

	// No DB, storage or queue: any attempt to create rows would panic.
	api := API{}
	body := `{"url":"https://proceedings.neurips.cc/paper_files/paper/2024","title_hint":"Advances in Neural Information Processing Systems 37","add_to_library":false}`
	req := httptest.NewRequest(http.MethodPost, "/papers/from-url", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	api.addFromURL(rec, req)
	if rec.Code != 422 || !strings.Contains(rec.Body.String(), `"code":"not_a_paper"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

func TestIsPublicIP(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.1.1", "::1", "fc00::1"} {
		if isPublicIP(net.ParseIP(raw)) {
			t.Fatalf("private address %s was allowed", raw)
		}
	}
	for _, raw := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"} {
		if !isPublicIP(net.ParseIP(raw)) {
			t.Fatalf("public address %s was rejected", raw)
		}
	}
}

func TestNormalizeArticleURLRejectsSingleLabelHosts(t *testing.T) {
	for _, raw := range []string{"http://postgres/x", "https://minio:9000/bucket/paper.pdf", "http://catalog/papers"} {
		if _, err := normalizeArticleURL(raw); !errors.Is(err, errInvalidArticleURL) {
			t.Errorf("%s: want errInvalidArticleURL, got %v", raw, err)
		}
	}
	if _, err := normalizeArticleURL("https://arxiv.org/abs/1706.03762"); err != nil {
		t.Fatalf("public host rejected: %v", err)
	}
}

func TestWriteResolveErrorHidesUpstreamDetail(t *testing.T) {
	for _, err := range []error{
		errors.New(`Get "http://nosuch.example/x": lookup nosuch.example on 127.0.0.11:53: no such host`),
		&sourceFetchError{message: "Failed to fetch arXiv metadata", err: errors.New("dial tcp 10.0.0.5:443: connect: refused")},
	} {
		rec := httptest.NewRecorder()
		writeResolveError(rec, err, true)
		if rec.Code != 502 || strings.Contains(rec.Body.String(), "127.0.0.11") || strings.Contains(rec.Body.String(), "10.0.0.5") {
			t.Errorf("%v: got %d %s", err, rec.Code, rec.Body.String())
		}
	}
}

func TestPermanentDownloadFailure(t *testing.T) {
	for message, want := range map[string]bool{
		"Could not download the PDF from example.org: PDF is too large (max 50MB)":         true,
		"Could not download the PDF from example.org: download did not return a PDF":       true,
		"Could not download the PDF from example.org: PDF download returned 403 Forbidden": false,
		"PDF download from example.org timed out":                                          false,
	} {
		if got := permanentDownloadFailure(&message); got != want {
			t.Errorf("%q: got %v, want %v", message, got, want)
		}
	}
	if permanentDownloadFailure(nil) {
		t.Error("nil message is not permanent")
	}
}

func TestResolveEndpointsAreRateLimitedPerUser(t *testing.T) {
	mr := miniredis.RunT(t)
	api := API{Limiter: &throttle.Limiter{Redis: redis.NewClient(&redis.Options{Addr: mr.Addr()})}}
	send := func(user uuid.UUID) *httptest.ResponseRecorder {
		body := `{"url":"https://arxiv.org/list/cs.CV/recent","title_hint":"","add_to_library":false}`
		req := httptest.NewRequest(http.MethodPost, "/papers/from-url", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(identity.WithUserID(req.Context(), user))
		rec := httptest.NewRecorder()
		api.addFromURL(rec, req)
		return rec
	}
	heavy, light := uuid.New(), uuid.New()
	for i := 0; i < ruleResolveUser.Limit; i++ {
		if rec := send(heavy); rec.Code != 422 {
			t.Fatalf("request %d: got %d %s", i, rec.Code, rec.Body.String())
		}
	}
	rec := send(heavy)
	if rec.Code != 429 || !strings.Contains(rec.Body.String(), `"code":"rate_limited"`) || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("over the limit: got %d %s", rec.Code, rec.Body.String())
	}
	if rec = send(light); rec.Code != 422 {
		t.Fatalf("another user is not limited: got %d", rec.Code)
	}
}

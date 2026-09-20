package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// withFakeWeb routes every request of the article and PDF clients to handler,
// so tests can use real-looking public URLs (normalizeArticleURL rejects
// 127.0.0.1) without touching the network.
func withFakeWeb(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	base := srv.Client().Transport
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		clone := r.Clone(r.Context())
		clone.Header.Set("X-Original-Host", r.URL.Host)
		clone.URL.Scheme = target.Scheme
		clone.URL.Host = target.Host
		clone.Host = r.URL.Host
		return base.RoundTrip(clone)
	})}
	prevArticle, prevPDF := articleHTTPClient, pdfHTTPClient
	articleHTTPClient, pdfHTTPClient = client, client
	t.Cleanup(func() { articleHTTPClient, pdfHTTPClient = prevArticle, prevPDF })
}

func servePDF(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/pdf")
	_, _ = w.Write([]byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n1 0 obj\n<<>>\nendobj\n"))
}

func TestProbePDFAcceptsOnlyPDFBytes(t *testing.T) {
	withFakeWeb(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.pdf":
			servePDF(w)
		case "/forbidden.pdf":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("<html>Forbidden</html>"))
		case "/challenge.pdf":
			// PMC answers 200 with an HTML challenge page.
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("<!doctype html><html><body>Checking your browser</body></html>"))
		default:
			http.NotFound(w, r)
		}
	})
	ctx := context.Background()
	if !probePDF(ctx, "https://pub.example/ok.pdf") {
		t.Fatal("a real PDF must be accepted")
	}
	if probePDF(ctx, "https://pub.example/forbidden.pdf") {
		t.Fatal("a 403 must be rejected")
	}
	if probePDF(ctx, "https://pub.example/challenge.pdf") {
		t.Fatal("HTML served as application/pdf must be rejected")
	}
	if probePDF(ctx, "https://pub.example/missing.pdf") {
		t.Fatal("a 404 must be rejected")
	}
}

func TestProbePDFTimesOut(t *testing.T) {
	withFakeWeb(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	})
	prev := pdfProbeTimeout
	pdfProbeTimeout = 150 * time.Millisecond
	t.Cleanup(func() { pdfProbeTimeout = prev })
	start := time.Now()
	if probePDF(context.Background(), "https://slow.example/paper.pdf") {
		t.Fatal("a hanging server must not verify")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("probe ignored its timeout: %v", elapsed)
	}
}

func TestFirstVerifiedPDFKeepsPreferenceOrder(t *testing.T) {
	withFakeWeb(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow-good.pdf":
			time.Sleep(150 * time.Millisecond)
			servePDF(w)
		case "/fast-good.pdf":
			servePDF(w)
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	})
	ctx := context.Background()
	got := firstVerifiedPDF(ctx, []string{"https://a.example/bad.pdf", "https://b.example/slow-good.pdf", "https://c.example/fast-good.pdf"})
	if got != "https://b.example/slow-good.pdf" {
		t.Fatalf("earlier verified candidate must win, got %q", got)
	}
	if got := firstVerifiedPDF(ctx, []string{"https://a.example/bad.pdf", "https://b.example/also-bad.pdf"}); got != "" {
		t.Fatalf("no candidate verified, got %q", got)
	}
	if got := firstVerifiedPDF(ctx, nil); got != "" {
		t.Fatalf("empty list, got %q", got)
	}
}

func TestPDFCandidateListDropsUnfetchable(t *testing.T) {
	got := pdfCandidateList("", "http://127.0.0.1/x.pdf", "https://pub.example/a.pdf", "https://pub.example/a.pdf#page=2", "file:///etc/passwd", "https://pub.example/b.pdf")
	if strings.Join(got, ",") != "https://pub.example/a.pdf,https://pub.example/b.pdf" {
		t.Fatalf("unexpected candidates %v", got)
	}
}

func TestResolveArticleURLVerifiesPDFLinks(t *testing.T) {
	withFakeWeb(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/good.pdf"):
			servePDF(w)
		case r.URL.Path == "/query":
			_, _ = w.Write([]byte(`<feed xmlns="http://www.w3.org/2005/Atom"></feed>`))
		case r.URL.Path == "/works":
			_, _ = w.Write([]byte(`{"results":[]}`))
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	})
	prevArxiv, prevAlex := arxivQueryBaseURL, openAlexBaseURL
	arxivQueryBaseURL, openAlexBaseURL = "https://export.example/query", "https://api.example"
	t.Cleanup(func() { arxivQueryBaseURL, openAlexBaseURL = prevArxiv, prevAlex })

	got, err := resolveArticleURL(context.Background(), "https://openaccess.example/content/x/Ibrahim_3DRRDB_CVPRW_2022_paper/good.pdf", "CVPRW 2022 PDF")
	if err != nil || got.Kind != "pdf" || !got.pdfVerified {
		t.Fatalf("verified PDF expected, got %+v err=%v", got, err)
	}
	if got.Meta.Title == "CVPRW 2022 PDF" {
		t.Fatalf("uninformative link text must not become the title")
	}
	if _, err := resolveArticleURL(context.Background(), "https://downloads.example/journals/cin/2018/7068349.pdf", ""); err == nil {
		t.Fatal("a PDF link answering 403 must not resolve")
	}
}

func TestProbePDFRejectsFilesTheWorkerRefuses(t *testing.T) {
	withFakeWeb(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Length", "60000000")
		_, _ = w.Write([]byte("%PDF-1.7\n"))
	})
	if probePDF(context.Background(), "https://thesis.example.org/huge.pdf") {
		t.Fatal("a PDF over the worker's size limit must not verify")
	}
}

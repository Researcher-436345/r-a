package catalog

import (
	"net"
	"testing"
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

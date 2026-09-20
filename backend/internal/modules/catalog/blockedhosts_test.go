package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestResolveBlockedHostURLReadsDOIFromPath(t *testing.T) {
	failing := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Errorf("unexpected network call to %s", r.URL)
		return nil, errors.New("network disabled")
	})}
	prevArticle, prevPDF := articleHTTPClient, pdfHTTPClient
	articleHTTPClient, pdfHTTPClient = failing, failing
	t.Cleanup(func() { articleHTTPClient, pdfHTTPClient = prevArticle, prevPDF })

	cases := map[string]string{
		"https://dl.acm.org/doi/abs/10.1145/3292500.3330701":             "10.1145/3292500.3330701",
		"https://dl.acm.org/doi/10.1145/3292500.3330701":                 "10.1145/3292500.3330701",
		"https://link.springer.com/article/10.1007/s11263-015-0816-y":    "10.1007/s11263-015-0816-y",
		"https://link.springer.com/chapter/10.1007/978-3-030-58452-8_13": "10.1007/978-3-030-58452-8_13",
	}
	for raw, want := range cases {
		got, err := resolveArticleURL(context.Background(), raw, "")
		if err != nil || got.Kind != "doi" || got.Value != want {
			t.Errorf("%s: got %+v, %v; want doi %s", raw, got, err, want)
		}
	}
}

func TestRefusedSpringerPDFStillOpensByDOI(t *testing.T) {
	withFakeWeb(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	})
	got, err := resolveArticleURL(context.Background(), "https://link.springer.com/content/pdf/10.1007/s11263-015-0816-y.pdf", "")
	if err != nil || got.Kind != "doi" || got.Value != "10.1007/s11263-015-0816-y" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestResolveBlockedHostURLMapsSciencedirectPII(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/works" || r.URL.Query().Get("filter") != "alternative-id:S0893608014002135" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"items":[{"DOI":"10.1016/j.neunet.2014.09.003"}]}}`))
	}))
	t.Cleanup(srv.Close)
	prevBase, prevClient := crossrefBaseURL, articleHTTPClient
	crossrefBaseURL, articleHTTPClient = srv.URL, srv.Client()
	t.Cleanup(func() { crossrefBaseURL, articleHTTPClient = prevBase, prevClient })

	got, err := resolveArticleURL(context.Background(), "https://www.sciencedirect.com/science/article/pii/S0893608014002135", "[5]")
	if err != nil || got.Kind != "doi" || got.Value != "10.1016/j.neunet.2014.09.003" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestSemanticScholarSlugCandidates(t *testing.T) {
	got := semanticScholarSlugCandidates("Deep-Reinforcement-Learning-Li", "Deep Reinforcement Learning")
	want := []slugCandidate{{title: "deep reinforcement learning", surnames: []string{"li"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("with link text: got %+v", got)
	}
	got = semanticScholarSlugCandidates("Deep-Residual-Learning-for-Image-Recognition-He-Zhang", "")
	want = []slugCandidate{
		{title: "deep residual learning for image recognition he", surnames: []string{"zhang"}},
		{title: "deep residual learning for image recognition", surnames: []string{"he", "zhang"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("without link text: got %+v", got)
	}
	if got = semanticScholarSlugCandidates("Li", ""); got != nil {
		t.Fatalf("too short: got %+v", got)
	}
}

const deepRLAtom = `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">
<entry><id>http://arxiv.org/abs/1810.06339v1</id><title>Deep Reinforcement Learning</title>
<published>2018-10-15T00:00:00Z</published><author><name>Yuxi Li</name></author></entry>
</feed>`

func TestSemanticScholarShortTitleResolvesWithAuthor(t *testing.T) {
	withFakeIndexes(t, deepRLAtom, `{"results":[]}`)
	link := "https://www.semanticscholar.org/paper/Deep-Reinforcement-Learning-Li/f2ac2a3f0e4f5d63a4a5c3d3b7bd0e7b1a1e3c44"
	got, err := resolveArticleURL(context.Background(), link, "PDF Deep Reinforcement Learning | Semantic Scholar")
	if err != nil || got.Kind != "arxiv" || got.Value != "1810.06339" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestSemanticScholarShortTitleNeedsMatchingAuthor(t *testing.T) {
	withFakeIndexes(t, strings.Replace(deepRLAtom, "Yuxi Li", "Someone Else", 1), `{"results":[]}`)
	link := "https://www.semanticscholar.org/paper/Deep-Reinforcement-Learning-Li/f2ac2a3f0e4f5d63a4a5c3d3b7bd0e7b1a1e3c44"
	if got, err := resolveArticleURL(context.Background(), link, "Deep Reinforcement Learning"); !errors.Is(err, errUnsupportedArticleURL) {
		t.Fatalf("a namesake by another author must not open: %+v, %v", got, err)
	}
}

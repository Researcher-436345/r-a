package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTitlesMatch(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"Deep Residual Learning for Image Recognition", "Deep residual learning for image recognition.", true},
		{"An Introduction to Convolutional Neural Networks", "An Introduction to Convolutional Neural Networks (Extended Abstract)", true},
		{"Attention Is All You Need", "Attention is all you need", true},
		{"Attention Is All You Need", "All You Need Is Attention", false},
		{"Short", "Short title here", false},
		{"", "Anything", false},
	}
	for _, c := range cases {
		if got := titlesMatch(c.a, c.b); got != c.want {
			t.Errorf("titlesMatch(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestOpenAlexRecordPrefersArxivThenOAPDF(t *testing.T) {
	work := openAlexWork{Title: "Paper"}
	work.IDs.DOI = "https://doi.org/10.1109/CVPR.2016.90"
	work.Locations = []openAlexLocation{
		{LandingPageURL: "https://doi.org/10.1109/cvpr.2016.90"},
		{LandingPageURL: "https://arxiv.org/abs/1512.03385v1", PDFURL: "https://arxiv.org/pdf/1512.03385v1", IsOA: true},
	}
	rec := work.record()
	if rec.ArxivID != "1512.03385" || rec.DOI != "10.1109/cvpr.2016.90" {
		t.Fatalf("unexpected record %+v", rec)
	}

	work.Locations = []openAlexLocation{{LandingPageURL: "https://doi.org/x", PDFURL: "https://pub.example/paper.pdf", IsOA: true}}
	work.BestOA = nil
	rec = work.record()
	if rec.ArxivID != "" || rec.PDFURL != "https://pub.example/paper.pdf" {
		t.Fatalf("expected OA pdf fallback, got %+v", rec)
	}

	work.Locations = []openAlexLocation{{LandingPageURL: "https://doi.org/10.48550/arxiv.1407.6245", IsOA: true}}
	if rec = work.record(); rec.ArxivID != "1407.6245" {
		t.Fatalf("arXiv DataCite DOI must map to the arXiv id, got %+v", rec)
	}
}

func TestOpenAlexPDFCandidatesOrder(t *testing.T) {
	work := openAlexWork{
		PrimaryLocation: &openAlexLocation{LandingPageURL: "http://mipro-proceedings.com/files/upload/sp/sp_008.pdf"},
		BestOA:          &openAlexLocation{PDFURL: "http://downloads.hindawi.com/journals/cin/2018/7068349.pdf", IsOA: true},
		Locations: []openAlexLocation{
			{LandingPageURL: "https://doi.org/10.1155/2018/7068349", PDFURL: "http://downloads.hindawi.com/journals/cin/2018/7068349.pdf", IsOA: true},
			{LandingPageURL: "https://pub.example/landing", PDFURL: "https://pub.example/closed.pdf"},
			{LandingPageURL: "https://www.ncbi.nlm.nih.gov/pmc/articles/4081273", IsOA: true},
		},
	}
	got := strings.Join(work.pdfCandidates(), " ")
	want := "http://downloads.hindawi.com/journals/cin/2018/7068349.pdf https://pub.example/closed.pdf http://mipro-proceedings.com/files/upload/sp/sp_008.pdf"
	if got != want {
		t.Fatalf("candidates:\n got %s\nwant %s", got, want)
	}
	if work.pmcid() != "PMC4081273" {
		t.Fatalf("pmcid from PMC landing page, got %q", work.pmcid())
	}
}

func TestRebuildAbstract(t *testing.T) {
	index := map[string][]int{
		"Abstract": {0}, "The": {1, 5}, "purpose": {2}, "of": {3}, "this": {4}, "paper": {6}, "is": {7},
	}
	if got := rebuildAbstract(index); got != "The purpose of this The paper is" {
		t.Fatalf("got %q", got)
	}
	if got := rebuildAbstract(nil); got != "" {
		t.Fatalf("empty index, got %q", got)
	}
	if got := rebuildAbstract(map[string][]int{"huge": {1 << 30}, "ok": {0}}); got != "ok" {
		t.Fatalf("absurd positions must be ignored, got %q", got)
	}
}

func TestWorkTypeAcceptable(t *testing.T) {
	cases := []struct {
		typ, hint string
		want      bool
	}{
		{"article", "Anything", true},
		{"book", "Computer Vision: Algorithms and Applications", true},
		{"", "Untyped", true},
		{"book-review", "Computer Vision: Algorithms and Applications", false},
		{"paratext", "Advances in Neural Information Processing Systems 36", false},
		{"peer-review", "Some paper", false},
		{"editorial", "Deep Learning for Computer Vision", false},
		{"editorial", "Editorial- Deep Learning for Computer Vision", true},
		{"erratum", "Erratum: A great result", true},
	}
	for _, c := range cases {
		if got := workTypeAcceptable(c.typ, c.hint); got != c.want {
			t.Errorf("workTypeAcceptable(%q, %q) = %v, want %v", c.typ, c.hint, got, c.want)
		}
	}
}

type fakeIndexes struct {
	arxivAtom    string
	openAlexJSON string
	crossrefJSON string
	crossrefCode int
	arxivDelay   time.Duration
	alexDelay    time.Duration
}

func (f fakeIndexes) install(t *testing.T) {
	t.Helper()
	wait := func(r *http.Request, d time.Duration) {
		if d == 0 {
			return
		}
		select {
		case <-r.Context().Done():
		case <-time.After(d):
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/query":
			wait(r, f.arxivDelay)
			w.Header().Set("Content-Type", "application/atom+xml")
			_, _ = w.Write([]byte(f.arxivAtom))
		case strings.HasPrefix(r.URL.Path, "/crossref/"):
			if f.crossrefCode != 0 {
				w.WriteHeader(f.crossrefCode)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(f.crossrefJSON))
		default:
			wait(r, f.alexDelay)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(f.openAlexJSON))
		}
	}))
	t.Cleanup(srv.Close)
	prevArxiv, prevAlex, prevCrossref, prevClient := arxivQueryBaseURL, openAlexBaseURL, crossrefBaseURL, articleHTTPClient
	arxivQueryBaseURL = srv.URL + "/query"
	openAlexBaseURL = srv.URL
	crossrefBaseURL = srv.URL + "/crossref"
	articleHTTPClient = srv.Client()
	t.Cleanup(func() {
		arxivQueryBaseURL, openAlexBaseURL, crossrefBaseURL, articleHTTPClient = prevArxiv, prevAlex, prevCrossref, prevClient
	})
}

func withFakeIndexes(t *testing.T, arxivAtom string, openAlexJSON string) {
	t.Helper()
	fakeIndexes{arxivAtom: arxivAtom, openAlexJSON: openAlexJSON, crossrefJSON: journalArticleJSON}.install(t)
}

const (
	emptyAtom          = `<feed xmlns="http://www.w3.org/2005/Atom"></feed>`
	journalArticleJSON = `{"message":{"type":"journal-article","title":["A paper"]}}`
)

const fakeAtom = `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">
<entry><id>http://arxiv.org/abs/1511.08458v2</id><title>An Introduction to Convolutional Neural Networks</title></entry>
<entry><id>http://arxiv.org/abs/2001.00001v1</id><title>Something unrelated about networks</title></entry>
</feed>`

func TestResolveByTitleFindsArxivByExactTitle(t *testing.T) {
	withFakeIndexes(t, fakeAtom, `{"results":[]}`)
	got, ok := resolveByTitle(context.Background(), "An introduction to convolutional neural networks")
	if !ok || got.Kind != "arxiv" || got.Value != "1511.08458" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestResolveByTitleCleansDecoratedHints(t *testing.T) {
	withFakeIndexes(t, fakeAtom, `{"results":[]}`)
	got, ok := resolveByTitle(context.Background(), "[PDF] An Introduction to Convolutional Neural Networks | Semantic Scholar")
	if !ok || got.Value != "1511.08458" {
		t.Fatalf("decorated hint must still match, got %+v ok=%v", got, ok)
	}
}

func TestResolveByTitleFallsBackToOpenAlexDOI(t *testing.T) {
	withFakeIndexes(t, emptyAtom,
		`{"results":[{"title":"Deep Learning for Computer Vision: A Brief Review","ids":{"doi":"https://doi.org/10.1155/2018/7068349"},"best_oa_location":{"pdf_url":"http://downloads.hindawi.com/journals/cin/2018/7068349.pdf","is_oa":true},"locations":[]}]}`)
	got, ok := resolveByTitle(context.Background(), "Deep Learning for Computer Vision: A Brief Review")
	if !ok || got.Kind != "doi" || got.Value != "10.1155/2018/7068349" || got.Meta.PDFURL == "" || got.Crossref == nil || got.Work == nil {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestResolveByTitleRejectsLooseMatches(t *testing.T) {
	withFakeIndexes(t, emptyAtom,
		`{"results":[{"title":"A completely different survey of vision","ids":{"doi":"https://doi.org/10.1/x"},"locations":[]}]}`)
	if got, ok := resolveByTitle(context.Background(), "Computer Vision: Algorithms and Applications"); ok {
		t.Fatalf("loose search hit must not be accepted: %+v", got)
	}
}

func TestResolveByTitleSkipsBookReviews(t *testing.T) {
	withFakeIndexes(t, emptyAtom, `{"results":[
		{"id":"https://openalex.org/W2125186487","title":"Computer vision: algorithms and applications","type":"book-review","ids":{"doi":"https://doi.org/10.5860/choice.48-5140"}},
		{"id":"https://openalex.org/W1","title":"Computer Vision: Algorithms and Applications","type":"book","ids":{"doi":"https://doi.org/10.1007/978-1-84882-935-0"}}
	]}`)
	got, ok := resolveByTitle(context.Background(), "Computer Vision: Algorithms and Applications")
	if !ok || got.Kind != "doi" || got.Value != "10.1007/978-1-84882-935-0" {
		t.Fatalf("the book, not its review, must be chosen: %+v ok=%v", got, ok)
	}

	withFakeIndexes(t, emptyAtom, `{"results":[
		{"id":"https://openalex.org/W2125186487","title":"Computer vision: algorithms and applications","type":"book-review","ids":{"doi":"https://doi.org/10.5860/choice.48-5140"}}
	]}`)
	if got, ok := resolveByTitle(context.Background(), "Computer Vision: Algorithms and Applications"); ok {
		t.Fatalf("a book review alone must not resolve: %+v", got)
	}
}

func TestResolveByTitleRejectsContainerDOIs(t *testing.T) {
	fakeIndexes{
		arxivAtom:    emptyAtom,
		openAlexJSON: `{"results":[{"id":"https://openalex.org/W7133188547","title":"Advances in Neural Information Processing Systems 36","type":"book","ids":{"doi":"https://doi.org/10.52202/075280"}}]}`,
		crossrefJSON: `{"message":{"type":"proceedings","title":["Advances in Neural Information Processing Systems 36"]}}`,
	}.install(t)
	if got, ok := resolveByTitle(context.Background(), "Advances in Neural Information Processing Systems 36"); ok {
		t.Fatalf("a proceedings volume must never become a paper: %+v", got)
	}
}

const openCVWorkJSON = `{"results":[{"id":"https://openalex.org/W2124828452","title":"A brief introduction to OpenCV","type":"article","publication_year":2012,
	"ids":{"openalex":"https://openalex.org/W2124828452"},"doi":null,
	"authorships":[{"author":{"display_name":"Ivan Čuljak"}},{"author":{"display_name":"David Abram"}}],
	"primary_location":{"is_oa":false,"landing_page_url":"http://mipro-proceedings.com/sites/mipro-proceedings.com/files/upload/sp/sp_008.pdf","pdf_url":null,"source":{"display_name":"International Convention on Information and Communication Technology, Electronics and Microelectronics"}},
	"best_oa_location":null,
	"locations":[{"is_oa":false,"landing_page_url":"http://mipro-proceedings.com/sites/mipro-proceedings.com/files/upload/sp/sp_008.pdf","pdf_url":null}],
	"abstract_inverted_index":{"The":[0],"purpose":[1],"of":[2],"this":[3],"paper":[4],"is":[5],"OpenCV.":[6]}}]}`

func TestResolveByTitleReturnsMetadataOnlyWork(t *testing.T) {
	withFakeIndexes(t, emptyAtom, openCVWorkJSON)
	got, ok := resolveByTitle(context.Background(), "[PDF] A brief introduction to OpenCV | Semantic Scholar")
	if !ok || got.Kind != "openalex" || got.Value != "https://openalex.org/W2124828452" || got.Work == nil {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
	meta := got.Meta
	if meta.Title != "A brief introduction to OpenCV" || meta.Abstract != "The purpose of this paper is OpenCV." ||
		len(meta.Authors) != 2 || meta.Year == nil || *meta.Year != 2012 || !strings.HasPrefix(meta.Venue, "International Convention") {
		t.Fatalf("metadata not taken from OpenAlex: %+v", meta)
	}
	if c := got.Work.pdfCandidates(); len(c) != 1 || !strings.HasSuffix(c[0], "sp_008.pdf") {
		t.Fatalf("closed PDF-looking landing page must be a probe candidate, got %v", c)
	}
}

func TestResolveByTitleKeepsWorkWhenCrossrefIsUnavailable(t *testing.T) {
	fakeIndexes{
		arxivAtom:    emptyAtom,
		openAlexJSON: `{"results":[{"id":"https://openalex.org/W9","title":"Deep Learning for Multiple-Image Super-Resolution","type":"article","ids":{"doi":"https://doi.org/10.9999/datacite.1"}}]}`,
		crossrefCode: http.StatusNotFound,
	}.install(t)
	got, ok := resolveByTitle(context.Background(), "Deep Learning for Multiple-Image Super-Resolution")
	if !ok || got.Kind != "openalex" || got.Meta.DOI != "10.9999/datacite.1" {
		t.Fatalf("a DOI Crossref does not know must fall back to the OpenAlex record: %+v ok=%v", got, ok)
	}
}

func shrinkIndexTimeouts(t *testing.T, call time.Duration) {
	t.Helper()
	prevCall, prevBudget := indexCallTimeout, titleResolveBudget
	indexCallTimeout, titleResolveBudget = call, 3*call
	t.Cleanup(func() { indexCallTimeout, titleResolveBudget = prevCall, prevBudget })
}

func TestResolveByTitleDoesNotWaitForSlowArxiv(t *testing.T) {
	shrinkIndexTimeouts(t, 300*time.Millisecond)
	fakeIndexes{arxivAtom: fakeAtom, openAlexJSON: openCVWorkJSON, arxivDelay: 5 * time.Second}.install(t)
	start := time.Now()
	got, ok := resolveByTitle(context.Background(), "A brief introduction to OpenCV")
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("resolveByTitle waited %v for a hanging arXiv", elapsed)
	}
	if !ok || got.Kind != "openalex" {
		t.Fatalf("OpenAlex result expected after arXiv timeout, got %+v ok=%v", got, ok)
	}
}

func TestResolveByTitlePrefersArxivWithoutWaitingForOpenAlex(t *testing.T) {
	shrinkIndexTimeouts(t, 2*time.Second)
	fakeIndexes{arxivAtom: fakeAtom, openAlexJSON: `{"results":[]}`, alexDelay: 5 * time.Second}.install(t)
	start := time.Now()
	got, ok := resolveByTitle(context.Background(), "An Introduction to Convolutional Neural Networks")
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("an arXiv match must return without waiting for OpenAlex (%v)", elapsed)
	}
	if !ok || got.Kind != "arxiv" || got.Value != "1511.08458" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestResolveByTitleRunsIndexesConcurrently(t *testing.T) {
	shrinkIndexTimeouts(t, 2*time.Second)
	fakeIndexes{arxivAtom: emptyAtom, openAlexJSON: openCVWorkJSON, arxivDelay: 400 * time.Millisecond, alexDelay: 400 * time.Millisecond}.install(t)
	start := time.Now()
	if _, ok := resolveByTitle(context.Background(), "A brief introduction to OpenCV"); !ok {
		t.Fatal("expected a match")
	}
	if elapsed := time.Since(start); elapsed > 700*time.Millisecond {
		t.Fatalf("lookups ran sequentially (%v)", elapsed)
	}
}

func TestHostBlocksScrapers(t *testing.T) {
	if !hostBlocksScrapers("https://www.semanticscholar.org/paper/x/abc") || !hostBlocksScrapers("https://openreview.net/forum?id=1") {
		t.Fatal("known scraper-hostile hosts must be flagged")
	}
	if hostBlocksScrapers("https://aclanthology.org/2020.acl-main.1/") {
		t.Fatal("open venues must not be flagged")
	}
}

func TestTitleIsSpecific(t *testing.T) {
	if titleIsSpecific("Deep learning") || titleIsSpecific("Attention") || titleIsSpecific("A Survey") {
		t.Fatal("generic short titles must not be resolved by name")
	}
	if !titleIsSpecific("Attention Is All You Need") || !titleIsSpecific("Deep Residual Learning for Image Recognition") {
		t.Fatal("real paper titles must pass")
	}
}

func TestFindArxivByTitleSkipsGenericTitles(t *testing.T) {
	withFakeIndexes(t, `<feed xmlns="http://www.w3.org/2005/Atom"><entry><id>http://arxiv.org/abs/1807.07987v2</id><title>Deep Learning</title></entry></feed>`, `{"results":[]}`)
	if id, ok, _ := findArxivByTitle(context.Background(), "Deep learning"); ok {
		t.Fatalf("generic title must not match, got %q", id)
	}
}

func TestTitlesMatchAcceptsDroppedSubtitle(t *testing.T) {
	if !titlesMatch("Foundations and Models in Modern Computer Vision", "Foundations and Models in Modern Computer Vision: Key Building Blocks in Landmark Architectures") {
		t.Fatal("a title without its subtitle must still match")
	}
	if titlesMatch("Foundations and Models in Modern Computer Vision", "Vision Transformers: Foundations and Models in Modern Computer Vision Applications") {
		t.Fatal("containment in the middle of a much longer title must not match")
	}
}

func TestArxivTitleQueryUsesDistinctiveWords(t *testing.T) {
	got := arxivTitleQuery("Foundations and Models in Modern Computer Vision")
	if got != "ti:foundations AND ti:models AND ti:modern AND ti:computer AND ti:vision" {
		t.Fatalf("unexpected query %q", got)
	}
}

func TestTitlesMatchStrict(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"Deep Residual Learning for Image Recognition", "Deep residual learning for image recognition.", true},
		// titlesMatch accepts this prefix; a write to an existing record must not.
		{"Deep Learning for Computer Vision", "Deep Learning for Computer Vision in Agriculture", false},
		{"Attention Is All You Need", "Attention Is All You Need In Speech Separation", false},
		{"A Survey on Deep Learning in Medical Image", "A Survey on Deep Learning in Medical Image: Methods", true},
		{"A Survey on Deep Learning in Medical Image", "A Survey on Deep Learning in Medical Image - Methods and Open Problems Ahead", false},
	}
	for _, c := range cases {
		if got := titlesMatchStrict(c.a, c.b); got != c.want {
			t.Errorf("titlesMatchStrict(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
	if !titlesMatch("Deep Learning for Computer Vision", "Deep Learning for Computer Vision in Agriculture") {
		t.Fatal("precondition: the loose match still accepts the prefix")
	}
}

func TestConfirmsPaper(t *testing.T) {
	year := func(y int) *int { return &y }
	if !confirmsPaper([]string{"Yuxi Li"}, nil, []string{"Li, Yuxi"}, nil) {
		t.Error("shared surname confirms")
	}
	if !confirmsPaper(nil, year(2018), nil, year(2017)) {
		t.Error("a year apart confirms")
	}
	if confirmsPaper([]string{"Ada Lovelace"}, year(2018), []string{"Alan Turing"}, year(2021)) {
		t.Error("other authors, other year must not confirm")
	}
	if confirmsPaper(nil, nil, []string{"Alan Turing"}, year(2021)) {
		t.Error("nothing to compare must not confirm")
	}
}

func TestFindArxivForPaperRejectsLongerNamesake(t *testing.T) {
	atom := `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">
<entry><id>http://arxiv.org/abs/2101.00001v1</id><title>Deep Learning for Computer Vision in Agriculture</title>
<published>2021-01-01T00:00:00Z</published><author><name>Ada Lovelace</name></author></entry>
<entry><id>http://arxiv.org/abs/1702.00002v1</id><title>Deep Learning for Computer Vision</title>
<published>2017-02-01T00:00:00Z</published><author><name>Alan Turing</name></author></entry>
</feed>`
	withFakeIndexes(t, atom, `{"results":[]}`)
	year := 2018
	id, ok := findArxivForPaper(context.Background(), "Deep Learning for Computer Vision", []string{"A. M. Turing"}, &year)
	if !ok || id != "1702.00002" {
		t.Fatalf("got %q %v, want the exact title by the same author", id, ok)
	}
	if id, ok = findArxivForPaper(context.Background(), "Deep Learning for Computer Vision", []string{"Grace Hopper"}, nil); ok {
		t.Fatalf("an unconfirmed candidate was accepted: %q", id)
	}
}

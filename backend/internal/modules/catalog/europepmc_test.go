package catalog

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func jatsFixture(body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE article PUBLIC "-//NLM//DTD JATS (Z39.96) Journal Archiving and Interchange DTD with MathML3 v1.4 20241031//EN" "JATS-archivearticle1-4-mathml3.dtd">
<article article-type="review-article" xmlns:mml="http://www.w3.org/1998/Math/MathML">
<front>
  <journal-meta><journal-title-group><journal-title>Computational Intelligence and Neuroscience</journal-title></journal-title-group></journal-meta>
  <article-meta>
    <article-id pub-id-type="pmcid">PMC5816885</article-id>
    <title-group><article-title>Deep Learning for Computer Vision: A Brief Review</article-title></title-group>
    <contrib-group><contrib><name><surname>Voulodimos</surname><given-names>Athanasios</given-names></name></contrib></contrib-group>
    <abstract><p>Over the last years deep learning methods have been shown to outperform previous techniques.</p></abstract>
    <abstract abstract-type="graphical"><p>Graphical abstract text.</p></abstract>
  </article-meta>
</front>
<body>` + body + `</body>
<back>
  <ack><title>Acknowledgments</title><p>Funded by someone.</p></ack>
  <ref-list><ref id="B1"><element-citation><article-title>A logical calculus</article-title></element-citation></ref></ref-list>
</back>
</article>`
}

func TestJATSToText(t *testing.T) {
	long := strings.Repeat("Deep learning allows computational models of multiple processing layers to learn representations. ", 20)
	body := `<sec id="sec1"><title>1. Introduction</title>
  <p>` + long + ` McCulloch and Pitts [<xref rid="B1" ref-type="bibr">1</xref>] studied neurons &amp; networks.` + "\x00" + `</p>
  <table-wrap id="tab1"><label>Table 1</label><caption><p>Important milestones in the history of neural networks.</p></caption>
    <table><tr><td>Cell soup</td></tr></table></table-wrap>
  <disp-formula id="EEq1"><label>(1)</label><mml:math><mml:mi>y</mml:mi><mml:mo>=</mml:mo><mml:mi>σ</mml:mi></mml:math></disp-formula>
  <sec id="sec1.1"><title>1.1. Convolutional Neural Networks</title>
    <p>CNNs use <inline-formula><mml:math><mml:mi>k</mml:mi></mml:math></inline-formula> kernels.</p>
    <list><list-item><p>Pooling layers</p></list-item><list-item><p>Fully connected layers</p></list-item></list>
    <fig id="fig1"><label>Figure 1</label><caption><p>Example architecture of a CNN.</p></caption><graphic href="fig1.jpg"/></fig>
  </sec>
</sec>
<sec sec-type="supplementary-material"><title>Supplementary Material</title><p>Download me.</p></sec>`
	markdown, plain := jatsToText([]byte(jatsFixture(body)))
	for _, want := range []string{
		"# Deep Learning for Computer Vision: A Brief Review",
		"## Abstract\n\nOver the last years deep learning methods",
		"## 1. Introduction",
		"### 1.1. Convolutional Neural Networks",
		"McCulloch and Pitts [1] studied neurons & networks.",
		"Table 1. Important milestones in the history of neural networks.",
		"Figure 1. Example architecture of a CNN.",
		"- Pooling layers",
		"CNNs use k kernels.",
	} {
		if !strings.Contains(markdown, want) {
			t.Errorf("markdown lacks %q:\n%s", want, markdown)
		}
	}
	for _, unwanted := range []string{"Cell soup", "σ", "A logical calculus", "Funded by someone", "Graphical abstract", "Download me", "Voulodimos", "\x00", "Computational Intelligence and Neuroscience"} {
		if strings.Contains(markdown, unwanted) || strings.Contains(plain, unwanted) {
			t.Errorf("output must not contain %q", unwanted)
		}
	}
	if strings.Contains(plain, "#") {
		t.Errorf("plain text must not carry markdown headings")
	}
	if !strings.Contains(plain, "1. Introduction\n\n") {
		t.Errorf("plain text must keep headings as paragraphs:\n%.200s", plain)
	}
}

func TestJATSToTextRejectsAbstractOnlyRecords(t *testing.T) {
	markdown, plain := jatsToText([]byte(jatsFixture(`<sec><title>Intro</title><p>Short.</p></sec>`)))
	if markdown != "" || plain != "" {
		t.Fatalf("a record without a real body must not be stored, got %q", markdown)
	}
}

func TestNormalizePMCID(t *testing.T) {
	cases := map[string]string{
		"PMC5816885": "PMC5816885",
		"5816885":    "PMC5816885",
		"https://www.ncbi.nlm.nih.gov/pmc/articles/4081273":     "PMC4081273",
		"https://www.ncbi.nlm.nih.gov/pmc/articles/PMC4081273/": "PMC4081273",
		"":                                    "",
		"https://pubmed.ncbi.nlm.nih.gov/abc": "",
	}
	for in, want := range cases {
		if got := normalizePMCID(in); got != want {
			t.Errorf("normalizePMCID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEuropePMCSearchAndFullText(t *testing.T) {
	long := strings.Repeat("Body text of an open access article about computer vision. ", 40)
	withFakeWeb(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/search"):
			if !strings.Contains(r.URL.Query().Get("query"), `DOI:"10.1155/2018/7068349"`) {
				t.Errorf("unexpected query %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"resultList":{"result":[
				{"pmcid":"PMC0000001","doi":"10.9999/other"},
				{"pmcid":"PMC5816885","doi":"10.1155/2018/7068349","isOpenAccess":"Y"}]}}`))
		case strings.HasSuffix(r.URL.Path, "/PMC5816885/fullTextXML"):
			_, _ = w.Write([]byte(jatsFixture(`<sec><title>Introduction</title><p>` + long + `</p></sec>`)))
		default:
			http.NotFound(w, r)
		}
	})
	prev := europePMCBaseURL
	europePMCBaseURL = "https://www.ebi.example/europepmc/webservices/rest"
	t.Cleanup(func() { europePMCBaseURL = prev })

	ctx := context.Background()
	pmcid, err := searchEuropePMCByDOI(ctx, "10.1155/2018/7068349")
	if err != nil || pmcid != "PMC5816885" {
		t.Fatalf("pmcid=%q err=%v", pmcid, err)
	}
	markdown, plain, err := fetchEuropePMCFullText(ctx, pmcid)
	if err != nil || !strings.Contains(markdown, "## Introduction") || !strings.Contains(plain, "Body text of an open access article") {
		t.Fatalf("full text not converted: err=%v markdown=%.200q", err, markdown)
	}
	if _, _, err := fetchEuropePMCFullText(ctx, "PMC1234"); err == nil {
		t.Fatal("a missing full text must report an error")
	}
}

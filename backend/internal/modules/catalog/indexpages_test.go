package catalog

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// Index pages users actually got in chat answers (real-answers matrix).
var nonPaperURLs = []string{
	"https://arxiv.org/archive/cs.CV",
	"https://arxiv.org/list/cs.CV/recent",
	"https://arxiv.org/list/cs.CV/current",
	"https://arxiv.org/list/cs/recent",
	"https://arxiv.org/list/cs.LG/new",
	"https://arxiv.org/list/cs.IR/new",
	"https://arxiv.org/list/cs/new",
	"https://arxiv.org/list/cs.AI/new",
	"https://www.arxiv.org/list/cs.CV/recent?skip=111&show=100",
	"https://arxiv.org/catchup/cs.CV/2026-09-01",
	"https://arxiv.org/year/cs/2023",
	"https://arxiv.org/search/?query=vision&searchtype=all",
	"https://arxiv.org/a/szeliski_r_1",
	"https://arxiv.org/",
	"https://arxiv.org/login",
	"https://arxiv.org/help/api",
	"https://info.arxiv.org/help/find/index.html",
	"https://blog.arxiv.org/2026/",
	"https://blog.arxiv.org/",
	"https://blog.arxiv.org/2025/",
	"https://status.arxiv.org/",
	"https://search.arxiv.org/",
	"https://openreview.net/venues",
	"https://openreview.net/",
	"https://openreview.net/venue?id=ACM.org",
	"https://openreview.net/?id=S1ecm2C9K",
	"https://openreview.net/group?id=ICLR.cc/2024/Conference",
	"https://www.semanticscholar.org/",
	"https://www.semanticscholar.org/topic/Computer-vision/5332",
	"https://www.semanticscholar.org/product",
	"https://www.semanticscholar.org/search?q=opencv",
	"https://www.semanticscholar.org/author/R.-Szeliski/1717841",
	"https://www.sciencedirect.com/",
	"https://www.sciencedirect.com/journal/computer-vision-and-image-understanding/vol/252/suppl/C",
	"https://www.sciencedirect.com/journal/computer-vision-and-image-understanding/vol/256/suppl/C",
	"https://www.sciencedirect.com/browse/journals-and-books",
	"https://proceedings.neurips.cc/paper_files/paper/2024",
	"https://proceedings.neurips.cc/paper/2022",
	"https://proceedings.neurips.cc/paper_files/paper/2023",
	"https://proceedings.neurips.cc/paper/2021",
	"https://papers.nips.cc/paper/2019",
	"https://papers.nips.cc/",
	"https://openaccess.thecvf.com/ICCV2025?day=all",
	"https://openaccess.thecvf.com/menu",
	"https://openaccess.thecvf.com/menu_other.html",
	"https://openaccess.thecvf.com/ICCV2021_workshops/menu",
	"https://aclanthology.org/",
	"https://aclanthology.org/events/acl-2024/",
	"https://aclanthology.org/venues/acl/",
	"https://proceedings.mlr.press/v202/",
	"https://scholar.google.com/scholar?q=opencv",
	"https://pubmed.ncbi.nlm.nih.gov/?term=deep+learning",
	// Root documents the frontend and websearch classifiers also flag.
	"https://ijcai.org/index.php",
	"https://proceedings.mlr.press/index.htm",
	"https://www.jmlr.org/INDEX.HTML",
}

// Paper URLs on the same hosts that must keep resolving.
var paperURLs = []string{
	"https://arxiv.org/abs/1706.06266",
	"http://arxiv.org/abs/2007.03107",
	"https://arxiv.org/html/2509.22692v1",
	"https://www.arxiv.org/abs/2412.09846",
	"http://www.arxiv.org/abs/1607.07680",
	"https://arxiv.org/pdf/2202.13124",
	"https://ar5iv.labs.arxiv.org/html/2201.09746",
	"https://www.semanticscholar.org/paper/A-brief-introduction-to-OpenCV-%C4%8Culjak-Abram/3356363c5857414591b0bf9c20544cc7e88d2cfb",
	"https://openreview.net/forum?id=kAHhFAoZtk",
	"https://openreview.net/pdf?id=kAHhFAoZtk",
	"https://www.sciencedirect.com/science/article/abs/pii/S0031320324006861",
	"https://www.sciencedirect.com/org/science/article/pii/S1526149223001182",
	"https://www.sciencedirect.com/science/article/pii/S1877050920308218/pdf?md5=9ced58aea1b17ed22bcb823e29640ebb&pid=1-s2.0-S1877050920308218-main.pdf",
	"https://openaccess.thecvf.com/content/CVPR2022W/PBVS/papers/Ibrahim_3DRRDB_Super_Resolution_of_Multiple_Remote_Sensing_Images_Using_3D_CVPRW_2022_paper.pdf",
	"https://openaccess.thecvf.com/content_cvpr_2016/html/He_Deep_Residual_Learning_CVPR_2016_paper.html",
	"https://proceedings.neurips.cc/paper_files/paper/2024/hash/0123456789abcdef-Abstract-Conference.html",
	"https://proceedings.neurips.cc/paper/2017/file/3f5ee243547dee91fbd053c1c4a845aa-Paper.pdf",
	"https://papers.nips.cc/paper/7181-attention-is-all-you-need",
	"https://aclanthology.org/2020.acl-main.1/",
	"https://aclanthology.org/P19-1001.pdf",
	"https://doi.org/10.1109/LGRS.2019.2940483",
	"https://ieeexplore.ieee.org/document/10909490/",
	"https://www.nature.com/articles/s41598-025-93049-7",
	"https://proceedings.mlr.press/v202/smith23a.html",
	"https://pubmed.ncbi.nlm.nih.gov/29487619/",
	"https://habr.com/ru/articles/688316/",
	"https://szeliski.org/Book/",
}

func TestIsNonPaperURL(t *testing.T) {
	for _, raw := range nonPaperURLs {
		if !isNonPaperURL(raw) {
			t.Errorf("index page %s was not flagged", raw)
		}
	}
	for _, raw := range paperURLs {
		if isNonPaperURL(raw) {
			t.Errorf("paper URL %s was flagged as an index page", raw)
		}
	}
}

func TestResolveArticleURLRejectsIndexPagesWithoutNetwork(t *testing.T) {
	failing := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		t.Errorf("unexpected network call to %s", r.URL)
		return nil, errors.New("network disabled")
	})}
	prevArticle, prevPDF := articleHTTPClient, pdfHTTPClient
	articleHTTPClient, pdfHTTPClient = failing, failing
	t.Cleanup(func() { articleHTTPClient, pdfHTTPClient = prevArticle, prevPDF })

	for _, raw := range nonPaperURLs {
		if _, err := resolveArticleURL(context.Background(), raw, "Computer Vision and Pattern Recognition"); !errors.Is(err, errNotAPaper) {
			t.Errorf("%s: want errNotAPaper, got %v", raw, err)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

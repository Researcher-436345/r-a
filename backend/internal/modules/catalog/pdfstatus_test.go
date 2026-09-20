package catalog

import (
	"net/http"
	"strings"
	"testing"

	"github.com/centraluniversity/researcher/internal/platform/queue"
	"github.com/google/uuid"
)

func strPtr(s string) *string { return &s }

func TestPDFStateFor(t *testing.T) {
	key := strPtr("papers/x/y.pdf")
	cases := []struct {
		name       string
		v          Version
		wantStatus int
		wantCode   string
		detailHas  string
	}{
		{name: "ready with key", v: Version{Source: "arxiv", Status: "ready", PDFKey: key}, wantStatus: http.StatusOK},
		{name: "processing remote", v: Version{Source: "web_pdf", Status: "processing"}, wantStatus: http.StatusConflict, wantCode: pdfCodeProcessing},
		{name: "processing upload with key", v: Version{Source: "upload", Status: "processing", PDFKey: key}, wantStatus: http.StatusConflict, wantCode: pdfCodeProcessing},
		{name: "failed download", v: Version{Source: "web_pdf", Status: "failed", ErrorMessage: strPtr("PDF download returned 403 Forbidden")}, wantStatus: http.StatusUnprocessableEntity, wantCode: pdfCodeUnavailable, detailHas: "403 Forbidden"},
		{name: "failed without message", v: Version{Source: "arxiv", Status: "failed"}, wantStatus: http.StatusUnprocessableEntity, wantCode: pdfCodeUnavailable, detailHas: "processing failed"},
		{name: "failed even with stale key", v: Version{Source: "upload", Status: "failed", PDFKey: key}, wantStatus: http.StatusUnprocessableEntity, wantCode: pdfCodeUnavailable},
		{name: "doi metadata only", v: Version{Source: "doi", Status: "ready"}, wantStatus: http.StatusUnprocessableEntity, wantCode: pdfCodeUnavailable, detailHas: "open access"},
		{name: "openalex metadata only", v: Version{Source: "openalex", Status: "ready"}, wantStatus: http.StatusUnprocessableEntity, wantCode: pdfCodeUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pdfStateFor(tc.v)
			if got.HTTPStatus != tc.wantStatus || got.Code != tc.wantCode {
				t.Fatalf("pdfStateFor = %d %q, want %d %q", got.HTTPStatus, got.Code, tc.wantStatus, tc.wantCode)
			}
			if tc.wantStatus != http.StatusOK && got.Detail == "" {
				t.Fatal("error states need a detail for the reader")
			}
			if tc.detailHas != "" && !strings.Contains(got.Detail, tc.detailHas) {
				t.Fatalf("detail %q does not mention %q", got.Detail, tc.detailHas)
			}
		})
	}
}

func TestRetryTaskFor(t *testing.T) {
	key := strPtr("papers/x/y.pdf")
	src := strPtr("https://example.org/paper.pdf")
	paperID := uuid.New()
	cases := []struct {
		name       string
		v          Version
		wantTask   string
		wantRefuse bool
	}{
		{name: "ready pdf needs nothing", v: Version{Source: "arxiv", Status: "ready", PDFKey: key, SourceURL: src}},
		{name: "failed web_pdf retries download", v: Version{Source: "web_pdf", Status: "failed", SourceURL: src}, wantTask: queue.ProcessArxivPDF},
		{name: "stuck arxiv processing is requeued", v: Version{Source: "arxiv", Status: "processing", SourceURL: src}, wantTask: queue.ProcessArxivPDF},
		{name: "failed upload refinalizes", v: Version{Source: "upload", Status: "failed", PDFKey: key}, wantTask: queue.FinalizeUploadedPDF},
		{name: "web_pdf without url", v: Version{Source: "web_pdf", Status: "failed"}, wantRefuse: true},
		{name: "upload without key", v: Version{Source: "upload", Status: "failed"}, wantRefuse: true},
		{name: "doi metadata only", v: Version{Source: "doi", Status: "ready", SourceURL: strPtr("https://doi.org/10.1/x")}, wantRefuse: true},
		{name: "openalex metadata only", v: Version{Source: "openalex", Status: "ready", SourceURL: strPtr("https://openalex.org/W1")}, wantRefuse: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.v.PaperID = paperID
			task, refusal := retryTaskFor(tc.v)
			if task != tc.wantTask {
				t.Fatalf("task = %q, want %q", task, tc.wantTask)
			}
			if (refusal != nil) != tc.wantRefuse {
				t.Fatalf("refusal = %+v, want refuse=%v", refusal, tc.wantRefuse)
			}
			if refusal == nil {
				return
			}
			if refusal.HTTPStatus != http.StatusUnprocessableEntity || refusal.Code != pdfCodeUnavailable {
				t.Fatalf("refusal = %d %q, want 422 %q", refusal.HTTPStatus, refusal.Code, pdfCodeUnavailable)
			}
			if !strings.Contains(refusal.Detail, "/papers/"+paperID.String()+"/find-fulltext") {
				t.Fatalf("refusal detail %q must point to find-fulltext", refusal.Detail)
			}
		})
	}
}

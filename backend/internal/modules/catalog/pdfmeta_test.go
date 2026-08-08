package catalog

import (
	"os"
	"testing"
)

func TestDecodeUTF16OctalTitleRejectedAsOutline(t *testing.T) {
	// Outline-style entry must be ignored (/Dest + /Parent).
	pdf := []byte("%PDF-1.4\n17 0 obj\n<< /Title (\\376\\377\\000A\\000b\\000s\\000t\\000r\\000a\\000c\\000t)\n/Dest(section*.1)\n/Parent 16 0 R\n>>\nendobj\n")
	info := ExtractPDFInfo(pdf)
	if info.Title != "" {
		t.Fatalf("outline title should be ignored, got %q", info.Title)
	}
}

func TestInfoTitlePreferred(t *testing.T) {
	pdf := []byte("%PDF-1.4\n" +
		"17 0 obj<< /Title (\\376\\377\\000A\\000b\\000s\\000t\\000r\\000a\\000c\\000t) /Dest(section*.1) /Parent 16 0 R >>endobj\n" +
		"2 0 obj<</Producer(Ghostscript)/CreationDate(D:2023)/Title(arXiv:2306.16004v1  [cs.IR]  28 Jun 2023)>>endobj\n")
	info := ExtractPDFInfo(pdf)
	if info.Title != "arXiv:2306.16004v1 [cs.IR] 28 Jun 2023" {
		t.Fatalf("title=%q", info.Title)
	}
	if info.ArxivID != "2306.16004" {
		t.Fatalf("arxiv=%q", info.ArxivID)
	}
}

func TestDecodeHexUTF16Title(t *testing.T) {
	pdf := []byte("%PDF-1.4\n<< /Producer(x) /Title <FEFF004300610074> >>\n")
	got := ExtractTitleFromPDF(pdf, "fallback")
	if got != "Cat" {
		t.Fatalf("got %q", got)
	}
}

func TestAuthorSplit(t *testing.T) {
	pdf := []byte("%PDF-1.4\n<< /Producer(x) /Title (Real Paper Title) /Author (Alice Smith; Bob Jones) >>\n")
	info := ExtractPDFInfo(pdf)
	if info.Title != "Real Paper Title" {
		t.Fatalf("title=%q", info.Title)
	}
	if len(info.Authors) != 2 || info.Authors[0] != "Alice Smith" || info.Authors[1] != "Bob Jones" {
		t.Fatalf("authors=%v", info.Authors)
	}
}

func TestOldUntitledPDFFixture(t *testing.T) {
	data, err := os.ReadFile("/tmp/old-untitled.pdf")
	if err != nil {
		t.Skip("fixture missing")
	}
	info := ExtractPDFInfo(data)
	if info.ArxivID != "2306.16004" {
		t.Fatalf("arxiv=%q title=%q", info.ArxivID, info.Title)
	}
	if info.Title == "" || info.Title == "Abstract" {
		t.Fatalf("bad title %q", info.Title)
	}
}

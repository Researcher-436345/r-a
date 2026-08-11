package assistant

import (
	"strings"
	"testing"

	"github.com/centraluniversity/researcher/internal/modules/content"
	"github.com/google/uuid"
)

func TestFormatPaperWithPageMarkers(t *testing.T) {
	paperID := uuid.New()
	versionID := uuid.New()
	chunks := []content.Chunk{
		{ID: uuid.New(), PaperID: paperID, VersionID: versionID, ChunkIndex: 0, PageStart: 1, PageEnd: 1, Text: "Intro A"},
		{ID: uuid.New(), PaperID: paperID, VersionID: versionID, ChunkIndex: 1, PageStart: 1, PageEnd: 1, Text: "Intro B"},
		{ID: uuid.New(), PaperID: paperID, VersionID: versionID, ChunkIndex: 2, PageStart: 3, PageEnd: 3, Text: "Methods"},
	}
	out := FormatPaperWithPageMarkers(chunks, "fallback")
	if !strings.Contains(out, "<<<p=1>>>") || !strings.Contains(out, "<<<p=3>>>") {
		t.Fatalf("missing markers: %q", out)
	}
	if strings.Count(out, "<<<p=1>>>") != 1 {
		t.Fatalf("expected one page-1 marker, got %q", out)
	}
	if !strings.Contains(out, "Intro A") || !strings.Contains(out, "Intro B") || !strings.Contains(out, "Methods") {
		t.Fatalf("missing chunk text: %q", out)
	}
}

func TestFormatPaperWithPageMarkersFallback(t *testing.T) {
	if got := FormatPaperWithPageMarkers(nil, "  plain  "); got != "plain" {
		t.Fatalf("got %q", got)
	}
	if got := FormatPaperWithPageMarkers([]content.Chunk{{Text: "   "}}, "plain"); got != "plain" {
		t.Fatalf("empty chunks should fallback, got %q", got)
	}
}

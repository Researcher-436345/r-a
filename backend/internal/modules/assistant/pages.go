package assistant

import (
	"fmt"
	"strings"

	"github.com/centraluniversity/researcher/internal/modules/content"
)

// FormatPaperWithPageMarkers builds LLM context with explicit page anchors.
// Chunks from the PDF parser include page_start; TeX fallback often has page=1 only.
func FormatPaperWithPageMarkers(chunks []content.Chunk, plainFallback string) string {
	plainFallback = strings.TrimSpace(plainFallback)
	if len(chunks) == 0 {
		return plainFallback
	}

	var b strings.Builder
	currentPage := -1
	for _, c := range chunks {
		text := strings.TrimSpace(c.Text)
		if text == "" {
			continue
		}
		page := c.PageStart
		if page < 1 {
			page = 1
		}
		if page != currentPage {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			fmt.Fprintf(&b, "<<<p=%d>>>\n", page)
			currentPage = page
		} else {
			b.WriteString("\n\n")
		}
		b.WriteString(text)
	}
	if b.Len() == 0 {
		return plainFallback
	}
	return b.String()
}

package assistant

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/centraluniversity/researcher/internal/modules/content"
)

var pageMarkerRE = regexp.MustCompile(`<<<p=(\d+)>>>`)

// HasPageStructure reports whether formatted paper text carries more than one
// distinct page marker. The arXiv TeX path puts every chunk on page 1, and a
// model handed a single marker will still invent page numbers — so callers use
// this to turn page citations off rather than emit wrong ones.
func HasPageStructure(paper string) bool {
	seen := make(map[string]struct{}, 2)
	for _, match := range pageMarkerRE.FindAllStringSubmatch(paper, -1) {
		seen[match[1]] = struct{}{}
		if len(seen) > 1 {
			return true
		}
	}
	return false
}

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

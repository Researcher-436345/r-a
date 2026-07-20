package services

import (
	"regexp"
	"strings"
)

var pdfTitleRE = regexp.MustCompile(`(?s)/Title\s*\(([^)]{1,1000})\)`)

func ExtractTitleFromPDF(data []byte, fallback string) string {
	m := pdfTitleRE.FindSubmatch(data)
	if len(m) == 2 {
		if title := strings.TrimSpace(string(m[1])); title != "" {
			return title
		}
	}
	return fallback
}

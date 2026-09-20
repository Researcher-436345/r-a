package catalog

import (
	"regexp"
	"strings"
)

// Server twin of cleanPaperTitle in frontend/src/features/papers/link-kind.ts:
// link text and search-result titles arrive decorated ("[PDF] … | Semantic
// Scholar", "2507.23357 …", "… - arXiv"), and the decoration breaks
// titlesMatch against the indexes.

var (
	hintURLRE          = regexp.MustCompile(`(?i)^(?:[a-z][a-z0-9+.-]*://|www\.)\S*$|^[a-z0-9-]+(?:\.[a-z0-9-]+)+/\S*$`)
	hintBracketTagRE   = regexp.MustCompile(`(?i)^\[(?:pdf|html|book|citation|c)\]\s*`)
	hintPDFPrefixRE    = regexp.MustCompile(`^PDF\s+`)
	hintLeadingIDRE    = regexp.MustCompile(`(?i)^\[?(?:arxiv[:\s]\s*)?(?:\d{4}\.\d{4,5}|[a-z-]+(?:\.[a-z]{2})?/\d{7})(?:v\d+)?\]?(?:\s*[:|,-]\s*|\s+|$)`)
	hintTrailingIDRE   = regexp.MustCompile(`(?i)(?:^|\s*[:|,-]?\s+)\[?(?:arxiv[:\s]\s*)?(?:\d{4}\.\d{4,5}|[a-z-]+(?:\.[a-z]{2})?/\d{7})(?:v\d+)?\]?$`)
	hintDOIOnlyRE      = regexp.MustCompile(`(?i)^(?:doi[:\s]\s*)?10\.\d{4,9}/\S+$`)
	hintSiteSuffixRE   = regexp.MustCompile(`(?i)\s*[|/–—-]\s*(?:semantic\s+scholar|arxiv(?:\.org)?|ar5iv|sciencedirect(?:\.com)?|хабр|habr|openreview|researchgate|springerlink|ieee\s+xplore|acm\s+digital\s+library|pubmed|europe\s+pmc|google\s+scholar)\s*$`)
	hintEllipsisRE     = regexp.MustCompile(`\s*(?:\.{3,}|…)\s*$`)
	hintSeparatorEndRE = regexp.MustCompile(`\s*[|:,–—-]\s*$`)
)

// hintBareWords are what is left of decorations like "arXiv 2202.13124" or
// "CVPRW PDF" once the identifier is gone; they are not titles.
var hintBareWords = map[string]bool{"arxiv": true, "pdf": true, "html": true, "doi": true, "ar5iv": true}

func cleanTitleHint(raw string) string {
	value := strings.Join(strings.Fields(raw), " ")
	if value == "" || hintURLRE.MatchString(value) || hintDOIOnlyRE.MatchString(value) {
		return ""
	}
	for i := 0; i < 6; i++ {
		before := value
		value = hintBracketTagRE.ReplaceAllString(value, "")
		value = hintPDFPrefixRE.ReplaceAllString(value, "")
		value = hintSiteSuffixRE.ReplaceAllString(value, "")
		value = hintEllipsisRE.ReplaceAllString(value, "")
		value = hintLeadingIDRE.ReplaceAllString(value, "")
		value = hintTrailingIDRE.ReplaceAllString(value, "")
		value = hintSeparatorEndRE.ReplaceAllString(value, "")
		value = strings.TrimSpace(value)
		if value == before {
			break
		}
	}
	value = strings.Join(strings.Fields(value), " ")
	if hintBareWords[strings.ToLower(value)] || hintURLRE.MatchString(value) || hintDOIOnlyRE.MatchString(value) {
		return ""
	}
	return value
}

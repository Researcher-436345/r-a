package catalog

import (
	"bytes"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

var (
	pdfInfoTitleRE  = regexp.MustCompile(`(?s)/Title\s*(\((?:\\.|[^\\()])*\)|<[^>]{4,}>)`)
	pdfInfoAuthorRE = regexp.MustCompile(`(?s)/Author\s*(\((?:\\.|[^\\()])*\)|<[^>]{4,}>)`)
	arxivInTextRE   = regexp.MustCompile(`(?i)(?:arxiv:|arxiv\.org/(?:abs|pdf)/)?(\d{4}\.\d{4,5})(?:v\d+)?`)
)

var junkTitles = map[string]bool{
	"abstract": true, "untitled": true, "title": true, "document": true,
	"introduction": true, "references": true, "conclusion": true,
	"microsoft word": true, "powerpoint presentation": true,
}

// ExtractTitleFromPDF reads document Info /Title (not outline bookmarks).
func ExtractTitleFromPDF(data []byte, fallback string) string {
	info := ExtractPDFInfo(data)
	if info.Title != "" {
		return info.Title
	}
	return cleanFilenameFallback(fallback)
}

type PDFInfo struct {
	Title   string
	Authors []string
	ArxivID string
}

func ExtractPDFInfo(data []byte) PDFInfo {
	windows := pdfScanWindows(data)
	var info PDFInfo
	for _, w := range windows {
		if info.Title == "" {
			info.Title = findDocumentTitle(w)
		}
		if len(info.Authors) == 0 {
			info.Authors = findDocumentAuthors(w)
		}
	}
	info.ArxivID = findArxivID(data, info.Title)
	if info.Title != "" {
		info.Title = normalizePDFTitle(info.Title)
	}
	return info
}

func pdfScanWindows(data []byte) [][]byte {
	const head, mid, tail = 1 << 20, 512 << 10, 512 << 10
	if len(data) <= head+tail {
		return [][]byte{data}
	}
	// Info dict is often near the end (before xref); outlines sit mid-file.
	out := [][]byte{data[len(data)-tail:], data[:head]}
	if len(data) > head+tail+mid {
		start := len(data)/2 - mid/2
		out = append(out, data[start:start+mid])
	}
	return out
}

func findDocumentTitle(w []byte) string {
	var best string
	bestScore := -1
	for _, m := range pdfInfoTitleRE.FindAllSubmatchIndex(w, -1) {
		if len(m) < 4 {
			continue
		}
		start, end := m[2], m[3]
		raw := w[start:end]
		// Outline bookmarks look like: /Title (...) /Dest ... /Parent ...
		after := w[end:min(len(w), end+96)]
		if bytes.Contains(after, []byte("/Dest")) || bytes.Contains(after, []byte("/Parent")) || bytes.Contains(after, []byte("/First")) {
			continue
		}
		before := w[max(0, start-120):start]
		score := 0
		for _, marker := range [][]byte{[]byte("/Producer"), []byte("/Creator"), []byte("/CreationDate"), []byte("/ModDate"), []byte("/Author")} {
			if bytes.Contains(before, marker) || bytes.Contains(after, marker) {
				score += 3
			}
		}
		title := normalizePDFTitle(decodePDFString(raw))
		if title == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(title), "arxiv:") {
			score += 2
		}
		score += min(utf8.RuneCountInString(title)/10, 5)
		if score > bestScore {
			bestScore = score
			best = title
		}
	}
	return best
}

func findDocumentAuthors(w []byte) []string {
	for _, m := range pdfInfoAuthorRE.FindAllSubmatchIndex(w, -1) {
		if len(m) < 4 {
			continue
		}
		start, end := m[2], m[3]
		after := w[end:min(len(w), end+96)]
		if bytes.Contains(after, []byte("/Dest")) || bytes.Contains(after, []byte("/Parent")) {
			continue
		}
		authors := splitAuthors(decodePDFString(w[start:end]))
		if len(authors) > 0 {
			return authors
		}
	}
	return nil
}

func findArxivID(data []byte, title string) string {
	if id := firstArxivID([]byte(title)); id != "" {
		return id
	}
	// Header banner often in the first content stream / Info title.
	windows := [][]byte{data[:min(len(data), 80<<10)], data[max(0, len(data)-80<<10):]}
	for _, w := range windows {
		if id := firstArxivID(w); id != "" {
			return id
		}
	}
	return ""
}

func firstArxivID(b []byte) string {
	m := arxivInTextRE.FindSubmatch(b)
	if len(m) < 2 {
		return ""
	}
	return string(m[1])
}

func decodePDFString(raw []byte) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '(':
		if raw[len(raw)-1] != ')' {
			return ""
		}
		return decodePDFLiteral(raw[1 : len(raw)-1])
	case '<':
		if raw[len(raw)-1] != '>' {
			return ""
		}
		return decodePDFHex(raw[1 : len(raw)-1])
	default:
		return ""
	}
}

func decodePDFLiteral(in []byte) string {
	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); i++ {
		if in[i] != '\\' {
			out = append(out, in[i])
			continue
		}
		i++
		if i >= len(in) {
			break
		}
		switch in[i] {
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case '(', ')', '\\':
			out = append(out, in[i])
		case '\n', '\r':
			if in[i] == '\r' && i+1 < len(in) && in[i+1] == '\n' {
				i++
			}
		case '0', '1', '2', '3', '4', '5', '6', '7':
			val := int(in[i] - '0')
			for n := 0; n < 2 && i+1 < len(in) && in[i+1] >= '0' && in[i+1] <= '7'; n++ {
				i++
				val = val*8 + int(in[i]-'0')
			}
			out = append(out, byte(val))
		default:
			out = append(out, in[i])
		}
	}
	return decodePDFTextBytes(out)
}

func decodePDFHex(in []byte) string {
	clean := make([]byte, 0, len(in))
	for _, b := range in {
		if b == ' ' || b == '\n' || b == '\r' || b == '\t' {
			continue
		}
		clean = append(clean, b)
	}
	if len(clean)%2 == 1 {
		clean = append(clean, '0')
	}
	decoded := make([]byte, len(clean)/2)
	if _, err := hex.Decode(decoded, clean); err != nil {
		return ""
	}
	return decodePDFTextBytes(decoded)
}

func decodePDFTextBytes(b []byte) string {
	if len(b) >= 2 {
		if b[0] == 0xFE && b[1] == 0xFF {
			return strings.TrimSpace(utf16BytesToString(b[2:], true))
		}
		if b[0] == 0xFF && b[1] == 0xFE {
			return strings.TrimSpace(utf16BytesToString(b[2:], false))
		}
		if looksLikeUTF16BE(b) {
			return strings.TrimSpace(utf16BytesToString(b, true))
		}
	}
	if !utf8.Valid(b) {
		r := make([]rune, len(b))
		for i, c := range b {
			r[i] = rune(c)
		}
		return strings.TrimSpace(string(r))
	}
	return strings.TrimSpace(string(b))
}

func looksLikeUTF16BE(b []byte) bool {
	if len(b) < 4 || len(b)%2 != 0 {
		return false
	}
	zeros, printable := 0, 0
	for i := 0; i+1 < len(b) && i < 40; i += 2 {
		if b[i] == 0 && b[i+1] != 0 {
			zeros++
			if b[i+1] >= 0x20 && b[i+1] < 0x7f {
				printable++
			}
		}
	}
	return zeros >= 2 && printable >= 2
}

func utf16BytesToString(b []byte, bigEndian bool) string {
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		if bigEndian {
			u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
		} else {
			u = append(u, uint16(b[i+1])<<8|uint16(b[i]))
		}
	}
	return string(utf16.Decode(u))
}

func normalizePDFTitle(title string) string {
	title = strings.Join(strings.Fields(title), " ")
	title = strings.TrimSpace(title)
	if title == "" || !isUsefulTitle(title) {
		return ""
	}
	if utf8.RuneCountInString(title) > 500 {
		r := []rune(title)
		title = string(r[:500])
	}
	return title
}

func isUsefulTitle(title string) bool {
	lower := strings.ToLower(title)
	if junkTitles[lower] {
		return false
	}
	// Section-like outline leftovers: "1 Introduction", "2.1 Soft rewrites"
	if matched, _ := regexp.MatchString(`^\d+(\.\d+)*(\s|$)`, title); matched && utf8.RuneCountInString(title) < 80 {
		rest := regexp.MustCompile(`^\d+(\.\d+)*\s*`).ReplaceAllString(lower, "")
		if junkTitles[rest] || rest == "introduction" || rest == "conclusion" || rest == "references" {
			return false
		}
	}
	printable, letters := 0, 0
	for _, r := range title {
		if unicode.IsPrint(r) {
			printable++
		}
		if unicode.IsLetter(r) {
			letters++
		}
	}
	if printable == 0 || letters < 2 {
		return false
	}
	if strings.Contains(title, `\000`) || strings.Contains(title, `\376`) {
		return false
	}
	return float64(printable)/float64(utf8.RuneCountInString(title)) > 0.8
}

func splitAuthors(raw string) []string {
	raw = strings.Join(strings.Fields(raw), " ")
	if raw == "" {
		return nil
	}
	parts := regexp.MustCompile(`\s*;\s*|\s+and\s+|\s*,\s*`).Split(raw, -1)
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && utf8.RuneCountInString(p) >= 2 {
			out = append(out, p)
		}
	}
	return out
}

func cleanFilenameFallback(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "Untitled PDF"
	}
	return name
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

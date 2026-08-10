package content

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	texIncludeRE    = regexp.MustCompile(`\\includegraphics(?:\[[^\]]*\])?\{[^}]*\}`)
	texCiteRE       = regexp.MustCompile(`\\(?:cite|citep|ref|label|eqref|pageref)\*?\{[^}]*\}`)
	texBeginEndRE   = regexp.MustCompile(`\\(?:begin|end)\{[^}]+\}`)
	texWhitespaceRE = regexp.MustCompile(`[ \t]+\n`)
	texBlankRE      = regexp.MustCompile(`\n{3,}`)
	sectionRE       = regexp.MustCompile(`\\((?:sub)*section|chapter|title)\*?\{([^{}]*)\}`)
)

type TexResult struct {
	PlainText string
	Markdown  string
	Engine    string
	PageCount int
	Warnings  []string
}

// TryArxivTeX downloads arXiv e-print source and extracts readable text.
// Returns ok=false when source is missing, PDF-only, or too short.
func TryArxivTeX(ctx context.Context, arxivID string) (TexResult, bool, error) {
	id := strings.TrimSpace(arxivID)
	if id == "" {
		return TexResult{}, false, nil
	}
	raw, err := downloadEPrint(ctx, id)
	if err != nil {
		return TexResult{Warnings: []string{err.Error()}}, false, nil
	}
	if len(raw) >= 4 && string(raw[:4]) == "%PDF" {
		return TexResult{Warnings: []string{"e-print is PDF-only"}}, false, nil
	}
	files, err := extractTexFiles(raw)
	if err != nil || len(files) == 0 {
		msg := "no .tex files in e-print"
		if err != nil {
			msg = err.Error()
		}
		return TexResult{Warnings: []string{msg}}, false, nil
	}
	combined := pickAndJoinTeX(files)
	if strings.TrimSpace(combined) == "" {
		return TexResult{}, false, nil
	}
	plain := CleanTeX(combined)
	if utf8.RuneCountInString(plain) < 400 {
		return TexResult{Warnings: []string{"tex yield too short"}}, false, nil
	}
	return TexResult{
		PlainText: plain,
		Markdown:  plain,
		Engine:    "arxiv_tex",
		PageCount: 0,
	}, true, nil
}

func downloadEPrint(ctx context.Context, arxivID string) ([]byte, error) {
	urls := []string{
		"https://export.arxiv.org/e-print/" + arxivID,
		"https://arxiv.org/e-print/" + arxivID,
	}
	client := &http.Client{Timeout: 90 * time.Second}
	var last error
	for _, u := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			last = err
			continue
		}
		req.Header.Set("User-Agent", "Researcher/1.0 (paper-library)")
		resp, err := client.Do(req)
		if err != nil {
			last = err
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 40<<20))
		resp.Body.Close()
		if readErr != nil {
			last = readErr
			continue
		}
		if resp.StatusCode/100 != 2 {
			last = fmt.Errorf("e-print %s", resp.Status)
			continue
		}
		return data, nil
	}
	if last == nil {
		last = fmt.Errorf("e-print unavailable")
	}
	return nil, last
}

func extractTexFiles(raw []byte) (map[string]string, error) {
	out := map[string]string{}

	payload := raw
	if gr, err := gzip.NewReader(bytes.NewReader(raw)); err == nil {
		ungzipped, err := io.ReadAll(io.LimitReader(gr, 80<<20))
		_ = gr.Close()
		if err != nil {
			return nil, err
		}
		payload = ungzipped
	}

	tr := tar.NewReader(bytes.NewReader(payload))
	tarOK := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			if !tarOK {
				break
			}
			return nil, err
		}
		tarOK = true
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		name := path.Clean(hdr.Name)
		if !strings.HasSuffix(strings.ToLower(name), ".tex") {
			continue
		}
		if hdr.Size > 8<<20 {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(tr, 8<<20))
		if err != nil || !utf8.Valid(body) {
			continue
		}
		out[name] = string(body)
	}
	if tarOK {
		return out, nil
	}

	if utf8.Valid(payload) {
		s := string(payload)
		if strings.Contains(s, `\documentclass`) || strings.Contains(s, `\begin{document}`) || strings.Contains(s, `\section`) {
			out["main.tex"] = s
		}
	}
	return out, nil
}

func pickAndJoinTeX(files map[string]string) string {
	type scored struct {
		name  string
		score int
		body  string
	}
	list := make([]scored, 0, len(files))
	for name, body := range files {
		s := 0
		lower := strings.ToLower(name)
		base := path.Base(lower)
		if strings.Contains(body, `\begin{document}`) {
			s += 50
		}
		if strings.Contains(body, `\documentclass`) {
			s += 40
		}
		if base == "main.tex" || base == "paper.tex" || base == "ms.tex" {
			s += 30
		}
		if strings.Contains(lower, "suppl") || strings.Contains(lower, "appendix") {
			s -= 10
		}
		s += len(body) / 5000
		list = append(list, scored{name: name, score: s, body: body})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].score == list[j].score {
			return list[i].name < list[j].name
		}
		return list[i].score > list[j].score
	})
	limit := 3
	if len(list) < limit {
		limit = len(list)
	}
	var b strings.Builder
	for i := 0; i < limit; i++ {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(list[i].body)
	}
	return b.String()
}

// CleanTeX turns LaTeX source into LLM-friendly text while keeping math delimiters.
func CleanTeX(src string) string {
	s := src
	if i := strings.Index(s, `\begin{document}`); i >= 0 {
		s = s[i+len(`\begin{document}`):]
	}
	if j := strings.Index(s, `\end{document}`); j >= 0 {
		s = s[:j]
	}
	s = stripTeXComments(s)
	s = texIncludeRE.ReplaceAllString(s, "")
	s = texCiteRE.ReplaceAllString(s, "")

	s = sectionRE.ReplaceAllStringFunc(s, func(m string) string {
		parts := sectionRE.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}
		kind, title := parts[1], strings.TrimSpace(parts[2])
		switch kind {
		case "title":
			return "\n\n# " + title + "\n\n"
		case "chapter", "section":
			return "\n\n## " + title + "\n\n"
		case "subsection":
			return "\n\n### " + title + "\n\n"
		default:
			return "\n\n#### " + title + "\n\n"
		}
	})

	s = texBeginEndRE.ReplaceAllString(s, "\n")
	s = replaceCommandsKeepArgs(s)
	s = strings.ReplaceAll(s, "~", " ")
	s = strings.ReplaceAll(s, `\,`, " ")
	s = strings.ReplaceAll(s, `\;`, " ")
	s = strings.ReplaceAll(s, `\!`, "")
	s = strings.ReplaceAll(s, `\&`, "&")
	s = strings.ReplaceAll(s, `\%`, "%")
	s = strings.ReplaceAll(s, `\_`, "_")
	s = strings.ReplaceAll(s, `\{`, "{")
	s = strings.ReplaceAll(s, `\}`, "}")
	s = texWhitespaceRE.ReplaceAllString(s, "\n")
	s = texBlankRE.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func stripTeXComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '%' && (i == 0 || s[i-1] != '\\') {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func replaceCommandsKeepArgs(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] != '\\' {
			b.WriteByte(s[i])
			i++
			continue
		}
		if i+1 < len(s) && strings.ContainsRune("$&%#_~^{}", rune(s[i+1])) {
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		j := i + 1
		for j < len(s) && ((s[j] >= 'a' && s[j] <= 'z') || (s[j] >= 'A' && s[j] <= 'Z')) {
			j++
		}
		if j < len(s) && s[j] == '*' {
			j++
		}
		if j == i+1 {
			// unknown short escape — drop slash, keep next char
			if j < len(s) {
				b.WriteByte(s[j])
				i = j + 1
			} else {
				i = j
			}
			continue
		}
		k := j
		for k < len(s) && (s[k] == '[' || s[k] == '{') {
			open := s[k]
			closeCh := byte(']')
			if open == '{' {
				closeCh = '}'
			}
			k++
			start := k
			depth := 1
			for k < len(s) && depth > 0 {
				if s[k] == open {
					depth++
				} else if s[k] == closeCh {
					depth--
				}
				k++
			}
			if closeCh == '}' && start < k-1 {
				b.WriteString(s[start : k-1])
				b.WriteByte(' ')
			}
		}
		i = k
	}
	return b.String()
}

func ChunkPlainText(text string, targetTokens int) []ParseChunk {
	if targetTokens < 200 {
		targetTokens = 1000
	}
	paras := strings.Split(text, "\n\n")
	var chunks []ParseChunk
	var buf []string
	est := 0
	flush := func() {
		body := strings.TrimSpace(strings.Join(buf, "\n\n"))
		if body == "" {
			buf = nil
			est = 0
			return
		}
		idx := len(chunks)
		chunks = append(chunks, ParseChunk{
			ID:            fmt.Sprintf("c%04d", idx+1),
			PageStart:     1,
			PageEnd:       1,
			Section:       fmt.Sprintf("chunk %d", idx+1),
			Text:          body,
			TokenEstimate: (utf8.RuneCountInString(body) + 3) / 4,
		})
		buf = nil
		est = 0
	}
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		t := (utf8.RuneCountInString(p) + 3) / 4
		if len(buf) > 0 && est+t > targetTokens {
			flush()
		}
		buf = append(buf, p)
		est += t
	}
	flush()
	return chunks
}

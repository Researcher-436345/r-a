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
	// \input{file}, \include{file}, \subfile{file} — arXiv sources are routinely
	// split into sections/*.tex, so the main file alone is just a table of contents.
	texInputRE = regexp.MustCompile(`\\(?:input|include|subfile)\s*\{\s*([^{}]+?)\s*\}`)
)

// texMinYieldRunes is the smallest plausible paper body. Anything shorter means
// the source was not really extracted (unresolved inputs, a wrapper file, a
// PDF-only submission) and the PDF parser must take over.
const texMinYieldRunes = 1500

// texMaxInputDepth caps \input recursion (cycles are also tracked explicitly).
const texMaxInputDepth = 12

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
	res, ok := ExtractTeXFromEPrint(raw)
	return res, ok, nil
}

// ExtractTeXFromEPrint turns a downloaded e-print (tar.gz, gz or a bare .tex)
// into readable text. ok=false when there is nothing usable in it.
func ExtractTeXFromEPrint(raw []byte) (TexResult, bool) {
	files, err := extractTexFiles(raw)
	if err != nil || len(files) == 0 {
		msg := "no .tex files in e-print"
		if err != nil {
			msg = err.Error()
		}
		return TexResult{Warnings: []string{msg}}, false
	}
	combined := pickAndJoinTeX(files)
	if strings.TrimSpace(combined) == "" {
		return TexResult{}, false
	}
	plain := CleanTeX(combined)
	if utf8.RuneCountInString(plain) < texMinYieldRunes {
		return TexResult{Warnings: []string{fmt.Sprintf("tex yield too short: %d runes", utf8.RuneCountInString(plain))}}, false
	}
	return TexResult{
		PlainText: plain,
		Markdown:  plain,
		Engine:    "arxiv_tex",
		PageCount: 0,
	}, true
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
	if len(list) == 0 {
		return ""
	}

	// A real document root pulls the rest in through \input, so expanding it
	// alone yields the whole paper in reading order. Joining several files on
	// top of that would duplicate every section.
	if strings.Contains(list[0].body, `\begin{document}`) {
		visited := map[string]bool{list[0].name: true}
		return expandTeXInputs(files, list[0].name, list[0].body, visited, 0)
	}

	// No root: take the best few files, each with its own inputs resolved.
	limit := 3
	if len(list) < limit {
		limit = len(list)
	}
	visited := map[string]bool{}
	var b strings.Builder
	for i := 0; i < limit; i++ {
		if visited[list[i].name] {
			continue
		}
		visited[list[i].name] = true
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(expandTeXInputs(files, list[i].name, list[i].body, visited, 0))
	}
	return b.String()
}

// expandTeXInputs replaces \input{x} / \include{x} / \subfile{x} with the body
// of x (resolved relative to the including file, then to the archive root),
// recursively. Unresolved or already-visited inputs are dropped so their file
// names do not leak into the text as if they were prose.
func expandTeXInputs(files map[string]string, name, body string, visited map[string]bool, depth int) string {
	if depth >= texMaxInputDepth {
		return texInputRE.ReplaceAllString(body, "")
	}
	dir := path.Dir(name)
	return texInputRE.ReplaceAllStringFunc(body, func(m string) string {
		parts := texInputRE.FindStringSubmatch(m)
		if len(parts) < 2 {
			return ""
		}
		target, ok := resolveTeXInput(files, dir, parts[1])
		if !ok || visited[target] {
			return ""
		}
		visited[target] = true
		return "\n" + expandTeXInputs(files, target, files[target], visited, depth+1) + "\n"
	})
}

// resolveTeXInput finds the archive entry an \input argument refers to.
func resolveTeXInput(files map[string]string, dir, arg string) (string, bool) {
	arg = strings.Trim(strings.TrimSpace(arg), `"`)
	if arg == "" {
		return "", false
	}
	bases := []string{arg}
	if !strings.HasSuffix(strings.ToLower(arg), ".tex") {
		bases = append(bases, arg+".tex")
	}
	for _, base := range bases {
		for _, candidate := range []string{path.Join(dir, base), path.Clean(base)} {
			if _, ok := files[candidate]; ok {
				return candidate, true
			}
		}
	}
	return "", false
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
	s = texInputRE.ReplaceAllString(s, "")
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

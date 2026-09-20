package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Europe PMC's REST API is the one full-text source for biomedical open access
// that answers from the server: its PDF renderer and PMC itself return 403 or
// a browser challenge, but fullTextXML serves the JATS article.

var (
	europePMCBaseURL = "https://www.ebi.ac.uk/europepmc/webservices/rest"
	europePMCTimeout = 15 * time.Second
	pmcidRE          = regexp.MustCompile(`(?i)(?:^|/|pmc)(\d{3,10})/?$`)
)

const (
	europePMCEngine       = "europepmc_jats"
	europePMCMaxXMLBytes  = 12 << 20
	europePMCMaxTextRunes = 800_000
	// Below this the XML carried no body (abstract-only records).
	europePMCMinBodyRunes = 1500
)

func normalizePMCID(raw string) string {
	m := pmcidRE.FindStringSubmatch(strings.TrimSpace(raw))
	if len(m) < 2 {
		return ""
	}
	return "PMC" + m[1]
}

func europePMCGet(ctx context.Context, endpoint string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "researcher-api/0.1 (mailto:"+openAlexMailto+")")
	resp, err := articleHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errUnsupportedArticleURL
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Europe PMC returned %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// searchEuropePMCByDOI returns the PMCID of the record with exactly this DOI.
func searchEuropePMCByDOI(ctx context.Context, doi string) (string, error) {
	endpoint := europePMCBaseURL + "/search?query=" + url.QueryEscape(`DOI:"`+doi+`"`) +
		"&format=json&resultType=lite&pageSize=3"
	raw, err := europePMCGet(ctx, endpoint, 1<<20)
	if err != nil {
		return "", err
	}
	var out struct {
		ResultList struct {
			Result []struct {
				PMCID string `json:"pmcid"`
				DOI   string `json:"doi"`
			} `json:"result"`
		} `json:"resultList"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	for _, r := range out.ResultList.Result {
		if strings.EqualFold(strings.TrimSpace(r.DOI), doi) {
			if pmcid := normalizePMCID(r.PMCID); pmcid != "" {
				return pmcid, nil
			}
		}
	}
	return "", nil
}

// fetchEuropePMCFullText returns the article as markdown and plain text, or
// empty strings when Europe PMC has no body for it.
func fetchEuropePMCFullText(ctx context.Context, pmcid string) (string, string, error) {
	pmcid = normalizePMCID(pmcid)
	if pmcid == "" {
		return "", "", nil
	}
	raw, err := europePMCGet(ctx, europePMCBaseURL+"/"+pmcid+"/fullTextXML", europePMCMaxXMLBytes)
	if err != nil {
		return "", "", err
	}
	markdown, plain := jatsToText(raw)
	return markdown, plain, nil
}

// JATS elements whose content is not prose: formulas render as MathML noise,
// tables as cell soup, and references would drown the body in citations.
var jatsSkipElements = map[string]bool{
	"disp-formula": true, "tex-math": true, "annotation": true, "table": true, "graphic": true,
	"media": true, "supplementary-material": true, "ref-list": true, "fn-group": true,
	"object-id": true, "inline-graphic": true, "chem-struct-wrap": true, "code": true,
}

type jatsState struct {
	md, plain   strings.Builder
	buf         strings.Builder
	title       strings.Builder
	runes       int
	bodyRunes   int
	skipDepth   int
	secDepth    int
	floatDepth  int // inside fig / table-wrap: only label and caption count
	captionIn   int
	floatLabel  string
	inTitle     bool
	headingNext int
	region      string // "", "title", "abstract", "body"
	abstractSet bool
	full        bool
	bulletNext  bool // list items wrap their text in <p>, so the marker waits
}

func (s *jatsState) emit(text string, heading int) {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" || s.full {
		return
	}
	if heading == 0 && s.bulletNext {
		text = "- " + text
		s.bulletNext = false
	}
	n := utf8.RuneCountInString(text)
	if s.runes+n > europePMCMaxTextRunes {
		s.full = true
		return
	}
	s.runes += n
	if s.region == "body" && heading == 0 {
		s.bodyRunes += n
	}
	if s.md.Len() > 0 {
		s.md.WriteString("\n\n")
		s.plain.WriteString("\n\n")
	}
	if heading > 0 {
		s.md.WriteString(strings.Repeat("#", heading) + " ")
	}
	s.md.WriteString(text)
	s.plain.WriteString(text)
}

func (s *jatsState) flush() {
	text := s.buf.String()
	s.buf.Reset()
	heading := 0
	if s.inTitle {
		heading = s.headingNext
	}
	s.emit(text, heading)
}

func jatsToText(raw []byte) (string, string) {
	// encoding/xml stops at the first control character, which would drop
	// everything after it.
	raw = bytes.Map(func(r rune) rune {
		if r == utf8.RuneError || (r < 0x20 && r != '\t' && r != '\n' && r != '\r') || r == 0xFFFE || r == 0xFFFF {
			return -1
		}
		return r
	}, raw)
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity
	var s jatsState
	var stack []string
	parent := func(back int) string {
		if len(stack) > back {
			return stack[len(stack)-1-back]
		}
		return ""
	}
	for !s.full {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			stack = append(stack, name)
			if s.skipDepth > 0 {
				s.skipDepth++
				continue
			}
			switch {
			case name == "article-title" && parent(1) == "title-group" && parent(2) == "article-meta" && s.title.Len() == 0:
				s.region = "title"
			case name == "abstract" && parent(1) == "article-meta":
				if s.abstractSet || attrValue(t, "abstract-type") != "" {
					s.skipDepth = 1
					continue
				}
				s.abstractSet = true
				s.region = "abstract"
				s.emit("Abstract", 2)
			case name == "body" && s.region == "":
				s.region = "body"
			case s.region == "abstract" || s.region == "body":
				s.startBlock(name, t)
			}
		case xml.EndElement:
			if len(stack) == 0 {
				continue
			}
			name := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if s.skipDepth > 0 {
				s.skipDepth--
				continue
			}
			switch {
			case s.region == "title" && name == "article-title":
				s.region = ""
				s.emit(s.title.String(), 1)
			case s.region == "abstract" && name == "abstract", s.region == "body" && name == "body":
				s.flush()
				s.region = ""
			case s.region == "abstract" || s.region == "body":
				s.endBlock(name)
			}
		case xml.CharData:
			if s.skipDepth > 0 {
				continue
			}
			switch s.region {
			case "title":
				s.title.Write(t)
			case "abstract", "body":
				if s.floatDepth > 0 && s.captionIn == 0 && parent(0) != "label" {
					continue
				}
				if s.floatDepth > 0 && s.captionIn == 0 {
					s.floatLabel += string(t)
					continue
				}
				s.buf.Write(t)
			}
		}
	}
	if s.bodyRunes < europePMCMinBodyRunes {
		return "", ""
	}
	return cleanStoredText(s.md.String()), cleanStoredText(s.plain.String())
}

func (s *jatsState) startBlock(name string, t xml.StartElement) {
	if jatsSkipElements[name] || (name == "sec" && isSkippedSecType(attrValue(t, "sec-type"))) {
		s.skipDepth = 1
		return
	}
	switch name {
	case "fig", "table-wrap":
		s.flush()
		s.floatDepth++
		if s.floatDepth == 1 {
			s.floatLabel = ""
		}
	case "caption":
		if s.floatDepth > 0 {
			s.captionIn++
		}
	case "sec", "app":
		s.flush()
		s.secDepth++
	case "title":
		s.flush()
		s.inTitle = true
		s.headingNext = min(2+s.secDepth-1, 4)
		if s.region == "abstract" || s.secDepth == 0 {
			s.headingNext = 3
		}
	case "p", "list-item", "disp-quote", "boxed-text", "def-item", "statement":
		s.flush()
		if name == "list-item" {
			s.bulletNext = true
		}
	}
}

func (s *jatsState) endBlock(name string) {
	switch name {
	case "fig", "table-wrap":
		if s.floatDepth > 0 {
			s.floatDepth--
		}
	case "caption":
		if s.captionIn > 0 {
			label := strings.TrimSpace(strings.Join(strings.Fields(s.floatLabel), " "))
			caption := s.buf.String()
			s.buf.Reset()
			if label != "" {
				caption = label + ". " + caption
			}
			s.captionIn--
			s.emit(caption, 0)
		}
	case "sec", "app":
		s.flush()
		if s.secDepth > 0 {
			s.secDepth--
		}
	case "title":
		s.flush()
		s.inTitle = false
	case "p", "list-item", "disp-quote", "boxed-text", "def-item", "statement":
		if s.captionIn == 0 {
			s.flush()
		}
	}
}

func isSkippedSecType(secType string) bool {
	switch strings.ToLower(secType) {
	case "supplementary-material", "display-objects", "data-availability":
		return true
	}
	return false
}

func attrValue(t xml.StartElement, name string) string {
	for _, a := range t.Attr {
		if strings.EqualFold(a.Name.Local, name) {
			return a.Value
		}
	}
	return ""
}

// cleanStoredText removes what Postgres text columns reject (NUL bytes,
// invalid UTF-8): one such byte kept a parse task retrying forever.
func cleanStoredText(s string) string {
	s = strings.ToValidUTF8(s, "")
	return strings.ReplaceAll(s, "\x00", "")
}

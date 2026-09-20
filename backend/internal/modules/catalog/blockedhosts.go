package catalog

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

// Sites in hostBlocksScrapers never show us their page, but many of their URLs
// name the work: a DOI in the path (ACM, Springer), an Elsevier PII
// (ScienceDirect) or a title slug with author surnames (Semantic Scholar).

var (
	// dl.acm.org/doi/abs/10.1145/…, link.springer.com/article/10.1007/…, …/content/pdf/10.1007/….pdf
	pathDOIRE          = regexp.MustCompile(`(?i)^/(?:doi(?:/(?:abs|full|pdf|epdf|fullhtml))?|article|chapter|book|referenceworkentry|protocol|content/pdf)/(10\.\d{4,9}/[^?#]+?)(?:\.pdf)?/?$`)
	sciencedirectPIIRE = regexp.MustCompile(`(?i)^/science/article/(?:abs/)?pii/([a-z0-9]{16,20})/?$`)
	s2PaperSlugRE      = regexp.MustCompile(`^/paper/([^/]+)/[0-9a-fA-F]{40}/?$`)
)

// resolveBlockedHostURL reads an identifier from the URL of a scraper-blocking
// site. When none resolves it returns the best title for the title fallback:
// the link text, or the Semantic Scholar slug title when the text is empty.
func resolveBlockedHostURL(ctx context.Context, normalized, hint string) (resolvedArticle, bool, string) {
	parsed, err := url.Parse(normalized)
	if err != nil {
		return resolvedArticle{}, false, hint
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	switch host {
	case "dl.acm.org", "link.springer.com":
		if m := pathDOIRE.FindStringSubmatch(parsed.Path); m != nil {
			if doi, doiErr := NormalizeDOI(m[1]); doiErr == nil {
				return resolvedArticle{Kind: "doi", Value: doi, Meta: articleMetadata{Title: hint}}, true, hint
			}
		}
	case "sciencedirect.com":
		if m := sciencedirectPIIRE.FindStringSubmatch(parsed.Path); m != nil {
			callCtx, stop := context.WithTimeout(ctx, crossrefCallTimeout)
			doi, lookupErr := lookupDOIByPII(callCtx, m[1])
			stop()
			if lookupErr == nil {
				return resolvedArticle{Kind: "doi", Value: doi, Meta: articleMetadata{Title: hint}}, true, hint
			}
		}
	case "semanticscholar.org":
		m := s2PaperSlugRE.FindStringSubmatch(parsed.Path)
		if m == nil {
			break
		}
		fallback := hint
		for _, candidate := range semanticScholarSlugCandidates(m[1], hint) {
			if titleIsSpecific(candidate.title) {
				// A specific title needs no author check; the title fallback runs it.
				if !titleIsSpecific(fallback) {
					fallback = candidate.title
				}
				continue
			}
			if resolved, ok := resolveByTitleAndAuthors(ctx, candidate.title, candidate.surnames); ok {
				return resolved, true, fallback
			}
		}
		return resolvedArticle{}, false, fallback
	}
	return resolvedArticle{}, false, hint
}

type slugCandidate struct {
	title    string
	surnames []string
}

// semanticScholarSlugCandidates splits "Deep-Reinforcement-Learning-Li" into
// title and one or two trailing author surnames. The link text, when it is
// the start of the slug, tells exactly where the title ends.
func semanticScholarSlugCandidates(slug, hint string) []slugCandidate {
	if unescaped, err := url.PathUnescape(slug); err == nil {
		slug = unescaped
	}
	words := strings.Fields(normalizeTitle(strings.ReplaceAll(slug, "-", " ")))
	if len(words) < 3 {
		return nil
	}
	if titleWords := strings.Fields(normalizeTitle(hint)); len(titleWords) > 0 && len(titleWords) < len(words) {
		if strings.Join(words[:len(titleWords)], " ") == strings.Join(titleWords, " ") {
			if rest := words[len(titleWords):]; len(rest) <= 2 {
				return []slugCandidate{{title: strings.Join(titleWords, " "), surnames: rest}}
			}
		}
	}
	var out []slugCandidate
	for n := 1; n <= 2 && len(words)-n >= 2; n++ {
		out = append(out, slugCandidate{title: strings.Join(words[:len(words)-n], " "), surnames: words[len(words)-n:]})
	}
	return out
}

// resolveByTitleAndAuthors finds a work whose title equals title exactly and
// that lists one of the surnames. The author makes short titles ("Deep
// Reinforcement Learning", Li) safe to look up; titleIsSpecific alone refuses them.
func resolveByTitleAndAuthors(ctx context.Context, title string, surnames []string) (resolvedArticle, bool) {
	normalized := normalizeTitle(title)
	if len(strings.Fields(normalized)) < 2 || len(normalized) < 10 || len(surnames) == 0 {
		return resolvedArticle{}, false
	}
	ctx, cancel := context.WithTimeout(ctx, titleResolveBudget)
	defer cancel()

	arxivCh := make(chan string, 1)
	go func() {
		id := ""
		if query := arxivTitleQuery(title); query != "" {
			callCtx, stop := context.WithTimeout(ctx, indexCallTimeout)
			entries, _ := searchArxiv(callCtx, query+" AND au:"+surnames[0], 10)
			stop()
			for _, entry := range entries {
				if normalizeTitle(entry.Title) != normalized || !namesHaveSurname(entry.authorNames(), surnames) {
					continue
				}
				if parsed, idErr := NormalizeArxivID(entry.ID); idErr == nil {
					id = CanonicalArxivID(parsed)
					break
				}
			}
		}
		arxivCh <- id
	}()

	callCtx, stop := context.WithTimeout(ctx, indexCallTimeout)
	works, worksErr := searchOpenAlexWorks(callCtx, title, 25)
	stop()
	if id := <-arxivCh; id != "" {
		return resolvedArticle{Kind: "arxiv", Value: id, Meta: articleMetadata{Title: title}, FromTitle: true}, true
	}
	if worksErr != nil {
		return resolvedArticle{}, false
	}
	for _, work := range works {
		if normalizeTitle(work.title()) != normalized || !workTypeAcceptable(work.Type, title) || !namesHaveSurname(work.authors(), surnames) {
			continue
		}
		if resolved, ok := resolveOpenAlexWork(ctx, work, title); ok {
			return resolved, true
		}
	}
	return resolvedArticle{}, false
}

func namesHaveSurname(names, surnames []string) bool {
	for _, name := range names {
		for _, word := range strings.Fields(normalizeTitle(name)) {
			for _, surname := range surnames {
				if word == surname {
					return true
				}
			}
		}
	}
	return false
}

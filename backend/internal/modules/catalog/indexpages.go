package catalog

import (
	"net/url"
	"regexp"
	"strings"
)

// Index, listing, home, login and help pages on scholarly sites are not
// papers: the model cites them as sources, and resolving them only produced
// junk records or a long wait before a 422. The decision is made from the URL
// alone. Keep in sync with isNonPaperUrl in
// frontend/src/features/papers/link-kind.ts and is_non_paper_url in
// services/websearch/app/main.py (both are broader: any site root, /vol/N).

var (
	neuripsIndexPathRE = regexp.MustCompile(`^(?:/paper_files)?(?:/paper)?(?:/\d{4})?$`)
	pmlrVolumeRE       = regexp.MustCompile(`^/v\d+$`)
	jmlrVolumeRE       = regexp.MustCompile(`^/papers/v\d+$`)
)

// nonPaperHosts never serve a single paper.
var nonPaperHosts = map[string]bool{
	"info.arxiv.org":       true,
	"blog.arxiv.org":       true,
	"status.arxiv.org":     true,
	"search.arxiv.org":     true,
	"labs.arxiv.org":       true,
	"confluence.arxiv.org": true,
	"scholar.google.com":   true,
}

// scholarlyHosts are sites whose bare root (and search/login/help pages) is a
// portal, while deeper paths may be papers.
var scholarlyHosts = map[string]bool{
	"arxiv.org": true, "export.arxiv.org": true, "ar5iv.labs.arxiv.org": true, "ar5iv.org": true,
	"openreview.net": true, "semanticscholar.org": true, "sciencedirect.com": true,
	"aclanthology.org": true, "proceedings.neurips.cc": true, "papers.nips.cc": true, "papers.neurips.cc": true,
	"openaccess.thecvf.com": true, "proceedings.mlr.press": true, "jmlr.org": true,
	"dl.acm.org": true, "ieeexplore.ieee.org": true, "link.springer.com": true, "springer.com": true,
	"nature.com": true, "science.org": true, "cell.com": true, "pnas.org": true,
	"pubmed.ncbi.nlm.nih.gov": true, "ncbi.nlm.nih.gov": true, "pmc.ncbi.nlm.nih.gov": true, "europepmc.org": true,
	"researchgate.net": true, "paperswithcode.com": true, "doi.org": true, "dx.doi.org": true,
	"openalex.org": true, "crossref.org": true, "biorxiv.org": true, "medrxiv.org": true, "ssrn.com": true,
	"papers.ssrn.com": true, "jstor.org": true, "onlinelibrary.wiley.com": true, "tandfonline.com": true,
	"mdpi.com": true, "frontiersin.org": true, "journals.plos.org": true, "hindawi.com": true,
	"ojs.aaai.org": true, "ijcai.org": true, "dblp.org": true, "cyberleninka.ru": true, "elibrary.ru": true,
}

// portalSegments are first path segments that are portal pages on any
// scholarly host.
var portalSegments = map[string]bool{
	"search": true, "login": true, "logout": true, "signin": true, "sign-in": true, "signup": true,
	"register": true, "help": true, "about": true, "contact": true, "faq": true, "subscribe": true,
}

var arxivPortalSegments = map[string]bool{
	"list": true, "archive": true, "catchup": true, "year": true, "a": true, "user": true,
	"auth": true, "stats": true, "submit": true, "localization": true, "corr": true, "rss": true,
	"new": true, "multi": true, "institutional_banner": true, "IgnoreMe": true,
}

var openreviewPortalSegments = map[string]bool{
	"venues": true, "venue": true, "group": true, "profile": true, "tasks": true, "activity": true,
	"messages": true, "sponsors": true, "legal": true, "invitation": true,
}

var semanticScholarPortalSegments = map[string]bool{
	"topic": true, "product": true, "author": true, "venue": true, "me": true, "alerts": true,
	"feed": true, "library": true, "api": true, "research": true, "cord19": true, "faqs": true,
}

var sciencedirectPortalSegments = map[string]bool{
	"journal": true, "browse": true, "topics": true, "user": true,
}

var aclPortalSegments = map[string]bool{
	"events": true, "venues": true, "people": true, "volumes": true, "sigs": true, "info": true, "posts": true,
}

func isNonPaperURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.TrimPrefix(strings.TrimSuffix(strings.ToLower(parsed.Hostname()), "."), "www.")
	if nonPaperHosts[host] || strings.HasPrefix(host, "scholar.google.") {
		return true
	}
	if !scholarlyHosts[host] {
		return false
	}
	cleanPath := strings.TrimRight(parsed.Path, "/")
	switch strings.ToLower(cleanPath) {
	case "", "/index.html", "/index.htm", "/index.php":
		return true
	}
	segments := strings.Split(strings.TrimPrefix(cleanPath, "/"), "/")
	first := segments[0]
	if portalSegments[strings.ToLower(first)] {
		return true
	}
	switch host {
	case "arxiv.org", "export.arxiv.org":
		return arxivPortalSegments[first] || arxivPortalSegments[strings.ToLower(first)]
	case "ar5iv.labs.arxiv.org", "ar5iv.org":
		return first != "html" && first != "abs" && first != "pdf"
	case "openreview.net":
		return openreviewPortalSegments[strings.ToLower(first)]
	case "semanticscholar.org":
		return semanticScholarPortalSegments[strings.ToLower(first)]
	case "sciencedirect.com":
		return sciencedirectPortalSegments[strings.ToLower(first)]
	case "aclanthology.org":
		return aclPortalSegments[strings.ToLower(first)]
	case "proceedings.neurips.cc", "papers.nips.cc", "papers.neurips.cc":
		return neuripsIndexPathRE.MatchString(cleanPath) || first == "book" || first == "author" || first == "admin"
	case "openaccess.thecvf.com":
		// Every CVF paper and PDF lives under /content*; the rest are menus
		// and per-conference listings (/ICCV2025, /CVPR2022_workshops/menu).
		return !strings.HasPrefix(strings.ToLower(first), "content")
	case "proceedings.mlr.press":
		return pmlrVolumeRE.MatchString(cleanPath)
	case "jmlr.org":
		return jmlrVolumeRE.MatchString(cleanPath) || cleanPath == "/papers"
	}
	return false
}

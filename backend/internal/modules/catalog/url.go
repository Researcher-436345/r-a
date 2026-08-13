package catalog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/centraluniversity/researcher/internal/platform/queue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/net/html"
)

var (
	errInvalidArticleURL     = errors.New("invalid article URL")
	errUnsupportedArticleURL = errors.New("the page does not expose an arXiv ID, DOI, or PDF")
	yearPrefixRE             = regexp.MustCompile(`\b(19|20)\d{2}\b`)
)

type sourceFetchError struct {
	message string
	err     error
}

func (e *sourceFetchError) Error() string { return e.message + ": " + e.err.Error() }
func (e *sourceFetchError) Unwrap() error { return e.err }

type articleMetadata struct {
	Title    string
	Abstract string
	Venue    string
	DOI      string
	ArxivID  string
	PDFURL   string
	Authors  []string
	Year     *int
}

type resolvedArticle struct {
	Kind  string
	Value string
	Meta  articleMetadata
}

func (a API) addFromURL(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL          string `json:"url"`
		TitleHint    string `json:"title_hint"`
		AddToLibrary *bool  `json:"add_to_library"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if len([]rune(body.TitleHint)) > 1000 {
		httpx.Error(w, 400, "Title is too long")
		return
	}

	resolved, err := resolveArticleURL(r.Context(), body.URL, body.TitleHint)
	if err != nil {
		switch {
		case errors.Is(err, errInvalidArticleURL):
			httpx.Error(w, 400, err.Error())
		case errors.Is(err, errUnsupportedArticleURL):
			httpx.Error(w, 422, err.Error())
		default:
			httpx.Error(w, 502, "Failed to inspect article URL: "+err.Error())
		}
		return
	}

	userID := identity.UserID(r)
	add := addToLibrary(body.AddToLibrary)
	var paperID uuid.UUID
	switch resolved.Kind {
	case "arxiv":
		paperID, err = a.addArxivPaper(r.Context(), userID, resolved.Value, add)
	case "doi":
		paperID, err = a.addDOIPaper(r.Context(), userID, resolved.Value, add)
		if err == nil && resolved.Meta.PDFURL != "" {
			err = a.attachRemotePDF(r.Context(), paperID, resolved.Meta.PDFURL)
		}
	case "pdf":
		paperID, err = a.addRemotePDFPaper(r.Context(), userID, resolved.Value, resolved.Meta, add)
	default:
		err = errUnsupportedArticleURL
	}
	if err != nil {
		var sourceErr *sourceFetchError
		if errors.As(err, &sourceErr) {
			httpx.Error(w, 502, sourceErr.Error())
			return
		}
		httpx.Error(w, 500, err.Error())
		return
	}
	a.paperResponse(w, r, paperID, http.StatusCreated)
}

// addArxivPaper contains the existing arXiv handler workflow without HTTP concerns.
func (a API) addArxivPaper(ctx context.Context, userID uuid.UUID, id string, add bool) (uuid.UUID, error) {
	id = CanonicalArxivID(id)
	paper, err := a.store().FindByArxiv(ctx, id)
	if err != nil {
		return uuid.Nil, err
	}
	if paper != nil {
		if add {
			err = a.Membership.Add(ctx, userID, paper.ID)
		}
		if err == nil {
			if version, versionErr := a.store().LatestVersion(ctx, paper.ID); versionErr == nil {
				err = a.requeueRemotePDF(version)
			}
		}
		return paper.ID, err
	}

	metadata, err := FetchArxivMetadata(ctx, id)
	if err != nil {
		return uuid.Nil, &sourceFetchError{message: "Failed to fetch arXiv metadata", err: err}
	}
	venue := "arXiv"
	created, err := a.store().CreatePaper(ctx, metadata.Title, ptr(metadata.Abstract), metadata.Year, &venue, nil, &metadata.ArxivID)
	if err != nil {
		return uuid.Nil, err
	}
	if err = a.store().AttachAuthors(ctx, created.ID, metadata.Authors); err != nil {
		return uuid.Nil, err
	}
	version, err := a.store().CreateVersion(ctx, created.ID, 1, "arxiv", &metadata.PDFURL, nil, nil, nil, "processing")
	if err != nil {
		return uuid.Nil, err
	}
	if add {
		if err = a.Membership.Add(ctx, userID, created.ID); err != nil {
			return uuid.Nil, err
		}
	}
	if err = a.requeueRemotePDF(version); err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

// addDOIPaper contains the existing Crossref handler workflow without HTTP concerns.
func (a API) addDOIPaper(ctx context.Context, userID uuid.UUID, doi string, add bool) (uuid.UUID, error) {
	paper, err := a.store().FindByDOI(ctx, doi)
	if err != nil {
		return uuid.Nil, err
	}
	if paper != nil {
		if add {
			err = a.Membership.Add(ctx, userID, paper.ID)
		}
		return paper.ID, err
	}

	metadata, err := FetchCrossrefMetadata(ctx, doi)
	if err != nil {
		return uuid.Nil, &sourceFetchError{message: "Failed to fetch Crossref metadata", err: err}
	}
	created, err := a.store().CreatePaper(ctx, metadata.Title, metadata.Abstract, metadata.Year, metadata.Venue, &metadata.DOI, nil)
	if err != nil {
		return uuid.Nil, err
	}
	if err = a.store().AttachAuthors(ctx, created.ID, metadata.Authors); err != nil {
		return uuid.Nil, err
	}
	sourceURL := "https://doi.org/" + metadata.DOI
	if _, err = a.store().CreateVersion(ctx, created.ID, 1, "doi", &sourceURL, nil, nil, nil, "ready"); err != nil {
		return uuid.Nil, err
	}
	if add {
		if err = a.Membership.Add(ctx, userID, created.ID); err != nil {
			return uuid.Nil, err
		}
	}
	return created.ID, nil
}

func (a API) addRemotePDFPaper(ctx context.Context, userID uuid.UUID, sourceURL string, metadata articleMetadata, add bool) (uuid.UUID, error) {
	existing, err := a.store().FindVersionBySourceURL(ctx, sourceURL)
	if err != nil {
		return uuid.Nil, err
	}
	if existing != nil {
		if err = a.requeueRemotePDF(*existing); err != nil {
			return uuid.Nil, err
		}
		if add {
			if err = a.Membership.Add(ctx, userID, existing.PaperID); err != nil {
				return uuid.Nil, err
			}
		}
		return existing.PaperID, nil
	}

	title := cleanMetadataValue(metadata.Title)
	if title == "" {
		title = fallbackURLTitle(sourceURL)
	}
	var abstract, venue *string
	if value := cleanMetadataValue(metadata.Abstract); value != "" {
		abstract = &value
	}
	if value := cleanMetadataValue(metadata.Venue); value != "" {
		venue = &value
	} else if parsed, parseErr := url.Parse(sourceURL); parseErr == nil {
		value = parsed.Hostname()
		venue = &value
	}
	created, err := a.store().CreatePaper(ctx, title, abstract, metadata.Year, venue, nil, nil)
	if err != nil {
		return uuid.Nil, err
	}
	if err = a.store().AttachAuthors(ctx, created.ID, metadata.Authors); err != nil {
		return uuid.Nil, err
	}
	if err = a.attachRemotePDF(ctx, created.ID, sourceURL); err != nil {
		return uuid.Nil, err
	}
	if add {
		if err = a.Membership.Add(ctx, userID, created.ID); err != nil {
			return uuid.Nil, err
		}
	}
	return created.ID, nil
}

func (a API) attachRemotePDF(ctx context.Context, paperID uuid.UUID, sourceURL string) error {
	existing, err := a.store().FindVersionBySourceURL(ctx, sourceURL)
	if err != nil {
		return err
	}
	if existing != nil {
		return a.requeueRemotePDF(*existing)
	}
	latest, err := a.store().LatestVersion(ctx, paperID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil && (latest.PDFKey != nil || (latest.Status == "processing" && (latest.Source == "arxiv" || latest.Source == "web_pdf"))) {
		return nil
	}
	number, err := a.store().NextVersionNumber(ctx, paperID)
	if err != nil {
		return err
	}
	version, err := a.store().CreateVersion(ctx, paperID, number, "web_pdf", &sourceURL, nil, nil, nil, "processing")
	if err != nil {
		return err
	}
	return a.requeueRemotePDF(version)
}

func (a API) requeueRemotePDF(version Version) error {
	if a.Queue == nil || version.Status != "processing" || version.PDFKey != nil {
		return nil
	}
	if version.Source != "arxiv" && version.Source != "web_pdf" {
		return nil
	}
	// Reuse the original task name so already-running workers can process
	// generic remote PDFs without a synchronized deployment. Re-enqueueing also
	// repairs records left waiting by the short-lived process_remote_pdf name.
	return queue.Enqueue(a.Queue, queue.ProcessArxivPDF, version.ID.String())
}

func resolveArticleURL(ctx context.Context, rawURL, titleHint string) (resolvedArticle, error) {
	normalized, err := normalizeArticleURL(rawURL)
	if err != nil {
		return resolvedArticle{}, err
	}
	if known, ok := classifyKnownArticleURL(normalized); ok {
		known.Meta.Title = cleanMetadataValue(titleHint)
		return known, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalized, nil)
	if err != nil {
		return resolvedArticle{}, fmt.Errorf("%w: %v", errInvalidArticleURL, err)
	}
	req.Header.Set("User-Agent", "Researcher/1.0 (paper-library)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/pdf;q=0.9,*/*;q=0.1")
	resp, err := articleHTTPClient.Do(req)
	if err != nil {
		return resolvedArticle{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return resolvedArticle{}, fmt.Errorf("article page returned %s", resp.Status)
	}
	finalURL, err := normalizeArticleURL(resp.Request.URL.String())
	if err != nil {
		return resolvedArticle{}, err
	}
	if known, ok := classifyKnownArticleURL(finalURL); ok {
		known.Meta.Title = cleanMetadataValue(titleHint)
		return known, nil
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return resolvedArticle{}, err
	}
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType == "application/pdf" || hasPDFHeader(raw) {
		return resolvedArticle{Kind: "pdf", Value: finalURL, Meta: articleMetadata{Title: cleanMetadataValue(titleHint)}}, nil
	}
	if mediaType != "" && mediaType != "text/html" && mediaType != "application/xhtml+xml" {
		return resolvedArticle{}, fmt.Errorf("%w: unsupported content type %s", errUnsupportedArticleURL, mediaType)
	}

	metadata := parseArticleHTML(raw, finalURL)
	if metadata.Title == "" {
		metadata.Title = cleanMetadataValue(titleHint)
	}
	if metadata.ArxivID != "" {
		return resolvedArticle{Kind: "arxiv", Value: metadata.ArxivID, Meta: metadata}, nil
	}
	if metadata.DOI != "" {
		return resolvedArticle{Kind: "doi", Value: metadata.DOI, Meta: metadata}, nil
	}
	if metadata.PDFURL != "" {
		return resolvedArticle{Kind: "pdf", Value: metadata.PDFURL, Meta: metadata}, nil
	}
	return resolvedArticle{}, errUnsupportedArticleURL
}

func classifyKnownArticleURL(raw string) (resolvedArticle, bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return resolvedArticle{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "arxiv.org" || host == "www.arxiv.org" || host == "export.arxiv.org" {
		if id, err := NormalizeArxivID(raw); err == nil {
			return resolvedArticle{Kind: "arxiv", Value: CanonicalArxivID(id)}, true
		}
	}
	if host == "doi.org" || host == "dx.doi.org" || host == "www.doi.org" {
		value, unescapeErr := url.PathUnescape(strings.TrimPrefix(parsed.EscapedPath(), "/"))
		if unescapeErr == nil {
			if doi, doiErr := NormalizeDOI(value); doiErr == nil {
				return resolvedArticle{Kind: "doi", Value: doi}, true
			}
		}
	}
	if strings.EqualFold(path.Ext(parsed.Path), ".pdf") {
		return resolvedArticle{Kind: "pdf", Value: raw}, true
	}
	return resolvedArticle{}, false
}

func normalizeArticleURL(raw string) (string, error) {
	if len(raw) > 4000 {
		return "", fmt.Errorf("%w: URL is too long", errInvalidArticleURL)
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errInvalidArticleURL
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: only http and https are supported", errInvalidArticleURL)
	}
	if parsed.User != nil {
		return "", fmt.Errorf("%w: credentials are not allowed", errInvalidArticleURL)
	}
	if err := validatePublicURLShape(parsed); err != nil {
		return "", err
	}
	parsed.Fragment = ""
	query := parsed.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" {
			query.Del(key)
		}
	}
	parsed.RawQuery = query.Encode()
	value := parsed.String()
	if len(value) > 1000 {
		return "", fmt.Errorf("%w: normalized URL is too long", errInvalidArticleURL)
	}
	return value, nil
}

func parseArticleHTML(raw []byte, pageURL string) articleMetadata {
	var metadata articleMetadata
	baseURL, _ := url.Parse(pageURL)
	base := baseURL
	insideTitle := false
	tokenizer := html.NewTokenizer(strings.NewReader(string(raw)))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return normalizeArticleMetadata(metadata, base)
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			switch strings.ToLower(token.Data) {
			case "title":
				insideTitle = true
			case "base":
				if href := attr(token, "href"); href != "" {
					if candidate := resolvePageReference(baseURL, href); candidate != nil {
						base = candidate
					}
				}
			case "meta":
				key := strings.ToLower(firstNonEmpty(attr(token, "name"), attr(token, "property"), attr(token, "itemprop")))
				value := cleanMetadataValue(attr(token, "content"))
				if value == "" {
					continue
				}
				switch key {
				case "citation_title":
					metadata.Title = value
				case "dc.title", "og:title":
					if metadata.Title == "" {
						metadata.Title = value
					}
				case "citation_author", "dc.creator":
					metadata.Authors = appendUnique(metadata.Authors, value)
				case "citation_pdf_url":
					if metadata.PDFURL == "" {
						metadata.PDFURL = value
					}
				case "citation_doi", "dc.identifier.doi", "prism.doi":
					if metadata.DOI == "" {
						metadata.DOI = value
					}
				case "dc.identifier":
					if metadata.DOI == "" && strings.Contains(strings.ToLower(value), "10.") {
						metadata.DOI = value
					}
				case "citation_arxiv_id", "arxiv_id":
					if metadata.ArxivID == "" {
						metadata.ArxivID = value
					}
				case "citation_journal_title", "citation_conference_title", "dc.source":
					if metadata.Venue == "" {
						metadata.Venue = value
					}
				case "citation_abstract", "description", "og:description":
					if metadata.Abstract == "" {
						metadata.Abstract = value
					}
				case "citation_publication_date", "citation_date", "dc.date", "article:published_time":
					if metadata.Year == nil {
						metadata.Year = yearFromText(value)
					}
				}
			case "link":
				href := attr(token, "href")
				typ := strings.ToLower(attr(token, "type"))
				if metadata.PDFURL == "" && href != "" && (typ == "application/pdf" || strings.EqualFold(path.Ext(parsePath(href)), ".pdf")) {
					metadata.PDFURL = href
				}
			}
		case html.EndTagToken:
			if strings.EqualFold(tokenizer.Token().Data, "title") {
				insideTitle = false
			}
		case html.TextToken:
			if insideTitle && metadata.Title == "" {
				metadata.Title = cleanMetadataValue(string(tokenizer.Text()))
			}
		}
	}
}

func normalizeArticleMetadata(metadata articleMetadata, base *url.URL) articleMetadata {
	if metadata.DOI != "" {
		if doi, err := NormalizeDOI(metadata.DOI); err == nil {
			metadata.DOI = doi
		} else {
			metadata.DOI = ""
		}
	}
	if metadata.ArxivID != "" {
		if id, err := NormalizeArxivID(metadata.ArxivID); err == nil {
			metadata.ArxivID = CanonicalArxivID(id)
		} else {
			metadata.ArxivID = ""
		}
	}
	if metadata.PDFURL != "" {
		if candidate := resolvePageReference(base, metadata.PDFURL); candidate != nil {
			if normalized, err := normalizeArticleURL(candidate.String()); err == nil {
				metadata.PDFURL = normalized
			} else {
				metadata.PDFURL = ""
			}
		} else {
			metadata.PDFURL = ""
		}
	}
	return metadata
}

func attr(token html.Token, name string) string {
	for _, value := range token.Attr {
		if strings.EqualFold(value.Key, name) {
			return value.Val
		}
	}
	return ""
}

func resolvePageReference(base *url.URL, raw string) *url.URL {
	reference, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || base == nil {
		return nil
	}
	return base.ResolveReference(reference)
}

func parsePath(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Path
}

func yearFromText(value string) *int {
	match := yearPrefixRE.FindString(value)
	if match == "" {
		return nil
	}
	year, err := strconv.Atoi(match)
	if err != nil {
		return nil
	}
	return &year
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func cleanMetadataValue(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 1000 {
		value = string(runes[:1000])
	}
	return value
}

func fallbackURLTitle(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "Web article"
	}
	name, _ := url.PathUnescape(path.Base(parsed.Path))
	name = strings.TrimSuffix(name, path.Ext(name))
	name = strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(name))
	if name == "" || name == "." || name == "/" {
		name = parsed.Hostname()
	}
	if name == "" {
		return "Web article"
	}
	return cleanMetadataValue(name)
}

func validatePublicURLShape(parsed *url.URL) error {
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: only http and https are supported", errInvalidArticleURL)
	}
	if parsed.User != nil {
		return fmt.Errorf("%w: credentials are not allowed", errInvalidArticleURL)
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || host == "metadata.google.internal" {
		return fmt.Errorf("%w: private hosts are not allowed", errInvalidArticleURL)
	}
	if ip := net.ParseIP(host); ip != nil && !isPublicIP(ip) {
		return fmt.Errorf("%w: private addresses are not allowed", errInvalidArticleURL)
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsUnspecified() || address.IsMulticast() {
		return false
	}
	shared := netip.MustParsePrefix("100.64.0.0/10")
	return !shared.Contains(address)
}

func publicDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if parsed := net.ParseIP(host); parsed != nil {
		if !isPublicIP(parsed) {
			return nil, fmt.Errorf("private address is not allowed")
		}
		return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, address)
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			continue
		}
		connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		err = dialErr
	}
	if err == nil {
		err = fmt.Errorf("host does not resolve to a public address")
	}
	return nil, err
}

var publicHTTPTransport = &http.Transport{
	DialContext:           publicDialContext,
	TLSHandshakeTimeout:   15 * time.Second,
	ResponseHeaderTimeout: 30 * time.Second,
	IdleConnTimeout:       90 * time.Second,
}

func newPublicHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: publicHTTPTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return validatePublicURLShape(req.URL)
		},
	}
}

var articleHTTPClient = newPublicHTTPClient(30 * time.Second)
var pdfHTTPClient = newPublicHTTPClient(90 * time.Second)

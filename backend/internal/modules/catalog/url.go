package catalog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/centraluniversity/researcher/internal/platform/queue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/net/html"
)

var (
	errInvalidArticleURL     = errors.New("invalid article URL")
	errUnsupportedArticleURL = errors.New("the paper was not found in open sources")
	errNotAPaper             = errors.New("the link points to an index or listing page, not a paper")
	yearPrefixRE             = regexp.MustCompile(`\b(19|20)\d{2}\b`)

	articlePageTimeout   = 12 * time.Second
	arxivMetadataTimeout = 15 * time.Second
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
	Kind  string // arxiv | doi | pdf | openalex
	Value string
	// Meta.PDFURL is only a candidate and is probed before use.
	Meta     articleMetadata
	Work     *openAlexWork
	Crossref *CrossrefPaper
	// FromTitle marks results found by title search, not read from the URL.
	FromTitle   bool
	pdfVerified bool
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
	if !a.allow(w, r, ruleResolveUser) {
		return
	}
	hint := cleanTitleHint(body.TitleHint)
	missKey := resolveMissKey(body.URL, hint)
	if cached := resolveMisses.get(missKey); cached != nil {
		writeResolveError(w, cached, true)
		return
	}

	// Index pages are rejected inside resolveArticleURL before any network
	// call or database write.
	resolved, err := resolveArticleURL(r.Context(), body.URL, hint)
	if err != nil {
		resolveMisses.remember(missKey, err)
		writeResolveError(w, err, true)
		return
	}
	paperID, err := a.addResolved(r.Context(), identity.UserID(r), resolved, hint, addToLibrary(body.AddToLibrary))
	if err != nil {
		resolveMisses.remember(missKey, err)
		writeResolveError(w, err, false)
		return
	}
	a.paperResponse(w, r, paperID, http.StatusCreated)
}

// writeResolveError maps resolution failures onto the from-url contract:
// 400 bad URL, 422 not_a_paper / not_found, 502 upstream failure. Upstream
// and internal error text stays in the log: dial errors name internal hosts.
func writeResolveError(w http.ResponseWriter, err error, resolving bool) {
	var sourceErr *sourceFetchError
	switch {
	case errors.Is(err, errInvalidArticleURL):
		httpx.Error(w, 400, err.Error())
	case errors.Is(err, errNotAPaper):
		httpx.ErrorCode(w, http.StatusUnprocessableEntity, "not_a_paper", err.Error())
	case errors.Is(err, errUnsupportedArticleURL):
		httpx.ErrorCode(w, http.StatusUnprocessableEntity, "not_found", err.Error())
	case errors.As(err, &sourceErr):
		log.Printf("catalog: %v", err)
		httpx.Error(w, 502, sourceErr.message)
	case resolving || errors.Is(err, context.DeadlineExceeded):
		log.Printf("catalog: inspect article URL: %v", err)
		httpx.Error(w, 502, "Failed to inspect article URL")
	default:
		log.Printf("catalog: add paper: %v", err)
		httpx.Error(w, 500, "Failed to add the paper")
	}
}

func (a API) addResolved(ctx context.Context, userID uuid.UUID, resolved resolvedArticle, hint string, add bool) (uuid.UUID, error) {
	switch resolved.Kind {
	case "arxiv":
		return a.addArxivForLink(ctx, userID, resolved, hint, add)
	case "doi":
		return a.addDOIResolved(ctx, userID, doiRequest{
			DOI:           resolved.Value,
			PDFHint:       resolved.Meta.PDFURL,
			TitleHint:     hint,
			Work:          resolved.Work,
			Crossref:      resolved.Crossref,
			TitleSearched: resolved.FromTitle,
		}, add)
	case "pdf":
		if !resolved.pdfVerified && !probePDF(ctx, resolved.Value) {
			return uuid.Nil, errUnsupportedArticleURL
		}
		return a.addRemotePDFPaper(ctx, userID, resolved.Value, resolved.Meta, add)
	case "openalex":
		if resolved.Work == nil {
			return uuid.Nil, errUnsupportedArticleURL
		}
		return a.addOpenAlexPaper(ctx, userID, *resolved.Work, add)
	}
	return uuid.Nil, errUnsupportedArticleURL
}

func (a API) addMembership(ctx context.Context, userID, paperID uuid.UUID, add bool) error {
	if !add {
		return nil
	}
	return a.Membership.Add(ctx, userID, paperID)
}

// linkNamesOtherPaper reports that the link text is a specific title that is
// not the title of the paper the URL points to.
func linkNamesOtherPaper(paperTitle, hint string) bool {
	return titleIsSpecific(hint) && strings.TrimSpace(paperTitle) != "" && !titlesMatch(paperTitle, hint)
}

// addArxivForLink opens the arXiv paper a link points to, unless the link
// text names a different paper: the model sometimes pairs a correct title
// with a wrong arXiv id, and the user clicked the title.
func (a API) addArxivForLink(ctx context.Context, userID uuid.UUID, resolved resolvedArticle, hint string, add bool) (uuid.UUID, error) {
	id := CanonicalArxivID(resolved.Value)
	existing, err := a.findOpenableArxivPaper(ctx, userID, id)
	if err != nil {
		return uuid.Nil, err
	}
	var metadata *ArxivPaper
	title := ""
	if existing != nil {
		title = existing.Title
	} else {
		fetched, fetchErr := fetchArxivMetadataBounded(ctx, id)
		if fetchErr != nil {
			// Only a missing id is worth a title search: during an arXiv outage
			// the title would map to a DOI-only duplicate of this arXiv paper.
			if !errors.Is(fetchErr, errUnsupportedArticleURL) {
				return uuid.Nil, &sourceFetchError{message: "Failed to fetch arXiv metadata", err: fetchErr}
			}
			// A made-up id can still come with the real paper's title.
			if !resolved.FromTitle {
				if alt, ok := resolveByTitle(ctx, hint); ok && !(alt.Kind == "arxiv" && alt.Value == id) {
					return a.addResolved(ctx, userID, alt, hint, add)
				}
			}
			return uuid.Nil, fetchErr
		}
		metadata = &fetched
		title = fetched.Title
	}
	if !resolved.FromTitle && linkNamesOtherPaper(title, hint) {
		if alt, ok := resolveByTitle(ctx, hint); ok && !(alt.Kind == "arxiv" && alt.Value == id) {
			return a.addResolved(ctx, userID, alt, hint, add)
		}
	}
	return a.addArxivPaperWithMeta(ctx, userID, id, metadata, add)
}

func fetchArxivMetadataBounded(ctx context.Context, id string) (ArxivPaper, error) {
	ctx, cancel := context.WithTimeout(ctx, arxivMetadataTimeout)
	defer cancel()
	release, err := takeSlot(ctx, arxivAPISlots)
	if err != nil {
		return ArxivPaper{}, err
	}
	defer release()
	metadata, err := FetchArxivMetadata(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ArxivPaper{}, fmt.Errorf("%w: arXiv has no paper %s", errUnsupportedArticleURL, id)
		}
		return ArxivPaper{}, err
	}
	// The export API answers malformed ids with an "Error" entry, not a 404.
	if strings.EqualFold(metadata.Title, "Error") {
		return ArxivPaper{}, fmt.Errorf("%w: arXiv has no paper %s", errUnsupportedArticleURL, id)
	}
	return metadata, nil
}

// addArxivPaper contains the existing arXiv handler workflow without HTTP concerns.
func (a API) addArxivPaper(ctx context.Context, userID uuid.UUID, id string, add bool) (uuid.UUID, error) {
	return a.addArxivPaperWithMeta(ctx, userID, id, nil, add)
}

// findOpenableArxivPaper returns the paper holding an arXiv id when this user
// may open it. Uploads read the id from the PDF bytes; a private upload gives
// the id up instead of routing other users to its (404) record or, through
// "add to folder", to the uploaded file.
func (a API) findOpenableArxivPaper(ctx context.Context, userID uuid.UUID, id string) (*Paper, error) {
	paper, err := a.store().FindByArxiv(ctx, id)
	if err != nil || paper == nil {
		return paper, err
	}
	public, err := a.store().IsPublic(ctx, paper.ID)
	if err != nil || public {
		return paper, err
	}
	if a.Membership != nil {
		mine, hasErr := a.Membership.Has(ctx, userID, paper.ID)
		if hasErr != nil {
			return nil, hasErr
		}
		if mine {
			return paper, nil
		}
	}
	if err = a.store().ClearArxivID(ctx, paper.ID); err != nil {
		return nil, err
	}
	return nil, nil
}

func (a API) addArxivPaperWithMeta(ctx context.Context, userID uuid.UUID, id string, metadata *ArxivPaper, add bool) (uuid.UUID, error) {
	id = CanonicalArxivID(id)
	paper, err := a.findOpenableArxivPaper(ctx, userID, id)
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

	if metadata == nil {
		fetched, fetchErr := fetchArxivMetadataBounded(ctx, id)
		if fetchErr != nil {
			if errors.Is(fetchErr, errUnsupportedArticleURL) {
				return uuid.Nil, fetchErr
			}
			return uuid.Nil, &sourceFetchError{message: "Failed to fetch arXiv metadata", err: fetchErr}
		}
		metadata = &fetched
	}
	venue := "arXiv"
	created, err := a.store().CreatePaper(ctx, metadata.Title, ptr(metadata.Abstract), metadata.Year, &venue, nil, &metadata.ArxivID)
	if err != nil {
		if isUniqueViolation(err) {
			if again, findErr := a.store().FindByArxiv(ctx, id); findErr == nil && again != nil {
				return again.ID, a.addMembership(ctx, userID, again.ID, add)
			}
		}
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

type doiRequest struct {
	DOI       string
	PDFHint   string
	TitleHint string
	// Already fetched during resolution; nil means look it up.
	Work     *openAlexWork
	Crossref *CrossrefPaper
	// TitleSearched: arXiv was already asked by title, and a title fallback
	// must not run again.
	TitleSearched bool
}

// addOpenDOIPaper prefers something the reader can open over a bare DOI
// record: an arXiv version (TeX-first parse, page cites) when OpenAlex or the
// arXiv title index knows one, otherwise the DOI record plus a verified PDF or
// Europe PMC full text. A DOI without full text still becomes a paper — the
// reader shows the metadata and the assistant works from the abstract — but
// the user stays inside the app instead of being sent to a publisher page.
func (a API) addOpenDOIPaper(ctx context.Context, userID uuid.UUID, doi, pdfHint string, add bool) (uuid.UUID, error) {
	return a.addDOIResolved(ctx, userID, doiRequest{DOI: doi, PDFHint: pdfHint}, add)
}

func (a API) addDOIResolved(ctx context.Context, userID uuid.UUID, req doiRequest, add bool) (uuid.UUID, error) {
	doi := req.DOI
	existing, err := a.store().FindByDOI(ctx, doi)
	if err != nil {
		return uuid.Nil, err
	}
	if existing != nil {
		if existing.ArxivID != nil && *existing.ArxivID != "" {
			return a.addArxivPaper(ctx, userID, *existing.ArxivID, add)
		}
		return existing.ID, a.addMembership(ctx, userID, existing.ID, add)
	}

	work, crossref := req.Work, req.Crossref
	var crossrefErr error
	var wg sync.WaitGroup
	if work == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if found, lookupErr := lookupOpenAlexByDOI(ctx, doi); lookupErr == nil {
				work = &found
			}
		}()
	}
	if crossref == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			callCtx, stop := context.WithTimeout(ctx, crossrefCallTimeout)
			defer stop()
			fetched, fetchErr := FetchCrossrefMetadata(callCtx, doi)
			if fetchErr == nil {
				crossref = &fetched
			} else {
				crossrefErr = fetchErr
			}
		}()
	}
	wg.Wait()

	if work != nil {
		if rec := work.record(); rec.ArxivID != "" {
			return a.addArxivPaper(ctx, userID, rec.ArxivID, add)
		}
	}
	containerType := ""
	if crossref != nil && isContainerCrossrefType(crossref.Type) {
		containerType = crossref.Type
	} else if crossref == nil && work != nil && isContainerWorkType(work.Type) {
		containerType = work.Type
	}
	if containerType != "" {
		// A volume DOI next to a real title: the user wanted the paper.
		if !req.TitleSearched {
			if alt, ok := resolveByTitle(ctx, req.TitleHint); ok && !(alt.Kind == "doi" && alt.Value == doi) {
				alt.FromTitle = true
				return a.addResolved(ctx, userID, alt, req.TitleHint, add)
			}
		}
		return uuid.Nil, fmt.Errorf("%w: DOI %s names a %s", errNotAPaper, doi, containerType)
	}
	if crossref == nil {
		if work != nil && work.workURL() != "" {
			return a.addOpenAlexPaper(ctx, userID, *work, add)
		}
		if errors.Is(crossrefErr, errUnsupportedArticleURL) {
			return uuid.Nil, crossrefErr
		}
		return uuid.Nil, &sourceFetchError{message: "Failed to fetch Crossref metadata", err: crossrefErr}
	}

	if !req.TitleSearched {
		title := crossref.Title
		if work != nil && work.title() != "" {
			title = work.title()
		}
		if id, ok := findArxivByTitleBounded(ctx, title); ok {
			return a.addArxivPaper(ctx, userID, id, add)
		}
	}

	paperID, version, err := a.createDOIPaper(ctx, *crossref, work)
	if err != nil {
		return uuid.Nil, err
	}
	if err = a.addMembership(ctx, userID, paperID, add); err != nil {
		return uuid.Nil, err
	}
	if version == nil {
		return paperID, nil
	}
	candidates := []string{req.PDFHint}
	pmcid := ""
	if work != nil {
		candidates = append(candidates, work.pdfCandidates()...)
		pmcid = work.pmcid()
	}
	if _, attachErr := a.attachOpenFullText(ctx, paperID, version.ID, doi, pmcid, pdfCandidateList(candidates...)); attachErr != nil {
		log.Printf("catalog: full text for doi %s: %v", doi, attachErr)
	}
	return paperID, nil
}

// isContainerWorkType is the OpenAlex view of a container DOI, used when
// Crossref cannot be asked.
func isContainerWorkType(t string) bool {
	return strings.EqualFold(strings.TrimSpace(t), "paratext") || isContainerCrossrefType(t)
}

// createDOIPaper stores a Crossref record, filling what Crossref lacks
// (abstracts, often) from OpenAlex. A nil version means another request
// created the same DOI concurrently and its paper is returned instead.
func (a API) createDOIPaper(ctx context.Context, crossref CrossrefPaper, work *openAlexWork) (uuid.UUID, *Version, error) {
	title := crossref.Title
	abstract, year, venue, authors := crossref.Abstract, crossref.Year, crossref.Venue, crossref.Authors
	if work != nil {
		meta := work.metadata()
		if strings.HasPrefix(title, "DOI:") && meta.Title != "" {
			title = meta.Title
		}
		if abstract == nil && meta.Abstract != "" {
			abstract = &meta.Abstract
		}
		if year == nil {
			year = meta.Year
		}
		if venue == nil && meta.Venue != "" {
			venue = &meta.Venue
		}
		if len(authors) == 0 {
			authors = meta.Authors
		}
	}
	if venue != nil {
		trimmed := truncateRunes(*venue, 500)
		venue = &trimmed
	}
	doi := crossref.DOI
	created, err := a.store().CreatePaper(ctx, title, abstract, year, venue, &doi, nil)
	if err != nil {
		if isUniqueViolation(err) {
			if again, findErr := a.store().FindByDOI(ctx, doi); findErr == nil && again != nil {
				return again.ID, nil, nil
			}
		}
		return uuid.Nil, nil, err
	}
	if err = a.store().AttachAuthors(ctx, created.ID, authors); err != nil {
		return uuid.Nil, nil, err
	}
	sourceURL := "https://doi.org/" + doi
	version, err := a.store().CreateVersion(ctx, created.ID, 1, "doi", &sourceURL, nil, nil, nil, "ready")
	if err != nil {
		return uuid.Nil, nil, err
	}
	return created.ID, &version, nil
}

// addRemotePDFPaper stores a paper for a PDF URL the caller has verified.
func (a API) addRemotePDFPaper(ctx context.Context, userID uuid.UUID, sourceURL string, metadata articleMetadata, add bool) (uuid.UUID, error) {
	existing, err := a.store().FindVersionBySourceURL(ctx, sourceURL)
	if err != nil {
		return uuid.Nil, err
	}
	if existing != nil {
		// The URL answered with a PDF just now, so an earlier failed
		// download of it deserves another attempt.
		if err = a.attachRemotePDF(ctx, existing.PaperID, sourceURL); err != nil {
			return uuid.Nil, err
		}
		if add {
			if err = a.Membership.Add(ctx, userID, existing.PaperID); err != nil {
				return uuid.Nil, err
			}
		}
		return existing.PaperID, nil
	}

	title := cleanTitleHint(metadata.Title)
	if title == "" {
		title = fallbackURLTitle(sourceURL)
	}
	var abstract, venue *string
	if value := cleanMetadataValue(metadata.Abstract); value != "" {
		abstract = &value
	}
	if value := truncateRunes(cleanMetadataValue(metadata.Venue), 500); value != "" {
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

// attachRemotePDF queues the download of a probe-verified PDF as the paper's
// newest version.
func (a API) attachRemotePDF(ctx context.Context, paperID uuid.UUID, sourceURL string) error {
	return a.attachPDFVersion(ctx, paperID, "web_pdf", sourceURL)
}

func (a API) attachPDFVersion(ctx context.Context, paperID uuid.UUID, source, sourceURL string) error {
	latest, err := a.store().LatestVersion(ctx, paperID)
	hasLatest := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	// A failed upload keeps its pdf_key but has no usable PDF.
	hasPDF := latest.PDFKey != nil && latest.Status != "failed"
	if hasLatest && (hasPDF || (latest.Status == "processing" && (latest.Source == "arxiv" || latest.Source == "web_pdf"))) {
		return a.requeueRemotePDF(latest)
	}
	if hasLatest && latest.Status == "failed" && latest.SourceURL != nil && *latest.SourceURL == sourceURL {
		if permanentDownloadFailure(latest.ErrorMessage) {
			return nil
		}
		latest.Status = "processing"
		latest.ErrorMessage = nil
		if err = a.store().UpdateVersion(ctx, latest); err != nil {
			return err
		}
		return a.requeueRemotePDF(latest)
	}
	number, err := a.store().NextVersionNumber(ctx, paperID)
	if err != nil {
		return err
	}
	version, err := a.store().CreateVersion(ctx, paperID, number, source, &sourceURL, nil, nil, nil, "processing")
	if err != nil {
		return err
	}
	return a.requeueRemotePDF(version)
}

// permanentDownloadFailure: the worker fails such a download the same way on
// every retry, while the probe (first bytes only) keeps passing, so resetting
// the version would only loop download → fail.
func permanentDownloadFailure(message *string) bool {
	if message == nil {
		return false
	}
	m := strings.ToLower(*message)
	return strings.Contains(m, "too large") || strings.Contains(m, "did not return a pdf")
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

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// resolveArticleURL decides what a link is. Kind "pdf" results are verified
// PDFs; everything that fails falls back to the link text as a title.
func resolveArticleURL(ctx context.Context, rawURL, titleHint string) (resolvedArticle, error) {
	normalized, err := normalizeArticleURL(rawURL)
	if err != nil {
		return resolvedArticle{}, err
	}
	if isNonPaperURL(normalized) {
		return resolvedArticle{}, errNotAPaper
	}
	hint := cleanTitleHint(titleHint)
	if known, ok := classifyKnownArticleURL(normalized); ok {
		known.Meta.Title = hint
		if known.Kind != "pdf" {
			return known, nil
		}
		if probePDF(ctx, normalized) {
			known.Meta.Title = pdfTitleFromHint(hint, normalized)
			known.pdfVerified = true
			return known, nil
		}
		title := hint
		if hostBlocksScrapers(normalized) {
			// link.springer.com/content/pdf/10.1007/….pdf still names the DOI.
			byID, ok, fallback := resolveBlockedHostURL(ctx, normalized, hint)
			if ok {
				return byID, nil
			}
			title = fallback
		}
		return fallbackByTitle(ctx, title, fmt.Errorf("%w: the PDF link did not return a PDF", errUnsupportedArticleURL))
	}

	if hostBlocksScrapers(normalized) {
		// The URL itself often names the work (DOI, PII, title slug); link text
		// can be empty ("[5]") or too generic for a title search.
		byID, ok, title := resolveBlockedHostURL(ctx, normalized, hint)
		if ok {
			return byID, nil
		}
		return fallbackByTitle(ctx, title, errUnsupportedArticleURL)
	}

	pageCtx, cancel := context.WithTimeout(ctx, articlePageTimeout)
	resolved, err := inspectArticlePage(pageCtx, normalized, hint)
	if err == nil && resolved.Kind == "pdf" && !resolved.pdfVerified {
		resolved.pdfVerified = probePDF(pageCtx, resolved.Value)
		if !resolved.pdfVerified {
			err = fmt.Errorf("%w: the page's PDF link did not return a PDF", errUnsupportedArticleURL)
		}
	}
	cancel()
	if err == nil {
		return resolved, nil
	}
	if errors.Is(err, errInvalidArticleURL) {
		return resolvedArticle{}, err
	}
	// The page refused us or exposes nothing useful: the assistant's link text
	// is usually the exact paper title, and open indexes can map that.
	title := hint
	if !titleIsSpecific(title) && resolved.Meta.Title != "" {
		title = resolved.Meta.Title
	}
	return fallbackByTitle(ctx, title, err)
}

func fallbackByTitle(ctx context.Context, title string, cause error) (resolvedArticle, error) {
	if byTitle, ok := resolveByTitle(ctx, title); ok {
		return byTitle, nil
	}
	return resolvedArticle{}, cause
}

// pdfTitleFromHint keeps link text that reads like a title and otherwise
// names the paper after its file ("CVPRW 2022 PDF" says nothing).
func pdfTitleFromHint(hint, pdfURL string) string {
	if utf8.RuneCountInString(hint) >= 20 && len(strings.Fields(hint)) >= 3 {
		return hint
	}
	return fallbackURLTitle(pdfURL)
}

// inspectArticlePage reads a page's citation metadata. On failure the
// returned article still carries what was learned (the page title).
func inspectArticlePage(ctx context.Context, normalized, titleHint string) (resolvedArticle, error) {
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
	if resp.StatusCode/100 == 4 {
		return resolvedArticle{}, fmt.Errorf("%w: article page returned %s", errUnsupportedArticleURL, resp.Status)
	}
	if resp.StatusCode/100 != 2 {
		return resolvedArticle{}, fmt.Errorf("article page returned %s", resp.Status)
	}
	finalURL, err := normalizeArticleURL(resp.Request.URL.String())
	if err != nil {
		return resolvedArticle{}, err
	}
	if isNonPaperURL(finalURL) {
		return resolvedArticle{}, fmt.Errorf("%w: the link redirects to an index page", errUnsupportedArticleURL)
	}
	if known, ok := classifyKnownArticleURL(finalURL); ok && known.Kind != "pdf" {
		known.Meta.Title = titleHint
		return known, nil
	}

	head := make([]byte, 1024)
	n, _ := io.ReadFull(resp.Body, head)
	head = head[:n]
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if hasPDFHeader(head) {
		meta := articleMetadata{Title: pdfTitleFromHint(titleHint, finalURL)}
		return resolvedArticle{Kind: "pdf", Value: finalURL, Meta: meta, pdfVerified: true}, nil
	}
	if mediaType != "" && mediaType != "text/html" && mediaType != "application/xhtml+xml" {
		return resolvedArticle{}, fmt.Errorf("%w: unsupported content type %s", errUnsupportedArticleURL, mediaType)
	}
	rest, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)-int64(n)))
	if err != nil {
		return resolvedArticle{}, err
	}
	raw := append(head, rest...)

	metadata := parseArticleHTML(raw, finalURL)
	if metadata.Title == "" {
		metadata.Title = titleHint
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
	return resolvedArticle{Meta: metadata}, errUnsupportedArticleURL
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
	if host == "ar5iv.labs.arxiv.org" || host == "ar5iv.org" || host == "www.ar5iv.org" {
		if id, err := NormalizeArxivID(parsed.Path); err == nil {
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
	// Single-label names ("postgres", "redis") are container or intranet hosts.
	if !strings.Contains(host, ".") && net.ParseIP(host) == nil {
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

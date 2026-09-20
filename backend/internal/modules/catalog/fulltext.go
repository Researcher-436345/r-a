package catalog

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/content"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// discoveryBudget bounds one find-fulltext run; every step inside also has
// its own timeout.
var discoveryBudget = 45 * time.Second

type fullTextOutcome string

const (
	fullTextNone fullTextOutcome = ""
	fullTextPDF  fullTextOutcome = "pdf"  // a PDF is ready or its download is queued
	fullTextText fullTextOutcome = "text" // Europe PMC body stored for overview/assistant
)

// findFullText is POST /papers/{paperID}/find-fulltext: the reader's "Find
// full text" button for papers that opened without a PDF.
func (a API) findFullText(w http.ResponseWriter, r *http.Request) {
	id, ok := a.requirePaper(w, r)
	if !ok || !a.allow(w, r, ruleFullTextUser) {
		return
	}
	outcome, err := a.discoverFullText(r.Context(), id)
	if err != nil {
		writeResolveError(w, err, false)
		return
	}
	if outcome == fullTextNone {
		httpx.ErrorCode(w, http.StatusUnprocessableEntity, "not_found", "No open-access full text was found for this paper")
		return
	}
	a.paperResponse(w, r, id, http.StatusOK)
}

// discoverFullText re-runs open-access discovery for an existing paper and
// attaches what it finds to that same record.
func (a API) discoverFullText(ctx context.Context, paperID uuid.UUID) (fullTextOutcome, error) {
	paper, err := a.store().GetPaperOut(ctx, paperID)
	if err != nil {
		return fullTextNone, err
	}
	latest, err := a.store().LatestVersion(ctx, paperID)
	hasLatest := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fullTextNone, err
	}
	if hasLatest && latest.PDFKey != nil && latest.Status == "ready" {
		return fullTextPDF, nil
	}
	if hasLatest && latest.Status == "processing" {
		return fullTextPDF, a.requeueRemotePDF(latest)
	}

	ctx, cancel := context.WithTimeout(ctx, discoveryBudget)
	defer cancel()

	if paper.ArxivID != nil && *paper.ArxivID != "" {
		if attached, attachErr := a.attachArxivIfServed(ctx, paper, *paper.ArxivID); attached || attachErr != nil {
			return fullTextPDF, attachErr
		}
	}

	doi := ""
	if paper.DOI != nil {
		doi = *paper.DOI
	}
	titleSpecific := titleIsSpecific(paper.Title)
	authors := make([]string, 0, len(paper.Authors))
	for _, author := range paper.Authors {
		authors = append(authors, author.Name)
	}
	arxivCh := make(chan string, 1)
	go func() {
		id := ""
		if titleSpecific && (paper.ArxivID == nil || *paper.ArxivID == "") {
			if found, ok := findArxivForPaper(ctx, paper.Title, authors, paper.Year); ok {
				id = found
			}
		}
		arxivCh <- id
	}()

	var works []openAlexWork
	if doi != "" {
		if work, lookupErr := lookupOpenAlexByDOI(ctx, doi); lookupErr == nil {
			works = append(works, work)
		}
	}
	if len(works) == 0 && titleSpecific {
		callCtx, stop := context.WithTimeout(ctx, indexCallTimeout)
		matches, lookupErr := lookupOpenAlexByTitle(callCtx, paper.Title)
		stop()
		if lookupErr == nil {
			for _, match := range matches {
				// A title twin under another DOI is another paper.
				if doi != "" && match.doi() != "" && match.doi() != doi {
					continue
				}
				// Found by title only, and its arXiv id or PDF lands on this record.
				if !titlesMatchStrict(match.title(), paper.Title) || !confirmsPaper(authors, paper.Year, match.authors(), match.year()) {
					continue
				}
				works = append(works, match)
				break
			}
		}
	}

	arxivID := ""
	for _, work := range works {
		if rec := work.record(); rec.ArxivID != "" {
			arxivID = rec.ArxivID
			break
		}
	}
	if byTitle := <-arxivCh; arxivID == "" {
		arxivID = byTitle
	}
	if arxivID != "" && (paper.ArxivID == nil || *paper.ArxivID != arxivID) {
		if attached, attachErr := a.attachArxivIfServed(ctx, paper, arxivID); attached || attachErr != nil {
			return fullTextPDF, attachErr
		}
	}

	var candidates []string
	pmcid := ""
	for _, work := range works {
		candidates = append(candidates, work.pdfCandidates()...)
		if pmcid == "" {
			pmcid = work.pmcid()
		}
	}
	skipURL := ""
	if hasLatest && latest.Status == "failed" && latest.SourceURL != nil {
		if permanentDownloadFailure(latest.ErrorMessage) {
			skipURL = *latest.SourceURL
		} else {
			// A download that failed earlier may work now (hosts have bad days).
			candidates = append(candidates, *latest.SourceURL)
		}
	}
	list := withoutURL(pdfCandidateList(candidates...), skipURL)
	if hasLatest {
		if paper.HasFullText {
			// The stored text stays as it is; only a PDF would be a find.
			if pdf := firstVerifiedPDF(ctx, list); pdf != "" {
				return fullTextPDF, a.attachRemotePDF(ctx, paperID, pdf)
			}
			return fullTextNone, nil
		}
		outcome, attachErr := a.attachOpenFullText(ctx, paperID, latest.ID, doi, pmcid, list)
		if attachErr != nil {
			log.Printf("catalog: find-fulltext %s: %v", paperID, attachErr)
		}
		return outcome, nil
	}
	if pdf := firstVerifiedPDF(ctx, list); pdf != "" {
		return fullTextPDF, a.attachRemotePDF(ctx, paperID, pdf)
	}
	return fullTextNone, nil
}

// withoutURL drops skip (compared after normalization) from candidates.
func withoutURL(candidates []string, skip string) []string {
	if skip == "" {
		return candidates
	}
	if normalized, err := normalizeArticleURL(skip); err == nil {
		skip = normalized
	}
	out := candidates[:0:0]
	for _, candidate := range candidates {
		if candidate != skip {
			out = append(out, candidate)
		}
	}
	return out
}

// attachArxivIfServed adds an arXiv PDF version to this paper when arXiv
// really serves the PDF, and records the arXiv id when no other paper owns it
// (the worker then prefers the TeX source).
func (a API) attachArxivIfServed(ctx context.Context, paper PaperOut, arxivID string) (bool, error) {
	arxivID = CanonicalArxivID(arxivID)
	pdfURL := "https://arxiv.org/pdf/" + arxivID
	if !probePDF(ctx, pdfURL) {
		return false, nil
	}
	if paper.ArxivID == nil || *paper.ArxivID == "" {
		// The id makes this record the one every arXiv link opens, so only
		// metadata from an index may carry it; a scraped page's title and
		// abstract (web_pdf) could be anything.
		indexed := paper.DOI != nil && *paper.DOI != ""
		if !indexed {
			found, err := a.store().HasVersionFrom(ctx, paper.ID, "doi", "openalex")
			if err != nil {
				return false, err
			}
			indexed = found
		}
		owner, err := a.store().FindByArxiv(ctx, arxivID)
		if err != nil {
			return false, err
		}
		if indexed && owner == nil {
			if err = a.store().UpdatePaperMeta(ctx, paper.ID, paper.Title, nil, nil, nil, nil, &arxivID); err != nil && !isUniqueViolation(err) {
				return false, err
			}
		}
	}
	return true, a.attachPDFVersion(ctx, paper.ID, "arxiv", pdfURL)
}

// attachOpenFullText attaches the first verified PDF candidate, or else
// stores Europe PMC's full text against the given version.
func (a API) attachOpenFullText(ctx context.Context, paperID, versionID uuid.UUID, doi, pmcid string, candidates []string) (fullTextOutcome, error) {
	if pdf := firstVerifiedPDF(ctx, candidates); pdf != "" {
		if err := a.attachRemotePDF(ctx, paperID, pdf); err != nil {
			return fullTextNone, err
		}
		return fullTextPDF, nil
	}
	saved, err := a.saveEuropePMCText(ctx, paperID, versionID, doi, pmcid)
	if saved {
		return fullTextText, nil
	}
	return fullTextNone, err
}

// saveEuropePMCText stores the JATS body as the paper's document so the
// overview and the assistant read the full text even without a PDF.
func (a API) saveEuropePMCText(ctx context.Context, paperID, versionID uuid.UUID, doi, pmcid string) (bool, error) {
	if a.DB == nil || (doi == "" && pmcid == "") {
		return false, nil
	}
	fetchCtx, cancel := context.WithTimeout(ctx, europePMCTimeout)
	defer cancel()
	if pmcid == "" {
		found, err := searchEuropePMCByDOI(fetchCtx, strings.ToLower(doi))
		if err != nil {
			return false, err
		}
		pmcid = found
	}
	if pmcid == "" {
		return false, nil
	}
	markdown, plain, err := fetchEuropePMCFullText(fetchCtx, pmcid)
	if err != nil {
		if errors.Is(err, errUnsupportedArticleURL) {
			return false, nil
		}
		return false, err
	}
	if plain == "" {
		return false, nil
	}
	parsed := content.ChunkPlainText(plain, 1000)
	chunks := make([]content.Chunk, 0, len(parsed))
	for i, c := range parsed {
		var section *string
		if value := strings.TrimSpace(c.Section); value != "" {
			section = &value
		}
		chunks = append(chunks, content.Chunk{
			ID:            uuid.New(),
			PaperID:       paperID,
			VersionID:     versionID,
			ChunkIndex:    i,
			PageStart:     c.PageStart,
			PageEnd:       c.PageEnd,
			Section:       section,
			Text:          c.Text,
			TokenEstimate: c.TokenEstimate,
		})
	}
	docs := content.Store{DB: a.DB}
	if err = docs.SaveReady(ctx, paperID, versionID, europePMCEngine, false, 0, markdown, plain, chunks); err != nil {
		return false, err
	}
	return true, nil
}

package catalog

import (
	"context"
	"log"

	"github.com/google/uuid"
)

// addOpenAlexPaper stores a paper that exists only as an OpenAlex record
// (no arXiv version, no Crossref DOI): source "openalex", status ready, no
// PDF, deduplicated by the work URL. The reader shows its abstract; a
// verified PDF or Europe PMC text is attached when one exists.
func (a API) addOpenAlexPaper(ctx context.Context, userID uuid.UUID, work openAlexWork, add bool) (uuid.UUID, error) {
	workURL := work.workURL()
	meta := work.metadata()
	if workURL == "" || meta.Title == "" {
		return uuid.Nil, errUnsupportedArticleURL
	}
	existing, err := a.store().FindVersionBySourceURL(ctx, workURL)
	if err != nil {
		return uuid.Nil, err
	}
	if existing != nil {
		return existing.PaperID, a.addMembership(ctx, userID, existing.PaperID, add)
	}
	if meta.DOI != "" {
		byDOI, findErr := a.store().FindByDOI(ctx, meta.DOI)
		if findErr != nil {
			return uuid.Nil, findErr
		}
		if byDOI != nil {
			return byDOI.ID, a.addMembership(ctx, userID, byDOI.ID, add)
		}
	}

	verified := firstVerifiedPDF(ctx, work.pdfCandidates())
	if verified != "" {
		byPDF, findErr := a.store().FindVersionBySourceURL(ctx, verified)
		if findErr != nil {
			return uuid.Nil, findErr
		}
		if byPDF != nil {
			if err = a.attachRemotePDF(ctx, byPDF.PaperID, verified); err != nil {
				return uuid.Nil, err
			}
			return byPDF.PaperID, a.addMembership(ctx, userID, byPDF.PaperID, add)
		}
	}

	var abstract, venue, doi *string
	if meta.Abstract != "" {
		abstract = &meta.Abstract
	}
	if value := truncateRunes(meta.Venue, 500); value != "" {
		venue = &value
	}
	if meta.DOI != "" {
		doi = &meta.DOI
	}
	created, err := a.store().CreatePaper(ctx, meta.Title, abstract, meta.Year, venue, doi, nil)
	if err != nil {
		if isUniqueViolation(err) && doi != nil {
			if again, findErr := a.store().FindByDOI(ctx, *doi); findErr == nil && again != nil {
				return again.ID, a.addMembership(ctx, userID, again.ID, add)
			}
		}
		return uuid.Nil, err
	}
	if err = a.store().AttachAuthors(ctx, created.ID, meta.Authors); err != nil {
		return uuid.Nil, err
	}
	version, err := a.store().CreateVersion(ctx, created.ID, 1, "openalex", &workURL, nil, nil, nil, "ready")
	if err != nil {
		return uuid.Nil, err
	}
	if err = a.addMembership(ctx, userID, created.ID, add); err != nil {
		return uuid.Nil, err
	}
	if verified != "" {
		if err = a.attachRemotePDF(ctx, created.ID, verified); err != nil {
			log.Printf("catalog: attach pdf for %s: %v", workURL, err)
		}
		return created.ID, nil
	}
	if _, err = a.saveEuropePMCText(ctx, created.ID, version.ID, meta.DOI, work.pmcid()); err != nil {
		log.Printf("catalog: europe pmc text for %s: %v", workURL, err)
	}
	return created.ID, nil
}

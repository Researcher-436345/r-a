package catalog

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Open-access resolution: turn a DOI or a bare title into something the reader
// can actually open — an arXiv id (best: TeX-first parse), a DOI record, a
// verified PDF, or at least the OpenAlex metadata and abstract.
// OpenAlex is used because it answers from the server's network; Semantic
// Scholar's API returns 403 there and OpenReview challenges every client.

var (
	openAlexBaseURL   = "https://api.openalex.org"
	arxivQueryBaseURL = "https://export.arxiv.org/api/query"
	openAlexMailto    = "researcher@localhost"
	arxivLandingRE    = regexp.MustCompile(`(?i)arxiv\.org/(?:abs|pdf|html)/(\d{4}\.\d{4,5}|[a-z-]+(?:\.[A-Z]{2})?/\d{7})(?:v\d+)?`)
	arxivDOIRE        = regexp.MustCompile(`(?i)10\.48550/arxiv\.(\d{4}\.\d{4,5})`)
	nonAlnumRE        = regexp.MustCompile(`[^a-z0-9]+`)
	htmlTagRE         = regexp.MustCompile(`<[^<>]{1,200}>`)
	abstractLabelRE   = regexp.MustCompile(`(?i)^\s*abstract[\s.:—-]+`)

	// Bounds for index lookups; variables so tests can shrink them.
	indexCallTimeout    = 6 * time.Second
	crossrefCallTimeout = 6 * time.Second
	titleResolveBudget  = 20 * time.Second
)

const openAlexSelect = "id,title,display_name,type,publication_year,ids,doi,authorships,primary_location,best_oa_location,locations,abstract_inverted_index"

// openAccessRecord is what we could learn about a paper from open indexes.
type openAccessRecord struct {
	Title   string
	DOI     string
	ArxivID string
	PDFURL  string
}

// normalizeTitle collapses a title to lowercase alphanumerics so that titles
// from different indexes can be compared despite punctuation and casing.
func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, s)
	return strings.Trim(nonAlnumRE.ReplaceAllString(s, " "), " ")
}

// titleIsSpecific rejects titles too generic to identify a paper by name:
// "Deep learning" (Nature, 2015) equals a dozen unrelated arXiv titles.
func titleIsSpecific(title string) bool {
	normalized := normalizeTitle(title)
	return len(normalized) >= 20 && len(strings.Fields(normalized)) >= 4
}

// titlesMatch accepts equal titles and titles that differ only by a short
// prefix/suffix (subtitles, "(extended abstract)"), never by their core.
func titlesMatch(a, b string) bool {
	na, nb := normalizeTitle(a), normalizeTitle(b)
	if na == "" || nb == "" {
		return false
	}
	if na == nb {
		return true
	}
	shorter, longer := na, nb
	if len(shorter) > len(longer) {
		shorter, longer = longer, shorter
	}
	if len(shorter) < 20 || len(strings.Fields(shorter)) < 4 {
		return false
	}
	// A subtitle after the colon is routinely dropped by one of the indexes:
	// "Foundations and Models in Modern Computer Vision" vs the same title
	// plus ": Key Building Blocks in Landmark Architectures".
	if strings.HasPrefix(longer, shorter+" ") {
		return true
	}
	return strings.Contains(longer, shorter) && len(shorter)*10 >= len(longer)*7
}

var subtitleSeparatorRE = regexp.MustCompile(`:|\s[-–—]\s`)

// titlesMatchStrict guards discovery that writes to an existing paper: equal
// titles, or a title plus a subtitle after ":" / " - " when the shared part is
// most of it. titlesMatch alone lets "Deep Learning for Computer Vision" pick
// up "Deep Learning for Computer Vision in Agriculture".
func titlesMatchStrict(a, b string) bool {
	na, nb := normalizeTitle(a), normalizeTitle(b)
	if na == "" || nb == "" {
		return false
	}
	if na == nb {
		return true
	}
	shortRaw, longRaw := a, b
	if len(na) > len(nb) {
		shortRaw, longRaw, na, nb = b, a, nb, na
	}
	if !titleIsSpecific(shortRaw) || len(na)*10 < len(nb)*7 {
		return false
	}
	loc := subtitleSeparatorRE.FindStringIndex(longRaw)
	return loc != nil && normalizeTitle(longRaw[:loc[0]]) == na
}

// confirmsPaper: a candidate found by title is the stored paper only if it
// also shares an author surname or appeared within a year of it.
func confirmsPaper(storedAuthors []string, storedYear *int, candidateAuthors []string, candidateYear *int) bool {
	if storedYear != nil && candidateYear != nil {
		if d := *storedYear - *candidateYear; d >= -1 && d <= 1 {
			return true
		}
	}
	stored := map[string]bool{}
	for _, name := range storedAuthors {
		if s := surnameOf(name); s != "" {
			stored[s] = true
		}
	}
	for _, name := range candidateAuthors {
		if s := surnameOf(name); s != "" && stored[s] {
			return true
		}
	}
	return false
}

// surnameOf handles "Yuxi Li" and "Li, Yuxi".
func surnameOf(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.Index(name, ","); i > 0 {
		return normalizeTitle(name[:i])
	}
	fields := strings.Fields(normalizeTitle(name))
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

// Process-wide caps on concurrent index calls, so many users resolving links
// at once do not get the server's IP throttled by arXiv or OpenAlex for everyone.
var (
	arxivAPISlots    = make(chan struct{}, 4)
	openAlexAPISlots = make(chan struct{}, 8)
)

func takeSlot(ctx context.Context, slots chan struct{}) (func(), error) {
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// arXiv's phrase search (ti:"…") is slow and times out under load; an AND of
// the distinctive words answers in well under a second and the title check
// afterwards keeps it precise.
var arxivStopWords = map[string]bool{
	"and": true, "the": true, "for": true, "with": true, "from": true, "into": true,
	"over": true, "via": true, "using": true, "toward": true, "towards": true, "based": true,
	"your": true, "our": true, "its": true, "are": true, "not": true, "all": true, "you": true,
}

func arxivTitleQuery(title string) string {
	terms := make([]string, 0, 8)
	for _, word := range strings.Fields(normalizeTitle(title)) {
		if len(word) < 3 || arxivStopWords[word] {
			continue
		}
		terms = append(terms, "ti:"+word)
		if len(terms) == 8 {
			break
		}
	}
	return strings.Join(terms, " AND ")
}

type openAlexWork struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	DisplayName     string `json:"display_name"`
	Type            string `json:"type"`
	PublicationYear int    `json:"publication_year"`
	DOI             string `json:"doi"`
	IDs             struct {
		DOI   string `json:"doi"`
		PMCID string `json:"pmcid"`
	} `json:"ids"`
	Authorships []struct {
		Author struct {
			DisplayName string `json:"display_name"`
		} `json:"author"`
	} `json:"authorships"`
	PrimaryLocation       *openAlexLocation  `json:"primary_location"`
	BestOA                *openAlexLocation  `json:"best_oa_location"`
	Locations             []openAlexLocation `json:"locations"`
	AbstractInvertedIndex map[string][]int   `json:"abstract_inverted_index"`
}

type openAlexLocation struct {
	LandingPageURL string `json:"landing_page_url"`
	PDFURL         string `json:"pdf_url"`
	IsOA           bool   `json:"is_oa"`
	Source         *struct {
		DisplayName string `json:"display_name"`
	} `json:"source"`
}

// nonPaperWorkTypes are OpenAlex (and passed-through Crossref) types that
// share a paper's title without being it: the top OpenAlex hit for
// "Computer Vision: Algorithms and Applications" is a Choice book review.
var nonPaperWorkTypes = map[string]bool{
	"book-review": true, "paratext": true, "erratum": true, "retraction": true, "peer-review": true,
	"supplementary-materials": true, "grant": true, "libguides": true, "editorial": true,
	"proceedings": true, "proceedings-series": true, "book-series": true, "book-set": true,
	"journal": true, "journal-volume": true, "journal-issue": true, "report-series": true, "component": true,
}

// workTypeAcceptable keeps editorials and errata when the link itself asked
// for one ("Editorial- Deep Learning for Computer Vision").
func workTypeAcceptable(workType, hint string) bool {
	t := strings.ToLower(strings.TrimSpace(workType))
	if !nonPaperWorkTypes[t] {
		return true
	}
	switch t {
	case "editorial", "erratum", "retraction":
		return strings.Contains(strings.ToLower(hint), t)
	}
	return false
}

func (w openAlexWork) title() string {
	return cleanMetadataValue(htmlTagRE.ReplaceAllString(firstNonEmpty(w.Title, w.DisplayName), ""))
}

func (w openAlexWork) doi() string {
	for _, raw := range []string{w.IDs.DOI, w.DOI} {
		if raw == "" {
			continue
		}
		if doi, err := NormalizeDOI(raw); err == nil {
			return doi
		}
	}
	return ""
}

// workURL is the OpenAlex work URL used as the metadata version's source_url.
func (w openAlexWork) workURL() string {
	id := strings.TrimSpace(w.ID)
	if strings.HasPrefix(id, "https://openalex.org/W") {
		return id
	}
	if strings.HasPrefix(id, "W") && len(id) > 1 {
		return "https://openalex.org/" + id
	}
	return ""
}

func (w openAlexWork) authors() []string {
	var names []string
	for _, a := range w.Authorships {
		if name := cleanMetadataValue(a.Author.DisplayName); name != "" {
			names = appendUnique(names, name)
		}
		if len(names) == 100 {
			break
		}
	}
	return names
}

func (w openAlexWork) year() *int {
	if w.PublicationYear < 1000 || w.PublicationYear > 3000 {
		return nil
	}
	year := w.PublicationYear
	return &year
}

func (w openAlexWork) venue() string {
	for _, loc := range w.allLocations() {
		if loc.Source != nil && strings.TrimSpace(loc.Source.DisplayName) != "" {
			return cleanMetadataValue(loc.Source.DisplayName)
		}
	}
	return ""
}

func (w openAlexWork) pmcid() string {
	if w.IDs.PMCID != "" {
		return normalizePMCID(w.IDs.PMCID)
	}
	for _, loc := range w.allLocations() {
		if strings.Contains(loc.LandingPageURL, "/pmc/articles/") {
			return normalizePMCID(loc.LandingPageURL)
		}
	}
	return ""
}

func (w openAlexWork) allLocations() []openAlexLocation {
	out := make([]openAlexLocation, 0, len(w.Locations)+2)
	if w.BestOA != nil {
		out = append(out, *w.BestOA)
	}
	if w.PrimaryLocation != nil {
		out = append(out, *w.PrimaryLocation)
	}
	return append(out, w.Locations...)
}

// metadata is the work as the catalog stores it.
func (w openAlexWork) metadata() articleMetadata {
	year := w.year()
	return articleMetadata{
		Title:    w.title(),
		Abstract: rebuildAbstract(w.AbstractInvertedIndex),
		Venue:    w.venue(),
		DOI:      w.doi(),
		Authors:  w.authors(),
		Year:     year,
	}
}

func (w openAlexWork) record() openAccessRecord {
	rec := openAccessRecord{Title: w.title(), DOI: w.doi()}
	if m := arxivDOIRE.FindStringSubmatch(rec.DOI); len(m) > 1 {
		rec.ArxivID = CanonicalArxivID(m[1])
		return rec
	}
	locations := w.allLocations()
	for _, loc := range locations {
		for _, candidate := range []string{loc.LandingPageURL, loc.PDFURL} {
			if m := arxivLandingRE.FindStringSubmatch(candidate); len(m) > 1 {
				rec.ArxivID = CanonicalArxivID(m[1])
				return rec
			}
			if m := arxivDOIRE.FindStringSubmatch(candidate); len(m) > 1 {
				rec.ArxivID = CanonicalArxivID(m[1])
				return rec
			}
		}
	}
	for _, loc := range locations {
		if loc.PDFURL != "" && loc.IsOA {
			rec.PDFURL = loc.PDFURL
			break
		}
	}
	return rec
}

// pdfCandidates lists every PDF-looking location in preference order: open
// access PDFs, then PDFs OpenAlex considers closed (they often download
// fine), then landing pages that are themselves PDFs. None is trusted until
// probed.
func (w openAlexWork) pdfCandidates() []string {
	locations := w.allLocations()
	var raw []string
	for _, loc := range locations {
		if loc.IsOA && loc.PDFURL != "" {
			raw = append(raw, loc.PDFURL)
		}
	}
	for _, loc := range locations {
		if !loc.IsOA && loc.PDFURL != "" {
			raw = append(raw, loc.PDFURL)
		}
	}
	for _, loc := range locations {
		if strings.EqualFold(path.Ext(parsePath(loc.LandingPageURL)), ".pdf") {
			raw = append(raw, loc.LandingPageURL)
		}
	}
	return pdfCandidateList(raw...)
}

// rebuildAbstract turns OpenAlex's abstract_inverted_index back into text.
func rebuildAbstract(index map[string][]int) string {
	const maxWords = 5000
	maxPos := -1
	for _, positions := range index {
		for _, p := range positions {
			if p >= 0 && p < maxWords && p > maxPos {
				maxPos = p
			}
		}
	}
	if maxPos < 0 {
		return ""
	}
	slots := make([]string, maxPos+1)
	for word, positions := range index {
		for _, p := range positions {
			if p >= 0 && p <= maxPos {
				slots[p] = word
			}
		}
	}
	text := strings.Join(strings.Fields(strings.Join(slots, " ")), " ")
	text = strings.Join(strings.Fields(htmlTagRE.ReplaceAllString(text, " ")), " ")
	text = strings.TrimSpace(abstractLabelRE.ReplaceAllString(text, ""))
	return truncateRunes(text, 10000)
}

func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit])
}

func openAlexGet(ctx context.Context, endpoint string, out any) error {
	release, err := takeSlot(ctx, openAlexAPISlots)
	if err != nil {
		return err
	}
	defer release()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "researcher-api/0.1 (mailto:"+openAlexMailto+")")
	req.Header.Set("Accept", "application/json")
	resp, err := articleHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errUnsupportedArticleURL
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("OpenAlex returned %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// lookupOpenAlexByDOI returns the OpenAlex work for a DOI.
func lookupOpenAlexByDOI(ctx context.Context, doi string) (openAlexWork, error) {
	ctx, cancel := context.WithTimeout(ctx, indexCallTimeout)
	defer cancel()
	var work openAlexWork
	endpoint := openAlexBaseURL + "/works/https://doi.org/" + url.PathEscape(doi) +
		"?select=" + openAlexSelect + "&mailto=" + url.QueryEscape(openAlexMailto)
	if err := openAlexGet(ctx, endpoint, &work); err != nil {
		return openAlexWork{}, err
	}
	if work.IDs.DOI == "" && work.DOI == "" {
		work.IDs.DOI = doi
	}
	return work, nil
}

// lookupOpenAlexByTitle searches OpenAlex for a title and returns, in rank
// order, the works whose title really matches and whose type is a paper —
// search results are ranked loosely.
func lookupOpenAlexByTitle(ctx context.Context, title string) ([]openAlexWork, error) {
	results, err := searchOpenAlexWorks(ctx, title, 8)
	if err != nil {
		return nil, err
	}
	var matches []openAlexWork
	for _, work := range results {
		if titlesMatch(work.title(), title) && workTypeAcceptable(work.Type, title) {
			matches = append(matches, work)
		}
	}
	return matches, nil
}

// searchOpenAlexWorks returns OpenAlex's relevance-ranked search results.
func searchOpenAlexWorks(ctx context.Context, title string, perPage int) ([]openAlexWork, error) {
	var out struct {
		Results []openAlexWork `json:"results"`
	}
	endpoint := openAlexBaseURL + "/works?search=" + url.QueryEscape(title) +
		"&select=" + openAlexSelect + fmt.Sprintf("&per_page=%d", perPage) + "&mailto=" + url.QueryEscape(openAlexMailto)
	if err := openAlexGet(ctx, endpoint, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

type arxivSearchEntry struct {
	ID        string `xml:"id"`
	Title     string `xml:"title"`
	Published string `xml:"published"`
	Authors   []struct {
		Name string `xml:"name"`
	} `xml:"author"`
}

func (e arxivSearchEntry) authorNames() []string {
	names := make([]string, 0, len(e.Authors))
	for _, author := range e.Authors {
		names = append(names, author.Name)
	}
	return names
}

func (e arxivSearchEntry) year() *int {
	return yearFromText(e.Published)
}

// searchArxiv runs an arXiv API query (e.g. "ti:deep AND au:li").
func searchArxiv(ctx context.Context, query string, maxResults int) ([]arxivSearchEntry, error) {
	release, err := takeSlot(ctx, arxivAPISlots)
	if err != nil {
		return nil, err
	}
	defer release()
	endpoint := arxivQueryBaseURL + fmt.Sprintf("?max_results=%d&search_query=", maxResults) + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "researcher-api/0.1 (paper-library)")
	resp, err := articleHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("arXiv search returned %s", resp.Status)
	}
	var feed struct {
		Entries []arxivSearchEntry `xml:"entry"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&feed); err != nil {
		return nil, err
	}
	return feed.Entries, nil
}

// findArxivByTitle asks arXiv for an exact-title match.
func findArxivByTitle(ctx context.Context, title string) (string, bool, error) {
	if !titleIsSpecific(title) {
		return "", false, nil
	}
	query := arxivTitleQuery(title)
	if query == "" {
		return "", false, nil
	}
	entries, err := searchArxiv(ctx, query, 5)
	if err != nil {
		return "", false, err
	}
	for _, entry := range entries {
		if !titlesMatch(entry.Title, title) {
			continue
		}
		if id, err := NormalizeArxivID(entry.ID); err == nil {
			return CanonicalArxivID(id), true, nil
		}
	}
	return "", false, nil
}

// findArxivForPaper is the title search for an existing record: the arXiv
// entry must match strictly and share an author or the year, because its id is
// then written onto the record for every user.
func findArxivForPaper(ctx context.Context, title string, authors []string, year *int) (string, bool) {
	query := arxivTitleQuery(title)
	if !titleIsSpecific(title) || query == "" {
		return "", false
	}
	ctx, cancel := context.WithTimeout(ctx, indexCallTimeout)
	defer cancel()
	entries, err := searchArxiv(ctx, query, 5)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if !titlesMatchStrict(entry.Title, title) || !confirmsPaper(authors, year, entry.authorNames(), entry.year()) {
			continue
		}
		if id, idErr := NormalizeArxivID(entry.ID); idErr == nil {
			return CanonicalArxivID(id), true
		}
	}
	return "", false
}

// findArxivByTitleBounded is findArxivByTitle under the per-call timeout.
func findArxivByTitleBounded(ctx context.Context, title string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, indexCallTimeout)
	defer cancel()
	id, ok, err := findArxivByTitle(ctx, title)
	return id, ok && err == nil
}

// resolveByTitle turns a paper title (usually the link text the assistant
// wrote) into an arXiv id, a DOI, or an OpenAlex work when the page itself
// cannot be inspected. arXiv and OpenAlex are asked concurrently, each under
// its own timeout; an arXiv match wins because it opens with full text.
func resolveByTitle(ctx context.Context, rawTitle string) (resolvedArticle, bool) {
	title := cleanTitleHint(rawTitle)
	if !titleIsSpecific(title) {
		return resolvedArticle{}, false
	}
	ctx, cancel := context.WithTimeout(ctx, titleResolveBudget)
	defer cancel()

	type arxivResult struct {
		id string
		ok bool
	}
	type openAlexResult struct {
		works []openAlexWork
		err   error
	}
	arxivCh := make(chan arxivResult, 1)
	openAlexCh := make(chan openAlexResult, 1)
	go func() {
		id, ok := findArxivByTitleBounded(ctx, title)
		arxivCh <- arxivResult{id: id, ok: ok}
	}()
	go func() {
		callCtx, stop := context.WithTimeout(ctx, indexCallTimeout)
		defer stop()
		works, err := lookupOpenAlexByTitle(callCtx, title)
		openAlexCh <- openAlexResult{works: works, err: err}
	}()

	if r := <-arxivCh; r.ok {
		return resolvedArticle{Kind: "arxiv", Value: r.id, Meta: articleMetadata{Title: title}, FromTitle: true}, true
	}
	found := <-openAlexCh
	if found.err != nil {
		return resolvedArticle{}, false
	}
	for _, work := range found.works {
		if resolved, ok := resolveOpenAlexWork(ctx, work, title); ok {
			return resolved, true
		}
	}
	return resolvedArticle{}, false
}

// resolveOpenAlexWork applies the match order: arXiv location, then a DOI
// that Crossref confirms is a real work, then the OpenAlex record itself
// (metadata-only paper; a verified PDF is attached when it is created).
func resolveOpenAlexWork(ctx context.Context, work openAlexWork, hint string) (resolvedArticle, bool) {
	meta := work.metadata()
	if meta.Title == "" {
		meta.Title = hint
	}
	rec := work.record()
	w := work
	if rec.ArxivID != "" {
		return resolvedArticle{Kind: "arxiv", Value: rec.ArxivID, Meta: meta, Work: &w, FromTitle: true}, true
	}
	if rec.DOI != "" {
		callCtx, stop := context.WithTimeout(ctx, crossrefCallTimeout)
		crossref, err := FetchCrossrefMetadata(callCtx, rec.DOI)
		stop()
		if err == nil {
			if isContainerCrossrefType(crossref.Type) {
				return resolvedArticle{}, false
			}
			meta.PDFURL = rec.PDFURL
			return resolvedArticle{Kind: "doi", Value: rec.DOI, Meta: meta, Work: &w, Crossref: &crossref, FromTitle: true}, true
		}
		// Not in Crossref (DataCite, regional registries) or Crossref is
		// down: the OpenAlex record still identifies the paper.
	}
	if work.workURL() == "" || meta.Title == "" {
		return resolvedArticle{}, false
	}
	return resolvedArticle{Kind: "openalex", Value: work.workURL(), Meta: meta, Work: &w, FromTitle: true}, true
}

// hostBlocksScrapers lists sites that never yield metadata to a plain GET:
// Semantic Scholar answers 403 and renders client-side, OpenReview requires a
// browser challenge. Fetching them only burns time before the title fallback.
func hostBlocksScrapers(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	switch host {
	case "semanticscholar.org", "openreview.net", "api.semanticscholar.org", "sciencedirect.com", "ieeexplore.ieee.org", "dl.acm.org", "link.springer.com":
		return true
	}
	return false
}

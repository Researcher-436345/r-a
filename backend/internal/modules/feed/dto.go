package feed

type TrendingPaper struct {
	ArxivID         string   `json:"arxiv_id"`
	Title           string   `json:"title"`
	Abstract        *string  `json:"abstract"`
	Authors         []string `json:"authors"`
	PublishedAt     string   `json:"published_at"`
	Category        string   `json:"category"`
	PopularityScore float64  `json:"popularity_score"`
	PDFURL          string   `json:"pdf_url"`
	AbsURL          string   `json:"abs_url"`
}

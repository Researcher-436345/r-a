package catalog

import (
	"context"
	"io"
	"net/http"
	"time"
)

// Index PDF links are often wrong: publishers answer 403, PMC serves a
// browser challenge, "is_oa=false" links sometimes download fine. A version is
// attached only after the first bytes really are a PDF, so the reader never
// polls a download that was doomed from the start.

var pdfProbeTimeout = 8 * time.Second

const (
	maxPDFProbeCandidates = 6
	// Same identity as the worker's downloadPDFOnce: the probe must predict
	// what the worker will get.
	pdfProbeUserAgent = "Researcher/1.0 (paper-library; +https://github.com/Researcher-436345/r-a)"
)

func probePDF(ctx context.Context, rawURL string) bool {
	ctx, cancel := context.WithTimeout(ctx, pdfProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", pdfProbeUserAgent)
	req.Header.Set("Accept", "application/pdf,*/*")
	resp, err := pdfHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return false
	}
	// The worker refuses these, so a version for it would fail on every retry.
	if resp.ContentLength > maxPDFBytes {
		return false
	}
	head := make([]byte, 1024)
	n, _ := io.ReadFull(resp.Body, head)
	return hasPDFHeader(head[:n])
}

// firstVerifiedPDF probes candidates concurrently and returns the earliest one
// (in the given preference order) that serves a PDF.
func firstVerifiedPDF(ctx context.Context, candidates []string) string {
	if len(candidates) > maxPDFProbeCandidates {
		candidates = candidates[:maxPDFProbeCandidates]
	}
	if len(candidates) == 0 {
		return ""
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		index int
		ok    bool
	}
	results := make(chan result, len(candidates))
	for i, candidate := range candidates {
		go func(i int, candidate string) {
			results <- result{index: i, ok: probePDF(ctx, candidate)}
		}(i, candidate)
	}
	// 0 pending, 1 verified, -1 rejected.
	state := make([]int, len(candidates))
	for range candidates {
		r := <-results
		if r.ok {
			state[r.index] = 1
		} else {
			state[r.index] = -1
		}
		for i := range state {
			if state[i] == 0 {
				break
			}
			if state[i] == 1 {
				return candidates[i]
			}
		}
	}
	return ""
}

// pdfCandidateList normalizes, de-duplicates and drops URLs that can never be
// fetched (private hosts, non-http schemes).
func pdfCandidateList(raw ...string) []string {
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, value := range raw {
		if value == "" {
			continue
		}
		normalized, err := normalizeArticleURL(value)
		if err != nil || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out
}

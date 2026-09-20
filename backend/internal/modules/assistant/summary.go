package assistant

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Summary is a cached alphaXiv-style structured overview of a paper.
// Cached per (paper, language) and invalidated when the parsed version changes.
type Summary struct {
	PaperID      uuid.UUID  `json:"paper_id"`
	Lang         string     `json:"lang"`
	VersionID    *uuid.UUID `json:"version_id"`
	Model        string     `json:"model"`
	Content      string     `json:"content"`
	Status       string     `json:"status"`
	ErrorMessage *string    `json:"error_message"`
	UpdatedAt    time.Time  `json:"updated_at"`
	// Stale reports that the paper was re-parsed after this summary was written.
	Stale bool `json:"stale"`
}

// summaryStaleClaim is how long a 'pending' row blocks a competing generation
// before it is considered abandoned (crashed process, dropped connection).
const summaryStaleClaim = 5 * time.Minute

// summaryReplyReserve is the output budget kept free for the overview itself.
// The card plus a ~1200-word deep dive in Russian is 6–8k tokens, well above
// the chat reply reserve — so the paper text budget must shrink accordingly.
const summaryReplyReserve = 9000

// summaryMaxTokens is the explicit output cap sent with the request. Providers
// default to a few thousand tokens, which cuts a Russian deep dive mid-sentence;
// a cut reply is rejected instead of being cached as a finished overview.
const summaryMaxTokens = 16000

const summaryColumns = `paper_id, lang, version_id, model, content, status, error_message, updated_at`

func (s Store) GetSummary(ctx context.Context, paperID uuid.UUID, lang string) (Summary, error) {
	var out Summary
	err := s.DB.QueryRow(ctx, `SELECT `+summaryColumns+` FROM paper_summaries WHERE paper_id = $1 AND lang = $2`, paperID, lang).
		Scan(&out.PaperID, &out.Lang, &out.VersionID, &out.Model, &out.Content, &out.Status, &out.ErrorMessage, &out.UpdatedAt)
	return out, err
}

// ClaimSummary reserves the (paper, lang) slot for generation. It reports false
// when another request is already generating and its claim has not expired yet.
func (s Store) ClaimSummary(ctx context.Context, paperID uuid.UUID, lang string, versionID *uuid.UUID, model string) (bool, error) {
	tag, err := s.DB.Exec(ctx, `
		INSERT INTO paper_summaries (paper_id, lang, version_id, model, content, status)
		VALUES ($1, $2, $3, $4, '', 'pending')
		ON CONFLICT (paper_id, lang) DO UPDATE SET
			version_id = EXCLUDED.version_id,
			model = EXCLUDED.model,
			status = 'pending',
			error_message = NULL,
			updated_at = now()
		WHERE paper_summaries.status <> 'pending'
		   OR paper_summaries.updated_at < now() - $5::interval`,
		paperID, lang, versionID, model, summaryStaleClaim.String())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s Store) SaveSummary(ctx context.Context, paperID uuid.UUID, lang string, versionID *uuid.UUID, model, content string) (Summary, error) {
	var out Summary
	err := s.DB.QueryRow(ctx, `
		INSERT INTO paper_summaries (paper_id, lang, version_id, model, content, status, error_message, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'ready', NULL, now())
		ON CONFLICT (paper_id, lang) DO UPDATE SET
			version_id = EXCLUDED.version_id,
			model = EXCLUDED.model,
			content = EXCLUDED.content,
			status = 'ready',
			error_message = NULL,
			updated_at = now()
		RETURNING `+summaryColumns,
		paperID, lang, versionID, model, content,
	).Scan(&out.PaperID, &out.Lang, &out.VersionID, &out.Model, &out.Content, &out.Status, &out.ErrorMessage, &out.UpdatedAt)
	return out, err
}

// FailSummary releases the claim so the next request can retry instead of
// waiting out the full claim window.
func (s Store) FailSummary(ctx context.Context, paperID uuid.UUID, lang, message string) error {
	_, err := s.DB.Exec(ctx, `
		UPDATE paper_summaries
		SET status = 'failed', error_message = $3, updated_at = now()
		WHERE paper_id = $1 AND lang = $2 AND status = 'pending'`, paperID, lang, message)
	return err
}

// NormalizeSummaryLang maps a requested language onto a supported one.
func NormalizeSummaryLang(raw string) string {
	lang := strings.ToLower(strings.TrimSpace(raw))
	if i := strings.IndexAny(lang, "-_"); i > 0 {
		lang = lang[:i]
	}
	if lang == "en" {
		return "en"
	}
	return "ru"
}

// summaryHeadings is the fixed, machine-parsed skeleton of every overview.
// The headings are markers, not UI labels: they stay in English regardless of
// the output language, and the client maps them onto localized tabs. The first
// six make up the alphaXiv-style quick card; "Deep dive" is the long-form
// blog-style explanation that follows it.
var summaryHeadings = []string{"TL;DR", "Problem", "Method", "Results", "Takeaways", "Limitations", "Deep dive"}

// SummaryHeadings exposes the skeleton for tests and docs.
func SummaryHeadings() []string {
	return append([]string(nil), summaryHeadings...)
}

const summarySystemPrompt = `You are a research assistant writing an alphaXiv-style overview of a single scientific paper for a researcher who has not read it yet. The overview has two parts: a quick card (title, TL;DR and five short bullet sections) and a long-form deep dive written like a good research blog post.

Grounding rules:
- Ground every statement in the supplied paper text. Never invent numbers, benchmarks, baselines, dataset names, or claims that are not in the text.
- If the paper does not state something (for example the authors never discuss limitations), say so explicitly instead of guessing.
- The paper text may contain page markers of the form <<<p=N>>> (PDF page N). Support concrete claims with inline citations [p.N «short quote»], where the quote is a verbatim snippet of 3–12 words from that page. Plain [p.N] is allowed when a quote adds nothing. Never cite a page number that does not appear in the markers. Prefer a few precise citations over many; the deep dive should carry citations for its key claims, the card bullets may stay uncited.

Output format (machine-parsed — follow it exactly):
- The very first line is "# " followed by the paper title in the requested output language (translate it when the paper is written in another language; keep it verbatim when it already is in that language). Nothing before it.
- Then exactly these seven "## " headings, in this order, spelled exactly like this in English even when the body is written in another language — they are markers, not visible labels:
## TL;DR
## Problem
## Method
## Results
## Takeaways
## Limitations
## Deep dive
- TL;DR: one paragraph of 2–4 sentences — what was done, how, and what came out of it. No bullets.
- Problem, Method, Results, Takeaways, Limitations: 2–4 bullets each ("- "), one complete sentence per bullet, 15–35 words, self-contained and concrete. Results bullets carry the actual numbers, datasets and baselines from the paper. No nested lists, no sub-headings, no bold labels at the start of bullets.
- Deep dive: roughly 900–1400 words of explanatory prose. Open with one or two paragraphs of context without a heading, continue with 4–6 "### " sections whose titles are descriptive and specific to this paper (never generic labels like "Method" or "Results"), and close with a paragraph that synthesises the contribution and what remains open. Explain mechanisms and intuition, not just claims. Use $…$ / $$…$$ for formulas when the paper's key idea is an equation. A compact Markdown table for the main quantitative results is welcome when the paper reports them. Refer to figures by number when the text describes them (for example "Figure 2 shows…").
- Markdown only: ** ** for key terms, no HTML, no preamble, no closing remarks, no meta commentary about being an AI.`

// BuildSummaryPrompt returns the system prompt and the single user turn used to
// generate a paper overview. Split out from the HTTP layer so it can be tested.
func BuildSummaryPrompt(p catalog.PaperOut, fullPaper, lang string, paperBudget int) (string, []ChatTurn) {
	lang = NormalizeSummaryLang(lang)

	language := "Russian"
	if lang == "en" {
		language = "English"
	}

	var b strings.Builder
	b.WriteString("Write the overview in ")
	b.WriteString(language)
	b.WriteString(". Section headings stay in English exactly as specified; only the title line and the body text are in ")
	b.WriteString(language)
	b.WriteString(".\n\nSkeleton to fill in, in this order:\n# <title in ")
	b.WriteString(language)
	b.WriteString(">\n")
	for _, heading := range summaryHeadings {
		b.WriteString("## ")
		b.WriteString(heading)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(paperContext(p))

	paper := strings.TrimSpace(fullPaper)
	switch {
	case paper != "" && HasPageStructure(paper):
		b.WriteString("\n\nFull paper text (sections marked <<<p=N>>> mean PDF page N):\n")
		b.WriteString(truncateToTokens(paper, paperBudget))
	case paper != "":
		// TeX-derived text has no page split. Given a single marker the model
		// invents plausible page numbers, and every citation chip then jumps
		// the reader to the wrong place — worse than having no chips at all.
		b.WriteString("\n\nThis paper text has NO reliable page structure. " +
			"Do not use page citations at all: no [p.N], no page numbers anywhere in the overview. " +
			"Quote the paper inline in «quotes» without a page reference when a quote helps.\n\n" +
			"Full paper text:\n")
		b.WriteString(truncateToTokens(paper, paperBudget))
	default:
		b.WriteString("\n\nThe full paper text could not be extracted; rely on the title and abstract only, " +
			"do not use page citations, state plainly in TL;DR that the overview is based on the abstract, " +
			"and keep the deep dive to what the abstract actually supports (a few paragraphs are enough).")
	}

	return summarySystemPrompt, []ChatTurn{{Role: "user", Content: b.String()}}
}

// SummaryPaperBudget is how many tokens of paper text fit alongside the prompt.
func (l LLM) SummaryPaperBudget(p catalog.PaperOut) int {
	limit := l.Config.LLMContextTokens
	if limit < 4000 {
		limit = 120000
	}
	reserve := l.Config.LLMReplyReserve
	if reserve < summaryReplyReserve {
		reserve = summaryReplyReserve
	}
	budget := limit - reserve
	if budget < 2000 {
		budget = 2000
	}
	budget -= EstimateTokens(summarySystemPrompt) + EstimateTokens(paperContext(p)) + 400
	if budget < 500 {
		budget = 500
	}
	return budget
}

// summaryContinuePrompt resumes an overview the provider cut mid-way.
const summaryContinuePrompt = "The overview above was cut off before it was finished. Continue it exactly from the point where it stopped: do not repeat anything already written, do not restart or re-title any section, no preamble. Output only the remaining text and finish the overview."

// summaryContinuations is how many resume passes a cut overview gets.
const summaryContinuations = 3

// looksComplete reports whether generated markdown ends like finished prose:
// a closed sentence, a table row, a display formula or a code fence. Some
// providers hand back a reply cut mid-word with a normal finish signal, so the
// finish_reason alone cannot be trusted for a long structured output.
func looksComplete(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.HasSuffix(s, "$$") || strings.HasSuffix(s, "```") {
		return true
	}
	runes := []rune(s)
	return strings.ContainsRune(".!?…»\")]|", runes[len(runes)-1])
}

// GenerateSummary streams a structured overview; onDelta may be nil.
//
// A reply that was cut gets up to summaryContinuations resume passes: the
// partial text goes back as an assistant turn and the model finishes it. In
// practice the cut is rarely the output cap — it is the context window running
// out (the paper alone can be ~30k tokens, and some routed providers stop at
// 32k with a plain "stop"), so every resume pass halves the paper text it
// carries to make room for the rest of the reply. Deltas of the continuation
// stream through the same callback, so the client sees one reply.
func (l LLM) GenerateSummary(
	ctx context.Context,
	p catalog.PaperOut,
	fullPaper, lang string,
	onDelta func(string) error,
) (string, error) {
	paperBudget := l.SummaryPaperBudget(p)
	system, turns := BuildSummaryPrompt(p, fullPaper, lang, paperBudget)
	l.MaxTokens = summaryMaxTokens

	out, err := l.requestStream(ctx, system, turns, onDelta)
	cut := errors.Is(err, ErrLLMTruncated)
	if err != nil && !cut {
		return "", err
	}
	if !cut && looksComplete(out) {
		return strings.TrimSpace(out), nil
	}
	if strings.TrimSpace(out) == "" {
		// Reasoning models can spend the whole cap thinking and emit nothing.
		return "", fmt.Errorf("обзор не уместился в лимит ответа модели %s — попробуйте другую модель", l.resolvedModel())
	}
	log.Printf("summary: model=%s first pass incomplete (cut=%v, %d chars) — resuming", l.resolvedModel(), cut, len(out))

	// What actually went to the model, as a base for shrinking.
	sentPaper := EstimateTokens(truncateToTokens(strings.TrimSpace(fullPaper), paperBudget))
	for attempt := 1; attempt <= summaryContinuations; attempt++ {
		shrunk := sentPaper >> attempt
		if shrunk < 500 {
			shrunk = 500
		}
		_, resumeTurns := BuildSummaryPrompt(p, fullPaper, lang, shrunk)
		resumeTurns = append(resumeTurns,
			ChatTurn{Role: "assistant", Content: out},
			ChatTurn{Role: "user", Content: summaryContinuePrompt},
		)
		more, err := l.requestStream(ctx, system, resumeTurns, onDelta)
		if err != nil && !errors.Is(err, ErrLLMTruncated) {
			if looksComplete(out) {
				return strings.TrimSpace(out), nil
			}
			return "", fmt.Errorf("обзор оборвался, а продолжить его не удалось: %w", err)
		}
		out += more
		if !errors.Is(err, ErrLLMTruncated) && looksComplete(out) {
			log.Printf("summary: model=%s completed after %d resume pass(es), %d chars", l.resolvedModel(), attempt, len(out))
			return strings.TrimSpace(out), nil
		}
		log.Printf("summary: model=%s resume pass %d still incomplete (+%d chars, paper budget %d tokens)", l.resolvedModel(), attempt, len(more), shrunk)
	}
	log.Printf("summary: model=%s giving up after %d resume passes, saving %d chars as is", l.resolvedModel(), summaryContinuations, len(out))
	return strings.TrimSpace(out), nil
}

// summaryOf loads a cached summary and flags it stale against the current parse.
func (a API) summaryOf(ctx context.Context, paperID uuid.UUID, lang string, currentVersion *uuid.UUID) (Summary, bool, error) {
	stored, err := a.store().GetSummary(ctx, paperID, lang)
	if err == pgx.ErrNoRows {
		return Summary{}, false, nil
	}
	if err != nil {
		return Summary{}, false, err
	}
	stored.Stale = currentVersion != nil && (stored.VersionID == nil || *stored.VersionID != *currentVersion)
	return stored, true, nil
}

package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// summarySource is the paper text a summary will be generated from.
type summarySource struct {
	Paper     catalog.PaperOut
	FullText  string
	VersionID *uuid.UUID
}

// loadSummarySource resolves the paper and its parsed text. It returns ok=false
// after writing an HTTP error, including 425 while parsing is still in flight.
func (a API) loadSummarySource(w http.ResponseWriter, r *http.Request, paperID uuid.UUID) (summarySource, bool) {
	paper, err := a.papers().GetPaperOut(r.Context(), paperID)
	if err != nil {
		httpx.Error(w, 404, "Paper not found")
		return summarySource{}, false
	}

	doc, err := a.docs().GetDocument(r.Context(), paperID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Never parsed (for example a DOI record without a PDF): abstract only.
		return summarySource{Paper: paper}, true
	}
	if err != nil {
		httpx.Error(w, 500, "Failed to load paper text")
		return summarySource{}, false
	}
	if doc.Status == "pending" {
		// 425 (not 409): the client must tell "text still parsing, retry the
		// POST later" apart from "someone else is generating, poll the GET".
		httpx.Error(w, http.StatusTooEarly, "Полный текст статьи ещё готовится — попробуйте через минуту")
		return summarySource{}, false
	}
	if doc.Status != "ready" {
		return summarySource{Paper: paper}, true
	}

	plain := strings.TrimSpace(doc.PlainText)
	if plain == "" {
		plain = strings.TrimSpace(doc.Markdown)
	}
	chunks, err := a.docs().ListChunks(r.Context(), paperID)
	if err != nil {
		httpx.Error(w, 500, "Failed to load paper text")
		return summarySource{}, false
	}
	versionID := doc.VersionID
	return summarySource{
		Paper:     paper,
		FullText:  FormatPaperWithPageMarkers(chunks, plain),
		VersionID: &versionID,
	}, true
}

// summaryModel picks the model for an overview: an explicit pick from the
// picker list, otherwise SUMMARY_LLM_MODEL (which need not be in the picker).
// Empty means the explicit pick was not a known model.
func (a API) summaryModel(requested string) string {
	if strings.TrimSpace(requested) == "" {
		if m := strings.TrimSpace(a.Config.SummaryLLMModel); m != "" {
			return m
		}
		return a.Config.LLMModel
	}
	modelID, ok := a.resolveModel(requested)
	if !ok {
		return ""
	}
	return modelID
}

// currentVersionID reports the parsed version a fresh summary would be built
// from, so cached rows can be flagged stale without regenerating them.
func (a API) currentVersionID(r *http.Request, paperID uuid.UUID) *uuid.UUID {
	doc, err := a.docs().GetDocument(r.Context(), paperID)
	if err != nil || doc.Status != "ready" {
		return nil
	}
	versionID := doc.VersionID
	return &versionID
}

func (a API) getSummary(w http.ResponseWriter, r *http.Request) {
	id, ok := a.Papers.RequirePaper(w, r)
	if !ok {
		return
	}
	lang := NormalizeSummaryLang(r.URL.Query().Get("lang"))

	stored, found, err := a.summaryOf(r.Context(), id, lang, a.currentVersionID(r, id))
	if err != nil {
		httpx.Error(w, 500, "Failed to load summary")
		return
	}
	if !found {
		httpx.Error(w, 404, "Summary has not been generated yet")
		return
	}
	httpx.JSON(w, 200, stored)
}

func (a API) createSummary(w http.ResponseWriter, r *http.Request) {
	id, ok := a.Papers.RequirePaper(w, r)
	if !ok {
		return
	}
	lang := NormalizeSummaryLang(r.URL.Query().Get("lang"))
	force := r.URL.Query().Get("force") == "1"
	modelID := a.summaryModel(r.URL.Query().Get("model"))
	if modelID == "" {
		httpx.Error(w, 400, "unknown model")
		return
	}

	source, ok := a.loadSummarySource(w, r, id)
	if !ok {
		return
	}

	if !force {
		stored, found, err := a.summaryOf(r.Context(), id, lang, source.VersionID)
		if err != nil {
			httpx.Error(w, 500, "Failed to load summary")
			return
		}
		if found && stored.Status == "ready" && !stored.Stale {
			a.writeSummary(w, r, stored)
			return
		}
	}

	claimed, err := a.store().ClaimSummary(r.Context(), id, lang, source.VersionID, modelID)
	if err != nil {
		httpx.Error(w, 500, "Failed to start summary generation")
		return
	}
	if !claimed {
		httpx.Error(w, 409, "Обзор уже генерируется — подождите немного")
		return
	}

	if wantsChatStream(r) {
		a.streamSummary(w, r, id, lang, modelID, source)
		return
	}

	content, err := a.llm(modelID).GenerateSummary(r.Context(), source.Paper, source.FullText, lang, nil)
	if err != nil {
		a.failSummary(r, id, lang, err)
		status := http.StatusBadGateway
		if errors.Is(err, ErrLLMNotConfigured) {
			status = http.StatusServiceUnavailable
		}
		httpx.Error(w, status, err.Error())
		return
	}

	saved, err := a.store().SaveSummary(r.Context(), id, lang, source.VersionID, modelID, content)
	if err != nil {
		httpx.Error(w, 500, "Summary generated but could not be saved")
		return
	}
	a.writeSummary(w, r, saved)
}

func (a API) streamSummary(
	w http.ResponseWriter,
	r *http.Request,
	paperID uuid.UUID,
	lang, modelID string,
	source summarySource,
) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		a.failSummary(r, paperID, lang, errors.New("streaming unsupported"))
		httpx.Error(w, 500, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	writeEvent := func(payload map[string]any) bool {
		raw, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	content, err := a.llm(modelID).GenerateSummary(
		r.Context(),
		source.Paper,
		source.FullText,
		lang,
		func(delta string) error {
			if !writeEvent(map[string]any{"type": "delta", "text": delta}) {
				return errors.New("client disconnected")
			}
			return nil
		},
	)
	if err != nil {
		a.failSummary(r, paperID, lang, err)
		_ = writeEvent(map[string]any{"type": "error", "detail": err.Error()})
		return
	}

	saved, err := a.store().SaveSummary(r.Context(), paperID, lang, source.VersionID, modelID, content)
	if err != nil {
		_ = writeEvent(map[string]any{"type": "error", "detail": "Обзор сгенерирован, но не сохранён"})
		return
	}
	_ = writeEvent(map[string]any{"type": "done", "summary": saved})
}

func (a API) writeSummary(w http.ResponseWriter, r *http.Request, summary Summary) {
	if !wantsChatStream(r) {
		httpx.JSON(w, 200, summary)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		httpx.JSON(w, 200, summary)
		return
	}
	// A cached hit still has to look like a stream to the SSE client.
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if raw, err := json.Marshal(map[string]any{"type": "done", "summary": summary}); err == nil {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
	}
	flusher.Flush()
}

// failSummary releases the pending claim. It uses a background-safe context
// because the request context may already be cancelled by a dropped client.
func (a API) failSummary(r *http.Request, paperID uuid.UUID, lang string, cause error) {
	ctx := context.WithoutCancel(r.Context())
	_ = a.store().FailSummary(ctx, paperID, lang, cause.Error())
}

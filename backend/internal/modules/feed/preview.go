package feed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/platform/httpx"
)

var previewID = regexp.MustCompile(`^(?:[0-9]{4}\.[0-9]{4,5}|[a-z-]+(?:\.[A-Z]{2})?/[0-9]{7})(?:v[1-9][0-9]*)?$`)
var previewSlots = make(chan struct{}, 3)

type PaperPreview struct {
	Image        string   `json:"image"`
	Affiliations []string `json:"affiliations"`
}

func (a API) preview(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("arxiv_id")
	if !previewID.MatchString(id) {
		httpx.Error(w, http.StatusBadRequest, "Invalid arXiv ID")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	key := "feed:preview:v3:" + id
	if a.Service.Redis != nil {
		if cached, err := a.Service.Redis.Get(ctx, key).Bytes(); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			_, _ = w.Write(cached)
			return
		}
	}
	// Не даём ленте запускать неограниченное число загрузок PDF одновременно.
	select {
	case previewSlots <- struct{}{}:
		defer func() { <-previewSlots }()
	default:
		w.Header().Set("Retry-After", "3")
		httpx.Error(w, http.StatusServiceUnavailable, "Preview is busy")
		return
	}
	result, err := a.Service.loadPreview(ctx, id)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "Preview is unavailable")
		return
	}
	if a.Service.Redis != nil {
		if data, err := json.Marshal(result); err == nil {
			_ = a.Service.Redis.Set(ctx, key, data, 7*24*time.Hour).Err()
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	httpx.JSON(w, http.StatusOK, result)
}

func (s Service) loadPreview(ctx context.Context, id string) (PaperPreview, error) {
	var result PaperPreview
	client := &http.Client{
		Timeout: 40 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || req.URL.Scheme != "https" || req.URL.Host != "arxiv.org" {
				return fmt.Errorf("unexpected PDF redirect")
			}
			return nil
		},
	}
	// URL строится только из проверенного ID, произвольные адреса не принимаются.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://arxiv.org/pdf/"+id, nil)
	if err != nil {
		return result, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	const maxPDF = 50 << 20
	if resp.StatusCode != http.StatusOK || resp.ContentLength > maxPDF {
		return result, fmt.Errorf("PDF unavailable or too large")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxPDF+1))
	if err != nil || len(raw) > maxPDF || !bytes.HasPrefix(raw, []byte("%PDF")) {
		return result, fmt.Errorf("invalid PDF response")
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	file, err := form.CreateFormFile("file", "paper.pdf")
	if err != nil {
		return result, err
	}
	_, _ = file.Write(raw)
	_ = form.Close()
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.ParserURL, "/")+"/v1/preview", &body)
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	parsed, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer parsed.Body.Close()
	if parsed.StatusCode != http.StatusOK {
		return result, fmt.Errorf("preview parser failed")
	}
	err = json.NewDecoder(io.LimitReader(parsed.Body, 4<<20)).Decode(&result)
	if err == nil && !strings.HasPrefix(result.Image, "data:image/png;base64,") {
		err = fmt.Errorf("invalid preview image")
	}
	return result, err
}

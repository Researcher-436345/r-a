package content

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
	OCR     string
}

type ParseChunk struct {
	ID            string `json:"id"`
	PageStart     int    `json:"page_start"`
	PageEnd       int    `json:"page_end"`
	Section       string `json:"section"`
	Text          string `json:"text"`
	TokenEstimate int    `json:"token_estimate"`
}

type ParseResult struct {
	Engine    string       `json:"engine"`
	OCRUsed   bool         `json:"ocr_used"`
	PageCount int          `json:"page_count"`
	Markdown  string       `json:"markdown"`
	PlainText string       `json:"plain_text"`
	Chunks    []ParseChunk `json:"chunks"`
	Warnings  []string     `json:"warnings"`
	Detail    string       `json:"detail"`
}

func (c Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 180 * time.Second}
}

func (c Client) ParsePDF(ctx context.Context, pdf []byte, paperID string) (ParseResult, error) {
	if c.BaseURL == "" {
		return ParseResult{}, fmt.Errorf("PARSER_SERVICE_URL is not configured")
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "paper.pdf")
	if err != nil {
		return ParseResult{}, err
	}
	if _, err = part.Write(pdf); err != nil {
		return ParseResult{}, err
	}
	ocr := c.OCR
	if ocr == "" {
		ocr = "auto"
	}
	_ = w.WriteField("ocr", ocr)
	if paperID != "" {
		_ = w.WriteField("paper_id", paperID)
	}
	if err = w.Close(); err != nil {
		return ParseResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, trimSlash(c.BaseURL)+"/v1/parse", &body)
	if err != nil {
		return ParseResult{}, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.client().Do(req)
	if err != nil {
		return ParseResult{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	var out ParseResult
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode/100 != 2 {
		detail := out.Detail
		if detail == "" {
			detail = string(raw)
		}
		if detail == "" {
			detail = resp.Status
		}
		return out, fmt.Errorf("parser error: %s", detail)
	}
	if out.PlainText == "" {
		out.PlainText = out.Markdown
	}
	return out, nil
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

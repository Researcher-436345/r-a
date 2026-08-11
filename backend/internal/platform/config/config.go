package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName     string
	Environment string
	HTTPAddr    string

	DatabaseURL string
	RedisURL    string

	JWTSecret       string
	JWTAlgorithm    string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	S3Endpoint       string
	S3PublicEndpoint string
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	S3Region         string
	S3PresignExpire  time.Duration

	LLMProvider    string
	LLMBaseURL     string
	LLMAPIKey      string
	LLMModel       string
	LLMModels      []LLMModelOption
	LLMTimeout     time.Duration
	LLMHTTPReferer string
	LLMAppTitle    string

	TranslationHTTPAddr      string
	TranslationServiceURL    string
	TranslationMaxChars      int
	TranslationMaxConcurrent int

	ParserServiceURL string
	ParserOCR        string
	ParserTimeout    time.Duration

	LLMContextTokens int
	LLMReplyReserve  int
	ChatRecentKeep   int

	// Citations — OpenAlex без ключа сейчас; S2 — когда появится ключ.
	CitationsEnabled      bool
	OpenAlexMailto        string
	SemanticScholarAPIKey string

	// Service discovery (gateway + inter-service).
	IdentityURL     string
	CatalogURL      string
	LibraryURL      string
	AnnotationsURL  string
	AssistantURL    string
	FeedURL         string
	InternalToken   string
}

type LLMModelOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func Load() Config {
	return Config{
		AppName:     getenv("APP_NAME", "researcher-api"),
		Environment: getenv("ENVIRONMENT", "local"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8000"),

		DatabaseURL: normalizeDBURL(getenv("DATABASE_URL", "postgresql://researcher:researcher@localhost:5432/researcher")),
		RedisURL:    getenv("REDIS_URL", "redis://localhost:6379/0"),

		JWTSecret:       getenv("JWT_SECRET", "dev-only-change-me-in-production"),
		JWTAlgorithm:    getenv("JWT_ALGORITHM", "HS256"),
		AccessTokenTTL:  minutes(getenv("ACCESS_TOKEN_EXPIRE_MINUTES", "30"), 30),
		RefreshTokenTTL: days(getenv("REFRESH_TOKEN_EXPIRE_DAYS", "14"), 14),

		S3Endpoint:       getenv("S3_ENDPOINT_URL", "http://localhost:9000"),
		S3PublicEndpoint: getenv("S3_PUBLIC_ENDPOINT_URL", "http://localhost:9000"),
		S3AccessKey:      getenv("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:      getenv("S3_SECRET_KEY", "minioadmin"),
		S3Bucket:         getenv("S3_BUCKET", "papers"),
		S3Region:         getenv("S3_REGION", "us-east-1"),
		S3PresignExpire:  seconds(getenv("S3_PRESIGN_EXPIRE_SECONDS", "900"), 900),

		LLMProvider:    getenv("LLM_PROVIDER", "openai_compatible"),
		LLMBaseURL:     getenv("LLM_BASE_URL", "https://api.aitunnel.ru/v1"),
		LLMAPIKey:      getenv("LLM_API_KEY", ""),
		LLMModel:       getenv("LLM_MODEL", "auto"),
		LLMModels:      parseLLMModels(getenv("LLM_MODELS", ""), getenv("LLM_MODEL", "auto")),
		LLMTimeout:     seconds(getenv("LLM_TIMEOUT_SECONDS", "60"), 60),
		LLMHTTPReferer: getenv("LLM_HTTP_REFERER", "http://localhost:5173"),
		LLMAppTitle:    getenv("LLM_APP_TITLE", "Researcher"),

		TranslationHTTPAddr:      getenv("TRANSLATION_HTTP_ADDR", ":8090"),
		TranslationServiceURL:    getenv("TRANSLATION_SERVICE_URL", "http://localhost:8090"),
		TranslationMaxChars:      positiveInt(getenv("TRANSLATION_MAX_CHARS", "5000"), 5000),
		TranslationMaxConcurrent: positiveInt(getenv("TRANSLATION_MAX_CONCURRENT", "8"), 8),

		ParserServiceURL: getenv("PARSER_SERVICE_URL", "http://localhost:8091"),
		ParserOCR:        getenv("PARSER_OCR", "auto"),
		ParserTimeout:    seconds(getenv("PARSER_TIMEOUT_SECONDS", "180"), 180),

		LLMContextTokens: positiveInt(getenv("LLM_CONTEXT_TOKENS", "120000"), 120000),
		LLMReplyReserve:  positiveInt(getenv("LLM_REPLY_RESERVE", "4000"), 4000),
		ChatRecentKeep:   positiveInt(getenv("CHAT_RECENT_KEEP", "8"), 8),

		CitationsEnabled:      getenv("CITATIONS_ENABLED", "true") != "false",
		OpenAlexMailto:        getenv("OPENALEX_MAILTO", "researcher@localhost"),
		SemanticScholarAPIKey: getenv("SEMANTIC_SCHOLAR_API_KEY", ""),

		IdentityURL:    strings.TrimRight(getenv("IDENTITY_URL", "http://localhost:8101"), "/"),
		CatalogURL:     strings.TrimRight(getenv("CATALOG_URL", "http://localhost:8102"), "/"),
		LibraryURL:     strings.TrimRight(getenv("LIBRARY_URL", "http://localhost:8103"), "/"),
		AnnotationsURL: strings.TrimRight(getenv("ANNOTATIONS_URL", "http://localhost:8104"), "/"),
		AssistantURL:   strings.TrimRight(getenv("ASSISTANT_URL", "http://localhost:8105"), "/"),
		FeedURL:        strings.TrimRight(getenv("FEED_URL", "http://localhost:8106"), "/"),
		InternalToken:  getenv("INTERNAL_TOKEN", ""),
	}
}

func positiveInt(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func normalizeDBURL(u string) string {
	u = strings.Replace(u, "postgresql+psycopg://", "postgres://", 1)
	u = strings.Replace(u, "postgresql+asyncpg://", "postgres://", 1)
	if strings.HasPrefix(u, "postgresql://") {
		u = "postgres://" + strings.TrimPrefix(u, "postgresql://")
	}
	return u
}

func minutes(s string, def int) time.Duration {
	n, err := strconv.Atoi(s)
	if err != nil {
		n = def
	}
	return time.Duration(n) * time.Minute
}

func days(s string, def int) time.Duration {
	n, err := strconv.Atoi(s)
	if err != nil {
		n = def
	}
	return time.Duration(n) * 24 * time.Hour
}

func seconds(s string, def int) time.Duration {
	n, err := strconv.Atoi(s)
	if err != nil {
		n = def
	}
	return time.Duration(n) * time.Second
}

// LLM_MODELS="id|Label,id2|Label2" — if empty, only default LLM_MODEL.
func parseLLMModels(raw, defaultID string) []LLMModelOption {
	defaultID = strings.TrimSpace(defaultID)
	seen := map[string]bool{}
	out := make([]LLMModelOption, 0, 4)
	add := func(id, label string) {
		id = strings.TrimSpace(id)
		label = strings.TrimSpace(label)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		if label == "" {
			label = shortModelLabel(id)
		}
		out = append(out, LLMModelOption{ID: id, Label: label})
	}
	if defaultID != "" {
		add(defaultID, shortModelLabel(defaultID))
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, label, ok := strings.Cut(part, "|")
		if !ok {
			add(part, shortModelLabel(part))
			continue
		}
		add(id, label)
	}
	return out
}

func shortModelLabel(id string) string {
	if i := strings.LastIndex(id, "/"); i >= 0 && i+1 < len(id) {
		return id[i+1:]
	}
	return id
}

func (c Config) ResolveLLMModel(requested string) (string, bool) {
	req := strings.TrimSpace(requested)
	if req == "" {
		return c.LLMModel, true
	}
	for _, m := range c.LLMModels {
		if m.ID == req {
			return m.ID, true
		}
	}
	if req == c.LLMModel {
		return req, true
	}
	return "", false
}

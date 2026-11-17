package config

import (
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

	// Auth sessions / email verification (identity).
	FrontendURL              string
	CookieSecure             bool
	SMTPHost                 string
	SMTPPort                 string
	SMTPUser                 string
	SMTPPass                 string
	SMTPFrom                 string
	MailEnabled              bool
	EmailVerificationEnabled bool

	S3Endpoint       string
	S3PublicEndpoint string
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	S3Region         string
	S3PresignExpire  time.Duration

	LLMProvider         string
	LLMBaseURL          string
	LLMAPIKey           string
	LLMModel            string
	LLMModels           []LLMModelOption
	LLMTimeout          time.Duration
	LLMHTTPReferer      string
	LLMAppTitle         string
	LLMMaxOutputTokens  int
	LLMReasoningEffort  string
	ChatReasoningEffort string

	// Paper overviews can override the shared model and reasoning/output budgets.
	SummaryLLMModel        string
	SummaryReasoningEffort string
	SummaryMaxOutputTokens int
	SummaryReplyReserve    int

	TranslationHTTPAddr      string
	TranslationServiceURL    string
	TranslationLLMModel      string
	TranslationBaseURL       string
	TranslationAPIKey        string
	TranslationTimeout       time.Duration
	TranslationMaxChars      int
	TranslationMaxConcurrent int
	FeedTranslationModel     string
	FeedTranslationBaseURL   string
	FeedTranslationAPIKey    string
	FeedRefreshInterval      time.Duration

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
	FrontendServiceURL  string
	IdentityURL         string
	CatalogURL          string
	LibraryURL          string
	AnnotationsURL      string
	AssistantURL        string
	FeedURL             string
	SearchAPIURL        string
	WebSearchServiceURL string
	WebSearchTimeout    time.Duration
	DeepResearchTimeout time.Duration
	InternalToken       string
}

type LLMModelOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func Load() Config {
	settings := ReadSettings()
	value := settings.Value
	seconds := func(name string) time.Duration {
		return time.Duration(settings.PositiveInt(name)) * time.Second
	}
	minutes := func(name string) time.Duration {
		return time.Duration(settings.PositiveInt(name)) * time.Minute
	}
	return Config{
		AppName:     value("APP_NAME"),
		Environment: value("ENVIRONMENT"),
		HTTPAddr:    value("HTTP_ADDR"),
		DatabaseURL: normalizeDBURL(value("DATABASE_URL")),
		RedisURL:    value("REDIS_URL"),

		JWTSecret:                value("JWT_SECRET"),
		JWTAlgorithm:             value("JWT_ALGORITHM"),
		AccessTokenTTL:           minutes("ACCESS_TOKEN_EXPIRE_MINUTES"),
		RefreshTokenTTL:          time.Duration(settings.PositiveInt("REFRESH_TOKEN_EXPIRE_DAYS")) * 24 * time.Hour,
		FrontendURL:              strings.TrimRight(value("FRONTEND_URL"), "/"),
		CookieSecure:             settings.Bool("COOKIE_SECURE"),
		SMTPHost:                 value("SMTP_HOST"),
		SMTPPort:                 value("SMTP_PORT"),
		SMTPUser:                 value("SMTP_USER"),
		SMTPPass:                 value("SMTP_PASS"),
		SMTPFrom:                 value("SMTP_FROM"),
		MailEnabled:              settings.Bool("MAIL_ENABLED"),
		EmailVerificationEnabled: settings.Bool("EMAIL_VERIFICATION_ENABLED"),

		S3Endpoint:       value("S3_ENDPOINT_URL"),
		S3PublicEndpoint: value("S3_PUBLIC_ENDPOINT_URL"),
		S3AccessKey:      value("S3_ACCESS_KEY"),
		S3SecretKey:      value("S3_SECRET_KEY"),
		S3Bucket:         value("S3_BUCKET"),
		S3Region:         value("S3_REGION"),
		S3PresignExpire:  seconds("S3_PRESIGN_EXPIRE_SECONDS"),

		LLMProvider:         value("LLM_PROVIDER"),
		LLMBaseURL:          value("LLM_BASE_URL"),
		LLMAPIKey:           value("LLM_API_KEY"),
		LLMModel:            value("LLM_MODEL"),
		LLMModels:           parseLLMModels(value("LLM_MODELS"), value("LLM_MODEL")),
		LLMTimeout:          seconds("LLM_TIMEOUT_SECONDS"),
		LLMMaxOutputTokens:  settings.PositiveInt("LLM_MAX_OUTPUT_TOKENS"),
		LLMReasoningEffort:  value("LLM_REASONING_EFFORT"),
		ChatReasoningEffort: value("CHAT_REASONING_EFFORT"),
		LLMHTTPReferer:      value("LLM_HTTP_REFERER"),
		LLMAppTitle:         value("LLM_APP_TITLE"),

		SummaryLLMModel:        value("SUMMARY_LLM_MODEL"),
		SummaryReasoningEffort: value("SUMMARY_REASONING_EFFORT"),
		SummaryMaxOutputTokens: settings.PositiveInt("SUMMARY_MAX_OUTPUT_TOKENS"),
		SummaryReplyReserve:    settings.PositiveInt("SUMMARY_REPLY_RESERVE"),

		TranslationHTTPAddr:      value("TRANSLATION_HTTP_ADDR"),
		TranslationServiceURL:    value("TRANSLATION_SERVICE_URL"),
		TranslationBaseURL:       value("TRANSLATION_BASE_URL"),
		TranslationAPIKey:        value("TRANSLATION_API_KEY"),
		TranslationLLMModel:      value("TRANSLATION_LLM_MODEL"),
		TranslationTimeout:       seconds("TRANSLATION_TIMEOUT_SECONDS"),
		TranslationMaxChars:      settings.PositiveInt("TRANSLATION_MAX_CHARS"),
		TranslationMaxConcurrent: settings.PositiveInt("TRANSLATION_MAX_CONCURRENT"),

		FeedTranslationModel:   value("FEED_TRANSLATION_MODEL"),
		FeedTranslationBaseURL: value("FEED_TRANSLATION_BASE_URL"),
		FeedTranslationAPIKey:  value("FEED_TRANSLATION_API_KEY"),
		FeedRefreshInterval:    minutes("FEED_REFRESH_MINUTES"),

		ParserServiceURL: value("PARSER_SERVICE_URL"),
		ParserOCR:        value("PARSER_OCR"),
		ParserTimeout:    seconds("PARSER_TIMEOUT_SECONDS"),

		LLMContextTokens: settings.PositiveInt("LLM_CONTEXT_TOKENS"),
		LLMReplyReserve:  settings.PositiveInt("LLM_REPLY_RESERVE"),
		ChatRecentKeep:   settings.PositiveInt("CHAT_RECENT_KEEP"),

		CitationsEnabled:      settings.Bool("CITATIONS_ENABLED"),
		OpenAlexMailto:        value("OPENALEX_MAILTO"),
		SemanticScholarAPIKey: value("SEMANTIC_SCHOLAR_API_KEY"),

		FrontendServiceURL:  strings.TrimRight(value("FRONTEND_SERVICE_URL"), "/"),
		IdentityURL:         strings.TrimRight(value("IDENTITY_URL"), "/"),
		CatalogURL:          strings.TrimRight(value("CATALOG_URL"), "/"),
		LibraryURL:          strings.TrimRight(value("LIBRARY_URL"), "/"),
		AnnotationsURL:      strings.TrimRight(value("ANNOTATIONS_URL"), "/"),
		AssistantURL:        strings.TrimRight(value("ASSISTANT_URL"), "/"),
		FeedURL:             strings.TrimRight(value("FEED_URL"), "/"),
		SearchAPIURL:        strings.TrimRight(value("SEARCH_API_URL"), "/"),
		WebSearchServiceURL: strings.TrimRight(value("WEBSEARCH_SERVICE_URL"), "/"),
		WebSearchTimeout:    seconds("WEBSEARCH_TIMEOUT_SECONDS"),
		DeepResearchTimeout: seconds("WEBSEARCH_DEEP_TIMEOUT_SECONDS"),
		InternalToken:       value("INTERNAL_TOKEN"),
	}
}

func normalizeDBURL(u string) string {
	u = strings.Replace(u, "postgresql+psycopg://", "postgres://", 1)
	u = strings.Replace(u, "postgresql+asyncpg://", "postgres://", 1)
	if strings.HasPrefix(u, "postgresql://") {
		u = "postgres://" + strings.TrimPrefix(u, "postgresql://")
	}
	return u
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

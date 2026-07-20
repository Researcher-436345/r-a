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
	LLMTimeout     time.Duration
	LLMHTTPReferer string
	LLMAppTitle    string
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
		LLMTimeout:     seconds(getenv("LLM_TIMEOUT_SECONDS", "60"), 60),
		LLMHTTPReferer: getenv("LLM_HTTP_REFERER", "http://localhost:5173"),
		LLMAppTitle:    getenv("LLM_APP_TITLE", "Researcher"),
	}
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

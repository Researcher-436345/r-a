// Package config loads and validates service configuration from environment
// variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the assistant service.
type Config struct {
	Port string

	OpenAIAPIKey   string
	OpenAIBaseURL  string // optional; empty means the SDK default
	OpenAIModel    string
	LLMMaxTokens   int64
	LLMTemperature float64

	ArticleServiceURL string
	ChatServiceURL    string

	RequestTimeout  time.Duration
	UpstreamTimeout time.Duration
}

// Load reads configuration from the environment, applies defaults and validates
// that required values are present. A .env file in the working directory is
// loaded first (real environment variables take precedence over it).
func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, fmt.Errorf("loading .env: %w", err)
	}

	c := Config{
		Port:              getenv("PORT", "8080"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:     os.Getenv("OPENAI_BASE_URL"),
		OpenAIModel:       getenv("OPENAI_MODEL", "gpt-4o"),
		ArticleServiceURL: os.Getenv("ARTICLE_SERVICE_URL"),
		ChatServiceURL:    os.Getenv("CHAT_SERVICE_URL"),
	}

	var err error
	if c.LLMMaxTokens, err = getenvInt("LLM_MAX_TOKENS", 1024); err != nil {
		return Config{}, err
	}
	if c.LLMTemperature, err = getenvFloat("LLM_TEMPERATURE", 0.2); err != nil {
		return Config{}, err
	}
	if c.RequestTimeout, err = getenvDuration("REQUEST_TIMEOUT", 60*time.Second); err != nil {
		return Config{}, err
	}
	if c.UpstreamTimeout, err = getenvDuration("UPSTREAM_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}

	var missing []string
	if c.OpenAIAPIKey == "" {
		missing = append(missing, "OPENAI_API_KEY")
	}
	if c.ArticleServiceURL == "" {
		missing = append(missing, "ARTICLE_SERVICE_URL")
	}
	if c.ChatServiceURL == "" {
		missing = append(missing, "CHAT_SERVICE_URL")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %v", missing)
	}

	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int64) (int64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return n, nil
}

func getenvFloat(key string, def float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return f, nil
}

func getenvDuration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return d, nil
}

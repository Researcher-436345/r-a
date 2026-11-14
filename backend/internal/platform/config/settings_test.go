package config

import "testing"

func TestSeparateProviderDefaultsAndModelInheritance(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-test")
	t.Setenv("OPENROUTER_API_KEY", "openrouter-test")
	t.Setenv("LLM_MODEL", "main-model")
	t.Setenv("SUMMARY_LLM_MODEL", "overview-model")
	cfg := Load()
	if cfg.LLMBaseURL != "https://api.openai.com/v1" || cfg.TranslationBaseURL != "https://openrouter.ai/api/v1" {
		t.Fatal("provider URLs are not independent")
	}
	if cfg.LLMAPIKey != "openai-test" || cfg.TranslationAPIKey != "openrouter-test" || cfg.FeedTranslationAPIKey != "openrouter-test" {
		t.Fatal("provider keys crossed")
	}
	if cfg.SummaryLLMModel != "overview-model" || cfg.FeedTranslationModel != "google/gemma-4-26b-a4b-it" {
		t.Fatal("per-feature models did not resolve")
	}
}

func TestUnknownSettingsFailFast(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("unknown setting silently accepted")
		}
	}()
	ReadSettings().Value("MISSPELLED_SETTING")
}

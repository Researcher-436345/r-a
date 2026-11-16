import importlib
import json
import subprocess
import sys

import pytest

import settings


@pytest.fixture
def load_settings(monkeypatch):

    yield lambda: importlib.reload(settings)

    monkeypatch.undo()
    importlib.reload(settings)


def exported_settings(*arguments):

    result = subprocess.run(
        [sys.executable, settings.__file__, *arguments],
        check=True,
        capture_output=True,
        text=True,
    )

    return json.loads(result.stdout)


def test_openai_defaults_use_medium_except_deep_and_preserve_gemma(monkeypatch, load_settings):

    for name in (
        "LLM_MODEL",
        "SUMMARY_LLM_MODEL",
        "WEBSEARCH_LLM_MODEL",
        "WEBSEARCH_DEEP_LLM_MODEL",
        "TRANSLATION_LLM_MODEL",
        "FEED_TRANSLATION_MODEL",
        "LLM_REASONING_EFFORT",
        "CHAT_REASONING_EFFORT",
        "SUMMARY_REASONING_EFFORT",
        "WEBSEARCH_REASONING_EFFORT",
        "WEBSEARCH_DEEP_REASONING_EFFORT",
        "LLM_MAX_OUTPUT_TOKENS",
        "SUMMARY_MAX_OUTPUT_TOKENS",
        "WEBSEARCH_MAX_OUTPUT_TOKENS",
        "WEBSEARCH_DEEP_MAX_OUTPUT_TOKENS",
    ):
        monkeypatch.delenv(name, raising=False)

    config = load_settings()

    assert config.LLM_MODEL == "gpt-6.1-terra"
    assert config.SUMMARY_LLM_MODEL == "gpt-6.1-terra"
    assert config.WEBSEARCH_LLM_MODEL == "gpt-6.1-terra"
    assert config.WEBSEARCH_DEEP_LLM_MODEL == "gpt-6.1-terra"
    assert config.LLM_REASONING_EFFORT == "medium"
    assert config.CHAT_REASONING_EFFORT == "medium"
    assert config.SUMMARY_REASONING_EFFORT == "medium"
    assert config.WEBSEARCH_REASONING_EFFORT == "medium"
    assert config.WEBSEARCH_DEEP_REASONING_EFFORT == "max"
    assert config.LLM_MAX_OUTPUT_TOKENS == "16000"
    assert config.SUMMARY_MAX_OUTPUT_TOKENS == "32000"
    assert config.WEBSEARCH_MAX_OUTPUT_TOKENS == "16000"
    assert config.WEBSEARCH_DEEP_MAX_OUTPUT_TOKENS == "64000"
    assert config.TRANSLATION_LLM_MODEL == "google/gemma-4-26b-a4b-it"
    assert config.FEED_TRANSLATION_MODEL == "google/gemma-4-26b-a4b-it"


def test_deep_reasoning_is_independent_and_can_be_overridden(monkeypatch, load_settings):

    for name in (
        "CHAT_REASONING_EFFORT",
        "SUMMARY_REASONING_EFFORT",
        "WEBSEARCH_REASONING_EFFORT",
        "WEBSEARCH_DEEP_REASONING_EFFORT",
    ):
        monkeypatch.delenv(name, raising=False)

    monkeypatch.setenv("LLM_REASONING_EFFORT", "low")
    config = load_settings()

    assert config.CHAT_REASONING_EFFORT == "low"
    assert config.SUMMARY_REASONING_EFFORT == "low"
    assert config.WEBSEARCH_REASONING_EFFORT == "low"
    assert config.WEBSEARCH_DEEP_REASONING_EFFORT == "max"

    monkeypatch.setenv("WEBSEARCH_DEEP_REASONING_EFFORT", "high")

    assert load_settings().WEBSEARCH_DEEP_REASONING_EFFORT == "high"


def test_provider_keys_and_urls_do_not_cross(monkeypatch, load_settings):

    monkeypatch.setenv("OPENAI_API_KEY", "openai-test-key")
    monkeypatch.setenv("OPENROUTER_API_KEY", "openrouter-test-key")
    monkeypatch.setenv("OPENAI_BASE_URL", "https://openai.example/v1")
    monkeypatch.setenv("OPENROUTER_BASE_URL", "https://openrouter.example/v1")
    config = load_settings()

    assert config.LLM_API_KEY == "openai-test-key"
    assert config.WEBSEARCH_LLM_API_KEY == "openai-test-key"
    assert config.TRANSLATION_API_KEY == "openrouter-test-key"
    assert config.FEED_TRANSLATION_API_KEY == "openrouter-test-key"
    assert config.WEBSEARCH_LLM_BASE_URL == "https://openai.example/v1"
    assert config.FEED_TRANSLATION_BASE_URL == "https://openrouter.example/v1"


def test_shared_model_can_be_overridden_per_feature(monkeypatch, load_settings):

    monkeypatch.setenv("LLM_MODEL", "main-model")
    monkeypatch.setenv("WEBSEARCH_DEEP_LLM_MODEL", "research-model")
    config = load_settings()

    assert config.SUMMARY_LLM_MODEL == "main-model"
    assert config.WEBSEARCH_LLM_MODEL == "main-model"
    assert config.WEBSEARCH_DEEP_LLM_MODEL == "research-model"


def test_empty_env_can_disable_domain_filter(monkeypatch, load_settings):

    monkeypatch.setenv("WEBSEARCH_DOMAINS", "")

    assert load_settings().WEBSEARCH_DOMAINS == ""


def test_display_hides_secrets_but_shows_models_and_budgets(monkeypatch):

    monkeypatch.setenv("OPENAI_API_KEY", "private-value")
    monkeypatch.setenv("LLM_MODEL", "visible-model")
    values = exported_settings()

    assert "private-value" not in str(values)
    assert values["LLM_API_KEY"] == "<задан>"
    assert values["LLM_MODEL"] == "visible-model"
    assert values["LLM_MAX_OUTPUT_TOKENS"] == "16000"
    assert values["OPENAI_BASE_URL"] == "https://api.openai.com/v1"


def test_frontend_export_never_contains_server_settings_or_secrets(monkeypatch):

    monkeypatch.setenv("OPENAI_API_KEY", "private-value")
    monkeypatch.setenv("DEV_PORT", "5180")
    values = exported_settings("--frontend")

    assert values["DEV_PORT"] == "5180"
    assert values["VITE_API_URL"] == "/api"
    assert "private-value" not in str(values)
    assert "DATABASE_URL" not in values
    assert "OPENAI_API_KEY" not in values

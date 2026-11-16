"""Настройки приложения: значения по умолчанию с переопределением через env."""

import os


# Приложение
APP_NAME = os.getenv("APP_NAME", "researcher-api")
ENVIRONMENT = os.getenv("ENVIRONMENT", "local")
HTTP_ADDR = os.getenv("HTTP_ADDR", ":8000")
DATABASE_URL = os.getenv("DATABASE_URL", "postgresql://researcher:researcher@localhost:5433/researcher")
REDIS_URL = os.getenv("REDIS_URL", "redis://localhost:6379/0")
FRONTEND_URL = os.getenv("FRONTEND_URL", "http://localhost:5180")
INTERNAL_TOKEN = os.getenv("INTERNAL_TOKEN", "")

# Авторизация и почта
JWT_SECRET = os.getenv("JWT_SECRET", "dev-only-change-me-in-production")
JWT_ALGORITHM = os.getenv("JWT_ALGORITHM", "HS256")
ACCESS_TOKEN_EXPIRE_MINUTES = os.getenv("ACCESS_TOKEN_EXPIRE_MINUTES", "30")
REFRESH_TOKEN_EXPIRE_DAYS = os.getenv("REFRESH_TOKEN_EXPIRE_DAYS", "14")
COOKIE_SECURE = os.getenv("COOKIE_SECURE", "false")
SMTP_HOST = os.getenv("SMTP_HOST", "localhost")
SMTP_PORT = os.getenv("SMTP_PORT", "1025")
SMTP_USER = os.getenv("SMTP_USER", "")
SMTP_PASS = os.getenv("SMTP_PASS", "")
SMTP_FROM = os.getenv("SMTP_FROM", "researcher@localhost")
MAIL_ENABLED = os.getenv("MAIL_ENABLED", "false")
EMAIL_VERIFICATION_ENABLED = os.getenv("EMAIL_VERIFICATION_ENABLED", "true")

# Хранилище файлов
S3_ENDPOINT_URL = os.getenv("S3_ENDPOINT_URL", "http://localhost:9000")
S3_PUBLIC_ENDPOINT_URL = os.getenv("S3_PUBLIC_ENDPOINT_URL", S3_ENDPOINT_URL)
S3_ACCESS_KEY = os.getenv("S3_ACCESS_KEY", "minioadmin")
S3_SECRET_KEY = os.getenv("S3_SECRET_KEY", "minioadmin")
S3_BUCKET = os.getenv("S3_BUCKET", "papers")
S3_REGION = os.getenv("S3_REGION", "us-east-1")
S3_PRESIGN_EXPIRE_SECONDS = os.getenv("S3_PRESIGN_EXPIRE_SECONDS", "900")

# Основная модель: прямой OpenAI API
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY", "")
OPENAI_BASE_URL = os.getenv("OPENAI_BASE_URL", "https://api.openai.com/v1")
LLM_PROVIDER = os.getenv("LLM_PROVIDER", "openai")
LLM_BASE_URL = os.getenv("LLM_BASE_URL", OPENAI_BASE_URL)
LLM_API_KEY = os.getenv("LLM_API_KEY", OPENAI_API_KEY)
LLM_MODEL = os.getenv("LLM_MODEL", "gpt-6.1-terra")
LLM_MODELS = os.getenv("LLM_MODELS", "")
LLM_TIMEOUT_SECONDS = os.getenv("LLM_TIMEOUT_SECONDS", "180")
LLM_MAX_OUTPUT_TOKENS = os.getenv("LLM_MAX_OUTPUT_TOKENS", "16000")
LLM_REASONING_EFFORT = os.getenv("LLM_REASONING_EFFORT", "medium")
CHAT_REASONING_EFFORT = os.getenv("CHAT_REASONING_EFFORT", LLM_REASONING_EFFORT)
SUMMARY_LLM_MODEL = os.getenv("SUMMARY_LLM_MODEL", LLM_MODEL)
SUMMARY_REASONING_EFFORT = os.getenv("SUMMARY_REASONING_EFFORT", LLM_REASONING_EFFORT)
SUMMARY_MAX_OUTPUT_TOKENS = os.getenv("SUMMARY_MAX_OUTPUT_TOKENS", "32000")
SUMMARY_REPLY_RESERVE = os.getenv("SUMMARY_REPLY_RESERVE", "9000")
LLM_CONTEXT_TOKENS = os.getenv("LLM_CONTEXT_TOKENS", "120000")
LLM_REPLY_RESERVE = os.getenv("LLM_REPLY_RESERVE", "4000")
CHAT_RECENT_KEEP = os.getenv("CHAT_RECENT_KEEP", "8")

# Перевод и описания ленты: прямой OpenRouter API
OPENROUTER_API_KEY = os.getenv("OPENROUTER_API_KEY", "")
OPENROUTER_BASE_URL = os.getenv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1")
LLM_HTTP_REFERER = os.getenv("LLM_HTTP_REFERER", FRONTEND_URL)
LLM_APP_TITLE = os.getenv("LLM_APP_TITLE", "Researcher")
TRANSLATION_HTTP_ADDR = os.getenv("TRANSLATION_HTTP_ADDR", ":8090")
TRANSLATION_SERVICE_URL = os.getenv("TRANSLATION_SERVICE_URL", "http://localhost:8090")
TRANSLATION_BASE_URL = os.getenv("TRANSLATION_BASE_URL", OPENROUTER_BASE_URL)
TRANSLATION_API_KEY = os.getenv("TRANSLATION_API_KEY", OPENROUTER_API_KEY)
TRANSLATION_LLM_MODEL = os.getenv("TRANSLATION_LLM_MODEL", "google/gemma-4-26b-a4b-it")
TRANSLATION_TIMEOUT_SECONDS = os.getenv("TRANSLATION_TIMEOUT_SECONDS", "60")
TRANSLATION_MAX_CHARS = os.getenv("TRANSLATION_MAX_CHARS", "5000")
TRANSLATION_MAX_CONCURRENT = os.getenv("TRANSLATION_MAX_CONCURRENT", "8")
FEED_TRANSLATION_MODEL = os.getenv("FEED_TRANSLATION_MODEL", TRANSLATION_LLM_MODEL)
FEED_TRANSLATION_BASE_URL = os.getenv("FEED_TRANSLATION_BASE_URL", TRANSLATION_BASE_URL)
FEED_TRANSLATION_API_KEY = os.getenv("FEED_TRANSLATION_API_KEY", TRANSLATION_API_KEY)
FEED_REFRESH_MINUTES = os.getenv("FEED_REFRESH_MINUTES", "15")

# Веб-поиск и глубокое исследование
WEBSEARCH_SERVICE_URL = os.getenv("WEBSEARCH_SERVICE_URL", "http://localhost:8092")
WEBSEARCH_LLM_BASE_URL = os.getenv("WEBSEARCH_LLM_BASE_URL", OPENAI_BASE_URL)
WEBSEARCH_LLM_API_KEY = os.getenv("WEBSEARCH_LLM_API_KEY", OPENAI_API_KEY)
WEBSEARCH_LLM_MODEL = os.getenv("WEBSEARCH_LLM_MODEL", LLM_MODEL)
WEBSEARCH_DEEP_LLM_MODEL = os.getenv("WEBSEARCH_DEEP_LLM_MODEL", LLM_MODEL)
WEBSEARCH_TIMEOUT_SECONDS = os.getenv("WEBSEARCH_TIMEOUT_SECONDS", "180")
WEBSEARCH_DEEP_TIMEOUT_SECONDS = os.getenv("WEBSEARCH_DEEP_TIMEOUT_SECONDS", "1800")
WEBSEARCH_REASONING_EFFORT = os.getenv("WEBSEARCH_REASONING_EFFORT", LLM_REASONING_EFFORT)
WEBSEARCH_DEEP_REASONING_EFFORT = os.getenv("WEBSEARCH_DEEP_REASONING_EFFORT", "max")
WEBSEARCH_MAX_TOOL_CALLS = os.getenv("WEBSEARCH_MAX_TOOL_CALLS", "6")
WEBSEARCH_DEEP_MAX_TOOL_CALLS = os.getenv("WEBSEARCH_DEEP_MAX_TOOL_CALLS", "40")
WEBSEARCH_MAX_OUTPUT_TOKENS = os.getenv("WEBSEARCH_MAX_OUTPUT_TOKENS", "16000")
WEBSEARCH_DEEP_MAX_OUTPUT_TOKENS = os.getenv("WEBSEARCH_DEEP_MAX_OUTPUT_TOKENS", "64000")
WEBSEARCH_STREAM_RETRIES = os.getenv("WEBSEARCH_STREAM_RETRIES", "3")
WEBSEARCH_DOMAINS = os.getenv(
    "WEBSEARCH_DOMAINS",
    "arxiv.org,doi.org,semanticscholar.org,aclanthology.org,proceedings.mlr.press,openaccess.thecvf.com,proceedings.neurips.cc,papers.nips.cc,ojs.aaai.org",
)

# Парсер PDF
PARSER_SERVICE_URL = os.getenv("PARSER_SERVICE_URL", "http://localhost:8091")
PARSER_ENGINE = os.getenv("PARSER_ENGINE", "pymupdf")
PARSER_OCR = os.getenv("PARSER_OCR", "auto")
PARSER_MAX_UPLOAD_BYTES = os.getenv("PARSER_MAX_UPLOAD_BYTES", "53477376")
PARSER_TIMEOUT_SECONDS = os.getenv("PARSER_TIMEOUT_SECONDS", "180")

# Цитирования
CITATIONS_ENABLED = os.getenv("CITATIONS_ENABLED", "true")
OPENALEX_MAILTO = os.getenv("OPENALEX_MAILTO", "researcher@localhost")
SEMANTIC_SCHOLAR_API_KEY = os.getenv("SEMANTIC_SCHOLAR_API_KEY", "")

# Адреса внутренних сервисов
IDENTITY_URL = os.getenv("IDENTITY_URL", "http://localhost:8101")
CATALOG_URL = os.getenv("CATALOG_URL", "http://localhost:8102")
LIBRARY_URL = os.getenv("LIBRARY_URL", "http://localhost:8103")
ANNOTATIONS_URL = os.getenv("ANNOTATIONS_URL", "http://localhost:8104")
ASSISTANT_URL = os.getenv("ASSISTANT_URL", "http://localhost:8105")
FEED_URL = os.getenv("FEED_URL", "http://localhost:8106")
SEARCH_API_URL = os.getenv("SEARCH_API_URL", "http://localhost:8107")

# Метрики
METRICS_ADDR = os.getenv("METRICS_ADDR", ":9091")
LLM_DAILY_BUDGET_USD = os.getenv("LLM_DAILY_BUDGET_USD", "0")

# Фронтенд
VITE_API_URL = os.getenv("VITE_API_URL", "/api")
DEV_API_TARGET = os.getenv("DEV_API_TARGET", "http://77.91.114.154:8081")
DEV_API_LOCAL_ADDRESS = os.getenv("DEV_API_LOCAL_ADDRESS", "")
DEV_PORT = os.getenv("DEV_PORT", "5180")
VITE_ENABLE_LANDING = os.getenv("VITE_ENABLE_LANDING", "false")
VITE_ENABLE_ACTIVE_SESSIONS = os.getenv("VITE_ENABLE_ACTIVE_SESSIONS", "false")
VITE_REQUIRE_LOGIN_ON_ENTRY = os.getenv("VITE_REQUIRE_LOGIN_ON_ENTRY", "false")
VITE_ENABLE_ANALYTICS = os.getenv("VITE_ENABLE_ANALYTICS", "true")


if __name__ == "__main__":
    import argparse
    import json

    parser = argparse.ArgumentParser(description="Показать настройки приложения без секретов.")
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument("--export", action="store_true", help="Внутренний экспорт настроек для Go.")
    modes.add_argument("--frontend", action="store_true", help="Экспорт публичных настроек для Vite.")
    args = parser.parse_args()
    values = {name: value for name, value in globals().copy().items() if name.isupper()}

    if args.frontend:
        values = {
            name: value for name, value in values.items()
            if name.startswith("VITE_") or name in ("DEV_API_TARGET", "DEV_API_LOCAL_ADDRESS", "DEV_PORT")
        }
    elif not args.export:
        for name, value in values.items():
            secret = any(part in name for part in ("API_KEY", "SECRET", "PASS", "INTERNAL_TOKEN", "DATABASE_URL", "REDIS_URL", "S3_ACCESS_KEY"))
            if secret:
                values[name] = "<задан>" if value else "<не задан>"

    print(json.dumps(values, ensure_ascii=False, indent=2))

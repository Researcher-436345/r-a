from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    app_name: str = "researcher-api"
    environment: str = "local"

    database_url: str = "postgresql+psycopg://researcher:researcher@localhost:5432/researcher"
    redis_url: str = "redis://localhost:6379/0"

    # Auth
    jwt_secret: str = "dev-only-change-me-in-production"
    jwt_algorithm: str = "HS256"
    access_token_expire_minutes: int = 30
    refresh_token_expire_days: int = 14

    # MinIO / S3
    # Internal endpoint used by API/worker inside Docker network.
    s3_endpoint_url: str = "http://localhost:9000"
    # Public endpoint for browser-facing signed URLs.
    s3_public_endpoint_url: str = "http://localhost:9000"
    s3_access_key: str = "minioadmin"
    s3_secret_key: str = "minioadmin"
    s3_bucket: str = "papers"
    s3_region: str = "us-east-1"
    s3_presign_expire_seconds: int = 900

    # LLM: openai_compatible (AITunnel, DeepSeek, OpenRouter) или gemini (не из РФ)
    llm_provider: str = "openai_compatible"
    llm_base_url: str = "https://api.aitunnel.ru/v1"
    llm_api_key: str | None = None
    # AITunnel: без VPN из РФ, оплата в рублях; auto — автовыбор модели
    llm_model: str = "auto"
    llm_timeout_seconds: int = 60
    llm_http_referer: str = "http://localhost:5173"
    llm_app_title: str = "Researcher"


settings = Settings()

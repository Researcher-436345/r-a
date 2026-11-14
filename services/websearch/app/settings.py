from __future__ import annotations

from dataclasses import dataclass

import settings


@dataclass(frozen=True)
class SearchProfile:
    model: str
    reasoning_effort: str
    timeout: int
    max_tool_calls: int
    max_output_tokens: int
    background: bool

    def __post_init__(self) -> None:

        if self.timeout <= 0 or self.max_tool_calls <= 0 or self.max_output_tokens <= 0:
            raise ValueError("Search timeout and budgets must be positive")


@dataclass(frozen=True)
class SearchSettings:
    base_url: str
    api_key: str
    domains: tuple[str, ...]
    web: SearchProfile
    deep: SearchProfile
    stream_retries: int
    internal_token: str

    @classmethod
    def load(cls) -> SearchSettings:

        domains = settings.WEBSEARCH_DOMAINS.split(",")
        retries = int(settings.WEBSEARCH_STREAM_RETRIES)

        if retries <= 0:
            raise ValueError("WEBSEARCH_STREAM_RETRIES must be positive")

        return cls(
            base_url=settings.WEBSEARCH_LLM_BASE_URL.rstrip("/"),
            api_key=settings.WEBSEARCH_LLM_API_KEY,
            domains=tuple(domain.strip() for domain in domains if domain.strip()),
            web=SearchProfile(
                model=settings.WEBSEARCH_LLM_MODEL.strip(),
                reasoning_effort=settings.WEBSEARCH_REASONING_EFFORT,
                timeout=int(settings.WEBSEARCH_TIMEOUT_SECONDS),
                max_tool_calls=int(settings.WEBSEARCH_MAX_TOOL_CALLS),
                max_output_tokens=int(settings.WEBSEARCH_MAX_OUTPUT_TOKENS),
                background=False,
            ),
            deep=SearchProfile(
                model=settings.WEBSEARCH_DEEP_LLM_MODEL.strip(),
                reasoning_effort=settings.WEBSEARCH_DEEP_REASONING_EFFORT,
                timeout=int(settings.WEBSEARCH_DEEP_TIMEOUT_SECONDS),
                max_tool_calls=int(settings.WEBSEARCH_DEEP_MAX_TOOL_CALLS),
                max_output_tokens=int(settings.WEBSEARCH_DEEP_MAX_OUTPUT_TOKENS),
                background=True,
            ),
            stream_retries=retries,
            internal_token=settings.INTERNAL_TOKEN,
        )
